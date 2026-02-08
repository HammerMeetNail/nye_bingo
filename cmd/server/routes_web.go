package main

import (
	"net/http"

	"github.com/HammerMeetNail/yearofbingo/internal/handlers"
)

type webRouteHandlers struct {
	pageHandler           *handlers.PageHandler
	reminderPublicHandler *handlers.ReminderPublicHandler
	ogImageHandler        *handlers.OGImageHandler
	shareOGImageHandler   *handlers.ShareOGImageHandler
	sharePublicHandler    *handlers.SharePublicHandler
}

func registerWebRoutes(
	mux *http.ServeMux,
	handlers *webRouteHandlers,
	requireSession func(http.Handler) http.Handler,
) {
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	mux.Handle("GET /r/img/{token}", http.HandlerFunc(handlers.reminderPublicHandler.ServeImage))
	mux.Handle("GET /r/unsubscribe", http.HandlerFunc(handlers.reminderPublicHandler.UnsubscribeConfirm))
	mux.Handle("POST /r/unsubscribe", http.HandlerFunc(handlers.reminderPublicHandler.UnsubscribeSubmit))

	mux.Handle("GET /og/default.png", http.HandlerFunc(handlers.ogImageHandler.Default))
	mux.Handle("GET /og/share/{token}", http.HandlerFunc(handlers.shareOGImageHandler.Serve))
	mux.Handle("GET /s/{token}", http.HandlerFunc(handlers.sharePublicHandler.Serve))
	mux.Handle("GET /api/docs", http.RedirectHandler("/static/swagger/index.html", http.StatusFound))

	mux.Handle("GET /{path...}", requireSession(http.HandlerFunc(handlers.pageHandler.Index)))
}
