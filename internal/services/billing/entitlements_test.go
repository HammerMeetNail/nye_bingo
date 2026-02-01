package billing

import (
	"testing"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func TestIsPremium(t *testing.T) {
	now := time.Now().UTC()

	if IsPremium(nil, now) {
		t.Fatal("expected false for nil user")
	}

	user := &models.User{BillingPlan: "free"}
	if IsPremium(user, now) {
		t.Fatal("expected false for free plan")
	}

	user = &models.User{BillingPlan: "premium"}
	if !IsPremium(user, now) {
		t.Fatal("expected true for premium lifetime")
	}

	expired := now.Add(-time.Hour)
	user = &models.User{BillingPlan: "premium", BillingCurrentPeriodEnd: &expired}
	if IsPremium(user, now) {
		t.Fatal("expected false for expired premium")
	}

	future := now.Add(time.Hour)
	user = &models.User{BillingPlan: "premium", BillingCurrentPeriodEnd: &future}
	if !IsPremium(user, now) {
		t.Fatal("expected true for active premium")
	}
}
