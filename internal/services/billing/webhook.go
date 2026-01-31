package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
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

	// Reject events with timestamps older than the tolerance to prevent replay attacks.
	now := time.Now().Unix()
	if now-tsInt > toleranceSec {
		return fmt.Errorf("%w: timestamp too old", ErrStripeSignatureInvalid)
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

	return ErrStripeSignatureInvalid
}
