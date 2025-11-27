package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrFriendshipNotFound     = errors.New("friendship not found")
	ErrFriendshipExists       = errors.New("friendship already exists")
	ErrCannotFriendSelf       = errors.New("cannot send friend request to yourself")
	ErrFriendshipNotPending   = errors.New("friendship is not pending")
	ErrNotFriendshipRecipient = errors.New("only the recipient can accept/reject")
	ErrNotFriend              = errors.New("you are not friends with this user")
)

type FriendService struct {
	db *pgxpool.Pool
}

func NewFriendService(db *pgxpool.Pool) *FriendService {
	return &FriendService{db: db}
}

func (s *FriendService) SearchUsers(ctx context.Context, currentUserID uuid.UUID, query string) ([]models.UserSearchResult, error) {
	query = strings.TrimSpace(query)
	if len(query) < 2 {
		return []models.UserSearchResult{}, nil
	}

	searchPattern := "%" + strings.ToLower(query) + "%"

	rows, err := s.db.Query(ctx,
		`SELECT id, display_name, email FROM users
		 WHERE id != $1
		   AND (LOWER(display_name) LIKE $2 OR LOWER(email) LIKE $2)
		 ORDER BY display_name
		 LIMIT 20`,
		currentUserID, searchPattern,
	)
	if err != nil {
		return nil, fmt.Errorf("searching users: %w", err)
	}
	defer rows.Close()

	var results []models.UserSearchResult
	for rows.Next() {
		var user models.UserSearchResult
		if err := rows.Scan(&user.ID, &user.DisplayName, &user.Email); err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		results = append(results, user)
	}

	if results == nil {
		results = []models.UserSearchResult{}
	}

	return results, nil
}

func (s *FriendService) SendRequest(ctx context.Context, userID, friendID uuid.UUID) (*models.Friendship, error) {
	if userID == friendID {
		return nil, ErrCannotFriendSelf
	}

	// Check if friendship already exists in either direction
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM friendships
			WHERE (user_id = $1 AND friend_id = $2)
			   OR (user_id = $2 AND friend_id = $1)
		)`,
		userID, friendID,
	).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("checking friendship existence: %w", err)
	}
	if exists {
		return nil, ErrFriendshipExists
	}

	friendship := &models.Friendship{}
	err = s.db.QueryRow(ctx,
		`INSERT INTO friendships (user_id, friend_id, status)
		 VALUES ($1, $2, 'pending')
		 RETURNING id, user_id, friend_id, status, created_at`,
		userID, friendID,
	).Scan(&friendship.ID, &friendship.UserID, &friendship.FriendID, &friendship.Status, &friendship.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating friendship: %w", err)
	}

	return friendship, nil
}

func (s *FriendService) AcceptRequest(ctx context.Context, userID, friendshipID uuid.UUID) (*models.Friendship, error) {
	// Get the friendship
	friendship, err := s.getByID(ctx, friendshipID)
	if err != nil {
		return nil, err
	}

	// Only the recipient (friend_id) can accept
	if friendship.FriendID != userID {
		return nil, ErrNotFriendshipRecipient
	}

	if friendship.Status != models.FriendshipStatusPending {
		return nil, ErrFriendshipNotPending
	}

	_, err = s.db.Exec(ctx,
		"UPDATE friendships SET status = 'accepted' WHERE id = $1",
		friendshipID,
	)
	if err != nil {
		return nil, fmt.Errorf("accepting friendship: %w", err)
	}

	friendship.Status = models.FriendshipStatusAccepted
	return friendship, nil
}

func (s *FriendService) RejectRequest(ctx context.Context, userID, friendshipID uuid.UUID) error {
	// Get the friendship
	friendship, err := s.getByID(ctx, friendshipID)
	if err != nil {
		return err
	}

	// Only the recipient (friend_id) can reject
	if friendship.FriendID != userID {
		return ErrNotFriendshipRecipient
	}

	if friendship.Status != models.FriendshipStatusPending {
		return ErrFriendshipNotPending
	}

	_, err = s.db.Exec(ctx,
		"DELETE FROM friendships WHERE id = $1",
		friendshipID,
	)
	if err != nil {
		return fmt.Errorf("rejecting friendship: %w", err)
	}

	return nil
}

func (s *FriendService) RemoveFriend(ctx context.Context, userID, friendshipID uuid.UUID) error {
	// Get the friendship
	friendship, err := s.getByID(ctx, friendshipID)
	if err != nil {
		return err
	}

	// Either party can remove the friendship
	if friendship.UserID != userID && friendship.FriendID != userID {
		return ErrFriendshipNotFound
	}

	_, err = s.db.Exec(ctx,
		"DELETE FROM friendships WHERE id = $1",
		friendshipID,
	)
	if err != nil {
		return fmt.Errorf("removing friendship: %w", err)
	}

	return nil
}

func (s *FriendService) CancelRequest(ctx context.Context, userID, friendshipID uuid.UUID) error {
	// Get the friendship
	friendship, err := s.getByID(ctx, friendshipID)
	if err != nil {
		return err
	}

	// Only the sender (user_id) can cancel
	if friendship.UserID != userID {
		return ErrFriendshipNotFound
	}

	if friendship.Status != models.FriendshipStatusPending {
		return ErrFriendshipNotPending
	}

	_, err = s.db.Exec(ctx,
		"DELETE FROM friendships WHERE id = $1",
		friendshipID,
	)
	if err != nil {
		return fmt.Errorf("canceling friendship: %w", err)
	}

	return nil
}

func (s *FriendService) ListFriends(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
	rows, err := s.db.Query(ctx,
		`SELECT f.id, f.user_id, f.friend_id, f.status, f.created_at,
		        CASE WHEN f.user_id = $1 THEN u2.display_name ELSE u1.display_name END,
		        CASE WHEN f.user_id = $1 THEN u2.email ELSE u1.email END
		 FROM friendships f
		 JOIN users u1 ON f.user_id = u1.id
		 JOIN users u2 ON f.friend_id = u2.id
		 WHERE (f.user_id = $1 OR f.friend_id = $1) AND f.status = 'accepted'
		 ORDER BY CASE WHEN f.user_id = $1 THEN u2.display_name ELSE u1.display_name END`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing friends: %w", err)
	}
	defer rows.Close()

	var friends []models.FriendWithUser
	for rows.Next() {
		var f models.FriendWithUser
		if err := rows.Scan(&f.ID, &f.UserID, &f.FriendID, &f.Status, &f.CreatedAt, &f.FriendDisplayName, &f.FriendEmail); err != nil {
			return nil, fmt.Errorf("scanning friend: %w", err)
		}
		friends = append(friends, f)
	}

	if friends == nil {
		friends = []models.FriendWithUser{}
	}

	return friends, nil
}

func (s *FriendService) ListPendingRequests(ctx context.Context, userID uuid.UUID) ([]models.FriendRequest, error) {
	rows, err := s.db.Query(ctx,
		`SELECT f.id, f.user_id, f.friend_id, f.status, f.created_at, u.display_name, u.email
		 FROM friendships f
		 JOIN users u ON f.user_id = u.id
		 WHERE f.friend_id = $1 AND f.status = 'pending'
		 ORDER BY f.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing pending requests: %w", err)
	}
	defer rows.Close()

	var requests []models.FriendRequest
	for rows.Next() {
		var r models.FriendRequest
		if err := rows.Scan(&r.ID, &r.UserID, &r.FriendID, &r.Status, &r.CreatedAt, &r.RequesterDisplayName, &r.RequesterEmail); err != nil {
			return nil, fmt.Errorf("scanning request: %w", err)
		}
		requests = append(requests, r)
	}

	if requests == nil {
		requests = []models.FriendRequest{}
	}

	return requests, nil
}

func (s *FriendService) ListSentRequests(ctx context.Context, userID uuid.UUID) ([]models.FriendWithUser, error) {
	rows, err := s.db.Query(ctx,
		`SELECT f.id, f.user_id, f.friend_id, f.status, f.created_at, u.display_name, u.email
		 FROM friendships f
		 JOIN users u ON f.friend_id = u.id
		 WHERE f.user_id = $1 AND f.status = 'pending'
		 ORDER BY f.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing sent requests: %w", err)
	}
	defer rows.Close()

	var requests []models.FriendWithUser
	for rows.Next() {
		var f models.FriendWithUser
		if err := rows.Scan(&f.ID, &f.UserID, &f.FriendID, &f.Status, &f.CreatedAt, &f.FriendDisplayName, &f.FriendEmail); err != nil {
			return nil, fmt.Errorf("scanning request: %w", err)
		}
		requests = append(requests, f)
	}

	if requests == nil {
		requests = []models.FriendWithUser{}
	}

	return requests, nil
}

func (s *FriendService) IsFriend(ctx context.Context, userID, otherUserID uuid.UUID) (bool, error) {
	var isFriend bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM friendships
			WHERE ((user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1))
			  AND status = 'accepted'
		)`,
		userID, otherUserID,
	).Scan(&isFriend)
	if err != nil {
		return false, fmt.Errorf("checking friendship: %w", err)
	}
	return isFriend, nil
}

func (s *FriendService) GetFriendUserID(ctx context.Context, currentUserID, friendshipID uuid.UUID) (uuid.UUID, error) {
	friendship, err := s.getByID(ctx, friendshipID)
	if err != nil {
		return uuid.Nil, err
	}

	// Check that current user is part of this friendship
	if friendship.UserID != currentUserID && friendship.FriendID != currentUserID {
		return uuid.Nil, ErrFriendshipNotFound
	}

	if friendship.Status != models.FriendshipStatusAccepted {
		return uuid.Nil, ErrNotFriend
	}

	// Return the other user's ID
	if friendship.UserID == currentUserID {
		return friendship.FriendID, nil
	}
	return friendship.UserID, nil
}

func (s *FriendService) getByID(ctx context.Context, friendshipID uuid.UUID) (*models.Friendship, error) {
	friendship := &models.Friendship{}
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, friend_id, status, created_at
		 FROM friendships WHERE id = $1`,
		friendshipID,
	).Scan(&friendship.ID, &friendship.UserID, &friendship.FriendID, &friendship.Status, &friendship.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFriendshipNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting friendship: %w", err)
	}
	return friendship, nil
}
