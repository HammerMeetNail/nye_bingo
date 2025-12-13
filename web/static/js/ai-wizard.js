const AIWizard = {
  state: {
    step: 'input', // input, loading, review
    inputs: {},
    results: [],
    mode: 'create', // 'create' or 'append'
    targetCardId: null,
    desiredCount: null,
  },

  computeOpenCellsForCard(card) {
    const itemCount = card?.items ? card.items.length : 0;
    return Math.max(0, 24 - itemCount);
  },

  computeOpenCellsFromDOM() {
    const grid = document.getElementById('bingo-grid');
    if (!grid) return null;
    const allCells = grid.querySelectorAll('.bingo-cell[data-position]');
    if (!allCells.length) return null;
    const filledCells = grid.querySelectorAll('.bingo-cell[data-position][data-item-id]');
    return Math.max(0, allCells.length - filledCells.length);
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
    const desiredCountNumber = Number(desiredCount);
    const desiredCountValue = Number.isFinite(desiredCountNumber) ? desiredCountNumber : null;

    this.state = { 
      step: 'input', 
      inputs: {}, 
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
    return `
      <div class="text-muted mb-md">
        ${this.state.mode === 'append'
          ? `Describe what you want, and we'll generate <strong>${desiredCount}</strong> goals to fill your empty squares.`
          : `Describe what you want, and we'll generate 24 custom Bingo goals for you.`}
      </div>
      ${showQuota ? `
        <div class="text-muted mb-md" style="font-size: 0.9rem;">
          Free AI generations left before verification is required: <strong>${remaining}</strong>
        </div>
      ` : ''}
      <form id="ai-wizard-form" onsubmit="AIWizard.handleGenerate(event)">
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

        <div class="form-group">
            <label class="form-label">Difficulty Level</label>
            <div style="display: flex; gap: 1rem; margin-top: 0.5rem;">
                <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
                    <input type="radio" name="difficulty" value="easy"> 
                    <span>Chill 😌</span>
                </label>
                <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
                    <input type="radio" name="difficulty" value="medium" checked> 
                    <span>Balanced ⚖️</span>
                </label>
                <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
                    <input type="radio" name="difficulty" value="hard"> 
                    <span>Hardcore 🔥</span>
                </label>
            </div>
        </div>

        <div class="form-group">
            <label class="form-label">Budget Level</label>
            <div style="display: flex; gap: 1rem; margin-top: 0.5rem;">
                <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
                    <input type="radio" name="budget" value="free" checked> 
                    <span>$ (Free/Cheap)</span>
                </label>
                <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
                    <input type="radio" name="budget" value="low"> 
                    <span>$$ (Moderate)</span>
                </label>
                <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
                    <input type="radio" name="budget" value="medium"> 
                    <span>$$$ (Pricey)</span>
                </label>
                <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
                    <input type="radio" name="budget" value="high"> 
                    <span>$$$$ (Splurge)</span>
                </label>
            </div>
        </div>

        <div class="form-group">
            <label class="form-label">Any other context?</label>
            <textarea id="ai-context" class="form-input" rows="2" placeholder="e.g. I live in a city, I don't drive..."></textarea>
        </div>

        <div style="display: flex; gap: 1rem; margin-top: 2rem;">
            <button type="button" class="btn btn-ghost" onclick="App.closeModal()">Cancel</button>
            <button type="submit" class="btn btn-primary" style="flex: 1;">Generate Goals ✨</button>
        </div>
      </form>
    `;
  },

  setupInputEvents() {
    // No special events needed for now
  },

  renderLoadingStep() {
    return `
      <div class="text-center" style="padding: 2rem;">
        <div class="spinner" style="margin-bottom: 1.5rem;"></div>
        <h3>Consulting the Oracle...</h3>
        <p class="text-muted">This usually takes about 5-10 seconds.</p>
        <p class="text-muted" style="font-size: 0.8rem; margin-top: 1rem;">Powered by AI</p>
      </div>
    `;
  },

  async handleGenerate(event) {
    event.preventDefault();
    if (!App.user) {
      App.toast('Please log in to use AI features.', 'error');
      return;
    }
    
    const category = document.getElementById('ai-category').value;
    const focus = document.getElementById('ai-focus').value;
    const difficultyRadio = document.querySelector('input[name="difficulty"]:checked');
    const budgetRadio = document.querySelector('input[name="budget"]:checked');
    if (!difficultyRadio) {
      App.toast('Please select a difficulty level.', 'error');
      return;
    }
    if (!budgetRadio) {
      App.toast('Please select a budget.', 'error');
      return;
    }
    const difficulty = difficultyRadio.value;
    const budget = budgetRadio.value;
    const context = document.getElementById('ai-context').value;

    this.state.inputs = { category, focus, difficulty, budget, context };
    this.state.step = 'loading';
    this.render();

    try {
      // Passing budget as the 4th argument
      const count = await this.resolveDesiredCount();
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
      App.toast(error.message, 'error');
      this.state.step = 'input';
      this.render();
    }
  },

  renderReviewStep() {
    const goalsList = this.state.results.map((goal, index) => `
      <div class="ai-goal-item" style="display: flex; gap: 0.5rem; margin-bottom: 0.5rem;">
        <span class="text-muted" style="width: 20px;">${index + 1}.</span>
        <input type="text" class="form-input form-input--sm ai-goal-input" value="${App.escapeHtml(goal)}" data-index="${index}">
      </div>
    `).join('');

    const actionButtonText = this.state.mode === 'append' ? 'Add to Card →' : 'Create Card →';
    const actionFunction = this.state.mode === 'append' ? 'AIWizard.addToCard()' : 'AIWizard.createCard()';

    return `
      <p class="text-muted">Review and edit your generated goals.</p>
      
      <div class="ai-results-list" style="max-height: 400px; overflow-y: auto; margin: 1rem 0; padding-right: 0.5rem;">
        ${goalsList}
      </div>

      <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
        <button type="button" class="btn btn-secondary" onclick="AIWizard.open('${App.escapeHtml(this.state.targetCardId || '')}')">Start Over</button>
        <button type="button" class="btn btn-primary" style="flex: 1;" onclick="${actionFunction}">${actionButtonText}</button>
      </div>
    `;
  },

  getDesiredCountSync() {
    if (this.state.mode !== 'append') return 24;
    const fromDom = this.computeOpenCellsFromDOM();
    if (typeof fromDom === 'number') return Math.max(0, Math.min(24, fromDom));
    if (typeof this.state.desiredCount === 'number' && Number.isFinite(this.state.desiredCount)) {
      const n = Math.max(0, Math.min(24, Math.floor(this.state.desiredCount)));
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
        const itemCount = res.card?.items ? res.card.items.length : 0;
        count = Math.max(0, 24 - itemCount);
      } catch (e) {
        // Ignore and fall back
      }
    }

    count = Math.max(1, Math.min(24, Math.floor(count)));
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
    const year = new Date().getFullYear();
    const focus = (this.state.inputs.focus || '').trim().replace(/\s+/g, ' ').slice(0, 50);
    const title = focus ? `${focus} Bingo` : `${year} AI Bingo`;
    const category = this.mapWizardCategoryToCardCategory(this.state.inputs.category);

    try {
      if (!App.user) {
         throw new Error("Please log in to use AI features.");
      }

      App.showLoading(document.querySelector('.modal-body'), 'Creating card...');

      const response = await API.cards.create(year, title, category);
      const cardId = response.card.id;

      await this.fillCard(cardId);

      App.closeModal();
      window.location.hash = `#card/${cardId}`;
      App.toast('AI Card Created! 🧙‍♂️', 'success');

    } catch (error) {
      App.toast(error.message, 'error');
      this.state.step = 'review';
      this.render();
    }
  },

  async addToCard() {
    if (!this.state.targetCardId) return;

    try {
      if (!App.user) {
         throw new Error("Please log in to use AI features.");
      }

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
    }
  },

  async fillCard(cardId) {
      // Get current card items to determine empty spots
      let existingItems = [];
      try {
          // If we are appending, we need to know what spots are taken.
          // Since we might be in the 'create' flow, we know it's empty.
          // If 'append', we should fetch the card or use App.currentCard if it matches.
          if (this.state.mode === 'append' && App.currentCard && App.currentCard.id === cardId) {
              existingItems = App.currentCard.items || [];
          } else if (this.state.mode === 'append') {
               const res = await API.cards.get(cardId);
               existingItems = res.card.items || [];
          }
      } catch (e) {
          // Ignore, assume empty
      }

      const takenPositions = new Set(existingItems.map(i => i.position));
      const availablePositions = [];
      for (let i = 0; i < 25; i++) {
          if (i === 12) continue; // Free space
          if (!takenPositions.has(i)) {
              availablePositions.push(i);
          }
      }

      const goalsToAdd = this.state.results.slice(0, availablePositions.length);
      
      const results = await Promise.allSettled(
        goalsToAdd.map((goal, index) => {
          const pos = availablePositions[index];
          return API.cards.addItem(cardId, goal, pos).then(() => ({ pos, goal }));
        })
      );

      const failures = results
        .map((r, i) => ({ index: i, status: r.status, reason: r.reason, goal: goalsToAdd[i] }))
        .filter(r => r.status === 'rejected');
      if (failures.length === 0) {
	        return;
      }

      console.error('Failed to add the following goals:', failures.map(f => ({
        index: f.index,
        goal: f.goal,
        reason: f.reason
      })));

      const successes = results
        .map((r, i) => ({ r, i }))
        .filter(({ r }) => r.status === 'fulfilled')
        .map(({ r, i }) => ({ ...r.value, index: i, goal: goalsToAdd[i] }));

      const rollbackResults = await Promise.allSettled(
        successes.map(({ pos }) => API.cards.removeItem(cardId, pos))
      );

      const rollbackFailures = rollbackResults
        .map((r, i) => ({ status: r.status, reason: r.reason, pos: successes[i].pos, goal: successes[i].goal }))
        .filter(r => r.status === 'rejected');
      if (rollbackFailures.length > 0) {
        console.error('Rollback failed for some items:', rollbackFailures);
        throw new Error('Failed to add some goals. Rollback was attempted but failed for some items. Please refresh the card and verify its contents.');
      }

      const maxToShow = 3;
      const failedPreview = failures
        .slice(0, maxToShow)
        .map(f => `"${f.goal}"`)
        .join(', ');
      const suffix = failures.length > maxToShow ? ` (and ${failures.length - maxToShow} more)` : '';
      throw new Error(`Failed to add some goals: ${failedPreview}${suffix}. Please try again.`);
  }
};

// Explicitly assign to window for global access
window.AIWizard = AIWizard;
