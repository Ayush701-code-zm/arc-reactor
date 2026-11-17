package runtimeclient

import (
	"context"
	"fmt"

	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

// Client is the Go side of proto/tiny/v1 RuntimeService.
// A real gRPC implementation will replace Stub once codegen is wired.
type Client interface {
	CreateInstance(ctx context.Context, inst types.Instance) (types.RuntimeView, error)
	ListInstances(ctx context.Context) ([]types.RuntimeView, error)
	InspectInstance(ctx context.Context, id string) (types.RuntimeView, error)
	StartInstance(ctx context.Context, id string) (types.RuntimeView, error)
	StopInstance(ctx context.Context, id string) (types.RuntimeView, error)
	DeleteInstance(ctx context.Context, id string) error
}

// Stub returns clear not-implemented / local errors until gRPC is connected.
// ProcessSupervisorLocal optionally backs Create/Stop/etc for single-process demos.
type Stub struct {
	Local LocalRuntime
}

// LocalRuntime is an in-process runtime used on the same machine for M1 demos
// until gRPC stubs are generated and tinyruntimed is listening.
type LocalRuntime interface {
	Create(ctx context.Context, inst types.Instance) (types.RuntimeView, error)
	List(ctx context.Context) ([]types.RuntimeView, error)
	Inspect(ctx context.Context, id string) (types.RuntimeView, error)
	Start(ctx context.Context, id string) (types.RuntimeView, error)
	Stop(ctx context.Context, id string) (types.RuntimeView, error)
	Delete(ctx context.Context, id string) error
}

func (s *Stub) CreateInstance(ctx context.Context, inst types.Instance) (types.RuntimeView, error) {
	if s.Local != nil {
		return s.Local.Create(ctx, inst)
	}
	return types.RuntimeView{}, fmt.Errorf("runtime CreateInstance: not connected (run tinyruntimed or enable local runtime)")
}

func (s *Stub) ListInstances(ctx context.Context) ([]types.RuntimeView, error) {
	if s.Local != nil {
		return s.Local.List(ctx)
	}
	return nil, fmt.Errorf("runtime ListInstances: not connected")
}

func (s *Stub) InspectInstance(ctx context.Context, id string) (types.RuntimeView, error) {
	if s.Local != nil {
		return s.Local.Inspect(ctx, id)
	}
	return types.RuntimeView{}, fmt.Errorf("runtime InspectInstance: not connected")
}

func (s *Stub) StartInstance(ctx context.Context, id string) (types.RuntimeView, error) {
	if s.Local != nil {
		return s.Local.Start(ctx, id)
	}
	return types.RuntimeView{}, fmt.Errorf("runtime StartInstance: not connected")
}

func (s *Stub) StopInstance(ctx context.Context, id string) (types.RuntimeView, error) {
	if s.Local != nil {
		return s.Local.Stop(ctx, id)
	}
	return types.RuntimeView{}, fmt.Errorf("runtime StopInstance: not connected")
}

func (s *Stub) DeleteInstance(ctx context.Context, id string) error {
	if s.Local != nil {
		return s.Local.Delete(ctx, id)
	}
	return fmt.Errorf("runtime DeleteInstance: not connected")
}
