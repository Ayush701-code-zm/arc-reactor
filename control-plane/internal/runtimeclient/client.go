package runtimeclient

import (
	"context"

	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

type Client interface {
	CreateInstance(ctx context.Context, inst types.Instance) (types.RuntimeView, error)
	ListInstances(ctx context.Context) ([]types.RuntimeView, error)
	InspectInstance(ctx context.Context, id string) (types.RuntimeView, error)
	StartInstance(ctx context.Context, id string) (types.RuntimeView, error)
	StopInstance(ctx context.Context, id string) (types.RuntimeView, error)
	DeleteInstance(ctx context.Context, id string) error
	AttachVolume(ctx context.Context, instanceID, volumeID, diskPath, rootfsPath, mountName string) (devicePath, mountPath string, err error)
	DetachVolume(ctx context.Context, instanceID, volumeID, devicePath, rootfsPath, mountName string) error
	AttachNetwork(ctx context.Context, instanceID, networkID, bridgeName, vethHost, vethGuest, guestIPCIDR, gatewayIP string) error
	DetachNetwork(ctx context.Context, instanceID, networkID, bridgeName, vethHost, vethGuest string, deleteBridgeIfEmpty bool) error
	Close() error
}
