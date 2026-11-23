package metadata

import (
	"context"
	"fmt"

	"github.com/ayushanand/tiny-aws/control-plane/internal/auth"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

type Service struct {
	Store *store.Store
}

func (s *Service) DocumentByName(ctx context.Context, name string) (Document, error) {
	inst, err := s.Store.GetByName(ctx, name)
	if err != nil {
		return Document{}, err
	}
	if err := auth.EnsureOwner(ctx, inst.OwnerID); err != nil {
		return Document{}, err
	}
	return s.documentFor(ctx, inst)
}

func (s *Service) DocumentByVeth(ctx context.Context, vethHost string) (Document, error) {
	if vethHost == "" {
		return Document{}, fmt.Errorf("veth host is required")
	}
	att, err := s.Store.GetAttachmentByVethHost(ctx, vethHost)
	if err != nil {
		return Document{}, err
	}
	inst, err := s.Store.GetByID(ctx, att.InstanceID)
	if err != nil {
		return Document{}, err
	}
	if err := auth.EnsureOwner(ctx, inst.OwnerID); err != nil {
		return Document{}, err
	}
	return s.documentFor(ctx, inst)
}

func (s *Service) PathByName(ctx context.Context, name, path string) (string, error) {
	doc, err := s.DocumentByName(ctx, name)
	if err != nil {
		return "", err
	}
	val, ok := PathValue(doc, path)
	if !ok {
		return "", fmt.Errorf("metadata path not found")
	}
	return val, nil
}

func (s *Service) documentFor(ctx context.Context, inst types.Instance) (Document, error) {
	if inst.Status != types.StatusRunning {
		return Document{}, ErrNotRunning(inst.Name)
	}
	atts, err := s.Store.ListAttachmentsByInstance(ctx, inst.ID)
	if err != nil {
		return Document{}, err
	}
	var att *types.NetworkAttachment
	if len(atts) > 0 {
		a := atts[0]
		att = &a
	}
	return Build(inst, att), nil
}
