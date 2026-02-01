package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultWebhookTimestampTolerance is the maximum age of a webhook event before it's rejected.
// Stripe recommends 5 minutes (300 seconds) to prevent replay attacks.
const DefaultWebhookTimestampTolerance = 300

func VerifyStripeSignature(secret string, payload []byte, sigHeader string) error {
	return VerifyStripeSignatureWithTolerance(secret, payload, sigHeader, DefaultWebhookTimestampTolerance)
}

func VerifyStripeSignatureWithTolerance(secret string, payload []byte, sigHeader string, toleranceSec int64) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return fmt.Errorf("%w: missing webhook secret", ErrStripeSignatureInvalid)
	}
	sigHeader = strings.TrimSpace(sigHeader)
	if sigHeader == "" {
		return ErrStripeSignatureInvalid
	}

	var ts string
	var v1Sigs []string
	parts := strings.Split(sigHeader, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		switch k {
		case "t":
			ts = v
		case "v1":
			v1Sigs = append(v1Sigs, v)
		}
	}

	if ts == "" || len(v1Sigs) == 0 {
		return ErrStripeSignatureInvalid
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return ErrStripeSignatureInvalid
	}

	// Reject events whose timestamps differ from the server time by more than the tolerance
	// to prevent replay attacks and limit clock skew.
	now := time.Now().Unix()
	var diff int64
	if now >= tsInt {
		diff = now - tsInt
	} else {
		diff = tsInt - now
	}
	if diff > toleranceSec {
		return fmt.Errorf("%w: timestamp outside allowed tolerance", ErrStripeSignatureInvalid)
	}

	signed := []byte(fmt.Sprintf("%s.%s", ts, string(payload)))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(signed)
	expected := hex.EncodeToString(mac.Sum(nil))

	for _, sig := range v1Sigs {
		if subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) == 1 {
			return nil
		}
	}

	// Debug aid for E2E: signature mismatches are hard to inspect otherwise.
	// We log only hashes of the secret and payload, plus the expected/provided v1 values.
	if os.Getenv("E2E_DEBUG_STRIPE_SIG") == "1" {
		sum := sha256.Sum256(payload)
		secretSum := sha256.Sum256([]byte(secret))
		slog.Info(
			"billing: stripe signature mismatch",
			"ts", ts,
			"expected_v1", expected,
			"provided_v1", strings.Join(v1Sigs, "|"),
			"payload_sha256", hex.EncodeToString(sum[:]),
			"secret_sha256", hex.EncodeToString(secretSum[:]),
		)
	}

	return ErrStripeSignatureInvalid
}
