package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/services/billing"
)

type BillingHandler struct {
	billing *billing.Service
}

func NewBillingHandler(svc *billing.Service) *BillingHandler {
	return &BillingHandler{billing: svc}
}

func (h *BillingHandler) Status(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	status := h.billing.Status(user, time.Now())
	writeJSON(w, http.StatusOK, status)
}

func (h *BillingHandler) CheckoutSubscription(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req struct {
		Interval string `json:"interval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	url, err := h.billing.CreateSubscriptionCheckoutURL(r.Context(), user, billing.CheckoutInterval(req.Interval))
	if errors.Is(err, billing.ErrBillingDisabled) {
		writeError(w, http.StatusNotFound, "Billing is not available")
		return
	}
	if errors.Is(err, billing.ErrInvalidInterval) {
		writeError(w, http.StatusBadRequest, "Invalid interval")
		return
	}
	if err != nil {
		slog.Error("billing: subscription checkout failed", "error", err, "user_id", user.ID)
		writeError(w, http.StatusInternalServerError, "Unable to start checkout")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *BillingHandler) CheckoutLifetime(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	url, err := h.billing.CreateLifetimeCheckoutURL(r.Context(), user)
	if errors.Is(err, billing.ErrBillingDisabled) {
		writeError(w, http.StatusNotFound, "Billing is not available")
		return
	}
	if err != nil {
		slog.Error("billing: lifetime checkout failed", "error", err, "user_id", user.ID)
		writeError(w, http.StatusInternalServerError, "Unable to start checkout")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *BillingHandler) CheckoutTip(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req struct {
		Amount int `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	url, err := h.billing.CreateTipCheckoutURL(r.Context(), user, billing.CheckoutTipAmount(req.Amount))
	if errors.Is(err, billing.ErrBillingDisabled) {
		writeError(w, http.StatusNotFound, "Billing is not available")
		return
	}
	if errors.Is(err, billing.ErrInvalidTipAmount) {
		writeError(w, http.StatusBadRequest, "Invalid tip amount")
		return
	}
	if err != nil {
		slog.Error("billing: tip checkout failed", "error", err, "user_id", user.ID)
		writeError(w, http.StatusInternalServerError, "Unable to start checkout")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *BillingHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req struct {
		PremiumKind string `json:"premium_kind"`
		Interval    string `json:"interval"`
		TipAmount   int    `json:"tip_amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	url, err := h.billing.CreateCombinedCheckoutURL(r.Context(), user, billing.CombinedCheckoutRequest{
		PremiumKind: billing.CheckoutPremiumKind(req.PremiumKind),
		Interval:    billing.CheckoutInterval(req.Interval),
		TipAmount:   billing.CheckoutTipAmount(req.TipAmount),
	})
	if errors.Is(err, billing.ErrBillingDisabled) {
		writeError(w, http.StatusNotFound, "Billing is not available")
		return
	}
	if errors.Is(err, billing.ErrInvalidInterval) {
		writeError(w, http.StatusBadRequest, "Invalid interval")
		return
	}
	if errors.Is(err, billing.ErrInvalidTipAmount) {
		writeError(w, http.StatusBadRequest, "Invalid tip amount")
		return
	}
	if errors.Is(err, billing.ErrInvalidCheckout) {
		writeError(w, http.StatusBadRequest, "Invalid checkout selection")
		return
	}
	if err != nil {
		slog.Error("billing: checkout failed", "error", err, "user_id", user.ID)
		writeError(w, http.StatusInternalServerError, "Unable to start checkout")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *BillingHandler) Portal(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	url, err := h.billing.CreatePortalURL(r.Context(), user)
	if errors.Is(err, billing.ErrBillingDisabled) {
		writeError(w, http.StatusNotFound, "Billing is not available")
		return
	}
	if err != nil {
		slog.Error("billing: portal session failed", "error", err, "user_id", user.ID)
		writeError(w, http.StatusInternalServerError, "Unable to open billing portal")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (h *BillingHandler) Redeem(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.billing.RedeemCode(r.Context(), user, req.Code, time.Now())
	if errors.Is(err, billing.ErrBillingDisabled) {
		writeError(w, http.StatusNotFound, "Billing is not available")
		return
	}
	if errors.Is(err, billing.ErrInvalidCode) {
		writeError(w, http.StatusBadRequest, "Invalid code")
		return
	}
	if err != nil {
		slog.Error("billing: redeem code failed", "error", err, "user_id", user.ID)
		writeError(w, http.StatusInternalServerError, "Unable to redeem code")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"is_premium": true,
		"features": billing.FeatureEntitlements{
			Templates: true,
		},
	})
}

func (h *BillingHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	if !h.billing.Enabled() {
		writeError(w, http.StatusNotFound, "Billing is not available")
		return
	}

	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	// Some HTTP stacks can reuse Body; make it safe for any downstream.
	r.Body = io.NopCloser(bytes.NewReader(payload))

	sig := r.Header.Get("Stripe-Signature")
	if err := h.billing.HandleWebhook(r.Context(), payload, sig, time.Now()); err != nil {
		if errors.Is(err, billing.ErrStripeSignatureInvalid) {
			writeError(w, http.StatusBadRequest, "invalid signature")
			return
		}
		slog.Error("billing: webhook processing failed", "error", err)
		writeError(w, http.StatusInternalServerError, "webhook processing failed")
		return
	}

	w.WriteHeader(http.StatusOK)
}
