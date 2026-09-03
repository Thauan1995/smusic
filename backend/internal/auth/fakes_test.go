package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"smusic/backend/internal/auth/oauth"
)

// --- fakeUserRepo ---

type fakeUserRepo struct {
	mu            sync.Mutex
	byEmail       map[string]User
	byID          map[string]User
	createErr     error
	getByEmailErr error
	getByIDErr    error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byEmail: map[string]User{}, byID: map[string]User{}}
}

func (f *fakeUserRepo) Create(ctx context.Context, u User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	if _, ok := f.byEmail[u.Email]; ok {
		return ErrEmailTaken
	}
	f.byEmail[u.Email] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeUserRepo) GetByEmail(ctx context.Context, email string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getByEmailErr != nil {
		return User{}, f.getByEmailErr
	}
	u, ok := f.byEmail[email]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) GetByID(ctx context.Context, id string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getByIDErr != nil {
		return User{}, f.getByIDErr
	}
	u, ok := f.byID[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

// --- fakeIdentityRepo ---

type fakeIdentityRepo struct {
	mu      sync.Mutex
	byKey   map[string]string
	getErr  error
	linkErr error
}

func newFakeIdentityRepo() *fakeIdentityRepo {
	return &fakeIdentityRepo{byKey: map[string]string{}}
}

func (f *fakeIdentityRepo) GetUserIDByProvider(ctx context.Context, provider, providerUserID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return "", f.getErr
	}
	id, ok := f.byKey[provider+"|"+providerUserID]
	if !ok {
		return "", ErrIdentityNotLinked
	}
	return id, nil
}

func (f *fakeIdentityRepo) Link(ctx context.Context, identity Identity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.linkErr != nil {
		return f.linkErr
	}
	f.byKey[identity.Provider+"|"+identity.ProviderUserID] = identity.UserID
	return nil
}

// --- fakeDeviceRepo ---

type fakeDeviceRepo struct {
	mu        sync.Mutex
	n         int
	upsertErr error
	last      Device
}

func (f *fakeDeviceRepo) Upsert(ctx context.Context, d Device) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.upsertErr != nil {
		return "", f.upsertErr
	}
	f.n++
	f.last = d
	return fmt.Sprintf("device-%d", f.n), nil
}

// --- fakeRefreshTokenRepo ---

type fakeRefreshTokenRepo struct {
	mu              sync.Mutex
	byID            map[string]RefreshToken
	hashToID        map[string]string
	storeErr        error
	getByHashErr    error
	revokeErr       error
	markReplacedErr error
	revokeAllErr    error
}

func newFakeRefreshTokenRepo() *fakeRefreshTokenRepo {
	return &fakeRefreshTokenRepo{byID: map[string]RefreshToken{}, hashToID: map[string]string{}}
}

func (f *fakeRefreshTokenRepo) Store(ctx context.Context, rt RefreshToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.storeErr != nil {
		return f.storeErr
	}
	f.byID[rt.ID] = rt
	f.hashToID[rt.TokenHash] = rt.ID
	return nil
}

func (f *fakeRefreshTokenRepo) GetByHash(ctx context.Context, tokenHash string) (RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getByHashErr != nil {
		return RefreshToken{}, f.getByHashErr
	}
	id, ok := f.hashToID[tokenHash]
	if !ok {
		return RefreshToken{}, ErrInvalidRefreshToken
	}
	return f.byID[id], nil
}

func (f *fakeRefreshTokenRepo) Revoke(ctx context.Context, id string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.revokeErr != nil {
		return f.revokeErr
	}
	rt, ok := f.byID[id]
	if !ok || rt.RevokedAt != nil {
		return nil
	}
	t := at
	rt.RevokedAt = &t
	f.byID[id] = rt
	return nil
}

func (f *fakeRefreshTokenRepo) MarkReplaced(ctx context.Context, oldID, newID string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markReplacedErr != nil {
		return f.markReplacedErr
	}
	rt, ok := f.byID[oldID]
	if !ok {
		return nil
	}
	rt.ReplacedBy = newID
	t := at
	rt.RevokedAt = &t
	f.byID[oldID] = rt
	return nil
}

func (f *fakeRefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.revokeAllErr != nil {
		return f.revokeAllErr
	}
	for id, rt := range f.byID {
		if rt.UserID == userID && rt.RevokedAt == nil {
			t := at
			rt.RevokedAt = &t
			f.byID[id] = rt
		}
	}
	return nil
}

// --- fakeHasher ---

type fakeHasher struct {
	hashErr   error
	verifyErr error
	verifyOK  bool
}

func (f *fakeHasher) Hash(passwordPlain string) (string, error) {
	if f.hashErr != nil {
		return "", f.hashErr
	}
	return "hashed:" + passwordPlain, nil
}

func (f *fakeHasher) Verify(passwordPlain, encoded string) (bool, error) {
	if f.verifyErr != nil {
		return false, f.verifyErr
	}
	return f.verifyOK, nil
}

// --- fakeSigner ---

type fakeSigner struct {
	mu      sync.Mutex
	clock   interface{ Now() time.Time }
	ttl     time.Duration
	signErr error
	calls   int
}

func (f *fakeSigner) Sign(userID string) (string, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.signErr != nil {
		return "", time.Time{}, f.signErr
	}
	f.calls++
	return fmt.Sprintf("access-token-%d-for-%s", f.calls, userID), f.clock.Now().Add(f.ttl), nil
}

// --- fakeRefreshGen ---

type fakeRefreshGen struct {
	mu     sync.Mutex
	n      int
	newErr error
}

func (f *fakeRefreshGen) New() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.newErr != nil {
		return "", f.newErr
	}
	f.n++
	return fmt.Sprintf("refresh-plain-%d", f.n), nil
}

// --- fakeOAuthVerifier ---

type fakeOAuthVerifier struct {
	subject string
	email   string
	err     error
}

func (f *fakeOAuthVerifier) Verify(ctx context.Context, provider oauth.Provider, idToken string) (string, string, error) {
	if f.err != nil {
		return "", "", f.err
	}
	return f.subject, f.email, nil
}
