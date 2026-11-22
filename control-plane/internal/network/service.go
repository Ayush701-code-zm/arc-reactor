package network

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/ayushanand/tiny-aws/control-plane/internal/runtimeclient"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
	"github.com/google/uuid"
)

type Service struct {
	Store   *store.Store
	Runtime runtimeclient.Client
}

func (s *Service) Create(ctx context.Context, name, cidr string) (types.Network, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return types.Network{}, fmt.Errorf("name is required")
	}
	ipNet, err := ParseCIDR(cidr)
	if err != nil {
		return types.Network{}, err
	}
	gw, err := GatewayIP(ipNet)
	if err != nil {
		return types.Network{}, err
	}
	if _, err := s.Store.GetNetworkByName(ctx, name); err == nil {
		return types.Network{}, fmt.Errorf("network name %q already exists", name)
	}
	id := uuid.NewString()
	net := types.Network{
		ID:         id,
		Name:       name,
		CIDR:       ipNet.String(),
		GatewayIP:  gw.String(),
		BridgeName: BridgeName(id),
		Status:     types.NetworkStatusReady,
	}
	if err := s.Store.InsertNetwork(ctx, net); err != nil {
		return types.Network{}, err
	}
	return net, nil
}

func (s *Service) List(ctx context.Context) ([]types.Network, error) {
	return s.Store.ListNetworks(ctx)
}

func (s *Service) Inspect(ctx context.Context, name string) (types.Network, error) {
	return s.Store.GetNetworkByName(ctx, name)
}

func (s *Service) Delete(ctx context.Context, name string) error {
	net, err := s.Store.GetNetworkByName(ctx, name)
	if err != nil {
		return err
	}
	n, err := s.Store.CountAttachmentsByNetwork(ctx, net.ID)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("network %q still has %d attachment(s)", name, n)
	}
	return s.Store.DeleteNetwork(ctx, net.ID)
}

func (s *Service) Attach(ctx context.Context, networkName, instanceName string) (types.NetworkAttachment, error) {
	nw, err := s.Store.GetNetworkByName(ctx, networkName)
	if err != nil {
		return types.NetworkAttachment{}, err
	}
	inst, err := s.Store.GetByName(ctx, instanceName)
	if err != nil {
		return types.NetworkAttachment{}, err
	}
	if inst.Status != types.StatusRunning {
		return types.NetworkAttachment{}, fmt.Errorf("instance %q is not RUNNING", instanceName)
	}
	if _, err := s.Store.GetAttachment(ctx, nw.ID, inst.ID); err == nil {
		return types.NetworkAttachment{}, fmt.Errorf("instance %q already attached to network %q", instanceName, networkName)
	}
	existing, err := s.Store.ListAttachmentsByInstance(ctx, inst.ID)
	if err != nil {
		return types.NetworkAttachment{}, err
	}
	if len(existing) > 0 {
		return types.NetworkAttachment{}, fmt.Errorf("instance %q already has a network attachment (M4 is single-NIC)", instanceName)
	}

	ipNet, err := ParseCIDR(nw.CIDR)
	if err != nil {
		return types.NetworkAttachment{}, err
	}
	gw := net.ParseIP(nw.GatewayIP)
	if gw == nil {
		return types.NetworkAttachment{}, fmt.Errorf("invalid gateway ip %q", nw.GatewayIP)
	}
	used := map[string]struct{}{nw.GatewayIP: {}}
	atts, err := s.Store.ListAttachmentsByNetwork(ctx, nw.ID)
	if err != nil {
		return types.NetworkAttachment{}, err
	}
	for _, a := range atts {
		used[a.GuestIP] = struct{}{}
	}
	guestIP, err := AllocateGuestIP(ipNet, gw, used)
	if err != nil {
		return types.NetworkAttachment{}, err
	}
	guestCIDR := GuestCIDR(guestIP, ipNet)
	vethHost := VethHostName(inst.ID)
	vethGuest := "eth0"

	if err := s.Runtime.AttachNetwork(ctx, inst.ID, nw.ID, nw.BridgeName, vethHost, vethGuest, guestCIDR, nw.GatewayIP); err != nil {
		return types.NetworkAttachment{}, err
	}
	att := types.NetworkAttachment{
		NetworkID:    nw.ID,
		NetworkName:  nw.Name,
		InstanceID:   inst.ID,
		InstanceName: inst.Name,
		GuestIP:      guestIP.String(),
		GuestIPCIDR:  guestCIDR,
		VethHost:     vethHost,
		VethGuest:    vethGuest,
		Status:       "ATTACHED",
	}
	if err := s.Store.InsertAttachment(ctx, att); err != nil {
		_ = s.Runtime.DetachNetwork(ctx, inst.ID, nw.ID, nw.BridgeName, vethHost, vethGuest, false)
		return types.NetworkAttachment{}, fmt.Errorf("db attach update failed (compensated detach): %w", err)
	}
	return att, nil
}

func (s *Service) Detach(ctx context.Context, networkName, instanceName string) error {
	nw, err := s.Store.GetNetworkByName(ctx, networkName)
	if err != nil {
		return err
	}
	inst, err := s.Store.GetByName(ctx, instanceName)
	if err != nil {
		return err
	}
	att, err := s.Store.GetAttachment(ctx, nw.ID, inst.ID)
	if err != nil {
		return err
	}
	n, err := s.Store.CountAttachmentsByNetwork(ctx, nw.ID)
	if err != nil {
		return err
	}
	deleteBridge := n <= 1
	if err := s.Runtime.DetachNetwork(ctx, inst.ID, nw.ID, nw.BridgeName, att.VethHost, att.VethGuest, deleteBridge); err != nil {
		return err
	}
	return s.Store.DeleteAttachment(ctx, nw.ID, inst.ID)
}

func (s *Service) DetachAllForInstance(ctx context.Context, instanceID string) error {
	atts, err := s.Store.ListAttachmentsByInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	for _, a := range atts {
		nw, err := s.Store.GetNetworkByID(ctx, a.NetworkID)
		if err != nil {
			return err
		}
		n, err := s.Store.CountAttachmentsByNetwork(ctx, nw.ID)
		if err != nil {
			return err
		}
		deleteBridge := n <= 1
		if err := s.Runtime.DetachNetwork(ctx, instanceID, nw.ID, nw.BridgeName, a.VethHost, a.VethGuest, deleteBridge); err != nil {
			return fmt.Errorf("detach network %q: %w", nw.Name, err)
		}
		if err := s.Store.DeleteAttachment(ctx, nw.ID, instanceID); err != nil {
			return err
		}
	}
	return nil
}
