package network

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
	_ "modernc.org/sqlite"
)

func openSQLite(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}

type fakeRuntime struct {
	mu       sync.Mutex
	attaches int
	detaches int
	failDB   bool
}

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
	return "", "", nil
}
func (f *fakeRuntime) DetachVolume(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *fakeRuntime) AttachNetwork(context.Context, string, string, string, string, string, string, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attaches++
	return nil
}
func (f *fakeRuntime) DetachNetwork(context.Context, string, string, string, string, string, bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.detaches++
	return nil
}
func (f *fakeRuntime) Close() error { return nil }

func seedRunning(t *testing.T, st *store.Store, id, name string) {
	t.Helper()
	ctx := context.Background()
	inst := types.Instance{
		ID: id, Name: name, Image: "alpine", Status: types.StatusPending, CPU: 1, MemoryBytes: 64 << 20,
	}
	if err := st.InsertPending(ctx, inst); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateLifecycle(ctx, id, types.StatusRunning, 42, ""); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkCreateListDelete(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "tiny.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := &Service{Store: st, Runtime: &fakeRuntime{}}
	ctx := context.Background()
	net, err := svc.Create(ctx, "priv", "10.20.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if net.GatewayIP != "10.20.0.1" || net.BridgeName == "" || net.Status != types.NetworkStatusReady {
		t.Fatalf("%+v", net)
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if err := svc.Delete(ctx, "priv"); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkAttachDetach(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "tiny.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	rt := &fakeRuntime{}
	svc := &Service{Store: st, Runtime: rt}
	ctx := context.Background()
	seedRunning(t, st, "inst-1", "web")
	if _, err := svc.Create(ctx, "priv", "10.30.0.0/24"); err != nil {
		t.Fatal(err)
	}
	att, err := svc.Attach(ctx, "priv", "web")
	if err != nil {
		t.Fatal(err)
	}
	if att.GuestIP != "10.30.0.2" || att.VethGuest != "eth0" || att.Status != "ATTACHED" {
		t.Fatalf("%+v", att)
	}
	if rt.attaches != 1 {
		t.Fatalf("attaches=%d", rt.attaches)
	}
	if err := svc.Detach(ctx, "priv", "web"); err != nil {
		t.Fatal(err)
	}
	if rt.detaches != 1 {
		t.Fatalf("detaches=%d", rt.detaches)
	}
}

func TestNetworkAttachCompensatesOnDBFailure(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tiny.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	rt := &fakeRuntime{}
	svc := &Service{Store: st, Runtime: rt}
	ctx := context.Background()
	seedRunning(t, st, "inst-1", "web")
	if _, err := svc.Create(ctx, "priv", "10.40.0.0/24"); err != nil {
		t.Fatal(err)
	}
	raw, err := openSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`CREATE TRIGGER fail_att BEFORE INSERT ON network_attachments
BEGIN SELECT RAISE(ABORT, 'forced insert failure'); END;`); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Attach(ctx, "priv", "web")
	if err == nil {
		t.Fatal("expected attach error")
	}
	if rt.attaches != 1 {
		t.Fatalf("attaches=%d", rt.attaches)
	}
	if rt.detaches != 1 {
		t.Fatalf("compensate detaches=%d want 1", rt.detaches)
	}
}
