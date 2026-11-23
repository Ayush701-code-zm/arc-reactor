package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ayushanand/tiny-aws/control-plane/internal/store"
	"github.com/ayushanand/tiny-aws/control-plane/internal/types"
	"github.com/google/uuid"
)

const (
	AccessTTL  = 15 * time.Minute
	RefreshTTL = 7 * 24 * time.Hour
)

type Service struct {
	Store *store.Store
}

type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	ExpiresInSeconds int64
	TokenType        string
	User             types.User
}

func (s *Service) Bootstrap(ctx context.Context, username, password string) (types.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return types.User{}, fmt.Errorf("username is required")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return types.User{}, err
	}
	user := types.User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: hash,
		Role:         types.RoleAdmin,
	}
	created, err := s.Store.BootstrapUser(ctx, user)
	if err != nil {
		if strings.Contains(err.Error(), "conflict") {
			return types.User{}, ErrConflict
		}
		return types.User{}, err
	}
	_ = s.Store.AdoptUnownedResources(ctx, created.ID)
	created.PasswordHash = ""
	return created, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (TokenPair, error) {
	user, err := s.Store.GetUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		return TokenPair{}, ErrInvalidCreds
	}
	if !VerifyPassword(user.PasswordHash, password) {
		return TokenPair{}, ErrInvalidCreds
	}
	return s.issuePair(ctx, user)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	hash := HashToken(refreshToken)
	sess, err := s.Store.GetRefreshSession(ctx, hash)
	if err != nil {
		return TokenPair{}, ErrUnauthorized
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.Store.DeleteRefreshSession(ctx, hash)
		return TokenPair{}, ErrUnauthorized
	}
	user, err := s.Store.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return TokenPair{}, ErrUnauthorized
	}
	_ = s.Store.DeleteRefreshSession(ctx, hash)
	_ = s.Store.DeleteAccessByUser(ctx, user.ID)
	return s.issuePair(ctx, user)
}

func (s *Service) Logout(ctx context.Context, accessToken, refreshToken string) error {
	if accessToken != "" {
		_ = s.Store.DeleteAccessSession(ctx, HashToken(accessToken))
	}
	if refreshToken != "" {
		_ = s.Store.DeleteRefreshSession(ctx, HashToken(refreshToken))
	}
	return nil
}

func (s *Service) WhoAmI(ctx context.Context) (types.User, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return types.User{}, err
	}
	user, err := s.Store.GetUserByID(ctx, p.UserID)
	if err != nil {
		return types.User{}, err
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *Service) AuthenticateAccess(ctx context.Context, accessToken string) (Principal, error) {
	hash := HashToken(accessToken)
	sess, err := s.Store.GetAccessSession(ctx, hash)
	if err != nil {
		return Principal{}, ErrUnauthorized
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		_ = s.Store.DeleteAccessSession(ctx, hash)
		return Principal{}, ErrUnauthorized
	}
	user, err := s.Store.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return Principal{}, ErrUnauthorized
	}
	return Principal{UserID: user.ID, Username: user.Username, Role: user.Role}, nil
}

func (s *Service) MaybeSeed(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil
	}
	n, err := s.Store.CountUsers(ctx)
	if err != nil || n > 0 {
		return err
	}
	_, err = s.Bootstrap(ctx, username, password)
	if err == ErrConflict {
		return nil
	}
	return err
}

func (s *Service) issuePair(ctx context.Context, user types.User) (TokenPair, error) {
	access, err := NewOpaqueToken()
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := NewOpaqueToken()
	if err != nil {
		return TokenPair{}, err
	}
	now := time.Now().UTC()
	if err := s.Store.InsertAccessSession(ctx, types.AccessSession{
		TokenHash: HashToken(access),
		UserID:    user.ID,
		ExpiresAt: now.Add(AccessTTL),
	}); err != nil {
		return TokenPair{}, err
	}
	if err := s.Store.InsertRefreshSession(ctx, types.RefreshSession{
		TokenHash: HashToken(refresh),
		UserID:    user.ID,
		ExpiresAt: now.Add(RefreshTTL),
	}); err != nil {
		return TokenPair{}, err
	}
	user.PasswordHash = ""
	return TokenPair{
		AccessToken:      access,
		RefreshToken:     refresh,
		ExpiresInSeconds: int64(AccessTTL.Seconds()),
		TokenType:        "Bearer",
		User:             user,
	}, nil
}

func EnsureOwner(ctx context.Context, ownerID string) error {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	if ownerID == "" || ownerID != p.UserID {
		return ErrForbidden
	}
	return nil
}
