package main

import (
	"net/http"

	"github.com/HammerMeetNail/yearofbingo/internal/middleware"
)

type middlewareChain struct {
	authenticate             func(http.Handler) http.Handler
	applyMaxBodySize         func(http.Handler) http.Handler
	protectCSRF              func(http.Handler) http.Handler
	applyCacheControl        func(http.Handler) http.Handler
	applyCompress            func(http.Handler) http.Handler
	applySecurityHeaders     func(http.Handler) http.Handler
	applyRequestLogger       func(http.Handler) http.Handler
	applyTrustedProxyHeaders func(http.Handler) http.Handler
}

func newMiddlewareChain(
	authMiddleware *middleware.AuthMiddleware,
	maxBodySize *middleware.MaxBodySize,
	csrfMiddleware *middleware.CSRFMiddleware,
	cacheControl *middleware.CacheControl,
	compress *middleware.Compress,
	securityHeaders *middleware.SecurityHeaders,
	requestLogger *middleware.RequestLogger,
	trustedProxyHeaders *middleware.TrustedProxyHeaders,
) *middlewareChain {
	return &middlewareChain{
		authenticate:             authMiddleware.Authenticate,
		applyMaxBodySize:         maxBodySize.Apply,
		protectCSRF:              csrfMiddleware.Protect,
		applyCacheControl:        cacheControl.Apply,
		applyCompress:            compress.Apply,
		applySecurityHeaders:     securityHeaders.Apply,
		applyRequestLogger:       requestLogger.Apply,
		applyTrustedProxyHeaders: trustedProxyHeaders.Apply,
	}
}

func buildMiddlewareChain(base http.Handler, chain *middlewareChain) http.Handler {
	handler := base
	handler = chain.authenticate(handler)
	handler = chain.applyMaxBodySize(handler)
	handler = chain.protectCSRF(handler)
	handler = chain.applyCacheControl(handler)
	handler = chain.applyCompress(handler)
	handler = chain.applySecurityHeaders(handler)
	handler = chain.applyRequestLogger(handler)
	handler = chain.applyTrustedProxyHeaders(handler)
	return handler
}
