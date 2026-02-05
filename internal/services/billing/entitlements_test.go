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

func TestFeatures(t *testing.T) {
	now := time.Now().UTC()

	free := &models.User{BillingPlan: "free"}
	features := Features(free, now)
	if features.Templates {
		t.Fatal("expected templates=false for free user")
	}

	premium := &models.User{BillingPlan: "premium"}
	features = Features(premium, now)
	if !features.Templates {
		t.Fatal("expected templates=true for premium user")
	}
}

func TestHasFeature(t *testing.T) {
	now := time.Now().UTC()
	user := &models.User{BillingPlan: "premium"}

	if !HasFeature(user, now, FeatureTemplates) {
		t.Fatal("expected templates feature for premium user")
	}
	if HasFeature(user, now, Feature("unknown")) {
		t.Fatal("expected false for unknown feature")
	}
}
