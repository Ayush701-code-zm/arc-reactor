package image

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ayushanand/tiny-aws/control-plane/internal/auth"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
	"github.com/google/uuid"
)

type NodePlacer interface {
	PlaceImage(ctx context.Context, nodeID, imageID, artifactPath, rootfsSource string) error
}

type Service struct {
	Store     *store.Store
	ImageRoot string
	Nodes     NodePlacer
}

func DefaultRoot() string {
	if v := os.Getenv("TINY_IMAGE_ROOT"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/tiny-aws/images"
	}
	return filepath.Join(home, ".tiny-aws", "images")
}

func (s *Service) root() string {
	if s.ImageRoot != "" {
		return s.ImageRoot
	}
	return DefaultRoot()
}

func (s *Service) Import(ctx context.Context, name, filePath string) (types.Image, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return types.Image{}, fmt.Errorf("name is required")
	}
	if filePath == "" {
		return types.Image{}, fmt.Errorf("file is required")
	}
	if !strings.HasSuffix(strings.ToLower(filePath), ".tar.gz") && !strings.HasSuffix(strings.ToLower(filePath), ".tgz") {
		return types.Image{}, fmt.Errorf("only .tar.gz images are supported")
	}
	ownerID, err := auth.OwnerID(ctx)
	if err != nil {
		return types.Image{}, err
	}
	if _, err := s.Store.GetImageByName(ctx, name); err == nil {
		return types.Image{}, fmt.Errorf("image name %q already exists", name)
	}

	id := uuid.NewString()
	base := filepath.Join(s.root(), id)
	artifact := filepath.Join(base, "artifact")
	rootfs := filepath.Join(base, "rootfs")
	if err := os.MkdirAll(artifact, 0o755); err != nil {
		return types.Image{}, err
	}
	if err := os.MkdirAll(rootfs, 0o755); err != nil {
		return types.Image{}, err
	}

	destArtifact := filepath.Join(artifact, "image.tar.gz")
	if err := copyFile(filePath, destArtifact); err != nil {
		_ = os.RemoveAll(base)
		return types.Image{}, err
	}

	img := types.Image{
		ID:           id,
		Name:         name,
		ArtifactPath: destArtifact,
		RootfsPath:   rootfs,
		Status:       types.ImageStatusReady,
		OwnerID:      ownerID,
	}
	if err := ExtractTarGz(destArtifact, rootfs); err != nil {
		img.Status = types.ImageStatusFailed
		img.ErrorMessage = err.Error()
		_ = s.Store.InsertImage(ctx, img)
		return img, fmt.Errorf("extract: %w", err)
	}
	if err := s.Store.InsertImage(ctx, img); err != nil {
		_ = os.RemoveAll(base)
		return types.Image{}, err
	}
	return img, nil
}

func (s *Service) List(ctx context.Context) ([]types.Image, error) {
	ownerID, err := auth.OwnerID(ctx)
	if err != nil {
		return nil, err
	}
	all, err := s.Store.ListImages(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]types.Image, 0, len(all))
	for _, img := range all {
		if img.OwnerID == ownerID {
			out = append(out, img)
		}
	}
	return out, nil
}

func (s *Service) Inspect(ctx context.Context, name string) (types.Image, error) {
	img, err := s.Store.GetImageByName(ctx, name)
	if err != nil {
		return types.Image{}, err
	}
	if err := auth.EnsureOwner(ctx, img.OwnerID); err != nil {
		return types.Image{}, err
	}
	return img, nil
}

func (s *Service) Delete(ctx context.Context, name string) error {
	img, err := s.Store.GetImageByName(ctx, name)
	if err != nil {
		return err
	}
	if err := auth.EnsureOwner(ctx, img.OwnerID); err != nil {
		return err
	}
	n, err := s.Store.CountInstancesByImage(ctx, name)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("image %q is used by %d instance(s)", name, n)
	}
	base := filepath.Join(s.root(), img.ID)
	_ = os.RemoveAll(base)
	return s.Store.DeleteImage(ctx, img.ID)
}

func (s *Service) ResolveRootfs(ctx context.Context, name string) (types.Image, error) {
	img, err := s.Store.GetImageByName(ctx, name)
	if err != nil {
		return types.Image{}, err
	}
	if err := auth.EnsureOwner(ctx, img.OwnerID); err != nil {
		return types.Image{}, err
	}
	if img.Status != types.ImageStatusReady {
		return img, fmt.Errorf("image %q is not READY", name)
	}
	if _, err := os.Stat(img.RootfsPath); err != nil {
		return img, fmt.Errorf("rootfs missing for image %q: %w", name, err)
	}
	return img, nil
}

func (s *Service) PlaceOnNode(ctx context.Context, name, nodeID string) (types.NodeImage, error) {
	name = strings.TrimSpace(name)
	nodeID = strings.TrimSpace(nodeID)
	if name == "" {
		return types.NodeImage{}, fmt.Errorf("image name is required")
	}
	if nodeID == "" {
		return types.NodeImage{}, fmt.Errorf("node id is required")
	}
	if s.Nodes == nil {
		return types.NodeImage{}, fmt.Errorf("node placer is not configured")
	}
	img, err := s.Inspect(ctx, name)
	if err != nil {
		return types.NodeImage{}, err
	}
	if img.Status != types.ImageStatusReady {
		return types.NodeImage{}, fmt.Errorf("image %q is not READY", name)
	}
	artifact := img.ArtifactPath
	rootfsSource := ""
	if artifact == "" {
		rootfsSource = img.RootfsPath
	}
	if artifact == "" && rootfsSource == "" {
		return types.NodeImage{}, fmt.Errorf("image %q has no artifact_path or rootfs_path", name)
	}
	if err := s.Nodes.PlaceImage(ctx, nodeID, img.ID, artifact, rootfsSource); err != nil {
		ni := types.NodeImage{
			NodeID:       nodeID,
			ImageID:      img.ID,
			Status:       types.NodeImageFailed,
			ErrorMessage: err.Error(),
		}
		_ = s.Store.UpsertNodeImage(ctx, ni)
		return ni, fmt.Errorf("place image %s on node %s: %w", img.ID, nodeID, err)
	}
	rootfsLocal := filepath.Join(img.ID, "rootfs")
	if node, err := s.Store.GetNode(ctx, nodeID); err == nil && node.ImageRoot != "" {
		rootfsLocal = filepath.Join(node.ImageRoot, img.ID, "rootfs")
	}
	ni := types.NodeImage{
		NodeID:     nodeID,
		ImageID:    img.ID,
		Status:     types.NodeImageReady,
		RootfsPath: rootfsLocal,
	}
	if err := s.Store.UpsertNodeImage(ctx, ni); err != nil {
		return types.NodeImage{}, err
	}
	return ni, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
