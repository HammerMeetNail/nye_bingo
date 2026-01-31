package billing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestStripeHTTPClient_CreateCustomer_SendsFields(t *testing.T) {
	var gotValues url.Values
	var gotUser string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, _, _ = r.BasicAuth()
		body, _ := io.ReadAll(r.Body)
		gotValues, _ = url.ParseQuery(string(body))
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "cus_123"})
	}))
	defer server.Close()

	client := &stripeHTTPClient{
		secretKey: "sk_test",
		client:    server.Client(),
		apiBase:   server.URL,
	}

	id, err := client.CreateCustomer(context.Background(), "u@example.com", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "cus_123" {
		t.Fatalf("expected customer id, got %q", id)
	}
	if gotUser != "sk_test" {
		t.Fatalf("expected basic auth user, got %q", gotUser)
	}
	if gotValues.Get("email") != "u@example.com" {
		t.Fatalf("expected email, got %q", gotValues.Get("email"))
	}
	if gotValues.Get("metadata[user_id]") != "user-1" {
		t.Fatalf("expected metadata user_id, got %q", gotValues.Get("metadata[user_id]"))
	}
	if gotValues.Get("metadata[app]") != "yearofbingo" {
		t.Fatalf("expected metadata app, got %q", gotValues.Get("metadata[app]"))
	}
}

func TestStripeHTTPClient_CreateCheckoutSession(t *testing.T) {
	var gotValues url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/checkout/sessions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		gotValues, _ = url.ParseQuery(string(body))
		_ = json.NewEncoder(w).Encode(map[string]string{"url": "https://checkout.example.test/session"})
	}))
	defer server.Close()

	client := &stripeHTTPClient{
		secretKey: "sk_test",
		client:    server.Client(),
		apiBase:   server.URL,
	}

	url, err := client.CreateCheckoutSession(context.Background(), CheckoutSessionParams{
		Mode:       CheckoutSessionModeSubscription,
		CustomerID: "cus_123",
		PriceID:    "price_123",
		SuccessURL: "https://example.test/success",
		CancelURL:  "https://example.test/cancel",
		Metadata: map[string]string{
			"user_id": "user-1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(url, "https://") {
		t.Fatalf("expected checkout url, got %q", url)
	}
	if gotValues.Get("mode") != "subscription" {
		t.Fatalf("expected mode subscription, got %q", gotValues.Get("mode"))
	}
	if gotValues.Get("customer") != "cus_123" {
		t.Fatalf("expected customer id, got %q", gotValues.Get("customer"))
	}
	if gotValues.Get("line_items[0][price]") != "price_123" {
		t.Fatalf("expected price id, got %q", gotValues.Get("line_items[0][price]"))
	}
	if gotValues.Get("metadata[user_id]") != "user-1" {
		t.Fatalf("expected metadata user_id, got %q", gotValues.Get("metadata[user_id]"))
	}
}

func TestStripeHTTPClient_CreatePortalSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"url": "https://portal.example.test/session"})
	}))
	defer server.Close()

	client := &stripeHTTPClient{
		secretKey: "sk_test",
		client:    server.Client(),
		apiBase:   server.URL,
	}

	url, err := client.CreatePortalSession(context.Background(), "cus_123", "https://example.test/return")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(url, "portal") {
		t.Fatalf("expected portal url, got %q", url)
	}
}

func TestStripeHTTPClient_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"message": "nope",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer server.Close()

	client := &stripeHTTPClient{
		secretKey: "sk_test",
		client:    server.Client(),
		apiBase:   server.URL,
	}

	_, err := client.CreatePortalSession(context.Background(), "cus_123", "https://example.test/return")
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("expected status error, got %v", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected message in error, got %v", err)
	}
}

func TestStripeHTTPClient_MissingID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer server.Close()

	client := &stripeHTTPClient{
		secretKey: "sk_test",
		client:    server.Client(),
		apiBase:   server.URL,
	}

	_, err := client.CreateCustomer(context.Background(), "u@example.com", "user-1")
	if err == nil || !strings.Contains(err.Error(), "missing customer id") {
		t.Fatalf("expected missing id error, got %v", err)
	}
}

func TestStripeHTTPClient_MissingURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer server.Close()

	client := &stripeHTTPClient{
		secretKey: "sk_test",
		client:    server.Client(),
		apiBase:   server.URL,
	}

	_, err := client.CreateCheckoutSession(context.Background(), CheckoutSessionParams{
		Mode:       CheckoutSessionModePayment,
		CustomerID: "cus_123",
		PriceID:    "price_123",
		SuccessURL: "https://example.test/success",
		CancelURL:  "https://example.test/cancel",
	})
	if err == nil || !strings.Contains(err.Error(), "missing checkout url") {
		t.Fatalf("expected missing url error, got %v", err)
	}
}

func TestStripeHTTPClient_MissingPortalURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer server.Close()

	client := &stripeHTTPClient{
		secretKey: "sk_test",
		client:    server.Client(),
		apiBase:   server.URL,
	}

	_, err := client.CreatePortalSession(context.Background(), "cus_123", "https://example.test/return")
	if err == nil || !strings.Contains(err.Error(), "missing portal url") {
		t.Fatalf("expected missing portal url error, got %v", err)
	}
}

func TestStripeHTTPClient_ErrorResponseNoMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &stripeHTTPClient{
		secretKey: "sk_test",
		client:    server.Client(),
		apiBase:   server.URL,
	}

	_, err := client.CreateCustomer(context.Background(), "u@example.com", "user-1")
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestStripeHTTPClient_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := &stripeHTTPClient{
		secretKey: "sk_test",
		client:    server.Client(),
		apiBase:   server.URL,
	}

	_, err := client.CreateCustomer(context.Background(), "u@example.com", "user-1")
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestNewStripeHTTPClient(t *testing.T) {
	client := NewStripeHTTPClient("sk_test")
	if client == nil {
		t.Fatal("expected client")
	}
}
