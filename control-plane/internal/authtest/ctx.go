package authtest

import (
	"context"

	"github.com/ayushanand/tiny-aws/control-plane/internal/auth"
)

func Ctx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:   "test-owner",
		Username: "tester",
		Role:     "admin",
	})
}

func CtxUser(id, name string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:   id,
		Username: name,
		Role:     "user",
	})
}
