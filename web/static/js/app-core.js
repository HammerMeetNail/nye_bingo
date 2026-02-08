// Year of Bingo - App Core Module (scaffold)
// SCAFFOLD: Not yet loaded in production. See plans/refactor.md for extraction status.

window.App = window.App || {};
var App = window.App;

if (!App._moduleCoreLoaded) {
  App._moduleCoreLoaded = true;

Object.assign(App, {
  qs(id) {
    return document.getElementById(id);
  },

  setText(el, text) {
    if (!el) return;
    el.textContent = text ?? '';
  },

  normalizeEntitlements(entitlements) {
    const source = entitlements && typeof entitlements === 'object' ? entitlements : {};
    return {
      templates: !!source.templates,
      edit_after_finalize: !!source.edit_after_finalize,
      ai_enhancements: !!source.ai_enhancements,
    };
  },

  applyAuthEntitlements(response) {
    this.user = response?.user || null;
    this.isPremium = !!response?.is_premium;
    this.entitlements = this.normalizeEntitlements(response?.features);
    if (!this.entitlements.ai_enhancements) {
      this.premiumAIStatus = null;
    }
  },

  applyBillingStatus(status) {
    this.billingStatus = status || null;
    this.isPremium = !!status?.is_premium;
    this.entitlements = this.normalizeEntitlements(status?.features);
    if (!this.entitlements.ai_enhancements) {
      this.premiumAIStatus = null;
    }
  },

  hasFeature(feature) {
    const key = String(feature || '').trim();
    if (!key) return false;
    if (Object.prototype.hasOwnProperty.call(this.entitlements || {}, key)) {
      return !!this.entitlements[key];
    }
    return this.isPremium;
  },

  setRobotsMeta(content) {
    const existing = document.querySelector('meta[name="robots"]');
    if (!content) {
      if (existing) existing.remove();
      return;
    }
    const meta = existing || document.createElement('meta');
    meta.setAttribute('name', 'robots');
    meta.setAttribute('content', content);
    if (!existing) {
      document.head.appendChild(meta);
    }
  },
});
}
