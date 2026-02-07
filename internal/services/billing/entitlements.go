package billing

import (
	"sync"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

type Feature string

const (
	FeatureTemplates         Feature = "templates"
	FeatureEditAfterFinalize Feature = "edit_after_finalize"
)

type FeatureEntitlements struct {
	Templates         bool `json:"templates"`
	EditAfterFinalize bool `json:"edit_after_finalize"`
}

var (
	globalFeatureSwitchesMu sync.RWMutex
	globalFeatureSwitches   = FeatureEntitlements{
		Templates:         true,
		EditAfterFinalize: true,
	}
)

func SetGlobalFeatureSwitches(switches FeatureEntitlements) {
	globalFeatureSwitchesMu.Lock()
	defer globalFeatureSwitchesMu.Unlock()
	globalFeatureSwitches = switches
}

func GlobalFeatureSwitches() FeatureEntitlements {
	globalFeatureSwitchesMu.RLock()
	defer globalFeatureSwitchesMu.RUnlock()
	return globalFeatureSwitches
}

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

func Features(user *models.User, now time.Time) FeatureEntitlements {
	isPremium := IsPremium(user, now)
	switches := GlobalFeatureSwitches()
	return FeatureEntitlements{
		Templates:         isPremium && switches.Templates,
		EditAfterFinalize: isPremium && switches.EditAfterFinalize,
	}
}

func HasFeature(user *models.User, now time.Time, feature Feature) bool {
	entitlements := Features(user, now)
	switch feature {
	case FeatureTemplates:
		return entitlements.Templates
	case FeatureEditAfterFinalize:
		return entitlements.EditAfterFinalize
	default:
		return false
	}
}
