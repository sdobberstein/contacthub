// Package local provides username/password authentication backed by the user store.
package local

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"github.com/sdobberstein/contacthub/internal/auth"
	"github.com/sdobberstein/contacthub/internal/store"
)

// Argon2id parameters. These are intentionally conservative.
const (
	argonMemory   = 65536 // 64 MiB
	argonTimeCost = 1
	argonThreads  = 4
	argonKeyLen   = 32
	argonSaltLen  = 16
)

// Provider implements auth.Provider using the local user store.
type Provider struct {
	users store.UserStore
}

// New returns a Provider backed by users.
func New(users store.UserStore) *Provider {
	return &Provider{users: users}
}

// Authenticate verifies username and password. Returns the user on success,
// auth.ErrInvalidCredentials on bad credentials.
func (p *Provider) Authenticate(ctx context.Context, username, password string) (*store.User, error) {
	u, err := p.users.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Perform a dummy verify to resist timing attacks that reveal
			// whether a username exists.
			_ = verifyArgon2id("$argon2id$v=19$m=65536,t=1,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", password) //nolint:errcheck // dummy verify, result intentionally ignored
			return nil, auth.ErrInvalidCredentials
		}
		return nil, err
	}

	if !verifyArgon2id(u.PasswordHash, password) {
		return nil, auth.ErrInvalidCredentials
	}
	return u, nil
}

// CreateUser hashes password and persists a new user via the provided UserStore.
// It is the single place that owns the "hash + build + insert" logic.
func CreateUser(ctx context.Context, st store.UserStore, username, displayName, password string, isAdmin bool) (*store.User, error) {
	if displayName == "" {
		displayName = username
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	now := time.Now()
	u := &store.User{
		ID:           uuid.NewString(),
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: hash,
		IsAdmin:      isAdmin,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := st.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// HashPassword hashes plain with Argon2id and returns the encoded hash string.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(plain), salt, argonTimeCost, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory, argonTimeCost, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// verifyArgon2id checks plain against the encoded Argon2id hash string.
// Returns false on any parse or verification failure.
func verifyArgon2id(encoded, plain string) bool {
	// format: $argon2id$v=<v>$m=<m>,t=<t>,p=<p>$<salt_b64>$<hash_b64>
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}

	var memory, timeCost, threads uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false
	}
	if threads > 255 {
		return false // guard before narrowing conversion below
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	storedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	keyLen := uint32(len(storedHash)) // #nosec G115 -- len is non-negative and fits in uint32 for any realistic key
	computed := argon2.IDKey([]byte(plain), salt, timeCost, memory, uint8(threads), keyLen) //nolint:gosec // threads bounded to ≤255 above; #nosec G115
	return subtle.ConstantTimeCompare(computed, storedHash) == 1
}
