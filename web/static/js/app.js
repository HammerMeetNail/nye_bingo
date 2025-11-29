// Year of Bingo - Main Application

const App = {
  user: null,
  currentCard: null,
  suggestions: [],
  usedSuggestions: new Set(),
  allowedEmojis: ['🎉', '👏', '🔥', '❤️', '⭐'],
  isLoading: false,
  isAnonymousMode: false, // True when editing an anonymous card (localStorage)

  async init() {
    await API.init();
    await this.checkAuth();
    this.setupNavigation();
    this.setupModal();
    this.setupOfflineDetection();
    this.route();
  },

  // Loading state management
  showLoading(container, message = 'Loading...') {
    this.isLoading = true;
    if (container) {
      container.innerHTML = `
        <div class="loading-state" role="status" aria-live="polite">
          <div class="spinner" aria-hidden="true"></div>
          <p class="loading-message">${this.escapeHtml(message)}</p>
        </div>
      `;
    }
  },

  hideLoading() {
    this.isLoading = false;
  },

  // Show inline loading on a button
  setButtonLoading(button, loading) {
    if (loading) {
      button.disabled = true;
      button.dataset.originalText = button.textContent;
      button.innerHTML = '<span class="spinner spinner--small" aria-hidden="true"></span> Loading...';
    } else {
      button.disabled = false;
      if (button.dataset.originalText) {
        button.textContent = button.dataset.originalText;
        delete button.dataset.originalText;
      }
    }
  },

  // Offline detection
  setupOfflineDetection() {
    window.addEventListener('online', () => {
      this.toast('Connection restored', 'success');
      // Could trigger a data refresh here
    });

    window.addEventListener('offline', () => {
      this.toast('You are offline. Some features may not work.', 'error');
    });
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
        <a href="#friends" class="nav-link">Friends</a>
        <a href="#profile" class="nav-link">Hi, ${this.escapeHtml(this.user.username)}</a>
        <button class="btn btn-ghost" onclick="App.logout()">Logout</button>
      `;
    } else {
      nav.innerHTML = `
        <a href="#login" class="btn btn-ghost">Login</a>
        <a href="#create" class="btn btn-primary">Get Started</a>
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

  async showCreateCardModal() {
    const currentYear = new Date().getFullYear();
    const nextYear = currentYear + 1;

    // Fetch categories
    let categories = [];
    try {
      const response = await API.cards.getCategories();
      categories = response.categories || [];
    } catch (error) {
      categories = [
        { id: 'personal', name: 'Personal Growth' },
        { id: 'health', name: 'Health & Fitness' },
        { id: 'food', name: 'Food & Dining' },
        { id: 'travel', name: 'Travel & Adventure' },
        { id: 'hobbies', name: 'Hobbies & Creativity' },
        { id: 'social', name: 'Social & Relationships' },
        { id: 'professional', name: 'Professional & Career' },
        { id: 'fun', name: 'Fun & Silly' },
      ];
    }

    const categoryOptions = categories.map(c =>
      `<option value="${this.escapeHtml(c.id)}">${this.escapeHtml(c.name)}</option>`
    ).join('');

    this.openModal('Create New Card', `
      <form onsubmit="App.handleCreateCardModal(event)">
        <div class="form-group">
          <label for="modal-card-year">Year</label>
          <select id="modal-card-year" class="form-input" required>
            <option value="${currentYear}">${currentYear}</option>
            <option value="${nextYear}">${nextYear}</option>
          </select>
        </div>

        <div class="form-group">
          <label for="modal-card-title">
            Title <span class="text-muted" style="font-weight: normal;">(optional)</span>
          </label>
          <input type="text" id="modal-card-title" class="form-input"
                 placeholder="e.g., Life Goals, Foods to Try"
                 maxlength="100">
          <small class="text-muted">Leave blank for default "${currentYear} Bingo Card"</small>
        </div>

        <div class="form-group">
          <label for="modal-card-category">
            Category <span class="text-muted" style="font-weight: normal;">(optional)</span>
          </label>
          <select id="modal-card-category" class="form-input">
            <option value="">None</option>
            ${categoryOptions}
          </select>
        </div>

        <div style="display: flex; gap: 0.5rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-ghost" style="flex: 1;" onclick="App.closeModal()">Cancel</button>
          <button type="submit" class="btn btn-primary" style="flex: 1;">Create Card</button>
        </div>
      </form>
    `);
  },

  async handleCreateCardModal(event) {
    event.preventDefault();

    const year = parseInt(document.getElementById('modal-card-year').value, 10);
    const title = document.getElementById('modal-card-title').value.trim() || null;
    const category = document.getElementById('modal-card-category').value || null;

    try {
      const response = await API.cards.create(year, title, category);

      // Check for conflict
      if (response.error === 'card_exists') {
        this.showCreateCardConflictModal(response.existing_card, year, category);
        return;
      }

      this.currentCard = response.card;
      this.closeModal();
      window.location.hash = `#card/${response.card.id}`;
      const cardName = title || `${year} Bingo Card`;
      this.toast(`${cardName} created!`, 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async showEditCardMetaModal() {
    if (!this.currentCard) return;

    // Fetch categories
    let categories = [];
    try {
      const response = await API.cards.getCategories();
      categories = response.categories || [];
    } catch (error) {
      categories = [
        { id: 'personal', name: 'Personal Growth' },
        { id: 'health', name: 'Health & Fitness' },
        { id: 'food', name: 'Food & Dining' },
        { id: 'travel', name: 'Travel & Adventure' },
        { id: 'hobbies', name: 'Hobbies & Creativity' },
        { id: 'social', name: 'Social & Relationships' },
        { id: 'professional', name: 'Professional & Career' },
        { id: 'fun', name: 'Fun & Silly' },
      ];
    }

    const currentTitle = this.currentCard.title || '';
    const currentCategory = this.currentCard.category || '';

    const categoryOptions = categories.map(c => {
      const selected = c.id === currentCategory ? 'selected' : '';
      return `<option value="${this.escapeHtml(c.id)}" ${selected}>${this.escapeHtml(c.name)}</option>`;
    }).join('');

    this.openModal('Edit Card', `
      <form onsubmit="App.saveCardMeta(event)">
        <div class="form-group">
          <label for="edit-card-title">Title</label>
          <input type="text" id="edit-card-title" class="form-input"
                 value="${this.escapeHtml(currentTitle)}"
                 placeholder="e.g., Life Goals, Foods to Try"
                 maxlength="100">
          <small class="text-muted">Leave blank for default "${this.currentCard.year} Bingo Card"</small>
        </div>

        <div class="form-group">
          <label for="edit-card-category">Category</label>
          <select id="edit-card-category" class="form-input">
            <option value="" ${!currentCategory ? 'selected' : ''}>None</option>
            ${categoryOptions}
          </select>
        </div>

        <div style="display: flex; gap: 1rem; justify-content: flex-end;">
          <button type="button" class="btn btn-ghost" onclick="App.closeModal()">Cancel</button>
          <button type="submit" class="btn btn-primary">Save</button>
        </div>
      </form>
    `);
  },

  async saveCardMeta(event) {
    event.preventDefault();

    const title = document.getElementById('edit-card-title').value.trim() || null;
    const category = document.getElementById('edit-card-category').value || null;

    try {
      const response = await API.cards.updateMeta(this.currentCard.id, title, category);
      this.currentCard = response.card;
      this.closeModal();
      this.toast('Card updated', 'success');

      // Re-render the current view
      const container = document.getElementById('main-container');
      if (this.currentCard.is_finalized) {
        this.renderFinalizedCard(container);
      } else {
        this.renderCardEditor(container);
      }
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  route() {
    const hash = window.location.hash.slice(1) || 'home';
    // Parse hash with query parameters: page?param=value
    const [pagePart, queryPart] = hash.split('?');
    const [page, ...params] = pagePart.split('/');
    const queryParams = new URLSearchParams(queryPart || '');

    const container = document.getElementById('main-container');
    if (!container) return;

    switch (page) {
      case 'home':
        this.renderHome(container);
        break;
      case 'login':
        this.renderLogin(container, queryParams.get('error'));
        break;
      case 'register':
        this.renderRegister(container);
        break;
      case 'magic-link':
        if (queryParams.has('token')) {
          this.handleMagicLinkVerify(container, queryParams.get('token'));
        } else {
          this.renderMagicLinkRequest(container);
        }
        break;
      case 'forgot-password':
        this.renderForgotPassword(container);
        break;
      case 'reset-password':
        this.renderResetPassword(container, queryParams.get('token'));
        break;
      case 'verify-email':
        this.handleVerifyEmail(container, queryParams.get('token'));
        break;
      case 'check-email':
        this.renderCheckEmail(container, queryParams.get('type'), queryParams.get('email'));
        break;
      case 'dashboard':
        this.requireAuth(() => this.renderDashboard(container));
        break;
      case 'create':
        // Allow anonymous users to create a card
        this.renderCreate(container);
        break;
      case 'card':
        this.requireAuth(() => this.renderCard(container, params[0]));
        break;
      case 'friends':
        this.requireAuth(() => this.renderFriends(container));
        break;
      case 'friend-card':
        this.requireAuth(() => this.renderFriendCard(container, params[0]));
        break;
      case 'archive':
        // Redirect to dashboard (archive merged into dashboard)
        window.location.hash = '#dashboard';
        return;
      case 'archive-card':
        this.requireAuth(() => this.renderArchiveCard(container, params[0]));
        break;
      case 'profile':
        this.requireAuth(() => this.renderProfile(container));
        break;
      case 'terms':
        this.renderTerms(container);
        break;
      case 'privacy':
        this.renderPrivacy(container);
        break;
      case 'security':
        this.renderSecurity(container);
        break;
      case 'support':
        this.renderSupport(container);
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
          Year of <span class="text-gold">Bingo</span>
        </h1>
        <p style="font-size: 1.25rem; max-width: 600px; margin: 0 auto 2rem;">
          Turn your goals into an exciting game! Create a bingo card
          with 24 goals and track your progress throughout the year.
        </p>
        ${this.user ? `
          <div style="display: flex; gap: 1rem; justify-content: center; flex-wrap: wrap;">
            <a href="#dashboard" class="btn btn-primary btn-lg">Go to Dashboard</a>
            <button class="btn btn-secondary btn-lg" onclick="App.showCreateCardModal()">Create New Card</button>
          </div>
        ` : `
          ${AnonymousCard.exists() ? `
            <a href="#create" class="btn btn-primary btn-lg">Continue Your Card</a>
          ` : `
            <a href="#create" class="btn btn-primary btn-lg">Create Your Card</a>
          `}
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

  renderLogin(container, errorMessage = null) {
    if (this.user) {
      window.location.hash = '#dashboard';
      return;
    }

    const errorMessages = {
      'invalid_link': 'This login link is invalid or has expired.',
      'link_used': 'This login link has already been used.',
    };
    const displayError = errorMessages[errorMessage] || errorMessage;

    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card">
          <div class="auth-header">
            <h2 class="auth-title">Welcome Back</h2>
            <p class="text-muted">Sign in to your account</p>
          </div>
          ${displayError ? `<div class="form-error" style="margin-bottom: 1rem;">${this.escapeHtml(displayError)}</div>` : ''}
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
          <div style="text-align: center; margin: 1rem 0;">
            <a href="#forgot-password" class="text-muted">Forgot password?</a>
          </div>
          <div class="auth-divider">
            <span>or</span>
          </div>
          <a href="#magic-link" class="btn btn-secondary btn-lg" style="width: 100%; margin-bottom: 1rem;">
            Sign in with email link
          </a>
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
              <label class="form-label" for="username">Username</label>
              <input type="text" id="username" class="form-input" required minlength="2" maxlength="100">
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
            <div class="form-group">
              <label class="checkbox-label">
                <input type="checkbox" id="searchable">
                <span>Allow others to find me by username</span>
              </label>
              <small class="text-muted">You can change this later in your account settings</small>
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
      const username = document.getElementById('username').value;
      const email = document.getElementById('email').value;
      const password = document.getElementById('password').value;
      const searchable = document.getElementById('searchable').checked;
      const errorEl = document.getElementById('register-error');

      try {
        const response = await API.auth.register(email, password, username, searchable);
        this.user = response.user;
        this.setupNavigation();
        window.location.hash = '#create';
        this.toast('Account created! Check your email to verify your account.', 'success');
      } catch (error) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      }
    });
  },

  // Magic Link Authentication
  renderMagicLinkRequest(container) {
    if (this.user) {
      window.location.hash = '#dashboard';
      return;
    }

    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card">
          <div class="auth-header">
            <h2 class="auth-title">Sign in with email link</h2>
            <p class="text-muted">We'll send you a link to sign in instantly</p>
          </div>
          <form id="magic-link-form">
            <div class="form-group">
              <label class="form-label" for="email">Email</label>
              <input type="email" id="email" class="form-input" required autocomplete="email">
            </div>
            <div id="magic-link-error" class="form-error hidden"></div>
            <button type="submit" class="btn btn-primary btn-lg" style="width: 100%;">
              Send login link
            </button>
          </form>
          <div class="auth-footer">
            <a href="#login">Back to sign in</a>
          </div>
        </div>
      </div>
    `;

    document.getElementById('magic-link-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const email = document.getElementById('email').value;
      const errorEl = document.getElementById('magic-link-error');
      const submitBtn = e.target.querySelector('button[type="submit"]');

      this.setButtonLoading(submitBtn, true);

      try {
        await API.auth.requestMagicLink(email);
        window.location.hash = `#check-email?type=magic-link&email=${encodeURIComponent(email)}`;
      } catch (error) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
        this.setButtonLoading(submitBtn, false);
      }
    });
  },

  async handleMagicLinkVerify(container, token) {
    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card text-center">
          <div class="spinner" style="margin: 2rem auto;"></div>
          <p>Signing you in...</p>
        </div>
      </div>
    `;

    try {
      const response = await API.auth.verifyMagicLink(token);
      this.user = response.user;
      this.setupNavigation();
      window.location.hash = '#dashboard';
      this.toast('Welcome back!', 'success');
    } catch (error) {
      window.location.hash = `#login?error=${encodeURIComponent(error.message)}`;
    }
  },

  // Forgot Password
  renderForgotPassword(container) {
    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card">
          <div class="auth-header">
            <h2 class="auth-title">Reset your password</h2>
            <p class="text-muted">Enter your email and we'll send you a reset link</p>
          </div>
          <form id="forgot-password-form">
            <div class="form-group">
              <label class="form-label" for="email">Email</label>
              <input type="email" id="email" class="form-input" required autocomplete="email">
            </div>
            <div id="forgot-error" class="form-error hidden"></div>
            <button type="submit" class="btn btn-primary btn-lg" style="width: 100%;">
              Send reset link
            </button>
          </form>
          <div class="auth-footer">
            <a href="#login">Back to sign in</a>
          </div>
        </div>
      </div>
    `;

    document.getElementById('forgot-password-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const email = document.getElementById('email').value;
      const submitBtn = e.target.querySelector('button[type="submit"]');

      this.setButtonLoading(submitBtn, true);

      try {
        await API.auth.forgotPassword(email);
        window.location.hash = `#check-email?type=reset&email=${encodeURIComponent(email)}`;
      } catch (error) {
        // Still redirect even on error to prevent email enumeration
        window.location.hash = `#check-email?type=reset&email=${encodeURIComponent(email)}`;
      }
    });
  },

  // Reset Password
  renderResetPassword(container, token) {
    if (!token) {
      container.innerHTML = `
        <div class="auth-page">
          <div class="card auth-card text-center">
            <h2>Invalid Reset Link</h2>
            <p class="text-muted">This password reset link is invalid or missing.</p>
            <a href="#forgot-password" class="btn btn-primary" style="margin-top: 1rem;">Request new link</a>
          </div>
        </div>
      `;
      return;
    }

    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card">
          <div class="auth-header">
            <h2 class="auth-title">Choose new password</h2>
            <p class="text-muted">Enter your new password below</p>
          </div>
          <form id="reset-password-form">
            <div class="form-group">
              <label class="form-label" for="password">New Password</label>
              <input type="password" id="password" class="form-input" required minlength="8" autocomplete="new-password">
              <small class="text-muted">At least 8 characters with uppercase, lowercase, and number</small>
            </div>
            <div class="form-group">
              <label class="form-label" for="confirm-password">Confirm Password</label>
              <input type="password" id="confirm-password" class="form-input" required minlength="8" autocomplete="new-password">
            </div>
            <div id="reset-error" class="form-error hidden"></div>
            <button type="submit" class="btn btn-primary btn-lg" style="width: 100%;">
              Reset Password
            </button>
          </form>
        </div>
      </div>
    `;

    document.getElementById('reset-password-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const password = document.getElementById('password').value;
      const confirmPassword = document.getElementById('confirm-password').value;
      const errorEl = document.getElementById('reset-error');
      const submitBtn = e.target.querySelector('button[type="submit"]');

      if (password !== confirmPassword) {
        errorEl.textContent = 'Passwords do not match';
        errorEl.classList.remove('hidden');
        return;
      }

      this.setButtonLoading(submitBtn, true);

      try {
        const response = await API.auth.resetPassword(token, password);
        this.user = response.user;
        this.setupNavigation();
        window.location.hash = '#dashboard';
        this.toast('Password reset successfully!', 'success');
      } catch (error) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
        this.setButtonLoading(submitBtn, false);
      }
    });
  },

  // Email Verification
  async handleVerifyEmail(container, token) {
    if (!token) {
      container.innerHTML = `
        <div class="auth-page">
          <div class="card auth-card text-center">
            <h2>Invalid Link</h2>
            <p class="text-muted">This verification link is invalid or missing.</p>
            ${this.user ? `<a href="#dashboard" class="btn btn-primary" style="margin-top: 1rem;">Go to Dashboard</a>` : `<a href="#login" class="btn btn-primary" style="margin-top: 1rem;">Sign In</a>`}
          </div>
        </div>
      `;
      return;
    }

    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card text-center">
          <div class="spinner" style="margin: 2rem auto;"></div>
          <p>Verifying your email...</p>
        </div>
      </div>
    `;

    try {
      await API.auth.verifyEmail(token);
      // Refresh user data
      if (this.user) {
        await this.checkAuth();
        this.setupNavigation();
      }
      container.innerHTML = `
        <div class="auth-page">
          <div class="card auth-card text-center">
            <div style="font-size: 4rem; margin-bottom: 1rem;">✓</div>
            <h2>Email Verified!</h2>
            <p class="text-muted">Your email has been verified successfully.</p>
            ${this.user ? `<a href="#dashboard" class="btn btn-primary" style="margin-top: 1rem;">Go to Dashboard</a>` : `<a href="#login" class="btn btn-primary" style="margin-top: 1rem;">Sign In</a>`}
          </div>
        </div>
      `;
    } catch (error) {
      container.innerHTML = `
        <div class="auth-page">
          <div class="card auth-card text-center">
            <div style="font-size: 4rem; margin-bottom: 1rem;">✗</div>
            <h2>Verification Failed</h2>
            <p class="text-muted">${this.escapeHtml(error.message)}</p>
            ${this.user ? `
              <button class="btn btn-primary" style="margin-top: 1rem;" onclick="App.resendVerification()">
                Resend Verification Email
              </button>
            ` : `<a href="#login" class="btn btn-primary" style="margin-top: 1rem;">Sign In</a>`}
          </div>
        </div>
      `;
    }
  },

  async resendVerification() {
    try {
      await API.auth.resendVerification();
      this.toast('Verification email sent!', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  // Check Email Interstitial
  renderCheckEmail(container, type, email) {
    const messages = {
      'magic-link': {
        title: 'Check your email',
        description: 'We sent a login link to',
        detail: 'Click the link in the email to sign in. The link expires in 15 minutes.',
      },
      'reset': {
        title: 'Check your email',
        description: 'If an account exists for',
        detail: 'you will receive a password reset link. The link expires in 1 hour.',
      },
      'verification': {
        title: 'Verify your email',
        description: 'We sent a verification link to',
        detail: 'Click the link to verify your email address. The link expires in 24 hours.',
      },
    };

    const msg = messages[type] || messages['magic-link'];

    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card text-center">
          <div style="font-size: 4rem; margin-bottom: 1rem;">✉️</div>
          <h2>${msg.title}</h2>
          <p class="text-muted">
            ${msg.description}<br>
            <strong>${email ? this.escapeHtml(email) : 'your email'}</strong>
          </p>
          <p class="text-muted" style="margin-top: 1rem;">
            ${msg.detail}
          </p>
          <div style="margin-top: 1.5rem;">
            <a href="#login" class="btn btn-ghost">Back to sign in</a>
          </div>
        </div>
      </div>
    `;
  },

  // Email verification banner for dashboard
  renderEmailVerificationBanner() {
    if (!this.user || this.user.email_verified) return '';
    return `
      <div class="verification-banner" style="background: rgba(245, 158, 11, 0.15); border: 1px solid #f59e0b; border-radius: 8px; padding: 1rem; margin-bottom: 1.5rem; display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 1rem;">
        <div style="color: #fff;">
          <strong style="color: #ffd700;">Please verify your email</strong>
          <span style="color: #b0b0c0;"> to enable all features.</span>
        </div>
        <button class="btn btn-secondary btn-sm" onclick="App.resendVerification()">
          Resend verification email
        </button>
      </div>
    `;
  },

  // Dashboard state
  selectedCards: [],
  dashboardCards: [],
  dashboardSortKey: localStorage.getItem('dashboardSort') || 'updated',

  async renderDashboard(container) {
    this.selectedCards = [];

    container.innerHTML = `
      ${this.renderEmailVerificationBanner()}
      <div class="dashboard-page">
        <div class="dashboard-header">
          <h2>My Bingo Cards</h2>
        </div>
        <div id="cards-list">
          <div class="text-center"><div class="spinner" style="margin: 2rem auto;"></div></div>
        </div>
      </div>
    `;

    try {
      const response = await API.cards.list();
      this.dashboardCards = response.cards || [];

      this.renderDashboardCards();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  renderDashboardCards() {
    const listEl = document.getElementById('cards-list');
    const cards = this.getSortedCards();

    if (cards.length === 0) {
      listEl.innerHTML = `
        <div class="card text-center" style="padding: 3rem;">
          <div style="font-size: 4rem; margin-bottom: 1rem;">🎯</div>
          <h3>No cards yet</h3>
          <p class="text-muted mb-lg">Create your first bingo card and start tracking your goals!</p>
          <a href="#create" class="btn btn-primary btn-lg">Create Your First Card</a>
        </div>
      `;
      return;
    }

    const hasSelection = this.selectedCards.length > 0;

    listEl.innerHTML = `
      <div class="dashboard-controls">
        <div class="dashboard-sort">
          <label for="sort-select" class="text-muted" style="font-size: 0.875rem;">Sort:</label>
          <select id="sort-select" class="form-input form-input--sm" onchange="App.changeDashboardSort(this.value)">
            <option value="updated" ${this.dashboardSortKey === 'updated' ? 'selected' : ''}>Recently Updated</option>
            <option value="year-desc" ${this.dashboardSortKey === 'year-desc' ? 'selected' : ''}>Year (newest)</option>
            <option value="year-asc" ${this.dashboardSortKey === 'year-asc' ? 'selected' : ''}>Year (oldest)</option>
            <option value="name-asc" ${this.dashboardSortKey === 'name-asc' ? 'selected' : ''}>Name (A-Z)</option>
            <option value="name-desc" ${this.dashboardSortKey === 'name-desc' ? 'selected' : ''}>Name (Z-A)</option>
            <option value="progress-desc" ${this.dashboardSortKey === 'progress-desc' ? 'selected' : ''}>Completion % (highest)</option>
            <option value="progress-asc" ${this.dashboardSortKey === 'progress-asc' ? 'selected' : ''}>Completion % (lowest)</option>
          </select>
        </div>
        <div class="dashboard-selection">
          <button class="btn btn-ghost btn-sm" onclick="App.selectAllCards()">Select All</button>
          <button class="btn btn-ghost btn-sm" onclick="App.deselectAllCards()">Deselect All</button>
          <span id="selected-count" class="text-muted">${this.selectedCards.length} selected</span>
        </div>
        <div class="dashboard-actions">
          <div class="dropdown" id="actions-dropdown">
            <button class="btn btn-secondary dropdown-toggle" aria-haspopup="true" aria-expanded="false">
              Actions
            </button>
            <div class="dropdown-menu" role="menu">
              <button class="dropdown-item ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" onclick="App.bulkSetArchive(true)" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-archive"></i> Archive
              </button>
              <button class="dropdown-item ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" onclick="App.bulkSetArchive(false)" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-box-open"></i> Unarchive
              </button>
              <div class="dropdown-divider"></div>
              <button class="dropdown-item ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" onclick="App.bulkSetVisibility(true)" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-eye"></i> Make Visible
              </button>
              <button class="dropdown-item ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" onclick="App.bulkSetVisibility(false)" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-eye-slash"></i> Make Private
              </button>
              <div class="dropdown-divider"></div>
              <button class="dropdown-item dropdown-item--danger ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" onclick="App.bulkDeleteCards()" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-trash"></i> Delete
              </button>
              <div class="dropdown-divider"></div>
              <button class="dropdown-item ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" onclick="App.exportSelectedCards()" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-download"></i> Export Cards
              </button>
            </div>
          </div>
          <button class="btn btn-primary" onclick="App.showCreateCardModal()">+ Card</button>
        </div>
      </div>
      <div class="dashboard-cards-list">
        ${cards.map(card => this.renderDashboardCardPreview(card)).join('')}
      </div>
    `;

    this.setupDropdowns();
  },

  renderDashboardCardPreview(card) {
    const itemCount = card.items ? card.items.length : 0;
    const completedCount = card.items ? card.items.filter(i => i.is_completed).length : 0;
    const progress = card.is_finalized ? Math.round((completedCount / 24) * 100) : Math.round((itemCount / 24) * 100);
    const displayName = this.getCardDisplayName(card);
    const categoryBadge = this.getCategoryBadge(card);
    const visibilityIcon = card.visible_to_friends ? 'eye' : 'eye-slash';
    const visibilityLabel = card.visible_to_friends ? 'Visible to friends' : 'Private';
    const isSelected = this.selectedCards.includes(card.id);
    const cardLink = card.is_archived ? `#archive-card/${card.id}` : `#card/${card.id}`;

    return `
      <div class="card dashboard-card-preview" style="margin-bottom: 1rem;">
        <div class="dashboard-card-preview-header">
          <div style="display: flex; align-items: flex-start; gap: 0.75rem;">
            <label class="dashboard-checkbox-label" onclick="event.stopPropagation();">
              <input type="checkbox" class="dashboard-card-checkbox" data-card-id="${card.id}" ${isSelected ? 'checked' : ''} onchange="App.updateDashboardSelection()">
            </label>
            <a href="${cardLink}" style="text-decoration: none; flex: 1;">
              <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 0.25rem;">
                <h3 style="margin: 0;">${displayName}</h3>
                <span class="year-badge">${card.year}</span>
                ${categoryBadge}
              </div>
              <p class="text-muted" style="margin: 0;">
                ${card.is_finalized
                  ? `${completedCount}/24 completed`
                  : `${itemCount}/24 items added`}
              </p>
            </a>
          </div>
          <div style="display: flex; align-items: center; gap: 0.5rem;">
            <span class="visibility-badge visibility-badge--${card.visible_to_friends ? 'visible' : 'private'}" title="${visibilityLabel}">
              <i class="fas fa-${visibilityIcon}"></i> ${card.visible_to_friends ? 'Visible' : 'Private'}
            </span>
            ${card.is_archived ? '<div class="archive-badge">Archived</div>' : ''}
          </div>
        </div>
        <a href="${cardLink}" style="text-decoration: none; display: block;">
          <div class="progress-bar mt-md">
            <div class="progress-fill" style="width: ${progress}%"></div>
          </div>
        </a>
      </div>
    `;
  },

  getSortedCards() {
    const cards = [...this.dashboardCards];
    const key = this.dashboardSortKey;

    const getDisplayName = (card) => {
      if (card.title) return card.title.toLowerCase();
      return `${card.year} bingo card`;
    };

    const getProgress = (card) => {
      if (!card.is_finalized) {
        const itemCount = card.items ? card.items.length : 0;
        return itemCount / 24;
      }
      const completedCount = card.items ? card.items.filter(i => i.is_completed).length : 0;
      return completedCount / 24;
    };

    switch (key) {
      case 'updated':
        return cards.sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at));
      case 'year-desc':
        return cards.sort((a, b) => b.year - a.year || new Date(b.updated_at) - new Date(a.updated_at));
      case 'year-asc':
        return cards.sort((a, b) => a.year - b.year || new Date(b.updated_at) - new Date(a.updated_at));
      case 'name-asc':
        return cards.sort((a, b) => getDisplayName(a).localeCompare(getDisplayName(b)));
      case 'name-desc':
        return cards.sort((a, b) => getDisplayName(b).localeCompare(getDisplayName(a)));
      case 'progress-desc':
        return cards.sort((a, b) => getProgress(b) - getProgress(a));
      case 'progress-asc':
        return cards.sort((a, b) => getProgress(a) - getProgress(b));
      default:
        return cards;
    }
  },

  changeDashboardSort(key) {
    this.dashboardSortKey = key;
    localStorage.setItem('dashboardSort', key);
    this.renderDashboardCards();
  },

  updateDashboardSelection() {
    const checkboxes = document.querySelectorAll('.dashboard-card-checkbox');
    this.selectedCards = Array.from(checkboxes)
      .filter(cb => cb.checked)
      .map(cb => cb.dataset.cardId);

    const countEl = document.getElementById('selected-count');
    if (countEl) {
      countEl.textContent = `${this.selectedCards.length} selected`;
    }

    // Re-render to update disabled states on dropdown items
    this.renderDashboardCards();
  },

  selectAllCards() {
    this.selectedCards = this.dashboardCards.map(card => card.id);
    this.renderDashboardCards();
  },

  deselectAllCards() {
    this.selectedCards = [];
    this.renderDashboardCards();
  },

  async bulkSetVisibility(visibleToFriends) {
    if (this.selectedCards.length === 0) {
      this.toast('Select cards first', 'warning');
      return;
    }

    try {
      const response = await API.cards.bulkUpdateVisibility(this.selectedCards, visibleToFriends);
      const count = response.updated_count || this.selectedCards.length;
      this.toast(`${count} card${count !== 1 ? 's' : ''} updated`, 'success');
      // Refresh the dashboard
      this.selectedCards = [];
      const cardsResponse = await API.cards.list();
      this.dashboardCards = cardsResponse.cards || [];
      this.renderDashboardCards();
    } catch (error) {
      this.toast(error.message || 'Failed to update visibility', 'error');
    }
  },

  async bulkSetArchive(isArchived) {
    if (this.selectedCards.length === 0) {
      this.toast('Select cards first', 'warning');
      return;
    }

    try {
      const response = await API.cards.bulkUpdateArchive(this.selectedCards, isArchived);
      const count = response.updated_count || this.selectedCards.length;
      const action = isArchived ? 'archived' : 'unarchived';
      this.toast(`${count} card${count !== 1 ? 's' : ''} ${action}`, 'success');
      // Refresh the dashboard
      this.selectedCards = [];
      const cardsResponse = await API.cards.list();
      this.dashboardCards = cardsResponse.cards || [];
      this.renderDashboardCards();
    } catch (error) {
      this.toast(error.message || 'Failed to update archive status', 'error');
    }
  },

  async bulkDeleteCards() {
    if (this.selectedCards.length === 0) {
      this.toast('Select cards first', 'warning');
      return;
    }

    const count = this.selectedCards.length;
    if (!confirm(`Are you sure you want to delete ${count} card${count !== 1 ? 's' : ''}? This cannot be undone.`)) {
      return;
    }

    try {
      const response = await API.cards.bulkDelete(this.selectedCards);
      const deletedCount = response.deleted_count || count;
      this.toast(`${deletedCount} card${deletedCount !== 1 ? 's' : ''} deleted`, 'success');
      // Refresh the dashboard
      this.selectedCards = [];
      const cardsResponse = await API.cards.list();
      this.dashboardCards = cardsResponse.cards || [];
      this.renderDashboardCards();
    } catch (error) {
      this.toast(error.message || 'Failed to delete cards', 'error');
    }
  },

  async exportSelectedCards() {
    // Close any open dropdowns
    document.querySelectorAll('.dropdown-menu--visible').forEach(menu => {
      menu.classList.remove('dropdown-menu--visible');
    });

    if (this.selectedCards.length === 0) {
      this.toast('Select cards first', 'warning');
      return;
    }

    // Get the selected cards from dashboardCards
    const cardsToExport = this.dashboardCards.filter(card =>
      this.selectedCards.includes(card.id)
    );

    if (cardsToExport.length === 0) {
      this.toast('No cards found to export', 'error');
      return;
    }

    try {
      const zip = new JSZip();
      const usedFilenames = new Set();

      for (const card of cardsToExport) {
        const csv = this.generateCSV(card);
        const filename = this.getUniqueFilename(card, usedFilenames);
        usedFilenames.add(filename);
        zip.file(filename, csv);
      }

      const blob = await zip.generateAsync({ type: 'blob' });
      const timestamp = new Date().toISOString().slice(0, 10);
      this.downloadBlob(blob, `yearofbingo_export_${timestamp}.zip`);

      this.toast(`Exported ${cardsToExport.length} card${cardsToExport.length > 1 ? 's' : ''}`, 'success');
    } catch (error) {
      this.toast('Error generating export: ' + error.message, 'error');
    }
  },

  // Get display name for a card (title if set, otherwise "YYYY Bingo Card")
  getCardDisplayName(card) {
    if (card.title) {
      return this.escapeHtml(card.title);
    }
    return `${card.year} Bingo Card`;
  },

  // Get category badge HTML if category is set
  getCategoryBadge(card) {
    if (!card.category) return '';
    const categoryNames = {
      personal: 'Personal Growth',
      health: 'Health & Fitness',
      food: 'Food & Dining',
      travel: 'Travel & Adventure',
      hobbies: 'Hobbies & Creativity',
      social: 'Social & Relationships',
      professional: 'Professional & Career',
      fun: 'Fun & Silly',
    };
    const name = categoryNames[card.category] || card.category;
    return `<span class="category-badge category-${this.escapeHtml(card.category)}">${this.escapeHtml(name)}</span>`;
  },

  async deleteCard(cardId) {
    // Get the card to show its name in the confirmation
    let cardName = 'this card';
    try {
      const response = await API.cards.get(cardId);
      if (response.card) {
        cardName = this.getCardDisplayName(response.card);
      }
    } catch (e) {
      // Ignore - use default name
    }

    if (!confirm(`Are you sure you want to delete "${cardName}"? This cannot be undone.`)) {
      return;
    }

    try {
      await API.cards.deleteCard(cardId);
      this.toast('Card deleted', 'success');
      this.renderDashboard(document.getElementById('main-container'));
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async renderCreate(container) {
    // If user is logged in, show the normal create form
    if (this.user) {
      await this.renderAuthenticatedCreate(container);
      return;
    }

    // For anonymous users, check if they already have an anonymous card
    if (AnonymousCard.exists()) {
      // Load and edit the existing anonymous card
      await this.renderAnonymousCardEditor(container);
      return;
    }

    // Show the create form for a new anonymous card
    const currentYear = new Date().getFullYear();
    const nextYear = currentYear + 1;

    // Fetch categories
    let categories = [];
    try {
      const response = await API.cards.getCategories();
      categories = response.categories || [];
    } catch (error) {
      categories = this.getFallbackCategories();
    }

    const categoryOptions = categories.map(c =>
      `<option value="${this.escapeHtml(c.id)}">${this.escapeHtml(c.name)}</option>`
    ).join('');

    container.innerHTML = `
      <div class="card" style="max-width: 500px; margin: 2rem auto;">
        <div class="card-header text-center">
          <h2 class="card-title">Create Your Bingo Card</h2>
          <p class="card-subtitle">Set up your bingo card - no account needed to start!</p>
        </div>
        <form id="create-card-form" onsubmit="App.handleAnonymousCreateCard(event)">
          <div class="form-group">
            <label for="card-year">Year</label>
            <select id="card-year" class="form-input" required>
              <option value="${currentYear}">${currentYear}</option>
              <option value="${nextYear}">${nextYear}</option>
            </select>
          </div>

          <div class="form-group">
            <label for="card-title">
              Title <span class="text-muted" style="font-weight: normal;">(optional)</span>
            </label>
            <input type="text" id="card-title" class="form-input"
                   placeholder="e.g., Life Goals, Foods to Try"
                   maxlength="100">
            <small class="text-muted">Leave blank for default "${currentYear} Bingo Card"</small>
          </div>

          <div class="form-group">
            <label for="card-category">
              Category <span class="text-muted" style="font-weight: normal;">(optional)</span>
            </label>
            <select id="card-category" class="form-input">
              <option value="">None</option>
              ${categoryOptions}
            </select>
          </div>

          <div style="display: flex; gap: 0.5rem; margin-top: 1rem;">
            <a href="#home" class="btn btn-ghost btn-lg" style="flex: 1; text-align: center;">Cancel</a>
            <button type="submit" class="btn btn-primary btn-lg" style="flex: 1;">Create Card</button>
          </div>
        </form>
      </div>
    `;
  },

  // Get fallback categories when API fails
  getFallbackCategories() {
    return [
      { id: 'personal', name: 'Personal Growth' },
      { id: 'health', name: 'Health & Fitness' },
      { id: 'food', name: 'Food & Dining' },
      { id: 'travel', name: 'Travel & Adventure' },
      { id: 'hobbies', name: 'Hobbies & Creativity' },
      { id: 'social', name: 'Social & Relationships' },
      { id: 'professional', name: 'Professional & Career' },
      { id: 'fun', name: 'Fun & Silly' },
    ];
  },

  // Handle anonymous card creation
  handleAnonymousCreateCard(event) {
    event.preventDefault();

    const year = parseInt(document.getElementById('card-year').value, 10);
    const title = document.getElementById('card-title').value.trim() || null;
    const category = document.getElementById('card-category').value || null;

    // Create anonymous card in localStorage
    const card = AnonymousCard.create(year, title, category);
    this.isAnonymousMode = true;
    this.currentCard = this.convertAnonymousCardToAppFormat(card);

    // Navigate to the editor
    this.renderAnonymousCardEditor(document.getElementById('main-container'));
    const cardName = title || `${year} Bingo Card`;
    this.toast(`${cardName} created! Add your goals below.`, 'success');
  },

  // Convert anonymous card format to the format used by the app
  convertAnonymousCardToAppFormat(anonCard) {
    return {
      id: 'anonymous',
      year: anonCard.year,
      title: anonCard.title,
      category: anonCard.category,
      is_finalized: false,
      items: anonCard.items.map(item => ({
        id: `anon-${item.position}`,
        position: item.position,
        content: item.text,
        notes: item.notes || '',
        is_completed: false,
      })),
    };
  },

  // Render the authenticated create form (original behavior)
  async renderAuthenticatedCreate(container) {
    const currentYear = new Date().getFullYear();
    const nextYear = currentYear + 1;

    // Fetch categories
    let categories = [];
    try {
      const response = await API.cards.getCategories();
      categories = response.categories || [];
    } catch (error) {
      categories = this.getFallbackCategories();
    }

    const categoryOptions = categories.map(c =>
      `<option value="${this.escapeHtml(c.id)}">${this.escapeHtml(c.name)}</option>`
    ).join('');

    container.innerHTML = `
      <div class="card" style="max-width: 500px; margin: 2rem auto;">
        <div class="card-header text-center">
          <h2 class="card-title">Create New Card</h2>
          <p class="card-subtitle">Set up your bingo card</p>
        </div>
        <form id="create-card-form" onsubmit="App.handleCreateCard(event)">
          <div class="form-group">
            <label for="card-year">Year</label>
            <select id="card-year" class="form-input" required>
              <option value="${currentYear}">${currentYear}</option>
              <option value="${nextYear}">${nextYear}</option>
            </select>
          </div>

          <div class="form-group">
            <label for="card-title">
              Title <span class="text-muted" style="font-weight: normal;">(optional)</span>
            </label>
            <input type="text" id="card-title" class="form-input"
                   placeholder="e.g., Life Goals, Foods to Try"
                   maxlength="100">
            <small class="text-muted">Leave blank for default "${currentYear} Bingo Card"</small>
          </div>

          <div class="form-group">
            <label for="card-category">
              Category <span class="text-muted" style="font-weight: normal;">(optional)</span>
            </label>
            <select id="card-category" class="form-input">
              <option value="">None</option>
              ${categoryOptions}
            </select>
          </div>

          <div style="display: flex; gap: 0.5rem; margin-top: 1rem;">
            <a href="#dashboard" class="btn btn-ghost btn-lg" style="flex: 1; text-align: center;">Cancel</a>
            <button type="submit" class="btn btn-primary btn-lg" style="flex: 1;">Create Card</button>
          </div>
        </form>
      </div>
    `;
  },

  async handleCreateCard(event) {
    event.preventDefault();

    const year = parseInt(document.getElementById('card-year').value, 10);
    const title = document.getElementById('card-title').value.trim() || null;
    const category = document.getElementById('card-category').value || null;

    try {
      const response = await API.cards.create(year, title, category);
      this.currentCard = response.card;
      window.location.hash = `#card/${response.card.id}`;
      const cardName = title || `${year} Bingo Card`;
      this.toast(`${cardName} created!`, 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  // Legacy method for backwards compatibility
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
    const displayName = this.getCardDisplayName(this.currentCard);
    const categoryBadge = this.getCategoryBadge(this.currentCard);
    const isAnon = this.isAnonymousMode;

    container.innerHTML = `
      ${isAnon ? `
        <div class="anonymous-card-banner">
          <div class="anonymous-card-banner-content">
            <span class="anonymous-card-banner-icon">💾</span>
            <span>This card is saved locally in your browser. <a href="#register" class="anonymous-card-banner-link">Create an account</a> to save it permanently.</span>
          </div>
        </div>
      ` : ''}

      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
        <a href="${isAnon ? '#home' : '#dashboard'}" class="btn btn-ghost">&larr; Back</a>
        <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; justify-content: center;">
          <h2 style="margin: 0;">${displayName}</h2>
          <span class="year-badge">${this.currentCard.year}</span>
          ${categoryBadge}
          <button class="btn btn-ghost btn-sm" onclick="App.${isAnon ? 'showEditAnonymousCardMetaModal' : 'showEditCardMetaModal'}()" title="Edit card name">✏️</button>
        </div>
        ${!isAnon ? `
          <button class="visibility-toggle-btn ${this.currentCard.visible_to_friends ? 'visibility-toggle-btn--visible' : 'visibility-toggle-btn--private'}" onclick="App.toggleCardVisibility('${this.currentCard.id}', ${!this.currentCard.visible_to_friends})">
            <i class="fas fa-${this.currentCard.visible_to_friends ? 'eye' : 'eye-slash'}"></i>
            <span>${this.currentCard.visible_to_friends ? 'Visible to friends' : 'Private'}</span>
          </button>
        ` : '<div></div>'}
      </div>

      <div class="progress-bar">
        <div class="progress-fill" style="width: ${progress}%"></div>
      </div>
      <p class="progress-text mb-lg">${itemCount}/24 items added</p>

      <div class="card-editor-layout">
        <div class="bingo-container editor-grid">
          <div class="bingo-grid" id="bingo-grid">
            ${this.renderGrid()}
          </div>
        </div>

        <div class="editor-sidebar">
          <div class="input-area editor-input">
            <input type="text" id="item-input" class="form-input" placeholder="Type your goal..." maxlength="500" ${itemCount >= 24 ? 'disabled' : ''}>
            <button class="btn btn-primary" id="add-btn" ${itemCount >= 24 ? 'disabled' : ''}>Add</button>
          </div>

          <div class="action-bar action-bar--side editor-actions">
            <button class="btn btn-secondary" onclick="App.shuffleCard()" ${itemCount === 0 ? 'disabled' : ''}>
              🔀 Shuffle
            </button>
            <button class="btn btn-primary" onclick="App.finalizeCard()" ${itemCount < 24 ? 'disabled' : ''}>
              ✓ Finalize Card
            </button>
          </div>

          <div class="suggestions-panel editor-suggestions">
            <div class="suggestions-header">
              <h3 class="suggestions-title">Suggestions</h3>
              <button class="btn btn-secondary btn-sm" id="fill-empty-btn" onclick="App.fillEmptySpaces()" ${itemCount >= 24 ? 'disabled' : ''}>
                ✨ Fill Empty
              </button>
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

          ${isAnon ? `
            <div class="editor-delete">
              <button class="btn btn-ghost" style="color: var(--danger);" onclick="App.confirmDeleteAnonymousCard()">
                Delete Card
              </button>
            </div>
          ` : ''}
        </div>
      </div>
    `;

    this.setupEditorEvents();
  },

  // Load and render the anonymous card editor (localStorage mode)
  async renderAnonymousCardEditor(container) {
    this.isAnonymousMode = true;

    // Load the anonymous card from localStorage
    const anonCard = AnonymousCard.get();
    if (!anonCard) {
      // No anonymous card exists, redirect to create
      window.location.hash = '#create';
      return;
    }

    // Convert to app format
    this.currentCard = this.convertAnonymousCardToAppFormat(anonCard);

    // Fetch suggestions
    try {
      const suggestionsResponse = await API.suggestions.getGrouped();
      this.suggestions = suggestionsResponse.grouped || [];
    } catch (error) {
      this.suggestions = [];
    }

    // Track used suggestions
    this.usedSuggestions = new Set(
      (this.currentCard.items || []).map(i => i.content.toLowerCase())
    );

    // Use the shared editor renderer
    this.renderCardEditor(container);
  },

  // Edit anonymous card metadata
  showEditAnonymousCardMetaModal() {
    const card = AnonymousCard.get();
    if (!card) return;

    const categories = this.getFallbackCategories();
    const currentTitle = card.title || '';
    const currentCategory = card.category || '';

    const categoryOptions = categories.map(c => {
      const selected = c.id === currentCategory ? 'selected' : '';
      return `<option value="${this.escapeHtml(c.id)}" ${selected}>${this.escapeHtml(c.name)}</option>`;
    }).join('');

    this.openModal('Edit Card', `
      <form onsubmit="App.saveAnonymousCardMeta(event)">
        <div class="form-group">
          <label for="edit-card-title">Title</label>
          <input type="text" id="edit-card-title" class="form-input"
                 value="${this.escapeHtml(currentTitle)}"
                 placeholder="e.g., Life Goals, Foods to Try"
                 maxlength="100">
          <small class="text-muted">Leave blank for default "${card.year} Bingo Card"</small>
        </div>

        <div class="form-group">
          <label for="edit-card-category">Category</label>
          <select id="edit-card-category" class="form-input">
            <option value="" ${!currentCategory ? 'selected' : ''}>None</option>
            ${categoryOptions}
          </select>
        </div>

        <div style="display: flex; gap: 1rem; justify-content: flex-end;">
          <button type="button" class="btn btn-ghost" onclick="App.closeModal()">Cancel</button>
          <button type="submit" class="btn btn-primary">Save</button>
        </div>
      </form>
    `);
  },

  saveAnonymousCardMeta(event) {
    event.preventDefault();

    const title = document.getElementById('edit-card-title').value.trim() || null;
    const category = document.getElementById('edit-card-category').value || null;

    AnonymousCard.updateMeta(title, category);
    this.currentCard.title = title;
    this.currentCard.category = category;
    this.closeModal();
    this.toast('Card updated', 'success');

    // Re-render
    this.renderAnonymousCardEditor(document.getElementById('main-container'));
  },

  confirmDeleteAnonymousCard() {
    if (confirm('Are you sure you want to delete this card? This cannot be undone.')) {
      AnonymousCard.clear();
      this.isAnonymousMode = false;
      this.currentCard = null;
      window.location.hash = '#home';
      this.toast('Card deleted', 'success');
    }
  },

  renderFinalizedCard(container) {
    const completedCount = this.currentCard.items.filter(i => i.is_completed).length;
    const progress = Math.round((completedCount / 24) * 100);
    const displayName = this.getCardDisplayName(this.currentCard);
    const categoryBadge = this.getCategoryBadge(this.currentCard);

    const visibilityIcon = this.currentCard.visible_to_friends ? 'eye' : 'eye-slash';
    const visibilityLabel = this.currentCard.visible_to_friends ? 'Visible to friends' : 'Private';

    container.innerHTML = `
      <div class="finalized-card-view">
        <div class="finalized-card-header">
          <a href="#dashboard" class="btn btn-ghost">&larr; Back</a>
          <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; justify-content: center;">
            <h2 style="margin: 0;">${displayName}</h2>
            <span class="year-badge">${this.currentCard.year}</span>
            ${categoryBadge}
          </div>
          <div class="card-header-actions">
            <button class="btn btn-ghost btn-sm" onclick="App.showEditCardMetaModal()" title="Edit card name">✏️</button>
            <button class="visibility-toggle-btn ${this.currentCard.visible_to_friends ? 'visibility-toggle-btn--visible' : 'visibility-toggle-btn--private'}" onclick="App.toggleCardVisibility('${this.currentCard.id}', ${!this.currentCard.visible_to_friends})" title="${visibilityLabel}">
              <i class="fas fa-${visibilityIcon}"></i>
              <span>${visibilityLabel}</span>
            </button>
          </div>
        </div>

        <div class="bingo-container bingo-container--finalized">
          <div class="bingo-grid bingo-grid--finalized" id="bingo-grid">
            ${this.renderGrid(true)}
          </div>
        </div>

        <div class="finalized-card-progress">
          <div class="progress-bar">
            <div class="progress-fill" style="width: ${progress}%"></div>
          </div>
          <p class="progress-text">${completedCount}/24 completed</p>
        </div>
      </div>
    `;

    this.setupFinalizedCardEvents();
  },

  renderGrid(finalized = false) {
    // B-I-N-G-O header row
    const headers = ['B', 'I', 'N', 'G', 'O'];
    const headerRow = headers.map(letter => `
      <div class="bingo-header">${letter}</div>
    `).join('');

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
          const shortText = this.truncateText(item.content, 50);
          cells.push(`
            <div class="bingo-cell ${isCompleted ? 'bingo-cell--completed' : ''}"
                 data-position="${i}"
                 data-item-id="${item.id}"
                 data-content="${this.escapeHtml(item.content)}"
                 ${!finalized ? 'draggable="true"' : ''}
                 title="${this.escapeHtml(item.content)}">
              <span class="bingo-cell-content">${this.escapeHtml(shortText)}</span>
            </div>
          `);
        } else {
          cells.push(`
            <div class="bingo-cell bingo-cell--empty" data-position="${i}"></div>
          `);
        }
      }
    }

    return headerRow + cells.join('');
  },

  truncateText(text, maxLength) {
    if (text.length <= maxLength) return text;
    // Find a good break point (space) near maxLength
    const truncated = text.substring(0, maxLength);
    const lastSpace = truncated.lastIndexOf(' ');
    if (lastSpace > maxLength * 0.5) {
      return truncated.substring(0, lastSpace) + '…';
    }
    return truncated + '…';
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
      const content = cell.dataset.content || cell.querySelector('.bingo-cell-content')?.textContent || '';
      const isCompleted = cell.classList.contains('bingo-cell--completed');

      // Show item detail modal
      this.showItemDetailModal(position, content, isCompleted);
    });
  },

  showItemDetailModal(position, content, isCompleted) {
    const item = this.currentCard.items?.find(i => i.position === position);
    const notes = item?.notes || '';

    if (isCompleted) {
      this.openModal('Goal Completed!', `
        <div class="item-detail">
          <p class="item-detail-content">${this.escapeHtml(content)}</p>
          ${notes ? `<p class="item-detail-notes"><strong>Notes:</strong> ${this.escapeHtml(notes)}</p>` : ''}
        </div>
        <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-secondary" style="flex: 1;" onclick="App.closeModal()">
            Close
          </button>
          <button type="button" class="btn btn-ghost" style="flex: 1;" onclick="App.uncompleteItem(${position})">
            Mark Incomplete
          </button>
        </div>
      `);
    } else {
      this.openModal('Mark Complete', `
        <div class="item-detail">
          <p class="item-detail-content">${this.escapeHtml(content)}</p>
        </div>
        <form id="complete-form">
          <div class="form-group" style="margin-top: 1rem;">
            <label class="form-label">Notes (optional)</label>
            <textarea id="complete-notes" class="form-input" rows="3" placeholder="How did you accomplish this?"></textarea>
          </div>
          <div style="display: flex; gap: 1rem;">
            <button type="button" class="btn btn-secondary" style="flex: 1;" onclick="App.closeModal()">
              Cancel
            </button>
            <button type="submit" class="btn btn-primary" style="flex: 1;">
              Mark Complete
            </button>
          </div>
        </form>
      `);

      document.getElementById('complete-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const notes = document.getElementById('complete-notes').value;
        await this.completeItem(position, notes);
      });
    }
  },

  async uncompleteItem(position) {
    try {
      await API.cards.uncompleteItem(this.currentCard.id, position);
      const cell = document.querySelector(`[data-position="${position}"]`);
      cell.classList.remove('bingo-cell--completed');
      this.closeModal();
      this.toast('Item marked incomplete', 'success');

      // Update progress
      const completedCount = document.querySelectorAll('.bingo-cell--completed').length;
      const progress = Math.round((completedCount / 24) * 100);
      document.querySelector('.progress-fill').style.width = `${progress}%`;
      document.querySelector('.progress-text').textContent = `${completedCount}/24 completed`;

      // Update local state
      const item = this.currentCard.items?.find(i => i.position === position);
      if (item) item.is_completed = false;
    } catch (error) {
      this.toast(error.message, 'error');
    }
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

      // Update local state
      const item = this.currentCard.items?.find(i => i.position === position);
      if (item) {
        item.is_completed = true;
        item.notes = notes || '';
      }

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
    const content = cell.dataset.content || cell.querySelector('.bingo-cell-content').textContent;

    this.openModal('Edit Item', `
      <div class="item-detail">
        <p class="item-detail-content">${this.escapeHtml(content)}</p>
      </div>
      <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
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
      let position;

      if (this.isAnonymousMode) {
        // Add to localStorage
        const item = AnonymousCard.addItem(content);
        if (!item) {
          this.toast('Card is full', 'error');
          return;
        }
        position = item.position;

        // Update local state
        if (!this.currentCard.items) this.currentCard.items = [];
        this.currentCard.items.push({
          id: `anon-${position}`,
          position: position,
          content: content,
          notes: '',
          is_completed: false,
        });
      } else {
        // Add to server
        const response = await API.cards.addItem(this.currentCard.id, content);
        position = response.item.position;

        // Update local state
        if (!this.currentCard.items) this.currentCard.items = [];
        this.currentCard.items.push(response.item);
      }

      input.value = '';
      this.usedSuggestions.add(content.toLowerCase());

      // Update grid with animation
      const cell = document.querySelector(`[data-position="${position}"]`);
      cell.classList.remove('bingo-cell--empty');
      cell.classList.add('bingo-cell--appearing');
      cell.dataset.itemId = this.isAnonymousMode ? `anon-${position}` : this.currentCard.items[this.currentCard.items.length - 1].id;
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
        document.getElementById('fill-empty-btn').disabled = true;
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

  async fillEmptySpaces() {
    const currentItemCount = this.currentCard.items ? this.currentCard.items.length : 0;
    const emptyCount = 24 - currentItemCount;

    if (emptyCount === 0) {
      this.toast('Card is already full', 'info');
      return;
    }

    // Get all unused suggestions from all categories
    const allUnusedSuggestions = [];
    for (const category of this.suggestions) {
      for (const suggestion of category.suggestions) {
        if (!this.usedSuggestions.has(suggestion.content.toLowerCase())) {
          allUnusedSuggestions.push(suggestion.content);
        }
      }
    }

    if (allUnusedSuggestions.length === 0) {
      this.toast('No more suggestions available', 'error');
      return;
    }

    // Shuffle and pick the number we need
    const shuffled = allUnusedSuggestions.sort(() => Math.random() - 0.5);
    const toAdd = shuffled.slice(0, Math.min(emptyCount, shuffled.length));

    if (toAdd.length < emptyCount) {
      this.toast(`Only ${toAdd.length} suggestions available, adding those`, 'info');
    }

    // Add items one by one
    let added = 0;
    for (const content of toAdd) {
      try {
        let position;

        if (this.isAnonymousMode) {
          // Add to localStorage
          const item = AnonymousCard.addItem(content);
          if (!item) {
            break; // Card is full
          }
          position = item.position;

          // Update local state
          if (!this.currentCard.items) this.currentCard.items = [];
          this.currentCard.items.push({
            position: item.position,
            content: content,
            is_completed: false,
          });
        } else {
          // Add to server
          const response = await API.cards.addItem(this.currentCard.id, content);
          position = response.item.position;

          // Update local state
          if (!this.currentCard.items) this.currentCard.items = [];
          this.currentCard.items.push(response.item);
        }

        this.usedSuggestions.add(content.toLowerCase());

        // Update grid with animation
        const cell = document.querySelector(`[data-position="${position}"]`);
        cell.classList.remove('bingo-cell--empty');
        cell.classList.add('bingo-cell--appearing');
        cell.dataset.itemId = this.isAnonymousMode ? `anon-${position}` : this.currentCard.items[this.currentCard.items.length - 1].id;
        cell.dataset.content = this.escapeHtml(content);
        cell.draggable = true;
        cell.title = content;
        cell.innerHTML = `<span class="bingo-cell-content">${this.escapeHtml(this.truncateText(content, 50))}</span>`;

        added++;
      } catch (error) {
        console.error('Failed to add item:', error);
        break;
      }
    }

    // Update progress
    const itemCount = this.currentCard.items.length;
    const progress = Math.round((itemCount / 24) * 100);
    document.querySelector('.progress-fill').style.width = `${progress}%`;
    document.querySelector('.progress-text').textContent = `${itemCount}/24 items added`;

    // Update buttons
    const isFull = itemCount >= 24;
    document.getElementById('item-input').disabled = isFull;
    document.getElementById('add-btn').disabled = isFull;
    document.getElementById('fill-empty-btn').disabled = isFull;
    document.querySelector('[onclick="App.shuffleCard()"]').disabled = itemCount === 0;
    document.querySelector('[onclick="App.finalizeCard()"]').disabled = itemCount < 24;

    // Update suggestions panel
    const activeTab = document.querySelector('.category-tab--active');
    if (activeTab) {
      document.getElementById('suggestions-list').innerHTML = this.renderSuggestions(activeTab.dataset.category);
    }

    this.toast(`Added ${added} item${added !== 1 ? 's' : ''} to your card`, 'success');
  },

  async removeItem(position) {
    try {
      const item = this.currentCard.items.find(i => i.position === position);

      if (this.isAnonymousMode) {
        // Remove from localStorage
        AnonymousCard.removeItem(position);
      } else {
        // Remove from server
        await API.cards.removeItem(this.currentCard.id, position);
      }

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
      document.getElementById('fill-empty-btn').disabled = false;
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

      if (this.isAnonymousMode) {
        // Shuffle in localStorage
        const shuffledCard = AnonymousCard.shuffle();
        this.currentCard = this.convertAnonymousCardToAppFormat(shuffledCard);
      } else {
        // Shuffle on server
        const response = await API.cards.shuffle(this.currentCard.id);
        this.currentCard = response.card;
      }

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

  async toggleCardVisibility(cardId, visibleToFriends) {
    try {
      const response = await API.cards.updateVisibility(cardId, visibleToFriends);
      this.currentCard = response.card;
      this.toast(visibleToFriends ? 'Card is now visible to friends' : 'Card is now private', 'success');
      // Re-render to update the UI
      this.route();
    } catch (error) {
      this.toast(error.message || 'Failed to update visibility', 'error');
    }
  },

  async finalizeCard() {
    // For anonymous users, show the auth modal instead of finalizing directly
    if (this.isAnonymousMode) {
      this.showFinalizeAuthModal();
      return;
    }

    this.showFinalizeConfirmModal();
  },

  showFinalizeConfirmModal() {
    this.openModal('Finalize Card', `
      <div class="finalize-confirm-modal">
        <p style="margin-bottom: 1.5rem;">
          Are you sure you want to finalize this card? You won't be able to change the items after this.
        </p>
        <div style="margin-bottom: 1.5rem;">
          <label class="checkbox-label" style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
            <input type="checkbox" id="finalize-visibility" checked>
            <span>Visible to friends</span>
          </label>
          <p class="text-muted" style="margin-top: 0.5rem; font-size: 0.875rem;">
            If unchecked, friends won't be able to see this card.
          </p>
        </div>
        <div style="display: flex; gap: 1rem; justify-content: flex-end;">
          <button class="btn btn-ghost" onclick="App.closeModal()">Cancel</button>
          <button class="btn btn-primary" onclick="App.confirmFinalize()">Finalize Card</button>
        </div>
      </div>
    `);
  },

  async confirmFinalize() {
    const visibilityCheckbox = document.getElementById('finalize-visibility');
    const visibleToFriends = visibilityCheckbox ? visibilityCheckbox.checked : true;

    try {
      this.closeModal();
      const response = await API.cards.finalize(this.currentCard.id, visibleToFriends);
      this.currentCard = response.card;
      this.renderFinalizedCard(document.getElementById('main-container'));
      this.toast('Card finalized! Good luck with your goals! 🎉', 'success');
      this.confetti(50);
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  // Show the auth modal when an anonymous user tries to finalize
  showFinalizeAuthModal() {
    this.openModal('Save Your Card', `
      <div class="finalize-auth-modal">
        <p style="margin-bottom: 1.5rem;">
          Your bingo card is ready! Create an account to save and finalize it.
        </p>
        <div style="display: flex; flex-direction: column; gap: 1rem;">
          <button class="btn btn-primary btn-lg" onclick="App.showFinalizeRegisterForm()">
            Create Account
          </button>
          <button class="btn btn-secondary btn-lg" onclick="App.showFinalizeLoginForm()">
            I Already Have an Account
          </button>
          <button class="btn btn-ghost" onclick="App.closeModal()">
            Cancel
          </button>
        </div>
      </div>
    `);
  },

  // Show inline registration form in the finalize modal
  showFinalizeRegisterForm() {
    const modalBody = document.getElementById('modal-body');
    modalBody.innerHTML = `
      <form id="finalize-register-form" onsubmit="App.handleFinalizeRegister(event)">
        <div class="form-group">
          <label class="form-label" for="finalize-username">Username</label>
          <input type="text" id="finalize-username" class="form-input" required minlength="2" maxlength="100">
        </div>
        <div class="form-group">
          <label class="form-label" for="finalize-email">Email</label>
          <input type="email" id="finalize-email" class="form-input" required autocomplete="email">
        </div>
        <div class="form-group">
          <label class="form-label" for="finalize-password">Password</label>
          <input type="password" id="finalize-password" class="form-input" required minlength="8" autocomplete="new-password">
          <small class="text-muted">At least 8 characters with uppercase, lowercase, and number</small>
        </div>
        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" id="finalize-searchable">
            <span>Allow others to find me by username</span>
          </label>
          <small class="text-muted">You can change this later in your account settings</small>
        </div>
        <div id="finalize-register-error" class="form-error hidden"></div>
        <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-ghost" style="flex: 1;" onclick="App.showFinalizeAuthModal()">
            Back
          </button>
          <button type="submit" class="btn btn-primary" style="flex: 1;">
            Create Account & Save Card
          </button>
        </div>
      </form>
    `;
  },

  // Handle registration from the finalize modal
  async handleFinalizeRegister(event) {
    event.preventDefault();

    const username = document.getElementById('finalize-username').value;
    const email = document.getElementById('finalize-email').value;
    const password = document.getElementById('finalize-password').value;
    const searchable = document.getElementById('finalize-searchable').checked;
    const errorEl = document.getElementById('finalize-register-error');

    try {
      // Register the user
      const response = await API.auth.register(email, password, username, searchable);
      this.user = response.user;
      this.setupNavigation();

      // Import the anonymous card
      await this.importAnonymousCard();
    } catch (error) {
      errorEl.textContent = error.message;
      errorEl.classList.remove('hidden');
    }
  },

  // Show inline login form in the finalize modal
  showFinalizeLoginForm() {
    const modalBody = document.getElementById('modal-body');
    modalBody.innerHTML = `
      <form id="finalize-login-form" onsubmit="App.handleFinalizeLogin(event)">
        <div class="form-group">
          <label class="form-label" for="finalize-login-email">Email</label>
          <input type="email" id="finalize-login-email" class="form-input" required autocomplete="email">
        </div>
        <div class="form-group">
          <label class="form-label" for="finalize-login-password">Password</label>
          <input type="password" id="finalize-login-password" class="form-input" required autocomplete="current-password">
        </div>
        <div id="finalize-login-error" class="form-error hidden"></div>
        <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-ghost" style="flex: 1;" onclick="App.showFinalizeAuthModal()">
            Back
          </button>
          <button type="submit" class="btn btn-primary" style="flex: 1;">
            Login & Save Card
          </button>
        </div>
      </form>
    `;
  },

  // Handle login from the finalize modal
  async handleFinalizeLogin(event) {
    event.preventDefault();

    const email = document.getElementById('finalize-login-email').value;
    const password = document.getElementById('finalize-login-password').value;
    const errorEl = document.getElementById('finalize-login-error');

    try {
      // Login the user
      const response = await API.auth.login(email, password);
      this.user = response.user;
      this.setupNavigation();

      // Import the anonymous card (with conflict detection)
      await this.importAnonymousCard();
    } catch (error) {
      errorEl.textContent = error.message;
      errorEl.classList.remove('hidden');
    }
  },

  // Import the anonymous card to the server
  async importAnonymousCard() {
    const anonCard = AnonymousCard.get();
    if (!anonCard) {
      this.toast('No card to import', 'error');
      return;
    }

    try {
      const importData = AnonymousCard.toAPIFormat();
      const response = await API.cards.import(importData);

      if (response.error === 'card_exists') {
        // Handle conflict
        this.showCardConflictModal(response.existing_card, anonCard);
        return;
      }

      // Success - clear anonymous card and show finalized card
      AnonymousCard.clear();
      this.isAnonymousMode = false;
      this.currentCard = response.card;
      this.closeModal();
      this.renderFinalizedCard(document.getElementById('main-container'));
      this.toast('Card saved and finalized! Good luck with your goals! 🎉', 'success');
      this.confetti(50);
    } catch (error) {
      this.toast(error.message || 'Failed to import card', 'error');
    }
  },

  // Show the conflict resolution modal
  showCardConflictModal(existingCard, anonymousCard) {
    const existingTitle = existingCard.title || `${existingCard.year} Bingo Card`;
    const itemCount = existingCard.item_count || (existingCard.items ? existingCard.items.length : 0);
    const isFinalized = existingCard.is_finalized ? 'finalized' : 'in progress';

    this.openModal('Card Already Exists', `
      <div class="conflict-modal">
        <p style="margin-bottom: 1rem;">
          You already have a <strong>${existingCard.year}</strong> card:
        </p>
        <div class="card" style="margin-bottom: 1.5rem; padding: 1rem;">
          <strong>${this.escapeHtml(existingTitle)}</strong>
          <p class="text-muted" style="margin: 0.25rem 0 0 0;">
            ${itemCount} items, ${isFinalized}
          </p>
        </div>
        <p style="margin-bottom: 1.5rem;">What would you like to do?</p>
        <div style="display: flex; flex-direction: column; gap: 0.75rem;">
          <button class="btn btn-secondary" onclick="App.handleConflictKeepExisting('${existingCard.id}')">
            Keep Existing Card
          </button>
          <button class="btn btn-primary" onclick="App.handleConflictSaveAsNew()">
            Save as New Card (with different title)
          </button>
          <button class="btn btn-ghost" style="color: var(--danger);" onclick="App.handleConflictReplace('${existingCard.id}')">
            Replace Existing Card
          </button>
          <button class="btn btn-ghost" onclick="App.closeModal()">
            Cancel
          </button>
        </div>
      </div>
    `);
  },

  // Handle conflict: keep existing card
  handleConflictKeepExisting(existingCardId) {
    AnonymousCard.clear();
    this.isAnonymousMode = false;
    this.closeModal();
    window.location.hash = `#card/${existingCardId}`;
    this.toast('Keeping your existing card. Anonymous card discarded.', 'success');
  },

  // Handle conflict: save with new title
  async handleConflictSaveAsNew() {
    const anonCard = AnonymousCard.get();
    const currentTitle = anonCard.title || `${anonCard.year} Bingo Card`;

    this.openModal('Save with New Title', `
      <form id="conflict-new-title-form" onsubmit="App.handleConflictSaveAsNewSubmit(event)">
        <div class="form-group">
          <label class="form-label" for="conflict-new-title">New Title</label>
          <input type="text" id="conflict-new-title" class="form-input" required
                 value="${this.escapeHtml(currentTitle)} (2)"
                 maxlength="100">
          <small class="text-muted">Choose a different title for your new card</small>
        </div>
        <div id="conflict-new-title-error" class="form-error hidden"></div>
        <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-ghost" style="flex: 1;" onclick="App.importAnonymousCard()">
            Back
          </button>
          <button type="submit" class="btn btn-primary" style="flex: 1;">
            Save Card
          </button>
        </div>
      </form>
    `);
  },

  async handleConflictSaveAsNewSubmit(event) {
    event.preventDefault();

    const newTitle = document.getElementById('conflict-new-title').value.trim();
    const errorEl = document.getElementById('conflict-new-title-error');

    if (!newTitle) {
      errorEl.textContent = 'Please enter a title';
      errorEl.classList.remove('hidden');
      return;
    }

    try {
      // Update the anonymous card with new title
      AnonymousCard.updateMeta(newTitle, AnonymousCard.get().category);

      // Try importing again
      const importData = AnonymousCard.toAPIFormat();
      const response = await API.cards.import(importData);

      if (response.error === 'card_exists') {
        errorEl.textContent = 'A card with this title already exists. Please choose a different title.';
        errorEl.classList.remove('hidden');
        return;
      }

      // Success
      AnonymousCard.clear();
      this.isAnonymousMode = false;
      this.currentCard = response.card;
      this.closeModal();
      this.renderFinalizedCard(document.getElementById('main-container'));
      this.toast('Card saved and finalized! Good luck with your goals! 🎉', 'success');
      this.confetti(50);
    } catch (error) {
      errorEl.textContent = error.message;
      errorEl.classList.remove('hidden');
    }
  },

  // Handle conflict: replace existing card
  async handleConflictReplace(existingCardId) {
    if (!confirm('Are you sure you want to replace your existing card? This cannot be undone.')) {
      return;
    }

    try {
      // Delete the existing card
      await API.cards.deleteCard(existingCardId);

      // Import the anonymous card
      const importData = AnonymousCard.toAPIFormat();
      const response = await API.cards.import(importData);

      // Success
      AnonymousCard.clear();
      this.isAnonymousMode = false;
      this.currentCard = response.card;
      this.closeModal();
      this.renderFinalizedCard(document.getElementById('main-container'));
      this.toast('Card replaced and finalized! Good luck with your goals! 🎉', 'success');
      this.confetti(50);
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  // Show conflict resolution modal for card creation (not anonymous import)
  showCreateCardConflictModal(existingCard, year, category) {
    const existingTitle = existingCard.title || `${existingCard.year} Bingo Card`;
    const itemCount = existingCard.item_count || 0;
    const isFinalized = existingCard.is_finalized ? 'finalized' : 'in progress';

    // Store context for use in handlers
    this.createConflictContext = { year, category };

    let buttons = `
      <button class="btn btn-secondary" onclick="App.handleCreateConflictGoToExisting('${existingCard.id}')">
        Go to Existing Card
      </button>
      <button class="btn btn-primary" onclick="App.handleCreateConflictSaveAsNew()">
        Create with Different Title
      </button>`;

    // Only offer replace for unfinalized cards
    if (!existingCard.is_finalized) {
      buttons += `
        <button class="btn btn-ghost" style="color: var(--danger);" onclick="App.handleCreateConflictReplace('${existingCard.id}')">
          Delete &amp; Create New
        </button>`;
    }

    buttons += `
      <button class="btn btn-ghost" onclick="App.closeModal()">
        Cancel
      </button>`;

    this.openModal('Card Already Exists', `
      <div class="conflict-modal">
        <p style="margin-bottom: 1rem;">
          You already have a <strong>${existingCard.year}</strong> card:
        </p>
        <div class="card" style="margin-bottom: 1.5rem; padding: 1rem;">
          <strong>${this.escapeHtml(existingTitle)}</strong>
          <p class="text-muted" style="margin: 0.25rem 0 0 0;">
            ${itemCount} items, ${isFinalized}
          </p>
        </div>
        <p style="margin-bottom: 1.5rem;">What would you like to do?</p>
        <div style="display: flex; flex-direction: column; gap: 0.75rem;">
          ${buttons}
        </div>
      </div>
    `);
  },

  // Handle create conflict: go to existing card
  handleCreateConflictGoToExisting(existingCardId) {
    this.closeModal();
    window.location.hash = `#card/${existingCardId}`;
  },

  // Handle create conflict: create with new title
  handleCreateConflictSaveAsNew() {
    const ctx = this.createConflictContext;
    const suggestedTitle = `${ctx.year} Bingo Card (2)`;

    this.openModal('Create with New Title', `
      <form id="create-conflict-title-form" onsubmit="App.handleCreateConflictSaveAsNewSubmit(event)">
        <div class="form-group">
          <label class="form-label" for="create-conflict-title">Card Title</label>
          <input type="text" id="create-conflict-title" class="form-input" required
                 value="${this.escapeHtml(suggestedTitle)}"
                 maxlength="100">
          <small class="text-muted">Choose a unique title for your new card</small>
        </div>
        <div id="create-conflict-error" class="form-error hidden"></div>
        <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-ghost" style="flex: 1;" onclick="App.closeModal()">
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" style="flex: 1;">
            Create Card
          </button>
        </div>
      </form>
    `);
  },

  async handleCreateConflictSaveAsNewSubmit(event) {
    event.preventDefault();

    const newTitle = document.getElementById('create-conflict-title').value.trim();
    const errorEl = document.getElementById('create-conflict-error');
    const ctx = this.createConflictContext;

    if (!newTitle) {
      errorEl.textContent = 'Please enter a title';
      errorEl.classList.remove('hidden');
      return;
    }

    try {
      const response = await API.cards.create(ctx.year, newTitle, ctx.category);

      if (response.error === 'card_exists') {
        errorEl.textContent = 'A card with this title already exists. Please choose a different title.';
        errorEl.classList.remove('hidden');
        return;
      }

      this.currentCard = response.card;
      this.closeModal();
      window.location.hash = `#card/${response.card.id}`;
      this.toast(`${newTitle} created!`, 'success');
    } catch (error) {
      errorEl.textContent = error.message;
      errorEl.classList.remove('hidden');
    }
  },

  // Handle create conflict: delete existing and create new
  async handleCreateConflictReplace(existingCardId) {
    if (!confirm('Are you sure you want to delete your existing card? This cannot be undone.')) {
      return;
    }

    const ctx = this.createConflictContext;

    try {
      // Delete the existing card
      await API.cards.deleteCard(existingCardId);

      // Create the new card
      const response = await API.cards.create(ctx.year, null, ctx.category);

      this.currentCard = response.card;
      this.closeModal();
      window.location.hash = `#card/${response.card.id}`;
      const cardName = `${ctx.year} Bingo Card`;
      this.toast(`${cardName} created!`, 'success');
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

  // Friends page
  async renderFriends(container) {
    container.innerHTML = `
      <div class="friends-page">
        <div class="friends-header">
          <h2>Friends</h2>
        </div>

        <div class="friends-search card">
          <h3>Find Friends</h3>
          <div class="search-input-group">
            <input type="text" id="friend-search" class="form-input" placeholder="Search by name or email...">
            <button class="btn btn-primary" id="search-btn">Search</button>
          </div>
          <div id="search-results" class="search-results"></div>
        </div>

        <div id="friend-requests" class="card" style="display: none;">
          <h3>Friend Requests</h3>
          <div id="requests-list"></div>
        </div>

        <div id="sent-requests" class="card" style="display: none;">
          <h3>Sent Requests</h3>
          <div id="sent-list"></div>
        </div>

        <div class="card">
          <h3>My Friends</h3>
          <div id="friends-list">
            <div class="text-center"><div class="spinner"></div></div>
          </div>
        </div>
      </div>
    `;

    this.setupFriendsEvents();
    await this.loadFriends();
  },

  setupFriendsEvents() {
    const searchInput = document.getElementById('friend-search');
    const searchBtn = document.getElementById('search-btn');

    searchBtn.addEventListener('click', () => this.searchFriends());
    searchInput.addEventListener('keypress', (e) => {
      if (e.key === 'Enter') this.searchFriends();
    });

    let debounceTimer;
    searchInput.addEventListener('input', () => {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => this.searchFriends(), 300);
    });
  },

  async searchFriends() {
    const query = document.getElementById('friend-search').value.trim();
    const resultsEl = document.getElementById('search-results');

    if (query.length < 2) {
      resultsEl.innerHTML = '';
      return;
    }

    try {
      const response = await API.friends.search(query);
      const users = response.users || [];

      if (users.length === 0) {
        resultsEl.innerHTML = '<p class="text-muted">No users found</p>';
      } else {
        resultsEl.innerHTML = users.map(user => `
          <div class="search-result-item">
            <div>
              <strong>${this.escapeHtml(user.username)}</strong>
            </div>
            <button class="btn btn-primary btn-sm" onclick="App.sendFriendRequest('${user.id}')">
              Add Friend
            </button>
          </div>
        `).join('');
      }
    } catch (error) {
      resultsEl.innerHTML = `<p class="text-muted">${error.message}</p>`;
    }
  },

  async sendFriendRequest(friendId) {
    try {
      await API.friends.sendRequest(friendId);
      this.toast('Friend request sent!', 'success');
      document.getElementById('friend-search').value = '';
      document.getElementById('search-results').innerHTML = '';
      await this.loadFriends();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async loadFriends() {
    try {
      const response = await API.friends.list();
      const { friends, requests, sent } = response;

      // Pending requests (received)
      const requestsEl = document.getElementById('friend-requests');
      const requestsListEl = document.getElementById('requests-list');
      if (requests && requests.length > 0) {
        requestsEl.style.display = 'block';
        requestsListEl.innerHTML = requests.map(req => `
          <div class="friend-item">
            <div>
              <strong>${this.escapeHtml(req.requester_username)}</strong>
              <span class="text-muted">${this.escapeHtml(req.requester_email)}</span>
            </div>
            <div class="friend-actions">
              <button class="btn btn-primary btn-sm" onclick="App.acceptRequest('${req.id}')">Accept</button>
              <button class="btn btn-ghost btn-sm" onclick="App.rejectRequest('${req.id}')">Reject</button>
            </div>
          </div>
        `).join('');
      } else {
        requestsEl.style.display = 'none';
      }

      // Sent requests
      const sentEl = document.getElementById('sent-requests');
      const sentListEl = document.getElementById('sent-list');
      if (sent && sent.length > 0) {
        sentEl.style.display = 'block';
        sentListEl.innerHTML = sent.map(req => `
          <div class="friend-item">
            <div>
              <strong>${this.escapeHtml(req.friend_username)}</strong>
              <span class="text-muted">${this.escapeHtml(req.friend_email)}</span>
            </div>
            <button class="btn btn-ghost btn-sm" onclick="App.cancelRequest('${req.id}')">Cancel</button>
          </div>
        `).join('');
      } else {
        sentEl.style.display = 'none';
      }

      // Friends list
      const friendsListEl = document.getElementById('friends-list');
      if (friends && friends.length > 0) {
        friendsListEl.innerHTML = friends.map(friend => `
          <div class="friend-item">
            <div>
              <strong>${this.escapeHtml(friend.friend_username)}</strong>
            </div>
            <div class="friend-actions">
              <a href="#friend-card/${friend.id}" class="btn btn-secondary btn-sm">View Card</a>
              <button class="btn btn-ghost btn-sm" onclick="App.removeFriend('${friend.id}', '${this.escapeHtml(friend.friend_username)}')">Remove</button>
            </div>
          </div>
        `).join('');
      } else {
        friendsListEl.innerHTML = '<p class="text-muted">No friends yet. Search for people to add!</p>';
      }
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async acceptRequest(friendshipId) {
    try {
      await API.friends.acceptRequest(friendshipId);
      this.toast('Friend request accepted!', 'success');
      await this.loadFriends();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async rejectRequest(friendshipId) {
    try {
      await API.friends.rejectRequest(friendshipId);
      this.toast('Friend request rejected', 'success');
      await this.loadFriends();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async cancelRequest(friendshipId) {
    try {
      await API.friends.cancelRequest(friendshipId);
      this.toast('Friend request canceled', 'success');
      await this.loadFriends();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async removeFriend(friendshipId, friendName) {
    if (!confirm(`Are you sure you want to remove ${friendName} as a friend?`)) {
      return;
    }
    try {
      await API.friends.remove(friendshipId);
      this.toast('Friend removed', 'success');
      await this.loadFriends();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  // Friend's card view (read-only with reactions)
  async renderFriendCard(container, friendshipId, selectedYear = null) {
    container.innerHTML = `
      <div class="text-center"><div class="spinner" style="margin: 2rem auto;"></div></div>
    `;

    try {
      const response = await API.friends.getCards(friendshipId);

      if (!response.cards || response.cards.length === 0) {
        container.innerHTML = `
          <div class="card text-center" style="padding: 3rem;">
            <h3>No Cards Available</h3>
            <p class="text-muted mb-lg">This friend has no finalized cards yet.</p>
            <a href="#friends" class="btn btn-primary">Back to Friends</a>
          </div>
        `;
        return;
      }

      this.friendCards = response.cards;
      this.friendCardOwner = response.owner;
      this.friendshipId = friendshipId;

      // Sort by year descending
      this.friendCards.sort((a, b) => b.year - a.year);

      // Select the requested year or default to most recent
      if (selectedYear) {
        this.currentCard = this.friendCards.find(c => c.year === parseInt(selectedYear)) || this.friendCards[0];
      } else {
        this.currentCard = this.friendCards[0];
      }

      this.renderFriendCardView(container);
    } catch (error) {
      container.innerHTML = `
        <div class="card text-center" style="padding: 3rem;">
          <h3>Error</h3>
          <p class="text-muted mb-lg">${error.message}</p>
          <a href="#friends" class="btn btn-primary">Back to Friends</a>
        </div>
      `;
    }
  },

  renderFriendCardView(container) {
    const completedCount = this.currentCard.items.filter(i => i.is_completed).length;
    const progress = Math.round((completedCount / 24) * 100);
    const currentYear = new Date().getFullYear();
    const isArchived = this.currentCard.year < currentYear;
    const displayName = this.getCardDisplayName(this.currentCard);
    const categoryBadge = this.getCategoryBadge(this.currentCard);

    // Build card selector if multiple cards
    let cardSelector = '';
    if (this.friendCards && this.friendCards.length > 1) {
      const cardOptions = this.friendCards.map(card => {
        const selected = card.id === this.currentCard.id ? 'selected' : '';
        const archived = card.year < currentYear ? ' (archived)' : '';
        const cardName = this.getCardDisplayName(card);
        return `<option value="${card.id}" ${selected}>${cardName} (${card.year})${archived}</option>`;
      }).join('');
      cardSelector = `
        <select id="friend-card-select" class="year-selector" onchange="App.switchFriendCard(this.value)">
          ${cardOptions}
        </select>
      `;
    }

    container.innerHTML = `
      <div class="finalized-card-view">
        <div class="finalized-card-header">
          <a href="#friends" class="btn btn-ghost">&larr; Friends</a>
          <div class="friend-card-title">
            <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; justify-content: center;">
              <h2 style="margin: 0;">${this.escapeHtml(this.friendCardOwner?.username || 'Friend')}'s ${displayName}</h2>
              <span class="year-badge">${this.currentCard.year}</span>
              ${categoryBadge}
              ${isArchived ? '<span class="archive-badge">Archived</span>' : ''}
            </div>
          </div>
          ${cardSelector || '<div></div>'}
        </div>

        <div class="bingo-container bingo-container--finalized">
          <div class="bingo-grid bingo-grid--finalized ${isArchived ? 'bingo-grid--archive' : ''}" id="bingo-grid">
            ${this.renderFriendGrid()}
          </div>
        </div>

        <div class="finalized-card-progress">
          <div class="progress-bar">
            <div class="progress-fill" style="width: ${progress}%"></div>
          </div>
          <p class="progress-text">${completedCount}/24 completed</p>
        </div>
      </div>
    `;

    this.setupFriendCardEvents();
  },

  // Switch friend card by ID (supports multiple cards per year)
  switchFriendCard(cardId) {
    const card = this.friendCards.find(c => c.id === cardId);
    if (card) {
      this.currentCard = card;
      const container = document.getElementById('main-container');
      this.renderFriendCardView(container);
    }
  },

  // Legacy method for backwards compatibility
  switchFriendYear(year) {
    const card = this.friendCards.find(c => c.year === parseInt(year));
    if (card) {
      this.currentCard = card;
      const container = document.getElementById('main-container');
      this.renderFriendCardView(container);
    }
  },

  renderFriendGrid() {
    const headers = ['B', 'I', 'N', 'G', 'O'];
    const headerRow = headers.map(letter => `
      <div class="bingo-header">${letter}</div>
    `).join('');

    const cells = [];
    const itemsByPosition = {};

    if (this.currentCard.items) {
      this.currentCard.items.forEach(item => {
        itemsByPosition[item.position] = item;
      });
    }

    for (let i = 0; i < 25; i++) {
      if (i === 12) {
        cells.push(`
          <div class="bingo-cell bingo-cell--free">
            <span class="bingo-cell-content">FREE</span>
          </div>
        `);
      } else {
        const item = itemsByPosition[i];
        if (item) {
          const isCompleted = item.is_completed;
          const shortText = this.truncateText(item.content, 50);
          cells.push(`
            <div class="bingo-cell ${isCompleted ? 'bingo-cell--completed' : ''}"
                 data-position="${i}"
                 data-item-id="${item.id}"
                 data-content="${this.escapeHtml(item.content)}"
                 title="${this.escapeHtml(item.content)}">
              <span class="bingo-cell-content">${this.escapeHtml(shortText)}</span>
            </div>
          `);
        } else {
          cells.push(`
            <div class="bingo-cell bingo-cell--empty" data-position="${i}"></div>
          `);
        }
      }
    }

    return headerRow + cells.join('');
  },

  setupFriendCardEvents() {
    document.getElementById('bingo-grid').addEventListener('click', async (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (!cell || cell.classList.contains('bingo-cell--free') || cell.classList.contains('bingo-cell--empty')) return;

      const itemId = cell.dataset.itemId;
      const content = cell.dataset.content;
      const isCompleted = cell.classList.contains('bingo-cell--completed');

      this.showFriendItemModal(itemId, content, isCompleted);
    });
  },

  async showFriendItemModal(itemId, content, isCompleted) {
    const item = this.currentCard.items?.find(i => i.id === itemId);
    const notes = item?.notes || '';

    let reactionsHtml = '';
    let userReaction = null;

    if (isCompleted) {
      try {
        const response = await API.reactions.get(itemId);
        const reactions = response.reactions || [];
        const summary = response.summary || [];

        userReaction = reactions.find(r => r.user_id === this.user.id);

        if (summary.length > 0) {
          reactionsHtml = `
            <div class="reactions-summary">
              ${summary.map(s => `<span class="reaction-badge">${s.emoji} ${s.count}</span>`).join('')}
            </div>
          `;
        }
      } catch (error) {
        console.error('Failed to load reactions:', error);
      }
    }

    const emojiPickerHtml = isCompleted ? `
      <div class="reaction-picker">
        <p>React to this achievement:</p>
        <div class="emoji-buttons">
          ${this.allowedEmojis.map(emoji => `
            <button class="emoji-btn ${userReaction?.emoji === emoji ? 'emoji-btn--selected' : ''}"
                    onclick="App.reactToItem('${itemId}', '${emoji}')">${emoji}</button>
          `).join('')}
          ${userReaction ? `<button class="emoji-btn emoji-btn--remove" onclick="App.removeReaction('${itemId}')">✕</button>` : ''}
        </div>
      </div>
    ` : '';

    this.openModal(isCompleted ? 'Completed Goal' : 'Goal', `
      <div class="item-detail">
        <p class="item-detail-content">${this.escapeHtml(content)}</p>
        ${notes && isCompleted ? `<p class="item-detail-notes"><strong>Notes:</strong> ${this.escapeHtml(notes)}</p>` : ''}
        ${reactionsHtml}
        ${emojiPickerHtml}
        ${!isCompleted ? '<p class="text-muted" style="margin-top: 1rem;">This goal hasn\'t been completed yet.</p>' : ''}
      </div>
      <div style="margin-top: 1.5rem;">
        <button type="button" class="btn btn-secondary" style="width: 100%;" onclick="App.closeModal()">
          Close
        </button>
      </div>
    `);
  },

  async reactToItem(itemId, emoji) {
    try {
      await API.reactions.add(itemId, emoji);
      this.toast('Reaction added!', 'success');
      this.closeModal();
      // Refresh the modal with updated reactions
      const item = this.currentCard.items?.find(i => i.id === itemId);
      if (item) {
        this.showFriendItemModal(itemId, item.content, item.is_completed);
      }
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async removeReaction(itemId) {
    try {
      await API.reactions.remove(itemId);
      this.toast('Reaction removed', 'success');
      this.closeModal();
      const item = this.currentCard.items?.find(i => i.id === itemId);
      if (item) {
        this.showFriendItemModal(itemId, item.content, item.is_completed);
      }
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  // Profile page
  renderProfile(container) {
    const verifiedBadge = this.user.email_verified
      ? '<span class="badge badge-success">Verified</span>'
      : '<span class="badge badge-warning">Not verified</span>';

    const verificationSection = this.user.email_verified
      ? ''
      : `
        <div class="profile-alert">
          <p><strong>Your email is not verified.</strong> Please check your inbox for the verification email.</p>
          <button class="btn btn-secondary btn-sm" onclick="App.resendVerification()">Resend verification email</button>
        </div>
      `;

    container.innerHTML = `
      <div class="profile-page">
        <div class="profile-header">
          <a href="#dashboard" class="btn btn-ghost">&larr; Back</a>
          <h2>Account Settings</h2>
          <div></div>
        </div>

        ${verificationSection}

        <div class="profile-sections">
          <div class="card profile-section">
            <h3>Profile Information</h3>
            <div class="profile-info-grid">
              <div class="profile-info-item">
                <label>Username</label>
                <span>${this.escapeHtml(this.user.username)}</span>
              </div>
              <div class="profile-info-item">
                <label>Email</label>
                <span>${this.escapeHtml(this.user.email)} ${verifiedBadge}</span>
              </div>
              <div class="profile-info-item">
                <label>Member Since</label>
                <span>${new Date(this.user.created_at).toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric' })}</span>
              </div>
            </div>
          </div>

          <div class="card profile-section">
            <h3>Privacy</h3>
            <div class="profile-privacy">
              <label class="checkbox-label">
                <input type="checkbox" id="searchable-toggle" ${this.user.searchable ? 'checked' : ''}>
                <span>Allow others to find me by username</span>
              </label>
              <small class="text-muted">When disabled, you won't appear in friend search results</small>
            </div>
          </div>

          <div class="card profile-section">
            <h3>Change Password</h3>
            <form id="change-password-form" class="profile-form">
              <div class="form-group">
                <label for="current-password">Current Password</label>
                <input type="password" id="current-password" class="form-input" required autocomplete="current-password">
              </div>
              <div class="form-group">
                <label for="new-password">New Password</label>
                <input type="password" id="new-password" class="form-input" required autocomplete="new-password">
                <small class="text-muted">At least 8 characters with uppercase, lowercase, and a number</small>
              </div>
              <div class="form-group">
                <label for="confirm-password">Confirm New Password</label>
                <input type="password" id="confirm-password" class="form-input" required autocomplete="new-password">
              </div>
              <div class="form-error hidden" id="password-error"></div>
              <button type="submit" class="btn btn-primary">Update Password</button>
            </form>
          </div>

          <div class="card profile-section">
            <h3>Account Actions</h3>
            <div class="profile-actions">
              <button class="btn btn-ghost" onclick="App.logout()">Sign Out</button>
            </div>
          </div>
        </div>
      </div>
    `;

    this.setupProfileEvents();
  },

  setupProfileEvents() {
    const form = document.getElementById('change-password-form');
    const errorEl = document.getElementById('password-error');

    // Privacy toggle
    const searchableToggle = document.getElementById('searchable-toggle');
    searchableToggle.addEventListener('change', async (e) => {
      try {
        const response = await API.auth.updateSearchable(e.target.checked);
        this.user = response.user;
        this.toast(e.target.checked ? 'You are now searchable' : 'You are now hidden from search', 'success');
      } catch (error) {
        e.target.checked = !e.target.checked; // Revert on error
        this.toast(error.message, 'error');
      }
    });

    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      errorEl.classList.add('hidden');

      const currentPassword = document.getElementById('current-password').value;
      const newPassword = document.getElementById('new-password').value;
      const confirmPassword = document.getElementById('confirm-password').value;

      if (newPassword !== confirmPassword) {
        errorEl.textContent = 'New passwords do not match';
        errorEl.classList.remove('hidden');
        return;
      }

      if (newPassword.length < 8) {
        errorEl.textContent = 'Password must be at least 8 characters';
        errorEl.classList.remove('hidden');
        return;
      }

      try {
        await API.auth.changePassword(currentPassword, newPassword);
        form.reset();
        this.toast('Password updated successfully', 'success');
      } catch (error) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      }
    });
  },

  // Archive card view (for viewing individual archived cards)
  async renderArchiveCard(container, cardId) {
    container.innerHTML = `
      <div class="text-center"><div class="spinner" style="margin: 2rem auto;"></div></div>
    `;

    try {
      const [cardResponse, statsResponse] = await Promise.all([
        API.cards.get(cardId),
        API.cards.getStats(cardId),
      ]);

      this.currentCard = cardResponse.card;
      this.currentStats = statsResponse.stats;

      this.renderArchiveCardView(container);
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

  renderArchiveCardView(container) {
    const completedCount = this.currentCard.items.filter(i => i.is_completed).length;
    const progress = Math.round((completedCount / 24) * 100);
    const stats = this.currentStats;
    const displayName = this.getCardDisplayName(this.currentCard);
    const categoryBadge = this.getCategoryBadge(this.currentCard);
    const visibilityIcon = this.currentCard.visible_to_friends ? 'eye' : 'eye-slash';
    const visibilityLabel = this.currentCard.visible_to_friends ? 'Visible' : 'Private';

    container.innerHTML = `
      <div class="archive-card-view">
        <div class="archive-card-header">
          <a href="#dashboard" class="btn btn-ghost">&larr; Back</a>
          <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; justify-content: center;">
            <h2 style="margin: 0;">${displayName}</h2>
            <span class="year-badge">${this.currentCard.year}</span>
            ${categoryBadge}
          </div>
          <div class="card-header-actions">
            <button class="visibility-toggle-btn ${this.currentCard.visible_to_friends ? 'visibility-toggle-btn--visible' : 'visibility-toggle-btn--private'}" onclick="App.toggleCardVisibility('${this.currentCard.id}', ${!this.currentCard.visible_to_friends})" title="${visibilityLabel}">
              <i class="fas fa-${visibilityIcon}"></i>
              <span>${visibilityLabel}</span>
            </button>
            <div class="archive-badge">Archived</div>
          </div>
        </div>

        <div class="archive-stats-grid">
          <div class="stat-card">
            <div class="stat-value">${stats.completed_items}/${stats.total_items}</div>
            <div class="stat-label">Completed</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">${stats.completion_rate.toFixed(0)}%</div>
            <div class="stat-label">Completion Rate</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">${stats.bingos_achieved}</div>
            <div class="stat-label">Bingos</div>
          </div>
        </div>

        ${stats.first_completion ? `
          <div class="archive-dates">
            <p class="text-muted">
              First completion: ${new Date(stats.first_completion).toLocaleDateString()}
              ${stats.last_completion ? ` | Last completion: ${new Date(stats.last_completion).toLocaleDateString()}` : ''}
            </p>
          </div>
        ` : ''}

        <div class="bingo-container bingo-container--finalized">
          <div class="bingo-grid bingo-grid--finalized bingo-grid--archive" id="bingo-grid">
            ${this.renderArchiveGrid()}
          </div>
        </div>

        <div class="finalized-card-progress">
          <div class="progress-bar">
            <div class="progress-fill" style="width: ${progress}%"></div>
          </div>
          <p class="progress-text">${completedCount}/24 completed</p>
        </div>
      </div>
    `;

    this.setupArchiveCardEvents();
  },

  renderArchiveGrid() {
    const headers = ['B', 'I', 'N', 'G', 'O'];
    const headerRow = headers.map(letter => `
      <div class="bingo-header">${letter}</div>
    `).join('');

    const cells = [];
    const itemsByPosition = {};

    if (this.currentCard.items) {
      this.currentCard.items.forEach(item => {
        itemsByPosition[item.position] = item;
      });
    }

    for (let i = 0; i < 25; i++) {
      if (i === 12) {
        cells.push(`
          <div class="bingo-cell bingo-cell--free">
            <span class="bingo-cell-content">FREE</span>
          </div>
        `);
      } else {
        const item = itemsByPosition[i];
        if (item) {
          const isCompleted = item.is_completed;
          const shortText = this.truncateText(item.content, 50);
          cells.push(`
            <div class="bingo-cell ${isCompleted ? 'bingo-cell--completed' : ''}"
                 data-position="${i}"
                 data-item-id="${item.id}"
                 data-content="${this.escapeHtml(item.content)}"
                 title="${this.escapeHtml(item.content)}">
              <span class="bingo-cell-content">${this.escapeHtml(shortText)}</span>
            </div>
          `);
        } else {
          cells.push(`
            <div class="bingo-cell bingo-cell--empty" data-position="${i}"></div>
          `);
        }
      }
    }

    return headerRow + cells.join('');
  },

  setupArchiveCardEvents() {
    document.getElementById('bingo-grid').addEventListener('click', (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (!cell || cell.classList.contains('bingo-cell--free') || cell.classList.contains('bingo-cell--empty')) return;

      const position = parseInt(cell.dataset.position);
      const content = cell.dataset.content;
      const isCompleted = cell.classList.contains('bingo-cell--completed');

      this.showArchiveItemModal(position, content, isCompleted);
    });
  },

  showArchiveItemModal(position, content, isCompleted) {
    const item = this.currentCard.items?.find(i => i.position === position);
    const notes = item?.notes || '';
    const completedAt = item?.completed_at ? new Date(item.completed_at).toLocaleDateString() : null;

    this.openModal(isCompleted ? 'Completed Goal' : 'Goal', `
      <div class="item-detail">
        <p class="item-detail-content">${this.escapeHtml(content)}</p>
        ${isCompleted ? `
          ${completedAt ? `<p class="text-muted" style="margin-top: 0.5rem;">Completed on ${completedAt}</p>` : ''}
          ${notes ? `<p class="item-detail-notes"><strong>Notes:</strong> ${this.escapeHtml(notes)}</p>` : ''}
        ` : `
          <p class="text-muted" style="margin-top: 1rem;">This goal was not completed.</p>
        `}
      </div>
      <div style="margin-top: 1.5rem;">
        <button type="button" class="btn btn-secondary" style="width: 100%;" onclick="App.closeModal()">
          Close
        </button>
      </div>
    `);
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

  // Dropdown Menu
  setupDropdowns() {
    document.querySelectorAll('.dropdown').forEach(dropdown => {
      const toggle = dropdown.querySelector('.dropdown-toggle');
      const menu = dropdown.querySelector('.dropdown-menu');

      if (!toggle || !menu) return;

      toggle.addEventListener('click', (e) => {
        e.stopPropagation();
        const isVisible = menu.classList.contains('dropdown-menu--visible');

        // Close all other dropdowns
        document.querySelectorAll('.dropdown-menu--visible').forEach(m => {
          m.classList.remove('dropdown-menu--visible');
        });

        if (!isVisible) {
          menu.classList.add('dropdown-menu--visible');
          toggle.setAttribute('aria-expanded', 'true');
        } else {
          toggle.setAttribute('aria-expanded', 'false');
        }
      });
    });

    // Close dropdowns when clicking outside
    document.addEventListener('click', () => {
      document.querySelectorAll('.dropdown-menu--visible').forEach(menu => {
        menu.classList.remove('dropdown-menu--visible');
      });
      document.querySelectorAll('.dropdown-toggle').forEach(toggle => {
        toggle.setAttribute('aria-expanded', 'false');
      });
    });
  },

  // Export helper functions
  generateCSV(card) {
    const categoryNames = {
      personal: 'Personal Growth',
      health: 'Health & Fitness',
      food: 'Food & Dining',
      travel: 'Travel & Adventure',
      hobbies: 'Hobbies & Creativity',
      social: 'Social & Relationships',
      professional: 'Professional & Career',
      fun: 'Fun & Silly',
    };

    const cardTitle = card.title || `${card.year} Bingo Card`;
    const categoryName = card.category ? (categoryNames[card.category] || card.category) : '';

    // CSV header
    const headers = ['card_title', 'year', 'category', 'position', 'item_text', 'completed', 'completion_date', 'notes'];

    // Generate rows
    const rows = (card.items || []).map(item => {
      const completedDate = item.completed_at ? item.completed_at.slice(0, 10) : '';
      const notes = item.notes || '';

      return [
        cardTitle,
        card.year.toString(),
        categoryName,
        item.position.toString(),
        item.content,
        item.is_completed ? 'yes' : 'no',
        completedDate,
        notes
      ];
    });

    // Sort by position
    rows.sort((a, b) => parseInt(a[3]) - parseInt(b[3]));

    // Build CSV with BOM for Excel compatibility
    const BOM = '\uFEFF';
    const csvContent = [
      headers.join(','),
      ...rows.map(row => row.map(cell => this.escapeCSV(cell)).join(','))
    ].join('\r\n');

    return BOM + csvContent;
  },

  escapeCSV(value) {
    if (value === null || value === undefined) {
      return '';
    }
    const str = String(value);
    // If the value contains comma, newline, or quote, wrap in quotes and escape quotes
    if (str.includes(',') || str.includes('\n') || str.includes('\r') || str.includes('"')) {
      return '"' + str.replace(/"/g, '""') + '"';
    }
    return str;
  },

  getUniqueFilename(card, usedFilenames) {
    const title = card.title || 'Bingo Card';
    // Sanitize filename: remove/replace invalid characters
    const sanitized = title
      .replace(/[<>:"/\\|?*]/g, '')
      .replace(/\s+/g, '_')
      .slice(0, 50);

    let filename = `${card.year}_${sanitized}.csv`;
    let counter = 1;

    while (usedFilenames.has(filename)) {
      filename = `${card.year}_${sanitized}_${counter}.csv`;
      counter++;
    }

    return filename;
  },

  downloadBlob(blob, filename) {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
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

  // Legal Pages
  renderTerms(container) {
    container.innerHTML = `
      <div class="legal-page">
        <h1>Terms of Service</h1>
        <p class="legal-updated">Last Updated: November 29, 2025</p>

        <div class="legal-content">
          <p class="legal-intro">
            Please read this agreement carefully before using Year of Bingo. By using Year of Bingo, you agree that your use is governed by this agreement. If you do not accept these terms, please do not use the service.
          </p>

          <h2>1. Overview</h2>
          <p>
            Year of Bingo ("Service," "we," "us," or "our") is a web application for creating and tracking annual bingo cards with personal goals. This Terms of Service Agreement ("Agreement") is between Year of Bingo and you ("you" or "User").
          </p>

          <h2>2. Your Account</h2>
          <p>
            To access certain features, you must create an account with a valid email address. You are responsible for maintaining the confidentiality of your password and account information. You are solely responsible for all activities that occur under your account.
          </p>
          <p>
            You agree to:
          </p>
          <ul>
            <li>Provide accurate account information</li>
            <li>Keep your password secure and confidential</li>
            <li>Notify us immediately of any unauthorized access</li>
            <li>Not create multiple accounts to circumvent limitations</li>
          </ul>

          <h2>3. Acceptable Use</h2>
          <p>
            You agree to use the Service in accordance with all applicable laws and regulations. You will not:
          </p>
          <ul>
            <li>Use the Service for any unlawful purpose</li>
            <li>Interfere with or disrupt the Service or servers</li>
            <li>Attempt to gain unauthorized access to any part of the Service</li>
            <li>Upload content that is harmful, offensive, or infringes on others' rights</li>
            <li>Use the Service to harass, abuse, or harm others</li>
            <li>Impersonate any person or entity</li>
          </ul>

          <h2>4. Your Content</h2>
          <p>
            "Content" means any data, text, or information you submit to the Service, including bingo card items, notes, and profile information. You retain ownership of your Content. By submitting Content, you grant us a license to store, display, and process your Content solely for the purpose of providing the Service to you.
          </p>
          <p>
            You are solely responsible for your Content and ensuring it complies with this Agreement and applicable laws. You represent that you have the right to submit any Content you provide.
          </p>

          <h2>5. Data Backup</h2>
          <p>
            You are responsible for maintaining backups of your Content. We are not responsible for any loss or deletion of Content. While we take reasonable measures to protect your data, we make no guarantees regarding data preservation.
          </p>

          <h2>6. Privacy</h2>
          <p>
            Your use of the Service is also governed by our <a href="#privacy">Privacy Policy</a>, which describes how we collect, use, and protect your personal data.
          </p>

          <h2>7. Changes to the Service</h2>
          <p>
            We may modify, suspend, or discontinue any part of the Service at any time. We will make reasonable efforts to notify users of significant changes, but are not obligated to do so.
          </p>

          <h2>8. Termination</h2>
          <p>
            You may stop using the Service at any time. We may suspend or terminate your access if we believe you have violated this Agreement or for any other reason at our discretion. Upon termination, your right to use the Service ceases immediately.
          </p>

          <h2>9. Disclaimer of Warranties</h2>
          <p>
            THE SERVICE IS PROVIDED "AS IS" AND "AS AVAILABLE" WITHOUT WARRANTIES OF ANY KIND, EXPRESS OR IMPLIED. WE DO NOT WARRANT THAT THE SERVICE WILL BE UNINTERRUPTED, ERROR-FREE, OR SECURE. WE MAKE NO WARRANTIES REGARDING THE ACCURACY OR RELIABILITY OF ANY CONTENT OR INFORMATION OBTAINED THROUGH THE SERVICE.
          </p>

          <h2>10. Limitation of Liability</h2>
          <p>
            TO THE MAXIMUM EXTENT PERMITTED BY LAW, WE SHALL NOT BE LIABLE FOR ANY INDIRECT, INCIDENTAL, SPECIAL, CONSEQUENTIAL, OR PUNITIVE DAMAGES, INCLUDING BUT NOT LIMITED TO LOSS OF DATA, LOSS OF PROFITS, OR BUSINESS INTERRUPTION, ARISING FROM YOUR USE OF THE SERVICE.
          </p>

          <h2>11. Indemnification</h2>
          <p>
            You agree to indemnify and hold harmless Year of Bingo and its operators from any claims, damages, or expenses arising from your use of the Service, your Content, or your violation of this Agreement.
          </p>

          <h2>12. Changes to Terms</h2>
          <p>
            We may modify this Agreement at any time by posting the revised terms on our website. Your continued use of the Service after changes are posted constitutes acceptance of the modified terms. It is your responsibility to review this Agreement periodically.
          </p>

          <h2>13. Governing Law</h2>
          <p>
            This Agreement shall be governed by and construed in accordance with applicable law, without regard to conflict of law principles.
          </p>

          <h2>14. Contact</h2>
          <p>
            If you have questions about this Agreement, please contact us at <a href="mailto:support@yearofbingo.com">support@yearofbingo.com</a>.
          </p>
        </div>
      </div>
    `;
  },

  renderPrivacy(container) {
    container.innerHTML = `
      <div class="legal-page">
        <h1>Privacy Policy</h1>
        <p class="legal-updated">Last Updated: November 29, 2025</p>

        <div class="legal-content">
          <nav class="legal-toc">
            <h3>Table of Contents</h3>
            <ol>
              <li><a href="#privacy-scope">Scope of this Privacy Policy</a></li>
              <li><a href="#privacy-collect">Information We Collect</a></li>
              <li><a href="#privacy-use">How We Use Your Information</a></li>
              <li><a href="#privacy-share">How We Share Your Information</a></li>
              <li><a href="#privacy-cookies">Cookies and Similar Technologies</a></li>
              <li><a href="#privacy-rights">Your Rights and Choices</a></li>
              <li><a href="#privacy-security">Security</a></li>
              <li><a href="#privacy-children">Children's Privacy</a></li>
              <li><a href="#privacy-international">International Data Transfers</a></li>
              <li><a href="#privacy-changes">Changes to this Policy</a></li>
              <li><a href="#privacy-contact">How to Contact Us</a></li>
            </ol>
          </nav>

          <h2 id="privacy-scope">1. Scope of this Privacy Policy</h2>
          <p>
            This Privacy Policy applies to personal data collected by Year of Bingo through the yearofbingo.com website. It describes how we collect, use, and protect your personal information.
          </p>
          <p>
            "Personal data" means any information that relates to an identified or identifiable person, such as a name, email address, or online identifier.
          </p>

          <h2 id="privacy-collect">2. Information We Collect</h2>
          <p>We collect the following categories of personal data:</p>

          <h3>Information You Provide</h3>
          <ul>
            <li><strong>Account information:</strong> Email address, username, and password when you create an account</li>
            <li><strong>Content:</strong> Bingo card titles, items, notes, and completion status</li>
            <li><strong>Social features:</strong> Friend connections and reactions to friends' cards</li>
          </ul>

          <h3>Information Collected Automatically</h3>
          <ul>
            <li><strong>Analytics data:</strong> We use Cloudflare Web Analytics, a privacy-focused analytics service that does not use cookies or collect personal data. It provides aggregate statistics about page views and visitor counts without tracking individual users.</li>
            <li><strong>Log data:</strong> Our servers may log IP addresses, browser type, and access times for security and troubleshooting purposes. This data is not linked to your account and is retained for a limited period.</li>
          </ul>

          <h2 id="privacy-use">3. How We Use Your Information</h2>
          <p>We use your personal data to:</p>
          <ul>
            <li><strong>Provide the Service:</strong> Create and manage your account, store your bingo cards, and enable social features</li>
            <li><strong>Authenticate you:</strong> Verify your identity when you log in</li>
            <li><strong>Communicate with you:</strong> Send password reset emails, verification emails, and important service announcements</li>
            <li><strong>Improve the Service:</strong> Understand how the Service is used through aggregate analytics to improve functionality</li>
            <li><strong>Ensure security:</strong> Detect and prevent fraud, abuse, and security incidents</li>
          </ul>
          <p>
            We do not sell your personal data. We do not use your data for advertising or marketing purposes beyond the Service.
          </p>

          <h2 id="privacy-share">4. How We Share Your Information</h2>
          <p>We share your information only in limited circumstances:</p>
          <ul>
            <li><strong>With your friends:</strong> If you add friends, they can see your bingo cards (unless you mark a card as private), including items and completion status</li>
            <li><strong>Service providers:</strong> We use third-party services to help operate the Service (such as email delivery and hosting). These providers are contractually obligated to protect your data</li>
            <li><strong>Legal requirements:</strong> We may disclose information if required by law, legal process, or government request</li>
            <li><strong>Business transfers:</strong> In connection with a merger, acquisition, or sale of assets, your information may be transferred</li>
          </ul>

          <h2 id="privacy-cookies">5. Cookies and Similar Technologies</h2>
          <p>
            <strong>We use only strictly necessary cookies.</strong> These are essential for the Service to function and cannot be disabled.
          </p>

          <h3>Cookies We Use</h3>
          <ul>
            <li><strong>Session cookie:</strong> A secure, HTTP-only cookie that keeps you logged in. This cookie is essential for authentication and expires after 30 days of inactivity.</li>
            <li><strong>CSRF token:</strong> A security cookie that protects against cross-site request forgery attacks.</li>
          </ul>

          <h3>Cloudflare Cookies</h3>
          <p>
            Our website is served through Cloudflare, which may set strictly necessary cookies for security purposes (such as bot detection). These cookies are essential for protecting the Service and do not track you for advertising purposes.
          </p>

          <h3>No Tracking Cookies</h3>
          <p>
            We do not use tracking cookies, advertising cookies, or third-party analytics that track individual users. Our analytics solution (Cloudflare Web Analytics) is cookie-free and privacy-focused.
          </p>
          <p>
            <strong>Because we only use strictly necessary cookies, no cookie consent banner is required under GDPR or similar regulations.</strong>
          </p>

          <h2 id="privacy-rights">6. Your Rights and Choices</h2>
          <p>
            Depending on your location, you may have the following rights regarding your personal data:
          </p>
          <ul>
            <li><strong>Access:</strong> Request information about the personal data we hold about you</li>
            <li><strong>Correction:</strong> Request correction of inaccurate personal data</li>
            <li><strong>Deletion:</strong> Request deletion of your personal data</li>
            <li><strong>Portability:</strong> Request a copy of your data in a portable format</li>
            <li><strong>Objection:</strong> Object to certain processing of your personal data</li>
            <li><strong>Withdrawal of consent:</strong> Where processing is based on consent, withdraw that consent</li>
          </ul>
          <p>
            To exercise these rights, please contact us at <a href="mailto:privacy@yearofbingo.com">privacy@yearofbingo.com</a>. We will respond to your request within the timeframe required by applicable law.
          </p>

          <h3>Account Settings</h3>
          <p>
            You can manage your account settings directly in the Service:
          </p>
          <ul>
            <li>Update your email or password in your Profile</li>
            <li>Control whether you appear in friend search (discoverability)</li>
            <li>Set individual cards as private or visible to friends</li>
            <li>Delete your account (this will permanently delete all your data)</li>
          </ul>

          <h2 id="privacy-security">7. Security</h2>
          <p>
            We implement appropriate technical and organizational measures to protect your personal data, including:
          </p>
          <ul>
            <li>Encryption of data in transit (HTTPS/TLS)</li>
            <li>Secure password hashing</li>
            <li>HTTP-only, secure session cookies</li>
            <li>CSRF protection</li>
            <li>Regular security updates</li>
          </ul>
          <p>
            For more details about our security practices, see our <a href="#security">Security page</a>.
          </p>

          <h2 id="privacy-children">8. Children's Privacy</h2>
          <p>
            Year of Bingo is not directed to children under 16 years of age. We do not knowingly collect personal data from children under 16. If you believe we have collected information from a child under 16, please contact us at <a href="mailto:privacy@yearofbingo.com">privacy@yearofbingo.com</a> and we will delete it.
          </p>

          <h2 id="privacy-international">9. International Data Transfers</h2>
          <p>
            Year of Bingo is operated from the United States. If you are accessing the Service from outside the United States, your data will be transferred to and processed in the United States. By using the Service, you consent to this transfer.
          </p>
          <p>
            For users in the European Economic Area (EEA), United Kingdom, or Switzerland: We rely on your consent and, where applicable, standard contractual clauses approved by the European Commission to ensure adequate protection for your data.
          </p>

          <h2 id="privacy-changes">10. Changes to this Policy</h2>
          <p>
            We may update this Privacy Policy from time to time. We will notify you of material changes by posting a notice on our website. Your continued use of the Service after changes are posted constitutes acceptance of the updated policy.
          </p>

          <h2 id="privacy-contact">11. How to Contact Us</h2>
          <p>
            If you have questions about this Privacy Policy or wish to exercise your privacy rights, please contact us at:
          </p>
          <p>
            <strong>Email:</strong> <a href="mailto:privacy@yearofbingo.com">privacy@yearofbingo.com</a>
          </p>
          <p>
            For EEA residents, you also have the right to lodge a complaint with your local data protection authority.
          </p>
        </div>
      </div>
    `;
  },

  renderSecurity(container) {
    container.innerHTML = `
      <div class="legal-page">
        <h1>Security</h1>
        <p class="legal-updated">Last Updated: November 29, 2025</p>

        <div class="legal-content">
          <p class="legal-intro">
            At Year of Bingo, we take the security of your data seriously. This page describes the security measures we implement to protect your information.
          </p>

          <h2>Infrastructure Security</h2>
          <ul>
            <li><strong>HTTPS Everywhere:</strong> All connections to Year of Bingo are encrypted using TLS (Transport Layer Security). We enforce HTTPS with HSTS (HTTP Strict Transport Security).</li>
            <li><strong>Cloudflare Protection:</strong> Our service is protected by Cloudflare, which provides DDoS mitigation, bot protection, and a Web Application Firewall (WAF).</li>
            <li><strong>Secure Hosting:</strong> Our infrastructure is hosted on secure, regularly updated systems.</li>
          </ul>

          <h2>Application Security</h2>
          <ul>
            <li><strong>Password Security:</strong> Passwords are hashed using industry-standard algorithms (bcrypt) and are never stored in plain text.</li>
            <li><strong>Session Security:</strong> Session tokens are cryptographically random, stored securely, and transmitted only via HTTP-only, secure cookies.</li>
            <li><strong>CSRF Protection:</strong> All state-changing requests are protected against Cross-Site Request Forgery attacks.</li>
            <li><strong>Content Security Policy:</strong> We implement strict Content Security Policy (CSP) headers to prevent XSS attacks.</li>
            <li><strong>Input Validation:</strong> All user input is validated and sanitized to prevent injection attacks.</li>
          </ul>

          <h2>Security Headers</h2>
          <p>We implement the following security headers on all responses:</p>
          <ul>
            <li><strong>Content-Security-Policy:</strong> Restricts resource loading to trusted sources</li>
            <li><strong>X-Frame-Options:</strong> Prevents clickjacking by blocking framing</li>
            <li><strong>X-Content-Type-Options:</strong> Prevents MIME type sniffing</li>
            <li><strong>Referrer-Policy:</strong> Controls referrer information sent with requests</li>
            <li><strong>Permissions-Policy:</strong> Restricts browser feature access</li>
          </ul>

          <h2>Data Protection</h2>
          <ul>
            <li><strong>Encryption in Transit:</strong> All data transmitted between your browser and our servers is encrypted.</li>
            <li><strong>Database Security:</strong> Our database is not directly accessible from the internet and requires authentication.</li>
            <li><strong>Access Controls:</strong> Access to production systems is restricted to authorized personnel only.</li>
          </ul>

          <h2>Responsible Disclosure</h2>
          <p>
            We appreciate the security research community's efforts to improve the security of our service. If you discover a security vulnerability, please report it responsibly:
          </p>
          <ul>
            <li><strong>Email:</strong> <a href="mailto:security@yearofbingo.com">security@yearofbingo.com</a></li>
            <li>Please provide detailed information about the vulnerability</li>
            <li>Allow reasonable time for us to address the issue before public disclosure</li>
            <li>Do not access or modify other users' data</li>
          </ul>

          <h2>What We Ask of You</h2>
          <p>
            Security is a shared responsibility. We ask that you:
          </p>
          <ul>
            <li>Use a strong, unique password for your account</li>
            <li>Keep your password confidential and do not share it</li>
            <li>Log out when using shared or public computers</li>
            <li>Report any suspicious activity on your account immediately</li>
            <li>Keep your browser and operating system up to date</li>
          </ul>

          <h2>Questions</h2>
          <p>
            If you have questions about our security practices, please contact us at <a href="mailto:security@yearofbingo.com">security@yearofbingo.com</a>.
          </p>
        </div>
      </div>
    `;
  },

  renderSupport(container) {
    const userEmail = this.user?.email || '';

    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card">
          <h2>Contact Support</h2>
          <p class="text-muted" style="margin-bottom: 1.5rem;">
            Have a question, found a bug, or want to request a feature? We'd love to hear from you!
          </p>

          <form id="support-form">
            <div class="form-group">
              <label class="form-label" for="support-email">Your Email</label>
              <input
                type="email"
                id="support-email"
                class="form-input"
                required
                placeholder="your@email.com"
                value="${this.escapeHtml(userEmail)}"
              >
            </div>

            <div class="form-group">
              <label class="form-label" for="support-category">Category</label>
              <select id="support-category" class="form-input" required>
                <option value="">Select a category...</option>
                <option value="Bug Report">Bug Report</option>
                <option value="Feature Request">Feature Request</option>
                <option value="Account Issue">Account Issue</option>
                <option value="General Question">General Question</option>
              </select>
            </div>

            <div class="form-group">
              <label class="form-label" for="support-message">Message</label>
              <textarea
                id="support-message"
                class="form-input"
                required
                rows="6"
                placeholder="Please describe your issue or question in detail..."
                minlength="10"
                maxlength="5000"
              ></textarea>
              <small class="form-hint">Minimum 10 characters</small>
            </div>

            <button type="submit" class="btn btn-primary btn-lg" style="width: 100%;">
              Send Message
            </button>
          </form>
        </div>
      </div>
    `;

    document.getElementById('support-form').addEventListener('submit', async (e) => {
      e.preventDefault();

      const email = document.getElementById('support-email').value.trim();
      const category = document.getElementById('support-category').value;
      const message = document.getElementById('support-message').value.trim();

      if (!email || !category || !message) {
        App.toast('Please fill in all fields', 'error');
        return;
      }

      if (message.length < 10) {
        App.toast('Message must be at least 10 characters', 'error');
        return;
      }

      const submitBtn = e.target.querySelector('button[type="submit"]');
      const originalText = submitBtn.textContent;
      submitBtn.disabled = true;
      submitBtn.textContent = 'Sending...';

      try {
        const result = await API.support.submit(email, category, message);
        App.toast(result.message || 'Message sent successfully!', 'success');

        // Clear the form
        document.getElementById('support-category').value = '';
        document.getElementById('support-message').value = '';
      } catch (error) {
        App.toast(error.message || 'Failed to send message', 'error');
      } finally {
        submitBtn.disabled = false;
        submitBtn.textContent = originalText;
      }
    });
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
