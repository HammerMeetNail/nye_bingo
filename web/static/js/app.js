// NYE Bingo - Main Application

const App = {
  user: null,
  currentCard: null,
  suggestions: [],
  usedSuggestions: new Set(),

  async init() {
    await API.init();
    await this.checkAuth();
    this.setupNavigation();
    this.setupModal();
    this.route();
  },

  async checkAuth() {
    try {
      const response = await API.auth.me();
      this.user = response.user;
    } catch (error) {
      this.user = null;
    }
  },

  setupNavigation() {
    const nav = document.getElementById('nav');
    if (!nav) return;

    if (this.user) {
      nav.innerHTML = `
        <a href="#dashboard" class="nav-link">My Cards</a>
        <span class="nav-link text-muted">Hi, ${this.escapeHtml(this.user.display_name)}</span>
        <button class="btn btn-ghost" onclick="App.logout()">Logout</button>
      `;
    } else {
      nav.innerHTML = `
        <a href="#login" class="btn btn-ghost">Login</a>
        <a href="#register" class="btn btn-primary">Get Started</a>
      `;
    }
  },

  setupModal() {
    const overlay = document.getElementById('modal-overlay');
    const closeBtn = document.getElementById('modal-close');

    if (overlay) {
      overlay.addEventListener('click', (e) => {
        if (e.target === overlay) this.closeModal();
      });
    }

    if (closeBtn) {
      closeBtn.addEventListener('click', () => this.closeModal());
    }

    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') this.closeModal();
    });
  },

  openModal(title, content) {
    const overlay = document.getElementById('modal-overlay');
    const titleEl = document.getElementById('modal-title');
    const bodyEl = document.getElementById('modal-body');

    if (titleEl) titleEl.textContent = title;
    if (bodyEl) bodyEl.innerHTML = content;
    if (overlay) overlay.classList.add('modal-overlay--visible');
  },

  closeModal() {
    const overlay = document.getElementById('modal-overlay');
    if (overlay) overlay.classList.remove('modal-overlay--visible');
  },

  route() {
    const hash = window.location.hash.slice(1) || 'home';
    const [page, ...params] = hash.split('/');

    const container = document.querySelector('.container');
    if (!container) return;

    switch (page) {
      case 'home':
        this.renderHome(container);
        break;
      case 'login':
        this.renderLogin(container);
        break;
      case 'register':
        this.renderRegister(container);
        break;
      case 'dashboard':
        this.requireAuth(() => this.renderDashboard(container));
        break;
      case 'create':
        this.requireAuth(() => this.renderCreate(container));
        break;
      case 'card':
        this.requireAuth(() => this.renderCard(container, params[0]));
        break;
      default:
        this.renderHome(container);
    }
  },

  requireAuth(callback) {
    if (!this.user) {
      window.location.hash = '#login';
      return;
    }
    callback();
  },

  // Page Renderers
  renderHome(container) {
    container.innerHTML = `
      <div class="text-center" style="padding: 4rem 0;">
        <h1 style="margin-bottom: 1rem;">
          <span class="text-gold">NYE</span> Bingo
        </h1>
        <p style="font-size: 1.25rem; max-width: 600px; margin: 0 auto 2rem;">
          Turn your New Year's resolutions into an exciting game! Create a bingo card
          with 24 goals and track your progress throughout the year.
        </p>
        ${this.user ? `
          <a href="#dashboard" class="btn btn-primary btn-lg">Go to Dashboard</a>
        ` : `
          <a href="#register" class="btn btn-primary btn-lg">Create Your Card</a>
          <p class="mt-md text-muted">
            Already have an account? <a href="#login">Login</a>
          </p>
        `}
      </div>
      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 2rem; margin-top: 4rem;">
        <div class="card text-center">
          <div style="font-size: 3rem; margin-bottom: 1rem;">🎯</div>
          <h3>24 Goals</h3>
          <p>Fill your bingo card with 24 meaningful goals for the year ahead.</p>
        </div>
        <div class="card text-center">
          <div style="font-size: 3rem; margin-bottom: 1rem;">✨</div>
          <h3>Track Progress</h3>
          <p>Mark items complete throughout the year with a satisfying stamp.</p>
        </div>
        <div class="card text-center">
          <div style="font-size: 3rem; margin-bottom: 1rem;">🎉</div>
          <h3>Celebrate Wins</h3>
          <p>Get bingos, share with friends, and celebrate your achievements.</p>
        </div>
      </div>
    `;
  },

  renderLogin(container) {
    if (this.user) {
      window.location.hash = '#dashboard';
      return;
    }

    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card">
          <div class="auth-header">
            <h2 class="auth-title">Welcome Back</h2>
            <p class="text-muted">Sign in to your account</p>
          </div>
          <form id="login-form">
            <div class="form-group">
              <label class="form-label" for="email">Email</label>
              <input type="email" id="email" class="form-input" required autocomplete="email">
            </div>
            <div class="form-group">
              <label class="form-label" for="password">Password</label>
              <input type="password" id="password" class="form-input" required autocomplete="current-password">
            </div>
            <div id="login-error" class="form-error hidden"></div>
            <button type="submit" class="btn btn-primary btn-lg" style="width: 100%;">
              Sign In
            </button>
          </form>
          <div class="auth-footer">
            Don't have an account? <a href="#register">Sign up</a>
          </div>
        </div>
      </div>
    `;

    document.getElementById('login-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const email = document.getElementById('email').value;
      const password = document.getElementById('password').value;
      const errorEl = document.getElementById('login-error');

      try {
        const response = await API.auth.login(email, password);
        this.user = response.user;
        this.setupNavigation();
        window.location.hash = '#dashboard';
        this.toast('Welcome back!', 'success');
      } catch (error) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      }
    });
  },

  renderRegister(container) {
    if (this.user) {
      window.location.hash = '#dashboard';
      return;
    }

    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card">
          <div class="auth-header">
            <h2 class="auth-title">Create Account</h2>
            <p class="text-muted">Start your resolution journey</p>
          </div>
          <form id="register-form">
            <div class="form-group">
              <label class="form-label" for="display-name">Display Name</label>
              <input type="text" id="display-name" class="form-input" required minlength="2" maxlength="100">
            </div>
            <div class="form-group">
              <label class="form-label" for="email">Email</label>
              <input type="email" id="email" class="form-input" required autocomplete="email">
            </div>
            <div class="form-group">
              <label class="form-label" for="password">Password</label>
              <input type="password" id="password" class="form-input" required minlength="8" autocomplete="new-password">
              <small class="text-muted">At least 8 characters with uppercase, lowercase, and number</small>
            </div>
            <div id="register-error" class="form-error hidden"></div>
            <button type="submit" class="btn btn-primary btn-lg" style="width: 100%;">
              Create Account
            </button>
          </form>
          <div class="auth-footer">
            Already have an account? <a href="#login">Sign in</a>
          </div>
        </div>
      </div>
    `;

    document.getElementById('register-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const displayName = document.getElementById('display-name').value;
      const email = document.getElementById('email').value;
      const password = document.getElementById('password').value;
      const errorEl = document.getElementById('register-error');

      try {
        const response = await API.auth.register(email, password, displayName);
        this.user = response.user;
        this.setupNavigation();
        window.location.hash = '#create';
        this.toast('Account created! Let\'s make your first card.', 'success');
      } catch (error) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      }
    });
  },

  async renderDashboard(container) {
    container.innerHTML = `
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem;">
        <h2>My Bingo Cards</h2>
        <a href="#create" class="btn btn-primary">+ New Card</a>
      </div>
      <div id="cards-list">
        <div class="text-center"><div class="spinner" style="margin: 2rem auto;"></div></div>
      </div>
    `;

    try {
      const response = await API.cards.list();
      const cards = response.cards || [];

      const listEl = document.getElementById('cards-list');
      if (cards.length === 0) {
        listEl.innerHTML = `
          <div class="card text-center" style="padding: 3rem;">
            <div style="font-size: 4rem; margin-bottom: 1rem;">🎯</div>
            <h3>No cards yet</h3>
            <p class="text-muted mb-lg">Create your first bingo card and start tracking your goals!</p>
            <a href="#create" class="btn btn-primary btn-lg">Create Your First Card</a>
          </div>
        `;
      } else {
        listEl.innerHTML = cards.map(card => this.renderCardPreview(card)).join('');
      }
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  renderCardPreview(card) {
    const itemCount = card.items ? card.items.length : 0;
    const completedCount = card.items ? card.items.filter(i => i.is_completed).length : 0;
    const progress = card.is_finalized ? Math.round((completedCount / 24) * 100) : Math.round((itemCount / 24) * 100);

    return `
      <a href="#card/${card.id}" class="card" style="display: block; margin-bottom: 1rem; text-decoration: none;">
        <div style="display: flex; justify-content: space-between; align-items: start;">
          <div>
            <h3 style="margin-bottom: 0.5rem;">${card.year} Bingo Card</h3>
            <p class="text-muted">
              ${card.is_finalized
                ? `${completedCount}/24 completed`
                : `${itemCount}/24 items added`}
            </p>
          </div>
          <span class="btn btn-ghost btn-sm">
            ${card.is_finalized ? 'View' : 'Continue'}
          </span>
        </div>
        <div class="progress-bar mt-md">
          <div class="progress-fill" style="width: ${progress}%"></div>
        </div>
      </a>
    `;
  },

  async renderCreate(container) {
    const currentYear = new Date().getFullYear();
    const nextYear = currentYear + 1;

    // Check if cards already exist
    let existingCards = [];
    try {
      const response = await API.cards.list();
      existingCards = response.cards || [];
    } catch (error) {
      // Ignore
    }

    const hasCurrentYear = existingCards.some(c => c.year === currentYear);
    const hasNextYear = existingCards.some(c => c.year === nextYear);

    if (hasCurrentYear && hasNextYear) {
      container.innerHTML = `
        <div class="card text-center" style="max-width: 500px; margin: 2rem auto; padding: 3rem;">
          <h3>All caught up!</h3>
          <p class="text-muted mb-lg">You already have bingo cards for ${currentYear} and ${nextYear}.</p>
          <a href="#dashboard" class="btn btn-primary">View My Cards</a>
        </div>
      `;
      return;
    }

    container.innerHTML = `
      <div class="card" style="max-width: 500px; margin: 2rem auto;">
        <div class="card-header text-center">
          <h2 class="card-title">Create New Card</h2>
          <p class="card-subtitle">Choose a year for your bingo card</p>
        </div>
        <div style="display: flex; flex-direction: column; gap: 1rem;">
          ${!hasCurrentYear ? `
            <button class="btn btn-secondary btn-lg" onclick="App.createCard(${currentYear})">
              ${currentYear} Card
            </button>
          ` : ''}
          ${!hasNextYear ? `
            <button class="btn btn-primary btn-lg" onclick="App.createCard(${nextYear})">
              ${nextYear} Card
            </button>
          ` : ''}
        </div>
      </div>
    `;
  },

  async createCard(year) {
    try {
      const response = await API.cards.create(year);
      this.currentCard = response.card;
      window.location.hash = `#card/${response.card.id}`;
      this.toast(`${year} card created!`, 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async renderCard(container, cardId) {
    container.innerHTML = `
      <div class="text-center"><div class="spinner" style="margin: 2rem auto;"></div></div>
    `;

    try {
      const [cardResponse, suggestionsResponse] = await Promise.all([
        API.cards.get(cardId),
        API.suggestions.getGrouped(),
      ]);

      this.currentCard = cardResponse.card;
      this.suggestions = suggestionsResponse.grouped || [];
      this.usedSuggestions = new Set(
        (this.currentCard.items || []).map(i => i.content.toLowerCase())
      );

      if (this.currentCard.is_finalized) {
        this.renderFinalizedCard(container);
      } else {
        this.renderCardEditor(container);
      }
    } catch (error) {
      container.innerHTML = `
        <div class="card text-center" style="padding: 3rem;">
          <h3>Card not found</h3>
          <p class="text-muted mb-lg">${error.message}</p>
          <a href="#dashboard" class="btn btn-primary">Back to Dashboard</a>
        </div>
      `;
    }
  },

  renderCardEditor(container) {
    const itemCount = this.currentCard.items ? this.currentCard.items.length : 0;
    const progress = Math.round((itemCount / 24) * 100);

    container.innerHTML = `
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
        <a href="#dashboard" class="btn btn-ghost">&larr; Back</a>
        <h2>${this.currentCard.year} Bingo Card</h2>
        <div></div>
      </div>

      <div class="progress-bar">
        <div class="progress-fill" style="width: ${progress}%"></div>
      </div>
      <p class="progress-text mb-lg">${itemCount}/24 items added</p>

      <div style="display: grid; grid-template-columns: 1fr 350px; gap: 2rem; align-items: start;">
        <div class="bingo-container">
          <div class="bingo-grid" id="bingo-grid">
            ${this.renderGrid()}
          </div>

          <div class="input-area" style="width: 100%; max-width: 600px;">
            <input type="text" id="item-input" class="form-input" placeholder="Type your goal or pick a suggestion..." maxlength="500" ${itemCount >= 24 ? 'disabled' : ''}>
            <button class="btn btn-primary" id="add-btn" ${itemCount >= 24 ? 'disabled' : ''}>Add</button>
          </div>

          <div class="action-bar">
            <button class="btn btn-secondary" onclick="App.shuffleCard()" ${itemCount === 0 ? 'disabled' : ''}>
              🔀 Shuffle
            </button>
            <button class="btn btn-primary" onclick="App.finalizeCard()" ${itemCount < 24 ? 'disabled' : ''}>
              ✓ Finalize Card
            </button>
          </div>
        </div>

        <div class="suggestions-panel">
          <div class="suggestions-header">
            <h3 class="suggestions-title">Suggestions</h3>
          </div>
          <div class="suggestions-categories" id="category-tabs">
            ${this.suggestions.map((cat, i) => `
              <button class="category-tab ${i === 0 ? 'category-tab--active' : ''}" data-category="${cat.category}">
                ${cat.category.split(' ')[0]}
              </button>
            `).join('')}
          </div>
          <div class="suggestions-list" id="suggestions-list">
            ${this.renderSuggestions(this.suggestions[0]?.category)}
          </div>
        </div>
      </div>
    `;

    this.setupEditorEvents();
  },

  renderFinalizedCard(container) {
    const completedCount = this.currentCard.items.filter(i => i.is_completed).length;
    const progress = Math.round((completedCount / 24) * 100);

    container.innerHTML = `
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
        <a href="#dashboard" class="btn btn-ghost">&larr; Back</a>
        <h2>${this.currentCard.year} Bingo Card</h2>
        <div></div>
      </div>

      <div class="progress-bar">
        <div class="progress-fill" style="width: ${progress}%"></div>
      </div>
      <p class="progress-text mb-lg">${completedCount}/24 completed</p>

      <div class="bingo-container">
        <div class="bingo-grid" id="bingo-grid">
          ${this.renderGrid(true)}
        </div>
      </div>
    `;

    this.setupFinalizedCardEvents();
  },

  renderGrid(finalized = false) {
    const cells = [];
    const itemsByPosition = {};

    if (this.currentCard.items) {
      this.currentCard.items.forEach(item => {
        itemsByPosition[item.position] = item;
      });
    }

    for (let i = 0; i < 25; i++) {
      if (i === 12) {
        // Free space
        cells.push(`
          <div class="bingo-cell bingo-cell--free">
            <span class="bingo-cell-content">FREE</span>
          </div>
        `);
      } else {
        const item = itemsByPosition[i];
        if (item) {
          const isCompleted = item.is_completed;
          cells.push(`
            <div class="bingo-cell ${isCompleted ? 'bingo-cell--completed' : ''}"
                 data-position="${i}"
                 data-item-id="${item.id}"
                 ${!finalized ? 'draggable="true"' : ''}>
              <span class="bingo-cell-content">${this.escapeHtml(item.content)}</span>
            </div>
          `);
        } else {
          cells.push(`
            <div class="bingo-cell bingo-cell--empty" data-position="${i}"></div>
          `);
        }
      }
    }

    return cells.join('');
  },

  renderSuggestions(category) {
    const categoryData = this.suggestions.find(c => c.category === category);
    if (!categoryData) return '<p class="text-muted">No suggestions available</p>';

    return categoryData.suggestions.map(suggestion => {
      const isUsed = this.usedSuggestions.has(suggestion.content.toLowerCase());
      return `
        <div class="suggestion-item ${isUsed ? 'suggestion-item--used' : ''}"
             data-content="${this.escapeHtml(suggestion.content)}"
             ${isUsed ? '' : 'onclick="App.addSuggestion(this)"'}>
          ${this.escapeHtml(suggestion.content)}
        </div>
      `;
    }).join('');
  },

  setupEditorEvents() {
    // Add item on button click or enter
    const input = document.getElementById('item-input');
    const addBtn = document.getElementById('add-btn');

    addBtn.addEventListener('click', () => this.addItem());
    input.addEventListener('keypress', (e) => {
      if (e.key === 'Enter') this.addItem();
    });

    // Category tabs
    document.getElementById('category-tabs').addEventListener('click', (e) => {
      if (e.target.classList.contains('category-tab')) {
        document.querySelectorAll('.category-tab').forEach(t => t.classList.remove('category-tab--active'));
        e.target.classList.add('category-tab--active');
        document.getElementById('suggestions-list').innerHTML = this.renderSuggestions(e.target.dataset.category);
      }
    });

    // Drag and drop
    this.setupDragAndDrop();

    // Cell click to remove (before finalized)
    document.getElementById('bingo-grid').addEventListener('click', (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (cell && !cell.classList.contains('bingo-cell--empty') && !cell.classList.contains('bingo-cell--free')) {
        this.showItemOptions(cell);
      }
    });
  },

  setupFinalizedCardEvents() {
    document.getElementById('bingo-grid').addEventListener('click', async (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (!cell || cell.classList.contains('bingo-cell--free')) return;

      const position = parseInt(cell.dataset.position);
      const isCompleted = cell.classList.contains('bingo-cell--completed');

      try {
        if (isCompleted) {
          await API.cards.uncompleteItem(this.currentCard.id, position);
          cell.classList.remove('bingo-cell--completed');
          this.toast('Item unchecked', 'success');
        } else {
          // Show completion modal
          this.showCompletionModal(position, cell);
        }

        // Update progress
        const completedCount = document.querySelectorAll('.bingo-cell--completed').length;
        const progress = Math.round((completedCount / 24) * 100);
        document.querySelector('.progress-fill').style.width = `${progress}%`;
        document.querySelector('.progress-text').textContent = `${completedCount}/24 completed`;
      } catch (error) {
        this.toast(error.message, 'error');
      }
    });
  },

  showCompletionModal(position, cell) {
    this.openModal('Mark Complete', `
      <form id="complete-form">
        <div class="form-group">
          <label class="form-label">Notes (optional)</label>
          <textarea id="complete-notes" class="form-input" rows="3" placeholder="How did you accomplish this?"></textarea>
        </div>
        <div style="display: flex; gap: 1rem;">
          <button type="button" class="btn btn-secondary" style="flex: 1;" onclick="App.completeItemQuick(${position})">
            Just Mark Complete
          </button>
          <button type="submit" class="btn btn-primary" style="flex: 1;">
            Save with Notes
          </button>
        </div>
      </form>
    `);

    document.getElementById('complete-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const notes = document.getElementById('complete-notes').value;
      await this.completeItem(position, notes);
    });
  },

  async completeItemQuick(position) {
    await this.completeItem(position, null);
  },

  async completeItem(position, notes) {
    try {
      await API.cards.completeItem(this.currentCard.id, position, notes);
      const cell = document.querySelector(`[data-position="${position}"]`);
      cell.classList.add('bingo-cell--completed', 'bingo-cell--completing');
      setTimeout(() => cell.classList.remove('bingo-cell--completing'), 400);
      this.closeModal();
      this.toast('Item completed! 🎉', 'success');
      this.checkForBingo();

      // Update progress
      const completedCount = document.querySelectorAll('.bingo-cell--completed').length;
      const progress = Math.round((completedCount / 24) * 100);
      document.querySelector('.progress-fill').style.width = `${progress}%`;
      document.querySelector('.progress-text').textContent = `${completedCount}/24 completed`;
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  setupDragAndDrop() {
    const grid = document.getElementById('bingo-grid');
    let draggedCell = null;

    grid.addEventListener('dragstart', (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (!cell || cell.classList.contains('bingo-cell--empty') || cell.classList.contains('bingo-cell--free')) {
        e.preventDefault();
        return;
      }
      draggedCell = cell;
      cell.classList.add('bingo-cell--dragging');
      e.dataTransfer.effectAllowed = 'move';
    });

    grid.addEventListener('dragend', (e) => {
      if (draggedCell) {
        draggedCell.classList.remove('bingo-cell--dragging');
        draggedCell = null;
      }
      document.querySelectorAll('.bingo-cell--drag-over').forEach(c => c.classList.remove('bingo-cell--drag-over'));
    });

    grid.addEventListener('dragover', (e) => {
      e.preventDefault();
      const cell = e.target.closest('.bingo-cell');
      if (cell && !cell.classList.contains('bingo-cell--free') && cell !== draggedCell) {
        cell.classList.add('bingo-cell--drag-over');
      }
    });

    grid.addEventListener('dragleave', (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (cell) {
        cell.classList.remove('bingo-cell--drag-over');
      }
    });

    grid.addEventListener('drop', async (e) => {
      e.preventDefault();
      const targetCell = e.target.closest('.bingo-cell');
      if (!targetCell || targetCell === draggedCell || targetCell.classList.contains('bingo-cell--free')) return;

      const fromPosition = parseInt(draggedCell.dataset.position);
      const toPosition = parseInt(targetCell.dataset.position);

      try {
        if (targetCell.classList.contains('bingo-cell--empty')) {
          // Move to empty cell
          await API.cards.updateItem(this.currentCard.id, fromPosition, { position: toPosition });
        } else {
          // Swap positions - need to handle this differently
          // For now, just show an error
          this.toast('Cannot swap items directly. Try shuffling instead.', 'error');
          return;
        }

        // Refresh the card
        const response = await API.cards.get(this.currentCard.id);
        this.currentCard = response.card;
        document.getElementById('bingo-grid').innerHTML = this.renderGrid();
        this.setupDragAndDrop();
      } catch (error) {
        this.toast(error.message, 'error');
      }
    });
  },

  showItemOptions(cell) {
    const position = cell.dataset.position;
    const content = cell.querySelector('.bingo-cell-content').textContent;

    this.openModal('Edit Item', `
      <p style="margin-bottom: 1rem;">${this.escapeHtml(content)}</p>
      <div style="display: flex; gap: 1rem;">
        <button class="btn btn-secondary" style="flex: 1;" onclick="App.closeModal()">
          Cancel
        </button>
        <button class="btn btn-primary" style="flex: 1; background: var(--color-error);" onclick="App.removeItem(${position})">
          Remove
        </button>
      </div>
    `);
  },

  async addItem() {
    const input = document.getElementById('item-input');
    const content = input.value.trim();

    if (!content) {
      this.toast('Please enter a goal', 'error');
      return;
    }

    try {
      const response = await API.cards.addItem(this.currentCard.id, content);
      input.value = '';

      // Update local state
      if (!this.currentCard.items) this.currentCard.items = [];
      this.currentCard.items.push(response.item);
      this.usedSuggestions.add(content.toLowerCase());

      // Update grid with animation
      const position = response.item.position;
      const cell = document.querySelector(`[data-position="${position}"]`);
      cell.classList.remove('bingo-cell--empty');
      cell.classList.add('bingo-cell--appearing');
      cell.dataset.itemId = response.item.id;
      cell.draggable = true;
      cell.innerHTML = `<span class="bingo-cell-content">${this.escapeHtml(content)}</span>`;

      // Update progress
      const itemCount = this.currentCard.items.length;
      const progress = Math.round((itemCount / 24) * 100);
      document.querySelector('.progress-fill').style.width = `${progress}%`;
      document.querySelector('.progress-text').textContent = `${itemCount}/24 items added`;

      // Update buttons
      if (itemCount >= 24) {
        input.disabled = true;
        document.getElementById('add-btn').disabled = true;
        document.querySelector('[onclick="App.finalizeCard()"]').disabled = false;
      }
      document.querySelector('[onclick="App.shuffleCard()"]').disabled = false;

      // Update suggestions
      const activeTab = document.querySelector('.category-tab--active');
      if (activeTab) {
        document.getElementById('suggestions-list').innerHTML = this.renderSuggestions(activeTab.dataset.category);
      }

      this.confetti();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  addSuggestion(element) {
    const content = element.dataset.content;
    document.getElementById('item-input').value = content;
    this.addItem();
  },

  async removeItem(position) {
    try {
      const item = this.currentCard.items.find(i => i.position === position);
      await API.cards.removeItem(this.currentCard.id, position);

      // Update local state
      this.currentCard.items = this.currentCard.items.filter(i => i.position !== position);
      if (item) {
        this.usedSuggestions.delete(item.content.toLowerCase());
      }

      // Update grid
      const cell = document.querySelector(`[data-position="${position}"]`);
      cell.className = 'bingo-cell bingo-cell--empty';
      cell.removeAttribute('data-item-id');
      cell.removeAttribute('draggable');
      cell.innerHTML = '';

      // Update progress
      const itemCount = this.currentCard.items.length;
      const progress = Math.round((itemCount / 24) * 100);
      document.querySelector('.progress-fill').style.width = `${progress}%`;
      document.querySelector('.progress-text').textContent = `${itemCount}/24 items added`;

      // Update buttons
      document.getElementById('item-input').disabled = false;
      document.getElementById('add-btn').disabled = false;
      document.querySelector('[onclick="App.finalizeCard()"]').disabled = true;
      if (itemCount === 0) {
        document.querySelector('[onclick="App.shuffleCard()"]').disabled = true;
      }

      // Update suggestions
      const activeTab = document.querySelector('.category-tab--active');
      if (activeTab) {
        document.getElementById('suggestions-list').innerHTML = this.renderSuggestions(activeTab.dataset.category);
      }

      this.closeModal();
      this.toast('Item removed', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async shuffleCard() {
    try {
      // Add shuffle animation to all cells
      document.querySelectorAll('.bingo-cell:not(.bingo-cell--free):not(.bingo-cell--empty)').forEach(cell => {
        cell.classList.add('bingo-cell--shuffling');
      });

      const response = await API.cards.shuffle(this.currentCard.id);
      this.currentCard = response.card;

      // Wait for animation then update
      setTimeout(() => {
        document.getElementById('bingo-grid').innerHTML = this.renderGrid();
        this.setupDragAndDrop();
      }, 300);

      this.toast('Items shuffled!', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async finalizeCard() {
    if (!confirm('Are you sure you want to finalize this card? You won\'t be able to change the items after this.')) {
      return;
    }

    try {
      const response = await API.cards.finalize(this.currentCard.id);
      this.currentCard = response.card;
      this.renderFinalizedCard(document.querySelector('.container'));
      this.toast('Card finalized! Good luck with your goals! 🎉', 'success');
      this.confetti(50);
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  checkForBingo() {
    const cells = document.querySelectorAll('.bingo-cell');
    const grid = [];
    cells.forEach((cell, i) => {
      grid.push(cell.classList.contains('bingo-cell--completed') || cell.classList.contains('bingo-cell--free'));
    });

    // Check rows
    for (let row = 0; row < 5; row++) {
      if (grid.slice(row * 5, row * 5 + 5).every(Boolean)) {
        this.toast('BINGO! Row complete! 🎉🎉🎉', 'success');
        this.confetti(100);
        return;
      }
    }

    // Check columns
    for (let col = 0; col < 5; col++) {
      if ([0, 1, 2, 3, 4].map(row => grid[row * 5 + col]).every(Boolean)) {
        this.toast('BINGO! Column complete! 🎉🎉🎉', 'success');
        this.confetti(100);
        return;
      }
    }

    // Check diagonals
    if ([0, 6, 12, 18, 24].map(i => grid[i]).every(Boolean)) {
      this.toast('BINGO! Diagonal complete! 🎉🎉🎉', 'success');
      this.confetti(100);
      return;
    }
    if ([4, 8, 12, 16, 20].map(i => grid[i]).every(Boolean)) {
      this.toast('BINGO! Diagonal complete! 🎉🎉🎉', 'success');
      this.confetti(100);
      return;
    }
  },

  async logout() {
    try {
      await API.auth.logout();
      this.user = null;
      this.setupNavigation();
      window.location.hash = '#home';
      this.toast('Logged out successfully', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  // Utilities
  toast(message, type = 'success') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast toast--${type}`;
    toast.textContent = message;
    container.appendChild(toast);

    setTimeout(() => {
      toast.style.opacity = '0';
      setTimeout(() => toast.remove(), 300);
    }, 3000);
  },

  confetti(count = 30) {
    const colors = ['#ffd700', '#ff6b6b', '#4ecdc4', '#a855f7', '#ffffff'];
    for (let i = 0; i < count; i++) {
      const confetti = document.createElement('div');
      confetti.className = 'confetti';
      confetti.style.left = Math.random() * 100 + 'vw';
      confetti.style.backgroundColor = colors[Math.floor(Math.random() * colors.length)];
      confetti.style.animationDelay = Math.random() * 2 + 's';
      confetti.style.transform = `rotate(${Math.random() * 360}deg)`;
      document.body.appendChild(confetti);

      setTimeout(() => confetti.remove(), 5000);
    }
  },

  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  },
};

// Initialize app
document.addEventListener('DOMContentLoaded', () => {
  App.init();
});

// Handle hash changes
window.addEventListener('hashchange', () => {
  App.route();
});
