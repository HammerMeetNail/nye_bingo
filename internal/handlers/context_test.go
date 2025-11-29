package handlers

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func TestSetUserInContext(t *testing.T) {
	user := &models.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Username: "Test User",
	}

	ctx := context.Background()
	newCtx := SetUserInContext(ctx, user)

	if newCtx == ctx {
		t.Error("SetUserInContext should return new context")
	}
}

func TestGetUserFromContext_WithUser(t *testing.T) {
	user := &models.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Username: "Test User",
	}

	ctx := SetUserInContext(context.Background(), user)
	retrieved := GetUserFromContext(ctx)

	if retrieved == nil {
		t.Fatal("expected user to be retrieved from context")
	}
	if retrieved.ID != user.ID {
		t.Errorf("expected ID %v, got %v", user.ID, retrieved.ID)
	}
	if retrieved.Email != user.Email {
		t.Errorf("expected email %q, got %q", user.Email, retrieved.Email)
	}
	if retrieved.Username != user.Username {
		t.Errorf("expected username %q, got %q", user.Username, retrieved.Username)
	}
}

func TestGetUserFromContext_NoUser(t *testing.T) {
	ctx := context.Background()
	retrieved := GetUserFromContext(ctx)

	if retrieved != nil {
		t.Error("expected nil when no user in context")
	}
}

func TestGetUserFromContext_NilContext(t *testing.T) {
	// This tests handling when context has wrong type
	ctx := context.WithValue(context.Background(), userContextKey, "not a user")
	retrieved := GetUserFromContext(ctx)

	if retrieved != nil {
		t.Error("expected nil when context value is wrong type")
	}
}

func TestContextKey_UniqueType(t *testing.T) {
	// Verify that our context key type is distinct
	user := &models.User{ID: uuid.New()}

	ctx := SetUserInContext(context.Background(), user)

	// Using a string key should not find the user
	retrieved := ctx.Value("user")
	if retrieved != nil {
		t.Error("string key should not find user (type safety)")
	}
}

func TestSetUserInContext_NilUser(t *testing.T) {
	ctx := SetUserInContext(context.Background(), nil)
	retrieved := GetUserFromContext(ctx)

	// Should return nil gracefully
	if retrieved != nil {
		t.Error("expected nil when nil user was set")
	}
}

func TestSetUserInContext_OverwriteUser(t *testing.T) {
	user1 := &models.User{ID: uuid.New(), Email: "user1@test.com"}
	user2 := &models.User{ID: uuid.New(), Email: "user2@test.com"}

	ctx := SetUserInContext(context.Background(), user1)
	ctx = SetUserInContext(ctx, user2)

	retrieved := GetUserFromContext(ctx)
	if retrieved == nil {
		t.Fatal("expected user in context")
	}
	if retrieved.Email != "user2@test.com" {
		t.Error("expected second user to overwrite first")
	}
}
