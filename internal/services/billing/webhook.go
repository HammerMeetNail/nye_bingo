package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

func VerifyStripeSignature(secret string, payload []byte, sigHeader string) error {
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
	if _, err := strconv.ParseInt(ts, 10, 64); err != nil {
		return ErrStripeSignatureInvalid
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
