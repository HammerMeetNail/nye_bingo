package billing

import (
	"errors"
	"time"
)

type CheckoutInterval string

const (
	IntervalMonth CheckoutInterval = "month"
	IntervalYear  CheckoutInterval = "year"
)

type CheckoutTipAmount int

const (
	Tip5  CheckoutTipAmount = 5
	Tip10 CheckoutTipAmount = 10
	Tip20 CheckoutTipAmount = 20
)

type BillingStatus struct {
	BillingEnabled    bool       `json:"billing_enabled"`
	IsPremium         bool       `json:"is_premium"`
	Plan              string     `json:"plan"`
	Status            string     `json:"status"`
	CurrentPeriodEnd  *time.Time `json:"current_period_end"`
	CancelAtPeriodEnd bool       `json:"cancel_at_period_end"`
}

var (
	ErrBillingDisabled        = errors.New("billing disabled")
	ErrInvalidCode            = errors.New("invalid code")
	ErrPremiumRequired        = errors.New("premium required")
	ErrStripeSignatureInvalid = errors.New("invalid stripe signature")
)
