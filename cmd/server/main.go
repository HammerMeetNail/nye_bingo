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

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db, redisDB)
	authHandler := handlers.NewAuthHandler(userService, authService, emailService, cfg.Server.Secure)
	cardHandler := handlers.NewCardHandler(cardService)
	suggestionHandler := handlers.NewSuggestionHandler(suggestionService)
	friendHandler := handlers.NewFriendHandler(friendService, cardService)
	reactionHandler := handlers.NewReactionHandler(reactionService)
	pageHandler, err := handlers.NewPageHandler("web/templates")
	if err != nil {
		return fmt.Errorf("loading templates: %w", err)
	}

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService)
	csrfMiddleware := middleware.NewCSRFMiddleware(cfg.Server.Secure)
	securityHeaders := middleware.NewSecurityHeaders(cfg.Server.Secure)
	cacheControl := middleware.NewCacheControl()
	compress := middleware.NewCompress()
	requestLogger := middleware.NewRequestLogger(logger)

	// Set up router
	mux := http.NewServeMux()

	// Health endpoints (no auth, no rate limit)
	mux.HandleFunc("GET /health", healthHandler.Health)
	mux.HandleFunc("GET /ready", healthHandler.Ready)
	mux.HandleFunc("GET /live", healthHandler.Live)

	// CSRF token endpoint
	mux.HandleFunc("GET /api/csrf", csrfMiddleware.GetToken)

	// Auth endpoints
	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/auth/me", authHandler.Me)
	mux.HandleFunc("POST /api/auth/password", authHandler.ChangePassword)
	mux.HandleFunc("POST /api/auth/verify-email", authHandler.VerifyEmail)
	mux.HandleFunc("POST /api/auth/resend-verification", authHandler.ResendVerification)
	mux.HandleFunc("POST /api/auth/magic-link", authHandler.MagicLink)
	mux.HandleFunc("GET /api/auth/magic-link/verify", authHandler.MagicLinkVerify)
	mux.HandleFunc("POST /api/auth/forgot-password", authHandler.ForgotPassword)
	mux.HandleFunc("POST /api/auth/reset-password", authHandler.ResetPassword)
	mux.HandleFunc("PUT /api/auth/searchable", authHandler.UpdateSearchable)

	// Card endpoints
	mux.HandleFunc("POST /api/cards", cardHandler.Create)
	mux.HandleFunc("GET /api/cards", cardHandler.List)
	mux.HandleFunc("GET /api/cards/archive", cardHandler.Archive)
	mux.HandleFunc("GET /api/cards/categories", cardHandler.GetCategories)
	mux.HandleFunc("GET /api/cards/export", cardHandler.ListExportable)
	mux.HandleFunc("POST /api/cards/import", cardHandler.Import)
	mux.HandleFunc("PUT /api/cards/visibility/bulk", cardHandler.BulkUpdateVisibility)
	mux.HandleFunc("DELETE /api/cards/bulk", cardHandler.BulkDelete)
	mux.HandleFunc("PUT /api/cards/archive/bulk", cardHandler.BulkUpdateArchive)
	mux.HandleFunc("GET /api/cards/{id}", cardHandler.Get)
	mux.HandleFunc("DELETE /api/cards/{id}", cardHandler.Delete)
	mux.HandleFunc("GET /api/cards/{id}/stats", cardHandler.Stats)
	mux.HandleFunc("PUT /api/cards/{id}/meta", cardHandler.UpdateMeta)
	mux.HandleFunc("PUT /api/cards/{id}/visibility", cardHandler.UpdateVisibility)
	mux.HandleFunc("POST /api/cards/{id}/items", cardHandler.AddItem)
	mux.HandleFunc("PUT /api/cards/{id}/items/{pos}", cardHandler.UpdateItem)
	mux.HandleFunc("DELETE /api/cards/{id}/items/{pos}", cardHandler.RemoveItem)
	mux.HandleFunc("POST /api/cards/{id}/shuffle", cardHandler.Shuffle)
	mux.HandleFunc("POST /api/cards/{id}/finalize", cardHandler.Finalize)
	mux.HandleFunc("PUT /api/cards/{id}/items/{pos}/complete", cardHandler.CompleteItem)
	mux.HandleFunc("PUT /api/cards/{id}/items/{pos}/uncomplete", cardHandler.UncompleteItem)
	mux.HandleFunc("PUT /api/cards/{id}/items/{pos}/notes", cardHandler.UpdateNotes)

	// Suggestion endpoints
	mux.HandleFunc("GET /api/suggestions", suggestionHandler.GetAll)
	mux.HandleFunc("GET /api/suggestions/categories", suggestionHandler.GetCategories)

	// Friend endpoints
	mux.HandleFunc("GET /api/friends", friendHandler.List)
	mux.HandleFunc("GET /api/friends/search", friendHandler.Search)
	mux.HandleFunc("POST /api/friends/request", friendHandler.SendRequest)
	mux.HandleFunc("PUT /api/friends/{id}/accept", friendHandler.AcceptRequest)
	mux.HandleFunc("PUT /api/friends/{id}/reject", friendHandler.RejectRequest)
	mux.HandleFunc("DELETE /api/friends/{id}", friendHandler.Remove)
	mux.HandleFunc("DELETE /api/friends/{id}/cancel", friendHandler.CancelRequest)
	mux.HandleFunc("GET /api/friends/{id}/card", friendHandler.GetFriendCard)
	mux.HandleFunc("GET /api/friends/{id}/cards", friendHandler.GetFriendCards)

	// Reaction endpoints
	mux.HandleFunc("POST /api/items/{id}/react", reactionHandler.AddReaction)
	mux.HandleFunc("DELETE /api/items/{id}/react", reactionHandler.RemoveReaction)
	mux.HandleFunc("GET /api/items/{id}/reactions", reactionHandler.GetReactions)
	mux.HandleFunc("GET /api/reactions/emojis", reactionHandler.GetAllowedEmojis)

	// Static files
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// SPA route - serve index.html for the root path
	// Hash-based routing (#home, #login, etc.) is handled client-side
	mux.HandleFunc("GET /{$}", pageHandler.Index)

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
