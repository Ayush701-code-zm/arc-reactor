package auth

import "context"

type ctxKey int

const userKey ctxKey = 1

type Principal struct {
	UserID   string
	Username string
	Role     string
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, userKey, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(userKey).(Principal)
	return p, ok
}

func RequirePrincipal(ctx context.Context) (Principal, error) {
	p, ok := PrincipalFrom(ctx)
	if !ok || p.UserID == "" {
		return Principal{}, ErrUnauthorized
	}
	return p, nil
}
