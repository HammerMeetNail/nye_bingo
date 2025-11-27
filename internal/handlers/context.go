package handlers

import (
	"context"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

type contextKey string

const userContextKey contextKey = "user"

func SetUserInContext(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func GetUserFromContext(ctx context.Context) *models.User {
	user, _ := ctx.Value(userContextKey).(*models.User)
	return user
}
