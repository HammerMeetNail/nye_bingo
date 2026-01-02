package handlers

import (
	"context"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type mockUserService struct {
	CreateFunc            func(ctx context.Context, params models.CreateUserParams) (*models.User, error)
	GetByIDFunc           func(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmailFunc        func(ctx context.Context, email string) (*models.User, error)
	UpdatePasswordFunc    func(ctx context.Context, userID uuid.UUID, newPasswordHash string) error
	MarkEmailVerifiedFunc func(ctx context.Context, userID uuid.UUID) error
	UpdateSearchableFunc  func(ctx context.Context, userID uuid.UUID, searchable bool) error
}

func (m *mockUserService) Create(ctx context.Context, params models.CreateUserParams) (*models.User, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, params)
	}
	return nil, nil
}

func (m *mockUserService) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockUserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.GetByEmailFunc != nil {
		return m.GetByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (m *mockUserService) UpdatePassword(ctx context.Context, userID uuid.UUID, newPasswordHash string) error {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(ctx, userID, newPasswordHash)
	}
	return nil
}

func (m *mockUserService) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	if m.MarkEmailVerifiedFunc != nil {
		return m.MarkEmailVerifiedFunc(ctx, userID)
	}
	return nil
}

func (m *mockUserService) UpdateSearchable(ctx context.Context, userID uuid.UUID, searchable bool) error {
	if m.UpdateSearchableFunc != nil {
		return m.UpdateSearchableFunc(ctx, userID, searchable)
	}
	return nil
}

type mockAuthService struct {
	HashPasswordFunc          func(password string) (string, error)
	VerifyPasswordFunc        func(hash, password string) bool
	GenerateSessionTokenFunc  func() (string, string, error)
	CreateSessionFunc         func(ctx context.Context, userID uuid.UUID) (string, error)
	ValidateSessionFunc       func(ctx context.Context, token string) (*models.User, error)
	DeleteSessionFunc         func(ctx context.Context, token string) error
	DeleteAllUserSessionsFunc func(ctx context.Context, userID uuid.UUID) error
}

func (m *mockAuthService) HashPassword(password string) (string, error) {
	if m.HashPasswordFunc != nil {
		return m.HashPasswordFunc(password)
	}
	return "hashed_" + password, nil
}

func (m *mockAuthService) VerifyPassword(hash, password string) bool {
	if m.VerifyPasswordFunc != nil {
		return m.VerifyPasswordFunc(hash, password)
	}
	return hash == "hashed_"+password
}

func (m *mockAuthService) GenerateSessionToken() (string, string, error) {
	if m.GenerateSessionTokenFunc != nil {
		return m.GenerateSessionTokenFunc()
	}
	return "token", "hash", nil
}

func (m *mockAuthService) CreateSession(ctx context.Context, userID uuid.UUID) (string, error) {
	if m.CreateSessionFunc != nil {
		return m.CreateSessionFunc(ctx, userID)
	}
	return "test_session_token", nil
}

func (m *mockAuthService) ValidateSession(ctx context.Context, token string) (*models.User, error) {
	if m.ValidateSessionFunc != nil {
		return m.ValidateSessionFunc(ctx, token)
	}
	return nil, nil
}

func (m *mockAuthService) DeleteSession(ctx context.Context, token string) error {
	if m.DeleteSessionFunc != nil {
		return m.DeleteSessionFunc(ctx, token)
	}
	return nil
}

func (m *mockAuthService) DeleteAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	if m.DeleteAllUserSessionsFunc != nil {
		return m.DeleteAllUserSessionsFunc(ctx, userID)
	}
	return nil
}

type mockEmailService struct {
	SendVerificationEmailFunc    func(ctx context.Context, userID uuid.UUID, email string) error
	VerifyEmailFunc              func(ctx context.Context, token string) error
	SendMagicLinkEmailFunc       func(ctx context.Context, email string) error
	VerifyMagicLinkFunc          func(ctx context.Context, token string) (string, error)
	SendPasswordResetEmailFunc   func(ctx context.Context, userID uuid.UUID, email string) error
	VerifyPasswordResetTokenFunc func(ctx context.Context, token string) (uuid.UUID, error)
	MarkPasswordResetUsedFunc    func(ctx context.Context, token string) error
	SendNotificationEmailFunc    func(ctx context.Context, toEmail, subject, html, text string) error
	SendSupportEmailFunc         func(ctx context.Context, fromEmail, category, message string, userID string) error
}

func (m *mockEmailService) SendVerificationEmail(ctx context.Context, userID uuid.UUID, email string) error {
	if m.SendVerificationEmailFunc != nil {
		return m.SendVerificationEmailFunc(ctx, userID, email)
	}
	return nil
}

func (m *mockEmailService) VerifyEmail(ctx context.Context, token string) error {
	if m.VerifyEmailFunc != nil {
		return m.VerifyEmailFunc(ctx, token)
	}
	return nil
}

func (m *mockEmailService) SendMagicLinkEmail(ctx context.Context, email string) error {
	if m.SendMagicLinkEmailFunc != nil {
		return m.SendMagicLinkEmailFunc(ctx, email)
	}
	return nil
}

func (m *mockEmailService) VerifyMagicLink(ctx context.Context, token string) (string, error) {
	if m.VerifyMagicLinkFunc != nil {
		return m.VerifyMagicLinkFunc(ctx, token)
	}
	return "", nil
}

func (m *mockEmailService) SendPasswordResetEmail(ctx context.Context, userID uuid.UUID, email string) error {
	if m.SendPasswordResetEmailFunc != nil {
		return m.SendPasswordResetEmailFunc(ctx, userID, email)
	}
	return nil
}

func (m *mockEmailService) VerifyPasswordResetToken(ctx context.Context, token string) (uuid.UUID, error) {
	if m.VerifyPasswordResetTokenFunc != nil {
		return m.VerifyPasswordResetTokenFunc(ctx, token)
	}
	return uuid.Nil, nil
}

func (m *mockEmailService) MarkPasswordResetUsed(ctx context.Context, token string) error {
	if m.MarkPasswordResetUsedFunc != nil {
		return m.MarkPasswordResetUsedFunc(ctx, token)
	}
	return nil
}

func (m *mockEmailService) SendNotificationEmail(ctx context.Context, toEmail, subject, html, text string) error {
	if m.SendNotificationEmailFunc != nil {
		return m.SendNotificationEmailFunc(ctx, toEmail, subject, html, text)
	}
	return nil
}

func (m *mockEmailService) SendSupportEmail(ctx context.Context, fromEmail, category, message string, userID string) error {
	if m.SendSupportEmailFunc != nil {
		return m.SendSupportEmailFunc(ctx, fromEmail, category, message, userID)
	}
	return nil
}

type mockCardService struct {
	CheckForConflictFunc     func(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error)
	CreateFunc               func(ctx context.Context, params models.CreateCardParams) (*models.BingoCard, error)
	ListByUserFunc           func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error)
	GetByIDFunc              func(ctx context.Context, cardID uuid.UUID) (*models.BingoCard, error)
	DeleteFunc               func(ctx context.Context, userID, cardID uuid.UUID) error
	AddItemFunc              func(ctx context.Context, userID uuid.UUID, params models.AddItemParams) (*models.BingoItem, error)
	UpdateConfigFunc         func(ctx context.Context, userID, cardID uuid.UUID, params models.UpdateCardConfigParams) (*models.BingoCard, error)
	CloneFunc                func(ctx context.Context, userID, cardID uuid.UUID, params services.CloneParams) (*services.CloneResult, error)
	UpdateItemFunc           func(ctx context.Context, userID, cardID uuid.UUID, position int, params models.UpdateItemParams) (*models.BingoItem, error)
	RemoveItemFunc           func(ctx context.Context, userID, cardID uuid.UUID, position int) error
	ShuffleFunc              func(ctx context.Context, userID, cardID uuid.UUID) (*models.BingoCard, error)
	SwapItemsFunc            func(ctx context.Context, userID, cardID uuid.UUID, pos1, pos2 int) error
	FinalizeFunc             func(ctx context.Context, userID, cardID uuid.UUID, params *services.FinalizeParams) (*models.BingoCard, error)
	CompleteItemFunc         func(ctx context.Context, userID, cardID uuid.UUID, position int, params models.CompleteItemParams) (*models.BingoItem, error)
	UncompleteItemFunc       func(ctx context.Context, userID, cardID uuid.UUID, position int) (*models.BingoItem, error)
	UpdateItemNotesFunc      func(ctx context.Context, userID, cardID uuid.UUID, position int, notes, proofURL *string) (*models.BingoItem, error)
	GetArchiveFunc           func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error)
	GetStatsFunc             func(ctx context.Context, userID, cardID uuid.UUID) (*models.CardStats, error)
	UpdateMetaFunc           func(ctx context.Context, userID, cardID uuid.UUID, params models.UpdateCardMetaParams) (*models.BingoCard, error)
	UpdateVisibilityFunc     func(ctx context.Context, userID, cardID uuid.UUID, visibleToFriends bool) (*models.BingoCard, error)
	BulkUpdateVisibilityFunc func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, visibleToFriends bool) (int, error)
	BulkDeleteFunc           func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID) (int, error)
	BulkUpdateArchiveFunc    func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, isArchived bool) (int, error)
	ImportFunc               func(ctx context.Context, params models.ImportCardParams) (*models.BingoCard, error)
}

func (m *mockCardService) CheckForConflict(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error) {
	if m.CheckForConflictFunc != nil {
		return m.CheckForConflictFunc(ctx, userID, year, title)
	}
	return nil, services.ErrCardNotFound
}

func (m *mockCardService) Create(ctx context.Context, params models.CreateCardParams) (*models.BingoCard, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, params)
	}
	return nil, nil
}

func (m *mockCardService) ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
	if m.ListByUserFunc != nil {
		return m.ListByUserFunc(ctx, userID)
	}
	return nil, nil
}

func (m *mockCardService) GetByID(ctx context.Context, cardID uuid.UUID) (*models.BingoCard, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, cardID)
	}
	return nil, nil
}

func (m *mockCardService) Delete(ctx context.Context, userID, cardID uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, userID, cardID)
	}
	return nil
}

func (m *mockCardService) AddItem(ctx context.Context, userID uuid.UUID, params models.AddItemParams) (*models.BingoItem, error) {
	if m.AddItemFunc != nil {
		return m.AddItemFunc(ctx, userID, params)
	}
	return nil, nil
}

func (m *mockCardService) UpdateConfig(ctx context.Context, userID, cardID uuid.UUID, params models.UpdateCardConfigParams) (*models.BingoCard, error) {
	if m.UpdateConfigFunc != nil {
		return m.UpdateConfigFunc(ctx, userID, cardID, params)
	}
	return nil, nil
}

func (m *mockCardService) Clone(ctx context.Context, userID, cardID uuid.UUID, params services.CloneParams) (*services.CloneResult, error) {
	if m.CloneFunc != nil {
		return m.CloneFunc(ctx, userID, cardID, params)
	}
	return nil, nil
}

func (m *mockCardService) UpdateItem(ctx context.Context, userID, cardID uuid.UUID, position int, params models.UpdateItemParams) (*models.BingoItem, error) {
	if m.UpdateItemFunc != nil {
		return m.UpdateItemFunc(ctx, userID, cardID, position, params)
	}
	return nil, nil
}

func (m *mockCardService) RemoveItem(ctx context.Context, userID, cardID uuid.UUID, position int) error {
	if m.RemoveItemFunc != nil {
		return m.RemoveItemFunc(ctx, userID, cardID, position)
	}
	return nil
}

func (m *mockCardService) Shuffle(ctx context.Context, userID, cardID uuid.UUID) (*models.BingoCard, error) {
	if m.ShuffleFunc != nil {
		return m.ShuffleFunc(ctx, userID, cardID)
	}
	return nil, nil
}

func (m *mockCardService) SwapItems(ctx context.Context, userID, cardID uuid.UUID, pos1, pos2 int) error {
	if m.SwapItemsFunc != nil {
		return m.SwapItemsFunc(ctx, userID, cardID, pos1, pos2)
	}
	return nil
}

func (m *mockCardService) Finalize(ctx context.Context, userID, cardID uuid.UUID, params *services.FinalizeParams) (*models.BingoCard, error) {
	if m.FinalizeFunc != nil {
		return m.FinalizeFunc(ctx, userID, cardID, params)
	}
	return nil, nil
}

func (m *mockCardService) CompleteItem(ctx context.Context, userID, cardID uuid.UUID, position int, params models.CompleteItemParams) (*models.BingoItem, error) {
	if m.CompleteItemFunc != nil {
		return m.CompleteItemFunc(ctx, userID, cardID, position, params)
	}
	return nil, nil
}

func (m *mockCardService) UncompleteItem(ctx context.Context, userID, cardID uuid.UUID, position int) (*models.BingoItem, error) {
	if m.UncompleteItemFunc != nil {
		return m.UncompleteItemFunc(ctx, userID, cardID, position)
	}
	return nil, nil
}

func (m *mockCardService) UpdateItemNotes(ctx context.Context, userID, cardID uuid.UUID, position int, notes, proofURL *string) (*models.BingoItem, error) {
	if m.UpdateItemNotesFunc != nil {
		return m.UpdateItemNotesFunc(ctx, userID, cardID, position, notes, proofURL)
	}
	return nil, nil
}

func (m *mockCardService) GetArchive(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error) {
	if m.GetArchiveFunc != nil {
		return m.GetArchiveFunc(ctx, userID)
	}
	return nil, nil
}

func (m *mockCardService) GetStats(ctx context.Context, userID, cardID uuid.UUID) (*models.CardStats, error) {
	if m.GetStatsFunc != nil {
		return m.GetStatsFunc(ctx, userID, cardID)
	}
	return nil, nil
}

func (m *mockCardService) UpdateMeta(ctx context.Context, userID, cardID uuid.UUID, params models.UpdateCardMetaParams) (*models.BingoCard, error) {
	if m.UpdateMetaFunc != nil {
		return m.UpdateMetaFunc(ctx, userID, cardID, params)
	}
	return nil, nil
}

func (m *mockCardService) UpdateVisibility(ctx context.Context, userID, cardID uuid.UUID, visibleToFriends bool) (*models.BingoCard, error) {
	if m.UpdateVisibilityFunc != nil {
		return m.UpdateVisibilityFunc(ctx, userID, cardID, visibleToFriends)
	}
	return nil, nil
}

func (m *mockCardService) BulkUpdateVisibility(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, visibleToFriends bool) (int, error) {
	if m.BulkUpdateVisibilityFunc != nil {
		return m.BulkUpdateVisibilityFunc(ctx, userID, cardIDs, visibleToFriends)
	}
	return 0, nil
}

func (m *mockCardService) BulkDelete(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID) (int, error) {
	if m.BulkDeleteFunc != nil {
		return m.BulkDeleteFunc(ctx, userID, cardIDs)
	}
	return 0, nil
}

func (m *mockCardService) BulkUpdateArchive(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, isArchived bool) (int, error) {
	if m.BulkUpdateArchiveFunc != nil {
		return m.BulkUpdateArchiveFunc(ctx, userID, cardIDs, isArchived)
	}
	return 0, nil
}

func (m *mockCardService) Import(ctx context.Context, params models.ImportCardParams) (*models.BingoCard, error) {
	if m.ImportFunc != nil {
		return m.ImportFunc(ctx, params)
	}
	return nil, nil
}

type mockSuggestionService struct {
	GetAllFunc               func(ctx context.Context) ([]*models.Suggestion, error)
	GetByCategoryFunc        func(ctx context.Context, category string) ([]*models.Suggestion, error)
	GetCategoriesFunc        func(ctx context.Context) ([]string, error)
	GetGroupedByCategoryFunc func(ctx context.Context) ([]services.SuggestionsByCategory, error)
}

func (m *mockSuggestionService) GetAll(ctx context.Context) ([]*models.Suggestion, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx)
	}
	return nil, nil
}

func (m *mockSuggestionService) GetByCategory(ctx context.Context, category string) ([]*models.Suggestion, error) {
	if m.GetByCategoryFunc != nil {
		return m.GetByCategoryFunc(ctx, category)
	}
	return nil, nil
}

func (m *mockSuggestionService) GetCategories(ctx context.Context) ([]string, error) {
	if m.GetCategoriesFunc != nil {
		return m.GetCategoriesFunc(ctx)
	}
	return nil, nil
}

func (m *mockSuggestionService) GetGroupedByCategory(ctx context.Context) ([]services.SuggestionsByCategory, error) {
	if m.GetGroupedByCategoryFunc != nil {
		return m.GetGroupedByCategoryFunc(ctx)
	}
	return nil, nil
}

type mockFriendService struct {
	SearchUsersFunc         func(ctx context.Context, currentUserID uuid.UUID, query string) ([]models.UserSearchResult, error)
	SendRequestFunc         func(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error)
	AcceptRequestFunc       func(ctx context.Context, userID, friendshipID uuid.UUID) (*models.Friendship, error)
	RejectRequestFunc       func(ctx context.Context, userID, friendshipID uuid.UUID) error
	RemoveFriendFunc        func(ctx context.Context, userID, friendshipID uuid.UUID) error
	CancelRequestFunc       func(ctx context.Context, userID, friendshipID uuid.UUID) error
	ListFriendsFunc         func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error)
	ListPendingRequestsFunc func(ctx context.Context, userID uuid.UUID) ([]models.FriendRequest, error)
	ListSentRequestsFunc    func(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error)
	IsFriendFunc            func(ctx context.Context, userID, otherUserID uuid.UUID) (bool, error)
	GetFriendUserIDFunc     func(ctx context.Context, currentUserID, friendshipID uuid.UUID) (uuid.UUID, error)
}

func (m *mockFriendService) SearchUsers(ctx context.Context, currentUserID uuid.UUID, query string) ([]models.UserSearchResult, error) {
	if m.SearchUsersFunc != nil {
		return m.SearchUsersFunc(ctx, currentUserID, query)
	}
	return nil, nil
}

func (m *mockFriendService) SendRequest(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error) {
	if m.SendRequestFunc != nil {
		return m.SendRequestFunc(ctx, userID, friendID)
	}
	return nil, nil
}

func (m *mockFriendService) AcceptRequest(ctx context.Context, userID, friendshipID uuid.UUID) (*models.Friendship, error) {
	if m.AcceptRequestFunc != nil {
		return m.AcceptRequestFunc(ctx, userID, friendshipID)
	}
	return nil, nil
}

func (m *mockFriendService) RejectRequest(ctx context.Context, userID, friendshipID uuid.UUID) error {
	if m.RejectRequestFunc != nil {
		return m.RejectRequestFunc(ctx, userID, friendshipID)
	}
	return nil
}

func (m *mockFriendService) RemoveFriend(ctx context.Context, userID, friendshipID uuid.UUID) error {
	if m.RemoveFriendFunc != nil {
		return m.RemoveFriendFunc(ctx, userID, friendshipID)
	}
	return nil
}

func (m *mockFriendService) CancelRequest(ctx context.Context, userID, friendshipID uuid.UUID) error {
	if m.CancelRequestFunc != nil {
		return m.CancelRequestFunc(ctx, userID, friendshipID)
	}
	return nil
}

func (m *mockFriendService) ListFriends(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
	if m.ListFriendsFunc != nil {
		return m.ListFriendsFunc(ctx, userID)
	}
	return nil, nil
}

func (m *mockFriendService) ListPendingRequests(ctx context.Context, userID uuid.UUID) ([]models.FriendRequest, error) {
	if m.ListPendingRequestsFunc != nil {
		return m.ListPendingRequestsFunc(ctx, userID)
	}
	return nil, nil
}

func (m *mockFriendService) ListSentRequests(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
	if m.ListSentRequestsFunc != nil {
		return m.ListSentRequestsFunc(ctx, userID)
	}
	return nil, nil
}

func (m *mockFriendService) IsFriend(ctx context.Context, userID, otherUserID uuid.UUID) (bool, error) {
	if m.IsFriendFunc != nil {
		return m.IsFriendFunc(ctx, userID, otherUserID)
	}
	return false, nil
}

func (m *mockFriendService) GetFriendUserID(ctx context.Context, currentUserID, friendshipID uuid.UUID) (uuid.UUID, error) {
	if m.GetFriendUserIDFunc != nil {
		return m.GetFriendUserIDFunc(ctx, currentUserID, friendshipID)
	}
	return uuid.Nil, nil
}

type mockReactionService struct {
	AddReactionFunc               func(ctx context.Context, userID, itemID uuid.UUID, emoji string) (*models.Reaction, error)
	RemoveReactionFunc            func(ctx context.Context, userID, itemID uuid.UUID) error
	GetReactionsForItemFunc       func(ctx context.Context, itemID uuid.UUID) ([]models.ReactionWithUser, error)
	GetReactionSummaryForItemFunc func(ctx context.Context, itemID uuid.UUID) ([]models.ReactionSummary, error)
	GetReactionsForCardFunc       func(ctx context.Context, cardID uuid.UUID) (map[uuid.UUID][]models.ReactionWithUser, error)
	GetUserReactionForItemFunc    func(ctx context.Context, userID, itemID uuid.UUID) (*models.Reaction, error)
}

func (m *mockReactionService) AddReaction(ctx context.Context, userID, itemID uuid.UUID, emoji string) (*models.Reaction, error) {
	if m.AddReactionFunc != nil {
		return m.AddReactionFunc(ctx, userID, itemID, emoji)
	}
	return nil, nil
}

func (m *mockReactionService) RemoveReaction(ctx context.Context, userID, itemID uuid.UUID) error {
	if m.RemoveReactionFunc != nil {
		return m.RemoveReactionFunc(ctx, userID, itemID)
	}
	return nil
}

func (m *mockReactionService) GetReactionsForItem(ctx context.Context, itemID uuid.UUID) ([]models.ReactionWithUser, error) {
	if m.GetReactionsForItemFunc != nil {
		return m.GetReactionsForItemFunc(ctx, itemID)
	}
	return nil, nil
}

func (m *mockReactionService) GetReactionSummaryForItem(ctx context.Context, itemID uuid.UUID) ([]models.ReactionSummary, error) {
	if m.GetReactionSummaryForItemFunc != nil {
		return m.GetReactionSummaryForItemFunc(ctx, itemID)
	}
	return nil, nil
}

func (m *mockReactionService) GetReactionsForCard(ctx context.Context, cardID uuid.UUID) (map[uuid.UUID][]models.ReactionWithUser, error) {
	if m.GetReactionsForCardFunc != nil {
		return m.GetReactionsForCardFunc(ctx, cardID)
	}
	return nil, nil
}

func (m *mockReactionService) GetUserReactionForItem(ctx context.Context, userID, itemID uuid.UUID) (*models.Reaction, error) {
	if m.GetUserReactionForItemFunc != nil {
		return m.GetUserReactionForItemFunc(ctx, userID, itemID)
	}
	return nil, nil
}

type mockApiTokenService struct {
	CreateFunc    func(ctx context.Context, userID uuid.UUID, name string, scope models.ApiTokenScope, expiresInDays int) (*models.ApiToken, string, error)
	ListFunc      func(ctx context.Context, userID uuid.UUID) ([]models.ApiToken, error)
	DeleteFunc    func(ctx context.Context, userID uuid.UUID, tokenID uuid.UUID) error
	DeleteAllFunc func(ctx context.Context, userID uuid.UUID) error
}

func (m *mockApiTokenService) Create(ctx context.Context, userID uuid.UUID, name string, scope models.ApiTokenScope, expiresInDays int) (*models.ApiToken, string, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, userID, name, scope, expiresInDays)
	}
	return nil, "", nil
}

func (m *mockApiTokenService) List(ctx context.Context, userID uuid.UUID) ([]models.ApiToken, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, userID)
	}
	return nil, nil
}

func (m *mockApiTokenService) Delete(ctx context.Context, userID uuid.UUID, tokenID uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, userID, tokenID)
	}
	return nil
}

func (m *mockApiTokenService) DeleteAll(ctx context.Context, userID uuid.UUID) error {
	if m.DeleteAllFunc != nil {
		return m.DeleteAllFunc(ctx, userID)
	}
	return nil
}

type mockNotificationService struct {
	GetSettingsFunc    func(ctx context.Context, userID uuid.UUID) (*models.NotificationSettings, error)
	UpdateSettingsFunc func(ctx context.Context, userID uuid.UUID, patch models.NotificationSettingsPatch) (*models.NotificationSettings, error)
	ListFunc           func(ctx context.Context, userID uuid.UUID, params services.NotificationListParams) ([]models.Notification, error)
	MarkReadFunc       func(ctx context.Context, userID, notificationID uuid.UUID) error
	MarkAllReadFunc    func(ctx context.Context, userID uuid.UUID) error
	DeleteFunc         func(ctx context.Context, userID, notificationID uuid.UUID) error
	DeleteAllFunc      func(ctx context.Context, userID uuid.UUID) error
	UnreadCountFunc    func(ctx context.Context, userID uuid.UUID) (int, error)
	NotifyRequestFunc  func(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error
	NotifyAcceptedFunc func(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error
	NotifyNewCardFunc  func(ctx context.Context, actorID, cardID uuid.UUID) error
	NotifyBingoFunc    func(ctx context.Context, actorID, cardID uuid.UUID, bingoCount int) error
}

func (m *mockNotificationService) GetSettings(ctx context.Context, userID uuid.UUID) (*models.NotificationSettings, error) {
	if m.GetSettingsFunc != nil {
		return m.GetSettingsFunc(ctx, userID)
	}
	return &models.NotificationSettings{}, nil
}

func (m *mockNotificationService) UpdateSettings(ctx context.Context, userID uuid.UUID, patch models.NotificationSettingsPatch) (*models.NotificationSettings, error) {
	if m.UpdateSettingsFunc != nil {
		return m.UpdateSettingsFunc(ctx, userID, patch)
	}
	return &models.NotificationSettings{}, nil
}

func (m *mockNotificationService) List(ctx context.Context, userID uuid.UUID, params services.NotificationListParams) ([]models.Notification, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, userID, params)
	}
	return []models.Notification{}, nil
}

func (m *mockNotificationService) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	if m.MarkReadFunc != nil {
		return m.MarkReadFunc(ctx, userID, notificationID)
	}
	return nil
}

func (m *mockNotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	if m.MarkAllReadFunc != nil {
		return m.MarkAllReadFunc(ctx, userID)
	}
	return nil
}

func (m *mockNotificationService) Delete(ctx context.Context, userID, notificationID uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, userID, notificationID)
	}
	return nil
}

func (m *mockNotificationService) DeleteAll(ctx context.Context, userID uuid.UUID) error {
	if m.DeleteAllFunc != nil {
		return m.DeleteAllFunc(ctx, userID)
	}
	return nil
}

func (m *mockNotificationService) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	if m.UnreadCountFunc != nil {
		return m.UnreadCountFunc(ctx, userID)
	}
	return 0, nil
}

func (m *mockNotificationService) NotifyFriendRequestReceived(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error {
	if m.NotifyRequestFunc != nil {
		return m.NotifyRequestFunc(ctx, recipientID, actorID, friendshipID)
	}
	return nil
}

func (m *mockNotificationService) NotifyFriendRequestAccepted(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error {
	if m.NotifyAcceptedFunc != nil {
		return m.NotifyAcceptedFunc(ctx, recipientID, actorID, friendshipID)
	}
	return nil
}

func (m *mockNotificationService) NotifyFriendsNewCard(ctx context.Context, actorID, cardID uuid.UUID) error {
	if m.NotifyNewCardFunc != nil {
		return m.NotifyNewCardFunc(ctx, actorID, cardID)
	}
	return nil
}

func (m *mockNotificationService) NotifyFriendsBingo(ctx context.Context, actorID, cardID uuid.UUID, bingoCount int) error {
	if m.NotifyBingoFunc != nil {
		return m.NotifyBingoFunc(ctx, actorID, cardID, bingoCount)
	}
	return nil
}
