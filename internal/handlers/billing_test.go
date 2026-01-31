package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
	"github.com/HammerMeetNail/yearofbingo/internal/services/billing"
	"github.com/HammerMeetNail/yearofbingo/internal/testutil"
)

type handlerStore struct {
	ensureFn     func(ctx context.Context, userID uuid.UUID, createFn func(context.Context) (string, error)) (string, error)
	redeemFn     func(ctx context.Context, userID uuid.UUID, codeHashHex string, now time.Time) error
	withWebhookFn func(ctx context.Context, meta billing.WebhookEventMeta, fn func(context.Context, services.Tx) error) (bool, error)
}

func (h *handlerStore) GetStripeCustomerID(ctx context.Context, userID uuid.UUID) (*string, error) {
	return nil, nil
}
func (h *handlerStore) EnsureStripeCustomerID(ctx context.Context, userID uuid.UUID, createFn func(context.Context) (string, error)) (string, error) {
	if h.ensureFn != nil {
		return h.ensureFn(ctx, userID, createFn)
	}
	return createFn(ctx)
}
func (h *handlerStore) SetStripeIDs(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string, conn services.DBConn) error {
	return nil
}
func (h *handlerStore) WithWebhookEvent(ctx context.Context, meta billing.WebhookEventMeta, fn func(ctx context.Context, tx services.Tx) error) (bool, error) {
	if h.withWebhookFn != nil {
		return h.withWebhookFn(ctx, meta, fn)
	}
	return false, fn(ctx, noopTx{})
}
func (h *handlerStore) FindUserIDByStripeCustomerID(ctx context.Context, customerID string, conn services.DBConn) (uuid.UUID, error) {
	return uuid.Nil, billing.ErrBillingUserNotFound
}
func (h *handlerStore) FindUserIDByStripeSubscriptionID(ctx context.Context, subscriptionID string, conn services.DBConn) (uuid.UUID, error) {
	return uuid.Nil, billing.ErrBillingUserNotFound
}
func (h *handlerStore) GrantLifetime(ctx context.Context, userID uuid.UUID, customerID string, conn services.DBConn) error {
	return nil
}
func (h *handlerStore) SetSubscriptionState(ctx context.Context, userID uuid.UUID, customerID, subscriptionID, status string, currentPeriodEnd time.Time, cancelAtPeriodEnd bool, conn services.DBConn) error {
	return nil
}
func (h *handlerStore) ResetToFree(ctx context.Context, userID uuid.UUID, conn services.DBConn) error {
	return nil
}
func (h *handlerStore) RedeemPremiumCode(ctx context.Context, userID uuid.UUID, codeHashHex string, now time.Time) error {
	if h.redeemFn != nil {
		return h.redeemFn(ctx, userID, codeHashHex, now)
	}
	return nil
}

type handlerStripe struct {
	createCustomerFn       func(ctx context.Context, email, userID string) (string, error)
	createCheckoutSessionFn func(ctx context.Context, params billing.CheckoutSessionParams) (string, error)
	createPortalSessionFn   func(ctx context.Context, customerID, returnURL string) (string, error)
}

func (h handlerStripe) CreateCustomer(ctx context.Context, email, userID string) (string, error) {
	if h.createCustomerFn != nil {
		return h.createCustomerFn(ctx, email, userID)
	}
	return "cus_test", nil
}
func (h handlerStripe) CreateCheckoutSession(ctx context.Context, params billing.CheckoutSessionParams) (string, error) {
	if h.createCheckoutSessionFn != nil {
		return h.createCheckoutSessionFn(ctx, params)
	}
	return "https://checkout.example.test/session", nil
}
func (h handlerStripe) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	if h.createPortalSessionFn != nil {
		return h.createPortalSessionFn(ctx, customerID, returnURL)
	}
	return "https://portal.example.test/session", nil
}

type noopTx struct{}

func (noopTx) Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error) { return nil, nil }
func (noopTx) Query(ctx context.Context, sql string, args ...any) (services.Rows, error)       { return nil, nil }
func (noopTx) QueryRow(ctx context.Context, sql string, args ...any) services.Row             { return nil }
func (noopTx) Commit(ctx context.Context) error                                                { return nil }
func (noopTx) Rollback(ctx context.Context) error                                              { return nil }

func withUser(req *http.Request, user *models.User) *http.Request {
	return req.WithContext(SetUserInContext(req.Context(), user))
}

func signatureHeader(secret string, payload []byte, ts int64) string {
	signed := []byte(fmt.Sprintf("%d.%s", ts, string(payload)))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(signed)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", ts, sig)
}

func newBillingService(enabled bool, store billing.StoreInterface, stripe billing.StripeClient) *billing.Service {
	cfg := config.BillingConfig{
		Enabled:                     enabled,
		StripeWebhookSecret:         "whsec_test",
		StripePremiumMonthlyPriceID: "price_month",
		StripePremiumYearlyPriceID:  "price_year",
		StripePremiumLifetimePriceID: "price_lifetime",
		StripeTip5PriceID:           "price_tip5",
		StripeTip10PriceID:          "price_tip10",
		StripeTip20PriceID:          "price_tip20",
	}
	return billing.NewService(cfg, "https://example.test", store, stripe)
}

func TestBillingHandler_Status_Unauthorized(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(true, store, handlerStripe{}))

	req := httptest.NewRequest(http.MethodGet, "/api/billing/status", nil)
	rr := httptest.NewRecorder()

	handler.Status(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusUnauthorized)
}

func TestBillingHandler_Status_Success(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(true, store, handlerStripe{}))

	user := &models.User{ID: uuid.New(), BillingPlan: "premium", BillingStatus: "active"}
	req := httptest.NewRequest(http.MethodGet, "/api/billing/status", nil)
	req = withUser(req, user)
	rr := httptest.NewRecorder()

	handler.Status(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusOK)
	resp := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if resp["is_premium"] != true {
		t.Fatalf("expected is_premium true, got %v", resp["is_premium"])
	}
}

func TestBillingHandler_CheckoutSubscription_InvalidJSON(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(true, store, handlerStripe{}))

	req := httptest.NewRequest(http.MethodPost, "/api/billing/checkout/subscription", bytes.NewBufferString("{bad"))
	req = withUser(req, &models.User{ID: uuid.New(), Email: "u@example.com"})
	rr := httptest.NewRecorder()

	handler.CheckoutSubscription(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusBadRequest)
}

func TestBillingHandler_CheckoutSubscription_Disabled(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(false, store, handlerStripe{}))

	req := testutil.NewTestRequestWithJSON(t, http.MethodPost, "/api/billing/checkout/subscription", map[string]string{"interval": "month"})
	req = withUser(req, &models.User{ID: uuid.New(), Email: "u@example.com"})
	rr := httptest.NewRecorder()

	handler.CheckoutSubscription(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusNotFound)
}

func TestBillingHandler_CheckoutSubscription_InvalidInterval(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(true, store, handlerStripe{}))

	req := testutil.NewTestRequestWithJSON(t, http.MethodPost, "/api/billing/checkout/subscription", map[string]string{"interval": "week"})
	req = withUser(req, &models.User{ID: uuid.New(), Email: "u@example.com"})
	rr := httptest.NewRecorder()

	handler.CheckoutSubscription(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusBadRequest)
}

func TestBillingHandler_CheckoutSubscription_Success(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(true, store, handlerStripe{}))

	req := testutil.NewTestRequestWithJSON(t, http.MethodPost, "/api/billing/checkout/subscription", map[string]string{"interval": "month"})
	req = withUser(req, &models.User{ID: uuid.New(), Email: "u@example.com"})
	rr := httptest.NewRecorder()

	handler.CheckoutSubscription(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusOK)
	resp := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if _, ok := resp["url"]; !ok {
		t.Fatalf("expected url in response, got %v", resp)
	}
}

func TestBillingHandler_CheckoutLifetime_Disabled(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(false, store, handlerStripe{}))

	req := httptest.NewRequest(http.MethodPost, "/api/billing/checkout/lifetime", nil)
	req = withUser(req, &models.User{ID: uuid.New(), Email: "u@example.com"})
	rr := httptest.NewRecorder()

	handler.CheckoutLifetime(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusNotFound)
}

func TestBillingHandler_CheckoutTip_InvalidAmount(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(true, store, handlerStripe{}))

	req := testutil.NewTestRequestWithJSON(t, http.MethodPost, "/api/billing/checkout/tip", map[string]int{"amount": 7})
	req = withUser(req, &models.User{ID: uuid.New(), Email: "u@example.com"})
	rr := httptest.NewRecorder()

	handler.CheckoutTip(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusBadRequest)
}

func TestBillingHandler_Portal_Disabled(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(false, store, handlerStripe{}))

	req := httptest.NewRequest(http.MethodPost, "/api/billing/portal", nil)
	req = withUser(req, &models.User{ID: uuid.New(), Email: "u@example.com"})
	rr := httptest.NewRecorder()

	handler.Portal(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusNotFound)
}

func TestBillingHandler_Redeem_InvalidCode(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(true, store, handlerStripe{}))

	req := testutil.NewTestRequestWithJSON(t, http.MethodPost, "/api/billing/redeem", map[string]string{"code": "bad"})
	req = withUser(req, &models.User{ID: uuid.New(), Email: "u@example.com"})
	rr := httptest.NewRecorder()

	handler.Redeem(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusBadRequest)
}

func TestBillingHandler_Redeem_Success(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(true, store, handlerStripe{}))

	code := "YOBP" + strings.Repeat("A", 24)
	req := testutil.NewTestRequestWithJSON(t, http.MethodPost, "/api/billing/redeem", map[string]string{"code": code})
	req = withUser(req, &models.User{ID: uuid.New(), Email: "u@example.com"})
	rr := httptest.NewRecorder()

	handler.Redeem(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusOK)
	resp := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if resp["is_premium"] != true {
		t.Fatalf("expected is_premium true, got %v", resp["is_premium"])
	}
}

func TestBillingHandler_Webhook_InvalidSignature(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(true, store, handlerStripe{}))

	req := httptest.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewBufferString(`{}`))
	req.Header.Set("Stripe-Signature", "t=1,v1=bad")
	rr := httptest.NewRecorder()

	handler.Webhook(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusBadRequest)
}

func TestBillingHandler_Webhook_Error(t *testing.T) {
	store := &handlerStore{
		withWebhookFn: func(ctx context.Context, meta billing.WebhookEventMeta, fn func(context.Context, services.Tx) error) (bool, error) {
			return false, fmt.Errorf("failed")
		},
	}
	handler := NewBillingHandler(newBillingService(true, store, handlerStripe{}))

	ts := time.Now().Unix()
	payload := []byte(fmt.Sprintf(`{"id":"evt_1","type":"invoice.created","livemode":false,"created":%d,"data":{"object":{}}}`, ts))
	req := httptest.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", signatureHeader("whsec_test", payload, ts))
	rr := httptest.NewRecorder()

	handler.Webhook(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusInternalServerError)
}

func TestBillingHandler_Webhook_Success(t *testing.T) {
	store := &handlerStore{}
	handler := NewBillingHandler(newBillingService(true, store, handlerStripe{}))

	ts := time.Now().Unix()
	payload := []byte(fmt.Sprintf(`{"id":"evt_2","type":"invoice.created","livemode":false,"created":%d,"data":{"object":{}}}`, ts))
	req := httptest.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", signatureHeader("whsec_test", payload, ts))
	rr := httptest.NewRecorder()

	handler.Webhook(rr, req)
	testutil.AssertStatusCode(t, rr, http.StatusOK)
}
