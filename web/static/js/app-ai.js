// Year of Bingo - AI Module (scaffold)

window.App = window.App || {};
var App = window.App;

Object.assign(App, {
  formatPremiumAIStatusLine(status) {
    if (!status || typeof status.remaining !== 'number' || typeof status.limit !== 'number') {
      return '';
    }
    const resetsAt = status.resets_at ? new Date(status.resets_at) : null;
    const resetText = resetsAt
      ? resetsAt.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
      : '';
    const suffix = resetText ? ` (resets ${resetText})` : '';
    return `AI Enhancements remaining: ${status.remaining} / ${status.limit}${suffix}`;
  },

  renderPremiumAIStatus() {
    const ids = ['ai-enhancements-status', 'premium-ai-status'];
    ids.forEach((id) => {
      const el = document.getElementById(id);
      if (!el) return;
      if (!this.user || !this.hasFeature('ai_enhancements')) {
        el.textContent = '';
        return;
      }
      if (!this.premiumAIStatus) {
        el.textContent = '';
        return;
      }
      el.textContent = this.formatPremiumAIStatusLine(this.premiumAIStatus);
    });
  },

  async refreshPremiumAIStatus() {
    if (!this.user || !this.hasFeature('ai_enhancements')) {
      this.premiumAIStatus = null;
      this.renderPremiumAIStatus();
      return null;
    }
    try {
      const status = await API.ai.getPremiumStatus();
      this.premiumAIStatus = status;
      this.renderPremiumAIStatus();
      return status;
    } catch (error) {
      this.premiumAIStatus = null;
      this.renderPremiumAIStatus();
      return null;
    }
  },

  applyPremiumAIUsageUpdate(payload) {
    if (!payload || typeof payload.enhancements_remaining !== 'number') return;
    if (!this.premiumAIStatus || typeof this.premiumAIStatus.limit !== 'number') {
      this.refreshPremiumAIStatus();
      return;
    }
    this.premiumAIStatus.remaining = payload.enhancements_remaining;
    this.premiumAIStatus.used = Math.max(0, this.premiumAIStatus.limit - payload.enhancements_remaining);
    if (payload.resets_at) {
      this.premiumAIStatus.resets_at = payload.resets_at;
    }
    this.renderPremiumAIStatus();
  },

  buildAIGuideAvoidList(excludeText = '') {
    const excludeKey = (excludeText || '').trim().toLowerCase();
    const avoid = [];
    const seen = new Set();
    const items = this.currentCard?.items || [];
    for (const item of items) {
      const content = (item.content || '').trim();
      if (!content) continue;
      const key = content.toLowerCase();
      if (excludeKey && key === excludeKey) continue;
      if (seen.has(key)) continue;
      seen.add(key);
      avoid.push(content.length > 100 ? content.slice(0, 100) : content);
      if (avoid.length >= 24) break;
    }
    return avoid;
  },

  renderAIGuideResults(resultsEl, goals, onSelect) {
    if (!resultsEl) return;
    if (!Array.isArray(goals) || goals.length === 0) {
      resultsEl.innerHTML = '<p class="text-muted">No suggestions yet.</p>';
      return;
    }
    resultsEl.innerHTML = goals.map((goal, index) => `
      <button type="button" class="btn btn-secondary btn-sm ai-guide-suggestion" data-ai-suggestion="${index}">
        ${this.escapeHtml(goal)}
      </button>
    `).join('');
    resultsEl.querySelectorAll('[data-ai-suggestion]').forEach((button, index) => {
      button.addEventListener('click', () => {
        if (typeof onSelect === 'function') {
          onSelect(goals[index]);
        }
      });
    });
  },

  async handleAIRefine(position) {
    if (this.isAnonymousMode || !this.user) {
      this.showAIAuthModal();
      return;
    }
    if (AIWizard.isVerificationRequiredForAI()) {
      AIWizard.showVerificationRequiredModal();
      return;
    }

    const textarea = document.getElementById(`edit-item-content-${position}`);
    if (!textarea) return;
    const currentGoal = textarea.value.trim();
    const mode = currentGoal ? 'refine' : 'new';
    const count = mode === 'new' ? 5 : 3;
    if (currentGoal && currentGoal.length > 500) {
      this.toast('Goal must be 500 characters or less', 'error');
      return;
    }

    const hint = document.getElementById('ai-refine-hint')?.value.trim() || '';
    const resultsEl = document.getElementById('ai-refine-results');
    const button = document.getElementById('ai-refine-generate');
    const originalLabel = button ? button.textContent.trim() : '';
    if (button) {
      button.disabled = true;
      button.textContent = 'Generating...';
    }
    if (resultsEl) {
      resultsEl.innerHTML = '<p class="text-muted">Generating suggestions...</p>';
    }

    try {
      const avoid = this.buildAIGuideAvoidList(currentGoal);
      const response = await API.ai.guide(mode, currentGoal, hint, count, avoid);
      if (this.user && !this.user.email_verified && typeof response?.free_remaining === 'number') {
        this.user.ai_free_generations_used = Math.max(0, 5 - response.free_remaining);
      }
      this.renderAIGuideResults(resultsEl, response?.goals || [], (goal) => {
        textarea.value = goal;
        textarea.focus();
      });
    } catch (error) {
      if (this.user && !this.user.email_verified && typeof error?.data?.free_remaining === 'number') {
        this.user.ai_free_generations_used = Math.max(0, 5 - error.data.free_remaining);
      }
      if (error?.status === 403 && this.user && !this.user.email_verified) {
        if (AIWizard.isVerificationRequiredForAI() || error?.data?.free_remaining === 0) {
          AIWizard.showVerificationRequiredModal();
          return;
        }
      }
      this.toast(error.message, 'error');
    } finally {
      const fallbackLabel = mode === 'new' ? '🧙 Suggest with AI' : '🧙 Refine with AI';
      if (button) {
        button.disabled = false;
        button.textContent = originalLabel || fallbackLabel;
      }
    }
  },

  async handleAIPremiumAssist(position) {
    if (this.isAnonymousMode || !this.user) {
      this.showAIAuthModal();
      return;
    }
    if (!this.hasFeature('ai_enhancements')) {
      this.navigate('/premium?upgrade=1', { skipWarning: true });
      return;
    }
    if (!this.currentCard?.id) {
      this.toast('No active card found', 'error');
      return;
    }

    const mode = (document.getElementById('ai-premium-mode')?.value || 'breakdown').trim();
    const notes = (document.getElementById('ai-premium-notes')?.value || '').trim();
    if (notes.length > 500) {
      this.toast('Notes must be 500 characters or less', 'error');
      return;
    }

    const resultsEl = document.getElementById('ai-premium-results');
    const button = document.getElementById('ai-premium-generate');
    const originalLabel = button ? button.textContent.trim() : '';
    if (button) {
      button.disabled = true;
      button.textContent = 'Thinking...';
    }
    if (resultsEl) {
      resultsEl.textContent = 'Generating guidance...';
    }

    try {
      const response = await API.ai.assistGoal(this.currentCard.id, position, mode, notes);
      if (resultsEl) {
        resultsEl.textContent = response?.reply || '';
      }
      this.applyPremiumAIUsageUpdate(response);
    } catch (error) {
      if (error?.status === 403 && /premium required/i.test(error?.message || '')) {
        this.navigate('/premium?upgrade=1', { skipWarning: true });
        return;
      }
      if (resultsEl && error?.data?.resets_at) {
        const reset = new Date(error.data.resets_at);
        resultsEl.textContent = `No AI Enhancements left this month. Resets ${reset.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}.`;
      }
      this.toast(error.message, 'error');
    } finally {
      if (button) {
        button.disabled = false;
        button.textContent = originalLabel || '✨ Ask Goal Assistant';
      }
    }
  },

  async fillEmptyWithAI() {
    if (this.isAnonymousMode || !this.user) {
      this.showAIAuthModal();
      return;
    }
    if (!this.hasFeature('ai_enhancements')) {
      this.navigate('/premium?upgrade=1', { skipWarning: true });
      return;
    }
    if (!this.currentCard?.id) {
      this.toast('No active card found', 'error');
      return;
    }
    if (this.currentCard.is_finalized) {
      this.toast('Card must be a draft', 'error');
      return;
    }

    const currentItemCount = this.currentCard.items ? this.currentCard.items.length : 0;
    const capacity = this.getCardCapacity(this.currentCard);
    if (currentItemCount >= capacity) {
      this.toast('Card is already full', 'info');
      return;
    }

    const button = document.getElementById('ai-fill-empty-btn');
    const originalLabel = button ? button.textContent.trim() : '';
    if (button) {
      button.disabled = true;
      button.textContent = 'Filling...';
    }

    try {
      const response = await API.ai.fillEmpty(this.currentCard.id, 'mix', '', 'medium', 'free', '');
      if (response?.card) {
        this.currentCard = response.card;
      } else {
        const refreshed = await API.cards.get(this.currentCard.id);
        this.currentCard = refreshed.card;
      }

      this.usedSuggestions = new Set(
        (this.currentCard.items || [])
          .map(item => (item.content || '').toLowerCase())
          .filter(Boolean)
      );

      const grid = document.getElementById('bingo-grid');
      if (grid) grid.innerHTML = this.renderGrid();

      const itemCount = this.currentCard.items ? this.currentCard.items.length : 0;
      const isFull = itemCount >= capacity;
      const progressEl = document.querySelector('progress.progress-bar');
      if (progressEl) {
        progressEl.max = capacity;
        progressEl.value = itemCount;
      }
      const progressText = document.querySelector('.progress-text');
      if (progressText) {
        progressText.textContent = `${itemCount}/${capacity} items added`;
      }

      const input = document.getElementById('item-input');
      if (input) input.disabled = isFull;
      const addBtn = document.getElementById('add-btn');
      if (addBtn) addBtn.disabled = isFull;
      const fillBtn = document.getElementById('fill-empty-btn');
      if (fillBtn) fillBtn.disabled = isFull;
      const aiBtn = document.getElementById('ai-btn');
      if (aiBtn) aiBtn.disabled = isFull;
      const clearBtn = document.getElementById('clear-btn');
      if (clearBtn) clearBtn.disabled = itemCount === 0;
      const shuffleBtn = document.getElementById('shuffle-btn');
      if (shuffleBtn) shuffleBtn.disabled = itemCount === 0;
      const finalizeBtn = document.getElementById('finalize-btn');
      if (finalizeBtn) finalizeBtn.disabled = itemCount < capacity;

      this.refreshSuggestionsList();
      this.applyPremiumAIUsageUpdate(response);
      this.toast('Filled empty squares with AI', 'success');
    } catch (error) {
      if (error?.status === 403 && /premium required/i.test(error?.message || '')) {
        this.navigate('/premium?upgrade=1', { skipWarning: true });
        return;
      }
      this.toast(error.message, 'error');
    } finally {
      if (button) {
        const itemCount = this.currentCard?.items ? this.currentCard.items.length : 0;
        button.disabled = itemCount >= capacity;
        button.textContent = originalLabel || '✨ AI Fill';
      }
    }
  },

  showAIAuthModal() {
    this.openModal('Use the AI Goal Wizard', `
      <div class="finalize-auth-modal">
        <p class="mb-lg">
          AI-generated goals are available after you create an account.
          This helps prevent abuse and keeps AI costs under control.
        </p>
        <div class="flex flex-col gap-md">
          <a class="btn btn-primary btn-lg" href="/register" data-action="close-modal">
            Create Account
          </a>
          <a class="btn btn-secondary btn-lg" href="/login" data-action="close-modal">
            I Already Have an Account
          </a>
          <button class="btn btn-ghost" data-action="close-modal">
            Cancel
          </button>
        </div>
      </div>
    `);
  },
});
