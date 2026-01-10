package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestAuthService_HashPassword(t *testing.T) {
	auth := &AuthService{}

	password := "securePassword123!"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hash == "" {
		t.Error("hash should not be empty")
	}
	if hash == password {
		t.Error("hash should not equal plain password")
	}
	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Error("hash should be bcrypt format")
	}
}

func TestAuthService_HashPassword_UniqueHashes(t *testing.T) {
	auth := &AuthService{}

	password := "samePassword123"
	hash1, _ := auth.HashPassword(password)
	hash2, _ := auth.HashPassword(password)

	if hash1 == hash2 {
		t.Error("same password should produce different hashes (due to salt)")
	}
}

func TestAuthService_VerifyPassword_Correct(t *testing.T) {
	auth := &AuthService{}

	password := "correctPassword123!"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !auth.VerifyPassword(hash, password) {
		t.Error("correct password should verify successfully")
	}
}

func TestAuthService_VerifyPassword_Incorrect(t *testing.T) {
	auth := &AuthService{}

	password := "correctPassword123!"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if auth.VerifyPassword(hash, "wrongPassword") {
		t.Error("incorrect password should not verify")
	}
}

func TestAuthService_VerifyPassword_EmptyPassword(t *testing.T) {
	auth := &AuthService{}

	hash, _ := auth.HashPassword("somePassword")

	if auth.VerifyPassword(hash, "") {
		t.Error("empty password should not verify")
	}
}

func TestAuthService_VerifyPassword_InvalidHash(t *testing.T) {
	auth := &AuthService{}

	if auth.VerifyPassword("not-a-valid-hash", "password") {
		t.Error("invalid hash should not verify")
	}
}

func TestAuthService_GenerateSessionToken(t *testing.T) {
	auth := &AuthService{}

	token, hash, err := auth.GenerateSessionToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token == "" {
		t.Error("token should not be empty")
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
	if token == hash {
		t.Error("token and hash should be different")
	}
}

func TestAuthService_GenerateSessionToken_Unique(t *testing.T) {
	auth := &AuthService{}

	token1, hash1, _ := auth.GenerateSessionToken()
	token2, hash2, _ := auth.GenerateSessionToken()

	if token1 == token2 {
		t.Error("tokens should be unique")
	}
	if hash1 == hash2 {
		t.Error("hashes should be unique")
	}
}

func TestAuthService_GenerateSessionToken_Length(t *testing.T) {
	auth := &AuthService{}

	token, hash, _ := auth.GenerateSessionToken()

	// Token is 32 bytes hex encoded = 64 chars
	if len(token) != 64 {
		t.Errorf("token should be 64 chars, got %d", len(token))
	}

	// Hash is SHA256 = 32 bytes hex encoded = 64 chars
	if len(hash) != 64 {
		t.Errorf("hash should be 64 chars, got %d", len(hash))
	}
}

func TestAuthService_HashToken(t *testing.T) {
	auth := &AuthService{}

	token := "some-test-token"
	hash1 := auth.hashToken(token)
	hash2 := auth.hashToken(token)

	// Same token should produce same hash
	if hash1 != hash2 {
		t.Error("same token should produce same hash")
	}

	// Different tokens should produce different hashes
	hash3 := auth.hashToken("different-token")
	if hash1 == hash3 {
		t.Error("different tokens should produce different hashes")
	}
}

func TestAuthService_HashToken_ConsistentWithGenerate(t *testing.T) {
	auth := &AuthService{}

	token, generatedHash, _ := auth.GenerateSessionToken()
	computedHash := auth.hashToken(token)

	if generatedHash != computedHash {
		t.Error("hashToken should produce same result as GenerateSessionToken")
	}
}

func TestPasswordComplexity(t *testing.T) {
	auth := &AuthService{}

	// Various password types should all hash successfully
	// Note: bcrypt has a 72 byte limit, so we test up to that
	passwords := []string{
		"simple",
		"WithNumbers123",
		"Special!@#$%^&*()",
		"Unicode日本語",
		"A moderately long password that fits within bcrypt limits", // Under 72 bytes
		" ", // Single space
	}

	for _, pwd := range passwords {
		t.Run(pwd[:min(10, len(pwd))], func(t *testing.T) {
			hash, err := auth.HashPassword(pwd)
			if err != nil {
				t.Errorf("failed to hash password: %v", err)
			}
			if !auth.VerifyPassword(hash, pwd) {
				t.Error("password should verify after hashing")
			}
		})
	}
}

func TestPasswordComplexity_TooLong(t *testing.T) {
	auth := &AuthService{}

	// bcrypt has a 72 byte limit
	longPassword := "This is a very long password that exceeds the seventy-two byte limit of bcrypt"

	_, err := auth.HashPassword(longPassword)
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("expected ErrPasswordTooLong, got %v", err)
	}
}

type fakeRedis struct {
	setErr      error
	getValue    string
	getErr      error
	expireErr   error
	delErr      error
	setCalls    int
	getCalls    int
	expireCalls int
	delCalls    int
}

func (f *fakeRedis) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	f.setCalls++
	return f.setErr
}

func (f *fakeRedis) Get(ctx context.Context, key string) (string, error) {
	f.getCalls++
	return f.getValue, f.getErr
}

func (f *fakeRedis) Expire(ctx context.Context, key string, expiration time.Duration) error {
	f.expireCalls++
	return f.expireErr
}

func (f *fakeRedis) Del(ctx context.Context, keys ...string) error {
	f.delCalls += len(keys)
	return f.delErr
}

func TestAuthService_CreateSession_RedisFailure_FallsBackToDB(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	execCalled := false

	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			execCalled = true
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	redis := &fakeRedis{setErr: errors.New("redis down")}

	auth := NewAuthService(db, redis)
	token, err := auth.CreateSession(ctx, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected token to be returned")
	}
	if !execCalled {
		t.Fatal("expected database fallback when redis set fails")
	}
}

func TestAuthService_ValidateSession_RedisHit(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	now := time.Now().UTC()

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(
				userID,
				"user@example.com",
				"hash",
				"username",
				true,
				nil,
				1,
				true,
				now,
				now,
			)
		},
	}
	redis := &fakeRedis{getValue: userID.String()}

	auth := NewAuthService(db, redis)
	user, err := auth.ValidateSession(ctx, "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != userID {
		t.Fatalf("expected user ID %v, got %v", userID, user.ID)
	}
	if redis.expireCalls != 1 {
		t.Fatalf("expected redis expire call, got %d", redis.expireCalls)
	}
}

func TestAuthService_ValidateSession_FiltersDeletedUsers(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if !strings.Contains(sql, "deleted_at IS NULL") {
				t.Fatalf("expected deleted_at filter in query, got %q", sql)
			}
			return rowFromValues(
				userID,
				"user@example.com",
				"hash",
				"user",
				false,
				nil,
				0,
				true,
				time.Now(),
				time.Now(),
			)
		},
	}

	redis := &fakeRedis{
		getValue: userID.String(),
	}

	auth := NewAuthService(db, redis)
	if _, err := auth.ValidateSession(ctx, "token"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthService_ValidateSession_DBExpired(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()
	expired := time.Now().Add(-2 * time.Hour)
	execCalled := false

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(sessionID, userID, "hash", expired, expired)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			execCalled = true
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	redis := &fakeRedis{getErr: errors.New("miss")}

	auth := NewAuthService(db, redis)
	_, err := auth.ValidateSession(ctx, "token")
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
	if !execCalled {
		t.Fatal("expected expired session cleanup to hit database")
	}
}

func TestAuthService_ValidateSession_DBNotFound(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	redis := &fakeRedis{getErr: errors.New("miss")}

	auth := NewAuthService(db, redis)
	_, err := auth.ValidateSession(ctx, "token")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestAuthService_DeleteAllUserSessions_DeletesRedisAndDB(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	execCalled := false

	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"hash1"}, {"hash2"}}}, nil
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			execCalled = true
			return fakeCommandTag{rowsAffected: 2}, nil
		},
	}
	redis := &fakeRedis{}

	auth := NewAuthService(db, redis)
	if err := auth.DeleteAllUserSessions(ctx, userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if redis.delCalls != 2 {
		t.Fatalf("expected 2 redis deletions, got %d", redis.delCalls)
	}
	if !execCalled {
		t.Fatal("expected database delete for user sessions")
	}
}

func TestAuthService_DeleteAllUserSessions_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("boom")
		},
	}
	redis := &fakeRedis{}

	auth := NewAuthService(db, redis)
	if err := auth.DeleteAllUserSessions(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthService_ValidateSession_RedisInvalidUserID(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{}
	redis := &fakeRedis{getValue: "not-a-uuid"}

	auth := NewAuthService(db, redis)
	_, err := auth.ValidateSession(ctx, "token")
	if err == nil {
		t.Fatal("expected error for invalid user id")
	}
}

func TestAuthService_DeleteSession_DBError(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("db error")
		},
	}
	redis := &fakeRedis{}

	auth := NewAuthService(db, redis)
	err := auth.DeleteSession(ctx, "token")
	if err == nil {
		t.Fatal("expected error on delete session")
	}
}

func TestAuthService_DeleteSession_Success(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	redis := &fakeRedis{}

	auth := NewAuthService(db, redis)
	if err := auth.DeleteSession(ctx, "token"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthService_CreateSession_RedisSuccess(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			t.Fatal("unexpected db fallback")
			return fakeCommandTag{}, nil
		},
	}
	redis := &fakeRedis{}

	auth := NewAuthService(db, redis)
	token, err := auth.CreateSession(ctx, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected token")
	}
	if redis.setCalls != 1 {
		t.Fatalf("expected redis set, got %d", redis.setCalls)
	}
}

func TestAuthService_ValidateSession_DBHit(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()
	expires := time.Now().Add(1 * time.Hour)
	now := time.Now().UTC()
	call := 0

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			call++
			if call == 1 {
				return rowFromValues(sessionID, userID, "hash", expires, now)
			}
			return rowFromValues(
				userID,
				"user@example.com",
				"hash",
				"username",
				true,
				nil,
				0,
				true,
				now,
				now,
			)
		},
	}
	redis := &fakeRedis{getErr: errors.New("miss")}

	auth := NewAuthService(db, redis)
	user, err := auth.ValidateSession(ctx, "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != userID {
		t.Fatalf("expected user ID %v, got %v", userID, user.ID)
	}
}

func TestAuthService_getUserByID_NotFound(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}

	auth := NewAuthService(db, &fakeRedis{})
	_, err := auth.getUserByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
