# Plan: Increase Test Coverage via Interface-Based Dependency Injection

## Executive Summary

This plan outlines a safe, incremental refactoring to introduce interfaces between handlers and services, enabling comprehensive unit testing without database dependencies. The current architecture has handlers tightly coupled to concrete service types, resulting in only ~31% handler coverage and ~9% service coverage.

**Goal**: Achieve 70%+ handler test coverage by enabling mock injection.

**Risk Mitigation**: Every step maintains backward compatibility. Existing production code continues to work unchanged.

---

## Current Architecture Analysis

### Dependency Graph
```
Handlers (concrete dependencies)
├── AuthHandler
│   ├── *services.UserService
│   ├── *services.AuthService
│   └── *services.EmailService
├── CardHandler
│   └── *services.CardService
├── SuggestionHandler
│   └── *services.SuggestionService
├── FriendHandler
│   ├── *services.FriendService
│   └── *services.CardService
├── ReactionHandler
│   └── *services.ReactionService
├── SupportHandler
│   ├── *services.EmailService
│   └── *redis.Client
└── HealthHandler (already uses interfaces ✓)
    ├── HealthChecker (interface)
    └── HealthChecker (interface)
```

### Cross-Service Dependencies
- `ReactionService` depends on `*FriendService` (for `IsFriend` check)
- `FriendHandler` uses both `FriendService` and `CardService`

### Existing Interfaces (Good Patterns to Follow)
1. `HealthChecker` in `handlers/health.go` - handlers depend on interface
2. `EmailProvider` in `services/email.go` - service uses interface internally

---

## Migration Strategy: "Parallel Interface Pattern"

The safest approach is to:
1. Define interfaces that exactly match existing concrete method signatures
2. Update handler struct fields from concrete types to interfaces
3. Concrete types already satisfy interfaces (no changes to services)
4. Production wiring in `main.go` remains unchanged
5. Tests can now inject mocks

**Why This Is Safe:**
- Go's implicit interface satisfaction means existing concrete types automatically implement the new interfaces
- No changes to service implementations
- No changes to `main.go` wiring
- Compile-time verification ensures interfaces match

---

## Phase 1: Create Interface Definitions

### File: `internal/services/interfaces.go` (NEW)

Create a single file containing all service interfaces. This keeps interfaces co-located with services and makes them easy to find.

```go
package services

import (
    "context"

    "github.com/google/uuid"

    "github.com/HammerMeetNail/yearofbingo/internal/models"
)

// UserServiceInterface defines the contract for user operations
type UserServiceInterface interface {
    Create(ctx context.Context, params models.CreateUserParams) (*models.User, error)
    GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
    GetByEmail(ctx context.Context, email string) (*models.User, error)
    UpdatePassword(ctx context.Context, userID uuid.UUID, newPasswordHash string) error
    MarkEmailVerified(ctx context.Context, userID uuid.UUID) error
    UpdateSearchable(ctx context.Context, userID uuid.UUID, searchable bool) error
}

// AuthServiceInterface defines the contract for authentication operations
type AuthServiceInterface interface {
    HashPassword(password string) (string, error)
    VerifyPassword(hash, password string) bool
    GenerateSessionToken() (token string, hash string, err error)
    CreateSession(ctx context.Context, userID uuid.UUID) (token string, err error)
    ValidateSession(ctx context.Context, token string) (*models.User, error)
    DeleteSession(ctx context.Context, token string) error
    DeleteAllUserSessions(ctx context.Context, userID uuid.UUID) error
}

// CardServiceInterface defines the contract for bingo card operations
type CardServiceInterface interface {
    Create(ctx context.Context, params models.CreateCardParams) (*models.BingoCard, error)
    GetByID(ctx context.Context, cardID uuid.UUID) (*models.BingoCard, error)
    ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error)
    Delete(ctx context.Context, userID, cardID uuid.UUID) error
    AddItem(ctx context.Context, userID uuid.UUID, params models.AddItemParams) (*models.BingoItem, error)
    UpdateItem(ctx context.Context, userID, cardID uuid.UUID, position int, params models.UpdateItemParams) (*models.BingoItem, error)
    RemoveItem(ctx context.Context, userID, cardID uuid.UUID, position int) error
    SwapItems(ctx context.Context, userID, cardID uuid.UUID, pos1, pos2 int) error
    Shuffle(ctx context.Context, userID, cardID uuid.UUID) (*models.BingoCard, error)
    Finalize(ctx context.Context, userID, cardID uuid.UUID, params *FinalizeParams) (*models.BingoCard, error)
    CompleteItem(ctx context.Context, userID, cardID uuid.UUID, position int, params models.CompleteItemParams) (*models.BingoItem, error)
    UncompleteItem(ctx context.Context, userID, cardID uuid.UUID, position int) (*models.BingoItem, error)
    UpdateItemNotes(ctx context.Context, userID, cardID uuid.UUID, position int, notes, proofURL *string) (*models.BingoItem, error)
    UpdateMeta(ctx context.Context, userID, cardID uuid.UUID, params models.UpdateCardMetaParams) (*models.BingoCard, error)
    UpdateVisibility(ctx context.Context, userID, cardID uuid.UUID, visibleToFriends bool) (*models.BingoCard, error)
    BulkUpdateVisibility(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, visibleToFriends bool) (int, error)
    BulkDelete(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID) (int, error)
    BulkUpdateArchive(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, isArchived bool) (int, error)
    GetArchive(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error)
    GetStats(ctx context.Context, userID, cardID uuid.UUID) (*models.CardStats, error)
    CheckForConflict(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error)
    Import(ctx context.Context, params models.ImportCardParams) (*models.BingoCard, error)
}

// SuggestionServiceInterface defines the contract for suggestion operations
type SuggestionServiceInterface interface {
    GetAll(ctx context.Context) ([]*models.Suggestion, error)
    GetByCategory(ctx context.Context, category string) ([]*models.Suggestion, error)
    GetCategories(ctx context.Context) ([]string, error)
    GetGroupedByCategory(ctx context.Context) ([]SuggestionsByCategory, error)
}

// FriendServiceInterface defines the contract for friend operations
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

// ReactionServiceInterface defines the contract for reaction operations
type ReactionServiceInterface interface {
    AddReaction(ctx context.Context, userID, itemID uuid.UUID, emoji string) (*models.Reaction, error)
    RemoveReaction(ctx context.Context, userID, itemID uuid.UUID) error
    GetReactionsForItem(ctx context.Context, itemID uuid.UUID) ([]models.ReactionWithUser, error)
    GetReactionSummaryForItem(ctx context.Context, itemID uuid.UUID) ([]models.ReactionSummary, error)
    GetReactionsForCard(ctx context.Context, cardID uuid.UUID) (map[uuid.UUID][]models.ReactionWithUser, error)
    GetUserReactionForItem(ctx context.Context, userID, itemID uuid.UUID) (*models.Reaction, error)
}

// EmailServiceInterface defines the contract for email operations
type EmailServiceInterface interface {
    SendVerificationEmail(ctx context.Context, userID uuid.UUID, email string) error
    VerifyEmail(ctx context.Context, token string) error
    SendMagicLinkEmail(ctx context.Context, email string) error
    VerifyMagicLink(ctx context.Context, token string) (string, error)
    SendPasswordResetEmail(ctx context.Context, userID uuid.UUID, email string) error
    VerifyPasswordResetToken(ctx context.Context, token string) (uuid.UUID, error)
    MarkPasswordResetUsed(ctx context.Context, token string) error
    SendSupportEmail(ctx context.Context, fromEmail, category, message string, userID string) error
}
```

### Verification Step

After creating `interfaces.go`, run:
```bash
go build ./...
```

This verifies the interfaces compile. No other changes yet.

---

## Phase 2: Update Handlers to Use Interfaces

Update each handler to depend on interfaces instead of concrete types. **The constructor signatures change, but `main.go` still passes concrete types which satisfy the interfaces.**

### 2.1 Update `internal/handlers/auth.go`

**Before:**
```go
type AuthHandler struct {
    userService  *services.UserService
    authService  *services.AuthService
    emailService *services.EmailService
    secure       bool
}

func NewAuthHandler(userService *services.UserService, authService *services.AuthService, emailService *services.EmailService, secure bool) *AuthHandler {
```

**After:**
```go
type AuthHandler struct {
    userService  services.UserServiceInterface
    authService  services.AuthServiceInterface
    emailService services.EmailServiceInterface
    secure       bool
}

func NewAuthHandler(userService services.UserServiceInterface, authService services.AuthServiceInterface, emailService services.EmailServiceInterface, secure bool) *AuthHandler {
```

### 2.2 Update `internal/handlers/card.go`

**Before:**
```go
type CardHandler struct {
    cardService *services.CardService
}

func NewCardHandler(cardService *services.CardService) *CardHandler {
```

**After:**
```go
type CardHandler struct {
    cardService services.CardServiceInterface
}

func NewCardHandler(cardService services.CardServiceInterface) *CardHandler {
```

### 2.3 Update `internal/handlers/suggestion.go`

**Before:**
```go
type SuggestionHandler struct {
    suggestionService *services.SuggestionService
}

func NewSuggestionHandler(suggestionService *services.SuggestionService) *SuggestionHandler {
```

**After:**
```go
type SuggestionHandler struct {
    suggestionService services.SuggestionServiceInterface
}

func NewSuggestionHandler(suggestionService services.SuggestionServiceInterface) *SuggestionHandler {
```

### 2.4 Update `internal/handlers/friend.go`

**Before:**
```go
type FriendHandler struct {
    friendService *services.FriendService
    cardService   *services.CardService
}

func NewFriendHandler(friendService *services.FriendService, cardService *services.CardService) *FriendHandler {
```

**After:**
```go
type FriendHandler struct {
    friendService services.FriendServiceInterface
    cardService   services.CardServiceInterface
}

func NewFriendHandler(friendService services.FriendServiceInterface, cardService services.CardServiceInterface) *FriendHandler {
```

### 2.5 Update `internal/handlers/reaction.go`

**Before:**
```go
type ReactionHandler struct {
    reactionService *services.ReactionService
}

func NewReactionHandler(reactionService *services.ReactionService) *ReactionHandler {
```

**After:**
```go
type ReactionHandler struct {
    reactionService services.ReactionServiceInterface
}

func NewReactionHandler(reactionService services.ReactionServiceInterface) *ReactionHandler {
```

### 2.6 Update `internal/handlers/support.go`

**Before:**
```go
type SupportHandler struct {
    emailService *services.EmailService
    redis        *redis.Client
}

func NewSupportHandler(emailService *services.EmailService, redisClient *redis.Client) *SupportHandler {
```

**After:**
```go
type SupportHandler struct {
    emailService services.EmailServiceInterface
    redis        *redis.Client
}

func NewSupportHandler(emailService services.EmailServiceInterface, redisClient *redis.Client) *SupportHandler {
```

### Verification Step

After updating all handlers:
```bash
go build ./...
go test ./...
```

**Why This Works:**
- `*services.UserService` implicitly implements `services.UserServiceInterface`
- Go's interface satisfaction is structural, not nominal
- `main.go` passes `*services.UserService` to `NewAuthHandler`, which accepts `services.UserServiceInterface`
- No runtime behavior change

---

## Phase 3: Update ReactionService Cross-Dependency (Optional)

The `ReactionService` depends on `*FriendService` for the `IsFriend` check. For full testability, this should also use an interface.

### 3.1 Update `internal/services/reaction.go`

**Before:**
```go
type ReactionService struct {
    db            *pgxpool.Pool
    friendService *FriendService
}

func NewReactionService(db *pgxpool.Pool, friendService *FriendService) *ReactionService {
```

**After:**
```go
// FriendChecker is the subset of FriendService needed by ReactionService
type FriendChecker interface {
    IsFriend(ctx context.Context, userID, otherUserID uuid.UUID) (bool, error)
}

type ReactionService struct {
    db            *pgxpool.Pool
    friendService FriendChecker
}

func NewReactionService(db *pgxpool.Pool, friendService FriendChecker) *ReactionService {
```

**Note:** We use a smaller interface (`FriendChecker`) rather than the full `FriendServiceInterface` because `ReactionService` only needs `IsFriend`. This follows the Interface Segregation Principle.

---

## Phase 4: Create Mock Implementations for Tests

### File: `internal/handlers/mocks_test.go` (NEW)

Create mock implementations in the test file. These are only compiled during testing.

```go
package handlers

import (
    "context"

    "github.com/google/uuid"

    "github.com/HammerMeetNail/yearofbingo/internal/models"
    "github.com/HammerMeetNail/yearofbingo/internal/services"
)

// MockUserService implements services.UserServiceInterface for testing
type MockUserService struct {
    CreateFunc           func(ctx context.Context, params models.CreateUserParams) (*models.User, error)
    GetByIDFunc          func(ctx context.Context, id uuid.UUID) (*models.User, error)
    GetByEmailFunc       func(ctx context.Context, email string) (*models.User, error)
    UpdatePasswordFunc   func(ctx context.Context, userID uuid.UUID, newPasswordHash string) error
    MarkEmailVerifiedFunc func(ctx context.Context, userID uuid.UUID) error
    UpdateSearchableFunc func(ctx context.Context, userID uuid.UUID, searchable bool) error
}

func (m *MockUserService) Create(ctx context.Context, params models.CreateUserParams) (*models.User, error) {
    if m.CreateFunc != nil {
        return m.CreateFunc(ctx, params)
    }
    return &models.User{ID: uuid.New(), Email: params.Email, Username: params.Username}, nil
}

func (m *MockUserService) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
    if m.GetByIDFunc != nil {
        return m.GetByIDFunc(ctx, id)
    }
    return nil, services.ErrUserNotFound
}

func (m *MockUserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
    if m.GetByEmailFunc != nil {
        return m.GetByEmailFunc(ctx, email)
    }
    return nil, services.ErrUserNotFound
}

func (m *MockUserService) UpdatePassword(ctx context.Context, userID uuid.UUID, newPasswordHash string) error {
    if m.UpdatePasswordFunc != nil {
        return m.UpdatePasswordFunc(ctx, userID, newPasswordHash)
    }
    return nil
}

func (m *MockUserService) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
    if m.MarkEmailVerifiedFunc != nil {
        return m.MarkEmailVerifiedFunc(ctx, userID)
    }
    return nil
}

func (m *MockUserService) UpdateSearchable(ctx context.Context, userID uuid.UUID, searchable bool) error {
    if m.UpdateSearchableFunc != nil {
        return m.UpdateSearchableFunc(ctx, userID, searchable)
    }
    return nil
}

// MockAuthService implements services.AuthServiceInterface for testing
type MockAuthService struct {
    HashPasswordFunc          func(password string) (string, error)
    VerifyPasswordFunc        func(hash, password string) bool
    GenerateSessionTokenFunc  func() (token string, hash string, err error)
    CreateSessionFunc         func(ctx context.Context, userID uuid.UUID) (token string, err error)
    ValidateSessionFunc       func(ctx context.Context, token string) (*models.User, error)
    DeleteSessionFunc         func(ctx context.Context, token string) error
    DeleteAllUserSessionsFunc func(ctx context.Context, userID uuid.UUID) error
}

func (m *MockAuthService) HashPassword(password string) (string, error) {
    if m.HashPasswordFunc != nil {
        return m.HashPasswordFunc(password)
    }
    return "hashed_" + password, nil
}

func (m *MockAuthService) VerifyPassword(hash, password string) bool {
    if m.VerifyPasswordFunc != nil {
        return m.VerifyPasswordFunc(hash, password)
    }
    return hash == "hashed_"+password
}

func (m *MockAuthService) GenerateSessionToken() (string, string, error) {
    if m.GenerateSessionTokenFunc != nil {
        return m.GenerateSessionTokenFunc()
    }
    return "token123", "hash123", nil
}

func (m *MockAuthService) CreateSession(ctx context.Context, userID uuid.UUID) (string, error) {
    if m.CreateSessionFunc != nil {
        return m.CreateSessionFunc(ctx, userID)
    }
    return "session_token", nil
}

func (m *MockAuthService) ValidateSession(ctx context.Context, token string) (*models.User, error) {
    if m.ValidateSessionFunc != nil {
        return m.ValidateSessionFunc(ctx, token)
    }
    return nil, services.ErrSessionNotFound
}

func (m *MockAuthService) DeleteSession(ctx context.Context, token string) error {
    if m.DeleteSessionFunc != nil {
        return m.DeleteSessionFunc(ctx, token)
    }
    return nil
}

func (m *MockAuthService) DeleteAllUserSessions(ctx context.Context, userID uuid.UUID) error {
    if m.DeleteAllUserSessionsFunc != nil {
        return m.DeleteAllUserSessionsFunc(ctx, userID)
    }
    return nil
}

// MockEmailService implements services.EmailServiceInterface for testing
type MockEmailService struct {
    SendVerificationEmailFunc   func(ctx context.Context, userID uuid.UUID, email string) error
    VerifyEmailFunc             func(ctx context.Context, token string) error
    SendMagicLinkEmailFunc      func(ctx context.Context, email string) error
    VerifyMagicLinkFunc         func(ctx context.Context, token string) (string, error)
    SendPasswordResetEmailFunc  func(ctx context.Context, userID uuid.UUID, email string) error
    VerifyPasswordResetTokenFunc func(ctx context.Context, token string) (uuid.UUID, error)
    MarkPasswordResetUsedFunc   func(ctx context.Context, token string) error
    SendSupportEmailFunc        func(ctx context.Context, fromEmail, category, message string, userID string) error
}

func (m *MockEmailService) SendVerificationEmail(ctx context.Context, userID uuid.UUID, email string) error {
    if m.SendVerificationEmailFunc != nil {
        return m.SendVerificationEmailFunc(ctx, userID, email)
    }
    return nil
}

func (m *MockEmailService) VerifyEmail(ctx context.Context, token string) error {
    if m.VerifyEmailFunc != nil {
        return m.VerifyEmailFunc(ctx, token)
    }
    return nil
}

func (m *MockEmailService) SendMagicLinkEmail(ctx context.Context, email string) error {
    if m.SendMagicLinkEmailFunc != nil {
        return m.SendMagicLinkEmailFunc(ctx, email)
    }
    return nil
}

func (m *MockEmailService) VerifyMagicLink(ctx context.Context, token string) (string, error) {
    if m.VerifyMagicLinkFunc != nil {
        return m.VerifyMagicLinkFunc(ctx, token)
    }
    return "", nil
}

func (m *MockEmailService) SendPasswordResetEmail(ctx context.Context, userID uuid.UUID, email string) error {
    if m.SendPasswordResetEmailFunc != nil {
        return m.SendPasswordResetEmailFunc(ctx, userID, email)
    }
    return nil
}

func (m *MockEmailService) VerifyPasswordResetToken(ctx context.Context, token string) (uuid.UUID, error) {
    if m.VerifyPasswordResetTokenFunc != nil {
        return m.VerifyPasswordResetTokenFunc(ctx, token)
    }
    return uuid.Nil, nil
}

func (m *MockEmailService) MarkPasswordResetUsed(ctx context.Context, token string) error {
    if m.MarkPasswordResetUsedFunc != nil {
        return m.MarkPasswordResetUsedFunc(ctx, token)
    }
    return nil
}

func (m *MockEmailService) SendSupportEmail(ctx context.Context, fromEmail, category, message string, userID string) error {
    if m.SendSupportEmailFunc != nil {
        return m.SendSupportEmailFunc(ctx, fromEmail, category, message, userID)
    }
    return nil
}

// MockCardService implements services.CardServiceInterface for testing
type MockCardService struct {
    CreateFunc              func(ctx context.Context, params models.CreateCardParams) (*models.BingoCard, error)
    GetByIDFunc             func(ctx context.Context, cardID uuid.UUID) (*models.BingoCard, error)
    ListByUserFunc          func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error)
    DeleteFunc              func(ctx context.Context, userID, cardID uuid.UUID) error
    AddItemFunc             func(ctx context.Context, userID uuid.UUID, params models.AddItemParams) (*models.BingoItem, error)
    UpdateItemFunc          func(ctx context.Context, userID, cardID uuid.UUID, position int, params models.UpdateItemParams) (*models.BingoItem, error)
    RemoveItemFunc          func(ctx context.Context, userID, cardID uuid.UUID, position int) error
    SwapItemsFunc           func(ctx context.Context, userID, cardID uuid.UUID, pos1, pos2 int) error
    ShuffleFunc             func(ctx context.Context, userID, cardID uuid.UUID) (*models.BingoCard, error)
    FinalizeFunc            func(ctx context.Context, userID, cardID uuid.UUID, params *services.FinalizeParams) (*models.BingoCard, error)
    CompleteItemFunc        func(ctx context.Context, userID, cardID uuid.UUID, position int, params models.CompleteItemParams) (*models.BingoItem, error)
    UncompleteItemFunc      func(ctx context.Context, userID, cardID uuid.UUID, position int) (*models.BingoItem, error)
    UpdateItemNotesFunc     func(ctx context.Context, userID, cardID uuid.UUID, position int, notes, proofURL *string) (*models.BingoItem, error)
    UpdateMetaFunc          func(ctx context.Context, userID, cardID uuid.UUID, params models.UpdateCardMetaParams) (*models.BingoCard, error)
    UpdateVisibilityFunc    func(ctx context.Context, userID, cardID uuid.UUID, visibleToFriends bool) (*models.BingoCard, error)
    BulkUpdateVisibilityFunc func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, visibleToFriends bool) (int, error)
    BulkDeleteFunc          func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID) (int, error)
    BulkUpdateArchiveFunc   func(ctx context.Context, userID uuid.UUID, cardIDs []uuid.UUID, isArchived bool) (int, error)
    GetArchiveFunc          func(ctx context.Context, userID uuid.UUID) ([]*models.BingoCard, error)
    GetStatsFunc            func(ctx context.Context, userID, cardID uuid.UUID) (*models.CardStats, error)
    CheckForConflictFunc    func(ctx context.Context, userID uuid.UUID, year int, title *string) (*models.BingoCard, error)
    ImportFunc              func(ctx context.Context, params models.ImportCardParams) (*models.BingoCard, error)
}

// Implement all CardServiceInterface methods...
// (Implementation similar to above - each method checks if the Func field is set)
```

**Note:** The full mock file will be ~400 lines. Each mock follows the same pattern:
1. Struct with `XxxFunc` function fields
2. Interface method calls the function field if set, otherwise returns sensible default

---

## Phase 5: Write Happy-Path Handler Tests

With mocks available, we can now test the full handler flow.

### Example: Testing Successful Registration

```go
func TestAuthHandler_Register_Success(t *testing.T) {
    userID := uuid.New()

    mockUser := &MockUserService{
        CreateFunc: func(ctx context.Context, params models.CreateUserParams) (*models.User, error) {
            return &models.User{
                ID:       userID,
                Email:    params.Email,
                Username: params.Username,
            }, nil
        },
    }

    mockAuth := &MockAuthService{
        HashPasswordFunc: func(password string) (string, error) {
            return "hashed_password", nil
        },
        CreateSessionFunc: func(ctx context.Context, uid uuid.UUID) (string, error) {
            return "session_token_123", nil
        },
    }

    mockEmail := &MockEmailService{} // SendVerificationEmail called in goroutine, will use default nil return

    handler := NewAuthHandler(mockUser, mockAuth, mockEmail, false)

    body := RegisterRequest{
        Email:    "test@example.com",
        Password: "SecurePass123",
        Username: "testuser",
    }
    bodyBytes, _ := json.Marshal(body)

    req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(bodyBytes))
    rr := httptest.NewRecorder()

    handler.Register(rr, req)

    if rr.Code != http.StatusCreated {
        t.Errorf("expected status 201, got %d", rr.Code)
    }

    var response AuthResponse
    json.Unmarshal(rr.Body.Bytes(), &response)

    if response.User == nil {
        t.Fatal("expected user in response")
    }
    if response.User.Email != "test@example.com" {
        t.Errorf("expected email test@example.com, got %s", response.User.Email)
    }

    // Verify session cookie was set
    cookies := rr.Result().Cookies()
    var found bool
    for _, c := range cookies {
        if c.Name == "session_token" && c.Value == "session_token_123" {
            found = true
        }
    }
    if !found {
        t.Error("expected session cookie to be set")
    }
}

func TestAuthHandler_Register_DuplicateEmail(t *testing.T) {
    mockUser := &MockUserService{
        CreateFunc: func(ctx context.Context, params models.CreateUserParams) (*models.User, error) {
            return nil, services.ErrEmailAlreadyExists
        },
    }
    mockAuth := &MockAuthService{}
    mockEmail := &MockEmailService{}

    handler := NewAuthHandler(mockUser, mockAuth, mockEmail, false)

    body := RegisterRequest{
        Email:    "existing@example.com",
        Password: "SecurePass123",
        Username: "newuser",
    }
    bodyBytes, _ := json.Marshal(body)

    req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBuffer(bodyBytes))
    rr := httptest.NewRecorder()

    handler.Register(rr, req)

    if rr.Code != http.StatusConflict {
        t.Errorf("expected status 409, got %d", rr.Code)
    }
}
```

---

## Implementation Checklist

### Phase 1: Interface Definitions
- [ ] Create `internal/services/interfaces.go`
- [ ] Define `UserServiceInterface`
- [ ] Define `AuthServiceInterface`
- [ ] Define `CardServiceInterface`
- [ ] Define `SuggestionServiceInterface`
- [ ] Define `FriendServiceInterface`
- [ ] Define `ReactionServiceInterface`
- [ ] Define `EmailServiceInterface`
- [ ] Run `go build ./...` to verify compilation

### Phase 2: Handler Updates
- [ ] Update `AuthHandler` struct and constructor
- [ ] Update `CardHandler` struct and constructor
- [ ] Update `SuggestionHandler` struct and constructor
- [ ] Update `FriendHandler` struct and constructor
- [ ] Update `ReactionHandler` struct and constructor
- [ ] Update `SupportHandler` struct and constructor
- [ ] Run `go build ./...` to verify compilation
- [ ] Run `go test ./...` to verify existing tests pass

### Phase 3: Cross-Service Dependency (Optional)
- [ ] Define `FriendChecker` interface in `services/reaction.go`
- [ ] Update `ReactionService` to use `FriendChecker`
- [ ] Run `go build ./...` to verify

### Phase 4: Mock Implementations
- [ ] Create `internal/handlers/mocks_test.go`
- [ ] Implement `MockUserService`
- [ ] Implement `MockAuthService`
- [ ] Implement `MockEmailService`
- [ ] Implement `MockCardService`
- [ ] Implement `MockSuggestionService`
- [ ] Implement `MockFriendService`
- [ ] Implement `MockReactionService`
- [ ] Run `go test ./...` to verify mocks compile

### Phase 5: Happy-Path Tests
- [ ] Add `TestAuthHandler_Register_Success`
- [ ] Add `TestAuthHandler_Register_DuplicateEmail`
- [ ] Add `TestAuthHandler_Register_DuplicateUsername`
- [ ] Add `TestAuthHandler_Login_Success`
- [ ] Add `TestAuthHandler_Login_InvalidPassword`
- [ ] Add `TestAuthHandler_Login_UserNotFound`
- [ ] Add `TestCardHandler_Create_Success`
- [ ] Add `TestCardHandler_Create_Conflict`
- [ ] Add `TestCardHandler_AddItem_Success`
- [ ] Add `TestCardHandler_AddItem_CardFull`
- [ ] Add `TestCardHandler_Finalize_Success`
- [ ] Add `TestFriendHandler_SendRequest_Success`
- [ ] Add `TestFriendHandler_SendRequest_AlreadyExists`
- [ ] (Continue for other handlers...)

### Verification
- [ ] Run full test suite: `go test ./... -cover`
- [ ] Verify handler coverage increased to 70%+
- [ ] Run in container: `./scripts/test.sh`
- [ ] Manual smoke test of critical flows

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Interface doesn't match service | Low | Build fails | Compile-time check catches this |
| Forgot to update a handler | Low | Build fails | Compile-time check |
| Mock behavior differs from real | Medium | False positives | Keep integration tests |
| Breaking change to main.go | Very Low | Runtime fail | No changes to main.go needed |

---

## Rollback Plan

If issues arise after deployment:
1. Revert the interface-related commits
2. Handlers return to using concrete types
3. No data migration needed
4. No configuration changes needed

The changes are purely structural refactoring with no behavioral changes to production code paths.

---

## Success Metrics

| Metric | Before | Target |
|--------|--------|--------|
| Handler test coverage | 30.9% | 70%+ |
| Service test coverage | 9.4% | 30%+ (via handler tests exercising mocks) |
| Number of handler tests | ~100 | ~200+ |
| Build time | No change expected | No change |
| Runtime performance | No change | No change |

---

## Timeline Estimate

| Phase | Effort |
|-------|--------|
| Phase 1: Interfaces | 1 hour |
| Phase 2: Handler updates | 1 hour |
| Phase 3: Cross-service (optional) | 30 min |
| Phase 4: Mocks | 2 hours |
| Phase 5: Tests | 4-6 hours |
| **Total** | **8-10 hours** |

---

## Appendix: Why Not Use a Mocking Library?

Go has mocking libraries like `gomock` and `testify/mock`. We're using hand-written mocks because:

1. **No external dependencies** - The project has minimal dependencies
2. **Explicit behavior** - Function fields make mock behavior obvious
3. **IDE support** - No code generation step needed
4. **Simpler debugging** - Stack traces point to real code

If the project grows significantly, consider migrating to `gomock` for automatic mock generation.
