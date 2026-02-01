package billing

import (
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func IsPremium(user *models.User, now time.Time) bool {
	if user == nil {
		return false
	}
	if user.BillingPlan != "premium" {
		return false
	}
	if user.BillingCurrentPeriodEnd == nil {
		return true
	}
	return user.BillingCurrentPeriodEnd.After(now)
}
