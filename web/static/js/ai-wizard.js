const AIWizard = {
  _busy: false,
  state: {
    step: 'input', // input, loading, review
    inputs: {},
    results: [],
    mode: 'create', // 'create' or 'append'
    targetCardId: null,
    desiredCount: null,
  },

  renderPremiumEnhancementsLine() {
    if (!App.user || !App.hasFeature('ai_enhancements')) return '';
    const line = App.formatPremiumAIStatusLine(App.premiumAIStatus);
    if (!line) return '';
    return `<div class="text-muted mb-md text-sm">${App.escapeHtml(line)}</div>`;
  },

  computeCreateGoalCountFromGridSize(gridSize) {
    const n = Number(gridSize);
    const size = Number.isFinite(n) && n >= 2 && n <= 5 ? n : 5;
    // Create flow defaults to FREE = ON.
    return size * size - 1;
  },

  computeCapacityFromConfig(config) {
    const n = Number(config?.grid_size);
    const size = Number.isFinite(n) && n >= 2 && n <= 5 ? n : 5;
    const hasFree = config?.has_free_space !== false;
    return size * size - (hasFree ? 1 : 0);
  },

  computeCardCapacity(card) {
    const n = Number(card?.grid_size);
    const size = Number.isFinite(n) && n >= 2 && n <= 5 ? n : 5;
    const hasFree = card?.has_free_space !== false;
    return size * size - (hasFree ? 1 : 0);
  },

  computeOpenCellsForCard(card) {
    const itemCount = card?.items ? card.items.length : 0;
    const capacity = this.computeCardCapacity(card);
    return Math.max(0, Math.min(capacity, capacity - itemCount));
  },

  computeOpenCellsFromDOM() {
    const grid = document.getElementById('bingo-grid');
    if (!grid) return null;
    const allCells = grid.querySelectorAll('.bingo-cell[data-position]:not(.bingo-cell--free)');
    if (!allCells.length) return null;
    const filledCells = grid.querySelectorAll('.bingo-cell[data-position][data-item-id]:not(.bingo-cell--free)');
    return Math.max(0, Math.min(allCells.length, allCells.length - filledCells.length));
  },

  isVerificationRequiredForAI() {
    if (!App.user) return false;
    if (App.user.email_verified) return false;
    const used = typeof App.user.ai_free_generations_used === 'number' ? App.user.ai_free_generations_used : 0;
    return used >= 5;
  },

	  showVerificationRequiredModal() {
	    const email = App.user?.email ? App.escapeHtml(App.user.email) : 'your email';
	    App.openModal('Verify Email Required', `
	      <div class="finalize-confirm-modal">
	        <p class="mb-md">
	          You've used your 5 free AI generations. Verify your email to keep using the AI Goal Wizard.
	        </p>
	        <p class="text-muted mb-lg">
	          A verification email was sent to <strong>${email}</strong>.
	        </p>
	        <div class="flex gap-md justify-end flex-wrap">
	          <button class="btn btn-ghost" data-action="close-modal">Close</button>
	          <button class="btn btn-primary" data-action="resend-verification-and-route">
	            Resend Verification Email
	          </button>
	        </div>
	      </div>
	    `);
	  },

  mapWizardCategoryToCardCategory(category) {
    const map = {
      hobbies: 'hobbies',
      health: 'health',
      career: 'professional',
      social: 'social',
      travel: 'travel',
      mix: null,
    };
    return Object.prototype.hasOwnProperty.call(map, category) ? map[category] : null;
  },

  open(targetCardId = null, desiredCount = null) {
    if (this.isVerificationRequiredForAI()) {
      this.showVerificationRequiredModal();
      return;
    }

    const desiredCountNumber = Number(desiredCount);
    const desiredCountValue = Number.isFinite(desiredCountNumber) ? desiredCountNumber : null;

    this.state = {
      step: 'input',
      inputs: targetCardId
        ? {}
        : { grid_size: 5 },
      results: [],
      mode: targetCardId ? 'append' : 'create',
      targetCardId: targetCardId,
      desiredCount: desiredCountValue,
    };
    this.render();
  },

  render() {
    if (this.state.step === 'input') {
      const title = this.state.mode === 'append' ? 'AI Goal Generator 🧙' : 'AI Goal Wizard 🧙';
      App.openModal(title, this.renderInputStep());
      this.setupInputEvents();
    } else if (this.state.step === 'loading') {
      App.openModal('Generating Magic ✨', this.renderLoadingStep());
    } else if (this.state.step === 'review') {
      App.openModal('Review Your Goals 🔮', this.renderReviewStep());
      this.setupReviewEvents();
    }
  },

  renderInputStep() {
    const freeLimit = 5;
    const showQuota = App.user && !App.user.email_verified;
    const used = showQuota && typeof App.user.ai_free_generations_used === 'number' ? App.user.ai_free_generations_used : 0;
    const remaining = showQuota ? Math.max(0, freeLimit - used) : null;
    const desiredCount = this.getDesiredCountSync();
    const goalWord = desiredCount === 1 ? 'goal' : 'goals';
    const createGridSize = Number(this.state.inputs.grid_size) || 5;
    const createCapacity = this.computeCreateGoalCountFromGridSize(createGridSize);
	    return `
	      <div class="text-muted mb-md">
	        ${this.state.mode === 'append'
	          ? `Describe what you want, and we'll generate <strong>${desiredCount}</strong> ${goalWord} to fill your empty squares.`
	          : `Describe what you want, and we'll generate <strong><span id="ai-create-capacity">${createCapacity}</span></strong> custom Bingo goals for your <strong><span id="ai-create-grid">${createGridSize}x${createGridSize}</span></strong> card.`}
	      </div>
	      ${showQuota ? `
	        <div class="text-muted mb-md text-sm">
	          Free AI generations left before verification is required: <strong>${remaining}</strong>
	        </div>
	      ` : ''}
	      ${this.renderPremiumEnhancementsLine()}
	      <form id="ai-wizard-form" data-action="ai-generate">
        <div class="form-group">
            <label class="form-label">What area of life is this for?</label>
            <select id="ai-category" class="form-input" required>
                <option value="hobbies">Hobbies & Skills</option>
                <option value="health">Health & Wellness</option>
                <option value="career">Career & Growth</option>
                <option value="social">Social & Fun</option>
                <option value="travel">Travel & Adventure</option>
                <option value="mix">Surprise Me (Mix)</option>
            </select>
        </div>
        
        <div class="form-group">
            <label class="form-label">Specific Interest (Optional)</label>
            <input type="text" id="ai-focus" class="form-input" placeholder="e.g. Italian Cooking, Hiking, Python Programming">
            <small class="text-muted">Narrow down the goals to a specific topic.</small>
        </div>

        ${this.state.mode === 'create' ? `
          <div class="form-group">
            <label class="form-label">Grid Size</label>
            <select id="ai-grid-size" class="form-input">
              <option value="2" ${createGridSize === 2 ? 'selected' : ''}>2x2</option>
              <option value="3" ${createGridSize === 3 ? 'selected' : ''}>3x3</option>
              <option value="4" ${createGridSize === 4 ? 'selected' : ''}>4x4</option>
              <option value="5" ${createGridSize === 5 ? 'selected' : ''}>5x5</option>
            </select>
            <small class="text-muted">FREE space is included by default.</small>
          </div>
	        ` : ''}

	        <div class="form-group">
	            <label class="form-label">Difficulty Level</label>
	            <div class="flex gap-md mt-sm">
	                <label class="flex items-center gap-sm cursor-pointer">
	                    <input type="radio" name="difficulty" value="easy"> 
	                    <span>Chill 😌</span>
	                </label>
	                <label class="flex items-center gap-sm cursor-pointer">
	                    <input type="radio" name="difficulty" value="medium" checked> 
	                    <span>Balanced ⚖️</span>
	                </label>
	                <label class="flex items-center gap-sm cursor-pointer">
	                    <input type="radio" name="difficulty" value="hard"> 
	                    <span>Hardcore 🔥</span>
	                </label>
	            </div>
	        </div>

	        <div class="form-group">
		            <label class="form-label">Budget Level</label>
		            <div class="flex gap-md mt-sm">
		                <label class="flex items-center gap-sm cursor-pointer">
		                    <input type="radio" name="budget" value="free" checked> 
		                    <span>$</span>
		                </label>
		                <label class="flex items-center gap-sm cursor-pointer">
		                    <input type="radio" name="budget" value="low"> 
		                    <span>$$</span>
		                </label>
		                <label class="flex items-center gap-sm cursor-pointer">
		                    <input type="radio" name="budget" value="medium"> 
		                    <span>$$$</span>
		                </label>
		                <label class="flex items-center gap-sm cursor-pointer">
		                    <input type="radio" name="budget" value="high"> 
		                    <span>$$$$</span>
		                </label>
		            </div>
		        </div>

	        <div class="form-group">
	            <label class="form-label">Any other context?</label>
	            <textarea id="ai-context" class="form-input" rows="2" placeholder="e.g. I live in a city, I don't drive..."></textarea>
	        </div>

	        <div class="flex gap-md mt-xl">
	            <button type="button" class="btn btn-ghost" data-action="close-modal">Cancel</button>
	            <button type="submit" class="btn btn-primary flex-1">Generate Goals ✨</button>
	        </div>
	      </form>
	    `;
	  },

  setupInputEvents() {
    if (this.state.mode !== 'create') return;

    const gridEl = document.getElementById('ai-grid-size');
    if (!gridEl) return;

    const apply = () => {
      const n = parseInt(gridEl.value, 10) || 5;
      this.state.inputs.grid_size = n;

      const capacityEl = document.getElementById('ai-create-capacity');
      if (capacityEl) {
        capacityEl.textContent = String(this.computeCreateGoalCountFromGridSize(n));
      }

      const gridLabelEl = document.getElementById('ai-create-grid');
      if (gridLabelEl) {
        gridLabelEl.textContent = `${n}x${n}`;
      }
    };

    gridEl.addEventListener('change', apply);
    apply();
  },

	  renderLoadingStep() {
	    return `
	      <div class="text-center p-xl">
	        <div class="spinner mb-lg"></div>
	        <h3>Consulting the Oracle...</h3>
	        <p class="text-muted">This usually takes about 5-10 seconds.</p>
	        <p class="text-muted text-xs mt-md">Powered by AI</p>
	      </div>
	    `;
	  },

  async handleGenerate(event) {
    event.preventDefault();
    if (this._busy) return;
    if (!App.user) {
      App.toast('Please log in to use AI features.', 'error');
      return;
    }

    if (this.isVerificationRequiredForAI()) {
      this.showVerificationRequiredModal();
      return;
    }

    this._busy = true;
    
    const category = document.getElementById('ai-category').value;
    const focus = document.getElementById('ai-focus').value;
    const difficultyRadio = document.querySelector('input[name="difficulty"]:checked');
    const budgetRadio = document.querySelector('input[name="budget"]:checked');
    if (!difficultyRadio) {
      App.toast('Please select a difficulty level.', 'error');
      this._busy = false;
      return;
    }
    if (!budgetRadio) {
      App.toast('Please select a budget.', 'error');
      this._busy = false;
      return;
    }
    const difficulty = difficultyRadio.value;
    const budget = budgetRadio.value;
    const context = document.getElementById('ai-context').value;

    const nextInputs = { category, focus, difficulty, budget, context };
    if (this.state.mode === 'create') {
      const gridSizeEl = document.getElementById('ai-grid-size');
      nextInputs.grid_size = parseInt(gridSizeEl?.value || '5', 10) || 5;
    }
    this.state.inputs = nextInputs;
    this.state.step = 'loading';
    this.render();

    try {
      const count = this.state.mode === 'create'
        ? this.computeCreateGoalCountFromGridSize(this.state.inputs.grid_size)
        : await this.resolveDesiredCount();
      const response = await API.ai.generate(category, focus, difficulty, budget, context, count);
      if (App.user && !App.user.email_verified && typeof response?.free_remaining === 'number') {
        App.user.ai_free_generations_used = Math.max(0, 5 - response.free_remaining);
      }
      this.state.results = response.goals;
      this.state.step = 'review';
      this.render(); // Re-render to show review step
    } catch (error) {
      if (App.user && !App.user.email_verified && typeof error?.data?.free_remaining === 'number') {
        App.user.ai_free_generations_used = Math.max(0, 5 - error.data.free_remaining);
      }
      if (error?.status === 403 && App.user && !App.user.email_verified) {
        if (this.isVerificationRequiredForAI() || error?.data?.free_remaining === 0) {
          this.showVerificationRequiredModal();
          return;
        }
      }
      App.toast(error.message, 'error');
      this.state.step = 'input';
      this.render();
    } finally {
      this._busy = false;
    }
  },

	  renderReviewStep() {
	    const goalsList = this.state.results.map((goal, index) => `
	      <div class="ai-goal-item">
	        <span class="text-muted ai-goal-index">${index + 1}.</span>
	        <input type="text" class="form-input form-input--sm ai-goal-input" value="${App.escapeHtml(goal)}" data-index="${index}">
	        ${App.hasFeature('ai_enhancements') ? `<button type="button" class="btn btn-secondary btn-sm" data-action="ai-regenerate-goal" data-index="${index}">Regenerate</button>` : ''}
	      </div>
	    `).join('');

    const actionButtonText = this.state.mode === 'append' ? 'Add to Card →' : 'Create Card →';
    const actionType = this.state.mode === 'append' ? 'ai-add-to-card' : 'ai-create-card';
    const startOverCardId = App.escapeHtml(this.state.targetCardId || '');

	    return `
	      <p class="text-muted">Review and edit your generated goals.</p>
	      ${this.renderPremiumEnhancementsLine()}
	      
	      <div class="ai-results-list">
	        ${goalsList}
	      </div>

	      <div class="flex gap-md mt-lg">
	        <button type="button" class="btn btn-secondary" data-action="open-ai-wizard" data-card-id="${startOverCardId}">Start Over</button>
	        <button type="button" class="btn btn-primary flex-1" data-action="${actionType}">${actionButtonText}</button>
	      </div>
	    `;
	  },

  async regenerateGoal(index, triggerEl = null) {
    if (this._busy) return;
    if (!App.user) {
      App.toast('Please log in to use AI features.', 'error');
      return;
    }
    if (!App.hasFeature('ai_enhancements')) {
      App.navigate('/premium?upgrade=1', { skipWarning: true });
      return;
    }
    if (index < 0 || index >= this.state.results.length) {
      return;
    }

    const button = triggerEl || document.querySelector(`[data-action="ai-regenerate-goal"][data-index="${index}"]`);
    const originalLabel = button ? button.textContent.trim() : '';
    if (button) {
      button.disabled = true;
      button.textContent = 'Regenerating...';
    }

    this._busy = true;
    try {
      const response = await API.ai.regenerate({
        category: this.state.inputs.category,
        focus: this.state.inputs.focus || '',
        difficulty: this.state.inputs.difficulty,
        budget: this.state.inputs.budget,
        context: this.state.inputs.context || '',
        existing_goals: this.state.results,
        replace_index: index,
      });
      if (response?.goal) {
        this.state.results[index] = response.goal;
        const input = document.querySelector(`.ai-goal-input[data-index="${index}"]`);
        if (input) input.value = response.goal;
      }
      App.applyPremiumAIUsageUpdate(response);
    } catch (error) {
      if (error?.status === 403 && /premium required/i.test(error?.message || '')) {
        App.navigate('/premium?upgrade=1', { skipWarning: true });
        return;
      }
      App.toast(error.message, 'error');
    } finally {
      this._busy = false;
      if (button) {
        button.disabled = false;
        button.textContent = originalLabel || 'Regenerate';
      }
    }
  },

  getDesiredCountSync() {
    if (this.state.mode !== 'append') return 24;
    const fromDom = this.computeOpenCellsFromDOM();
    if (typeof fromDom === 'number') return Math.max(0, Math.min(25, fromDom));
    if (typeof this.state.desiredCount === 'number' && Number.isFinite(this.state.desiredCount)) {
      const n = Math.max(0, Math.min(25, Math.floor(this.state.desiredCount)));
      return n;
    }
    if (App.currentCard && this.state.targetCardId && App.currentCard.id === this.state.targetCardId) {
      return this.computeOpenCellsForCard(App.currentCard);
    }
    return 24;
  },

  async resolveDesiredCount() {
    let count = this.getDesiredCountSync();
    if (this.state.mode === 'append' && this.state.targetCardId && !count) {
      try {
        const res = await API.cards.get(this.state.targetCardId);
        count = this.computeOpenCellsForCard(res.card);
      } catch (e) {
        // Ignore and fall back
      }
    }

    count = Math.max(1, Math.min(25, Math.floor(count)));
    this.state.desiredCount = count;
    return count;
  },

  setupReviewEvents() {
    const inputs = document.querySelectorAll('.ai-goal-input');
    inputs.forEach(input => {
      input.addEventListener('change', (e) => {
        const index = parseInt(e.target.dataset.index);
        this.state.results[index] = e.target.value;
      });
    });
  },

  async createCard() {
    if (this._busy) return;
    const year = new Date().getFullYear();
    const focus = (this.state.inputs.focus || '').trim().replace(/\s+/g, ' ').slice(0, 50);
    const title = focus ? `${focus} Bingo` : `${year} AI Bingo`;
    const category = this.mapWizardCategoryToCardCategory(this.state.inputs.category);
    const gridSize = parseInt(this.state.inputs.grid_size || '5', 10) || 5;

    try {
      if (!App.user) {
         throw new Error("Please log in to use AI features.");
      }

      this._busy = true;
      App.showLoading(document.querySelector('.modal-body'), 'Creating card...');

      const response = await API.cards.create(year, title, category, {
        gridSize,
      });
      const cardId = response.card.id;

      await this.fillCard(cardId);

      App.closeModal();
      App.navigate(`/card/${cardId}`);
      App.toast('AI Card Created! 🧙‍♂️', 'success');

    } catch (error) {
      App.toast(error.message, 'error');
      this.state.step = 'review';
      this.render();
    } finally {
      this._busy = false;
    }
  },

  async addToCard() {
    if (!this.state.targetCardId) return;
    if (this._busy) return;

    try {
      if (!App.user) {
         throw new Error("Please log in to use AI features.");
      }

      this._busy = true;
      App.showLoading(document.querySelector('.modal-body'), 'Adding goals...');

      await this.fillCard(this.state.targetCardId);

      App.closeModal();
      
      // Refresh the card view
      if (App.currentCard && App.currentCard.id === this.state.targetCardId) {
          App.renderCard(document.getElementById('main-container'), this.state.targetCardId);
      }
      
      App.toast('Goals added! 🧙‍♂️', 'success');

    } catch (error) {
      App.toast(error.message, 'error');
      this.state.step = 'review';
      this.render();
    } finally {
      this._busy = false;
    }
  },

  async fillCard(cardId) {
      try {
          // If we are appending, we need to know what spots are taken.
          // Since we might be in the 'create' flow, we know it's empty.
          // If 'append', we should fetch the card or use App.currentCard if it matches.
          if (this.state.mode === 'append' && App.currentCard && App.currentCard.id === cardId) {
              // no-op
          } else if (this.state.mode === 'append') {
               await API.cards.get(cardId);
          }
      } catch (e) {
          // Ignore, assume empty
      }

      const desiredCount = this.state.mode === 'append'
        ? await this.resolveDesiredCount()
        : (this.state.results?.length || 0);
      const goalsToAdd = (this.state.results || []).slice(0, desiredCount);

      const addedPositions = [];
      for (let i = 0; i < goalsToAdd.length; i++) {
        const goal = goalsToAdd[i];
        try {
          const res = await API.cards.addItem(cardId, goal);
          addedPositions.push(res.item.position);
        } catch (error) {
          console.error('Failed to add goal', { index: i, goal, reason: error });
          const rollbackResults = await Promise.allSettled(
            addedPositions.map(pos => API.cards.removeItem(cardId, pos))
          );
          const rollbackFailures = rollbackResults
            .map((r, idx) => ({ status: r.status, reason: r.reason, pos: addedPositions[idx] }))
            .filter(r => r.status === 'rejected');
          if (rollbackFailures.length > 0) {
            console.error('Rollback failed for some items:', rollbackFailures);
            throw new Error('Failed to add some goals. Rollback was attempted but failed for some items. Please refresh the card and verify its contents.');
          }
          throw error;
        }
      }
  }
};

// Explicitly assign to window for global access
window.AIWizard = AIWizard;
