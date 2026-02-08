package main

import (
	"net/http"

	"github.com/HammerMeetNail/yearofbingo/internal/handlers"
	"github.com/HammerMeetNail/yearofbingo/internal/middleware"
)

type apiRouteHandlers struct {
	healthHandler       *handlers.HealthHandler
	csrfMiddleware      *middleware.CSRFMiddleware
	authHandler         *handlers.AuthHandler
	providerAuthHandler *handlers.ProviderAuthHandler
	accountHandler      *handlers.AccountHandler
	apiTokenHandler     *handlers.ApiTokenHandler
	cardHandler         *handlers.CardHandler
	templatesHandler    *handlers.TemplateHandler
	suggestionHandler   *handlers.SuggestionHandler
	friendHandler       *handlers.FriendHandler
	blockHandler        *handlers.BlockHandler
	inviteHandler       *handlers.FriendInviteHandler
	notificationHandler *handlers.NotificationHandler
	reminderHandler     *handlers.ReminderHandler
	reactionHandler     *handlers.ReactionHandler
	supportHandler      *handlers.SupportHandler
	aiHandler           *handlers.AIHandler
	billingHandler      *handlers.BillingHandler
}

type apiRouteMiddleware struct {
	requireRead    func(http.Handler) http.Handler
	requireWrite   func(http.Handler) http.Handler
	requireSession func(http.Handler) http.Handler

	authLoginIPLimiter       *middleware.RateLimiter
	authLoginEmailLimiter    *middleware.RateLimiter
	authRegisterIPLimiter    *middleware.RateLimiter
	authRegisterEmailLimiter *middleware.RateLimiter

	authEmailFlowIPLimiter     *middleware.RateLimiter
	authEmailFlowEmailLimiter  *middleware.RateLimiter
	authResetPasswordIPLimiter *middleware.RateLimiter

	aiRateLimiter        *middleware.RateLimiter
	aiPremiumRateLimiter *middleware.RateLimiter
	redeemLimiter        *middleware.RateLimiter
}

func registerAPIRoutes(mux *http.ServeMux, handlers *apiRouteHandlers, middleware *apiRouteMiddleware) {
	registerHealthRoutes(mux, handlers.healthHandler)
	registerCSRFRoutes(mux, handlers.csrfMiddleware, middleware.requireSession)
	registerAuthRoutes(mux, handlers.authHandler, handlers.providerAuthHandler, middleware)
	registerAccountRoutes(mux, handlers.accountHandler, middleware.requireSession)
	registerTokenRoutes(mux, handlers.apiTokenHandler, middleware.requireSession)
	registerCardRoutes(mux, handlers.cardHandler, middleware.requireRead, middleware.requireWrite, middleware.requireSession)
	registerTemplateRoutes(mux, handlers.templatesHandler, middleware.requireRead, middleware.requireWrite)
	registerSuggestionRoutes(mux, handlers.suggestionHandler)
	registerFriendRoutes(
		mux,
		handlers.friendHandler,
		handlers.blockHandler,
		handlers.inviteHandler,
		handlers.notificationHandler,
		middleware.requireSession,
	)
	registerReminderRoutes(mux, handlers.reminderHandler, middleware.requireSession)
	registerReactionRoutes(mux, handlers.reactionHandler, middleware.requireSession)
	registerSupportRoutes(mux, handlers.supportHandler, middleware.requireSession)
	registerAIRoutes(
		mux,
		handlers.aiHandler,
		middleware.requireSession,
		middleware.aiRateLimiter,
		middleware.aiPremiumRateLimiter,
	)
	registerBillingRoutes(
		mux,
		handlers.billingHandler,
		middleware.requireRead,
		middleware.requireSession,
		middleware.redeemLimiter,
	)
}

func registerHealthRoutes(mux *http.ServeMux, healthHandler *handlers.HealthHandler) {
	mux.HandleFunc("GET /health", healthHandler.Health)
	mux.HandleFunc("GET /ready", healthHandler.Ready)
	mux.HandleFunc("GET /live", healthHandler.Live)
}

func registerCSRFRoutes(
	mux *http.ServeMux,
	csrfMiddleware *middleware.CSRFMiddleware,
	requireSession func(http.Handler) http.Handler,
) {
	mux.Handle("GET /api/csrf", requireSession(http.HandlerFunc(csrfMiddleware.GetToken)))
}

func registerAuthRoutes(
	mux *http.ServeMux,
	authHandler *handlers.AuthHandler,
	providerAuthHandler *handlers.ProviderAuthHandler,
	middleware *apiRouteMiddleware,
) {
	mux.Handle("POST /api/auth/register", middleware.requireSession(middleware.authRegisterIPLimiter.Middleware(middleware.authRegisterEmailLimiter.Middleware(http.HandlerFunc(authHandler.Register)))))
	mux.Handle("POST /api/auth/login", middleware.requireSession(middleware.authLoginIPLimiter.Middleware(middleware.authLoginEmailLimiter.Middleware(http.HandlerFunc(authHandler.Login)))))
	mux.Handle("POST /api/auth/logout", middleware.requireSession(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("GET /api/auth/me", middleware.requireRead(http.HandlerFunc(authHandler.Me)))
	mux.Handle("POST /api/auth/password", middleware.requireSession(http.HandlerFunc(authHandler.ChangePassword)))
	mux.Handle("POST /api/auth/verify-email", middleware.requireSession(middleware.authEmailFlowIPLimiter.Middleware(http.HandlerFunc(authHandler.VerifyEmail))))
	mux.Handle("POST /api/auth/resend-verification", middleware.requireSession(middleware.authEmailFlowIPLimiter.Middleware(http.HandlerFunc(authHandler.ResendVerification))))
	mux.Handle("POST /api/auth/magic-link", middleware.requireSession(middleware.authEmailFlowIPLimiter.Middleware(middleware.authEmailFlowEmailLimiter.Middleware(http.HandlerFunc(authHandler.MagicLink)))))
	mux.Handle("GET /api/auth/magic-link/verify", middleware.requireSession(http.HandlerFunc(authHandler.MagicLinkVerify)))
	mux.Handle("POST /api/auth/forgot-password", middleware.requireSession(middleware.authEmailFlowIPLimiter.Middleware(middleware.authEmailFlowEmailLimiter.Middleware(http.HandlerFunc(authHandler.ForgotPassword)))))
	mux.Handle("POST /api/auth/reset-password", middleware.requireSession(middleware.authResetPasswordIPLimiter.Middleware(http.HandlerFunc(authHandler.ResetPassword))))
	mux.Handle("PUT /api/auth/searchable", middleware.requireSession(http.HandlerFunc(authHandler.UpdateSearchable)))
	mux.Handle("GET /api/auth/{provider}/start", middleware.requireSession(http.HandlerFunc(providerAuthHandler.ProviderStart)))
	mux.Handle("GET /api/auth/{provider}/callback", middleware.requireSession(http.HandlerFunc(providerAuthHandler.ProviderCallback)))
	mux.Handle("POST /api/auth/{provider}/complete", middleware.requireSession(http.HandlerFunc(providerAuthHandler.ProviderComplete)))
}

func registerAccountRoutes(
	mux *http.ServeMux,
	accountHandler *handlers.AccountHandler,
	requireSession func(http.Handler) http.Handler,
) {
	mux.Handle("GET /api/account/export", requireSession(http.HandlerFunc(accountHandler.Export)))
	mux.Handle("DELETE /api/account", requireSession(http.HandlerFunc(accountHandler.Delete)))
}

func registerTokenRoutes(
	mux *http.ServeMux,
	apiTokenHandler *handlers.ApiTokenHandler,
	requireSession func(http.Handler) http.Handler,
) {
	mux.Handle("GET /api/tokens", requireSession(http.HandlerFunc(apiTokenHandler.List)))
	mux.Handle("POST /api/tokens", requireSession(http.HandlerFunc(apiTokenHandler.Create)))
	mux.Handle("DELETE /api/tokens/{id}", requireSession(http.HandlerFunc(apiTokenHandler.Delete)))
	mux.Handle("DELETE /api/tokens", requireSession(http.HandlerFunc(apiTokenHandler.DeleteAll)))
}

func registerCardRoutes(
	mux *http.ServeMux,
	cardHandler *handlers.CardHandler,
	requireRead func(http.Handler) http.Handler,
	requireWrite func(http.Handler) http.Handler,
	requireSession func(http.Handler) http.Handler,
) {
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
	mux.Handle("POST /api/cards/{id}/edit", requireWrite(http.HandlerFunc(cardHandler.EditFinalized)))
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
}

func registerTemplateRoutes(
	mux *http.ServeMux,
	templatesHandler *handlers.TemplateHandler,
	requireRead func(http.Handler) http.Handler,
	requireWrite func(http.Handler) http.Handler,
) {
	mux.Handle("GET /api/templates", requireRead(http.HandlerFunc(templatesHandler.ListTemplates)))
	mux.Handle("GET /api/templates/{id}", requireRead(http.HandlerFunc(templatesHandler.GetTemplate)))
	mux.Handle("POST /api/templates", requireWrite(http.HandlerFunc(templatesHandler.CreateTemplate)))
	mux.Handle("PUT /api/templates/{id}", requireWrite(http.HandlerFunc(templatesHandler.UpdateTemplate)))
	mux.Handle("PUT /api/templates/{id}/items", requireWrite(http.HandlerFunc(templatesHandler.ReplaceTemplateItems)))
	mux.Handle("DELETE /api/templates/{id}", requireWrite(http.HandlerFunc(templatesHandler.DeleteTemplate)))
	mux.Handle("POST /api/templates/{id}/create-card", requireWrite(http.HandlerFunc(templatesHandler.CreateCardFromTemplate)))
	mux.Handle("POST /api/cards/{id}/rollover", requireWrite(http.HandlerFunc(templatesHandler.RolloverCard)))
}

func registerSuggestionRoutes(mux *http.ServeMux, suggestionHandler *handlers.SuggestionHandler) {
	mux.Handle("GET /api/suggestions", http.HandlerFunc(suggestionHandler.GetAll))
	mux.Handle("GET /api/suggestions/categories", http.HandlerFunc(suggestionHandler.GetCategories))
}

func registerFriendRoutes(
	mux *http.ServeMux,
	friendHandler *handlers.FriendHandler,
	blockHandler *handlers.BlockHandler,
	inviteHandler *handlers.FriendInviteHandler,
	notificationHandler *handlers.NotificationHandler,
	requireSession func(http.Handler) http.Handler,
) {
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
}

func registerReminderRoutes(
	mux *http.ServeMux,
	reminderHandler *handlers.ReminderHandler,
	requireSession func(http.Handler) http.Handler,
) {
	mux.Handle("GET /api/reminders/settings", requireSession(http.HandlerFunc(reminderHandler.GetSettings)))
	mux.Handle("PUT /api/reminders/settings", requireSession(http.HandlerFunc(reminderHandler.UpdateSettings)))
	mux.Handle("GET /api/reminders/cards", requireSession(http.HandlerFunc(reminderHandler.ListCards)))
	mux.Handle("PUT /api/reminders/cards/{cardId}", requireSession(http.HandlerFunc(reminderHandler.UpsertCardCheckin)))
	mux.Handle("DELETE /api/reminders/cards/{cardId}", requireSession(http.HandlerFunc(reminderHandler.DeleteCardCheckin)))
	mux.Handle("GET /api/reminders/goals", requireSession(http.HandlerFunc(reminderHandler.ListGoals)))
	mux.Handle("POST /api/reminders/goals", requireSession(http.HandlerFunc(reminderHandler.UpsertGoalReminder)))
	mux.Handle("DELETE /api/reminders/goals/{id}", requireSession(http.HandlerFunc(reminderHandler.DeleteGoalReminder)))
	mux.Handle("POST /api/reminders/test", requireSession(http.HandlerFunc(reminderHandler.SendTest)))
}

func registerReactionRoutes(
	mux *http.ServeMux,
	reactionHandler *handlers.ReactionHandler,
	requireSession func(http.Handler) http.Handler,
) {
	mux.Handle("POST /api/items/{id}/react", requireSession(http.HandlerFunc(reactionHandler.AddReaction)))
	mux.Handle("DELETE /api/items/{id}/react", requireSession(http.HandlerFunc(reactionHandler.RemoveReaction)))
	mux.Handle("GET /api/items/{id}/reactions", requireSession(http.HandlerFunc(reactionHandler.GetReactions)))
	mux.Handle("GET /api/reactions/emojis", requireSession(http.HandlerFunc(reactionHandler.GetAllowedEmojis)))
}

func registerSupportRoutes(
	mux *http.ServeMux,
	supportHandler *handlers.SupportHandler,
	requireSession func(http.Handler) http.Handler,
) {
	mux.Handle("POST /api/support", requireSession(http.HandlerFunc(supportHandler.Submit)))
}

func registerAIRoutes(
	mux *http.ServeMux,
	aiHandler *handlers.AIHandler,
	requireSession func(http.Handler) http.Handler,
	aiRateLimiter *middleware.RateLimiter,
	aiPremiumRateLimiter *middleware.RateLimiter,
) {
	mux.Handle("POST /api/ai/generate", requireSession(aiRateLimiter.Middleware(http.HandlerFunc(aiHandler.Generate))))
	mux.Handle("POST /api/ai/guide", requireSession(aiRateLimiter.Middleware(http.HandlerFunc(aiHandler.Guide))))
	mux.Handle("GET /api/ai/premium/status", requireSession(http.HandlerFunc(aiHandler.PremiumStatus)))
	mux.Handle("POST /api/ai/assist", requireSession(aiPremiumRateLimiter.Middleware(http.HandlerFunc(aiHandler.Assist))))
	mux.Handle("POST /api/ai/regenerate", requireSession(aiPremiumRateLimiter.Middleware(http.HandlerFunc(aiHandler.Regenerate))))
	mux.Handle("POST /api/ai/fill-empty", requireSession(aiPremiumRateLimiter.Middleware(http.HandlerFunc(aiHandler.FillEmpty))))
}

func registerBillingRoutes(
	mux *http.ServeMux,
	billingHandler *handlers.BillingHandler,
	requireRead func(http.Handler) http.Handler,
	requireSession func(http.Handler) http.Handler,
	redeemLimiter *middleware.RateLimiter,
) {
	mux.Handle("GET /api/billing/status", requireSession(requireRead(http.HandlerFunc(billingHandler.Status))))
	mux.Handle("POST /api/billing/checkout", requireSession(http.HandlerFunc(billingHandler.Checkout)))
	mux.Handle("POST /api/billing/checkout/subscription", requireSession(http.HandlerFunc(billingHandler.CheckoutSubscription)))
	mux.Handle("POST /api/billing/checkout/lifetime", requireSession(http.HandlerFunc(billingHandler.CheckoutLifetime)))
	mux.Handle("POST /api/billing/checkout/tip", requireSession(http.HandlerFunc(billingHandler.CheckoutTip)))
	mux.Handle("POST /api/billing/portal", requireSession(http.HandlerFunc(billingHandler.Portal)))
	mux.Handle("POST /api/billing/redeem", requireSession(redeemLimiter.Middleware(http.HandlerFunc(billingHandler.Redeem))))
	mux.Handle("POST /api/billing/webhook", http.HandlerFunc(billingHandler.Webhook))
}
