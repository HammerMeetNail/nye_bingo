package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/database"
	"github.com/HammerMeetNail/yearofbingo/internal/handlers"
	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/middleware"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
	"github.com/HammerMeetNail/yearofbingo/internal/services/ai"
)

func main() {
	if err := run(); err != nil {
		logging.Error("Application error", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
}

func run() error {
	// Initialize logger
	logger := logging.New()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.Server.Debug {
		logger.SetLevel(logging.LevelDebug)
		logging.SetDefaultLevel(logging.LevelDebug)
		logger.Debug("Debug logging enabled", map[string]interface{}{
			"max_chars": cfg.Server.DebugMaxChars,
			"env":       cfg.Server.Environment,
		})
	}

	logger.Info("Starting Year of Bingo server...")

	// Connect to PostgreSQL
	logger.Info("Connecting to PostgreSQL", map[string]interface{}{
		"host": cfg.Database.Host,
		"port": cfg.Database.Port,
	})
	db, err := database.NewPostgresDB(cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer db.Close()
	logger.Info("Connected to PostgreSQL")

	// Run migrations
	logger.Info("Running database migrations...")
	migrator, err := database.NewMigrator(cfg.Database.DSN(), "migrations")
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	if err := migrator.Up(); err != nil {
		_ = migrator.Close()
		return fmt.Errorf("running migrations: %w", err)
	}
	_ = migrator.Close()
	logger.Info("Migrations completed")

	// Connect to Redis
	logger.Info("Connecting to Redis", map[string]interface{}{
		"addr": cfg.Redis.Addr(),
	})
	redisDB, err := database.NewRedisDB(cfg.Redis.Addr(), cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return fmt.Errorf("connecting to redis: %w", err)
	}
	defer func() { _ = redisDB.Close() }()
	logger.Info("Connected to Redis")

	// Initialize services
	dbAdapter := services.NewPoolAdapter(db.Pool)
	redisAdapter := services.NewRedisAdapter(redisDB.Client)

	userService := services.NewUserService(dbAdapter)
	authService := services.NewAuthService(dbAdapter, redisAdapter)
	providerAuthService := services.NewProviderAuthService(dbAdapter)
	emailService := services.NewEmailService(&cfg.Email, dbAdapter)
	cardService := services.NewCardService(dbAdapter)
	suggestionService := services.NewSuggestionService(dbAdapter)
	friendService := services.NewFriendService(dbAdapter)
	reactionService := services.NewReactionService(dbAdapter, friendService)
	apiTokenService := services.NewApiTokenService(dbAdapter)
	blockService := services.NewBlockService(dbAdapter)
	inviteService := services.NewFriendInviteService(dbAdapter)
	notificationService := services.NewNotificationService(dbAdapter, emailService, cfg.Email.BaseURL)
	reminderService := services.NewReminderService(dbAdapter, emailService, cfg.Email.BaseURL)
	accountService := services.NewAccountService(dbAdapter)
	aiService := ai.NewService(cfg, dbAdapter)

	oauthProviders := map[services.Provider]services.OAuthProvider{}
	if cfg.OAuth.Google.Enabled {
		googleProvider, err := services.NewOIDCProvider(context.Background(), services.OIDCProviderConfig{
			Provider:     services.ProviderGoogle,
			ClientID:     cfg.OAuth.Google.ClientID,
			ClientSecret: cfg.OAuth.Google.ClientSecret,
			RedirectURL:  cfg.OAuth.Google.RedirectURL,
			IssuerURL:    cfg.OAuth.Google.IssuerURL,
			Scopes:       cfg.OAuth.Google.Scopes,
		})
		if err != nil {
			return fmt.Errorf("initializing google oidc provider: %w", err)
		}
		oauthProviders[services.ProviderGoogle] = googleProvider
	}

	cardService.SetNotificationService(notificationService)
	friendService.SetNotificationService(notificationService)
	inviteService.SetNotificationService(notificationService)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db, redisDB)
	authHandler := handlers.NewAuthHandler(userService, authService, emailService, cfg.Server.Secure)
	providerAuthHandler := handlers.NewProviderAuthHandler(providerAuthService, authService, redisAdapter, oauthProviders, cfg.Server.Secure)
	cardHandler := handlers.NewCardHandler(cardService)
	suggestionHandler := handlers.NewSuggestionHandler(suggestionService)
	friendHandler := handlers.NewFriendHandler(friendService, cardService)
	reactionHandler := handlers.NewReactionHandler(reactionService)
	supportHandler := handlers.NewSupportHandler(emailService, redisDB.Client)
	apiTokenHandler := handlers.NewApiTokenHandler(apiTokenService)
	blockHandler := handlers.NewBlockHandler(blockService)
	inviteHandler := handlers.NewFriendInviteHandler(inviteService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	reminderHandler := handlers.NewReminderHandler(reminderService)
	reminderPublicHandler := handlers.NewReminderPublicHandler(reminderService)
	aiHandler := handlers.NewAIHandler(aiService)
	accountHandler := handlers.NewAccountHandler(accountService, authService, cfg.Server.Secure)
	pageHandler, err := handlers.NewPageHandler("web/templates", handlers.PageOAuthConfig{
		GoogleEnabled: cfg.OAuth.Google.Enabled,
	})
	if err != nil {
		return fmt.Errorf("loading templates: %w", err)
	}
	sharePublicHandler, err := handlers.NewSharePublicHandler("web/templates", cardService)
	if err != nil {
		return fmt.Errorf("loading share templates: %w", err)
	}
	shareOGImageHandler := handlers.NewShareOGImageHandler(cardService)
	ogImageHandler := handlers.NewOGImageHandler()

	if err := notificationService.CleanupOld(context.Background()); err != nil {
		logger.Warn("Notification cleanup failed", map[string]interface{}{"error": err.Error()})
	}
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	notificationService.SetAsyncContext(cleanupCtx)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-cleanupCtx.Done():
				return
			case <-ticker.C:
				if err := notificationService.CleanupOld(context.Background()); err != nil {
					logger.Warn("Notification cleanup failed", map[string]interface{}{"error": err.Error()})
				}
			}
		}
	}()

	if err := reminderService.CleanupOld(context.Background()); err != nil {
		logger.Warn("Reminder cleanup failed", map[string]interface{}{"error": err.Error()})
	}
	reminderCtx, reminderCancel := context.WithCancel(context.Background())
	go func() {
		interval := resolveRemindersPollInterval(logger, os.LookupEnv)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-reminderCtx.Done():
				return
			case <-ticker.C:
				if _, err := reminderService.RunDue(context.Background(), time.Now(), 50); err != nil {
					logger.Warn("Reminder runner failed", map[string]interface{}{"error": err.Error()})
				}
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-reminderCtx.Done():
				return
			case <-ticker.C:
				if err := reminderService.CleanupOld(context.Background()); err != nil {
					logger.Warn("Reminder cleanup failed", map[string]interface{}{"error": err.Error()})
				}
			}
		}
	}()

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService, userService, apiTokenService)
	csrfMiddleware := middleware.NewCSRFMiddleware(cfg.Server.Secure)
	securityHeaders := middleware.NewSecurityHeaders(cfg.Server.Secure)
	cacheControl := middleware.NewCacheControl()
	compress := middleware.NewCompress()
	requestLogger := middleware.NewRequestLogger(logger)

	// AI Rate Limit configuration
	aiRateLimit := resolveAIRateLimit(cfg, logger, os.LookupEnv)

	aiRateLimiter := middleware.NewRateLimiter(redisDB.Client, aiRateLimit, 1*time.Hour, "ratelimit:ai:", func(r *http.Request) string {
		user := handlers.GetUserFromContext(r.Context())
		if user != nil {
			return user.ID.String()
		}
		return ""
	}, false)

	// Helper middlewares for API token scope enforcement
	requireRead := authMiddleware.RequireScope(models.ScopeRead)
	requireWrite := authMiddleware.RequireScope(models.ScopeWrite)
	requireSession := authMiddleware.RequireSession

	// Set up router
	mux := http.NewServeMux()

	// Health endpoints (no auth, no rate limit)
	mux.HandleFunc("GET /health", healthHandler.Health)
	mux.HandleFunc("GET /ready", healthHandler.Ready)
	mux.HandleFunc("GET /live", healthHandler.Live)

	// CSRF token endpoint
	mux.Handle("GET /api/csrf", requireSession(http.HandlerFunc(csrfMiddleware.GetToken)))

	// Auth endpoints
	mux.Handle("POST /api/auth/register", requireSession(http.HandlerFunc(authHandler.Register)))
	mux.Handle("POST /api/auth/login", requireSession(http.HandlerFunc(authHandler.Login)))
	mux.Handle("POST /api/auth/logout", requireSession(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("GET /api/auth/me", requireRead(http.HandlerFunc(authHandler.Me)))
	mux.Handle("POST /api/auth/password", requireSession(http.HandlerFunc(authHandler.ChangePassword)))
	mux.Handle("POST /api/auth/verify-email", requireSession(http.HandlerFunc(authHandler.VerifyEmail)))
	mux.Handle("POST /api/auth/resend-verification", requireSession(http.HandlerFunc(authHandler.ResendVerification)))
	mux.Handle("POST /api/auth/magic-link", requireSession(http.HandlerFunc(authHandler.MagicLink)))
	mux.Handle("GET /api/auth/magic-link/verify", requireSession(http.HandlerFunc(authHandler.MagicLinkVerify)))
	mux.Handle("POST /api/auth/forgot-password", requireSession(http.HandlerFunc(authHandler.ForgotPassword)))
	mux.Handle("POST /api/auth/reset-password", requireSession(http.HandlerFunc(authHandler.ResetPassword)))
	mux.Handle("PUT /api/auth/searchable", requireSession(http.HandlerFunc(authHandler.UpdateSearchable)))
	mux.Handle("GET /api/auth/{provider}/start", requireSession(http.HandlerFunc(providerAuthHandler.ProviderStart)))
	mux.Handle("GET /api/auth/{provider}/callback", requireSession(http.HandlerFunc(providerAuthHandler.ProviderCallback)))
	mux.Handle("POST /api/auth/{provider}/complete", requireSession(http.HandlerFunc(providerAuthHandler.ProviderComplete)))

	// Account endpoints
	mux.Handle("GET /api/account/export", requireSession(http.HandlerFunc(accountHandler.Export)))
	mux.Handle("DELETE /api/account", requireSession(http.HandlerFunc(accountHandler.Delete)))

	// API Token endpoints
	mux.Handle("GET /api/tokens", requireSession(http.HandlerFunc(apiTokenHandler.List)))
	mux.Handle("POST /api/tokens", requireSession(http.HandlerFunc(apiTokenHandler.Create)))
	mux.Handle("DELETE /api/tokens/{id}", requireSession(http.HandlerFunc(apiTokenHandler.Delete)))
	mux.Handle("DELETE /api/tokens", requireSession(http.HandlerFunc(apiTokenHandler.DeleteAll)))

	// Card endpoints
	mux.Handle("POST /api/cards", requireWrite(http.HandlerFunc(cardHandler.Create)))
	mux.Handle("GET /api/cards", requireRead(http.HandlerFunc(cardHandler.List)))
	mux.Handle("GET /api/cards/archive", requireSession(http.HandlerFunc(cardHandler.Archive)))
	mux.Handle("GET /api/cards/categories", requireRead(http.HandlerFunc(cardHandler.GetCategories)))
	mux.Handle("GET /api/cards/export", requireSession(http.HandlerFunc(cardHandler.ListExportable)))
	mux.Handle("POST /api/cards/import", requireSession(http.HandlerFunc(cardHandler.Import)))
	mux.Handle("PUT /api/cards/visibility/bulk", requireSession(http.HandlerFunc(cardHandler.BulkUpdateVisibility)))
	mux.Handle("DELETE /api/cards/bulk", requireSession(http.HandlerFunc(cardHandler.BulkDelete)))
	mux.Handle("PUT /api/cards/archive/bulk", requireSession(http.HandlerFunc(cardHandler.BulkUpdateArchive)))
	mux.Handle("GET /api/cards/{id}", requireRead(http.HandlerFunc(cardHandler.Get)))
	mux.Handle("DELETE /api/cards/{id}", requireSession(http.HandlerFunc(cardHandler.Delete)))
	mux.Handle("GET /api/cards/{id}/stats", requireRead(http.HandlerFunc(cardHandler.Stats)))
	mux.Handle("PUT /api/cards/{id}/meta", requireSession(http.HandlerFunc(cardHandler.UpdateMeta)))
	mux.Handle("PUT /api/cards/{id}/visibility", requireSession(http.HandlerFunc(cardHandler.UpdateVisibility)))
	mux.Handle("PUT /api/cards/{id}/config", requireWrite(http.HandlerFunc(cardHandler.UpdateConfig)))
	mux.Handle("POST /api/cards/{id}/clone", requireWrite(http.HandlerFunc(cardHandler.Clone)))
	mux.Handle("POST /api/cards/{id}/items", requireWrite(http.HandlerFunc(cardHandler.AddItem)))
	mux.Handle("PUT /api/cards/{id}/items/{pos}", requireWrite(http.HandlerFunc(cardHandler.UpdateItem)))
	mux.Handle("DELETE /api/cards/{id}/items/{pos}", requireWrite(http.HandlerFunc(cardHandler.RemoveItem)))
	mux.Handle("POST /api/cards/{id}/shuffle", requireWrite(http.HandlerFunc(cardHandler.Shuffle)))
	mux.Handle("POST /api/cards/{id}/swap", requireWrite(http.HandlerFunc(cardHandler.SwapItems)))
	mux.Handle("POST /api/cards/{id}/finalize", requireWrite(http.HandlerFunc(cardHandler.Finalize)))
	mux.Handle("POST /api/cards/{id}/share", requireSession(http.HandlerFunc(cardHandler.CreateShare)))
	mux.Handle("GET /api/cards/{id}/share", requireSession(http.HandlerFunc(cardHandler.GetShareStatus)))
	mux.Handle("DELETE /api/cards/{id}/share", requireSession(http.HandlerFunc(cardHandler.RevokeShare)))
	mux.Handle("PUT /api/cards/{id}/items/{pos}/complete", requireWrite(http.HandlerFunc(cardHandler.CompleteItem)))
	mux.Handle("PUT /api/cards/{id}/items/{pos}/uncomplete", requireWrite(http.HandlerFunc(cardHandler.UncompleteItem)))
	mux.Handle("PUT /api/cards/{id}/items/{pos}/notes", requireWrite(http.HandlerFunc(cardHandler.UpdateNotes)))
	mux.Handle("GET /api/share/{token}", http.HandlerFunc(cardHandler.GetSharedCard))

	// Suggestion endpoints
	mux.Handle("GET /api/suggestions", http.HandlerFunc(suggestionHandler.GetAll))
	mux.Handle("GET /api/suggestions/categories", http.HandlerFunc(suggestionHandler.GetCategories))

	// Friend endpoints
	mux.Handle("GET /api/friends", requireSession(http.HandlerFunc(friendHandler.List)))
	mux.Handle("GET /api/friends/search", requireSession(http.HandlerFunc(friendHandler.Search)))
	mux.Handle("POST /api/friends/requests", requireSession(http.HandlerFunc(friendHandler.SendRequest)))
	mux.Handle("PUT /api/friends/requests/{id}/accept", requireSession(http.HandlerFunc(friendHandler.AcceptRequest)))
	mux.Handle("PUT /api/friends/requests/{id}/reject", requireSession(http.HandlerFunc(friendHandler.RejectRequest)))
	mux.Handle("DELETE /api/friends/{id}", requireSession(http.HandlerFunc(friendHandler.Remove)))
	mux.Handle("DELETE /api/friends/requests/{id}/cancel", requireSession(http.HandlerFunc(friendHandler.CancelRequest)))
	mux.Handle("GET /api/friends/{id}/card", requireSession(http.HandlerFunc(friendHandler.GetFriendCard)))
	mux.Handle("GET /api/friends/{id}/cards", requireSession(http.HandlerFunc(friendHandler.GetFriendCards)))
	mux.Handle("POST /api/blocks", requireSession(http.HandlerFunc(blockHandler.Block)))
	mux.Handle("DELETE /api/blocks/{id}", requireSession(http.HandlerFunc(blockHandler.Unblock)))
	mux.Handle("GET /api/blocks", requireSession(http.HandlerFunc(blockHandler.List)))
	mux.Handle("POST /api/friends/invites", requireSession(http.HandlerFunc(inviteHandler.Create)))
	mux.Handle("GET /api/friends/invites", requireSession(http.HandlerFunc(inviteHandler.List)))
	mux.Handle("DELETE /api/friends/invites/{id}/revoke", requireSession(http.HandlerFunc(inviteHandler.Revoke)))
	mux.Handle("POST /api/friends/invites/accept", requireSession(http.HandlerFunc(inviteHandler.Accept)))
	mux.Handle("GET /api/notifications", requireSession(http.HandlerFunc(notificationHandler.List)))
	mux.Handle("POST /api/notifications/{id}/read", requireSession(http.HandlerFunc(notificationHandler.MarkRead)))
	mux.Handle("POST /api/notifications/read-all", requireSession(http.HandlerFunc(notificationHandler.MarkAllRead)))
	mux.Handle("DELETE /api/notifications/{id}", requireSession(http.HandlerFunc(notificationHandler.Delete)))
	mux.Handle("DELETE /api/notifications", requireSession(http.HandlerFunc(notificationHandler.DeleteAll)))
	mux.Handle("GET /api/notifications/unread-count", requireSession(http.HandlerFunc(notificationHandler.UnreadCount)))
	mux.Handle("GET /api/notifications/settings", requireSession(http.HandlerFunc(notificationHandler.GetSettings)))
	mux.Handle("PUT /api/notifications/settings", requireSession(http.HandlerFunc(notificationHandler.UpdateSettings)))

	// Reminder endpoints
	mux.Handle("GET /api/reminders/settings", requireSession(http.HandlerFunc(reminderHandler.GetSettings)))
	mux.Handle("PUT /api/reminders/settings", requireSession(http.HandlerFunc(reminderHandler.UpdateSettings)))
	mux.Handle("GET /api/reminders/cards", requireSession(http.HandlerFunc(reminderHandler.ListCards)))
	mux.Handle("PUT /api/reminders/cards/{cardId}", requireSession(http.HandlerFunc(reminderHandler.UpsertCardCheckin)))
	mux.Handle("DELETE /api/reminders/cards/{cardId}", requireSession(http.HandlerFunc(reminderHandler.DeleteCardCheckin)))
	mux.Handle("GET /api/reminders/goals", requireSession(http.HandlerFunc(reminderHandler.ListGoals)))
	mux.Handle("POST /api/reminders/goals", requireSession(http.HandlerFunc(reminderHandler.UpsertGoalReminder)))
	mux.Handle("DELETE /api/reminders/goals/{id}", requireSession(http.HandlerFunc(reminderHandler.DeleteGoalReminder)))
	mux.Handle("POST /api/reminders/test", requireSession(http.HandlerFunc(reminderHandler.SendTest)))

	// Reaction endpoints
	mux.Handle("POST /api/items/{id}/react", requireSession(http.HandlerFunc(reactionHandler.AddReaction)))
	mux.Handle("DELETE /api/items/{id}/react", requireSession(http.HandlerFunc(reactionHandler.RemoveReaction)))
	mux.Handle("GET /api/items/{id}/reactions", requireSession(http.HandlerFunc(reactionHandler.GetReactions)))
	mux.Handle("GET /api/reactions/emojis", requireSession(http.HandlerFunc(reactionHandler.GetAllowedEmojis)))

	// Support endpoint
	mux.Handle("POST /api/support", requireSession(http.HandlerFunc(supportHandler.Submit)))

	// AI endpoint
	mux.Handle("POST /api/ai/generate", requireSession(aiRateLimiter.Middleware(http.HandlerFunc(aiHandler.Generate))))
	mux.Handle("POST /api/ai/guide", requireSession(aiRateLimiter.Middleware(http.HandlerFunc(aiHandler.Guide))))

	// Static files
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// Reminder public endpoints
	mux.Handle("GET /r/img/{token}", http.HandlerFunc(reminderPublicHandler.ServeImage))
	mux.Handle("GET /r/unsubscribe", http.HandlerFunc(reminderPublicHandler.UnsubscribeConfirm))
	mux.Handle("POST /r/unsubscribe", http.HandlerFunc(reminderPublicHandler.UnsubscribeSubmit))

	// OpenGraph images (public)
	mux.Handle("GET /og/default.png", http.HandlerFunc(ogImageHandler.Default))
	mux.Handle("GET /og/share/{token}", http.HandlerFunc(shareOGImageHandler.Serve))

	// Public share landing page (for link unfurls)
	mux.Handle("GET /s/{token}", http.HandlerFunc(sharePublicHandler.Serve))

	// API Docs redirect
	mux.Handle("GET /api/docs", http.RedirectHandler("/static/swagger/index.html", http.StatusFound))

	// SPA route - serve index.html for the root path
	// Hash-based routing (#home, #login, etc.) is handled client-side
	mux.Handle("GET /{$}", requireSession(http.HandlerFunc(pageHandler.Index)))

	// Build middleware chain (order matters: outermost first)
	var handler http.Handler = mux
	handler = authMiddleware.Authenticate(handler)
	handler = csrfMiddleware.Protect(handler)
	handler = cacheControl.Apply(handler)
	handler = compress.Apply(handler)
	handler = securityHeaders.Apply(handler)
	handler = requestLogger.Apply(handler)

	// Create server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:        addr,
		Handler:     handler,
		ReadTimeout: 15 * time.Second,
		// AI generation calls can legitimately take >15s; keep a higher write timeout
		// so the frontend gets a JSON error/response instead of a dropped connection.
		WriteTimeout: 95 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("Server is shutting down...")
		cleanupCancel()
		reminderCancel()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Could not gracefully shutdown the server", map[string]interface{}{
				"error": err.Error(),
			})
		}
		close(done)
	}()

	logger.Info("Server listening", map[string]interface{}{
		"addr": addr,
	})
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	<-done
	logger.Info("Server stopped")
	return nil
}

func resolveAIRateLimit(cfg *config.Config, logger *logging.Logger, lookupEnv func(string) (string, bool)) int64 {
	aiRateLimit := int64(10)
	if cfg.Server.Environment == "development" {
		aiRateLimit = 100
		logger.Info("Using development AI rate limit", map[string]interface{}{"limit": aiRateLimit})
	}
	if v, ok := lookupEnv("AI_RATE_LIMIT"); ok && v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			aiRateLimit = parsed
			logger.Info("Using AI rate limit from env", map[string]interface{}{"limit": aiRateLimit})
		} else {
			logger.Warn("Invalid AI_RATE_LIMIT; using default", map[string]interface{}{
				"value": v,
				"limit": aiRateLimit,
			})
		}
	}
	return aiRateLimit
}

func resolveRemindersPollInterval(logger *logging.Logger, lookupEnv func(string) (string, bool)) time.Duration {
	interval := time.Minute
	if value, ok := lookupEnv("REMINDERS_POLL_INTERVAL"); ok && value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			logger.Warn("Invalid REMINDERS_POLL_INTERVAL; using default", map[string]interface{}{
				"value":   value,
				"default": interval.String(),
			})
		} else {
			interval = parsed
			logger.Info("Using reminders poll interval from env", map[string]interface{}{"interval": interval.String()})
		}
	}
	return interval
}
