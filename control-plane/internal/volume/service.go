package volume

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ayushanand/tiny-aws/control-plane/internal/runtimeclient"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
	"github.com/google/uuid"
)

type ImageLookup interface {
	ResolveRootfs(ctx context.Context, name string) (types.Image, error)
}

type Service struct {
	Store   *store.Store
	Runtime runtimeclient.Client
	Images  ImageLookup
	VolRoot string
}

func DefaultRoot() string {
	if v := os.Getenv("TINY_VOLUME_ROOT"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/tiny-aws/volumes"
	}
	return filepath.Join(home, ".tiny-aws", "volumes")
}

func (s *Service) root() string {
	if s.VolRoot != "" {
		return s.VolRoot
	}
	return DefaultRoot()
}

func (s *Service) Create(ctx context.Context, name string, sizeBytes uint64) (types.Volume, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return types.Volume{}, fmt.Errorf("name is required")
	}
	if sizeBytes == 0 {
		return types.Volume{}, fmt.Errorf("size must be > 0")
	}
	if _, err := s.Store.GetVolumeByName(ctx, name); err == nil {
		return types.Volume{}, fmt.Errorf("volume name %q already exists", name)
	}

	id := uuid.NewString()
	base := filepath.Join(s.root(), id)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return types.Volume{}, err
	}
	disk := filepath.Join(base, "disk.img")
	f, err := os.OpenFile(disk, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		_ = os.RemoveAll(base)
		return types.Volume{}, err
	}
	if err := f.Truncate(int64(sizeBytes)); err != nil {
		_ = f.Close()
		_ = os.RemoveAll(base)
		return types.Volume{}, err
	}
	if err := f.Close(); err != nil {
		_ = os.RemoveAll(base)
		return types.Volume{}, err
	}

	vol := types.Volume{
		ID:        id,
		Name:      name,
		SizeBytes: sizeBytes,
		DiskPath:  disk,
		Status:    types.VolumeStatusAvailable,
		MountName: name,
	}
	if err := s.Store.InsertVolume(ctx, vol); err != nil {
		_ = os.RemoveAll(base)
		return types.Volume{}, err
	}
	return vol, nil
}

func (s *Service) List(ctx context.Context) ([]types.Volume, error) {
	return s.Store.ListVolumes(ctx)
}

func (s *Service) Inspect(ctx context.Context, name string) (types.Volume, error) {
	return s.Store.GetVolumeByName(ctx, name)
}

func (s *Service) Delete(ctx context.Context, name string) error {
	vol, err := s.Store.GetVolumeByName(ctx, name)
	if err != nil {
		return err
	}
	if vol.Status == types.VolumeStatusAttached {
		return fmt.Errorf("volume %q is attached; detach first", name)
	}
	base := filepath.Join(s.root(), vol.ID)
	_ = os.RemoveAll(base)
	return s.Store.DeleteVolume(ctx, vol.ID)
}

func (s *Service) Attach(ctx context.Context, volumeName, instanceName string) (types.Volume, error) {
	vol, err := s.Store.GetVolumeByName(ctx, volumeName)
	if err != nil {
		return types.Volume{}, err
	}
	if vol.Status == types.VolumeStatusAttached {
		return vol, fmt.Errorf("volume %q already attached", volumeName)
	}
	inst, err := s.Store.GetByName(ctx, instanceName)
	if err != nil {
		return types.Volume{}, err
	}
	if inst.Status != types.StatusRunning {
		return types.Volume{}, fmt.Errorf("instance %q is not RUNNING", instanceName)
	}
	img, err := s.Images.ResolveRootfs(ctx, inst.Image)
	if err != nil {
		return types.Volume{}, err
	}

	device, mountPath, err := s.Runtime.AttachVolume(ctx, inst.ID, vol.ID, vol.DiskPath, img.RootfsPath, vol.Name)
	if err != nil {
		_ = s.Store.UpdateVolumeAttach(ctx, vol.ID, types.VolumeStatusFailed, "", "", "", vol.Name, err.Error())
		vol.Status = types.VolumeStatusFailed
		vol.ErrorMessage = err.Error()
		return vol, err
	}
	if err := s.Store.UpdateVolumeAttach(ctx, vol.ID, types.VolumeStatusAttached, inst.ID, inst.Name, device, vol.Name, ""); err != nil {
		_ = s.Runtime.DetachVolume(ctx, inst.ID, vol.ID, device, img.RootfsPath, vol.Name)
		return vol, fmt.Errorf("db attach update failed (compensated detach): %w", err)
	}
	vol.Status = types.VolumeStatusAttached
	vol.AttachedInstanceID = inst.ID
	vol.AttachedInstance = inst.Name
	vol.DevicePath = device
	vol.MountName = vol.Name
	vol.ErrorMessage = ""
	_ = mountPath
	return vol, nil
}

func (s *Service) Detach(ctx context.Context, volumeName string) (types.Volume, error) {
	vol, err := s.Store.GetVolumeByName(ctx, volumeName)
	if err != nil {
		return types.Volume{}, err
	}
	if vol.Status != types.VolumeStatusAttached {
		return vol, fmt.Errorf("volume %q is not attached", volumeName)
	}
	inst, err := s.Store.GetByID(ctx, vol.AttachedInstanceID)
	rootfs := ""
	if err == nil {
		if img, ierr := s.Images.ResolveRootfs(ctx, inst.Image); ierr == nil {
			rootfs = img.RootfsPath
		}
	}
	if err := s.Runtime.DetachVolume(ctx, vol.AttachedInstanceID, vol.ID, vol.DevicePath, rootfs, vol.MountName); err != nil {
		return vol, err
	}
	if err := s.Store.UpdateVolumeAttach(ctx, vol.ID, types.VolumeStatusAvailable, "", "", "", vol.Name, ""); err != nil {
		return vol, err
	}
	vol.Status = types.VolumeStatusAvailable
	vol.AttachedInstanceID = ""
	vol.AttachedInstance = ""
	vol.DevicePath = ""
	vol.ErrorMessage = ""
	return vol, nil
}
