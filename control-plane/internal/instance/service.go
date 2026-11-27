package instance

import (
	"context"
	"fmt"

	"github.com/ayushanand/tiny-aws/control-plane/internal/auth"
	"github.com/ayushanand/tiny-aws/control-plane/internal/ebpfclient"
	"github.com/ayushanand/tiny-aws/control-plane/internal/runtimeclient"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
	"github.com/google/uuid"
)

type NodePlacer interface {
	Place(ctx context.Context, cpu uint32, memoryBytes uint64) (nodeID string, err error)
}

type NodeRuntime interface {
	HasConnected() bool
	CreateInstance(ctx context.Context, nodeID string, inst types.Instance) (types.RuntimeView, error)
	StartInstance(ctx context.Context, nodeID, id string) (types.RuntimeView, error)
	StopInstance(ctx context.Context, nodeID, id string) (types.RuntimeView, error)
	DeleteInstance(ctx context.Context, nodeID, id string) error
	InspectInstance(ctx context.Context, nodeID, id string) (types.RuntimeView, error)
}

type Service struct {
	Store     *store.Store
	Runtime   runtimeclient.Client
	Scheduler NodePlacer
	Nodes     NodeRuntime
	Images    ImageResolver
	Volumes   VolumeDetacher
	Networks  NetworkAttacher
	Ebpf      *ebpfclient.Client
}

type ImageResolver interface {
	ResolveRootfs(ctx context.Context, name string) (types.Image, error)
}

type VolumeDetacher interface {
	DetachAllForInstance(ctx context.Context, instanceID string) error
}

type NetworkAttacher interface {
	Attach(ctx context.Context, networkName, instanceName string) (types.NetworkAttachment, error)
	DetachAllForInstance(ctx context.Context, instanceID string) error
}

func (s *Service) Create(ctx context.Context, spec types.Spec) (types.Instance, error) {
	if err := validateSpec(spec); err != nil {
		return types.Instance{}, err
	}
	ownerID, err := auth.OwnerID(ctx)
	if err != nil {
		return types.Instance{}, err
	}

	var img types.Image
	if s.Images != nil {
		resolved, err := s.Images.ResolveRootfs(ctx, spec.Image)
		if err != nil {
			return types.Instance{}, err
		}
		img = resolved
	}

	nodeID := "local"
	useAgent := s.Nodes != nil && s.Nodes.HasConnected()
	if useAgent {
		if s.Scheduler == nil {
			return types.Instance{}, fmt.Errorf("scheduler required when nodes are connected")
		}
		placed, err := s.Scheduler.Place(ctx, spec.CPU, spec.MemoryBytes)
		if err != nil {
			return types.Instance{}, err
		}
		nodeID = placed
		if img.ID == "" {
			return types.Instance{}, fmt.Errorf("image id required for node create")
		}
		ni, err := s.Store.GetNodeImage(ctx, nodeID, img.ID)
		if err != nil || ni.Status != types.NodeImageReady {
			return types.Instance{}, fmt.Errorf(
				"image %s is not READY on node %s; place the image on that node before create",
				img.ID, nodeID)
		}
	}

	rootfs := ""
	if !useAgent {
		rootfs = img.RootfsPath
	}

	inst := types.Instance{
		ID:          uuid.NewString(),
		Name:        spec.Name,
		Image:       spec.Image,
		ImageID:     img.ID,
		RootfsPath:  rootfs,
		CPU:         spec.CPU,
		MemoryBytes: spec.MemoryBytes,
		Status:      types.StatusPending,
		OwnerID:     ownerID,
		NodeID:      nodeID,
	}

	if err := s.Store.InsertPending(ctx, inst); err != nil {
		return types.Instance{}, err
	}

	var view types.RuntimeView
	var createErr error
	if useAgent {
		view, createErr = s.Nodes.CreateInstance(ctx, nodeID, inst)
	} else {
		view, createErr = s.Runtime.CreateInstance(ctx, inst)
	}
	if createErr != nil {
		_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusFailed, 0, createErr.Error())
		inst.Status = types.StatusFailed
		inst.ErrorMessage = createErr.Error()
		return inst, createErr
	}

	status := view.Status
	if status == "" {
		status = types.StatusFailed
	}
	if err := s.Store.UpdateLifecycle(ctx, inst.ID, status, view.PID, view.ErrorMessage); err != nil {
		if delErr := s.deleteOnNode(ctx, inst); delErr != nil {
			msg := fmt.Sprintf("persist failed: %v; runtime cleanup failed: %v", err, delErr)
			_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusFailed, view.PID, msg)
			return types.Instance{}, fmt.Errorf("%s", msg)
		}
		_ = s.Store.Delete(ctx, inst.ID)
		return types.Instance{}, fmt.Errorf("persist runtime result: %w", err)
	}

	inst.Status = status
	inst.PID = view.PID
	inst.ErrorMessage = view.ErrorMessage
	if status == types.StatusFailed {
		return inst, fmt.Errorf("instance startup failed: %s", view.ErrorMessage)
	}
	if spec.Network != "" && s.Networks != nil {
		if _, err := s.Networks.Attach(ctx, spec.Network, inst.Name); err != nil {
			_ = s.deleteOnNode(ctx, inst)
			_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusFailed, 0, err.Error())
			inst.Status = types.StatusFailed
			inst.ErrorMessage = err.Error()
			return inst, fmt.Errorf("network attach after create: %w", err)
		}
	}
	if err := s.watchEbpf(inst); err != nil {
		_ = s.deleteOnNode(ctx, inst)
		_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusFailed, 0, err.Error())
		inst.Status = types.StatusFailed
		inst.ErrorMessage = err.Error()
		return inst, err
	}
	return inst, nil
}

func (s *Service) deleteOnNode(ctx context.Context, inst types.Instance) error {
	if s.Nodes != nil && s.Nodes.HasConnected() && inst.NodeID != "" && inst.NodeID != "local" {
		return s.Nodes.DeleteInstance(ctx, inst.NodeID, inst.ID)
	}
	return s.Runtime.DeleteInstance(ctx, inst.ID)
}

func (s *Service) viaAgent(inst types.Instance) bool {
	return s.Nodes != nil && s.Nodes.HasConnected() && inst.NodeID != "" && inst.NodeID != "local"
}

func (s *Service) List(ctx context.Context) ([]types.Instance, error) {
	ownerID, err := auth.OwnerID(ctx)
	if err != nil {
		return nil, err
	}
	all, err := s.Store.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]types.Instance, 0, len(all))
	for _, inst := range all {
		if inst.OwnerID == ownerID {
			out = append(out, inst)
		}
	}
	return out, nil
}

func (s *Service) Inspect(ctx context.Context, name string) (types.Instance, error) {
	inst, err := s.Store.GetByName(ctx, name)
	if err != nil {
		return types.Instance{}, err
	}
	if err := auth.EnsureOwner(ctx, inst.OwnerID); err != nil {
		return types.Instance{}, err
	}
	var view types.RuntimeView
	var err2 error
	if s.viaAgent(inst) {
		view, err2 = s.Nodes.InspectInstance(ctx, inst.NodeID, inst.ID)
	} else {
		view, err2 = s.Runtime.InspectInstance(ctx, inst.ID)
	}
	if err2 != nil {
		return inst, nil
	}
	_ = s.Store.UpdateLifecycle(ctx, inst.ID, view.Status, view.PID, view.ErrorMessage)
	inst.Status = view.Status
	inst.PID = view.PID
	inst.ErrorMessage = view.ErrorMessage
	inst.Observability = s.sampleEbpf(inst.ID)
	return inst, nil
}

func (s *Service) Stop(ctx context.Context, name string) (types.Instance, error) {
	inst, err := s.Store.GetByName(ctx, name)
	if err != nil {
		return types.Instance{}, err
	}
	if err := auth.EnsureOwner(ctx, inst.OwnerID); err != nil {
		return types.Instance{}, err
	}
	_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusStopping, inst.PID, "")
	var view types.RuntimeView
	if s.viaAgent(inst) {
		view, err = s.Nodes.StopInstance(ctx, inst.NodeID, inst.ID)
	} else {
		view, err = s.Runtime.StopInstance(ctx, inst.ID)
	}
	if err != nil {
		_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusFailed, inst.PID, err.Error())
		return types.Instance{}, err
	}
	if view.Status != types.StatusStopped {
		msg := view.ErrorMessage
		if msg == "" {
			msg = fmt.Sprintf("stop not confirmed: status=%s", view.Status)
		}
		_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusFailed, view.PID, msg)
		inst.Status = types.StatusFailed
		inst.PID = view.PID
		inst.ErrorMessage = msg
		return inst, fmt.Errorf("%s", msg)
	}
	_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusStopped, 0, view.ErrorMessage)
	inst.Status = types.StatusStopped
	inst.PID = 0
	inst.ErrorMessage = view.ErrorMessage
	return inst, nil
}

func (s *Service) Start(ctx context.Context, name string) (types.Instance, error) {
	inst, err := s.Store.GetByName(ctx, name)
	if err != nil {
		return types.Instance{}, err
	}
	if err := auth.EnsureOwner(ctx, inst.OwnerID); err != nil {
		return types.Instance{}, err
	}
	var view types.RuntimeView
	if s.viaAgent(inst) {
		view, err = s.Nodes.StartInstance(ctx, inst.NodeID, inst.ID)
	} else {
		view, err = s.Runtime.StartInstance(ctx, inst.ID)
	}
	if err != nil {
		_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusFailed, 0, err.Error())
		return types.Instance{}, err
	}
	_ = s.Store.UpdateLifecycle(ctx, inst.ID, view.Status, view.PID, view.ErrorMessage)
	inst.Status = view.Status
	inst.PID = view.PID
	inst.ErrorMessage = view.ErrorMessage
	return inst, nil
}

func (s *Service) Delete(ctx context.Context, name string) error {
	inst, err := s.Store.GetByName(ctx, name)
	if err != nil {
		return err
	}
	if err := auth.EnsureOwner(ctx, inst.OwnerID); err != nil {
		return err
	}
	if s.Ebpf != nil {
		_ = s.Ebpf.Unwatch(inst.ID)
	}
	if s.Networks != nil {
		if err := s.Networks.DetachAllForInstance(ctx, inst.ID); err != nil {
			return err
		}
	}
	if s.Volumes != nil {
		if err := s.Volumes.DetachAllForInstance(ctx, inst.ID); err != nil {
			return err
		}
	}
	if err := s.deleteOnNode(ctx, inst); err != nil {
		_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusFailed, inst.PID, err.Error())
		return fmt.Errorf("runtime delete not confirmed: %w", err)
	}
	return s.Store.Delete(ctx, inst.ID)
}

func validateSpec(spec types.Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	if spec.Image == "" {
		return fmt.Errorf("image is required")
	}
	if spec.CPU < 1 {
		return fmt.Errorf("cpu must be >= 1")
	}
	if spec.MemoryBytes == 0 {
		return fmt.Errorf("memory must be > 0")
	}
	return nil
}

func (s *Service) watchEbpf(inst types.Instance) error {
	if s.Ebpf == nil {
		if ebpfclient.Require() {
			return fmt.Errorf("TINY_REQUIRE_EBPF=1 but ebpf client is nil")
		}
		return nil
	}
	sample, err := s.Ebpf.Watch(inst.ID, inst.PID)
	if err != nil {
		if ebpfclient.Require() {
			return fmt.Errorf("ebpf watch required: %w", err)
		}
		return nil
	}
	if ebpfclient.Require() && !sample.Ebpf {
		return fmt.Errorf("ebpf watch required but skb attach skipped: %s", sample.Error)
	}
	return nil
}

func (s *Service) sampleEbpf(instanceID string) *types.Observability {
	if s.Ebpf == nil {
		return nil
	}
	sample, err := s.Ebpf.Sample(instanceID)
	if err != nil {
		return &types.Observability{Error: err.Error()}
	}
	return &types.Observability{
		Ebpf:        sample.Ebpf,
		CPUUsec:     sample.CPUUsec,
		MemoryBytes: sample.MemoryBytes,
		RxPackets:   sample.RxPackets,
		RxBytes:     sample.RxBytes,
		TxPackets:   sample.TxPackets,
		TxBytes:     sample.TxBytes,
		Error:       sample.Error,
	}
}
