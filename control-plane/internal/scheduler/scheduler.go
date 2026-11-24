package scheduler

import (
	"context"
	"fmt"

	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

type Hub interface {
	ConnectedIDs() []string
}

type Service struct {
	Store *store.Store
	Hub   Hub
}

func (s *Service) Place(ctx context.Context, cpu uint32, memoryBytes uint64) (string, error) {
	if s.Hub == nil {
		return "", fmt.Errorf("no node hub")
	}
	ids := s.Hub.ConnectedIDs()
	if len(ids) == 0 {
		return "", fmt.Errorf("no node with capacity")
	}
	for _, id := range ids {
		n, err := s.Store.GetNode(ctx, id)
		if err != nil {
			continue
		}
		if !n.Connected {
			continue
		}
		freeCPU := int64(n.AllocatableCPU) - int64(n.AllocatedCPU)
		freeMem := int64(n.AllocatableMemoryBytes) - int64(n.AllocatedMemoryBytes)
		if freeCPU >= int64(cpu) && freeMem >= int64(memoryBytes) {
			return id, nil
		}
	}
	return "", fmt.Errorf("no node with capacity")
}

func (s *Service) List(ctx context.Context) ([]types.Node, error) {
	return s.Store.ListNodes(ctx)
}
