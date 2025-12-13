package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFMiddleware_SafeMethodsAllowed(t *testing.T) {
	csrf := NewCSRFMiddleware(false)

	safeMethods := []string{http.MethodGet, http.MethodHead, http.MethodOptions}

	for _, method := range safeMethods {
		t.Run(method, func(t *testing.T) {
			handlerCalled := false
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(method, "/api/test", nil)
			rr := httptest.NewRecorder()

			csrf.Protect(handler).ServeHTTP(rr, req)

			if !handlerCalled {
				t.Errorf("%s request should call handler", method)
			}
			if rr.Code != http.StatusOK {
				t.Errorf("%s request: expected status 200, got %d", method, rr.Code)
			}
		})
	}
}

func TestCSRFMiddleware_UnsafeMethodsRequireToken(t *testing.T) {
	csrf := NewCSRFMiddleware(false)

	unsafeMethods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range unsafeMethods {
		t.Run(method+"_no_token", func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("handler should not be called without CSRF token")
			})

			req := httptest.NewRequest(method, "/api/test", nil)
			rr := httptest.NewRecorder()

			csrf.Protect(handler).ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Errorf("%s request without token: expected status 403, got %d", method, rr.Code)
			}
		})
	}
}

func TestCSRFMiddleware_ValidTokenAllowsRequest(t *testing.T) {
	csrf := NewCSRFMiddleware(false)

	// First, get a token via a GET request
	getReq := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	getRr := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	csrf.Protect(handler).ServeHTTP(getRr, getReq)

	// Extract the CSRF token from the response
	var csrfToken string
	for _, cookie := range getRr.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			csrfToken = cookie.Value
			break
		}
	}

	if csrfToken == "" {
		t.Fatal("CSRF token not set in cookie")
	}

	// Now make a POST request with the token
	postReq := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	postReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	postReq.Header.Set(csrfHeaderName, csrfToken)

	postRr := httptest.NewRecorder()
	handlerCalled := false

	postHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	csrf.Protect(postHandler).ServeHTTP(postRr, postReq)

	if !handlerCalled {
		t.Error("handler should be called with valid CSRF token")
	}
	if postRr.Code != http.StatusOK {
		t.Errorf("POST with valid token: expected status 200, got %d", postRr.Code)
	}
}

func TestCSRFMiddleware_MismatchedTokenFails(t *testing.T) {
	csrf := NewCSRFMiddleware(false)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with mismatched token")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "token-in-cookie"})
	req.Header.Set(csrfHeaderName, "different-token-in-header")

	rr := httptest.NewRecorder()
	csrf.Protect(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("mismatched token: expected status 403, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_MissingHeaderFails(t *testing.T) {
	csrf := NewCSRFMiddleware(false)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without CSRF header")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "valid-token"})
	// No X-CSRF-Token header

	rr := httptest.NewRecorder()
	csrf.Protect(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("missing header: expected status 403, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_GetToken(t *testing.T) {
	csrf := NewCSRFMiddleware(false)

	t.Run("generates new token when no cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/csrf", nil)
		rr := httptest.NewRecorder()

		csrf.GetToken(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		// Check cookie was set
		var tokenCookie *http.Cookie
		for _, cookie := range rr.Result().Cookies() {
			if cookie.Name == csrfCookieName {
				tokenCookie = cookie
				break
			}
		}

		if tokenCookie == nil {
			t.Fatal("CSRF cookie not set")
			return
		}
		if tokenCookie.Value == "" {
			t.Error("CSRF cookie value is empty")
		}
	})

	t.Run("returns existing token when cookie present", func(t *testing.T) {
		existingToken := "existing-csrf-token"
		req := httptest.NewRequest(http.MethodGet, "/api/csrf", nil)
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: existingToken})

		rr := httptest.NewRecorder()
		csrf.GetToken(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		// Body should contain the existing token
		body := rr.Body.String()
		if body != `{"token":"`+existingToken+`"}` {
			t.Errorf("unexpected response body: %s", body)
		}
	})
}

func TestCSRFMiddleware_SecureMode(t *testing.T) {
	csrf := NewCSRFMiddleware(true) // secure mode

	req := httptest.NewRequest(http.MethodGet, "/api/csrf", nil)
	rr := httptest.NewRecorder()

	csrf.GetToken(rr, req)

	// Check cookie has Secure flag
	var tokenCookie *http.Cookie
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			tokenCookie = cookie
			break
		}
	}

	if tokenCookie == nil {
		t.Fatal("CSRF cookie not set")
		return
	}
	if !tokenCookie.Secure {
		t.Error("CSRF cookie should have Secure flag in secure mode")
	}
}

func TestCSRFMiddleware_SameSiteStrict(t *testing.T) {
	csrf := NewCSRFMiddleware(false)

	req := httptest.NewRequest(http.MethodGet, "/api/csrf", nil)
	rr := httptest.NewRecorder()

	csrf.GetToken(rr, req)

	var tokenCookie *http.Cookie
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == csrfCookieName {
			tokenCookie = cookie
			break
		}
	}

	if tokenCookie == nil {
		t.Fatal("CSRF cookie not set")
		return
	}
	if tokenCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("expected SameSite=Strict, got %v", tokenCookie.SameSite)
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	token1, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token1 == "" {
		t.Error("token should not be empty")
	}

	token2, err := generateCSRFToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token1 == token2 {
		t.Error("tokens should be unique")
	}

	// Token should be base64 encoded
	if len(token1) < 40 {
		t.Errorf("token seems too short: %d chars", len(token1))
	}
}
