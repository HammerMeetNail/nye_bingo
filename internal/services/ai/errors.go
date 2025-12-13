package ai

import "errors"

var (
	ErrAIProviderUnavailable = errors.New("AI provider is currently unavailable")       // 503
	ErrAINotConfigured       = errors.New("AI provider is not configured")              // 503
	ErrSafetyViolation       = errors.New("generated content violated safety policies") // 400
	ErrRateLimitExceeded     = errors.New("rate limit exceeded")                        // 429
	ErrInvalidInput          = errors.New("invalid input parameters")                   // 400
)
