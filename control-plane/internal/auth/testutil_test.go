package auth

import (
	"context"
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("secret-pass")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, "secret-pass") {
		t.Fatal("verify failed")
	}
	if VerifyPassword(h, "wrong-pass!") {
		t.Fatal("wrong password accepted")
	}
}

func TestOpaqueToken(t *testing.T) {
	a, err := NewOpaqueToken()
	if err != nil || a == "" {
		t.Fatalf("%v %q", err, a)
	}
	if HashToken(a) == a {
		t.Fatal("hash should differ from token")
	}
}

func TestPrincipalContext(t *testing.T) {
	ctx := WithPrincipal(context.Background(), Principal{UserID: "u1", Username: "admin", Role: "admin"})
	p, err := RequirePrincipal(ctx)
	if err != nil || p.UserID != "u1" {
		t.Fatalf("%+v %v", p, err)
	}
}
