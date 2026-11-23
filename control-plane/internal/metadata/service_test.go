package metadata

import (
	"path/filepath"
	"testing"

	"github.com/ayushanand/tiny-aws/control-plane/internal/authtest"
	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

func TestDocumentRunningWithAttachment(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "tiny.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := &Service{Store: st}
	ctx := authtest.Ctx()

	inst := types.Instance{
		ID: "i1", Name: "web", Image: "alpine", Status: types.StatusRunning,
		CPU: 1, MemoryBytes: 64 << 20, OwnerID: "test-owner",
	}
	if err := st.InsertPending(ctx, inst); err != nil {
		t.Fatal(err)
	}
	_ = st.UpdateLifecycle(ctx, inst.ID, types.StatusRunning, 42, "")
	att := types.NetworkAttachment{
		NetworkID: "n1", InstanceID: "i1", InstanceName: "web",
		GuestIP: "10.50.0.2", GuestIPCIDR: "10.50.0.2/24",
		VethHost: "thaaaaaaaaaaaa", VethGuest: "eth0", Status: "ATTACHED",
	}
	if err := st.InsertAttachment(ctx, att); err != nil {
		t.Fatal(err)
	}

	doc, err := svc.DocumentByName(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if doc.InstanceID != "i1" || doc.LocalIPv4 != "10.50.0.2" || doc.VethHost != "thaaaaaaaaaaaa" {
		t.Fatalf("%+v", doc)
	}
	byVeth, err := svc.DocumentByVeth(ctx, "thaaaaaaaaaaaa")
	if err != nil || byVeth.Name != "web" {
		t.Fatalf("%+v %v", byVeth, err)
	}
	id, err := svc.PathByName(ctx, "web", "instance-id")
	if err != nil || id != "i1" {
		t.Fatalf("%q %v", id, err)
	}
}

func TestDocumentNotRunning(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "tiny.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := &Service{Store: st}
	ctx := authtest.Ctx()
	inst := types.Instance{
		ID: "i2", Name: "stopped", Image: "alpine", Status: types.StatusStopped,
		CPU: 1, MemoryBytes: 1, OwnerID: "test-owner",
	}
	if err := st.InsertPending(ctx, inst); err != nil {
		t.Fatal(err)
	}
	_ = st.UpdateLifecycle(ctx, inst.ID, types.StatusStopped, 0, "")
	if _, err := svc.DocumentByName(ctx, "stopped"); err == nil {
		t.Fatal("expected not running")
	}
}

func TestPathListing(t *testing.T) {
	doc := Build(types.Instance{ID: "x", Name: "n"}, nil)
	val, ok := PathValue(doc, "")
	if !ok || val == "" {
		t.Fatalf("%q %v", val, ok)
	}
}
