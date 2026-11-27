package scheduler

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
)

type fakeHub struct {
	ids []string
}

func (f *fakeHub) ConnectedIDs() []string { return f.ids }

func TestPlaceFirstFit(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	_ = st.UpsertNode(ctx, "n1", "h1", "", 1, 64*1024*1024, 0, true)
	_ = st.UpsertNode(ctx, "n2", "h2", "", 4, 512*1024*1024, 0, true)
	svc := &Service{Store: st, Hub: &fakeHub{ids: []string{"n1", "n2"}}}

	id, err := svc.Place(ctx, 2, 128*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if id != "n2" {
		t.Fatalf("got %s want n2", id)
	}
}

func TestPlaceHardFail(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	_ = st.UpsertNode(ctx, "n1", "h1", "", 1, 64*1024*1024, 0, true)
	svc := &Service{Store: st, Hub: &fakeHub{ids: []string{"n1"}}}
	_, err = svc.Place(ctx, 2, 128*1024*1024)
	if err == nil {
		t.Fatal("expected hard fail")
	}
}
