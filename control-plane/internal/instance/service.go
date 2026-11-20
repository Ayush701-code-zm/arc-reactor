package instance

import (
	"context"
	"fmt"

	"github.com/ayushanand/tiny-aws/control-plane/internal/runtimeclient"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
	"github.com/google/uuid"
)

type Service struct {
	Store   *store.Store
	Runtime runtimeclient.Client
	Images  ImageResolver
}

type ImageResolver interface {
	ResolveRootfs(ctx context.Context, name string) (types.Image, error)
}

func (s *Service) Create(ctx context.Context, spec types.Spec) (types.Instance, error) {
	if err := validateSpec(spec); err != nil {
		return types.Instance{}, err
	}

	var rootfs string
	if s.Images != nil {
		img, err := s.Images.ResolveRootfs(ctx, spec.Image)
		if err != nil {
			return types.Instance{}, err
		}
		rootfs = img.RootfsPath
	}

	inst := types.Instance{
		ID:          uuid.NewString(),
		Name:        spec.Name,
		Image:       spec.Image,
		RootfsPath:  rootfs,
		CPU:         spec.CPU,
		MemoryBytes: spec.MemoryBytes,
		Status:      types.StatusPending,
	}

	if err := s.Store.InsertPending(ctx, inst); err != nil {
		return types.Instance{}, err
	}

	view, err := s.Runtime.CreateInstance(ctx, inst)
	if err != nil {
		_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusFailed, 0, err.Error())
		inst.Status = types.StatusFailed
		inst.ErrorMessage = err.Error()
		return inst, err
	}

	status := view.Status
	if status == "" {
		status = types.StatusFailed
	}
	if err := s.Store.UpdateLifecycle(ctx, inst.ID, status, view.PID, view.ErrorMessage); err != nil {
		if delErr := s.Runtime.DeleteInstance(ctx, inst.ID); delErr != nil {
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
	return inst, nil
}

func (s *Service) List(ctx context.Context) ([]types.Instance, error) {
	return s.Store.List(ctx)
}

func (s *Service) Inspect(ctx context.Context, name string) (types.Instance, error) {
	inst, err := s.Store.GetByName(ctx, name)
	if err != nil {
		return types.Instance{}, err
	}
	view, err := s.Runtime.InspectInstance(ctx, inst.ID)
	if err != nil {
		return inst, nil
	}
	_ = s.Store.UpdateLifecycle(ctx, inst.ID, view.Status, view.PID, view.ErrorMessage)
	inst.Status = view.Status
	inst.PID = view.PID
	inst.ErrorMessage = view.ErrorMessage
	return inst, nil
}

func (s *Service) Stop(ctx context.Context, name string) (types.Instance, error) {
	inst, err := s.Store.GetByName(ctx, name)
	if err != nil {
		return types.Instance{}, err
	}
	_ = s.Store.UpdateLifecycle(ctx, inst.ID, types.StatusStopping, inst.PID, "")
	view, err := s.Runtime.StopInstance(ctx, inst.ID)
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
	view, err := s.Runtime.StartInstance(ctx, inst.ID)
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
	if err := s.Runtime.DeleteInstance(ctx, inst.ID); err != nil {
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
