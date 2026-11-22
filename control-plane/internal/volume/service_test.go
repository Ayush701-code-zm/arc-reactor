package volume

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

type fakeRuntime struct{}

func (f *fakeRuntime) CreateInstance(context.Context, types.Instance) (types.RuntimeView, error) {
	return types.RuntimeView{}, nil
}
func (f *fakeRuntime) ListInstances(context.Context) ([]types.RuntimeView, error) { return nil, nil }
func (f *fakeRuntime) InspectInstance(context.Context, string) (types.RuntimeView, error) {
	return types.RuntimeView{}, nil
}
func (f *fakeRuntime) StartInstance(context.Context, string) (types.RuntimeView, error) {
	return types.RuntimeView{}, nil
}
func (f *fakeRuntime) StopInstance(context.Context, string) (types.RuntimeView, error) {
	return types.RuntimeView{}, nil
}
func (f *fakeRuntime) DeleteInstance(context.Context, string) error { return nil }
func (f *fakeRuntime) AttachVolume(context.Context, string, string, string, string, string) (string, string, error) {
	return "/dev/loop7", "/tmp/rootfs/mnt/data", nil
}
func (f *fakeRuntime) DetachVolume(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *fakeRuntime) AttachNetwork(context.Context, string, string, string, string, string, string, string) error {
	return nil
}
func (f *fakeRuntime) DetachNetwork(context.Context, string, string, string, string, string, bool) error {
	return nil
}
func (f *fakeRuntime) Close() error { return nil }

type fakeImages struct {
	rootfs string
}

func (f *fakeImages) ResolveRootfs(context.Context, string) (types.Image, error) {
	return types.Image{Name: "alpine", RootfsPath: f.rootfs, Status: types.ImageStatusReady}, nil
}

func TestVolumeCreateListDelete(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "tiny.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	svc := &Service{Store: st, Runtime: &fakeRuntime{}, VolRoot: filepath.Join(dir, "vols")}
	ctx := context.Background()
	vol, err := svc.Create(ctx, "data", 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	if vol.Status != types.VolumeStatusAvailable {
		t.Fatalf("status=%s", vol.Status)
	}
	fi, err := os.Stat(vol.DiskPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != 8<<20 {
		t.Fatalf("size=%d", fi.Size())
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if err := svc.Delete(ctx, "data"); err != nil {
		t.Fatal(err)
	}
}

func TestVolumeAttachDetach(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "tiny.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	rootfs := filepath.Join(dir, "rootfs")
	_ = os.MkdirAll(rootfs, 0o755)

	ctx := context.Background()
	inst := types.Instance{
		ID: "inst-1", Name: "web", Image: "alpine", Status: types.StatusRunning, CPU: 1, MemoryBytes: 64 << 20,
	}
	if err := st.InsertPending(ctx, inst); err != nil {
		t.Fatal(err)
	}
	_ = st.UpdateLifecycle(ctx, inst.ID, types.StatusRunning, 42, "")

	svc := &Service{
		Store:   st,
		Runtime: &fakeRuntime{},
		Images:  &fakeImages{rootfs: rootfs},
		VolRoot: filepath.Join(dir, "vols"),
	}
	vol, err := svc.Create(ctx, "data", 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	attached, err := svc.Attach(ctx, "data", "web")
	if err != nil {
		t.Fatal(err)
	}
	if attached.Status != types.VolumeStatusAttached || attached.DevicePath != "/dev/loop7" {
		t.Fatalf("%+v", attached)
	}
	detached, err := svc.Detach(ctx, "data")
	if err != nil {
		t.Fatal(err)
	}
	if detached.Status != types.VolumeStatusAvailable || detached.DevicePath != "" {
		t.Fatalf("%+v", detached)
	}
	_ = vol
}
