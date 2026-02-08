package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
)

func (s *CardService) SetNotificationService(notificationService NotificationServiceInterface) {
	s.notificationService = notificationService
}

func (s *CardService) notifyFriendsNewCard(ctx context.Context, userID, cardID uuid.UUID) {
	if s.notificationService == nil {
		return
	}
	if err := s.notificationService.NotifyFriendsNewCard(ctx, userID, cardID); err != nil {
		logging.Error("Failed to notify friends about new card", map[string]interface{}{
			"error":   err.Error(),
			"user_id": userID.String(),
			"card_id": cardID.String(),
		})
	}
}

func (s *CardService) notifyFriendsBingo(ctx context.Context, userID, cardID uuid.UUID, bingoCount int) {
	if s.notificationService == nil {
		return
	}
	if err := s.notificationService.NotifyFriendsBingo(ctx, userID, cardID, bingoCount); err != nil {
		logging.Error("Failed to notify friends about bingo", map[string]interface{}{
			"error":       err.Error(),
			"user_id":     userID.String(),
			"card_id":     cardID.String(),
			"bingo_count": bingoCount,
		})
	}
}
