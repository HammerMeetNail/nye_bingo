const AIWizard = {
  state: {
    step: 'input', // input, loading, review
    inputs: {},
    results: [],
    mode: 'create', // 'create' or 'append'
    targetCardId: null,
  },

  open(targetCardId = null) {
    console.log('AIWizard: open()', { step: this.state.step, mode: this.state.mode, targetCardId: targetCardId });
    this.state = { 
      step: 'input', 
      inputs: {}, 
      results: [],
      mode: targetCardId ? 'append' : 'create',
      targetCardId: targetCardId
    };
    this.render();
  },

  render() {
    console.log('AIWizard: render()', { step: this.state.step });
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
    return `
      <div class="text-muted mb-md">
        Describe what you want, and we'll generate 24 custom Bingo goals for you.
      </div>
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
        <p class="text-muted" style="font-size: 0.8rem; margin-top: 1rem;">Powered by Gemini Flash ⚡</p>
      </div>
    `;
  },

  async handleGenerate(event) {
    event.preventDefault();
    console.log('AIWizard: handleGenerate() - form submitted');
    
    const category = document.getElementById('ai-category').value;
    const focus = document.getElementById('ai-focus').value;
    const difficulty = document.querySelector('input[name="difficulty"]:checked').value;
    const budget = document.querySelector('input[name="budget"]:checked').value;
    const context = document.getElementById('ai-context').value;

    this.state.inputs = { category, focus, difficulty, budget, context };
    this.state.step = 'loading';
    this.render();

    try {
      console.log('AIWizard: handleGenerate() - calling API.ai.generate');
      // Passing budget as the 4th argument
      const response = await API.ai.generate(category, focus, difficulty, budget, context);
      console.log('AIWizard: handleGenerate() - API call successful', response.goals);
      this.state.results = response.goals;
      this.state.step = 'review';
      this.render(); // Re-render to show review step
    } catch (error) {
      console.error('AIWizard: handleGenerate() - API call failed', error);
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
    console.log('AIWizard: createCard()');
    const year = new Date().getFullYear();
    const title = this.state.inputs.focus ? `${this.state.inputs.focus} Bingo` : `${year} AI Bingo`;
    const category = this.state.inputs.category === 'mix' ? null : this.state.inputs.category;

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
      console.error('AIWizard: createCard() - failed', error);
      App.toast(error.message, 'error');
      this.state.step = 'review';
      this.render();
    }
  },

  async addToCard() {
    console.log('AIWizard: addToCard()');
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
      console.error('AIWizard: addToCard() - failed', error);
      App.toast(error.message, 'error');
      this.state.step = 'review';
      this.render();
    }
  },

  async fillCard(cardId) {
      console.log('AIWizard: fillCard()', cardId);
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
      
      const promises = goalsToAdd.map((goal, index) => {
        const pos = availablePositions[index];
        return API.cards.addItem(cardId, goal, pos);
      });

      await Promise.all(promises);
      console.log('AIWizard: fillCard() - all items added');
  }
};

// Explicitly assign to window for global access
window.AIWizard = AIWizard;