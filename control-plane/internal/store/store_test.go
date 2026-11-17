package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
)

func TestInsertListInspectDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tiny.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	inst := types.Instance{
		ID:          "id-1",
		Name:        "web",
		Image:       "alpine",
		CPU:         1,
		MemoryBytes: 64 << 20,
		Status:      types.StatusPending,
	}
	if err := s.InsertPending(ctx, inst); err != nil {
		t.Fatal(err)
	}

	list, err := s.List(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "web" {
		t.Fatalf("list=%v err=%v", list, err)
	}

	if err := s.UpdateLifecycle(ctx, inst.ID, types.StatusRunning, 42, ""); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetByName(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != types.StatusRunning || got.PID != 42 {
		t.Fatalf("got=%+v", got)
	}

	if err := s.Delete(ctx, inst.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetByID(ctx, inst.ID); err == nil {
		t.Fatal("expected not found")
	}
}

func TestDuplicateName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tiny.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	inst := types.Instance{
		ID: "a", Name: "dup", Image: "alpine", CPU: 1, MemoryBytes: 1, Status: types.StatusPending,
	}
	if err := s.InsertPending(ctx, inst); err != nil {
		t.Fatal(err)
	}
	inst.ID = "b"
	if err := s.InsertPending(ctx, inst); err == nil {
		t.Fatal("expected unique name error")
	}
}
