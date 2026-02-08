package services

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrCardNotFound      = errors.New("card not found")
	ErrCardAlreadyExists = errors.New("card already exists for this year")
	ErrCardTitleExists   = errors.New("you already have a card with this title for this year")
	ErrCardFinalized     = errors.New("card is finalized and cannot be modified")
	ErrCardNotFinalized  = errors.New("card must be finalized first")
	ErrCardFull          = errors.New("card is full")
	ErrItemNotFound      = errors.New("item not found")
	ErrPositionOccupied  = errors.New("position is already occupied")
	ErrInvalidPosition   = errors.New("invalid position")
	ErrNotCardOwner      = errors.New("you do not own this card")
	ErrInvalidCategory   = errors.New("invalid category")
	ErrTitleTooLong      = errors.New("title must be 100 characters or less")
	ErrInvalidGridSize   = errors.New("invalid grid size")
	ErrInvalidHeaderText = errors.New("invalid header text")
	ErrNoSpaceForFree    = errors.New("no space available for free space")
)

type CardService struct {
	db                  DB
	notificationService NotificationServiceInterface
}

var editTitleSuffixPattern = regexp.MustCompile(`\s+\(Edit(?: \d+)?\)$`)

func NewCardService(db DB) *CardService {
	return &CardService{db: db}
}

type FinalizeParams struct {
	VisibleToFriends *bool // Optional; if nil, keeps current value (default true for new cards)
}

type CloneParams struct {
	Year         *int
	Title        *string
	Category     *string
	GridSize     int
	HeaderText   string
	HasFreeSpace *bool
}

type CloneResult struct {
	Card               *models.BingoCard
	TruncatedItemCount int
}

type EditFinalizedCardParams struct {
	Title         *string
	ShuffleLayout bool
	ResetProgress bool
}

func resolveCloneHasFreeSpace(sourceHasFreeSpace bool, override *bool) bool {
	if override == nil {
		return sourceHasFreeSpace
	}
	return *override
}

func mapBingoCardsUniqueViolationToCardExistsError(pgErr *pgconn.PgError, title *string) error {
	if pgErr == nil || pgErr.Code != "23505" {
		return nil
	}

	switch pgErr.ConstraintName {
	case "idx_bingo_cards_user_year_null_title":
		return ErrCardAlreadyExists
	case "idx_bingo_cards_user_year_title":
		return ErrCardTitleExists
	}

	if title == nil || strings.TrimSpace(*title) == "" {
		return ErrCardAlreadyExists
	}
	return ErrCardTitleExists
}

func withEditSuffix(base string, n int) string {
	base = strings.TrimSpace(base)
	base = editTitleSuffixPattern.ReplaceAllString(base, "")
	suffix := " (Edit)"
	if n > 1 {
		suffix = fmt.Sprintf(" (Edit %d)", n)
	}
	maxLen := 100
	suffixRunes := utf8.RuneCountInString(suffix)
	if suffixRunes >= maxLen {
		r := []rune(suffix)
		return string(r[:maxLen])
	}
	allowed := maxLen - suffixRunes
	baseRunes := []rune(base)
	if len(baseRunes) > allowed {
		base = string(baseRunes[:allowed])
		base = strings.TrimRight(base, " ")
	}
	if base == "" {
		base = "Card"
		baseRunes = []rune(base)
		if len(baseRunes) > allowed {
			base = string(baseRunes[:allowed])
		}
	}
	return base + suffix
}
