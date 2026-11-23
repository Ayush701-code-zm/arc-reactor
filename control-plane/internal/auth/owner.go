package auth

import "context"

func OwnerID(ctx context.Context) (string, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return "", err
	}
	return p.UserID, nil
}
