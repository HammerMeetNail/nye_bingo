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
        <a href="#archive" class="nav-link">Archive</a>
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
        this.requireAuth(() => this.renderArchive(container));
        break;
      case 'archive-card':
        this.requireAuth(() => this.renderArchiveCard(container, params[0]));
        break;
      case 'profile':
        this.requireAuth(() => this.renderProfile(container));
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

  async renderDashboard(container) {
    container.innerHTML = `
      ${this.renderEmailVerificationBanner()}
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem;">
        <h2>My Bingo Cards</h2>
        <div style="display: flex; gap: 0.5rem;">
          <div class="dropdown" id="actions-dropdown">
            <button class="btn btn-secondary dropdown-toggle" aria-haspopup="true" aria-expanded="false">
              Actions
            </button>
            <div class="dropdown-menu" role="menu">
              <button class="dropdown-item" role="menuitem" onclick="App.showExportModal()">
                Export Cards
              </button>
            </div>
          </div>
          <button class="btn btn-primary" onclick="App.showCreateCardModal()">+ New Card</button>
        </div>
      </div>
      <div id="cards-list">
        <div class="text-center"><div class="spinner" style="margin: 2rem auto;"></div></div>
      </div>
    `;

    this.setupDropdowns();

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

  renderCardPreview(card) {
    const itemCount = card.items ? card.items.length : 0;
    const completedCount = card.items ? card.items.filter(i => i.is_completed).length : 0;
    const progress = card.is_finalized ? Math.round((completedCount / 24) * 100) : Math.round((itemCount / 24) * 100);
    const displayName = this.getCardDisplayName(card);
    const categoryBadge = this.getCategoryBadge(card);
    const visibilityIcon = card.visible_to_friends ? 'eye' : 'eye-slash';
    const visibilityTitle = card.visible_to_friends ? 'Visible to friends' : 'Private';

    return `
      <div class="card" style="margin-bottom: 1rem;">
        <div style="display: flex; justify-content: space-between; align-items: start;">
          <a href="#card/${card.id}" style="flex: 1; text-decoration: none; color: inherit;">
            <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 0.25rem;">
              <h3 style="margin: 0;">${displayName}</h3>
              <span class="year-badge">${card.year}</span>
              ${categoryBadge}
            </div>
            <p class="text-muted" style="margin: 0.25rem 0 0 0;">
              ${card.is_finalized
                ? `${completedCount}/24 completed`
                : `${itemCount}/24 items added`}
            </p>
          </a>
          <div style="display: flex; gap: 0.5rem; align-items: center;">
            <button class="visibility-toggle visibility-toggle--dashboard" onclick="event.stopPropagation(); event.preventDefault(); App.toggleCardVisibility('${card.id}', ${!card.visible_to_friends})" title="${visibilityTitle}" aria-label="${visibilityTitle}">
              <i class="fas fa-${visibilityIcon}"></i>
            </button>
            <a href="#card/${card.id}" class="btn btn-ghost btn-sm">
              ${card.is_finalized ? 'View' : 'Continue'}
            </a>
            <button class="btn btn-ghost btn-sm" onclick="event.stopPropagation(); App.deleteCard('${card.id}')" title="Delete card" style="color: var(--danger);">
              &times;
            </button>
          </div>
        </div>
        <div class="progress-bar mt-md">
          <div class="progress-fill" style="width: ${progress}%"></div>
        </div>
      </div>
    `;
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

  // Archive page
  selectedArchiveCards: [],

  async renderArchive(container) {
    this.selectedArchiveCards = [];

    container.innerHTML = `
      <div class="archive-page">
        <div class="archive-header">
          <a href="#dashboard" class="btn btn-ghost">&larr; Back</a>
          <h2>Card Archive</h2>
          <div></div>
        </div>
        <div id="archive-list">
          <div class="text-center"><div class="spinner" style="margin: 2rem auto;"></div></div>
        </div>
      </div>
    `;

    try {
      const response = await API.cards.getArchive();
      const cards = response.cards || [];
      this.archiveCards = cards;

      const listEl = document.getElementById('archive-list');
      if (cards.length === 0) {
        listEl.innerHTML = `
          <div class="card text-center" style="padding: 3rem;">
            <div style="font-size: 4rem; margin-bottom: 1rem;">📚</div>
            <h3>No archived cards yet</h3>
            <p class="text-muted mb-lg">Completed cards from past years will appear here.</p>
            <a href="#dashboard" class="btn btn-primary">Go to Dashboard</a>
          </div>
        `;
      } else {
        listEl.innerHTML = `
          <div class="archive-bulk-controls">
            <div style="display: flex; gap: 0.5rem; align-items: center;">
              <button class="btn btn-ghost btn-sm" onclick="App.selectAllArchiveCards()">Select All</button>
              <button class="btn btn-ghost btn-sm" onclick="App.deselectAllArchiveCards()">Deselect All</button>
            </div>
            <div style="display: flex; gap: 0.5rem; align-items: center;">
              <span id="selected-count" class="text-muted" style="font-size: 0.875rem;">0 selected</span>
              <button class="btn btn-secondary btn-sm" onclick="App.bulkSetVisibility(true)" title="Make visible to friends">
                <i class="fas fa-eye"></i> Make Visible
              </button>
              <button class="btn btn-secondary btn-sm" onclick="App.bulkSetVisibility(false)" title="Make private">
                <i class="fas fa-eye-slash"></i> Make Private
              </button>
            </div>
          </div>
          <div class="archive-cards-list">
            ${cards.map(card => this.renderArchiveCardPreview(card)).join('')}
          </div>
        `;
      }
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  renderArchiveCardPreview(card) {
    const completedCount = card.items ? card.items.filter(i => i.is_completed).length : 0;
    const progress = Math.round((completedCount / 24) * 100);
    const displayName = this.getCardDisplayName(card);
    const categoryBadge = this.getCategoryBadge(card);
    const visibilityIcon = card.visible_to_friends ? 'eye' : 'eye-slash';
    const visibilityLabel = card.visible_to_friends ? 'Visible to friends' : 'Private';

    return `
      <div class="card archive-card-preview" style="margin-bottom: 1rem;">
        <div class="archive-card-preview-header">
          <div style="display: flex; align-items: flex-start; gap: 0.75rem;">
            <label class="archive-checkbox-label" onclick="event.stopPropagation();">
              <input type="checkbox" class="archive-card-checkbox" data-card-id="${card.id}" onchange="App.updateArchiveSelection()">
            </label>
            <a href="#archive-card/${card.id}" style="text-decoration: none; flex: 1;">
              <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 0.25rem;">
                <h3 style="margin: 0;">${displayName}</h3>
                <span class="year-badge">${card.year}</span>
                ${categoryBadge}
              </div>
              <p class="text-muted" style="margin: 0;">${completedCount}/24 completed</p>
            </a>
          </div>
          <div style="display: flex; align-items: center; gap: 0.5rem;">
            <span class="visibility-badge visibility-badge--${card.visible_to_friends ? 'visible' : 'private'}" title="${visibilityLabel}">
              <i class="fas fa-${visibilityIcon}"></i> ${visibilityLabel}
            </span>
            <div class="archive-badge">Archived</div>
          </div>
        </div>
        <a href="#archive-card/${card.id}" style="text-decoration: none; display: block;">
          <div class="progress-bar mt-md">
            <div class="progress-fill" style="width: ${progress}%"></div>
          </div>
        </a>
      </div>
    `;
  },

  updateArchiveSelection() {
    const checkboxes = document.querySelectorAll('.archive-card-checkbox');
    this.selectedArchiveCards = Array.from(checkboxes)
      .filter(cb => cb.checked)
      .map(cb => cb.dataset.cardId);

    const countEl = document.getElementById('selected-count');
    if (countEl) {
      countEl.textContent = `${this.selectedArchiveCards.length} selected`;
    }
  },

  selectAllArchiveCards() {
    const checkboxes = document.querySelectorAll('.archive-card-checkbox');
    checkboxes.forEach(cb => cb.checked = true);
    this.updateArchiveSelection();
  },

  deselectAllArchiveCards() {
    const checkboxes = document.querySelectorAll('.archive-card-checkbox');
    checkboxes.forEach(cb => cb.checked = false);
    this.updateArchiveSelection();
  },

  async bulkSetVisibility(visibleToFriends) {
    if (this.selectedArchiveCards.length === 0) {
      this.toast('No cards selected', 'warning');
      return;
    }

    try {
      const response = await API.cards.bulkUpdateVisibility(this.selectedArchiveCards, visibleToFriends);
      const count = response.updated_count || this.selectedArchiveCards.length;
      this.toast(`${count} card${count !== 1 ? 's' : ''} updated`, 'success');
      // Refresh the archive list
      this.renderArchive(document.getElementById('main-container'));
    } catch (error) {
      this.toast(error.message || 'Failed to update visibility', 'error');
    }
  },

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
          <a href="#archive" class="btn btn-primary">Back to Archive</a>
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
          <a href="#archive" class="btn btn-ghost">&larr; Archive</a>
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

  // Export functionality
  async showExportModal() {
    // Close any open dropdowns
    document.querySelectorAll('.dropdown-menu--visible').forEach(menu => {
      menu.classList.remove('dropdown-menu--visible');
    });

    this.openModal('Export Cards', `
      <div class="text-center"><div class="spinner" style="margin: 1rem auto;"></div></div>
      <p class="text-muted text-center">Loading cards...</p>
    `);

    try {
      const response = await API.cards.getExportable();
      const cards = response.cards || [];

      if (cards.length === 0) {
        document.getElementById('modal-body').innerHTML = `
          <div class="export-empty">
            <p>No cards to export.</p>
            <p class="text-muted">Create a card first to export it.</p>
          </div>
          <div style="margin-top: 1.5rem;">
            <button type="button" class="btn btn-secondary" style="width: 100%;" onclick="App.closeModal()">
              Close
            </button>
          </div>
        `;
        return;
      }

      // Sort by year descending, then by title
      cards.sort((a, b) => {
        if (b.year !== a.year) return b.year - a.year;
        const nameA = a.title || `${a.year} Bingo Card`;
        const nameB = b.title || `${b.year} Bingo Card`;
        return nameA.localeCompare(nameB);
      });

      const currentYear = new Date().getFullYear();
      const cardListHtml = cards.map(card => {
        const displayName = this.getCardDisplayName(card);
        const itemCount = card.items ? card.items.length : 0;
        const isArchived = card.year < currentYear;
        const archivedLabel = isArchived ? ' (Archived)' : '';

        return `
          <div class="export-card-item">
            <input type="checkbox" id="export-card-${card.id}" value="${card.id}" checked>
            <label for="export-card-${card.id}">
              <span class="export-card-name">${this.escapeHtml(displayName)}</span>
              <span class="export-card-meta">${card.year}${archivedLabel} - ${itemCount} items</span>
            </label>
          </div>
        `;
      }).join('');

      document.getElementById('modal-body').innerHTML = `
        <p class="text-muted">Select cards to export as CSV:</p>

        <div class="export-select-all">
          <input type="checkbox" id="export-select-all" checked>
          <label for="export-select-all">Select All</label>
        </div>

        <div class="export-card-list">
          ${cardListHtml}
        </div>

        <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-ghost" style="flex: 1;" onclick="App.closeModal()">
            Cancel
          </button>
          <button type="button" class="btn btn-primary" style="flex: 1;" id="export-download-btn" onclick="App.downloadExport()">
            Download ZIP
          </button>
        </div>
      `;

      // Store cards for export
      this.exportableCards = cards;

      // Setup select all functionality
      const selectAllCheckbox = document.getElementById('export-select-all');
      const cardCheckboxes = document.querySelectorAll('.export-card-item input[type="checkbox"]');

      selectAllCheckbox.addEventListener('change', () => {
        cardCheckboxes.forEach(cb => {
          cb.checked = selectAllCheckbox.checked;
        });
        this.updateExportButton();
      });

      cardCheckboxes.forEach(cb => {
        cb.addEventListener('change', () => {
          const allChecked = Array.from(cardCheckboxes).every(c => c.checked);
          const someChecked = Array.from(cardCheckboxes).some(c => c.checked);
          selectAllCheckbox.checked = allChecked;
          selectAllCheckbox.indeterminate = someChecked && !allChecked;
          this.updateExportButton();
        });
      });

    } catch (error) {
      document.getElementById('modal-body').innerHTML = `
        <div class="export-empty">
          <p>Error loading cards: ${this.escapeHtml(error.message)}</p>
        </div>
        <div style="margin-top: 1.5rem;">
          <button type="button" class="btn btn-secondary" style="width: 100%;" onclick="App.closeModal()">
            Close
          </button>
        </div>
      `;
    }
  },

  updateExportButton() {
    const cardCheckboxes = document.querySelectorAll('.export-card-item input[type="checkbox"]');
    const someChecked = Array.from(cardCheckboxes).some(c => c.checked);
    const downloadBtn = document.getElementById('export-download-btn');
    if (downloadBtn) {
      downloadBtn.disabled = !someChecked;
    }
  },

  async downloadExport() {
    const cardCheckboxes = document.querySelectorAll('.export-card-item input[type="checkbox"]:checked');
    const selectedIds = Array.from(cardCheckboxes).map(cb => cb.value);

    if (selectedIds.length === 0) {
      this.toast('Please select at least one card to export', 'error');
      return;
    }

    const selectedCards = this.exportableCards.filter(card => selectedIds.includes(card.id));

    const downloadBtn = document.getElementById('export-download-btn');
    if (downloadBtn) {
      this.setButtonLoading(downloadBtn, true);
    }

    try {
      const zip = new JSZip();
      const usedFilenames = new Set();

      for (const card of selectedCards) {
        const csv = this.generateCSV(card);
        const filename = this.getUniqueFilename(card, usedFilenames);
        usedFilenames.add(filename);
        zip.file(filename, csv);
      }

      const blob = await zip.generateAsync({ type: 'blob' });
      const timestamp = new Date().toISOString().slice(0, 10);
      this.downloadBlob(blob, `yearofbingo_export_${timestamp}.zip`);

      this.closeModal();
      this.toast(`Exported ${selectedCards.length} card${selectedCards.length > 1 ? 's' : ''}`, 'success');
    } catch (error) {
      this.toast('Error generating export: ' + error.message, 'error');
    } finally {
      if (downloadBtn) {
        this.setButtonLoading(downloadBtn, false);
      }
    }
  },

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
};

// Initialize app
document.addEventListener('DOMContentLoaded', () => {
  App.init();
});

// Handle hash changes
window.addEventListener('hashchange', () => {
  App.route();
});
