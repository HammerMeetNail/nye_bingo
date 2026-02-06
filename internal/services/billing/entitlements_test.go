package billing

import (
	"testing"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func withFeatureSwitches(t *testing.T, switches FeatureEntitlements) {
	t.Helper()
	prev := GlobalFeatureSwitches()
	SetGlobalFeatureSwitches(switches)
	t.Cleanup(func() {
		SetGlobalFeatureSwitches(prev)
	})
}

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
	withFeatureSwitches(t, FeatureEntitlements{
		Templates:         true,
		EditAfterFinalize: true,
	})

	free := &models.User{BillingPlan: "free"}
	features := Features(free, now)
	if features.Templates {
		t.Fatal("expected templates=false for free user")
	}
	if features.EditAfterFinalize {
		t.Fatal("expected edit_after_finalize=false for free user")
	}

	premium := &models.User{BillingPlan: "premium"}
	features = Features(premium, now)
	if !features.Templates {
		t.Fatal("expected templates=true for premium user")
	}
	if !features.EditAfterFinalize {
		t.Fatal("expected edit_after_finalize=true for premium user")
	}
}

func TestHasFeature(t *testing.T) {
	now := time.Now().UTC()
	withFeatureSwitches(t, FeatureEntitlements{
		Templates:         true,
		EditAfterFinalize: true,
	})
	user := &models.User{BillingPlan: "premium"}

	if !HasFeature(user, now, FeatureTemplates) {
		t.Fatal("expected templates feature for premium user")
	}
	if !HasFeature(user, now, FeatureEditAfterFinalize) {
		t.Fatal("expected edit_after_finalize feature for premium user")
	}
	if HasFeature(user, now, Feature("unknown")) {
		t.Fatal("expected false for unknown feature")
	}
}

func TestHasFeature_GlobalSwitchOverridesPremium(t *testing.T) {
	now := time.Now().UTC()
	user := &models.User{BillingPlan: "premium"}

	withFeatureSwitches(t, FeatureEntitlements{
		Templates:         false,
		EditAfterFinalize: true,
	})
	if HasFeature(user, now, FeatureTemplates) {
		t.Fatal("expected templates feature disabled by global switch")
	}
	if !HasFeature(user, now, FeatureEditAfterFinalize) {
		t.Fatal("expected edit_after_finalize feature enabled")
	}

	withFeatureSwitches(t, FeatureEntitlements{
		Templates:         true,
		EditAfterFinalize: false,
	})
	if !HasFeature(user, now, FeatureTemplates) {
		t.Fatal("expected templates feature enabled")
	}
	if HasFeature(user, now, FeatureEditAfterFinalize) {
		t.Fatal("expected edit_after_finalize feature disabled by global switch")
	}
}
