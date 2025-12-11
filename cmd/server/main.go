package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/database"
	"github.com/HammerMeetNail/yearofbingo/internal/handlers"
	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/middleware"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
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
	userService := services.NewUserService(db.Pool)
	authService := services.NewAuthService(db.Pool, redisDB.Client)
	emailService := services.NewEmailService(&cfg.Email, db.Pool)
	cardService := services.NewCardService(db.Pool)
	suggestionService := services.NewSuggestionService(db.Pool)
	friendService := services.NewFriendService(db.Pool)
	reactionService := services.NewReactionService(db.Pool, friendService)
	apiTokenService := services.NewApiTokenService(db.Pool)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db, redisDB)
	authHandler := handlers.NewAuthHandler(userService, authService, emailService, cfg.Server.Secure)
	cardHandler := handlers.NewCardHandler(cardService)
	suggestionHandler := handlers.NewSuggestionHandler(suggestionService)
	friendHandler := handlers.NewFriendHandler(friendService, cardService)
	reactionHandler := handlers.NewReactionHandler(reactionService)
	supportHandler := handlers.NewSupportHandler(emailService, redisDB.Client)
	apiTokenHandler := handlers.NewApiTokenHandler(apiTokenService)
	pageHandler, err := handlers.NewPageHandler("web/templates")
	if err != nil {
		return fmt.Errorf("loading templates: %w", err)
	}

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService, userService, apiTokenService)
	csrfMiddleware := middleware.NewCSRFMiddleware(cfg.Server.Secure)
	securityHeaders := middleware.NewSecurityHeaders(cfg.Server.Secure)
	cacheControl := middleware.NewCacheControl()
	compress := middleware.NewCompress()
	requestLogger := middleware.NewRequestLogger(logger)

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
	mux.Handle("POST /api/cards/{id}/items", requireWrite(http.HandlerFunc(cardHandler.AddItem)))
	mux.Handle("PUT /api/cards/{id}/items/{pos}", requireWrite(http.HandlerFunc(cardHandler.UpdateItem)))
	mux.Handle("DELETE /api/cards/{id}/items/{pos}", requireWrite(http.HandlerFunc(cardHandler.RemoveItem)))
	mux.Handle("POST /api/cards/{id}/shuffle", requireWrite(http.HandlerFunc(cardHandler.Shuffle)))
	mux.Handle("POST /api/cards/{id}/swap", requireWrite(http.HandlerFunc(cardHandler.SwapItems)))
	mux.Handle("POST /api/cards/{id}/finalize", requireWrite(http.HandlerFunc(cardHandler.Finalize)))
	mux.Handle("PUT /api/cards/{id}/items/{pos}/complete", requireWrite(http.HandlerFunc(cardHandler.CompleteItem)))
	mux.Handle("PUT /api/cards/{id}/items/{pos}/uncomplete", requireWrite(http.HandlerFunc(cardHandler.UncompleteItem)))
	mux.Handle("PUT /api/cards/{id}/items/{pos}/notes", requireWrite(http.HandlerFunc(cardHandler.UpdateNotes)))

	// Suggestion endpoints
	mux.Handle("GET /api/suggestions", http.HandlerFunc(suggestionHandler.GetAll))
	mux.Handle("GET /api/suggestions/categories", http.HandlerFunc(suggestionHandler.GetCategories))

	// Friend endpoints
	mux.Handle("GET /api/friends", requireSession(http.HandlerFunc(friendHandler.List)))
	mux.Handle("GET /api/friends/search", requireSession(http.HandlerFunc(friendHandler.Search)))
	mux.Handle("POST /api/friends/request", requireSession(http.HandlerFunc(friendHandler.SendRequest)))
	mux.Handle("PUT /api/friends/{id}/accept", requireSession(http.HandlerFunc(friendHandler.AcceptRequest)))
	mux.Handle("PUT /api/friends/{id}/reject", requireSession(http.HandlerFunc(friendHandler.RejectRequest)))
	mux.Handle("DELETE /api/friends/{id}", requireSession(http.HandlerFunc(friendHandler.Remove)))
	mux.Handle("DELETE /api/friends/{id}/cancel", requireSession(http.HandlerFunc(friendHandler.CancelRequest)))
	mux.Handle("GET /api/friends/{id}/card", requireSession(http.HandlerFunc(friendHandler.GetFriendCard)))
	mux.Handle("GET /api/friends/{id}/cards", requireSession(http.HandlerFunc(friendHandler.GetFriendCards)))

	// Reaction endpoints
	mux.Handle("POST /api/items/{id}/react", requireSession(http.HandlerFunc(reactionHandler.AddReaction)))
	mux.Handle("DELETE /api/items/{id}/react", requireSession(http.HandlerFunc(reactionHandler.RemoveReaction)))
	mux.Handle("GET /api/items/{id}/reactions", requireSession(http.HandlerFunc(reactionHandler.GetReactions)))
	mux.Handle("GET /api/reactions/emojis", requireSession(http.HandlerFunc(reactionHandler.GetAllowedEmojis)))

	// Support endpoint
	mux.Handle("POST /api/support", requireSession(http.HandlerFunc(supportHandler.Submit)))

	// Static files
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

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
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("Server is shutting down...")

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
