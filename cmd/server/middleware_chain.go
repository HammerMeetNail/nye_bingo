package main

import (
	"net/http"

	"github.com/HammerMeetNail/yearofbingo/internal/middleware"
)

type middlewareChain struct {
	authMiddleware     *middleware.AuthMiddleware
	maxBodySize        *middleware.MaxBodySize
	csrfMiddleware     *middleware.CSRFMiddleware
	cacheControl       *middleware.CacheControl
	compress           *middleware.Compress
	securityHeaders    *middleware.SecurityHeaders
	requestLogger      *middleware.RequestLogger
	trustedProxyHeader *middleware.TrustedProxyHeaders
}

func buildMiddlewareChain(base http.Handler, chain *middlewareChain) http.Handler {
	handler := base
	handler = chain.authMiddleware.Authenticate(handler)
	handler = chain.maxBodySize.Apply(handler)
	handler = chain.csrfMiddleware.Protect(handler)
	handler = chain.cacheControl.Apply(handler)
	handler = chain.compress.Apply(handler)
	handler = chain.securityHeaders.Apply(handler)
	handler = chain.requestLogger.Apply(handler)
	handler = chain.trustedProxyHeader.Apply(handler)
	return handler
}
