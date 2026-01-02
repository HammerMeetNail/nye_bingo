package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

type stubNotificationService struct {
	NotifyFriendRequestReceivedFunc func(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error
	NotifyFriendRequestAcceptedFunc func(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error
	NotifyFriendsNewCardFunc        func(ctx context.Context, actorID, cardID uuid.UUID) error
	NotifyFriendsBingoFunc          func(ctx context.Context, actorID, cardID uuid.UUID, bingoCount int) error
}

func (s *stubNotificationService) GetSettings(ctx context.Context, userID uuid.UUID) (*models.NotificationSettings, error) {
	return &models.NotificationSettings{}, nil
}

func (s *stubNotificationService) UpdateSettings(ctx context.Context, userID uuid.UUID, patch models.NotificationSettingsPatch) (*models.NotificationSettings, error) {
	return &models.NotificationSettings{}, nil
}

func (s *stubNotificationService) List(ctx context.Context, userID uuid.UUID, params NotificationListParams) ([]models.Notification, error) {
	return []models.Notification{}, nil
}

func (s *stubNotificationService) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	return nil
}

func (s *stubNotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (s *stubNotificationService) Delete(ctx context.Context, userID, notificationID uuid.UUID) error {
	return nil
}

func (s *stubNotificationService) DeleteAll(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (s *stubNotificationService) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return 0, nil
}

func (s *stubNotificationService) NotifyFriendRequestReceived(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error {
	if s.NotifyFriendRequestReceivedFunc != nil {
		return s.NotifyFriendRequestReceivedFunc(ctx, recipientID, actorID, friendshipID)
	}
	return nil
}

func (s *stubNotificationService) NotifyFriendRequestAccepted(ctx context.Context, recipientID, actorID, friendshipID uuid.UUID) error {
	if s.NotifyFriendRequestAcceptedFunc != nil {
		return s.NotifyFriendRequestAcceptedFunc(ctx, recipientID, actorID, friendshipID)
	}
	return nil
}

func (s *stubNotificationService) NotifyFriendsNewCard(ctx context.Context, actorID, cardID uuid.UUID) error {
	if s.NotifyFriendsNewCardFunc != nil {
		return s.NotifyFriendsNewCardFunc(ctx, actorID, cardID)
	}
	return nil
}

func (s *stubNotificationService) NotifyFriendsBingo(ctx context.Context, actorID, cardID uuid.UUID, bingoCount int) error {
	if s.NotifyFriendsBingoFunc != nil {
		return s.NotifyFriendsBingoFunc(ctx, actorID, cardID, bingoCount)
	}
	return nil
}
