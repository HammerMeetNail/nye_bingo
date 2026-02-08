// Year of Bingo - Billing/Premium Module (scaffold)

window.App = window.App || {};
var App = window.App;

Object.assign(App, {
  storePendingPremiumCode(code) {
    const raw = String(code || '').trim();
    if (!raw) return;
    // Do not validate against the server until the user is authenticated.
    sessionStorage.setItem('pendingPremiumCode', raw);
  },

  consumePendingPremiumCode() {
    const code = sessionStorage.getItem('pendingPremiumCode');
    if (!code) return null;
    sessionStorage.removeItem('pendingPremiumCode');
    return code;
  },

  peekPendingPremiumCode() {
    return sessionStorage.getItem('pendingPremiumCode') || '';
  },

  clearPendingPremiumCode() {
    sessionStorage.removeItem('pendingPremiumCode');
  },

  async loadBillingStatus() {
    const statusEl = document.getElementById('billing-status');
    if (!statusEl) return;

    try {
      const status = await API.billing.getStatus();
      this.applyBillingStatus(status);
      const badgeSlot = document.getElementById('premium-badge-slot');
      if (badgeSlot) {
        badgeSlot.innerHTML = this.isPremium ? '<span class="badge badge-premium">Premium</span>' : '';
      }
      this.renderBillingStatus(statusEl, status);
      await this.refreshPremiumAIStatus();
    } catch (error) {
      statusEl.innerHTML = '<p class="text-muted" id="billing-error"></p>';
      const errorEl = document.getElementById('billing-error');
      if (errorEl) errorEl.textContent = error.message;
    }
  },

  renderBillingStatus(container, status) {
    if (!status?.billing_enabled) {
      container.innerHTML = `
        <p class="text-muted">Billing is not available right now.</p>
      `;
      return;
    }

    const plan = status.is_premium ? 'Premium' : 'Free';
    const source = status.source || 'none';
    const periodEnd = status.current_period_end ? new Date(status.current_period_end) : null;
    const periodText = periodEnd ? periodEnd.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' }) : null;

    if (status.is_premium) {
      let timingLine = '';
      let noteLine = '';

      const willRenew = source === 'stripe_subscription' && !status.cancel_at_period_end && status.status !== 'canceled';
      const isSubscription = source === 'stripe_subscription';
      const isNonExpiring = !periodText && ['stripe_lifetime', 'code', 'grant'].includes(source);

      if (periodText) {
        if (isSubscription) {
          if (willRenew) {
            timingLine = `Renews ${periodText}`;
          } else {
            timingLine = `Active until ${periodText}`;
            noteLine = 'Will not renew.';
          }
        } else {
          timingLine = `Expires ${periodText}`;
        }
      } else if (isNonExpiring) {
        timingLine = 'No expiration';
      }

      const actions = isSubscription
        ? `<button class="btn btn-secondary btn-sm" data-action="open-billing-portal">Manage Subscription</button>`
        : '';

      container.innerHTML = `
        <div class="billing-plan">
          <div class="billing-plan__row">
            <div>
              <div class="billing-plan__label">Plan</div>
              <div class="billing-plan__value">${plan}</div>
              ${timingLine ? `<div class="text-muted">${timingLine}</div>` : ''}
              ${noteLine ? `<div class="text-muted">${noteLine}</div>` : ''}
            </div>
            <div class="billing-plan__actions">
              ${actions}
            </div>
          </div>
        </div>
      `;
      return;
    }

    container.innerHTML = `
      <div class="billing-plan">
        <div class="billing-plan__row">
          <div>
            <div class="billing-plan__label">Plan</div>
            <div class="billing-plan__value">${plan}</div>
          </div>
          <div class="billing-plan__actions">
            <button class="btn btn-primary btn-sm" data-action="open-upgrade-modal">Upgrade to Premium</button>
          </div>
        </div>
      </div>
    `;
  },

  handleBillingReturn() {
    const params = new URLSearchParams(window.location.search);
    const billingResult = params.get('billing');
    if (!billingResult) return;

    if (billingResult === 'cancel') {
      this.toast('Checkout canceled');
      this.stripQueryParams(['billing', 'session_id']);
      return;
    }

    if (billingResult === 'success') {
      this.stripQueryParams(['billing', 'session_id']);
      this.openModal('Processing Upgrade', `
        <div class="finalize-confirm-modal">
          <p class="text-muted">Processing your upgrade…</p>
          <div class="text-center"><div class="spinner spinner--small spinner--spaced"></div></div>
          <button class="btn btn-ghost" data-action="close-modal">Close</button>
        </div>
      `);
      this.pollBillingStatusUntilPremium();
    }
  },

  stripQueryParams(keys) {
    const url = new URL(window.location.href);
    keys.forEach((k) => url.searchParams.delete(k));
    window.history.replaceState({}, '', url.toString());
  },

  async pollBillingStatusUntilPremium() {
    const start = Date.now();
    const timeoutMs = 60000;

    while (Date.now() - start < timeoutMs) {
      try {
        const status = await API.billing.getStatus();
        this.applyBillingStatus(status);

        if (status.is_premium) {
          this.toast('Premium activated!', 'success');
          this.closeModal();
          await this.loadBillingStatus();
          return;
        }
      } catch (error) {
        // Ignore transient errors while polling.
      }
      await new Promise(resolve => setTimeout(resolve, 2000));
    }

    this.openModal('Almost There', `
      <div class="finalize-confirm-modal">
        <p class="text-muted">Your payment succeeded, but Premium is still processing. If this doesn’t update in a few minutes, contact support.</p>
        <button class="btn btn-ghost" data-action="close-modal">Close</button>
      </div>
    `);
  },

  openUpgradeModal() {
    // The Premium page renders quickly and loads billing status async.
    // If a user clicks before status loads, fetch it here so the modal can open reliably.
    if (!this.billingStatus) {
      API.billing.getStatus()
        .then((status) => {
          this.applyBillingStatus(status);
          this.openUpgradeModal();
        })
        .catch((error) => {
          this.toast(error.message, 'error');
        });
      return;
    }

    if (!this.billingStatus.billing_enabled) {
      this.toast('Billing is not available right now', 'error');
      return;
    }

    if (this.billingStatus.is_premium) {
      this.openModal('Premium', `
        <div class="finalize-confirm-modal">
          <p class="text-muted">You're already Premium.</p>
          <div class="upgrade-actions mt-md">
            <button class="btn btn-secondary" data-action="open-billing-portal">Manage subscription</button>
            <button class="btn btn-ghost" data-action="close-modal">Close</button>
          </div>
        </div>
      `);
      return;
    }

    this.openModal('Upgrade to Premium', `
      <div class="upgrade-modal" id="upgrade-modal" data-premium-kind="subscription" data-interval="month" data-tip-amount="0">
        <p class="text-muted">Premium only adds features — nothing you use today gets removed.</p>

        <h4 class="mt-lg">Premium Benefits</h4>
        <ul class="upgrade-list">
          <li>Premium badge (visible to friends)</li>
          <li>Templates + 1‑click New Year rollover</li>
          <li>AI Enhancements: 100/month</li>
        </ul>

        <h4 class="mt-lg">Premium plan</h4>
        <div class="upgrade-actions" role="group" aria-label="Premium plan">
          <button class="btn btn-primary" data-action="select-upgrade-premium" data-premium-kind="subscription" data-interval="month" aria-pressed="true">Monthly</button>
          <button class="btn btn-secondary" data-action="select-upgrade-premium" data-premium-kind="subscription" data-interval="year" aria-pressed="false">Yearly</button>
          <button class="btn btn-secondary" data-action="select-upgrade-premium" data-premium-kind="lifetime" aria-pressed="false">Lifetime</button>
          <button class="btn btn-ghost" data-action="select-upgrade-premium" data-premium-kind="" aria-pressed="false">Tip only</button>
        </div>

        <h4 class="mt-lg">Add a tip (optional)</h4>
        <div class="upgrade-actions" role="group" aria-label="Tip amount">
          <button class="btn btn-primary" data-action="select-upgrade-tip" data-tip-amount="0" aria-pressed="true">No tip</button>
          <button class="btn btn-ghost" data-action="select-upgrade-tip" data-tip-amount="5" aria-pressed="false">$5</button>
          <button class="btn btn-ghost" data-action="select-upgrade-tip" data-tip-amount="10" aria-pressed="false">$10</button>
          <button class="btn btn-ghost" data-action="select-upgrade-tip" data-tip-amount="20" aria-pressed="false">$20</button>
        </div>

        <p class="text-muted text-sm mt-md" id="upgrade-summary"></p>

        <div class="upgrade-footer mt-lg">
          <button class="btn btn-primary btn-lg" id="upgrade-checkout" data-action="billing-checkout-selected">Checkout</button>
          <button class="btn btn-secondary btn-lg" data-action="close-modal">Close</button>
        </div>
      </div>
    `);

    // Ensure initial UI state is consistent (e.g. after hot reload / DOM changes).
    const modal = document.getElementById('upgrade-modal');
    if (modal) this.updateUpgradeModalUI(modal);
  },

  getUpgradeModalState(modal) {
    const premiumKind = modal?.dataset?.premiumKind ?? 'subscription';
    const interval = modal?.dataset?.interval ?? 'month';
    const tipAmount = parseInt(modal?.dataset?.tipAmount ?? '0', 10);
    return {
      premiumKind,
      interval,
      tipAmount: Number.isNaN(tipAmount) ? 0 : tipAmount,
    };
  },

  setUpgradeModalState(modal, nextState) {
    if (!modal) return;
    if (typeof nextState.premiumKind === 'string') modal.dataset.premiumKind = nextState.premiumKind;
    if (typeof nextState.interval === 'string') modal.dataset.interval = nextState.interval;
    if (typeof nextState.tipAmount === 'number') modal.dataset.tipAmount = String(nextState.tipAmount);
    this.updateUpgradeModalUI(modal);
  },

  updateUpgradeModalUI(modal) {
    if (!modal) return;
    const state = this.getUpgradeModalState(modal);

    // Premium buttons
    const premiumButtons = modal.querySelectorAll('[data-action="select-upgrade-premium"]');
    premiumButtons.forEach((btn) => {
      const kind = btn.dataset.premiumKind ?? '';
      const interval = btn.dataset.interval ?? '';
      const isSelected = kind === state.premiumKind && (kind !== 'subscription' || interval === state.interval);
      btn.setAttribute('aria-pressed', isSelected ? 'true' : 'false');
      btn.classList.toggle('btn-primary', isSelected);
      btn.classList.toggle('btn-secondary', !isSelected && kind !== '');
      btn.classList.toggle('btn-ghost', !isSelected && kind === '');
    });

    // Tip buttons
    const tipButtons = modal.querySelectorAll('[data-action="select-upgrade-tip"]');
    tipButtons.forEach((btn) => {
      const amount = parseInt(btn.dataset.tipAmount ?? '0', 10);
      const isSelected = !Number.isNaN(amount) && amount === state.tipAmount;
      btn.setAttribute('aria-pressed', isSelected ? 'true' : 'false');
      btn.classList.toggle('btn-primary', isSelected);
      btn.classList.toggle('btn-ghost', !isSelected);
    });

    const summaryEl = modal.querySelector('#upgrade-summary');
    const parts = [];
    if (state.premiumKind === 'subscription') {
      parts.push(state.interval === 'year' ? 'Premium (yearly)' : 'Premium (monthly)');
    } else if (state.premiumKind === 'lifetime') {
      parts.push('Premium (lifetime)');
    } else {
      parts.push('Tip jar');
    }
    if (state.tipAmount > 0) {
      parts.push(`+$${state.tipAmount} tip`);
    }
    if (summaryEl) summaryEl.textContent = `Selected: ${parts.join(' ')}`;

    const checkoutBtn = modal.querySelector('#upgrade-checkout');
    const isValid = !(state.premiumKind === '' && state.tipAmount === 0);
    if (checkoutBtn) checkoutBtn.disabled = !isValid;
  },

  selectUpgradePremium(target) {
    const modal = target?.closest?.('.upgrade-modal');
    if (!modal) return;
    const kind = target?.dataset?.premiumKind ?? '';
    const interval = target?.dataset?.interval ?? '';
    if (kind === 'subscription') {
      this.setUpgradeModalState(modal, { premiumKind: 'subscription', interval: interval === 'year' ? 'year' : 'month' });
      return;
    }
    if (kind === 'lifetime') {
      this.setUpgradeModalState(modal, { premiumKind: 'lifetime', interval: '' });
      return;
    }
    // Tip-only
    this.setUpgradeModalState(modal, { premiumKind: '', interval: '' });
  },

  selectUpgradeTip(target) {
    const modal = target?.closest?.('.upgrade-modal');
    if (!modal) return;
    const amount = parseInt(target?.dataset?.tipAmount ?? '0', 10);
    if (![0, 5, 10, 20].includes(amount)) {
      this.toast('Invalid tip amount', 'error');
      return;
    }
    this.setUpgradeModalState(modal, { tipAmount: amount });
  },

  async startSelectedCheckout(target) {
    const modal = target?.closest?.('.upgrade-modal') || document.getElementById('upgrade-modal');
    if (!modal) return;
    const state = this.getUpgradeModalState(modal);

    if (state.premiumKind === '' && state.tipAmount === 0) {
      this.toast('Select Premium or a tip', 'error');
      return;
    }
    if (state.premiumKind === 'subscription' && !['month', 'year'].includes(state.interval)) {
      this.toast('Invalid interval', 'error');
      return;
    }

    const payload = {
      premium_kind: state.premiumKind,
      interval: state.premiumKind === 'subscription' ? state.interval : '',
      tip_amount: state.tipAmount,
    };

    try {
      this.setButtonLoading(target, true);
      const resp = await API.billing.createCheckoutSession(payload);
      if (resp?.url) window.location.href = resp.url;
    } catch (error) {
      this.toast(error.message, 'error');
    } finally {
      this.setButtonLoading(target, false);
    }
  },

  async openBillingPortal() {
    try {
      const resp = await API.billing.createPortalSession();
      if (resp?.url) {
        window.location.href = resp.url;
      }
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async startSubscriptionCheckout(target) {
    const interval = target?.dataset?.interval;
    if (interval !== 'month' && interval !== 'year') {
      this.toast('Invalid interval', 'error');
      return;
    }

    try {
      this.setButtonLoading(target, true);
      const resp = await API.billing.createSubscriptionCheckoutSession(interval);
      if (resp?.url) window.location.href = resp.url;
    } catch (error) {
      this.toast(error.message, 'error');
    } finally {
      this.setButtonLoading(target, false);
    }
  },

  async startLifetimeCheckout() {
    try {
      const resp = await API.billing.createLifetimeCheckoutSession();
      if (resp?.url) window.location.href = resp.url;
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async startTipCheckout(target) {
    const amount = parseInt(target?.dataset?.amount || '', 10);
    if (![5, 10, 20].includes(amount)) {
      this.toast('Invalid tip amount', 'error');
      return;
    }
    try {
      this.setButtonLoading(target, true);
      const resp = await API.billing.createTipCheckoutSession(amount);
      if (resp?.url) window.location.href = resp.url;
    } catch (error) {
      this.toast(error.message, 'error');
    } finally {
      this.setButtonLoading(target, false);
    }
  },

  async redeemPremiumCode(target) {
    const modal = target?.closest?.('.upgrade-modal');
    const input = modal ? modal.querySelector('#premium-code-input') : document.getElementById('premium-code-input');
    const errorEl = modal ? modal.querySelector('#premium-code-error') : document.getElementById('premium-code-error');
    const code = input?.value || '';

    if (errorEl) {
      errorEl.textContent = '';
      errorEl.classList.add('hidden');
    }

    if (!code.trim()) {
      if (errorEl) {
        errorEl.textContent = 'Enter a code';
        errorEl.classList.remove('hidden');
      } else {
        this.toast('Enter a code', 'error');
      }
      return;
    }

    if (!this.user) {
      this.storePendingPremiumCode(code);
      this.openModal('Redeem Premium Code', `
        <div class="finalize-confirm-modal">
          <p class="text-muted">Create an account (or sign in) to redeem your code. We'll apply it right after.</p>
          <div class="upgrade-actions mt-md">
            <a href="/register" class="btn btn-primary" data-action="set-post-auth-next" data-next="/premium?redeem=1">Create account</a>
            <a href="/login" class="btn btn-secondary" data-action="set-post-auth-next" data-next="/premium?redeem=1">Sign in</a>
          </div>
          <div class="mt-lg">
            <button class="btn btn-ghost" data-action="close-modal">Close</button>
          </div>
        </div>
      `);
      return;
    }

    try {
      this.setButtonLoading(target, true);
      await API.billing.redeemCode(code);
      this.toast('Premium activated!', 'success');
      if (input) input.value = '';
      this.clearPendingPremiumCode();
      this.closeModal();
      // Best-effort refresh depending on where the user is.
      if (this.currentView === 'premium') {
        try {
          const status = await API.billing.getStatus();
          this.applyBillingStatus(status);
          const statusEl = document.getElementById('premium-billing-status');
          if (statusEl) this.renderBillingStatus(statusEl, status);
        } catch (error) {
          // Ignore refresh failures; user can refresh page.
        }
      } else {
        await this.loadBillingStatus();
      }
    } catch (error) {
      if (errorEl) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      }
      this.toast(error.message, 'error');
      if (input) input.value = '';
      this.clearPendingPremiumCode();
      input?.focus?.();
    } finally {
      this.setButtonLoading(target, false);
    }
  },

  async renderPremium(container, queryParams) {
    this.currentView = 'premium';

    container.innerHTML = `
      <div class="premium-page">
        <div class="premium-hero">
          <div class="premium-hero__title">
            <i class="fa-solid fa-star premium-hero__icon" aria-hidden="true"></i>
            <h1>Premium</h1>
          </div>
          <p class="text-muted premium-hero__subtitle">
            Premium helps keep Year of Bingo running and funds new, additive features. Nothing you use today gets removed.
          </p>
          <div class="premium-hero__cta" id="premium-cta-slot"></div>
        </div>

        <div class="premium-grid">
          <div class="card premium-feature">
            <h3>Premium badge</h3>
            <p class="text-muted">Show a Premium badge on your profile and to friends.</p>
          </div>
          <div class="card premium-feature">
            <h3>AI Enhancements</h3>
            <p class="text-muted">Get 100 premium AI actions per month for assist/regenerate/fill features.</p>
          </div>
          <div class="card premium-feature">
            <h3>Templates + rollover</h3>
            <p class="text-muted">Create reusable templates and roll over a card to a new year in one click.</p>
          </div>
        </div>

        <div class="card premium-status">
          <h2>Your plan</h2>
          <div id="premium-billing-status" class="billing-status">
            <div class="text-center"><div class="spinner spinner--small"></div></div>
          </div>
          <p id="premium-ai-status" class="text-muted text-sm mt-md"></p>
          <p class="text-muted text-sm mt-md">
            After checkout, you'll return to your Profile while we activate Premium (webhook-driven; may take a moment).
          </p>
        </div>

        <div class="premium-fineprint text-muted text-sm">
          <p>
            Manage/cancel anytime via the Stripe customer portal. Need help? <a href="/support">Contact support</a>.
          </p>
        </div>
      </div>
    `;

    const ctaSlot = document.getElementById('premium-cta-slot');
    const statusEl = document.getElementById('premium-billing-status');

    const wantsRedeem = queryParams?.get?.('redeem') === '1';
    if (wantsRedeem) {
      this.stripQueryParams(['redeem']);
    }

    if (!this.user) {
      if (ctaSlot) {
        ctaSlot.innerHTML = `
          <div class="premium-hero__cta-row">
            <a href="/login" class="btn btn-primary" data-action="set-post-auth-next" data-next="/premium">Sign in to upgrade</a>
            <a href="/register" class="btn btn-secondary" data-action="set-post-auth-next" data-next="/premium">Create account</a>
            <button class="btn btn-ghost" data-action="open-premium-code-modal">Have a code?</button>
          </div>
        `;
      }
      if (statusEl) {
        statusEl.innerHTML = `<p class="text-muted">Sign in to view billing status and upgrade options.</p>`;
      }
      return;
    }

    if (ctaSlot) {
      // Render a usable CTA immediately; billing status loads async and will refine this UI.
      if (this.isPremium) {
        ctaSlot.innerHTML = `
          <div class="premium-hero__cta-row">
            <a href="/profile" class="btn btn-ghost">View profile</a>
          </div>
        `;
      } else {
        ctaSlot.innerHTML = `
          <div class="premium-hero__cta-row">
            <button class="btn btn-primary" data-action="open-upgrade-modal">Upgrade to Premium</button>
            <button class="btn btn-secondary" data-action="open-premium-code-modal">Have a code?</button>
          </div>
        `;
      }
    }

    let status = null;
    try {
      status = await API.billing.getStatus();
      this.applyBillingStatus(status);
      if (statusEl) this.renderBillingStatus(statusEl, status);
      await this.refreshPremiumAIStatus();
    } catch (error) {
      if (statusEl) {
        statusEl.innerHTML = '<p class="text-muted" id="premium-billing-error"></p>';
        const errorEl = document.getElementById('premium-billing-error');
        if (errorEl) errorEl.textContent = error.message;
      }
      this.renderPremiumAIStatus();
    }

    if (ctaSlot) {
      // If we couldn't load status, keep the optimistic CTA already rendered above.
      if (!status) {
        // Keep the existing CTA as a best-effort (user can still attempt checkout).
        // Billing endpoints will respond with a clear error if billing is actually disabled.
      } else if (!status.billing_enabled) {
        ctaSlot.innerHTML = `<p class="text-muted">Premium is not available right now.</p>`;
      } else if (status.is_premium) {
        const isSubscription = status.source === 'stripe_subscription';
        ctaSlot.innerHTML = `
          <div class="premium-hero__cta-row">
            <a href="/profile" class="btn btn-ghost">View profile</a>
            ${isSubscription ? `<button class="btn btn-secondary" data-action="open-billing-portal">Manage subscription</button>` : ''}
          </div>
        `;
      } else {
        ctaSlot.innerHTML = `
          <div class="premium-hero__cta-row">
            <button class="btn btn-primary" data-action="open-upgrade-modal">Upgrade to Premium</button>
            <button class="btn btn-secondary" data-action="open-premium-code-modal">Have a code?</button>
          </div>
        `;
      }
    }

    const codeToRedeem = wantsRedeem ? this.consumePendingPremiumCode() : '';
    if (codeToRedeem) {
      try {
        await API.billing.redeemCode(codeToRedeem);
        this.toast('Premium activated!', 'success');
        // Refresh status UI (best-effort).
        try {
          const refreshed = await API.billing.getStatus();
          this.applyBillingStatus(refreshed);
          if (statusEl) this.renderBillingStatus(statusEl, refreshed);
        } catch (error) {
          // Ignore refresh failures; user can refresh page.
        }
      } catch (error) {
        // If redeem fails, do not keep/auto-retry the code; prompt the user to re-enter.
        this.toast(error.message, 'error');
        this.openPremiumCodeModal({ errorMessage: error.message, initialCode: '' });
      }
    }
  },

  openPremiumCodeModal({ errorMessage = '', initialCode = null } = {}) {
    const pending = initialCode === null ? this.peekPendingPremiumCode() : String(initialCode || '');
    this.openModal('Have a code?', `
      <div class="premium-code-modal">
        <p class="text-muted">Redeem a Premium code to activate Premium.</p>
        <div class="form-error hidden mt-md" id="premium-code-error" role="alert"></div>
        <div class="upgrade-redeem mt-md">
          <input id="premium-code-input" class="form-input" type="text" autocomplete="off" placeholder="YOBP-...." value="${this.escapeHtml(pending)}" />
          <button class="btn btn-secondary" data-action="billing-redeem-code">Redeem</button>
        </div>
        <div class="mt-lg text-center">
          <button class="btn btn-ghost" data-action="close-modal">Close</button>
        </div>
      </div>
    `);
    const input = document.getElementById('premium-code-input');
    input?.focus?.();

    const errorEl = document.getElementById('premium-code-error');
    if (errorEl && String(errorMessage || '').trim()) {
      errorEl.textContent = String(errorMessage);
      errorEl.classList.remove('hidden');
    }
  },
});
