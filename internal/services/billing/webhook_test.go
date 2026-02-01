package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

func stripeSignatureHeader(t *testing.T, secret string, payload []byte, ts int64) string {
	t.Helper()
	signed := fmt.Sprintf("%d.%s", ts, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signed))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", ts, sig)
}

func TestVerifyStripeSignature_Valid(t *testing.T) {
	secret := "whsec_test"
	ts := time.Now().Unix()
	payload := []byte(fmt.Sprintf(`{"id":"evt_test","type":"checkout.session.completed","livemode":false,"created":%d,"data":{"object":{}}}`, ts))
	header := stripeSignatureHeader(t, secret, payload, ts)

	if err := VerifyStripeSignature(secret, payload, header); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestVerifyStripeSignature_InvalidRejected(t *testing.T) {
	secret := "whsec_test"
	ts := time.Now().Unix()
	payload := []byte(fmt.Sprintf(`{"id":"evt_test","type":"checkout.session.completed","livemode":false,"created":%d,"data":{"object":{}}}`, ts))

	if err := VerifyStripeSignature(secret, payload, fmt.Sprintf("t=%d,v1=not-a-real-sig", ts)); err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifyStripeSignature_TimestampTooOld(t *testing.T) {
	secret := "whsec_test"
	// Use a timestamp older than the default 5-minute tolerance
	ts := time.Now().Unix() - DefaultWebhookTimestampTolerance - 10
	payload := []byte(fmt.Sprintf(`{"id":"evt_test","type":"checkout.session.completed","livemode":false,"created":%d,"data":{"object":{}}}`, ts))
	header := stripeSignatureHeader(t, secret, payload, ts)

	err := VerifyStripeSignature(secret, payload, header)
	if err == nil {
		t.Fatal("expected error for old timestamp")
	}
	if !errors.Is(err, ErrStripeSignatureInvalid) {
		t.Fatalf("expected ErrStripeSignatureInvalid, got %v", err)
	}
}

func TestVerifyStripeSignatureWithTolerance_CustomTolerance(t *testing.T) {
	secret := "whsec_test"
	// Use a timestamp 10 seconds old
	ts := time.Now().Unix() - 10
	payload := []byte(fmt.Sprintf(`{"id":"evt_test","type":"checkout.session.completed","livemode":false,"created":%d,"data":{"object":{}}}`, ts))
	header := stripeSignatureHeader(t, secret, payload, ts)

	// Should pass with 60 second tolerance
	if err := VerifyStripeSignatureWithTolerance(secret, payload, header, 60); err != nil {
		t.Fatalf("expected nil with 60s tolerance, got %v", err)
	}

	// Should fail with 5 second tolerance
	if err := VerifyStripeSignatureWithTolerance(secret, payload, header, 5); err == nil {
		t.Fatal("expected error with 5s tolerance")
	}
}

func TestVerifyStripeSignature_TimestampTooFarInFuture(t *testing.T) {
	secret := "whsec_test"
	// Use a timestamp far in the future (beyond tolerance)
	ts := time.Now().Unix() + DefaultWebhookTimestampTolerance + 10
	payload := []byte(fmt.Sprintf(`{"id":"evt_test","type":"checkout.session.completed","livemode":false,"created":%d,"data":{"object":{}}}`, ts))
	header := stripeSignatureHeader(t, secret, payload, ts)

	err := VerifyStripeSignature(secret, payload, header)
	if err == nil {
		t.Fatal("expected error for future timestamp")
	}
	if !errors.Is(err, ErrStripeSignatureInvalid) {
		t.Fatalf("expected ErrStripeSignatureInvalid, got %v", err)
	}
}

type memStore struct {
	processed map[string]bool
	setIDs    int
	grants    int
}

func (m *memStore) GetStripeCustomerID(ctx context.Context, userID uuid.UUID) (*string, error) {
	return nil, nil
}
func (m *memStore) EnsureStripeCustomerID(ctx context.Context, userID uuid.UUID, createFn func(context.Context) (string, error)) (string, error) {
	return "cus_test", nil
}
func (m *memStore) SetStripeIDs(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string, conn services.DBConn) error {
	m.setIDs++
	return nil
}
func (m *memStore) WithWebhookEvent(ctx context.Context, meta WebhookEventMeta, fn func(ctx context.Context, tx services.Tx) error) (bool, error) {
	if m.processed == nil {
		m.processed = map[string]bool{}
	}
	if m.processed[meta.StripeEventID] {
		return true, nil
	}
	if err := fn(ctx, noopTx{}); err != nil {
		return false, err
	}
	m.processed[meta.StripeEventID] = true
	return false, nil
}
func (m *memStore) FindUserIDByStripeCustomerID(ctx context.Context, customerID string, conn services.DBConn) (uuid.UUID, error) {
	return uuid.Nil, ErrBillingUserNotFound
}
func (m *memStore) FindUserIDByStripeSubscriptionID(ctx context.Context, subscriptionID string, conn services.DBConn) (uuid.UUID, error) {
	return uuid.Nil, ErrBillingUserNotFound
}
func (m *memStore) GrantLifetime(ctx context.Context, userID uuid.UUID, customerID string, conn services.DBConn) error {
	m.grants++
	return nil
}
func (m *memStore) SetSubscriptionState(ctx context.Context, userID uuid.UUID, customerID, subscriptionID, status string, currentPeriodEnd time.Time, cancelAtPeriodEnd bool, conn services.DBConn) error {
	return nil
}
func (m *memStore) ResetToFree(ctx context.Context, userID uuid.UUID, conn services.DBConn) error {
	return nil
}
func (m *memStore) RedeemPremiumCode(ctx context.Context, userID uuid.UUID, codeHashHex string, now time.Time) error {
	return nil
}

type noopStripe struct{}

func (noopStripe) CreateCustomer(ctx context.Context, email, userID string) (string, error) {
	return "cus_test", nil
}
func (noopStripe) CreateCheckoutSession(ctx context.Context, params CheckoutSessionParams) (string, error) {
	return "https://checkout.example.test/session", nil
}
func (noopStripe) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	return "https://portal.example.test/session", nil
}

func TestService_HandleWebhook_Idempotent(t *testing.T) {
	store := &memStore{}
	svc := NewService(config.BillingConfig{Enabled: true, StripeWebhookSecret: "whsec_test"}, "https://example.test", store, noopStripe{})

	userID := uuid.New()
	ts := time.Now().Unix()
	payload := []byte(fmt.Sprintf(`{"id":"evt_test","type":"checkout.session.completed","livemode":false,"created":%d,"data":{"object":{"id":"cs_test","customer":"cus_test","subscription":"","metadata":{"user_id":"%s","purchase":"lifetime"}}}}`, ts, userID.String()))
	sig := stripeSignatureHeader(t, "whsec_test", payload, ts)

	if err := svc.HandleWebhook(context.Background(), payload, sig, time.Unix(ts, 0)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.setIDs != 1 || store.grants != 1 {
		t.Fatalf("expected setIDs=1 grants=1, got setIDs=%d grants=%d", store.setIDs, store.grants)
	}

	if err := svc.HandleWebhook(context.Background(), payload, sig, time.Unix(ts, 0)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.setIDs != 1 || store.grants != 1 {
		t.Fatalf("expected idempotent no-op, got setIDs=%d grants=%d", store.setIDs, store.grants)
	}
}

func TestService_RedeemCode_InvalidFormat(t *testing.T) {
	store := &memStore{}
	svc := NewService(config.BillingConfig{Enabled: true}, "https://example.test", store, noopStripe{})

	user := &models.User{ID: uuid.New()}
	if err := svc.RedeemCode(context.Background(), user, "not-a-code", time.Now()); err == nil {
		t.Fatal("expected error")
	}
}

func TestService_CreateSubscriptionCheckoutURL_BillingDisabled(t *testing.T) {
	store := &memStore{}
	svc := NewService(config.BillingConfig{Enabled: false}, "https://example.test", store, noopStripe{})

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	_, err := svc.CreateSubscriptionCheckoutURL(context.Background(), user, IntervalMonth)
	if !errors.Is(err, ErrBillingDisabled) {
		t.Fatalf("expected ErrBillingDisabled, got %v", err)
	}
}

func TestService_CreateSubscriptionCheckoutURL_MapsIntervalToServerPrice(t *testing.T) {
	store := &memStore{}
	svc := NewService(config.BillingConfig{
		Enabled:                     true,
		StripePremiumMonthlyPriceID: "price_month",
		StripePremiumYearlyPriceID:  "price_year",
	}, "https://example.test", store, noopStripe{})

	user := &models.User{ID: uuid.New(), Email: "u@example.com"}
	url, err := svc.CreateSubscriptionCheckoutURL(context.Background(), user, IntervalMonth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix([]byte(url), []byte("https://")) {
		t.Fatalf("expected url, got %q", url)
	}
}

func TestVerifyStripeSignature_EmptySecret(t *testing.T) {
	ts := time.Now().Unix()
	payload := []byte(fmt.Sprintf(`{"id":"evt_test","type":"checkout.session.completed","livemode":false,"created":%d,"data":{"object":{}}}`, ts))
	header := fmt.Sprintf("t=%d,v1=somesig", ts)

	err := VerifyStripeSignature("", payload, header)
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
	if !errors.Is(err, ErrStripeSignatureInvalid) {
		t.Fatalf("expected ErrStripeSignatureInvalid, got %v", err)
	}
}

func TestVerifyStripeSignature_EmptyHeader(t *testing.T) {
	payload := []byte(`{"id":"evt_test"}`)
	err := VerifyStripeSignature("whsec_test", payload, "")
	if err == nil {
		t.Fatal("expected error for empty header")
	}
	if !errors.Is(err, ErrStripeSignatureInvalid) {
		t.Fatalf("expected ErrStripeSignatureInvalid, got %v", err)
	}
}

func TestVerifyStripeSignature_MissingTimestamp(t *testing.T) {
	payload := []byte(`{"id":"evt_test"}`)
	err := VerifyStripeSignature("whsec_test", payload, "v1=abc123")
	if err == nil {
		t.Fatal("expected error for missing timestamp")
	}
	if !errors.Is(err, ErrStripeSignatureInvalid) {
		t.Fatalf("expected ErrStripeSignatureInvalid, got %v", err)
	}
}

func TestVerifyStripeSignature_MissingV1Sig(t *testing.T) {
	ts := time.Now().Unix()
	payload := []byte(`{"id":"evt_test"}`)
	err := VerifyStripeSignature("whsec_test", payload, fmt.Sprintf("t=%d", ts))
	if err == nil {
		t.Fatal("expected error for missing v1 signature")
	}
	if !errors.Is(err, ErrStripeSignatureInvalid) {
		t.Fatalf("expected ErrStripeSignatureInvalid, got %v", err)
	}
}

func TestVerifyStripeSignature_InvalidTimestampFormat(t *testing.T) {
	payload := []byte(`{"id":"evt_test"}`)
	err := VerifyStripeSignature("whsec_test", payload, "t=not-a-number,v1=abc123")
	if err == nil {
		t.Fatal("expected error for invalid timestamp format")
	}
	if !errors.Is(err, ErrStripeSignatureInvalid) {
		t.Fatalf("expected ErrStripeSignatureInvalid, got %v", err)
	}
}

func TestVerifyStripeSignature_MultipleV1Sigs(t *testing.T) {
	secret := "whsec_test"
	ts := time.Now().Unix()
	payload := []byte(fmt.Sprintf(`{"id":"evt_test","type":"checkout.session.completed","livemode":false,"created":%d,"data":{"object":{}}}`, ts))

	// Create a valid signature
	validHeader := stripeSignatureHeader(t, secret, payload, ts)

	// Add extra invalid v1 sigs - extract the valid sig and add invalid ones before it
	parts := strings.Split(validHeader, ",")
	header := fmt.Sprintf("t=%d,v1=invalid1,v1=invalid2,%s", ts, parts[1])

	if err := VerifyStripeSignature(secret, payload, header); err != nil {
		t.Fatalf("expected success with valid sig among multiple, got %v", err)
	}
}

func TestVerifyStripeSignature_MalformedParts(t *testing.T) {
	payload := []byte(`{"id":"evt_test"}`)
	ts := time.Now().Unix()

	// Header with empty parts and malformed key-value pairs
	err := VerifyStripeSignature("whsec_test", payload, fmt.Sprintf("t=%d,,v1=abc,badpart,=nokey,novalue=", ts))
	if err == nil {
		t.Fatal("expected error")
	}
}
