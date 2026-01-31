package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/config"
	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type stubStore struct {
	ensureStripeCustomerIDFn func(ctx context.Context, userID uuid.UUID, createFn func(context.Context) (string, error)) (string, error)
	redeemPremiumCodeFn      func(ctx context.Context, userID uuid.UUID, codeHashHex string, now time.Time) error
	withWebhookEventFn       func(ctx context.Context, meta WebhookEventMeta, fn func(context.Context, services.Tx) error) (bool, error)
	setStripeIDsFn           func(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string, conn services.DBConn) error
	findUserByCustomerFn     func(ctx context.Context, customerID string, conn services.DBConn) (uuid.UUID, error)
	findUserBySubFn          func(ctx context.Context, subscriptionID string, conn services.DBConn) (uuid.UUID, error)
	grantLifetimeFn          func(ctx context.Context, userID uuid.UUID, customerID string, conn services.DBConn) error
	setSubscriptionStateFn   func(ctx context.Context, userID uuid.UUID, customerID, subscriptionID, status string, currentPeriodEnd time.Time, cancelAtPeriodEnd bool, conn services.DBConn) error
	resetToFreeFn            func(ctx context.Context, userID uuid.UUID, conn services.DBConn) error
}

func (s *stubStore) GetStripeCustomerID(ctx context.Context, userID uuid.UUID) (*string, error) {
	return nil, nil
}
func (s *stubStore) EnsureStripeCustomerID(ctx context.Context, userID uuid.UUID, createFn func(context.Context) (string, error)) (string, error) {
	if s.ensureStripeCustomerIDFn != nil {
		return s.ensureStripeCustomerIDFn(ctx, userID, createFn)
	}
	return "", errors.New("EnsureStripeCustomerID not set")
}
func (s *stubStore) SetStripeIDs(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string, conn services.DBConn) error {
	if s.setStripeIDsFn != nil {
		return s.setStripeIDsFn(ctx, userID, customerID, subscriptionID, conn)
	}
	return nil
}
func (s *stubStore) WithWebhookEvent(ctx context.Context, meta WebhookEventMeta, fn func(ctx context.Context, tx services.Tx) error) (bool, error) {
	if s.withWebhookEventFn != nil {
		return s.withWebhookEventFn(ctx, meta, fn)
	}
	return false, nil
}
func (s *stubStore) FindUserIDByStripeCustomerID(ctx context.Context, customerID string, conn services.DBConn) (uuid.UUID, error) {
	if s.findUserByCustomerFn != nil {
		return s.findUserByCustomerFn(ctx, customerID, conn)
	}
	return uuid.Nil, ErrBillingUserNotFound
}
func (s *stubStore) FindUserIDByStripeSubscriptionID(ctx context.Context, subscriptionID string, conn services.DBConn) (uuid.UUID, error) {
	if s.findUserBySubFn != nil {
		return s.findUserBySubFn(ctx, subscriptionID, conn)
	}
	return uuid.Nil, ErrBillingUserNotFound
}
func (s *stubStore) GrantLifetime(ctx context.Context, userID uuid.UUID, customerID string, conn services.DBConn) error {
	if s.grantLifetimeFn != nil {
		return s.grantLifetimeFn(ctx, userID, customerID, conn)
	}
	return nil
}
func (s *stubStore) SetSubscriptionState(ctx context.Context, userID uuid.UUID, customerID, subscriptionID, status string, currentPeriodEnd time.Time, cancelAtPeriodEnd bool, conn services.DBConn) error {
	if s.setSubscriptionStateFn != nil {
		return s.setSubscriptionStateFn(ctx, userID, customerID, subscriptionID, status, currentPeriodEnd, cancelAtPeriodEnd, conn)
	}
	return nil
}
func (s *stubStore) ResetToFree(ctx context.Context, userID uuid.UUID, conn services.DBConn) error {
	if s.resetToFreeFn != nil {
		return s.resetToFreeFn(ctx, userID, conn)
	}
	return nil
}
func (s *stubStore) RedeemPremiumCode(ctx context.Context, userID uuid.UUID, codeHashHex string, now time.Time) error {
	if s.redeemPremiumCodeFn != nil {
		return s.redeemPremiumCodeFn(ctx, userID, codeHashHex, now)
	}
	return nil
}

type stubStripe struct {
	createCustomerFn        func(ctx context.Context, email, userID string) (string, error)
	createCheckoutSessionFn func(ctx context.Context, params CheckoutSessionParams) (string, error)
	createPortalSessionFn   func(ctx context.Context, customerID, returnURL string) (string, error)
}

func (s stubStripe) CreateCustomer(ctx context.Context, email, userID string) (string, error) {
	if s.createCustomerFn != nil {
		return s.createCustomerFn(ctx, email, userID)
	}
	return "cus_stub", nil
}
func (s stubStripe) CreateCheckoutSession(ctx context.Context, params CheckoutSessionParams) (string, error) {
	if s.createCheckoutSessionFn != nil {
		return s.createCheckoutSessionFn(ctx, params)
	}
	return "https://checkout.example.test/session", nil
}
func (s stubStripe) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	if s.createPortalSessionFn != nil {
		return s.createPortalSessionFn(ctx, customerID, returnURL)
	}
	return "https://portal.example.test/session", nil
}

type noopConn struct{}

func (noopConn) Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
	return nil, nil
}
func (noopConn) Query(ctx context.Context, sql string, args ...any) (services.Rows, error) {
	return nil, nil
}
func (noopConn) QueryRow(ctx context.Context, sql string, args ...any) services.Row { return nil }

type noopTx struct{ noopConn }

func (noopTx) Commit(ctx context.Context) error   { return nil }
func (noopTx) Rollback(ctx context.Context) error { return nil }

func stripeSig(secret string, payload []byte, ts int64) string {
	signed := fmt.Sprintf("%d.%s", ts, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signed))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", ts, sig)
}

func TestService_Status_Defaults(t *testing.T) {
	store := &stubStore{}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stubStripe{})

	user := &models.User{BillingPlan: "", BillingStatus: ""}
	status := svc.Status(user, time.Now())

	if !status.BillingEnabled {
		t.Fatal("expected billing enabled")
	}
	if status.Plan != "free" {
		t.Fatalf("expected plan free, got %q", status.Plan)
	}
	if status.Status != "inactive" {
		t.Fatalf("expected status inactive, got %q", status.Status)
	}
	if status.IsPremium {
		t.Fatal("expected not premium")
	}
}

func TestService_CreateSubscriptionCheckoutURL_InvalidInterval(t *testing.T) {
	store := &stubStore{}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stubStripe{})

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	_, err := svc.CreateSubscriptionCheckoutURL(context.Background(), user, CheckoutInterval("week"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestService_CreateSubscriptionCheckoutURL_MissingPrice(t *testing.T) {
	store := &stubStore{}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stubStripe{})

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	_, err := svc.CreateSubscriptionCheckoutURL(context.Background(), user, IntervalMonth)
	if err == nil || !strings.Contains(err.Error(), "missing stripe price id") {
		t.Fatalf("expected missing price error, got %v", err)
	}
}

func TestService_CreateSubscriptionCheckoutURL_Success(t *testing.T) {
	var gotParams CheckoutSessionParams
	store := &stubStore{
		ensureStripeCustomerIDFn: func(ctx context.Context, userID uuid.UUID, createFn func(context.Context) (string, error)) (string, error) {
			return createFn(ctx)
		},
	}
	stripe := stubStripe{
		createCustomerFn: func(ctx context.Context, email, userID string) (string, error) {
			return "cus_123", nil
		},
		createCheckoutSessionFn: func(ctx context.Context, params CheckoutSessionParams) (string, error) {
			gotParams = params
			return "https://checkout.example.test/session", nil
		},
	}
	svc := NewService(config.BillingConfig{
		Enabled:                     true,
		StripePremiumMonthlyPriceID: "price_month",
	}, "https://example.test", store, stripe)

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	url, err := svc.CreateSubscriptionCheckoutURL(context.Background(), user, IntervalMonth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(url, "https://") {
		t.Fatalf("expected url, got %q", url)
	}
	if gotParams.Mode != CheckoutSessionModeSubscription {
		t.Fatalf("expected subscription mode, got %q", gotParams.Mode)
	}
	if gotParams.PriceID != "price_month" {
		t.Fatalf("expected price id, got %q", gotParams.PriceID)
	}
	if gotParams.Metadata["user_id"] == "" || gotParams.Metadata["purchase"] != "subscription" {
		t.Fatalf("expected metadata, got %+v", gotParams.Metadata)
	}
}

func TestService_CreateLifetimeCheckoutURL_MissingPrice(t *testing.T) {
	store := &stubStore{}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stubStripe{})

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	_, err := svc.CreateLifetimeCheckoutURL(context.Background(), user)
	if err == nil || !strings.Contains(err.Error(), "missing stripe lifetime price id") {
		t.Fatalf("expected missing price error, got %v", err)
	}
}

func TestService_CreateLifetimeCheckoutURL_Success(t *testing.T) {
	var gotParams CheckoutSessionParams
	store := &stubStore{
		ensureStripeCustomerIDFn: func(ctx context.Context, userID uuid.UUID, createFn func(context.Context) (string, error)) (string, error) {
			return createFn(ctx)
		},
	}
	stripe := stubStripe{
		createCustomerFn: func(ctx context.Context, email, userID string) (string, error) {
			return "cus_456", nil
		},
		createCheckoutSessionFn: func(ctx context.Context, params CheckoutSessionParams) (string, error) {
			gotParams = params
			return "https://checkout.example.test/lifetime", nil
		},
	}
	svc := NewService(config.BillingConfig{
		Enabled:                      true,
		StripePremiumLifetimePriceID: "price_lifetime",
	}, "https://example.test", store, stripe)

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	_, err := svc.CreateLifetimeCheckoutURL(context.Background(), user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotParams.Mode != CheckoutSessionModePayment {
		t.Fatalf("expected payment mode, got %q", gotParams.Mode)
	}
	if gotParams.PriceID != "price_lifetime" {
		t.Fatalf("expected price id, got %q", gotParams.PriceID)
	}
	if gotParams.Metadata["purchase"] != "lifetime" {
		t.Fatalf("expected lifetime metadata, got %+v", gotParams.Metadata)
	}
}

func TestService_CreateTipCheckoutURL_InvalidAmount(t *testing.T) {
	store := &stubStore{}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stubStripe{})

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	_, err := svc.CreateTipCheckoutURL(context.Background(), user, CheckoutTipAmount(7))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestService_CreateTipCheckoutURL_MissingPrice(t *testing.T) {
	store := &stubStore{}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stubStripe{})

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	_, err := svc.CreateTipCheckoutURL(context.Background(), user, Tip5)
	if err == nil || !strings.Contains(err.Error(), "missing stripe tip price id") {
		t.Fatalf("expected missing price error, got %v", err)
	}
}

func TestService_CreatePortalURL_Disabled(t *testing.T) {
	store := &stubStore{}
	svc := NewService(config.BillingConfig{Enabled: false}, "https://example.test", store, stubStripe{})

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	_, err := svc.CreatePortalURL(context.Background(), user)
	if !errors.Is(err, ErrBillingDisabled) {
		t.Fatalf("expected ErrBillingDisabled, got %v", err)
	}
}

func TestService_RedeemCode_Normalizes(t *testing.T) {
	var gotHash string
	store := &stubStore{
		redeemPremiumCodeFn: func(ctx context.Context, userID uuid.UUID, codeHashHex string, now time.Time) error {
			gotHash = codeHashHex
			return nil
		},
	}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stubStripe{})

	code := "yobp-aaaa bbbb-cccc-dddd-eeee-ffff"
	user := &models.User{ID: uuid.New()}
	if err := svc.RedeemCode(context.Background(), user, code, time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	normalized := normalizePremiumCode(code)
	sum := sha256.Sum256([]byte(normalized))
	expected := hex.EncodeToString(sum[:])
	if gotHash != expected {
		t.Fatalf("expected hash %q, got %q", expected, gotHash)
	}
}

func TestService_HandleWebhook_InvalidJSON(t *testing.T) {
	store := &stubStore{}
	secret := "whsec_test"
	svc := NewService(config.BillingConfig{Enabled: true, StripeWebhookSecret: secret}, "https://example.test", store, stubStripe{})

	ts := time.Now().Unix()
	payload := []byte(`{not-json}`)
	sig := stripeSig(secret, payload, ts)

	err := svc.HandleWebhook(context.Background(), payload, sig, time.Unix(ts, 0))
	if err == nil || !strings.Contains(err.Error(), "decode stripe event") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestService_HandleWebhook_MissingID(t *testing.T) {
	store := &stubStore{}
	secret := "whsec_test"
	svc := NewService(config.BillingConfig{Enabled: true, StripeWebhookSecret: secret}, "https://example.test", store, stubStripe{})

	ts := time.Now().Unix()
	payload := []byte(fmt.Sprintf(`{"id":"","type":"checkout.session.completed","livemode":false,"created":%d,"data":{"object":{}}}`, ts))
	sig := stripeSig(secret, payload, ts)

	err := svc.HandleWebhook(context.Background(), payload, sig, time.Unix(ts, 0))
	if err == nil || !strings.Contains(err.Error(), "stripe event missing id") {
		t.Fatalf("expected missing id error, got %v", err)
	}
}

func TestService_HandleSubscriptionEvent_ResetsOnExpired(t *testing.T) {
	var resetCalls int
	store := &stubStore{
		findUserBySubFn: func(ctx context.Context, subscriptionID string, conn services.DBConn) (uuid.UUID, error) {
			return uuid.New(), nil
		},
		resetToFreeFn: func(ctx context.Context, userID uuid.UUID, conn services.DBConn) error {
			resetCalls++
			return nil
		},
	}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stubStripe{})

	now := time.Now().UTC()
	sub := stripeSubscription{
		ID:               "sub_1",
		Customer:         "cus_1",
		Status:           "active",
		CurrentPeriodEnd: now.Add(-time.Hour).Unix(),
	}
	if err := svc.handleSubscriptionEvent(context.Background(), noopConn{}, sub, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resetCalls != 1 {
		t.Fatalf("expected reset call, got %d", resetCalls)
	}
}

func TestService_HandleSubscriptionEvent_SetsSubscriptionState(t *testing.T) {
	var setCalls int
	store := &stubStore{
		findUserBySubFn: func(ctx context.Context, subscriptionID string, conn services.DBConn) (uuid.UUID, error) {
			return uuid.New(), nil
		},
		setSubscriptionStateFn: func(ctx context.Context, userID uuid.UUID, customerID, subscriptionID, status string, currentPeriodEnd time.Time, cancelAtPeriodEnd bool, conn services.DBConn) error {
			setCalls++
			if status != "active" {
				return fmt.Errorf("unexpected status")
			}
			return nil
		},
	}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stubStripe{})

	now := time.Now().UTC()
	sub := stripeSubscription{
		ID:               "sub_2",
		Customer:         "cus_2",
		Status:           "active",
		CurrentPeriodEnd: now.Add(time.Hour).Unix(),
	}
	if err := svc.handleSubscriptionEvent(context.Background(), noopConn{}, sub, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if setCalls != 1 {
		t.Fatalf("expected set call, got %d", setCalls)
	}
}

func TestService_HandleSubscriptionEvent_FallbackToCustomer(t *testing.T) {
	var customerCalls int
	store := &stubStore{
		findUserBySubFn: func(ctx context.Context, subscriptionID string, conn services.DBConn) (uuid.UUID, error) {
			return uuid.Nil, ErrBillingUserNotFound
		},
		findUserByCustomerFn: func(ctx context.Context, customerID string, conn services.DBConn) (uuid.UUID, error) {
			customerCalls++
			return uuid.New(), nil
		},
		setSubscriptionStateFn: func(ctx context.Context, userID uuid.UUID, customerID, subscriptionID, status string, currentPeriodEnd time.Time, cancelAtPeriodEnd bool, conn services.DBConn) error {
			return nil
		},
	}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stubStripe{})

	now := time.Now().UTC()
	sub := stripeSubscription{
		ID:               "sub_3",
		Customer:         "cus_3",
		Status:           "active",
		CurrentPeriodEnd: now.Add(time.Hour).Unix(),
	}
	if err := svc.handleSubscriptionEvent(context.Background(), noopConn{}, sub, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if customerCalls != 1 {
		t.Fatalf("expected customer lookup, got %d", customerCalls)
	}
}

func TestMapStripeSubscriptionStatus(t *testing.T) {
	cases := map[string]string{
		"active":   "active",
		"trialing": "trialing",
		"past_due": "past_due",
		"canceled": "canceled",
		"other":    "inactive",
	}
	for in, expected := range cases {
		if got := mapStripeSubscriptionStatus(in); got != expected {
			t.Fatalf("expected %q for %q, got %q", expected, in, got)
		}
	}
}

func TestService_HandleCheckoutCompleted_Lifetime(t *testing.T) {
	var setIDs int
	var grants int
	store := &stubStore{
		setStripeIDsFn: func(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string, conn services.DBConn) error {
			setIDs++
			return nil
		},
		grantLifetimeFn: func(ctx context.Context, userID uuid.UUID, customerID string, conn services.DBConn) error {
			grants++
			return nil
		},
	}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stubStripe{})

	session := stripeCheckoutSession{
		Customer: "cus",
		Metadata: map[string]string{
			"user_id":  uuid.New().String(),
			"purchase": "lifetime",
		},
	}
	if err := svc.handleCheckoutCompleted(context.Background(), noopConn{}, session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if setIDs != 1 || grants != 1 {
		t.Fatalf("expected setIDs=1 grants=1, got setIDs=%d grants=%d", setIDs, grants)
	}
}

func TestService_HandleWebhook_UnknownEvent(t *testing.T) {
	store := &stubStore{
		withWebhookEventFn: func(ctx context.Context, meta WebhookEventMeta, fn func(context.Context, services.Tx) error) (bool, error) {
			return false, fn(ctx, noopTx{})
		},
	}
	secret := "whsec_test"
	svc := NewService(config.BillingConfig{Enabled: true, StripeWebhookSecret: secret}, "https://example.test", store, stubStripe{})

	ts := time.Now().Unix()
	payload := []byte(fmt.Sprintf(`{"id":"evt_unknown","type":"invoice.created","livemode":false,"created":%d,"data":{"object":{}}}`, ts))
	sig := stripeSig(secret, payload, ts)

	if err := svc.HandleWebhook(context.Background(), payload, sig, time.Unix(ts, 0)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_SuccessCancelReturnURLs(t *testing.T) {
	store := &stubStore{}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test/", store, stubStripe{})

	if !strings.Contains(svc.successURL(), "billing=success") {
		t.Fatalf("unexpected success url: %s", svc.successURL())
	}
	if !strings.Contains(svc.cancelURL(), "billing=cancel") {
		t.Fatalf("unexpected cancel url: %s", svc.cancelURL())
	}
	if !strings.HasSuffix(svc.returnURL(), "/#profile") {
		t.Fatalf("unexpected return url: %s", svc.returnURL())
	}
}

func TestService_RedeemCode_Disabled(t *testing.T) {
	store := &stubStore{}
	svc := NewService(config.BillingConfig{Enabled: false}, "https://example.test", store, stubStripe{})

	user := &models.User{ID: uuid.New()}
	err := svc.RedeemCode(context.Background(), user, "YOBP"+strings.Repeat("A", 24), time.Now())
	if !errors.Is(err, ErrBillingDisabled) {
		t.Fatalf("expected ErrBillingDisabled, got %v", err)
	}
}

func TestService_CreatePortalURL_Success(t *testing.T) {
	store := &stubStore{
		ensureStripeCustomerIDFn: func(ctx context.Context, userID uuid.UUID, createFn func(context.Context) (string, error)) (string, error) {
			return createFn(ctx)
		},
	}
	stripe := stubStripe{
		createCustomerFn: func(ctx context.Context, email, userID string) (string, error) {
			return "cus_789", nil
		},
		createPortalSessionFn: func(ctx context.Context, customerID, returnURL string) (string, error) {
			if !strings.HasSuffix(returnURL, "/#profile") {
				return "", fmt.Errorf("bad return url")
			}
			return "https://portal.example.test/session", nil
		},
	}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, stripe)

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	url, err := svc.CreatePortalURL(context.Background(), user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(url, "https://") {
		t.Fatalf("expected url, got %q", url)
	}
}

func TestService_CreateTipCheckoutURL_Success(t *testing.T) {
	var gotParams CheckoutSessionParams
	store := &stubStore{
		ensureStripeCustomerIDFn: func(ctx context.Context, userID uuid.UUID, createFn func(context.Context) (string, error)) (string, error) {
			return createFn(ctx)
		},
	}
	stripe := stubStripe{
		createCustomerFn: func(ctx context.Context, email, userID string) (string, error) {
			return "cus_tip", nil
		},
		createCheckoutSessionFn: func(ctx context.Context, params CheckoutSessionParams) (string, error) {
			gotParams = params
			return "https://checkout.example.test/tip", nil
		},
	}
	svc := NewService(config.BillingConfig{
		Enabled:           true,
		StripeTip5PriceID: "price_tip5",
	}, "https://example.test", store, stripe)

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	_, err := svc.CreateTipCheckoutURL(context.Background(), user, Tip5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotParams.PriceID != "price_tip5" {
		t.Fatalf("expected tip price id, got %q", gotParams.PriceID)
	}
	if gotParams.Metadata["purchase"] != "tip" {
		t.Fatalf("expected tip metadata, got %+v", gotParams.Metadata)
	}
}

func TestService_HandleWebhook_InvalidSignature(t *testing.T) {
	store := &stubStore{}
	secret := "whsec_test"
	svc := NewService(config.BillingConfig{Enabled: true, StripeWebhookSecret: secret}, "https://example.test", store, stubStripe{})

	ts := time.Now().Unix()
	payload := []byte(fmt.Sprintf(`{"id":"evt_bad","type":"checkout.session.completed","livemode":false,"created":%d,"data":{"object":{}}}`, ts))

	err := svc.HandleWebhook(context.Background(), payload, "t=1,v1=bad", time.Unix(ts, 0))
	if err == nil || !errors.Is(err, ErrStripeSignatureInvalid) {
		t.Fatalf("expected signature error, got %v", err)
	}
}

func TestNormalizePremiumCode(t *testing.T) {
	normalized := normalizePremiumCode(" yobp-abcd efgh-ijkl ")
	if normalized != "YOBPABCDEFGHIJKL" {
		t.Fatalf("unexpected normalize result: %s", normalized)
	}
}

func TestService_HandleWebhook_DecodesSubscription(t *testing.T) {
	var gotStatus string
	store := &stubStore{
		withWebhookEventFn: func(ctx context.Context, meta WebhookEventMeta, fn func(context.Context, services.Tx) error) (bool, error) {
			return false, fn(ctx, noopTx{})
		},
		findUserBySubFn: func(ctx context.Context, subscriptionID string, conn services.DBConn) (uuid.UUID, error) {
			return uuid.New(), nil
		},
		setSubscriptionStateFn: func(ctx context.Context, userID uuid.UUID, customerID, subscriptionID, status string, currentPeriodEnd time.Time, cancelAtPeriodEnd bool, conn services.DBConn) error {
			gotStatus = status
			return nil
		},
	}
	secret := "whsec_test"
	svc := NewService(config.BillingConfig{Enabled: true, StripeWebhookSecret: secret}, "https://example.test", store, stubStripe{})

	now := time.Now().UTC()
	event := stripeEvent{
		ID:       "evt_sub",
		Type:     "customer.subscription.updated",
		Livemode: false,
		Created:  now.Unix(),
	}
	sub := stripeSubscription{
		ID:               "sub_1",
		Customer:         "cus_1",
		Status:           "active",
		CurrentPeriodEnd: now.Add(time.Hour).Unix(),
	}
	data, _ := json.Marshal(sub)
	event.Data.Object = data
	payload, _ := json.Marshal(event)
	sig := stripeSig(secret, payload, now.Unix())

	err := svc.HandleWebhook(context.Background(), payload, sig, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotStatus != "active" {
		t.Fatalf("expected active status, got %q", gotStatus)
	}
}
