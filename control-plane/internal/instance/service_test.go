package instance

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ayushanand/tiny-aws/control-plane/internal/authtest"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

type fakeRuntime struct {
	createFn func(types.Instance) (types.RuntimeView, error)
	stopFn   func(string) (types.RuntimeView, error)
	deleteFn func(string) error
	deleted  []string
}

func (f *fakeRuntime) CreateInstance(_ context.Context, inst types.Instance) (types.RuntimeView, error) {
	if f.createFn != nil {
		return f.createFn(inst)
	}
	return types.RuntimeView{
		ID: inst.ID, Name: inst.Name, Image: inst.Image, CPU: inst.CPU,
		MemoryBytes: inst.MemoryBytes, Status: types.StatusRunning, PID: 99,
	}, nil
}
func (f *fakeRuntime) ListInstances(context.Context) ([]types.RuntimeView, error) {
	return nil, nil
}
func (f *fakeRuntime) InspectInstance(_ context.Context, id string) (types.RuntimeView, error) {
	return types.RuntimeView{ID: id, Status: types.StatusRunning, PID: 99}, nil
}
func (f *fakeRuntime) StartInstance(_ context.Context, id string) (types.RuntimeView, error) {
	return types.RuntimeView{ID: id, Status: types.StatusRunning, PID: 100}, nil
}
func (f *fakeRuntime) StopInstance(_ context.Context, id string) (types.RuntimeView, error) {
	if f.stopFn != nil {
		return f.stopFn(id)
	}
	return types.RuntimeView{ID: id, Status: types.StatusStopped, PID: 0}, nil
}
func (f *fakeRuntime) DeleteInstance(_ context.Context, id string) error {
	if f.deleteFn != nil {
		return f.deleteFn(id)
	}
	f.deleted = append(f.deleted, id)
	return nil
}
func (f *fakeRuntime) AttachVolume(context.Context, string, string, string, string, string) (string, string, error) {
	return "/dev/loop0", "/mnt/vol", nil
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

func newService(t *testing.T, rt *fakeRuntime) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "tiny.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return &Service{Store: st, Runtime: rt}
}

type fakeVolumes struct {
	detached []string
}

func (f *fakeVolumes) DetachAllForInstance(_ context.Context, instanceID string) error {
	f.detached = append(f.detached, instanceID)
	return nil
}

func TestCreateListStopStartDelete(t *testing.T) {
	vols := &fakeVolumes{}
	svc := newService(t, &fakeRuntime{})
	svc.Volumes = vols
	ctx := authtest.Ctx()

	inst, err := svc.Create(ctx, types.Spec{Name: "web", Image: "alpine", CPU: 1, MemoryBytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if inst.Status != types.StatusRunning || inst.PID != 99 {
		t.Fatalf("create=%+v", inst)
	}

	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}

	stopped, err := svc.Stop(ctx, "web")
	if err != nil || stopped.Status != types.StatusStopped {
		t.Fatalf("stop=%+v err=%v", stopped, err)
	}

	started, err := svc.Start(ctx, "web")
	if err != nil || started.Status != types.StatusRunning {
		t.Fatalf("start=%+v err=%v", started, err)
	}

	if err := svc.Delete(ctx, "web"); err != nil {
		t.Fatal(err)
	}
	if len(vols.detached) != 1 || vols.detached[0] != inst.ID {
		t.Fatalf("detach calls=%v", vols.detached)
	}
}

func TestCreateRuntimeFailureMarksFailed(t *testing.T) {
	rt := &fakeRuntime{
		createFn: func(types.Instance) (types.RuntimeView, error) {
			return types.RuntimeView{}, errors.New("boom")
		},
	}
	svc := newService(t, rt)
	inst, err := svc.Create(authtest.Ctx(), types.Spec{
		Name: "bad", Image: "alpine", CPU: 1, MemoryBytes: 1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if inst.Status != types.StatusFailed {
		t.Fatalf("status=%s", inst.Status)
	}
	got, err := svc.Store.GetByName(authtest.Ctx(), "bad")
	if err != nil || got.Status != types.StatusFailed {
		t.Fatalf("db=%+v err=%v", got, err)
	}
}

func TestDeleteKeepsRowWhenRuntimeFails(t *testing.T) {
	rt := &fakeRuntime{
		deleteFn: func(string) error { return errors.New("still running") },
	}
	svc := newService(t, rt)
	ctx := authtest.Ctx()
	if _, err := svc.Create(ctx, types.Spec{Name: "keep", Image: "alpine", CPU: 1, MemoryBytes: 1}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, "keep"); err == nil {
		t.Fatal("expected delete error")
	}
	if _, err := svc.Store.GetByName(ctx, "keep"); err != nil {
		t.Fatalf("row should remain: %v", err)
	}
}

func TestStopRequiresStoppedConfirmation(t *testing.T) {
	rt := &fakeRuntime{
		stopFn: func(id string) (types.RuntimeView, error) {
			return types.RuntimeView{ID: id, Status: types.StatusRunning, PID: 7}, nil
		},
	}
	svc := newService(t, rt)
	ctx := authtest.Ctx()
	if _, err := svc.Create(ctx, types.Spec{Name: "sticky", Image: "alpine", CPU: 1, MemoryBytes: 1}); err != nil {
		t.Fatal(err)
	}
	inst, err := svc.Stop(ctx, "sticky")
	if err == nil {
		t.Fatal("expected stop error")
	}
	if inst.Status == types.StatusStopped {
		t.Fatal("must not persist STOPPED without confirmation")
	}
	got, err := svc.Store.GetByName(ctx, "sticky")
	if err != nil || got.Status != types.StatusFailed {
		t.Fatalf("db=%+v err=%v", got, err)
	}
}

func TestValidateSpec(t *testing.T) {
	svc := newService(t, &fakeRuntime{})
	_, err := svc.Create(authtest.Ctx(), types.Spec{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
