package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

// UserServiceInterface defines the contract for user operations.
type UserServiceInterface interface {
	Create(ctx context.Context, params models.CreateUserParams) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	UpdatePassword(ctx context.Context, userID uuid.UUID, newPasswordHash string) error
	MarkEmailVerified(ctx context.Context, userID uuid.UUID) error
	UpdateSearchable(ctx context.Context, userID uuid.UUID, searchable bool) error
}

// AuthServiceInterface defines the contract for authentication operations.
type AuthServiceInterface interface {
	HashPassword(password string) (string, error)
	VerifyPassword(hash, password string) bool
	GenerateSessionToken() (token string, hash string, err error)
	CreateSession(ctx context.Context, userID uuid.UUID) (token string, err error)
	ValidateSession(ctx context.Context, token string) (*models.User, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteAllUserSessions(ctx context.Context, userID uuid.UUID) error
}

// CardServiceInterface defines the contract for bingo card operations used by handlers.
type CardServiceInterface interface {
	CheckForConflict(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error)
	Create(ctx context.Context, params models.CreateCardParams) (*models.BingoCard, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error)
	GetByID(ctx context.Context, cardID uuid.UUID) (*models.BingoCard, error)
	Delete(ctx context.Context, userID, cardID uuid.UUID) error
	AddItem(ctx context.Context, userID uuid.UUID, params models.AddItemParams) (*models.BingoItem, error)
	UpdateConfig(ctx context.Context, userID, cardID uuid.UUID, params models.UpdateCardConfigParams) (*models.BingoCard, error)
	Clone(ctx context.Context, userID, cardID uuid.UUID, params CloneParams) (*CloneResult, error)
	UpdateItem(ctx context.Context, userID, cardID uuid.UUID, position int, params models.UpdateItemParams) (*models.BingoItem, error)
	RemoveItem(ctx context.Context, userID, cardID uuid.UUID, position int) error
	Shuffle(ctx context.Context, userID, cardID uuid.UUID) (*models.BingoCard, error)
	SwapItems(ctx context.Context, userID, cardID uuid.UUID, pos1, pos2 int) error
	Finalize(ctx context.Context, userID, cardID uuid.UUID, params *FinalizeParams) (*models.BingoCard, error)
	CompleteItem(ctx context.Context, userID, cardID uuid.UUID, position int, params models.CompleteItemParams) (*models.BingoItem, error)
	UncompleteItem(ctx context.Context, userID, cardID uuid.UUID, position int) (*models.BingoItem, error)
	UpdateItemNotes(ctx context.Context, userID, cardID uuid.UUID, position int, notes, proofURL *string) (*models.BingoItem, error)
	GetArchive(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error)
	GetStats(ctx context.Context, userID, cardID uuid.UUID) (*models.CardStats, error)
	UpdateMeta(ctx context.Context, userID, cardID uuid.UUID, params models.UpdateCardMetaParams) (*models.BingoCard, error)
	UpdateVisibility(ctx context.Context, userID, cardID uuid.UUID, visibleToFriends bool) (*models.BingoCard, error)
	BulkUpdateVisibility(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, visibleToFriends bool) (int, error)
	BulkDelete(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID) (int, error)
	BulkUpdateArchive(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, isArchived bool) (int, error)
	Import(ctx context.Context, params models.ImportCardParams) (*models.BingoCard, error)
}

// SuggestionServiceInterface defines the contract for suggestion operations.
type SuggestionServiceInterface interface {
	GetAll(ctx context.Context) ([]*models.Suggestion, error)
	GetByCategory(ctx context.Context, category string) ([]*models.Suggestion, error)
	GetCategories(ctx context.Context) ([]string, error)
	GetGroupedByCategory(ctx context.Context) ([]SuggestionsByCategory, error)
}

// FriendServiceInterface defines the contract for friendship operations.
type FriendServiceInterface interface {
	SearchUsers(ctx context.Context, currentUserID uuid.UUID, query string) ([]models.UserSearchResult, error)
	SendRequest(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error)
	AcceptRequest(ctx context.Context, userID, friendshipID uuid.UUID) (*models.Friendship, error)
	RejectRequest(ctx context.Context, userID, friendshipID uuid.UUID) error
	RemoveFriend(ctx context.Context, userID, friendshipID uuid.UUID) error
	CancelRequest(ctx context.Context, userID, friendshipID uuid.UUID) error
	ListFriends(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error)
	ListPendingRequests(ctx context.Context, userID uuid.UUID) ([]models.FriendRequest, error)
	ListSentRequests(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error)
	IsFriend(ctx context.Context, userID, otherUserID uuid.UUID) (bool, error)
	GetFriendUserID(ctx context.Context, currentUserID, friendshipID uuid.UUID) (uuid.UUID, error)
}

// FriendChecker is a lightweight interface for friendship checks used by the reaction service.
type FriendChecker interface {
	IsFriend(ctx context.Context, userID, otherUserID uuid.UUID) (bool, error)
}

// BlockServiceInterface defines the contract for blocking operations.
type BlockServiceInterface interface {
	Block(ctx context.Context, blockerID, blockedID uuid.UUID) error
	Unblock(ctx context.Context, blockerID, blockedID uuid.UUID) error
	IsBlocked(ctx context.Context, userID, otherUserID uuid.UUID) (bool, error)
	ListBlocked(ctx context.Context, blockerID uuid.UUID) ([]models.BlockedUser, error)
}

// FriendInviteServiceInterface defines the contract for friend invite operations.
type FriendInviteServiceInterface interface {
	CreateInvite(ctx context.Context, inviterID uuid.UUID, expiresInDays int) (*models.FriendInvite, string, error)
	ListInvites(ctx context.Context, inviterID uuid.UUID) ([]models.FriendInvite, error)
	RevokeInvite(ctx context.Context, inviterID, inviteID uuid.UUID) error
	AcceptInvite(ctx context.Context, recipientID uuid.UUID, token string) (*models.UserSearchResult, error)
}

// ReactionServiceInterface defines the contract for reaction operations.
type ReactionServiceInterface interface {
	AddReaction(ctx context.Context, userID, itemID uuid.UUID, emoji string) (*models.Reaction, error)
	RemoveReaction(ctx context.Context, userID, itemID uuid.UUID) error
	GetReactionsForItem(ctx context.Context, itemID uuid.UUID) ([]models.ReactionWithUser, error)
	GetReactionSummaryForItem(ctx context.Context, itemID uuid.UUID) ([]models.ReactionSummary, error)
	GetReactionsForCard(ctx context.Context, cardID uuid.UUID) (map[uuid.UUID][]models.ReactionWithUser, error)
	GetUserReactionForItem(ctx context.Context, userID, itemID uuid.UUID) (*models.Reaction, error)
}

// NotificationServiceInterface defines the contract for notification operations.
type NotificationServiceInterface interface {
	GetSettings(ctx context.Context, userID uuid.UUID) (*models.NotificationSettings, error)
	UpdateSettings(ctx context.Context, userID uuid.UUID, patch models.NotificationSettingsPatch) (*models.NotificationSettings, error)
	List(ctx context.Context, userID uuid.UUID, params NotificationListParams) ([]models.Notification, error)
	MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	Delete(ctx context.Context, userID, notificationID uuid.UUID) error
	DeleteAll(ctx context.Context, userID uuid.UUID) error
	UnreadCount(ctx context.Context, userID uuid.UUID) (int, error)
	NotifyFriendRequestReceived(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error
	NotifyFriendRequestAccepted(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error
	NotifyFriendsNewCard(ctx context.Context, actorID, cardID uuid.UUID) error
	NotifyFriendsBingo(ctx context.Context, actorID, cardID uuid.UUID, bingoCount int) error
}

// EmailServiceInterface defines the contract for email operations.
type EmailServiceInterface interface {
	SendVerificationEmail(ctx context.Context, userID uuid.UUID, email string) error
	VerifyEmail(ctx context.Context, token string) error
	SendMagicLinkEmail(ctx context.Context, email string) error
	VerifyMagicLink(ctx context.Context, token string) (string, error)
	SendPasswordResetEmail(ctx context.Context, userID uuid.UUID, email string) error
	VerifyPasswordResetToken(ctx context.Context, token string) (uuid.UUID, error)
	MarkPasswordResetUsed(ctx context.Context, token string) error
	SendNotificationEmail(ctx context.Context, toEmail, subject, html, text string) error
	SendSupportEmail(ctx context.Context, fromEmail, category, message string, userID string) error
}

// ApiTokenServiceInterface defines the contract for API token operations used by handlers.
type ApiTokenServiceInterface interface {
	Create(ctx context.Context, userID uuid.UUID, name string, scope models.ApiTokenScope, expiresInDays int) (*models.ApiToken, string, error)
	List(ctx context.Context, userID uuid.UUID) ([]models.ApiToken, error)
	Delete(ctx context.Context, userID uuid.UUID, tokenID uuid.UUID) error
	DeleteAll(ctx context.Context, userID uuid.UUID) error
}
