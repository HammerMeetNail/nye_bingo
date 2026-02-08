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
	"github.com/HammerMeetNail/yearofbingo/internal/httpx"
	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/middleware"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
	"github.com/HammerMeetNail/yearofbingo/internal/services/ai"
	"github.com/HammerMeetNail/yearofbingo/internal/services/billing"
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
	billing.SetGlobalFeatureSwitches(billing.FeatureEntitlements{
		Templates:         cfg.Billing.FeatureTemplatesEnabled,
		EditAfterFinalize: cfg.Billing.FeatureEditAfterFinalizeEnabled,
		AIEnhancements:    cfg.Billing.FeatureAIEnhancementsEnabled,
	})

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
	billingStore := billing.NewStore(dbAdapter)
	stripeClient := billing.NewStripeHTTPClientWithAPIBase(cfg.Billing.StripeSecretKey, cfg.Billing.StripeAPIBaseURL)
	billingService := billing.NewService(cfg.Billing, cfg.Email.BaseURL, billingStore, stripeClient)
	templateService := services.NewTemplateService(dbAdapter, cardService)

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
	billingHandler := handlers.NewBillingHandler(billingService)
	templatesHandler := handlers.NewTemplateHandler(templateService)
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

	cleanupCancel := startNotificationBackgroundJobs(notificationService, logger)
	defer cleanupCancel()
	reminderCancel := startReminderBackgroundJobs(reminderService, logger, os.LookupEnv)
	defer reminderCancel()

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService, userService, apiTokenService)
	csrfMiddleware := middleware.NewCSRFMiddleware(cfg.Server.Secure)
	securityHeaders := middleware.NewSecurityHeaders(cfg.Server.Secure)
	cacheControl := middleware.NewCacheControl()
	compress := middleware.NewCompress()
	requestLogger := middleware.NewRequestLogger(logger)
	trustedProxyChecker, err := httpx.NewTrustedProxyChecker(cfg.Server.TrustedProxyCIDRs)
	if err != nil {
		return fmt.Errorf("parsing TRUSTED_PROXY_CIDRS: %w", err)
	}
	trustedProxyHeaders := middleware.NewTrustedProxyHeaders(trustedProxyChecker)
	maxBodySize := middleware.NewMaxBodySize(1 << 20) // 1MiB for JSON APIs

	rateLimiters := buildRouteRateLimiters(cfg, logger, redisDB.Client, os.LookupEnv)

	// Helper middlewares for API token scope enforcement
	requireRead := authMiddleware.RequireScope(models.ScopeRead)
	requireWrite := authMiddleware.RequireScope(models.ScopeWrite)
	requireSession := authMiddleware.RequireSession

	// Set up router
	mux := http.NewServeMux()
	registerAPIRoutes(mux, &apiRouteHandlers{
		healthHandler:       healthHandler,
		csrfMiddleware:      csrfMiddleware,
		authHandler:         authHandler,
		providerAuthHandler: providerAuthHandler,
		accountHandler:      accountHandler,
		apiTokenHandler:     apiTokenHandler,
		cardHandler:         cardHandler,
		templatesHandler:    templatesHandler,
		suggestionHandler:   suggestionHandler,
		friendHandler:       friendHandler,
		blockHandler:        blockHandler,
		inviteHandler:       inviteHandler,
		notificationHandler: notificationHandler,
		reminderHandler:     reminderHandler,
		reactionHandler:     reactionHandler,
		supportHandler:      supportHandler,
		aiHandler:           aiHandler,
		billingHandler:      billingHandler,
	}, &apiRouteMiddleware{
		requireRead:                requireRead,
		requireWrite:               requireWrite,
		requireSession:             requireSession,
		authLoginIPLimiter:         rateLimiters.authLoginIPLimiter,
		authLoginEmailLimiter:      rateLimiters.authLoginEmailLimiter,
		authRegisterIPLimiter:      rateLimiters.authRegisterIPLimiter,
		authRegisterEmailLimiter:   rateLimiters.authRegisterEmailLimiter,
		authEmailFlowIPLimiter:     rateLimiters.authEmailFlowIPLimiter,
		authEmailFlowEmailLimiter:  rateLimiters.authEmailFlowEmailLimiter,
		authResetPasswordIPLimiter: rateLimiters.authResetPasswordIPLimiter,
		aiRateLimiter:              rateLimiters.aiRateLimiter,
		aiPremiumRateLimiter:       rateLimiters.aiPremiumRateLimiter,
		redeemLimiter:              rateLimiters.redeemLimiter,
	})

	registerWebRoutes(mux, &webRouteHandlers{
		pageHandler:           pageHandler,
		reminderPublicHandler: reminderPublicHandler,
		ogImageHandler:        ogImageHandler,
		shareOGImageHandler:   shareOGImageHandler,
		sharePublicHandler:    sharePublicHandler,
	}, requireSession)

	handler := buildMiddlewareChain(
		mux,
		newMiddlewareChain(
			authMiddleware,
			maxBodySize,
			csrfMiddleware,
			cacheControl,
			compress,
			securityHeaders,
			requestLogger,
			trustedProxyHeaders,
		),
	)

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
