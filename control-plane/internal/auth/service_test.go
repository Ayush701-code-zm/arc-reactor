package auth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
)

func TestBootstrapLoginRefresh(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "tiny.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := &Service{Store: st}
	ctx := CtxBackground()

	user, err := svc.Bootstrap(ctx, "admin", "password1")
	if err != nil || user.Role != "admin" {
		t.Fatalf("%+v %v", user, err)
	}
	if _, err := svc.Bootstrap(ctx, "other", "password1"); err != ErrConflict {
		t.Fatalf("second bootstrap: %v", err)
	}
	pair, err := svc.Login(ctx, "admin", "password1")
	if err != nil || pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("%+v %v", pair, err)
	}
	p, err := svc.AuthenticateAccess(ctx, pair.AccessToken)
	if err != nil || p.Username != "admin" {
		t.Fatalf("%+v %v", p, err)
	}
	next, err := svc.Refresh(ctx, pair.RefreshToken)
	if err != nil || next.AccessToken == "" {
		t.Fatalf("%+v %v", next, err)
	}
	if _, err := svc.AuthenticateAccess(ctx, pair.AccessToken); err == nil {
		t.Fatal("old access should be revoked after refresh")
	}
}

func CtxBackground() context.Context {
	return context.Background()
}
