package runtimeclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	tinyv1 "github.com/ayushanand/tiny-aws/control-plane/gen/tiny/v1"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPC struct {
	conn   *grpc.ClientConn
	client tinyv1.RuntimeServiceClient
}

func Dial(addr string) (*GRPC, error) {
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("runtime addr is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial runtime gRPC %s: %w", addr, err)
	}
	return &GRPC{
		conn:   conn,
		client: tinyv1.NewRuntimeServiceClient(conn),
	}, nil
}

func (c *GRPC) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *GRPC) CreateInstance(ctx context.Context, inst types.Instance) (types.RuntimeView, error) {
	resp, err := c.client.CreateInstance(ctx, &tinyv1.CreateInstanceRequest{
		Id:          inst.ID,
		Name:        inst.Name,
		Image:       inst.Image,
		Cpu:         inst.CPU,
		MemoryBytes: inst.MemoryBytes,
		RootfsPath:  inst.RootfsPath,
	})
	if err != nil {
		return types.RuntimeView{}, err
	}
	return types.RuntimeView{
		ID:           resp.GetId(),
		Name:         inst.Name,
		Image:        inst.Image,
		CPU:          inst.CPU,
		MemoryBytes:  inst.MemoryBytes,
		Status:       statusFromProto(resp.GetStatus()),
		PID:          resp.GetPid(),
		ErrorMessage: resp.GetErrorMessage(),
	}, nil
}

func (c *GRPC) ListInstances(ctx context.Context) ([]types.RuntimeView, error) {
	resp, err := c.client.ListInstances(ctx, &tinyv1.ListInstancesRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]types.RuntimeView, 0, len(resp.GetInstances()))
	for _, it := range resp.GetInstances() {
		out = append(out, viewFromProto(it))
	}
	return out, nil
}

func (c *GRPC) InspectInstance(ctx context.Context, id string) (types.RuntimeView, error) {
	resp, err := c.client.InspectInstance(ctx, &tinyv1.InspectInstanceRequest{Id: id})
	if err != nil {
		return types.RuntimeView{}, err
	}
	return viewFromProto(resp.GetInstance()), nil
}

func (c *GRPC) StartInstance(ctx context.Context, id string) (types.RuntimeView, error) {
	resp, err := c.client.StartInstance(ctx, &tinyv1.StartInstanceRequest{Id: id})
	if err != nil {
		return types.RuntimeView{}, err
	}
	return types.RuntimeView{
		ID:           resp.GetId(),
		Status:       statusFromProto(resp.GetStatus()),
		PID:          resp.GetPid(),
		ErrorMessage: resp.GetErrorMessage(),
	}, nil
}

func (c *GRPC) StopInstance(ctx context.Context, id string) (types.RuntimeView, error) {
	resp, err := c.client.StopInstance(ctx, &tinyv1.StopInstanceRequest{Id: id})
	if err != nil {
		return types.RuntimeView{}, err
	}
	return types.RuntimeView{
		ID:           resp.GetId(),
		Status:       statusFromProto(resp.GetStatus()),
		ErrorMessage: resp.GetErrorMessage(),
	}, nil
}

func (c *GRPC) DeleteInstance(ctx context.Context, id string) error {
	resp, err := c.client.DeleteInstance(ctx, &tinyv1.DeleteInstanceRequest{Id: id})
	if err != nil {
		return err
	}
	if resp.GetErrorMessage() != "" {
		return fmt.Errorf("%s", resp.GetErrorMessage())
	}
	return nil
}

func (c *GRPC) AttachVolume(ctx context.Context, instanceID, volumeID, diskPath, rootfsPath, mountName string) (string, string, error) {
	resp, err := c.client.AttachVolume(ctx, &tinyv1.AttachVolumeRequest{
		InstanceId: instanceID,
		VolumeId:   volumeID,
		DiskPath:   diskPath,
		RootfsPath: rootfsPath,
		MountName:  mountName,
	})
	if err != nil {
		return "", "", err
	}
	if resp.GetErrorMessage() != "" {
		return "", "", fmt.Errorf("%s", resp.GetErrorMessage())
	}
	return resp.GetDevicePath(), resp.GetMountPath(), nil
}

func (c *GRPC) DetachVolume(ctx context.Context, instanceID, volumeID, devicePath, rootfsPath, mountName string) error {
	resp, err := c.client.DetachVolume(ctx, &tinyv1.DetachVolumeRequest{
		InstanceId: instanceID,
		VolumeId:   volumeID,
		DevicePath: devicePath,
		RootfsPath: rootfsPath,
		MountName:  mountName,
	})
	if err != nil {
		return err
	}
	if resp.GetErrorMessage() != "" {
		return fmt.Errorf("%s", resp.GetErrorMessage())
	}
	return nil
}

func viewFromProto(it *tinyv1.Instance) types.RuntimeView {
	if it == nil {
		return types.RuntimeView{}
	}
	return types.RuntimeView{
		ID:           it.GetId(),
		Name:         it.GetName(),
		Image:        it.GetImage(),
		CPU:          it.GetCpu(),
		MemoryBytes:  it.GetMemoryBytes(),
		Status:       statusFromProto(it.GetStatus()),
		PID:          it.GetPid(),
		ErrorMessage: it.GetErrorMessage(),
	}
}

func statusFromProto(s tinyv1.InstanceStatus) types.Status {
	switch s {
	case tinyv1.InstanceStatus_INSTANCE_STATUS_PENDING:
		return types.StatusPending
	case tinyv1.InstanceStatus_INSTANCE_STATUS_RUNNING:
		return types.StatusRunning
	case tinyv1.InstanceStatus_INSTANCE_STATUS_STOPPING:
		return types.StatusStopping
	case tinyv1.InstanceStatus_INSTANCE_STATUS_STOPPED:
		return types.StatusStopped
	case tinyv1.InstanceStatus_INSTANCE_STATUS_FAILED:
		return types.StatusFailed
	default:
		return types.StatusFailed
	}
}
