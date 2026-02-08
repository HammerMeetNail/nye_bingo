// Year of Bingo - Auth/Profile Module (scaffold)
// SCAFFOLD: Not yet loaded in production. See plans/refactor.md for extraction status.

window.App = window.App || {};
var App = window.App;

if (!App._moduleAuthLoaded) {
  App._moduleAuthLoaded = true;

Object.assign(App, {
  async checkAuth() {
    // On cold starts (especially in CI/containerized environments) the app can briefly return 5xx
    // for auth-dependent calls (DB/Redis settling). Don't immediately treat that as "logged out".
    const maxAttempts = 5;
    for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
      try {
        const response = await API.auth.me();
        this.applyAuthEntitlements(response);
        if (this.user) {
          this.isAnonymousMode = false;
          await this.refreshNotificationCount();
          this.startNotificationPolling();
          await this.refreshPremiumAIStatus();
        }
        return;
      } catch (error) {
        const status = typeof error?.status === 'number' ? error.status : 0;
        const retryable = status === 0 || status >= 500;
        if (!retryable || attempt === maxAttempts) {
          this.user = null;
          this.isPremium = false;
          this.entitlements = {};
          this.premiumAIStatus = null;
          this.stopNotificationPolling();
          return;
        }
        // Small backoff to give the backend a chance to settle.
        await new Promise((resolve) => setTimeout(resolve, 200 * attempt));
      }
    }
  },

  requireAuth(callback) {
    if (!this.user) {
      this.navigate('/login', { skipWarning: true });
      return;
    }
    callback();
  },

  requirePremium(callback) {
    if (!this.user) {
      this.navigate('/login', { skipWarning: true });
      return;
    }
    if (!this.isPremium) {
      this.navigate('/premium?upgrade=1', { skipWarning: true });
      return;
    }
    callback();
  },

  requireFeature(feature, callback) {
    if (!this.user) {
      this.navigate('/login', { skipWarning: true });
      return;
    }
    if (!this.hasFeature(feature)) {
      this.navigate('/premium?upgrade=1', { skipWarning: true });
      return;
    }
    callback();
  },

  storePendingInviteToken(token) {
    if (!token) return;
    sessionStorage.setItem('pendingInviteToken', token);
  },

  consumePendingInviteToken() {
    const token = sessionStorage.getItem('pendingInviteToken');
    if (!token) return null;
    sessionStorage.removeItem('pendingInviteToken');
    return token;
  },

  storePostAuthNextPath(target) {
    if (!target) return;
    const normalized = this.normalizePath(target);
    let url;
    try {
      url = new URL(normalized, window.location.origin);
    } catch (error) {
      return;
    }
    if (url.origin !== window.location.origin) return;
    if (!this.isSpaPath(url.pathname)) return;
    if (url.pathname === '/login' || url.pathname === '/register') return;
    sessionStorage.setItem('postAuthNextPath', `${url.pathname}${url.search}`);
  },

  consumePostAuthNextPath() {
    const nextPath = sessionStorage.getItem('postAuthNextPath');
    if (!nextPath) return null;
    sessionStorage.removeItem('postAuthNextPath');
    return nextPath;
  },

  getOAuthNextPath() {
    const token = sessionStorage.getItem('pendingInviteToken');
    if (token) {
      return `/friend-invite/${token}`;
    }
    return '';
  },

  redirectAfterAuth(defaultPath = '/dashboard') {
    const token = this.consumePendingInviteToken();
    if (token) {
      this.navigate(`/friend-invite/${token}`, { skipWarning: true });
      return;
    }
    const nextPath = this.consumePostAuthNextPath();
    if (nextPath) {
      this.navigate(nextPath, { skipWarning: true });
      return;
    }
    this.navigate(defaultPath, { skipWarning: true });
  },


  renderHome(container) {
    container.innerHTML = `
      <div class="home-hero text-center">
        <h1 class="home-title">
          Year of <span class="text-gold">Bingo</span>
        </h1>
        <p class="home-subtitle">
          Turn your goals into an exciting game! Create a bingo card
          with 24 goals and track your progress throughout the year.
        </p>
        ${this.user ? `
          <div class="home-actions">
            <a href="/dashboard" class="btn btn-primary btn-lg">Go to Dashboard</a>
            <button class="btn btn-secondary btn-lg" data-action="show-create-card-modal">Create New Card</button>
          </div>
        ` : `
          ${AnonymousCard.exists() ? `
            <a href="/create" class="btn btn-primary btn-lg">Continue Your Card</a>
          ` : `
            <a href="/create" class="btn btn-primary btn-lg">Create Your Card</a>
          `}
          <p class="mt-md text-muted">
            Already have an account? <a href="/login">Login</a>
          </p>
        `}
      </div>
      <div class="home-features">
        <div class="card text-center">
          <div class="home-feature-icon">🎯</div>
          <h3>24 Goals</h3>
          <p>Fill your bingo card with 24 meaningful goals for the year ahead.</p>
        </div>
        <div class="card text-center">
          <div class="home-feature-icon">✨</div>
          <h3>Track Progress</h3>
          <p>Mark items complete throughout the year with a satisfying stamp.</p>
        </div>
        <div class="card text-center">
          <div class="home-feature-icon">🎉</div>
          <h3>Celebrate Wins</h3>
          <p>Get bingos, share with friends, and celebrate your achievements.</p>
        </div>
      </div>
    `;
  },

  renderLogin(container, errorMessage = null) {
    if (this.user) {
      this.navigate('/dashboard', { replace: true, skipWarning: true });
      return;
    }

    const errorMessages = {
      'invalid_link': 'This login link is invalid or has expired.',
      'link_used': 'This login link has already been used.',
      'access_denied': 'Google sign-in was cancelled.',
      'invalid_request': 'Google sign-in failed. Please try again.',
      'invalid_scope': 'Google sign-in failed. Please try again.',
      'unauthorized_client': 'Google sign-in failed. Please try again.',
      'unsupported_response_type': 'Google sign-in failed. Please try again.',
      'server_error': 'Google sign-in failed. Please try again.',
      'temporarily_unavailable': 'Google sign-in is temporarily unavailable. Please try again.',
      'oauth_error': 'Google sign-in failed. Please try again.',
      'oauth_invalid': 'Google sign-in failed. Please try again.',
      'oauth_missing': 'Google sign-in failed. Please try again.',
      'oauth_state': 'Google sign-in failed. Please try again.',
      'oauth_nonce': 'Google sign-in failed. Please try again.',
      'oauth_exchange': 'Google sign-in failed. Please try again.',
      'oauth_unverified': 'Your Google account email is not verified.',
      'oauth_link': 'Google sign-in failed. Please try again.',
    };
    const displayError = errorMessages[errorMessage] || errorMessage;
    const googleEnabled = this.googleOAuthEnabled;
    const oauthNext = googleEnabled ? this.getOAuthNextPath() : '';
    const googleUrl = googleEnabled && oauthNext ? `/api/auth/google/start?next=${encodeURIComponent(oauthNext)}` : '/api/auth/google/start';
    const googleButton = googleEnabled ? `
          <a href="${googleUrl}" class="btn btn-google btn-lg btn-full mb-md">
            <img class="google-icon" src="/static/img/google-g.svg" alt="" aria-hidden="true">
            Continue with Google
          </a>
    ` : '';

    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card">
          <div class="auth-header">
            <h2 class="auth-title">Welcome Back</h2>
            <p class="text-muted">Sign in to your account</p>
          </div>
          ${displayError ? `<div class="form-error mb-md">${this.escapeHtml(displayError)}</div>` : ''}
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
            <button type="submit" class="btn btn-primary btn-lg btn-full">
              Sign In
            </button>
          </form>
          <div class="text-center my-md">
            <a href="/forgot-password" class="text-muted">Forgot password?</a>
          </div>
          <div class="auth-divider">
            <span>or</span>
          </div>
          ${googleButton}
          <a href="/magic-link" class="btn btn-secondary btn-lg btn-full mb-md">
            Sign in with email link
          </a>
          <div class="auth-footer">
            Don't have an account? <a href="/register">Sign up</a>
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
        this.applyAuthEntitlements(response);
        this.setupNavigation();
        await this.refreshNotificationCount();
        this.startNotificationPolling();
        this.redirectAfterAuth('/dashboard');
        this.toast('Welcome back!', 'success');
      } catch (error) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      }
    });
  },

  renderRegister(container) {
    if (this.user) {
      this.navigate('/dashboard', { replace: true, skipWarning: true });
      return;
    }

    const googleEnabled = this.googleOAuthEnabled;
    const oauthNext = googleEnabled ? this.getOAuthNextPath() : '';
    const googleUrl = googleEnabled && oauthNext ? `/api/auth/google/start?next=${encodeURIComponent(oauthNext)}` : '/api/auth/google/start';
    const googleBlock = googleEnabled ? `
          <div class="auth-divider">
            <span>or</span>
          </div>
          <a href="${googleUrl}" class="btn btn-google btn-lg btn-full mb-md">
            <img class="google-icon" src="/static/img/google-g.svg" alt="" aria-hidden="true">
            Continue with Google
          </a>
    ` : '';

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
            <button type="submit" class="btn btn-primary btn-lg btn-full">
              Create Account
            </button>
          </form>
          ${googleBlock}
          <div class="auth-footer">
            Already have an account? <a href="/login">Sign in</a>
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
        this.applyAuthEntitlements(response);
        this.setupNavigation();
        await this.refreshNotificationCount();
        this.startNotificationPolling();
        this.redirectAfterAuth('/create');
        this.toast('Account created! Check your email to verify your account.', 'success');
      } catch (error) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      }
    });
  },

  renderGoogleComplete(container) {
    if (this.user) {
      this.navigate('/dashboard', { replace: true, skipWarning: true });
      return;
    }

    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card">
          <div class="auth-header">
            <h2 class="auth-title">Complete Your Signup</h2>
            <p class="text-muted">Pick a username to finish creating your account</p>
          </div>
          <form id="google-complete-form">
            <div class="form-group">
              <label class="form-label" for="google-username">Username</label>
              <input type="text" id="google-username" class="form-input" required minlength="2" maxlength="100">
            </div>
            <div class="form-group">
              <label class="checkbox-label">
                <input type="checkbox" id="google-searchable">
                <span>Allow others to find me by username</span>
              </label>
              <small class="text-muted">You can change this later in your account settings</small>
            </div>
            <div id="google-complete-error" class="form-error hidden"></div>
            <button type="submit" class="btn btn-primary btn-lg btn-full">
              Finish Signup
            </button>
          </form>
        </div>
      </div>
    `;

    document.getElementById('google-complete-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const username = document.getElementById('google-username').value;
      const searchable = document.getElementById('google-searchable').checked;
      const errorEl = document.getElementById('google-complete-error');

      try {
        const response = await API.auth.providerComplete('google', username, searchable);
        this.applyAuthEntitlements(response);
        this.setupNavigation();
        await this.refreshNotificationCount();
        this.startNotificationPolling();
        this.redirectAfterAuth(response.next || '/dashboard');
        this.toast('Account created! Welcome!', 'success');
      } catch (error) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      }
    });
  },

  // Magic Link Authentication
  renderMagicLinkRequest(container) {
    if (this.user) {
      this.navigate('/dashboard', { replace: true, skipWarning: true });
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
            <button type="submit" class="btn btn-primary btn-lg btn-full">
              Send login link
            </button>
          </form>
          <div class="auth-footer">
            <a href="/login">Back to sign in</a>
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
        this.navigate(`/check-email?type=magic-link&email=${encodeURIComponent(email)}`, { skipWarning: true });
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
          <div class="spinner spinner--spaced"></div>
          <p>Signing you in...</p>
        </div>
      </div>
    `;

    try {
      const response = await API.auth.verifyMagicLink(token);
      this.applyAuthEntitlements(response);
      this.setupNavigation();
      this.redirectAfterAuth('/dashboard');
      this.toast('Welcome back!', 'success');
    } catch (error) {
      this.navigate(`/login?error=${encodeURIComponent(error.message)}`, { replace: true, skipWarning: true });
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
            <button type="submit" class="btn btn-primary btn-lg btn-full">
              Send reset link
            </button>
          </form>
          <div class="auth-footer">
            <a href="/login">Back to sign in</a>
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
        this.navigate(`/check-email?type=reset&email=${encodeURIComponent(email)}`, { skipWarning: true });
      } catch (error) {
        // Still redirect even on error to prevent email enumeration
        this.navigate(`/check-email?type=reset&email=${encodeURIComponent(email)}`, { skipWarning: true });
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
            <a href="/forgot-password" class="btn btn-primary mt-md">Request new link</a>
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
            <button type="submit" class="btn btn-primary btn-lg btn-full">
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
        this.applyAuthEntitlements(response);
        this.setupNavigation();
        this.navigate('/dashboard', { skipWarning: true });
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
            ${this.user ? `<a href="/dashboard" class="btn btn-primary mt-md">Go to Dashboard</a>` : `<a href="/login" class="btn btn-primary mt-md">Sign In</a>`}
          </div>
        </div>
      `;
      return;
    }

    container.innerHTML = `
      <div class="auth-page">
        <div class="card auth-card text-center">
          <div class="spinner spinner--spaced"></div>
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
            <div class="status-icon">✓</div>
            <h2>Email Verified!</h2>
            <p class="text-muted">Your email has been verified successfully.</p>
            ${this.user ? `<a href="/dashboard" class="btn btn-primary mt-md">Go to Dashboard</a>` : `<a href="/login" class="btn btn-primary mt-md">Sign In</a>`}
          </div>
        </div>
      `;
    } catch (error) {
      container.innerHTML = `
        <div class="auth-page">
          <div class="card auth-card text-center">
            <div class="status-icon">✗</div>
            <h2>Verification Failed</h2>
            <p class="text-muted" id="verify-error-message"></p>
            ${this.user ? `
              <button class="btn btn-primary mt-md" data-action="resend-verification">
                Resend Verification Email
              </button>
            ` : `<a href="/login" class="btn btn-primary mt-md">Sign In</a>`}
          </div>
        </div>
      `;
      const errorEl = document.getElementById('verify-error-message');
      if (errorEl) errorEl.textContent = error.message;
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
          <div class="status-icon">✉️</div>
          <h2>${msg.title}</h2>
          <p class="text-muted">
            ${msg.description}<br>
            <strong>${email ? this.escapeHtml(email) : 'your email'}</strong>
          </p>
          <p class="text-muted mt-md">
            ${msg.detail}
          </p>
          <div class="mt-lg">
            <a href="/login" class="btn btn-ghost">Back to sign in</a>
          </div>
        </div>
      </div>
    `;
  },

  // Email verification banner for dashboard
  renderEmailVerificationBanner() {
    if (!this.user || this.user.email_verified) return '';
    const freeLimit = 5;
    const used = typeof this.user.ai_free_generations_used === 'number' ? this.user.ai_free_generations_used : 0;
    const remaining = Math.max(0, freeLimit - used);
    return `
      <div class="verification-banner">
        <div>
          <strong class="verification-banner-title">Please verify your email</strong>
          <span class="verification-banner-subtitle"> to enable all features.</span>
          <div class="text-muted verification-banner-detail">
            AI Goal Wizard: <strong>${remaining}</strong> free generations left before verification is required.
          </div>
        </div>
        <button class="btn btn-secondary btn-sm" data-action="resend-verification">
          Resend verification email
        </button>
      </div>
    `;
  },

  async exportAccountData(button) {
    if (!this.user) return;
    const actionButton = button || document.querySelector('[data-action="export-account"]');

    try {
      if (actionButton) this.setButtonLoading(actionButton, true);
      const blob = await API.account.export();
      const timestamp = new Date().toISOString().slice(0, 10);
      this.downloadBlob(blob, `yearofbingo_account_export_${timestamp}.zip`);
      this.toast('Export downloaded', 'success');
    } catch (error) {
      this.toast(error.message || 'Unable to export account data', 'error');
    } finally {
      if (actionButton) this.setButtonLoading(actionButton, false);
    }
  },

  openDeleteAccountModal() {
    if (!this.user) return;
    const username = this.escapeHtml(this.user.username);

    this.openModal('Delete Account', `
      <div class="danger-zone__modal">
        <div class="danger-zone__banner">
          <strong>This will permanently delete your account and all associated data. This cannot be undone.</strong>
        </div>
        <ul class="danger-zone__list">
          <li>Cards and items</li>
          <li>Friends and friend requests</li>
          <li>Reminders and notifications</li>
          <li>API tokens and share links</li>
        </ul>
        <form data-action="delete-account" class="profile-form" id="delete-account-form">
          <div class="form-group">
            <label for="delete-account-username">Type "${username}" to confirm</label>
            <input type="text" id="delete-account-username" class="form-input" autocomplete="off" required>
          </div>
          <div class="form-group">
            <label for="delete-account-password">Enter your password</label>
            <input type="password" id="delete-account-password" class="form-input" autocomplete="current-password" required>
          </div>
          <label class="checkbox-label">
            <input type="checkbox" id="delete-account-confirm">
            <span>I understand this action is permanent and cannot be undone.</span>
          </label>
          <div class="form-error hidden" id="delete-account-error"></div>
          <div class="modal-actions">
            <button type="button" class="btn btn-ghost" data-action="close-modal">Cancel</button>
            <button type="submit" class="btn btn-danger" id="delete-account-submit" disabled>Delete Account</button>
          </div>
        </form>
      </div>
    `);

    const usernameInput = document.getElementById('delete-account-username');
    const passwordInput = document.getElementById('delete-account-password');
    const confirmInput = document.getElementById('delete-account-confirm');

    const update = () => this.updateDeleteAccountModalState();
    usernameInput?.addEventListener('input', update);
    passwordInput?.addEventListener('input', update);
    confirmInput?.addEventListener('change', update);

    this.updateDeleteAccountModalState();
  },

  updateDeleteAccountModalState() {
    const usernameInput = document.getElementById('delete-account-username');
    const passwordInput = document.getElementById('delete-account-password');
    const confirmInput = document.getElementById('delete-account-confirm');
    const submitButton = document.getElementById('delete-account-submit');
    const errorEl = document.getElementById('delete-account-error');

    if (!usernameInput || !passwordInput || !confirmInput || !submitButton) return;

    const usernameMatch = this.matchesDeleteAccountConfirmation(this.user?.username || '', usernameInput.value);
    const hasPassword = passwordInput.value.length > 0;
    const confirmed = confirmInput.checked;

    submitButton.disabled = !(usernameMatch && hasPassword && confirmed);

    if (!usernameMatch && usernameInput.value.trim().length > 0) {
      if (errorEl) {
        errorEl.textContent = 'Username confirmation must match exactly.';
        errorEl.classList.remove('hidden');
      }
      return;
    }

    if (errorEl) {
      errorEl.textContent = '';
      errorEl.classList.add('hidden');
    }
  },

  matchesDeleteAccountConfirmation(username, inputValue) {
    return inputValue.trim() === username;
  },

  async handleDeleteAccount(event, form) {
    event.preventDefault();
    if (!this.user || !form) return;

    const usernameInput = form.querySelector('#delete-account-username');
    const passwordInput = form.querySelector('#delete-account-password');
    const confirmInput = form.querySelector('#delete-account-confirm');
    const submitButton = form.querySelector('#delete-account-submit');
    const errorEl = document.getElementById('delete-account-error');

    const confirmUsername = usernameInput?.value || '';
    const password = passwordInput?.value || '';
    const confirmed = confirmInput?.checked || false;

    if (errorEl) {
      errorEl.textContent = '';
      errorEl.classList.add('hidden');
    }

    if (!this.matchesDeleteAccountConfirmation(this.user.username, confirmUsername)) {
      if (errorEl) {
        errorEl.textContent = 'Username confirmation must match exactly.';
        errorEl.classList.remove('hidden');
      }
      return;
    }

    if (!password) {
      if (errorEl) {
        errorEl.textContent = 'Password is required.';
        errorEl.classList.remove('hidden');
      }
      return;
    }

    if (!confirmed) {
      if (errorEl) {
        errorEl.textContent = 'Please confirm that you understand the consequences.';
        errorEl.classList.remove('hidden');
      }
      return;
    }

    try {
      if (submitButton) this.setButtonLoading(submitButton, true);
        await API.account.delete(confirmUsername.trim(), password);
        this.closeModal();
        this.user = null;
        this.isPremium = false;
        this.entitlements = {};
        this.billingStatus = null;
        this.notificationSettings = null;
        this.notificationUnreadCount = 0;
        this.stopNotificationPolling();
      this.setupNavigation();
      sessionStorage.removeItem('pendingInviteToken');
      this.navigate('/', { skipWarning: true });
      this.toast('Account deleted', 'success');
    } catch (error) {
      if (errorEl) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      } else {
        this.toast(error.message, 'error');
      }
    } finally {
      if (submitButton) this.setButtonLoading(submitButton, false);
    }
  },

  // Get display name for a card (title if set, otherwise "YYYY Bingo Card")

  renderProfile(container) {
    const verifiedBadge = this.user.email_verified
      ? '<span class="badge badge-success">Verified</span>'
      : '<span class="badge badge-warning">Not verified</span>';

    const premiumBadge = this.isPremium
      ? '<span class="badge badge-premium">Premium</span>'
      : '';

    const verificationSection = this.user.email_verified
      ? ''
      : `
        <div class="profile-alert">
          <p><strong>Your email is not verified.</strong> Please check your inbox for the verification email.</p>
          <button class="btn btn-secondary btn-sm" data-action="resend-verification">Resend verification email</button>
        </div>
      `;

    container.innerHTML = `
      <div class="profile-page">
        <div class="profile-header">
          <a href="/dashboard" class="btn btn-ghost">&larr; Back</a>
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
                <span>${this.escapeHtml(this.user.username)} <span id="premium-badge-slot">${premiumBadge}</span></span>
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

          <div class="card profile-section" id="billing-section">
            <h3>Plan</h3>
            <div id="billing-status" class="billing-status">
              <div class="text-center"><div class="spinner spinner--small"></div></div>
            </div>
            <p id="ai-enhancements-status" class="text-muted text-sm mt-md"></p>
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
            <h3>Notifications</h3>
            <div id="notification-settings" class="notification-settings">
              <div class="text-center"><div class="spinner spinner--small"></div></div>
            </div>
          </div>

          <div class="card profile-section">
            <h3>Reminders</h3>
            <div id="reminder-settings" class="reminder-settings">
              <div class="text-center"><div class="spinner spinner--small"></div></div>
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
            <h3>API Tokens</h3>
            <div class="profile-tokens">
              <p class="text-muted mb-md">
                Create API tokens to access your data programmatically.
                <a href="/api/docs" target="_blank">View API Documentation</a>
              </p>
              <button class="btn btn-secondary btn-sm" data-action="show-create-token-modal">Create New Token</button>
              <div id="api-tokens-list" class="tokens-list">
                <div class="text-center"><div class="spinner spinner--small"></div></div>
              </div>
            </div>
          </div>

          <div class="card profile-section">
            <h3>Account Actions</h3>
            <div class="profile-actions">
              <button class="btn btn-ghost" data-action="logout">Sign Out</button>
            </div>
          </div>

          <div class="card profile-section">
            <h3>Export Your Data</h3>
            <p class="text-muted">
              Download a ZIP of CSV files containing your account data (cards, items, friends, reminders, notifications, etc.).
            </p>
            <button class="btn btn-secondary btn-sm" data-action="export-account">Download Export</button>
          </div>

          <div class="card profile-section danger-zone">
            <h3>Danger Zone: Delete Account</h3>
            <p class="danger-zone__warning">
              Permanently delete your account and all associated data. This cannot be undone.
            </p>
            <button class="btn btn-danger" data-action="open-delete-account-modal">Delete Account</button>
          </div>
        </div>
      </div>
    `;

    this.setupProfileEvents();
    this.loadNotificationSettings();
    this.loadReminderSettings();
    this.loadApiTokens();
    this.loadBillingStatus();
    this.handleBillingReturn();
  },

  setupProfileEvents() {
    const form = document.getElementById('change-password-form');
    const errorEl = document.getElementById('password-error');

    // Privacy toggle
    const searchableToggle = document.getElementById('searchable-toggle');
    searchableToggle.addEventListener('change', async (e) => {
      try {
        const response = await API.auth.updateSearchable(e.target.checked);
        this.applyAuthEntitlements(response);
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

  async logout() {
    if (this.shouldWarnUnfinalizedCardNavigation()) {
      this.confirmLogoutUnfinalizedCard();
      return;
    }
    await this.confirmedLogout();
  },

  confirmLogoutUnfinalizedCard() {
    this.openModal('Draft Saved', `
      <div class="finalize-confirm-modal">
        <p class="mb-lg">
          Your card is saved as a draft. If you log out now, you can pick up where you left off when you sign back in. Finalizing locks the layout so you can start tracking completion.
        </p>
        <div class="flex gap-md justify-end flex-wrap">
          <button class="btn btn-ghost" data-action="close-modal">Stay</button>
          <button class="btn btn-secondary" data-action="confirmed-logout">Log Out</button>
          <button class="btn btn-primary" data-action="open-finalize-from-navigation-warning">Finalize Card</button>
        </div>
      </div>
    `);
  },

  async confirmedLogout() {
    try {
      this.closeModal();
      await API.auth.logout();
      this.user = null;
      this.isPremium = false;
      this.entitlements = {};
      this.billingStatus = null;
      this.notificationSettings = null;
      this.notificationUnreadCount = 0;
      this.stopNotificationPolling();
      this.setupNavigation();
      sessionStorage.removeItem('pendingInviteToken');
      this.navigate('/', { skipWarning: true });
      this.toast('Logged out successfully', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async loadApiTokens() {
    const listEl = document.getElementById('api-tokens-list');
    if (!listEl) return;

    try {
      const response = await API.tokens.list();
      const tokens = response.tokens || [];
      const scopeClasses = {
        read: 'scope-read',
        write: 'scope-write',
        read_write: 'scope-read_write',
      };
      const scopeLabels = {
        read: 'read',
        write: 'write',
        read_write: 'read & write',
      };

      if (tokens.length === 0) {
        listEl.innerHTML = '<p class="text-muted mt-md">No active tokens.</p>';
        return;
      }

      listEl.innerHTML = tokens.map(token => {
        const scopeClass = scopeClasses[token.scope] || 'scope-unknown';
        const scopeLabel = scopeLabels[token.scope] || 'unknown';
        return `
          <div class="token-item">
            <div class="token-info">
              <div class="fw-medium">${this.escapeHtml(token.name)}</div>
              <div class="token-meta text-muted text-sm">
                <code>${this.escapeHtml(token.token_prefix)}...</code>
                <span>•</span>
                <span class="token-scope ${scopeClass}">${this.escapeHtml(scopeLabel)}</span>
                <span>•</span>
                <span>${token.expires_at ? 'Expires ' + new Date(token.expires_at).toLocaleDateString() : 'Never expires'}</span>
              </div>
              <div class="token-meta text-muted text-sm">
                Last used: ${token.last_used_at ? new Date(token.last_used_at).toLocaleString() : 'Never'}
              </div>
            </div>
            <button class="btn btn-ghost btn-sm btn-ghost-danger" data-action="delete-token" data-token-id="${this.escapeHtml(token.id)}" title="Revoke Token">
              <i class="fas fa-trash"></i>
            </button>
          </div>
        `;
      }).join('');

      // Add Revoke All button if tokens exist
      if (tokens.length > 1) {
        listEl.innerHTML += `
          <div class="mt-md text-right">
            <button class="btn btn-ghost btn-sm btn-ghost-danger" data-action="revoke-all-tokens">Revoke All Tokens</button>
          </div>
        `;
      }
    } catch (error) {
      listEl.innerHTML = '<p class="text-muted text-danger" id="tokens-error"></p>';
      const errorEl = document.getElementById('tokens-error');
      if (errorEl) errorEl.textContent = `Failed to load tokens: ${error.message}`;
    }
  },

    showCreateTokenModal() {
      this.openModal('Create API Token', `
        <form data-action="create-token">
        <div class="form-group">
          <label for="token-name">Name</label>
          <input type="text" id="token-name" class="form-input" required placeholder="e.g., Backup Script" maxlength="100">
        </div>
        <div class="form-group">
          <label for="token-scope">Permissions</label>
          <select id="token-scope" class="form-input">
            <option value="read">Read Only</option>
            <option value="write">Write Only</option>
            <option value="read_write">Read & Write</option>
          </select>
        </div>
        <div class="form-group">
          <label for="token-expiry">Expiration</label>
          <select id="token-expiry" class="form-input">
            <option value="30">30 Days</option>
            <option value="7">7 Days</option>
            <option value="90">3 months</option>
            <option value="365">1 year</option>
            <option value="0">Never</option>
          </select>
        </div>
          <div class="flex gap-md justify-end">
            <button type="button" class="btn btn-ghost" data-action="close-modal">Cancel</button>
            <button type="submit" class="btn btn-primary">Generate Token</button>
          </div>
        </form>
      `);
    },

  async handleCreateToken(event) {
    event.preventDefault();
    const name = document.getElementById('token-name').value;
    const scope = document.getElementById('token-scope').value;
    const expiry = document.getElementById('token-expiry').value;

    try {
      const response = await API.tokens.create(name, scope, expiry);
      this.closeModal();
      this.showTokenCreatedModal(response.token, response.token_metadata);
      this.loadApiTokens(); // Refresh list if visible
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

    showTokenCreatedModal(token, meta) {
      this.openModal('Token Generated', `
        <div class="token-created-modal">
          <p><strong>Save this token now!</strong> You won't be able to see it again.</p>
          <div class="token-display">
            <code id="new-token" class="break-all">${this.escapeHtml(token)}</code>
            <button class="btn btn-secondary btn-sm" data-action="copy-new-token">Copy</button>
          </div>
          <p class="text-muted mt-md text-sm">
            Use this token in the <code>Authorization</code> header:
            <br>
            <code class="block surface-2 p-sm mt-sm rounded-sm">Authorization: Bearer ${this.escapeHtml(token.substring(0, 10))}...</code>
          </p>
          <div class="mt-lg text-right">
            <button class="btn btn-primary" data-action="token-modal-done">Done</button>
          </div>
        </div>
      `);
    },

  copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
      this.toast('Copied to clipboard', 'success');
    }).catch(() => {
      this.toast('Failed to copy', 'error');
    });
  },

  async deleteToken(id) {
    if (!confirm('Revoke this token? Any scripts using it will stop working.')) return;
    try {
      await API.tokens.delete(id);
      this.toast('Token revoked', 'success');
      this.loadApiTokens();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async revokeAllTokens() {
    if (!confirm('Revoke ALL API tokens? This cannot be undone.')) return;
    try {
      await API.tokens.deleteAll();
      this.toast('All tokens revoked', 'success');
      this.loadApiTokens();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },
});
}
