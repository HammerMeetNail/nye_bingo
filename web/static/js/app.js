// Year of Bingo - Main Application

const App = {
  user: null,
  currentCard: null,
  suggestions: [],
  usedSuggestions: new Set(),
  allowedEmojis: ['🎉', '👏', '🔥', '❤️', '⭐'],
  isLoading: false,
  isAnonymousMode: false, // True when editing an anonymous card (localStorage)
  currentView: null,
  notificationSettings: null,
  notificationUnreadCount: 0,
  notificationPoller: null,
  reminderSettings: null,
  reminderCards: [],
  goalReminders: [],
  goalRemindersByItem: {},
  reminderSelectedCardId: null,
  modalScrollY: null,
  isSharedView: false,
  currentShareStatus: null,
  _lastHash: '',
  _pendingNavigationHash: null,
  _revertingHashChange: false,
  _allowNextHashRoute: false,

  async init() {
    await API.init();
    await this.checkAuth();
    this.setupActionDelegation();
    this.setupNavigation();
    this.setupModal();
    this.setupOfflineDetection();
    this._lastHash = window.location.hash || '#home';
    this.route();
  },

  qs(id) {
    return document.getElementById(id);
  },

  setText(el, text) {
    if (!el) return;
    el.textContent = text ?? '';
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

  setupActionDelegation() {
    document.addEventListener('click', (event) => {
      const stopEl = event.target.closest ? event.target.closest('[data-stop-propagation]') : null;
      if (stopEl) event.stopPropagation();

      const actionEl = event.target.closest ? event.target.closest('[data-action]') : null;
      if (!actionEl) return;
      if (actionEl.classList.contains('dropdown-item--disabled')) return;
      const ariaDisabled = actionEl.getAttribute('aria-disabled');
      const ariaDisabledProp = actionEl.ariaDisabled;
      if (actionEl.disabled || ariaDisabled === 'true' || ariaDisabledProp === 'true') return;
      const action = actionEl.dataset.action;
      if (!action) return;
      this.handleActionClick(action, actionEl, event);
    });

    document.addEventListener('submit', (event) => {
      const form = event.target.closest ? event.target.closest('form[data-action]') : null;
      if (!form) return;
      const action = form.dataset.action;
      if (!action) return;
      this.handleActionSubmit(action, form, event);
    });

    document.addEventListener('change', (event) => {
      const target = event.target.closest ? event.target.closest('[data-change-action]') : null;
      if (!target) return;
      const action = target.dataset.changeAction;
      if (!action) return;
      this.handleActionChange(action, target, event);
    });
  },

  handleActionClick(action, target, event) {
    switch (action) {
      case 'close-modal':
        this.closeModal();
        break;
      case 'proceed-pending-navigation':
        this.proceedPendingNavigation();
        break;
      case 'open-finalize-from-navigation-warning':
        this.openFinalizeFromNavigationWarning();
        break;
      case 'toggle-mobile-menu':
        this.toggleMobileMenu();
        break;
      case 'logout':
        this.logout();
        break;
      case 'mark-notification-read':
        this.markNotificationRead(target);
        break;
      case 'mark-all-notifications-read':
        this.markAllNotificationsRead();
        break;
      case 'delete-notification':
        this.deleteNotification(target);
        break;
      case 'delete-all-notifications':
        this.deleteAllNotifications();
        break;
      case 'save-card-checkin':
        this.saveCardCheckin();
        break;
      case 'apply-card-checkin-all':
        this.applyCardCheckinToAll();
        break;
      case 'delete-card-checkin':
        this.deleteCardCheckin();
        break;
      case 'send-reminder-test':
        this.sendReminderTest();
        break;
      case 'set-goal-reminder':
        this.setGoalReminder(target);
        break;
      case 'delete-goal-reminder':
        this.deleteGoalReminder(target);
        break;
      case 'confirmed-logout':
        this.confirmedLogout();
        break;
      case 'open-ai-wizard': {
        const cardId = target.dataset.cardId || null;
        const desiredCount = target.dataset.desiredCount;
        AIWizard.open(cardId || null, desiredCount ? parseInt(desiredCount, 10) : null);
        break;
      }
      case 'open-ai-wizard-from-modal':
        this.closeModal();
        AIWizard.open();
        break;
      case 'ai-create-card':
        AIWizard.createCard();
        break;
      case 'ai-add-to-card':
        AIWizard.addToCard();
        break;
      case 'show-create-card-modal':
        this.showCreateCardModal();
        break;
      case 'resend-verification':
        this.resendVerification();
        break;
      case 'resend-verification-and-route':
        this.resendVerification();
        window.location.hash = `#check-email?type=verification&email=${encodeURIComponent(this.user?.email || '')}`;
        break;
      case 'select-all-cards':
        this.selectAllCards();
        break;
      case 'deselect-all-cards':
        this.deselectAllCards();
        break;
      case 'bulk-archive':
        this.bulkSetArchive(true);
        break;
      case 'bulk-unarchive':
        this.bulkSetArchive(false);
        break;
      case 'bulk-visible':
        this.bulkSetVisibility(true);
        break;
      case 'bulk-private':
        this.bulkSetVisibility(false);
        break;
      case 'bulk-delete':
        this.bulkDeleteCards();
        break;
      case 'export-cards':
        this.exportSelectedCards();
        break;
      case 'delete-card':
        if (target.dataset.cardId) this.deleteCard(target.dataset.cardId);
        break;
      case 'show-ai-auth-modal':
        this.showAIAuthModal();
        break;
      case 'edit-card-meta':
        if (this.isAnonymousMode) {
          this.showEditAnonymousCardMetaModal();
        } else {
          this.showEditCardMetaModal();
        }
        break;
      case 'toggle-card-visibility': {
        const cardId = target.dataset.cardId;
        const visible = target.dataset.visible === 'true';
        if (cardId) this.toggleCardVisibility(cardId, visible);
        break;
      }
      case 'confirm-clear-card-items':
        this.confirmClearCardItems();
        break;
      case 'shuffle-card':
        this.shuffleCard();
        break;
      case 'show-clone-card-modal':
        this.showCloneCardModal();
        break;
      case 'open-share-modal':
        this.showShareCardModal();
        break;
      case 'enable-share':
        this.enableShare();
        break;
      case 'regenerate-share':
        this.regenerateShare();
        break;
      case 'disable-share':
        this.disableShare();
        break;
      case 'copy-share-link':
        this.copyShareLink();
        break;
      case 'finalize-card':
        this.finalizeCard();
        break;
      case 'fill-empty-spaces':
        this.fillEmptySpaces();
        break;
      case 'confirm-delete-anonymous-card':
        this.confirmDeleteAnonymousCard();
        break;
      case 'clear-card-items':
        this.clearCardItems();
        break;
      case 'add-suggestion':
        this.addSuggestion(target);
        break;
      case 'uncomplete-item': {
        const position = parseInt(target.dataset.position, 10);
        if (!Number.isNaN(position)) this.uncompleteItem(position);
        break;
      }
      case 'ai-refine': {
        const position = parseInt(target.dataset.position, 10);
        if (!Number.isNaN(position)) this.handleAIRefine(position);
        break;
      }
      case 'remove-item': {
        const position = parseInt(target.dataset.position, 10);
        if (!Number.isNaN(position)) this.removeItem(position);
        break;
      }
      case 'confirm-finalize':
        this.confirmFinalize();
        break;
      case 'show-finalize-register-form':
        this.showFinalizeRegisterForm();
        break;
      case 'show-finalize-login-form':
        this.showFinalizeLoginForm();
        break;
      case 'show-finalize-auth-modal':
        this.showFinalizeAuthModal();
        break;
      case 'conflict-keep-existing':
        if (target.dataset.cardId) this.handleConflictKeepExisting(target.dataset.cardId);
        break;
      case 'conflict-save-as-new':
        this.handleConflictSaveAsNew();
        break;
      case 'conflict-replace':
        if (target.dataset.cardId) this.handleConflictReplace(target.dataset.cardId);
        break;
      case 'import-anonymous-card':
        this.importAnonymousCard();
        break;
      case 'create-conflict-go-to-existing':
        if (target.dataset.cardId) this.handleCreateConflictGoToExisting(target.dataset.cardId);
        break;
      case 'create-conflict-save-as-new':
        this.handleCreateConflictSaveAsNew();
        break;
      case 'create-conflict-replace':
        if (target.dataset.cardId) this.handleCreateConflictReplace(target.dataset.cardId);
        break;
      case 'send-friend-request':
        if (target.dataset.userId) this.sendFriendRequest(target.dataset.userId);
        break;
      case 'copy-invite-link': {
        const input = document.getElementById('invite-link-input');
        if (input?.value) this.copyInviteLink(input.value);
        break;
      }
      case 'revoke-invite':
        if (target.dataset.inviteId) this.revokeInvite(target.dataset.inviteId);
        break;
      case 'accept-request':
        if (target.dataset.requestId) this.acceptRequest(target.dataset.requestId);
        break;
      case 'reject-request':
        if (target.dataset.requestId) this.rejectRequest(target.dataset.requestId);
        break;
      case 'cancel-request':
        if (target.dataset.requestId) this.cancelRequest(target.dataset.requestId);
        break;
      case 'remove-friend': {
        const friendName = target.closest('.friend-item')?.querySelector('strong')?.textContent?.trim() || 'this user';
        if (target.dataset.friendshipId) this.removeFriend(target.dataset.friendshipId, friendName);
        break;
      }
      case 'block-user': {
        const friendName = target.closest('.friend-item')?.querySelector('strong')?.textContent?.trim() || 'this user';
        if (target.dataset.otherUserId) this.blockUser(target.dataset.otherUserId, friendName);
        break;
      }
      case 'unblock-user': {
        const friendName = target.closest('.friend-item')?.querySelector('strong')?.textContent?.trim() || 'this user';
        if (target.dataset.userId) this.unblockUser(target.dataset.userId, friendName);
        break;
      }
      case 'react-item':
        if (target.dataset.itemId && target.dataset.emoji) {
          this.reactToItem(target.dataset.itemId, target.dataset.emoji);
        }
        break;
      case 'remove-reaction':
        if (target.dataset.itemId) this.removeReaction(target.dataset.itemId);
        break;
      case 'show-create-token-modal':
        this.showCreateTokenModal();
        break;
      case 'delete-token':
        if (target.dataset.tokenId) this.deleteToken(target.dataset.tokenId);
        break;
      case 'revoke-all-tokens':
        this.revokeAllTokens();
        break;
      case 'copy-new-token': {
        const tokenEl = document.getElementById('new-token');
        if (tokenEl?.textContent) this.copyToClipboard(tokenEl.textContent);
        break;
      }
      case 'token-modal-done':
        this.closeModal();
        this.loadApiTokens();
        break;
      default:
        break;
    }
  },

  handleActionSubmit(action, form, event) {
    switch (action) {
      case 'create-card-modal':
        this.handleCreateCardModal(event);
        break;
      case 'save-card-meta':
        this.saveCardMeta(event);
        break;
      case 'save-anon-card-meta':
        this.saveAnonymousCardMeta(event);
        break;
      case 'create-card-anon':
        this.handleAnonymousCreateCard(event);
        break;
      case 'create-card':
        this.handleCreateCard(event);
        break;
      case 'save-item-edit': {
        const position = parseInt(form.dataset.position, 10);
        if (!Number.isNaN(position)) this.saveItemEdit(event, position);
        break;
      }
      case 'clone-card':
        this.handleCloneCard(event);
        break;
      case 'finalize-register':
        this.handleFinalizeRegister(event);
        break;
      case 'finalize-login':
        this.handleFinalizeLogin(event);
        break;
      case 'conflict-save-as-new-submit':
        this.handleConflictSaveAsNewSubmit(event);
        break;
      case 'create-conflict-save-as-new-submit':
        this.handleCreateConflictSaveAsNewSubmit(event);
        break;
      case 'create-token':
        this.handleCreateToken(event);
        break;
      case 'ai-generate':
        AIWizard.handleGenerate(event);
        break;
      default:
        break;
    }
  },

  handleActionChange(action, target, event) {
    switch (action) {
      case 'dashboard-sort':
        this.changeDashboardSort(target.value);
        break;
      case 'dashboard-selection':
        this.updateDashboardSelection();
        break;
      case 'friend-card-select':
        this.switchFriendCard(target.value);
        break;
      case 'notification-master-toggle':
        this.handleNotificationMasterToggle(target);
        break;
      case 'notification-scenario-toggle':
        this.handleNotificationScenarioToggle(target);
        break;
      case 'reminder-master-toggle':
        this.handleReminderMasterToggle(target);
        break;
      case 'reminder-card-select':
        this.handleReminderCardSelect(target);
        break;
      default:
        break;
    }
  },

  shouldWarnUnfinalizedCardNavigation() {
    if (!this.currentCard) return false;
    if (this.currentCard.is_finalized) return false;
    if (this.currentView !== 'card-editor') return false;
    const itemCount = this.currentCard.items ? this.currentCard.items.length : 0;
    const capacity = this.getCardCapacity(this.currentCard);
    return capacity > 0 && itemCount >= capacity;
  },

  handleHashChange() {
    const newHash = window.location.hash || '#home';

    if (this._revertingHashChange) {
      this._revertingHashChange = false;
      this._lastHash = newHash;
      return;
    }

    if (this._allowNextHashRoute) {
      this._allowNextHashRoute = false;
      this._lastHash = newHash;
      this.route();
      return;
    }

    const oldHash = this._lastHash || '#home';
    if (this.shouldWarnUnfinalizedCardNavigation() && newHash !== oldHash) {
      this._pendingNavigationHash = newHash;
      this._revertingHashChange = true;
      window.location.hash = oldHash;
      this.showUnfinalizedCardNavigationModal();
      return;
    }

    this._lastHash = newHash;
    this.route();
  },

  showUnfinalizedCardNavigationModal() {
    this.openModal('Draft Saved', `
      <div class="finalize-confirm-modal">
        <p style="margin-bottom: 1.5rem;">
          Your card is saved as a draft. You can leave and come back later without losing your goals. Finalizing locks the layout so you can start tracking completion.
        </p>
        <div style="display: flex; gap: 1rem; justify-content: flex-end; flex-wrap: wrap;">
          <button class="btn btn-ghost" data-action="close-modal">Stay</button>
          <button class="btn btn-secondary" data-action="proceed-pending-navigation">Leave</button>
          <button class="btn btn-primary" data-action="open-finalize-from-navigation-warning">Finalize Card</button>
        </div>
      </div>
    `);
  },

  openFinalizeFromNavigationWarning() {
    this.closeModal();
    this.finalizeCard();
  },

  proceedPendingNavigation() {
    const target = this._pendingNavigationHash;
    this._pendingNavigationHash = null;
    this.closeModal();
    if (!target) return;
    this._allowNextHashRoute = true;
    window.location.hash = target;
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
      if (this.user) {
        this.isAnonymousMode = false;
        await this.refreshNotificationCount();
        this.startNotificationPolling();
      }
    } catch (error) {
      this.user = null;
      this.stopNotificationPolling();
    }
  },

  setupNavigation() {
    const nav = document.getElementById('nav');
    if (!nav) return;

    if (this.user) {
      nav.innerHTML = `
        <a href="#dashboard" class="nav-link nav-link--primary">My Cards</a>
        <button class="nav-hamburger" data-action="toggle-mobile-menu" aria-label="Toggle menu" aria-expanded="false">
          <span class="hamburger-line"></span>
          <span class="hamburger-line"></span>
          <span class="hamburger-line"></span>
        </button>
        <div class="nav-menu">
          <a href="#profile" class="nav-link">Hi, ${this.escapeHtml(this.user.username)}</a>
          <a href="#friends" class="nav-link">Friends</a>
          <a href="#notifications" class="nav-link nav-link--notifications">
            <span>Notifications</span>
            <span class="nav-badge nav-badge--hidden" id="notification-badge" aria-hidden="true"></span>
          </a>
          <a href="#faq" class="nav-link">FAQ</a>
          <button class="btn btn-ghost" data-action="logout">Logout</button>
        </div>
      `;
    } else {
      nav.innerHTML = `
        <button class="nav-hamburger" data-action="toggle-mobile-menu" aria-label="Toggle menu" aria-expanded="false">
          <span class="hamburger-line"></span>
          <span class="hamburger-line"></span>
          <span class="hamburger-line"></span>
        </button>
        <div class="nav-menu">
          <a href="#faq" class="nav-link">FAQ</a>
        </div>
        <a href="#login" class="btn btn-ghost nav-auth-btn">Login</a>
        <a href="#create" class="btn btn-primary nav-auth-btn">Get Started</a>
      `;
    }
    this.updateNotificationBadge();
  },

  startNotificationPolling() {
    if (this.notificationPoller || !this.user) return;
    this.notificationPoller = setInterval(() => {
      if (!this.user) return;
      this.refreshNotificationCount();
    }, 60000);
  },

  stopNotificationPolling() {
    if (!this.notificationPoller) return;
    clearInterval(this.notificationPoller);
    this.notificationPoller = null;
  },

  async refreshNotificationCount() {
    if (!this.user) return;
    try {
      const response = await API.notifications.unreadCount();
      this.notificationUnreadCount = response?.count || 0;
      this.updateNotificationBadge();
    } catch (error) {
      // Best effort; avoid noisy errors for background polling.
    }
  },

  updateNotificationBadge() {
    const badge = document.getElementById('notification-badge');
    if (!badge) return;
    const count = this.notificationUnreadCount || 0;
    if (count > 0) {
      badge.textContent = count > 99 ? '99+' : String(count);
      badge.classList.remove('nav-badge--hidden');
      badge.setAttribute('aria-hidden', 'false');
      badge.setAttribute('aria-label', `${count} unread notifications`);
    } else {
      badge.textContent = '';
      badge.classList.add('nav-badge--hidden');
      badge.setAttribute('aria-hidden', 'true');
      badge.removeAttribute('aria-label');
    }
  },

  async renderNotifications(container) {
    this.currentView = 'notifications';
    container.innerHTML = `
      <div class="notifications-page">
        <div class="notifications-header">
          <a href="#dashboard" class="btn btn-ghost">&larr; Back</a>
          <h2>Notifications</h2>
          <div class="notifications-actions">
            <button class="btn btn-secondary btn-sm" data-action="mark-all-notifications-read" id="mark-all-notifications-btn">
              Mark all as read
            </button>
            <button class="btn btn-danger-outline btn-sm" data-action="delete-all-notifications" id="delete-all-notifications-btn">
              Delete all
            </button>
          </div>
        </div>
        <div id="notifications-list" class="notifications-list">
          <div class="text-center"><div class="spinner" style="margin: 2rem auto;"></div></div>
        </div>
      </div>
    `;

    const listEl = document.getElementById('notifications-list');
    const markAllBtn = document.getElementById('mark-all-notifications-btn');
    const deleteAllBtn = document.getElementById('delete-all-notifications-btn');

    try {
      const response = await API.notifications.list({ limit: 50 });
      const notifications = response?.notifications || [];
      const counts = this.renderNotificationList(listEl, notifications);
      this.updateNotificationMarkAllButton(markAllBtn, counts.unreadCount);
      this.updateNotificationDeleteAllButton(deleteAllBtn, counts.totalCount);
      await this.markViewedNotifications(notifications);
    } catch (error) {
      if (listEl) {
        listEl.innerHTML = `
          <div class="card text-center" style="padding: 2rem;">
            <p class="text-muted">${this.escapeHtml(error.message)}</p>
          </div>
        `;
      }
      this.updateNotificationMarkAllButton(markAllBtn, 0);
      this.updateNotificationDeleteAllButton(deleteAllBtn, 0);
    }

    await this.refreshNotificationCount();
  },

  renderNotificationList(container, notifications) {
    if (!container) return { unreadCount: 0, totalCount: 0 };
    container.innerHTML = '';

    if (!Array.isArray(notifications) || notifications.length === 0) {
      container.innerHTML = '<p class="text-muted">No notifications yet.</p>';
      return { unreadCount: 0, totalCount: 0 };
    }

    let unreadCount = 0;
    notifications.forEach((notification) => {
      const item = document.createElement('div');
      const isUnread = !notification.read_at;
      item.className = `notification-item${isUnread ? ' notification-item--unread' : ''}`;
      item.dataset.notificationId = notification.id;

      if (isUnread) unreadCount += 1;

      const content = document.createElement('div');
      content.className = 'notification-content';

      const message = document.createElement('p');
      message.className = 'notification-message';
      message.textContent = this.buildNotificationMessage(notification);

      const meta = document.createElement('div');
      meta.className = 'notification-meta';

      const timeEl = document.createElement('span');
      timeEl.className = 'notification-time';
      const createdAt = notification.created_at ? new Date(notification.created_at) : null;
      timeEl.textContent = createdAt ? createdAt.toLocaleString('en-US', { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' }) : '';

      const link = document.createElement('a');
      link.className = 'notification-link';
      link.href = this.getNotificationLink(notification);
      link.textContent = 'View';

      meta.appendChild(timeEl);
      meta.appendChild(link);

      content.appendChild(message);
      content.appendChild(meta);

      item.appendChild(content);

      const actions = document.createElement('div');
      actions.className = 'notification-actions';

      if (isUnread) {
        const markBtn = document.createElement('button');
        markBtn.type = 'button';
        markBtn.className = 'btn btn-ghost btn-sm';
        markBtn.dataset.action = 'mark-notification-read';
        markBtn.dataset.notificationId = notification.id;
        markBtn.textContent = 'Mark as read';
        actions.appendChild(markBtn);
      }

      const deleteBtn = document.createElement('button');
      deleteBtn.type = 'button';
      deleteBtn.className = 'btn btn-ghost btn-sm notification-delete';
      deleteBtn.dataset.action = 'delete-notification';
      deleteBtn.dataset.notificationId = notification.id;
      deleteBtn.setAttribute('aria-label', 'Delete notification');
      deleteBtn.setAttribute('title', 'Delete');
      deleteBtn.innerHTML = `
        <svg class="notification-delete-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <path d="M9 3h6l1 2h4v2h-1l-1 13a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 7H4V5h4l1-2zm1 6v9h2V9H10zm4 0v9h2V9h-2z" fill="currentColor"></path>
        </svg>
      `;
      actions.appendChild(deleteBtn);

      item.appendChild(actions);

      container.appendChild(item);
    });

    return { unreadCount, totalCount: notifications.length };
  },

  async markViewedNotifications(notifications) {
    if (!Array.isArray(notifications) || notifications.length === 0) return;
    const unreadIDs = notifications
      .filter((notification) => !notification.read_at && notification.id)
      .map((notification) => notification.id);
    if (unreadIDs.length === 0) return;

    const results = await Promise.allSettled(
      unreadIDs.map((id) => API.notifications.markRead(id)),
    );

    unreadIDs.forEach((id, index) => {
      if (results[index].status !== 'fulfilled') return;
      const item = document.querySelector(`.notification-item[data-notification-id="${id}"]`);
      if (!item) return;
      item.classList.remove('notification-item--unread');
      const btn = item.querySelector('[data-action="mark-notification-read"]');
      if (btn) btn.remove();
    });

    const markAllBtn = document.getElementById('mark-all-notifications-btn');
    const unreadCount = document.querySelectorAll('.notification-item--unread').length;
    this.updateNotificationMarkAllButton(markAllBtn, unreadCount);
    await this.refreshNotificationCount();
  },

  buildNotificationMessage(notification) {
    const actor = notification.actor_username || 'A friend';
    const cardName = this.getNotificationCardName(notification);

    switch (notification.type) {
      case 'friend_request_received':
        return `${actor} sent you a friend request.`;
      case 'friend_request_accepted':
        return `${actor} accepted your friend request.`;
      case 'friend_bingo': {
        const total = notification.bingo_count ? ` (${notification.bingo_count} total)` : '';
        return `${actor} got a bingo on ${cardName}${total}.`;
      }
      case 'friend_new_card':
        return `${actor} created a new card: ${cardName}.`;
      default:
        return 'You have a new notification.';
    }
  },

  getNotificationLink(notification) {
    if (notification.type === 'friend_bingo' || notification.type === 'friend_new_card') {
      if (notification.friendship_id) {
        return `#friend-card/${notification.friendship_id}`;
      }
    }
    return '#friends';
  },

  getNotificationCardName(notification) {
    if (notification.card_title) {
      return notification.card_title;
    }
    if (notification.card_year) {
      return `${notification.card_year} Bingo Card`;
    }
    return 'a bingo card';
  },

  updateNotificationMarkAllButton(button, unreadCount) {
    if (!button) return;
    const disabled = unreadCount === 0;
    button.disabled = disabled;
    button.setAttribute('aria-disabled', disabled ? 'true' : 'false');
  },

  updateNotificationDeleteAllButton(button, totalCount) {
    if (!button) return;
    const disabled = totalCount === 0;
    button.disabled = disabled;
    button.setAttribute('aria-disabled', disabled ? 'true' : 'false');
  },

  async markNotificationRead(target) {
    const notificationId = target?.dataset?.notificationId;
    if (!notificationId) return;

    try {
      await API.notifications.markRead(notificationId);
      const item = target.closest('.notification-item');
      if (item) {
        item.classList.remove('notification-item--unread');
        target.remove();
      }
      const unreadCount = document.querySelectorAll('.notification-item--unread').length;
      const markAllBtn = document.getElementById('mark-all-notifications-btn');
      this.updateNotificationMarkAllButton(markAllBtn, unreadCount);
      await this.refreshNotificationCount();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async deleteNotification(target) {
    const notificationId = target?.dataset?.notificationId;
    if (!notificationId) return;

    try {
      await API.notifications.delete(notificationId);
      const item = target.closest('.notification-item');
      if (item) {
        item.remove();
      }
      const listEl = document.getElementById('notifications-list');
      const totalCount = listEl ? listEl.querySelectorAll('.notification-item').length : 0;
      if (listEl && totalCount === 0) {
        listEl.innerHTML = '<p class="text-muted">No notifications yet.</p>';
      }
      const unreadCount = document.querySelectorAll('.notification-item--unread').length;
      const markAllBtn = document.getElementById('mark-all-notifications-btn');
      const deleteAllBtn = document.getElementById('delete-all-notifications-btn');
      this.updateNotificationMarkAllButton(markAllBtn, unreadCount);
      this.updateNotificationDeleteAllButton(deleteAllBtn, totalCount);
      await this.refreshNotificationCount();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async markAllNotificationsRead() {
    try {
      await API.notifications.markAllRead();
      document.querySelectorAll('.notification-item--unread').forEach((item) => {
        item.classList.remove('notification-item--unread');
        const btn = item.querySelector('[data-action="mark-notification-read"]');
        if (btn) btn.remove();
      });
      const markAllBtn = document.getElementById('mark-all-notifications-btn');
      this.updateNotificationMarkAllButton(markAllBtn, 0);
      await this.refreshNotificationCount();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async deleteAllNotifications() {
    if (!confirm('Delete all notifications? This cannot be undone.')) return;
    try {
      await API.notifications.deleteAll();
      const listEl = document.getElementById('notifications-list');
      if (listEl) {
        listEl.innerHTML = '<p class="text-muted">No notifications yet.</p>';
      }
      const markAllBtn = document.getElementById('mark-all-notifications-btn');
      const deleteAllBtn = document.getElementById('delete-all-notifications-btn');
      this.updateNotificationMarkAllButton(markAllBtn, 0);
      this.updateNotificationDeleteAllButton(deleteAllBtn, 0);
      await this.refreshNotificationCount();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async loadNotificationSettings() {
    const container = document.getElementById('notification-settings');
    if (!container) return;

    container.innerHTML = '<div class="text-center"><div class="spinner spinner--small"></div></div>';
    try {
      const response = await API.notifications.getSettings();
      this.notificationSettings = response.settings;
      this.renderNotificationSettings(container, response.settings);
    } catch (error) {
      container.innerHTML = `<p class="text-muted">${this.escapeHtml(error.message)}</p>`;
    }
  },

  renderNotificationSettings(container, settings) {
    if (!settings) {
      container.innerHTML = '<p class="text-muted">Unable to load notification settings.</p>';
      return;
    }

    const emailNote = this.user?.email_verified
      ? 'Email notifications are sent to your verified email address.'
      : 'Verify your email to enable email notifications.';

    container.innerHTML = `
      <div class="notification-settings-grid">
        <div class="notification-channel">
          <label class="checkbox-label notification-master">
            <input type="checkbox" id="notify-in-app-enabled" data-change-action="notification-master-toggle" data-channel="in_app" ${settings.in_app_enabled ? 'checked' : ''}>
            <span>In-app notifications</span>
          </label>
          <div class="notification-options" data-channel="in_app">
            <label class="checkbox-label">
              <input type="checkbox" data-change-action="notification-scenario-toggle" data-setting="in_app_friend_request_received" ${settings.in_app_friend_request_received ? 'checked' : ''}>
              <span>Friend request received</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" data-change-action="notification-scenario-toggle" data-setting="in_app_friend_request_accepted" ${settings.in_app_friend_request_accepted ? 'checked' : ''}>
              <span>Friend request accepted</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" data-change-action="notification-scenario-toggle" data-setting="in_app_friend_bingo" ${settings.in_app_friend_bingo ? 'checked' : ''}>
              <span>Friend gets a bingo</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" data-change-action="notification-scenario-toggle" data-setting="in_app_friend_new_card" ${settings.in_app_friend_new_card ? 'checked' : ''}>
              <span>Friend creates a new card</span>
            </label>
          </div>
        </div>
        <div class="notification-channel">
          <label class="checkbox-label notification-master">
            <input type="checkbox" id="notify-email-enabled" data-change-action="notification-master-toggle" data-channel="email" ${settings.email_enabled ? 'checked' : ''}>
            <span>Email notifications</span>
          </label>
          <small class="text-muted">${emailNote}</small>
          <div class="notification-options" data-channel="email">
            <label class="checkbox-label">
              <input type="checkbox" data-change-action="notification-scenario-toggle" data-setting="email_friend_request_received" ${settings.email_friend_request_received ? 'checked' : ''}>
              <span>Friend request received</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" data-change-action="notification-scenario-toggle" data-setting="email_friend_request_accepted" ${settings.email_friend_request_accepted ? 'checked' : ''}>
              <span>Friend request accepted</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" data-change-action="notification-scenario-toggle" data-setting="email_friend_bingo" ${settings.email_friend_bingo ? 'checked' : ''}>
              <span>Friend gets a bingo</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" data-change-action="notification-scenario-toggle" data-setting="email_friend_new_card" ${settings.email_friend_new_card ? 'checked' : ''}>
              <span>Friend creates a new card</span>
            </label>
          </div>
        </div>
      </div>
    `;

    this.applyNotificationSettingsState();
  },

  applyNotificationSettingsState() {
    if (!this.notificationSettings) return;

    const inAppEnabled = this.notificationSettings.in_app_enabled;
    const emailEnabled = this.notificationSettings.email_enabled;
    const emailLocked = !this.user?.email_verified;

    const inAppMaster = document.getElementById('notify-in-app-enabled');
    const emailMaster = document.getElementById('notify-email-enabled');
    if (inAppMaster) inAppMaster.checked = inAppEnabled;
    if (emailMaster) emailMaster.checked = emailEnabled;

    const inAppOptions = document.querySelector('.notification-options[data-channel="in_app"]');
    const emailOptions = document.querySelector('.notification-options[data-channel="email"]');

    if (inAppOptions) {
      inAppOptions.classList.toggle('notification-options--disabled', !inAppEnabled);
      inAppOptions.querySelectorAll('input[type=\"checkbox\"]').forEach((input) => {
        input.disabled = !inAppEnabled;
      });
    }

    if (emailOptions) {
      const disableEmail = emailLocked || !emailEnabled;
      emailOptions.classList.toggle('notification-options--disabled', disableEmail);
      emailOptions.querySelectorAll('input[type=\"checkbox\"]').forEach((input) => {
        input.disabled = disableEmail;
      });
    }

    if (emailMaster) {
      emailMaster.disabled = emailLocked;
    }
  },

  async handleNotificationMasterToggle(target) {
    if (!this.notificationSettings) return;
    const channel = target.dataset.channel;
    if (!channel) return;
    const enabled = target.checked;
    const patch = {};
    patch[channel === 'email' ? 'email_enabled' : 'in_app_enabled'] = enabled;
    await this.saveNotificationSettings(patch, target, !enabled);
  },

  async handleNotificationScenarioToggle(target) {
    if (!this.notificationSettings) return;
    const setting = target.dataset.setting;
    if (!setting) return;
    const enabled = target.checked;
    const patch = { [setting]: enabled };
    await this.saveNotificationSettings(patch, target, !enabled);
  },

  async saveNotificationSettings(patch, target, revertValue) {
    try {
      const response = await API.notifications.updateSettings(patch);
      this.notificationSettings = response.settings;
      this.applyNotificationSettingsState();
      this.toast('Notification settings updated', 'success');
    } catch (error) {
      if (target && typeof revertValue === 'boolean') {
        target.checked = revertValue;
      }
      this.applyNotificationSettingsState();
      this.toast(error.message, 'error');
    }
  },

  async loadReminderSettings() {
    const container = document.getElementById('reminder-settings');
    if (!container) return;

    container.innerHTML = '<div class="text-center"><div class="spinner spinner--small"></div></div>';
    try {
      const [settingsResponse, cardsResponse, goalsResponse] = await Promise.all([
        API.reminders.getSettings(),
        API.reminders.listCards(),
        API.reminders.listGoals(),
      ]);
      this.reminderSettings = settingsResponse.settings;
      this.reminderCards = cardsResponse.cards || [];
      this.goalReminders = goalsResponse.reminders || [];
      this.goalRemindersByItem = this.mapGoalReminders(this.goalReminders);
      this.renderReminderSettings(container, this.reminderSettings, this.reminderCards, this.goalReminders);
    } catch (error) {
      container.innerHTML = `<p class="text-muted">${this.escapeHtml(error.message)}</p>`;
    }
  },

  renderReminderSettings(container, settings, cards, goalReminders) {
    if (!settings) {
      container.innerHTML = '<p class="text-muted">Unable to load reminder settings.</p>';
      return;
    }

    const emailLocked = !this.user?.email_verified;
    const emailNote = emailLocked
      ? 'Verify your email to enable reminder emails.'
      : 'Email reminders are sent to your verified address.';

    const selectedCard = this.getSelectedReminderCard(cards);
    const hasCards = !!selectedCard;
    const schedule = this.getReminderSchedule(selectedCard?.checkin);
    const includeImage = selectedCard?.checkin?.include_image !== false;
    const includeRecommendations = selectedCard?.checkin?.include_recommendations !== false;
    const nextSend = selectedCard?.checkin?.next_send_at
      ? this.formatReminderTimestamp(selectedCard.checkin.next_send_at)
      : 'Not scheduled';

    const cardOptions = cards.map((card) => {
      const label = this.escapeHtml(card.card_title || `${card.card_year} Bingo Card`);
      const selected = card.card_id === this.reminderSelectedCardId ? 'selected' : '';
      return `<option value="${card.card_id}" ${selected}>${label}</option>`;
    }).join('');

    const dayOptions = Array.from({ length: 28 }, (_, i) => {
      const day = i + 1;
      const selected = day === schedule.day ? 'selected' : '';
      return `<option value="${day}" ${selected}>${day}</option>`;
    }).join('');

    const reminderEnabled = settings.email_enabled;
    const disableControls = emailLocked || !reminderEnabled || !hasCards;

    container.innerHTML = `
      <div class="reminder-section">
        <label class="checkbox-label reminder-master">
          <input type="checkbox" id="reminder-email-enabled" data-change-action="reminder-master-toggle" ${settings.email_enabled ? 'checked' : ''} ${emailLocked ? 'disabled' : ''}>
          <span>Email reminders</span>
        </label>
        <small class="text-muted">${emailNote}</small>
      </div>

      <div class="reminder-section">
        <h4>Card check-ins</h4>
        ${!hasCards ? '<p class="text-muted">No finalized cards available yet.</p>' : `
          <div class="reminder-card-controls ${disableControls ? 'reminder-controls--disabled' : ''}">
            <div class="form-group">
              <label class="form-label" for="reminder-card-select">Card</label>
              <select id="reminder-card-select" class="form-input" data-change-action="reminder-card-select" ${disableControls ? 'disabled' : ''}>
                ${cardOptions}
              </select>
            </div>

            <div class="reminder-inline">
              <div class="form-group">
                <label class="form-label" for="reminder-day">Day of month</label>
                <select id="reminder-day" class="form-input" ${disableControls ? 'disabled' : ''}>
                  ${dayOptions}
                </select>
              </div>
              <div class="form-group">
                <label class="form-label" for="reminder-time">Time</label>
                <input type="time" id="reminder-time" class="form-input" value="${schedule.time}" ${disableControls ? 'disabled' : ''}>
              </div>
            </div>
            <p class="text-muted">Times use the server clock.</p>

            <label class="checkbox-label">
              <input type="checkbox" id="reminder-include-image" ${includeImage ? 'checked' : ''} ${disableControls ? 'disabled' : ''}>
              <span>Include card image</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" id="reminder-include-recommendations" ${includeRecommendations ? 'checked' : ''} ${disableControls ? 'disabled' : ''}>
              <span>Include suggested goals</span>
            </label>

            <p class="text-muted">Next send: ${this.escapeHtml(nextSend)}</p>

            <div class="reminder-actions">
              <button class="btn btn-secondary btn-sm" data-action="save-card-checkin" ${disableControls ? 'disabled' : ''}>Save schedule</button>
              <button class="btn btn-secondary btn-sm" data-action="apply-card-checkin-all" ${disableControls || cards.length < 2 ? 'disabled' : ''}>Apply to all cards</button>
              <button class="btn btn-ghost btn-sm" data-action="delete-card-checkin" ${disableControls || !selectedCard?.checkin ? 'disabled' : ''}>Remove schedule</button>
              <button class="btn btn-secondary btn-sm" data-action="send-reminder-test" ${disableControls ? 'disabled' : ''}>Send test email</button>
            </div>
          </div>
        `}
      </div>

      <div class="reminder-section">
        <h4>Goal reminders</h4>
        <div id="reminder-goal-list">
          ${this.renderGoalReminderList(goalReminders)}
        </div>
      </div>
    `;
  },

  renderGoalReminderList(goalReminders) {
    if (!goalReminders || goalReminders.length === 0) {
      return '<p class="text-muted">No goal reminders set yet.</p>';
    }

    return goalReminders.map((reminder) => {
      const cardName = this.escapeHtml(reminder.card_title || `${reminder.card_year} Bingo Card`);
      const nextSend = reminder.next_send_at ? this.formatReminderTimestamp(reminder.next_send_at) : 'Not scheduled';
      const goalText = this.escapeHtml(reminder.item_text);
      return `
        <div class="reminder-goal-item">
          <div>
            <p class="reminder-goal-text">${goalText}</p>
            <p class="reminder-goal-meta">${cardName} - ${this.escapeHtml(nextSend)}</p>
          </div>
          <button class="btn btn-ghost btn-sm" data-action="delete-goal-reminder" data-reminder-id="${reminder.id}">Stop</button>
        </div>
      `;
    }).join('');
  },

  applyReminderSettingsState() {
    if (!this.reminderSettings) return;
    const emailLocked = !this.user?.email_verified;
    const enabled = this.reminderSettings.email_enabled;
    const master = document.getElementById('reminder-email-enabled');
    if (master) master.checked = enabled;
    if (master) master.disabled = emailLocked;
  },

  async handleReminderMasterToggle(target) {
    if (!this.reminderSettings) return;
    const enabled = target.checked;
    try {
      const response = await API.reminders.updateSettings({ email_enabled: enabled });
      this.reminderSettings = response.settings;
      this.applyReminderSettingsState();
      this.toast('Reminder settings updated', 'success');
      await this.loadReminderSettings();
    } catch (error) {
      target.checked = !enabled;
      this.applyReminderSettingsState();
      this.toast(error.message, 'error');
    }
  },

  handleReminderCardSelect(target) {
    this.reminderSelectedCardId = target.value;
    const container = document.getElementById('reminder-settings');
    if (container) {
      this.renderReminderSettings(container, this.reminderSettings, this.reminderCards, this.goalReminders);
    }
  },

  getSelectedReminderCard(cards) {
    if (!cards || cards.length === 0) return null;
    let selected = cards.find(card => card.card_id === this.reminderSelectedCardId);
    if (!selected) {
      selected = cards[0];
      this.reminderSelectedCardId = selected.card_id;
    }
    return selected;
  },

  getReminderSchedule(checkin) {
    if (!checkin || !checkin.schedule) {
      return { day: 1, time: '09:00' };
    }
    let schedule = checkin.schedule;
    if (typeof schedule === 'string') {
      try {
        schedule = JSON.parse(schedule);
      } catch (error) {
        schedule = {};
      }
    }
    return {
      day: Number(schedule.day_of_month) || 1,
      time: schedule.time || '09:00',
    };
  },

  async saveCardCheckin() {
    const selected = this.getSelectedReminderCard(this.reminderCards);
    if (!selected) return;

    const day = Number.parseInt(document.getElementById('reminder-day')?.value || '1', 10);
    const time = document.getElementById('reminder-time')?.value || '09:00';
    const includeImage = document.getElementById('reminder-include-image')?.checked !== false;
    const includeRecommendations = document.getElementById('reminder-include-recommendations')?.checked !== false;

    try {
      await API.reminders.upsertCardCheckin(selected.card_id, {
        frequency: 'monthly',
        schedule: { day_of_month: day, time },
        include_image: includeImage,
        include_recommendations: includeRecommendations,
      });
      this.toast('Check-in schedule saved', 'success');
      await this.loadReminderSettings();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async applyCardCheckinToAll() {
    if (!this.reminderCards || this.reminderCards.length === 0) return;

    const day = Number.parseInt(document.getElementById('reminder-day')?.value || '1', 10);
    const time = document.getElementById('reminder-time')?.value || '09:00';
    const includeImage = document.getElementById('reminder-include-image')?.checked !== false;
    const includeRecommendations = document.getElementById('reminder-include-recommendations')?.checked !== false;

    try {
      await Promise.all(this.reminderCards.map(card => API.reminders.upsertCardCheckin(card.card_id, {
        frequency: 'monthly',
        schedule: { day_of_month: day, time },
        include_image: includeImage,
        include_recommendations: includeRecommendations,
      })));
      this.toast('Schedules applied to all cards', 'success');
      await this.loadReminderSettings();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async deleteCardCheckin() {
    const selected = this.getSelectedReminderCard(this.reminderCards);
    if (!selected || !selected.checkin) return;
    if (!confirm('Remove this check-in schedule?')) return;

    try {
      await API.reminders.deleteCardCheckin(selected.card_id);
      this.toast('Check-in removed', 'success');
      await this.loadReminderSettings();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async sendReminderTest() {
    const selected = this.getSelectedReminderCard(this.reminderCards);
    if (!selected) return;
    try {
      await API.reminders.sendTestEmail(selected.card_id);
      this.toast('Test email sent', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async loadGoalReminders(cardId = null) {
    try {
      const response = await API.reminders.listGoals(cardId);
      this.goalReminders = response.reminders || [];
      this.goalRemindersByItem = this.mapGoalReminders(this.goalReminders);
      const container = document.getElementById('reminder-goal-list');
      if (container) {
        container.innerHTML = this.renderGoalReminderList(this.goalReminders);
      }
    } catch (error) {
      const container = document.getElementById('reminder-goal-list');
      if (container) {
        container.innerHTML = `<p class="text-muted">${this.escapeHtml(error.message)}</p>`;
      }
    }
  },

  mapGoalReminders(reminders) {
    const map = {};
    reminders.forEach((reminder) => {
      if (reminder.item_id) {
        map[reminder.item_id] = reminder;
      }
    });
    return map;
  },

  async setGoalReminder(target) {
    const itemId = target.dataset.itemId;
    if (!itemId) return;

    let sendAt = '';
    const preset = target.dataset.preset;
    if (preset === 'custom') {
      const input = document.getElementById('reminder-custom-datetime');
      if (!input || !input.value) {
        this.toast('Select a date and time', 'error');
        return;
      }
      sendAt = input.value;
    } else {
      sendAt = this.getPresetReminderTime(preset);
    }

    if (!sendAt) {
      this.toast('Unable to set reminder time', 'error');
      return;
    }

    try {
      await API.reminders.upsertGoalReminder({
        item_id: itemId,
        kind: 'one_time',
        schedule: { send_at: sendAt },
      });
      await this.loadGoalReminders(this.currentCard?.id || null);
      const item = this.currentCard.items?.find(i => i.id === itemId);
      if (item) {
        this.showItemDetailModal(item.position, item.content, item.is_completed);
      }
      this.toast('Goal reminder saved', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async deleteGoalReminder(target) {
    const reminderId = target.dataset.reminderId;
    if (!reminderId) {
      return;
    }
    if (!confirm('Stop reminders for this goal?')) return;
    const reminder = this.goalReminders?.find(entry => entry.id === reminderId);
    const itemId = reminder?.item_id;
    try {
      await API.reminders.deleteGoalReminder(reminderId);
      await this.loadGoalReminders(this.currentCard?.id || null);
      if (itemId && this.currentCard?.items) {
        const item = this.currentCard.items.find(i => i.id === itemId);
        if (item) {
          this.showItemDetailModal(item.position, item.content, item.is_completed);
        }
      }
      this.toast('Goal reminder removed', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  getPresetReminderTime(preset) {
    const base = new Date();
    base.setSeconds(0, 0);
    if (preset === 'tomorrow') {
      base.setDate(base.getDate() + 1);
    } else if (preset === 'week') {
      base.setDate(base.getDate() + 7);
    } else if (preset === 'month') {
      base.setMonth(base.getMonth() + 1);
    } else {
      return '';
    }
    base.setHours(9, 0, 0, 0);
    return this.formatLocalDateTime(base);
  },

  formatLocalDateTime(date) {
    const pad = (value) => String(value).padStart(2, '0');
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
  },

  formatReminderTimestamp(value) {
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) return 'Not scheduled';
    return parsed.toLocaleString('en-US', { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
  },

  toggleMobileMenu() {
    const nav = document.getElementById('nav');
    const hamburger = nav?.querySelector('.nav-hamburger');
    const menu = nav?.querySelector('.nav-menu');
    if (!nav || !menu) return;

    const isOpen = nav.classList.toggle('nav--open');
    hamburger?.setAttribute('aria-expanded', isOpen ? 'true' : 'false');
  },

  closeMobileMenu() {
    const nav = document.getElementById('nav');
    const hamburger = nav?.querySelector('.nav-hamburger');
    if (nav) {
      nav.classList.remove('nav--open');
      hamburger?.setAttribute('aria-expanded', 'false');
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
    if (bodyEl) bodyEl.scrollTop = 0;
    if (overlay) overlay.scrollTop = 0;
    this.modalScrollY = window.scrollY || window.pageYOffset || 0;
    if (overlay) overlay.classList.add('modal-overlay--visible');
    document.body.classList.add('modal-open');
    document.body.style.top = `-${this.modalScrollY}px`;
  },

  closeModal() {
    const overlay = document.getElementById('modal-overlay');
    if (overlay) overlay.classList.remove('modal-overlay--visible');
    document.body.classList.remove('modal-open');
    document.body.style.top = '';
    if (typeof this.modalScrollY === 'number') {
      window.scrollTo(0, this.modalScrollY);
      this.modalScrollY = null;
    }
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
      <div class="text-center mb-lg" style="padding-bottom: 1.5rem; border-bottom: 1px solid var(--border-color);">
        <button class="btn btn-secondary btn-lg" data-action="open-ai-wizard-from-modal" style="width: 100%; display: flex; align-items: center; justify-content: center; gap: 0.5rem;">
            <span>✨</span> Generate with AI Wizard
        </button>
        <p class="text-muted mt-sm" style="font-size: 0.9rem;">Let AI create a custom card for you!</p>
      </div>

      <form data-action="create-card-modal">
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

        <div class="form-group">
          <label for="modal-card-grid-size">Grid Size</label>
          <select id="modal-card-grid-size" class="form-input">
            <option value="2">2x2</option>
            <option value="3">3x3</option>
            <option value="4">4x4</option>
            <option value="5" selected>5x5</option>
          </select>
        </div>

        <div class="form-group">
          <label class="checkbox-label" style="display: flex; align-items: center; gap: 0.5rem;">
            <input type="checkbox" id="modal-card-free-space" checked>
            <span>Include FREE space</span>
          </label>
        </div>

        <div class="form-group">
          <label for="modal-card-header">Header</label>
          <input type="text" id="modal-card-header" class="form-input" maxlength="5" value="BINGO" required>
          <small class="text-muted" id="modal-card-header-help">1-5 characters.</small>
        </div>

        <div style="display: flex; gap: 0.5rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-ghost" style="flex: 1;" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary" style="flex: 1;">Create Card</button>
        </div>
      </form>
    `);

    const gridSizeEl = document.getElementById('modal-card-grid-size');
    const headerEl = document.getElementById('modal-card-header');
    const headerHelpEl = document.getElementById('modal-card-header-help');
    if (gridSizeEl && headerEl) {
      const apply = () => {
        const n = parseInt(gridSizeEl.value, 10) || 5;
        headerEl.maxLength = n;
        if (headerHelpEl) headerHelpEl.textContent = `1-${n} characters.`;
        if (headerEl.value.length > n) headerEl.value = Array.from(headerEl.value).slice(0, n).join('');
        if (!headerEl.dataset.touched) headerEl.value = Array.from('BINGO').slice(0, n).join('');
      };
      headerEl.addEventListener('input', () => {
        headerEl.dataset.touched = 'true';
      });
      gridSizeEl.addEventListener('change', apply);
      apply();
    }
  },

  async handleCreateCardModal(event) {
    event.preventDefault();

    const year = parseInt(document.getElementById('modal-card-year').value, 10);
    const title = document.getElementById('modal-card-title').value.trim() || null;
    const category = document.getElementById('modal-card-category').value || null;
    const gridSize = parseInt(document.getElementById('modal-card-grid-size')?.value || '5', 10);
    const hasFreeSpace = !!document.getElementById('modal-card-free-space')?.checked;
    const headerText = document.getElementById('modal-card-header')?.value?.trim() || '';

    try {
      const response = await API.cards.create(year, title, category, {
        gridSize,
        hasFreeSpace,
        headerText,
      });

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
      <form data-action="save-card-meta">
        <div class="form-group">
          <label for="edit-card-title">Title</label>
          <input type="text" id="edit-card-title" class="form-input"
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
          <button type="button" class="btn btn-ghost" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary">Save</button>
        </div>
      </form>
    `);
    const titleInput = document.getElementById('edit-card-title');
    if (titleInput) titleInput.value = currentTitle;
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
    this.closeMobileMenu();
    window.scrollTo(0, 0);
    this.currentView = null;
    this.isSharedView = false;
    const hash = window.location.hash.slice(1) || 'home';
    // Parse hash with query parameters: page?param=value
    const [pagePart, queryPart] = hash.split('?');
    const [page, ...params] = pagePart.split('/');
    const queryParams = new URLSearchParams(queryPart || '');
    this.setRobotsMeta(page === 'share' ? 'noindex' : null);

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
        this.requireAuth(() => this.renderCard(container, params[0], queryParams.get('item')));
        break;
      case 'share':
        this.renderSharedCard(container, params[0]);
        break;
      case 'friends':
        this.requireAuth(() => this.renderFriends(container));
        break;
      case 'notifications':
        this.requireAuth(() => this.renderNotifications(container));
        break;
      case 'friend-invite':
        if (this.user) {
          this.renderInviteAccept(container, params[0]);
        } else {
          this.renderInviteGate(container, params[0]);
        }
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
      case 'about':
        this.renderAbout(container);
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
      case 'faq':
        this.renderFAQ(container);
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

  redirectAfterAuth(defaultHash = '#dashboard') {
    const token = this.consumePendingInviteToken();
    if (token) {
      window.location.hash = `#friend-invite/${token}`;
      return;
    }
    window.location.hash = defaultHash;
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
            <button class="btn btn-secondary btn-lg" data-action="show-create-card-modal">Create New Card</button>
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
        await this.refreshNotificationCount();
        this.startNotificationPolling();
        this.redirectAfterAuth('#dashboard');
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
        await this.refreshNotificationCount();
        this.startNotificationPolling();
        this.redirectAfterAuth('#create');
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
      this.redirectAfterAuth('#dashboard');
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
            <p class="text-muted" id="verify-error-message"></p>
            ${this.user ? `
              <button class="btn btn-primary" style="margin-top: 1rem;" data-action="resend-verification">
                Resend Verification Email
              </button>
            ` : `<a href="#login" class="btn btn-primary" style="margin-top: 1rem;">Sign In</a>`}
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
    const freeLimit = 5;
    const used = typeof this.user.ai_free_generations_used === 'number' ? this.user.ai_free_generations_used : 0;
    const remaining = Math.max(0, freeLimit - used);
    return `
      <div class="verification-banner" style="background: rgba(245, 158, 11, 0.15); border: 1px solid #f59e0b; border-radius: 8px; padding: 1rem; margin-bottom: 1.5rem; display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 1rem;">
        <div style="color: #fff;">
          <strong style="color: #ffd700;">Please verify your email</strong>
          <span style="color: #b0b0c0;"> to enable all features.</span>
          <div class="text-muted" style="margin-top: 0.35rem; font-size: 0.9rem;">
            AI Goal Wizard: <strong>${remaining}</strong> free generations left before verification is required.
          </div>
        </div>
        <button class="btn btn-secondary btn-sm" data-action="resend-verification">
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
          <select id="sort-select" class="form-input form-input--sm" data-change-action="dashboard-sort">
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
          <button class="btn btn-ghost btn-sm" data-action="select-all-cards">Select All</button>
          <button class="btn btn-ghost btn-sm" data-action="deselect-all-cards">Deselect All</button>
          <span id="selected-count" class="text-muted">${this.selectedCards.length} selected</span>
        </div>
        <div class="dashboard-actions">
          <div class="dropdown" id="actions-dropdown">
            <button class="btn btn-secondary dropdown-toggle" aria-haspopup="true" aria-expanded="false">
              Actions
            </button>
            <div class="dropdown-menu" role="menu">
              <button class="dropdown-item ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" data-action="bulk-archive" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-archive"></i> Archive
              </button>
              <button class="dropdown-item ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" data-action="bulk-unarchive" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-box-open"></i> Unarchive
              </button>
              <div class="dropdown-divider"></div>
              <button class="dropdown-item ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" data-action="bulk-visible" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-eye"></i> Make Visible
              </button>
              <button class="dropdown-item ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" data-action="bulk-private" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-eye-slash"></i> Make Private
              </button>
              <div class="dropdown-divider"></div>
              <button class="dropdown-item dropdown-item--danger ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" data-action="bulk-delete" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-trash"></i> Delete
              </button>
              <div class="dropdown-divider"></div>
              <button class="dropdown-item ${hasSelection ? '' : 'dropdown-item--disabled'}" role="menuitem" data-action="export-cards" ${hasSelection ? '' : 'title="Select cards first"'}>
                <i class="fas fa-download"></i> Export Cards
              </button>
            </div>
          </div>
          <button class="btn btn-primary" data-action="show-create-card-modal">+ Card</button>
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
    const capacity = this.getCardCapacity(card);
    const progress = capacity ? (card.is_finalized ? Math.round((completedCount / capacity) * 100) : Math.round((itemCount / capacity) * 100)) : 0;
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
            <label class="dashboard-checkbox-label" data-stop-propagation="true">
              <input type="checkbox" class="dashboard-card-checkbox" data-card-id="${card.id}" ${isSelected ? 'checked' : ''} data-change-action="dashboard-selection">
            </label>
            <a href="${cardLink}" style="text-decoration: none; flex: 1;">
              <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 0.25rem;">
                <h3 style="margin: 0;">${displayName}</h3>
                <span class="year-badge">${card.year}</span>
                ${categoryBadge}
              </div>
              <p class="text-muted" style="margin: 0;">
                ${card.is_finalized
                  ? `${completedCount}/${capacity} completed`
                  : `${itemCount}/${capacity} items added`}
              </p>
            </a>
          </div>
          <div style="display: flex; align-items: center; gap: 0.5rem;">
            <span class="visibility-badge visibility-badge--${card.visible_to_friends ? 'visible' : 'private'}" title="${visibilityLabel}">
              <i class="fas fa-${visibilityIcon}"></i> ${card.visible_to_friends ? 'Visible' : 'Private'}
            </span>
            ${card.is_archived ? '<div class="archive-badge">Archived</div>' : ''}
            <button class="btn btn-ghost btn-sm dashboard-delete-btn" style="color: var(--color-danger);" data-action="delete-card" data-card-id="${card.id}" data-stop-propagation="true" aria-label="Delete card" title="Delete card">
              <i class="fas fa-trash"></i>
            </button>
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
      const capacity = this.getCardCapacity(card);
      if (!capacity) return 0;
      if (!card.is_finalized) {
        const itemCount = card.items ? card.items.length : 0;
        return itemCount / capacity;
      }
      const completedCount = card.items ? card.items.filter(i => i.is_completed).length : 0;
      return completedCount / capacity;
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

        <div class="card" style="margin: 0 0 1rem 0; padding: 1rem; background: var(--bg-secondary); border: 1px solid var(--border-color);">
          <div style="display: flex; gap: 0.75rem; align-items: flex-start;">
            <div style="font-size: 1.25rem; line-height: 1;">🧙</div>
            <div>
              <div style="font-weight: 600; margin-bottom: 0.25rem;">Want AI-generated goals?</div>
              <div class="text-muted" style="font-size: 0.95rem; margin-bottom: 0.75rem;">
                The AI Goal Wizard is available after you create an account.
              </div>
              <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
                <button type="button" class="btn btn-secondary btn-sm" data-action="show-ai-auth-modal" style="display: flex; align-items: center; gap: 0.35rem;">
                  <span>✨</span> Generate with AI Wizard
                </button>
              </div>
            </div>
          </div>
        </div>

        <form id="create-card-form" data-action="create-card-anon">
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

          <div class="form-group">
            <label for="card-grid-size">Grid Size</label>
            <select id="card-grid-size" class="form-input">
              <option value="2">2x2</option>
              <option value="3">3x3</option>
              <option value="4">4x4</option>
              <option value="5" selected>5x5</option>
            </select>
          </div>

          <div class="form-group">
            <label class="checkbox-label" style="display: flex; align-items: center; gap: 0.5rem;">
              <input type="checkbox" id="card-free-space" checked>
              <span>Include FREE space</span>
            </label>
          </div>

          <div class="form-group">
            <label for="card-header">Header</label>
            <input type="text" id="card-header" class="form-input" maxlength="5" value="BINGO" required>
            <small class="text-muted" id="card-header-help">1-5 characters.</small>
          </div>

          <div style="display: flex; gap: 0.5rem; margin-top: 1rem;">
            <a href="#home" class="btn btn-ghost btn-lg" style="flex: 1; text-align: center;">Cancel</a>
            <button type="submit" class="btn btn-primary btn-lg" style="flex: 1;">Create Card</button>
          </div>
        </form>
      </div>
    `;

    const gridSizeEl = document.getElementById('card-grid-size');
    const headerEl = document.getElementById('card-header');
    const headerHelpEl = document.getElementById('card-header-help');
    if (gridSizeEl && headerEl) {
      const apply = () => {
        const n = parseInt(gridSizeEl.value, 10) || 5;
        headerEl.maxLength = n;
        if (headerHelpEl) headerHelpEl.textContent = `1-${n} characters.`;
        if (headerEl.value.length > n) headerEl.value = Array.from(headerEl.value).slice(0, n).join('');
        if (!headerEl.dataset.touched) headerEl.value = Array.from('BINGO').slice(0, n).join('');
      };
      headerEl.addEventListener('input', () => {
        headerEl.dataset.touched = 'true';
      });
      gridSizeEl.addEventListener('change', apply);
      apply();
    }
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
    const gridSize = parseInt(document.getElementById('card-grid-size')?.value || '5', 10);
    const hasFreeSpace = !!document.getElementById('card-free-space')?.checked;
    const headerText = document.getElementById('card-header')?.value?.trim() || '';

    // Create anonymous card in localStorage
    const card = AnonymousCard.create(year, title, category, gridSize, headerText, hasFreeSpace);
    this.isAnonymousMode = true;
    this.currentCard = this.convertAnonymousCardToAppFormat(card);

    // Navigate to the editor
    this.renderAnonymousCardEditor(document.getElementById('main-container'));
    const cardName = title || `${year} Bingo Card`;
    this.toast(`${cardName} created! Add your goals below.`, 'success');
  },

  // Convert anonymous card format to the format used by the app
  convertAnonymousCardToAppFormat(anonCard) {
    const gridSize = anonCard.grid_size || 5;
    const totalSquares = gridSize * gridSize;
    const hasFreeSpace = typeof anonCard.has_free_space === 'boolean' ? anonCard.has_free_space : true;
    const defaultFreePos = gridSize % 2 === 1 ? Math.floor(totalSquares / 2) : 0;
    return {
      id: 'anonymous',
      year: anonCard.year,
      title: anonCard.title,
      category: anonCard.category,
      grid_size: gridSize,
      header_text: anonCard.header_text || 'BINGO',
      has_free_space: hasFreeSpace,
      free_space_position: hasFreeSpace
        ? (typeof anonCard.free_space_position === 'number' ? anonCard.free_space_position : defaultFreePos)
        : null,
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
        <div class="text-center mb-lg" style="padding-bottom: 1.5rem; border-bottom: 1px solid var(--border-color);">
            <button class="btn btn-secondary btn-lg" data-action="open-ai-wizard" style="width: 100%; display: flex; align-items: center; justify-content: center; gap: 0.5rem;">
                <span>✨</span> Generate with AI Wizard
            </button>
            <p class="text-muted mt-sm" style="font-size: 0.9rem;">Let AI create a custom card for you!</p>
        </div>

        <div class="card-header text-center">
          <h2 class="card-title">Create New Card</h2>
          <p class="card-subtitle">Set up your bingo card</p>
        </div>
        <form id="create-card-form" data-action="create-card">
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

  async renderCard(container, cardId, itemId = null) {
    container.innerHTML = `
      <div class="text-center"><div class="spinner" style="margin: 2rem auto;"></div></div>
    `;

    try {
      // Ensure we don't keep rendering server cards in anonymous mode due to stale state.
      this.isAnonymousMode = false;

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
      if (itemId) {
        await this.openItemById(itemId);
      }
    } catch (error) {
      container.innerHTML = `
        <div class="card text-center" style="padding: 3rem;">
          <h3>Card not found</h3>
          <p class="text-muted mb-lg" id="card-error-message"></p>
          <a href="#dashboard" class="btn btn-primary">Back to Dashboard</a>
        </div>
      `;
      const errorEl = document.getElementById('card-error-message');
      if (errorEl) errorEl.textContent = error.message;
    }
  },

  async openItemById(itemId) {
    if (!this.currentCard?.items || !itemId) return;
    if (this.user && this.currentCard.is_finalized && !this.goalRemindersByItem?.[itemId]) {
      await this.loadGoalReminders(this.currentCard.id);
    }
    const item = this.currentCard.items.find(i => i.id === itemId);
    if (!item) return;
    const cell = document.querySelector(`[data-position=\"${item.position}\"]`);
    const content = item.content || cell?.querySelector('.bingo-cell-content')?.textContent || '';
    const isCompleted = item.is_completed || cell?.classList.contains('bingo-cell--completed');
    this.showItemDetailModal(item.position, content, isCompleted);
  },

  renderCardEditor(container) {
    this.currentView = 'card-editor';
    const itemCount = this.currentCard.items ? this.currentCard.items.length : 0;
    const gridSize = this.getGridSize(this.currentCard);
    const capacity = this.getCardCapacity(this.currentCard);
    const progress = capacity ? Math.round((itemCount / capacity) * 100) : 0;
    const displayName = this.getCardDisplayName(this.currentCard);
    const categoryBadge = this.getCategoryBadge(this.currentCard);
    const isAnon = this.isAnonymousMode;

    container.innerHTML = `
      ${this.user && !this.user.email_verified
        ? this.renderEmailVerificationBanner()
        : isAnon ? `
        <div class="anonymous-card-banner">
          <div class="anonymous-card-banner-content">
            <span class="anonymous-card-banner-icon">💾</span>
            <span>
              This card is saved locally in your browser.
              <a href="#register" class="anonymous-card-banner-link">Create an account</a> to save it permanently and unlock the AI Goal Wizard.
            </span>
          </div>
        </div>
      ` : ''}

      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
        <a href="${isAnon ? '#home' : '#dashboard'}" class="btn btn-ghost">&larr; Back</a>
        <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; justify-content: center;">
          <h2 style="margin: 0;">${displayName}</h2>
          <span class="year-badge">${this.currentCard.year}</span>
          ${categoryBadge}
          <button class="btn btn-ghost btn-sm" data-action="edit-card-meta" title="Edit card name">✏️</button>
        </div>
        ${!isAnon ? `
          <button class="visibility-toggle-btn ${this.currentCard.visible_to_friends ? 'visibility-toggle-btn--visible' : 'visibility-toggle-btn--private'}" data-action="toggle-card-visibility" data-card-id="${this.currentCard.id}" data-visible="${!this.currentCard.visible_to_friends}">
            <i class="fas fa-${this.currentCard.visible_to_friends ? 'eye' : 'eye-slash'}"></i>
            <span>${this.currentCard.visible_to_friends ? 'Visible to friends' : 'Private'}</span>
          </button>
        ` : '<div></div>'}
      </div>

      <div class="progress-bar">
        <div class="progress-fill" style="width: ${progress}%"></div>
      </div>
      <p class="progress-text mb-lg">${itemCount}/${capacity} items added</p>

      <div class="card-editor-layout">
        <div class="bingo-container editor-grid">
          <div class="bingo-grid" id="bingo-grid" style="--grid-size: ${gridSize};">
            ${this.renderGrid()}
          </div>
        </div>

        <div class="editor-sidebar">
          <div class="input-area editor-input">
            <input type="text" id="item-input" class="form-input" placeholder="Type your goal..." maxlength="500" ${itemCount >= capacity ? 'disabled' : ''}>
            <button class="btn btn-primary" id="add-btn" ${itemCount >= capacity ? 'disabled' : ''}>Add</button>
          </div>

          <div class="card-config-panel" style="margin-top: 0.75rem;">
            <div class="form-group" style="margin-bottom: 0.75rem;">
              <label class="form-label">Header</label>
              <input type="text" id="card-header-input" class="form-input" maxlength="${gridSize}">
              <small class="text-muted">1-${gridSize} characters.</small>
            </div>
            <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer; user-select: none;">
              <input type="checkbox" id="card-free-toggle" ${this.getHasFreeSpace(this.currentCard) ? 'checked' : ''}>
              <span>Include FREE space</span>
            </label>
          </div>

          <div class="action-bar action-bar--side editor-actions">
            <button class="btn btn-secondary btn-danger-outline" id="clear-btn" data-action="confirm-clear-card-items" ${itemCount === 0 ? 'disabled' : ''}>
              🧹 Clear
            </button>
            <button class="btn btn-secondary" id="shuffle-btn" data-action="shuffle-card" ${itemCount === 0 ? 'disabled' : ''}>
              🔀 Shuffle
            </button>
            ${!isAnon ? `
              <button class="btn btn-secondary" data-action="show-clone-card-modal">
                📄 Clone
              </button>
            ` : ''}
            <button class="btn btn-primary" id="finalize-btn" data-action="finalize-card" ${itemCount < capacity ? 'disabled' : ''}>
              ✓ Finalize Card
            </button>
          </div>

          <div class="suggestions-panel editor-suggestions">
            <div class="suggestions-header">
              <h3 class="suggestions-title">Suggestions</h3>
              <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
                ${isAnon ? `
                  <button class="btn btn-secondary btn-sm" data-action="show-ai-auth-modal" title="Create an account to use AI features">
                    🧙 AI
                  </button>
                ` : `
                  <button class="btn btn-secondary btn-sm" id="ai-btn" data-action="open-ai-wizard" data-card-id="${this.currentCard.id}" data-desired-count="${capacity - itemCount}" title="Generate goals with AI" ${itemCount >= capacity ? 'disabled' : ''}>
                    🧙 AI
                  </button>
                `}
                <button class="btn btn-secondary btn-sm" id="fill-empty-btn" data-action="fill-empty-spaces" ${itemCount >= capacity ? 'disabled' : ''}>
                  ✨ Fill
                </button>
              </div>
            </div>
            <div class="suggestions-categories" id="category-tabs">
              ${this.suggestions.map((cat, i) => `
                <button class="category-tab ${i === 0 ? 'category-tab--active' : ''}" data-index="${i}">
                  ${this.escapeHtml(cat.category.split(' ')[0])}
                </button>
              `).join('')}
            </div>
            <div class="suggestions-list" id="suggestions-list">
              ${this.renderSuggestions(0)}
            </div>
          </div>

          ${isAnon ? `
            <div class="editor-delete">
              <button class="btn btn-ghost" style="color: var(--color-danger);" data-action="confirm-delete-anonymous-card">
                Delete Card
              </button>
            </div>
          ` : ''}
        </div>
      </div>
    `;

    const headerInput = document.getElementById('card-header-input');
    if (headerInput) headerInput.value = this.getHeaderText(this.currentCard);

    this.setupEditorEvents();
  },

  confirmClearCardItems() {
    const itemCount = this.currentCard?.items ? this.currentCard.items.length : 0;
    if (itemCount === 0) return;

    this.openModal('Clear Card', `
      <div class="finalize-confirm-modal">
        <p style="margin-bottom: 1.5rem;">
          Clear all ${itemCount} items from this card? This can't be undone.
        </p>
        <div style="display: flex; gap: 1rem; justify-content: flex-end;">
          <button class="btn btn-ghost" data-action="close-modal">Cancel</button>
          <button class="btn btn-primary" style="background: var(--color-danger); border-color: var(--color-danger);" data-action="clear-card-items">Clear All</button>
        </div>
      </div>
    `);
  },

  async clearCardItems() {
    const items = this.currentCard?.items ? [...this.currentCard.items] : [];
    if (items.length === 0) {
      this.closeModal();
      return;
    }

    try {
      if (this.isAnonymousMode) {
        const ok = AnonymousCard.clearItems();
        if (!ok) {
          throw new Error('No card found to clear.');
        }
        const anonCard = AnonymousCard.get();
        this.currentCard = this.convertAnonymousCardToAppFormat(anonCard);
      } else {
        await Promise.all(items.map(item => API.cards.removeItem(this.currentCard.id, item.position)));
        this.currentCard.items = [];
      }

      this.usedSuggestions = new Set();

      this.closeModal();
      const container = document.getElementById('main-container');
      if (container) {
        this.renderCardEditor(container);
      }
      this.toast('Card cleared', 'success');
    } catch (error) {
      this.toast(error.message, 'error');

      if (!this.isAnonymousMode && this.currentCard?.id) {
        try {
          const response = await API.cards.get(this.currentCard.id);
          if (response?.card) {
            this.currentCard = response.card;
            this.usedSuggestions = new Set((this.currentCard.items || []).map(i => (i.content || '').toLowerCase()));
            this.closeModal();
            const container = document.getElementById('main-container');
            if (container) {
              this.renderCardEditor(container);
            }
          }
        } catch (refreshError) {
          this.toast('Failed to refresh card state: ' + refreshError.message, 'error');
        }
      }
    }
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
      <form data-action="save-anon-card-meta">
        <div class="form-group">
          <label for="edit-card-title">Title</label>
          <input type="text" id="edit-card-title" class="form-input"
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
          <button type="button" class="btn btn-ghost" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary">Save</button>
        </div>
      </form>
    `);
    const titleInput = document.getElementById('edit-card-title');
    if (titleInput) titleInput.value = currentTitle;
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

  renderFinalizedCard(container, options = {}) {
    const readOnly = !!options.readOnly;
    this.currentView = readOnly ? 'shared-card' : 'finalized-card';
    const completedCount = this.currentCard.items.filter(i => i.is_completed).length;
    const gridSize = this.getGridSize(this.currentCard);
    const capacity = this.getCardCapacity(this.currentCard);
    const progress = capacity ? Math.round((completedCount / capacity) * 100) : 0;
    const displayName = this.getCardDisplayName(this.currentCard);
    const categoryBadge = this.getCategoryBadge(this.currentCard);

    const sharedView = !!options.shared;
    const showActions = !readOnly && this.user && !this.isAnonymousMode;
    const backLink = sharedView ? '#home' : '#dashboard';
    const backLabel = sharedView ? 'Home' : 'Back';
    const sharedBadge = sharedView ? '<span class="badge badge-warning">Shared view</span>' : '';

    let actionsHtml = '';
    if (showActions) {
      const visibilityIcon = this.currentCard.visible_to_friends ? 'eye' : 'eye-slash';
      const visibilityLabel = this.currentCard.visible_to_friends ? 'Visible to friends' : 'Private';
      actionsHtml = `
        <button class="btn btn-ghost btn-sm" data-action="edit-card-meta" title="Edit card name">✏️</button>
        <button class="btn btn-ghost btn-sm" data-action="show-clone-card-modal" title="Clone card">📄</button>
        <button class="btn btn-ghost btn-sm" data-action="open-share-modal" title="Share card">🔗</button>
        <button class="visibility-toggle-btn ${this.currentCard.visible_to_friends ? 'visibility-toggle-btn--visible' : 'visibility-toggle-btn--private'}" data-action="toggle-card-visibility" data-card-id="${this.currentCard.id}" data-visible="${!this.currentCard.visible_to_friends}" title="${visibilityLabel}">
          <i class="fas fa-${visibilityIcon}"></i>
          <span>${visibilityLabel}</span>
        </button>
      `;
    }

    container.innerHTML = `
      <div class="finalized-card-view">
        <div class="finalized-card-header">
          <a href="${backLink}" class="btn btn-ghost">&larr; ${backLabel}</a>
          <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; justify-content: center;">
            <h2 style="margin: 0;">${displayName}</h2>
            <span class="year-badge">${this.currentCard.year}</span>
            ${categoryBadge}
            ${sharedBadge}
          </div>
          <div class="card-header-actions">
            ${actionsHtml}
          </div>
        </div>

        <div class="bingo-container bingo-container--finalized">
          <div class="bingo-grid bingo-grid--finalized" id="bingo-grid" style="--grid-size: ${gridSize};">
            ${this.renderGrid(true)}
          </div>
        </div>

        <div class="finalized-card-progress">
          <div class="progress-bar">
            <div class="progress-fill" style="width: ${progress}%"></div>
          </div>
          <p class="progress-text">${completedCount}/${capacity} completed</p>
        </div>
      </div>
    `;

    this.setupFinalizedCardEvents({ readOnly });
    if (this.user && !this.isAnonymousMode && !readOnly) {
      this.loadGoalReminders(this.currentCard.id);
    }
  },

  async renderSharedCard(container, token) {
    this.currentView = 'shared-card';
    this.isSharedView = true;
    this.isAnonymousMode = false; // Shared views are read-only; anon mode is for localStorage edits.

    if (!token) {
      container.innerHTML = `
        <div class="card text-center" style="padding: 3rem;">
          <h3>Invalid Share Link</h3>
          <p class="text-muted mb-lg">This share link is missing or malformed.</p>
          <a href="#home" class="btn btn-primary">Back to Home</a>
        </div>
      `;
      return;
    }

    this.showLoading(container, 'Loading shared card...');
    try {
      const response = await API.share.get(token);
      const items = response.items || [];
      this.currentCard = response.card || {};
      this.currentCard.items = items;
      this.renderFinalizedCard(container, { readOnly: true, shared: true });
    } catch (error) {
      container.innerHTML = `
        <div class="card text-center" style="padding: 3rem;">
          <h3>Share Link Not Found</h3>
          <p class="text-muted mb-lg" id="share-error"></p>
          <a href="#home" class="btn btn-primary">Back to Home</a>
        </div>
      `;
      const errorEl = document.getElementById('share-error');
      if (errorEl) errorEl.textContent = error.message;
    }
  },

  getGridSize(card = this.currentCard) {
    const n = Number(card?.grid_size);
    return Number.isFinite(n) && n >= 2 && n <= 5 ? n : 5;
  },

  getHasFreeSpace(card = this.currentCard) {
    return card?.has_free_space !== false;
  },

  getFreeSpacePosition(card = this.currentCard) {
    if (!this.getHasFreeSpace(card)) return null;
    const n = this.getGridSize(card);
    const total = n * n;
    const pos = Number(card?.free_space_position);
    if (Number.isFinite(pos) && pos >= 0 && pos < total) return pos;
    return n % 2 === 1 ? Math.floor(total / 2) : 0;
  },

  getCardCapacity(card = this.currentCard) {
    const n = this.getGridSize(card);
    const total = n * n;
    return this.getHasFreeSpace(card) ? total - 1 : total;
  },

  getHeaderText(card = this.currentCard) {
    const n = this.getGridSize(card);
    const raw = (card?.header_text || 'BINGO').toString().trim().toUpperCase();
    const letters = Array.from(raw);
    const sliced = letters.slice(0, n).join('');
    if (sliced) return sliced;
    return Array.from('BINGO').slice(0, n).join('');
  },

  renderGrid(finalized = false) {
    const gridSize = this.getGridSize(this.currentCard);
    const hasFreeSpace = this.getHasFreeSpace(this.currentCard);
    const freePos = this.getFreeSpacePosition(this.currentCard);

    const headerLetters = Array.from(this.getHeaderText(this.currentCard));
    const headerRow = Array.from({ length: gridSize }).map((_, i) => `
      <div class="bingo-header">${this.escapeHtml(headerLetters[i] || '')}</div>
    `).join('');

    const cells = [];
    const itemsByPosition = {};

    if (this.currentCard.items) {
      this.currentCard.items.forEach(item => {
        itemsByPosition[item.position] = item;
      });
    }

    for (let i = 0; i < gridSize * gridSize; i++) {
      if (hasFreeSpace && i === freePos) {
        const draggable = !finalized ? 'draggable="true"' : '';
        cells.push(`
          <div class="bingo-cell bingo-cell--free" data-position="${i}" ${draggable}>
            <span class="bingo-cell-content">FREE</span>
          </div>
        `);
      } else {
        const item = itemsByPosition[i];
        if (item) {
          const isCompleted = item.is_completed;
          const shortText = this.truncateText(item.content, 50);
          const itemIdAttr = item.id ? `data-item-id="${item.id}"` : '';
          cells.push(`
            <div class="bingo-cell ${isCompleted ? 'bingo-cell--completed' : ''}"
                 data-position="${i}"
                 ${itemIdAttr}
                 ${!finalized ? 'draggable="true"' : ''}
                 >
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

  renderSuggestions(categoryIndex = 0) {
    const categoryData = this.suggestions[categoryIndex];
    if (!categoryData) return '<p class="text-muted">No suggestions available</p>';

    return categoryData.suggestions.map(suggestion => {
      const isUsed = this.usedSuggestions.has(suggestion.content.toLowerCase());
      const actionAttr = isUsed ? '' : 'data-action="add-suggestion"';
      const disabledAttr = isUsed ? 'aria-disabled="true"' : '';
      return `
        <div class="suggestion-item ${isUsed ? 'suggestion-item--used' : ''}"
             ${actionAttr} ${disabledAttr}>
          ${this.escapeHtml(suggestion.content)}
        </div>
      `;
    }).join('');
  },

  getActiveSuggestionIndex() {
    const activeTab = document.querySelector('.category-tab--active');
    if (!activeTab) return 0;
    const rawIndex = parseInt(activeTab.dataset.index, 10);
    const count = Array.isArray(this.suggestions) ? this.suggestions.length : 0;
    if (count === 0) return 0;
    let index = Number.isNaN(rawIndex) ? 0 : rawIndex;
    if (index < 0) index = 0;
    if (index >= count) index = count - 1;
    return index;
  },

  refreshSuggestionsList() {
    const list = document.getElementById('suggestions-list');
    if (!list) return;
    list.innerHTML = this.renderSuggestions(this.getActiveSuggestionIndex());
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
        const rawIndex = parseInt(e.target.dataset.index, 10);
        const count = Array.isArray(this.suggestions) ? this.suggestions.length : 0;
        let index = Number.isNaN(rawIndex) ? 0 : rawIndex;
        if (index < 0 || index >= count) index = 0;
        document.getElementById('suggestions-list').innerHTML = this.renderSuggestions(index);
      }
    });

    // Draft-only card config (header/FREE)
    const headerInput = document.getElementById('card-header-input');
    if (headerInput) {
      headerInput.addEventListener('change', async () => {
        await this.updateDraftConfig({ headerText: headerInput.value });
      });
    }
    const freeToggle = document.getElementById('card-free-toggle');
    if (freeToggle) {
      freeToggle.addEventListener('change', async () => {
        await this.updateDraftConfig({ hasFreeSpace: freeToggle.checked });
      });
    }

    // Drag and drop
    this.setupDragAndDrop();

    // Cell click to add/edit (before finalized)
    document.getElementById('bingo-grid').addEventListener('click', (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (cell && !cell.classList.contains('bingo-cell--free')) {
        this.showItemOptions(cell);
      }
    });
  },

  setupFinalizedCardEvents({ readOnly = false } = {}) {
    document.getElementById('bingo-grid').addEventListener('click', async (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (!cell || cell.classList.contains('bingo-cell--free') || cell.classList.contains('bingo-cell--empty')) return;

      const position = parseInt(cell.dataset.position);
      const item = this.currentCard.items?.find(i => i.position === position);
      const content = item?.content || cell.querySelector('.bingo-cell-content')?.textContent || '';
      const isCompleted = cell.classList.contains('bingo-cell--completed');

      if (readOnly) {
        this.showSharedItemModal(content, isCompleted);
        return;
      }

      // Show item detail modal
      this.showItemDetailModal(position, content, isCompleted);
    });
  },

  showSharedItemModal(content, isCompleted) {
    const statusText = isCompleted ? 'Completed' : 'Not completed yet';
    const statusClass = isCompleted ? 'badge badge-success' : 'badge badge-warning';
    this.openModal(isCompleted ? 'Completed Goal' : 'Goal', `
      <div class="item-detail">
        <p class="item-detail-content">${this.escapeHtml(content)}</p>
        <p style="margin-top: 1rem;"><span class="${statusClass}">${statusText}</span></p>
      </div>
      <div style="margin-top: 1.5rem;">
        <button type="button" class="btn btn-secondary" style="width: 100%;" data-action="close-modal">
          Close
        </button>
      </div>
    `);
  },

  renderGoalReminderControls(item) {
    if (!this.user || this.isAnonymousMode || !this.currentCard?.is_finalized || !item || item.is_completed) {
      return '';
    }

    const existing = this.goalRemindersByItem?.[item.id];
    const status = existing?.next_send_at
      ? `Next reminder: ${this.escapeHtml(this.formatReminderTimestamp(existing.next_send_at))}`
      : '';
    const disable = !this.user.email_verified;
    const disableAttr = disable ? 'disabled' : '';
    const note = disable ? '<p class="text-muted">Verify your email to enable reminders.</p>' : '';
    const header = existing ? 'Edit reminder' : 'Remind me';

    return `
      <div class="reminder-modal">
        <h3>${header}</h3>
        ${note}
        ${status ? `<p class="text-muted" id="goal-reminder-status">${status}</p>` : ''}
        <div class="reminder-presets">
          <button type="button" class="btn btn-secondary btn-sm" data-action="set-goal-reminder" data-item-id="${item.id}" data-preset="tomorrow" ${disableAttr}>Tomorrow morning</button>
          <button type="button" class="btn btn-secondary btn-sm" data-action="set-goal-reminder" data-item-id="${item.id}" data-preset="week" ${disableAttr}>Next week</button>
          <button type="button" class="btn btn-secondary btn-sm" data-action="set-goal-reminder" data-item-id="${item.id}" data-preset="month" ${disableAttr}>Next month</button>
        </div>
        <div class="reminder-custom">
          <input type="datetime-local" id="reminder-custom-datetime" class="form-input" ${disableAttr}>
          <button type="button" class="btn btn-secondary btn-sm" data-action="set-goal-reminder" data-item-id="${item.id}" data-preset="custom" ${disableAttr}>Set custom reminder</button>
        </div>
        <p class="text-muted">Reminder times use the server clock.</p>
        ${existing ? `
          <button type="button" class="btn btn-ghost btn-sm" data-action="delete-goal-reminder" data-reminder-id="${existing.id}" ${disableAttr}>Stop reminders for this goal</button>
        ` : ''}
      </div>
    `;
  },

  showItemDetailModal(position, content, isCompleted) {
    const item = this.currentCard.items?.find(i => i.position === position);
    const notes = item?.notes || '';

    const reminderControls = this.renderGoalReminderControls(item);

    if (isCompleted) {
      this.openModal('Goal Completed!', `
        <div class="item-detail">
          <p class="item-detail-content">${this.escapeHtml(content)}</p>
          ${notes ? `<p class="item-detail-notes"><strong>Notes:</strong> ${this.escapeHtml(notes)}</p>` : ''}
        </div>
        ${reminderControls}
        <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-secondary" style="flex: 1;" data-action="close-modal">
            Close
          </button>
          <button type="button" class="btn btn-ghost" style="flex: 1;" data-action="uncomplete-item" data-position="${position}">
            Mark Incomplete
          </button>
        </div>
      `);
    } else {
      this.openModal('Mark Complete', `
        <div class="item-detail">
          <p class="item-detail-content">${this.escapeHtml(content)}</p>
        </div>
        ${reminderControls}
        <form id="complete-form">
          <div class="form-group" style="margin-top: 1rem;">
            <label class="form-label">Notes (optional)</label>
            <textarea id="complete-notes" class="form-input" rows="3" placeholder="How did you accomplish this?"></textarea>
          </div>
          <div style="display: flex; gap: 1rem;">
            <button type="button" class="btn btn-secondary" style="flex: 1;" data-action="close-modal">
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
      const capacity = this.getCardCapacity(this.currentCard);
      const progress = capacity ? Math.round((completedCount / capacity) * 100) : 0;
      document.querySelector('.progress-fill').style.width = `${progress}%`;
      document.querySelector('.progress-text').textContent = `${completedCount}/${capacity} completed`;

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
      const capacity = this.getCardCapacity(this.currentCard);
      const progress = capacity ? Math.round((completedCount / capacity) * 100) : 0;
      document.querySelector('.progress-fill').style.width = `${progress}%`;
      document.querySelector('.progress-text').textContent = `${completedCount}/${capacity} completed`;
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  setupDragAndDrop() {
    const grid = document.getElementById('bingo-grid');
    let draggedCell = null;

    grid.addEventListener('dragstart', (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (!cell || cell.classList.contains('bingo-cell--empty')) {
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
        if (this.isAnonymousMode) {
          // Use localStorage for anonymous cards
          AnonymousCard.swapItems(fromPosition, toPosition);
          const anonCard = AnonymousCard.get();
          this.currentCard = this.convertAnonymousCardToAppFormat(anonCard);
        } else {
          // Use swap API - handles both moving to empty cells and swapping with filled cells
          await API.cards.swap(this.currentCard.id, fromPosition, toPosition);
          const response = await API.cards.get(this.currentCard.id);
          this.currentCard = response.card;
        }
        document.getElementById('bingo-grid').innerHTML = this.renderGrid();
      } catch (error) {
        this.toast(error.message, 'error');
      }
    });

    // Touch event handling for mobile drag and drop (only setup once)
    if (!grid.dataset.touchSetup) {
      grid.dataset.touchSetup = 'true';
      this.setupTouchDragAndDrop(grid);
    }
  },

  setupTouchDragAndDrop(grid) {
    let touchDraggedCell = null;
    let touchClone = null;
    let touchStartTimer = null;
    let touchStartPos = { x: 0, y: 0 };
    let isDragging = false;
    const LONG_PRESS_DELAY = 300; // ms
    const MOVE_THRESHOLD = 10; // pixels before cancelling long press

    const getCellAtPoint = (x, y) => {
      // Hide clone temporarily to get element underneath
      if (touchClone) touchClone.style.display = 'none';
      const element = document.elementFromPoint(x, y);
      if (touchClone) touchClone.style.display = '';
      return element?.closest('.bingo-cell');
    };

    const createDragClone = (cell, x, y) => {
      const clone = cell.cloneNode(true);
      clone.className = 'bingo-cell bingo-cell--drag-clone';
      clone.style.cssText = `
        position: fixed;
        width: ${cell.offsetWidth}px;
        height: ${cell.offsetHeight}px;
        left: ${x - cell.offsetWidth / 2}px;
        top: ${y - cell.offsetHeight / 2}px;
        z-index: 10000;
        pointer-events: none;
        opacity: 0.9;
        transform: scale(1.05);
        box-shadow: 0 8px 32px rgba(0,0,0,0.4);
      `;
      document.body.appendChild(clone);
      return clone;
    };

    const cleanupDrag = () => {
      if (touchClone) {
        touchClone.remove();
        touchClone = null;
      }
      if (touchDraggedCell) {
        touchDraggedCell.classList.remove('bingo-cell--dragging');
        touchDraggedCell = null;
      }
      document.querySelectorAll('.bingo-cell--drag-over').forEach(c => c.classList.remove('bingo-cell--drag-over'));
      isDragging = false;
      if (touchStartTimer) {
        clearTimeout(touchStartTimer);
        touchStartTimer = null;
      }
    };

    grid.addEventListener('touchstart', (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (!cell || cell.classList.contains('bingo-cell--empty')) {
        return;
      }
      if (!cell.hasAttribute('draggable')) return;

      const touch = e.touches[0];
      touchStartPos = { x: touch.clientX, y: touch.clientY };

      // Start long press timer
      touchStartTimer = setTimeout(() => {
        isDragging = true;
        touchDraggedCell = cell;
        cell.classList.add('bingo-cell--dragging');
        touchClone = createDragClone(cell, touch.clientX, touch.clientY);

        // Haptic feedback if available
        if (navigator.vibrate) navigator.vibrate(50);
      }, LONG_PRESS_DELAY);
    }, { passive: true });

    grid.addEventListener('touchmove', (e) => {
      const touch = e.touches[0];

      // Cancel long press if moved too much before timer fires
      if (!isDragging && touchStartTimer) {
        const dx = Math.abs(touch.clientX - touchStartPos.x);
        const dy = Math.abs(touch.clientY - touchStartPos.y);
        if (dx > MOVE_THRESHOLD || dy > MOVE_THRESHOLD) {
          clearTimeout(touchStartTimer);
          touchStartTimer = null;
        }
        return;
      }

      if (!isDragging || !touchClone) return;

      e.preventDefault();

      // Move the clone
      touchClone.style.left = `${touch.clientX - touchClone.offsetWidth / 2}px`;
      touchClone.style.top = `${touch.clientY - touchClone.offsetHeight / 2}px`;

      // Highlight cell under finger
      document.querySelectorAll('.bingo-cell--drag-over').forEach(c => c.classList.remove('bingo-cell--drag-over'));
      const cellUnder = getCellAtPoint(touch.clientX, touch.clientY);
      if (cellUnder && cellUnder !== touchDraggedCell && !cellUnder.classList.contains('bingo-cell--free')) {
        cellUnder.classList.add('bingo-cell--drag-over');
      }
    }, { passive: false });

    grid.addEventListener('touchend', async (e) => {
      if (touchStartTimer) {
        clearTimeout(touchStartTimer);
        touchStartTimer = null;
      }

      if (!isDragging || !touchDraggedCell) {
        cleanupDrag();
        return;
      }

      const touch = e.changedTouches[0];
      const targetCell = getCellAtPoint(touch.clientX, touch.clientY);

      if (!targetCell || targetCell === touchDraggedCell || targetCell.classList.contains('bingo-cell--free')) {
        cleanupDrag();
        return;
      }

      const fromPosition = parseInt(touchDraggedCell.dataset.position);
      const toPosition = parseInt(targetCell.dataset.position);

      cleanupDrag();

      try {
        if (this.isAnonymousMode) {
          // Use localStorage for anonymous cards
          AnonymousCard.swapItems(fromPosition, toPosition);
          const anonCard = AnonymousCard.get();
          this.currentCard = this.convertAnonymousCardToAppFormat(anonCard);
        } else {
          // Use swap API - handles both moving to empty cells and swapping with filled cells
          await API.cards.swap(this.currentCard.id, fromPosition, toPosition);
          const response = await API.cards.get(this.currentCard.id);
          this.currentCard = response.card;
        }
        document.getElementById('bingo-grid').innerHTML = this.renderGrid();
      } catch (error) {
        this.toast(error.message, 'error');
      }
    });

    grid.addEventListener('touchcancel', () => {
      cleanupDrag();
    });
  },

  showItemOptions(cell) {
    const position = parseInt(cell.dataset.position, 10);
    const item = this.currentCard.items?.find(i => i.position === position);
    const content = item?.content || '';
    const isEmpty = cell.classList.contains('bingo-cell--empty');
    const modalTitle = isEmpty ? 'Add Goal' : 'Edit Goal';
    const aiButtonLabel = isEmpty ? '🧙 Suggest with AI' : '🧙 Refine with AI';
    const aiHintPlaceholder = isEmpty ? 'Theme or constraint (optional)' : 'What should change? (optional)';

    const aiSection = `
        <div class="form-group ai-guide-section">
          <label class="form-label">AI Assist</label>
          <input type="text" id="ai-refine-hint" class="form-input form-input--sm" placeholder="${aiHintPlaceholder}" maxlength="500">
          <button type="button" class="btn btn-secondary btn-sm" id="ai-refine-generate" data-action="ai-refine" data-position="${position}">
            ${aiButtonLabel}
          </button>
          <div id="ai-refine-results" class="ai-guide-results"></div>
        </div>
    `;
    const removeButton = `
          <button type="button" class="btn btn-primary" style="flex: 1; background: var(--color-error);" data-action="remove-item" data-position="${position}" ${isEmpty ? 'disabled aria-disabled="true" title="No goal to remove"' : ''}>
            Remove
          </button>
    `;

    this.openModal(modalTitle, `
      <form data-action="save-item-edit" data-position="${position}">
        <div class="form-group">
          <label class="form-label" for="edit-item-content-${position}">Goal</label>
          <textarea id="edit-item-content-${position}" class="form-input" rows="4" maxlength="500" autofocus>${this.escapeHtml(content)}</textarea>
        </div>
        ${aiSection}
        <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-secondary" style="flex: 1;" data-action="close-modal">
            Cancel
          </button>
          ${removeButton}
          <button type="submit" class="btn btn-primary" style="flex: 1;">
            Save
          </button>
        </div>
      </form>
    `);
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

  updateUsedSuggestionsForContentChange(position, oldContent, newContent) {
    const oldKey = (oldContent || '').toLowerCase();
    const newKey = (newContent || '').toLowerCase();
    if (!oldKey || !newKey || oldKey === newKey) return;

    const stillUsesOld = (this.currentCard.items || []).some(
      i => i.position !== position && (i.content || '').toLowerCase() === oldKey
    );
    if (!stillUsesOld) {
      this.usedSuggestions.delete(oldKey);
    }
    this.usedSuggestions.add(newKey);
  },

  async addItemAtPosition(position, content) {
    const items = this.currentCard?.items || [];
    if (items.some(i => i.position === position)) {
      this.toast('That cell already has a goal', 'error');
      return;
    }

    const capacity = this.getCardCapacity(this.currentCard);
    if (items.length >= capacity) {
      this.toast('Card is full', 'error');
      return;
    }

    try {
      let newItem;

      if (this.isAnonymousMode) {
        const item = AnonymousCard.addItem(content, position);
        if (!item) {
          throw new Error('Failed to add goal');
        }
        newItem = {
          id: `anon-${item.position}`,
          position: item.position,
          content: content,
          notes: '',
          is_completed: false,
        };
      } else {
        const response = await API.cards.addItem(this.currentCard.id, content, position);
        newItem = response.item;
      }

      if (!this.currentCard.items) this.currentCard.items = [];
      this.currentCard.items.push(newItem);
      this.usedSuggestions.add(content.toLowerCase());

      const cell = document.querySelector(`[data-position="${position}"]`);
      if (cell) {
        const shortText = this.truncateText(content, 50);
        cell.classList.remove('bingo-cell--empty');
        cell.classList.add('bingo-cell--appearing');
        cell.dataset.itemId = this.isAnonymousMode ? `anon-${position}` : newItem.id;
        cell.title = content;
        cell.draggable = true;
        cell.innerHTML = '';
        const contentEl = document.createElement('span');
        contentEl.className = 'bingo-cell-content';
        contentEl.textContent = shortText;
        cell.appendChild(contentEl);
      }

      const itemCount = this.currentCard.items.length;
      const progress = capacity ? Math.round((itemCount / capacity) * 100) : 0;
      document.querySelector('.progress-fill').style.width = `${progress}%`;
      document.querySelector('.progress-text').textContent = `${itemCount}/${capacity} items added`;

      const isFull = itemCount >= capacity;
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

      this.closeModal();
      this.toast('Goal added', 'success');
      this.confetti();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async saveItemEdit(event, position) {
    event.preventDefault();

    const textarea = document.getElementById(`edit-item-content-${position}`);
    if (!textarea) return;

    const newContent = textarea.value.trim();
    if (!newContent) {
      this.toast('Goal cannot be empty', 'error');
      return;
    }
    if (newContent.length > 500) {
      this.toast('Goal must be 500 characters or less', 'error');
      return;
    }

    const item = this.currentCard.items?.find(i => i.position === position);
    if (!item) {
      await this.addItemAtPosition(position, newContent);
      return;
    }
    const oldContent = item.content || '';

    if (newContent === oldContent) {
      this.closeModal();
      return;
    }

    try {
      if (this.isAnonymousMode) {
        const ok = AnonymousCard.updateItem(position, newContent);
        if (!ok) throw new Error('Failed to update goal');
        item.content = newContent;
      } else {
        const response = await API.cards.updateItem(this.currentCard.id, position, { content: newContent });
        if (response?.item) {
          Object.assign(item, response.item);
        } else {
          item.content = newContent;
        }
      }

      this.updateUsedSuggestionsForContentChange(position, oldContent, item.content);

      const cell = document.querySelector(`.bingo-cell[data-position="${position}"]`);
      if (cell) {
        cell.title = item.content;
        const contentEl = cell.querySelector('.bingo-cell-content');
        if (contentEl) {
          contentEl.textContent = this.truncateText(item.content, 50);
        }
      }

      this.refreshSuggestionsList();

      this.closeModal();
      this.toast('Goal updated', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
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
      cell.title = content;
      cell.draggable = true;
      cell.innerHTML = '';
      const contentEl = document.createElement('span');
      contentEl.className = 'bingo-cell-content';
      contentEl.textContent = this.truncateText(content, 50);
      cell.appendChild(contentEl);

      // Update progress
      const itemCount = this.currentCard.items.length;
      const capacity = this.getCardCapacity(this.currentCard);
      const progress = capacity ? Math.round((itemCount / capacity) * 100) : 0;
      document.querySelector('.progress-fill').style.width = `${progress}%`;
      document.querySelector('.progress-text').textContent = `${itemCount}/${capacity} items added`;

      // Update buttons
      if (itemCount >= capacity) {
        input.disabled = true;
        document.getElementById('add-btn').disabled = true;
        document.getElementById('fill-empty-btn').disabled = true;
        const finalizeBtn = document.getElementById('finalize-btn');
        if (finalizeBtn) finalizeBtn.disabled = false;
      }
      const clearBtn = document.getElementById('clear-btn');
      if (clearBtn) clearBtn.disabled = itemCount === 0;
      const aiBtn = document.getElementById('ai-btn');
      if (aiBtn) aiBtn.disabled = itemCount >= capacity;
      const shuffleBtn = document.getElementById('shuffle-btn');
      if (shuffleBtn) shuffleBtn.disabled = false;

      // Update suggestions
      this.refreshSuggestionsList();

      this.confetti();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  addSuggestion(element) {
    if (!element || element.classList.contains('suggestion-item--used')) return;
    const content = element.textContent?.trim() || '';
    if (!content) return;
    document.getElementById('item-input').value = content;
    this.addItem();
  },

  async fillEmptySpaces() {
    const currentItemCount = this.currentCard.items ? this.currentCard.items.length : 0;
    const capacity = this.getCardCapacity(this.currentCard);
    const emptyCount = capacity - currentItemCount;

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
        cell.draggable = true;
        cell.title = content;
        cell.innerHTML = '';
        const contentEl = document.createElement('span');
        contentEl.className = 'bingo-cell-content';
        contentEl.textContent = this.truncateText(content, 50);
        cell.appendChild(contentEl);

        added++;
      } catch (error) {
        console.error('Failed to add item:', error);
        break;
      }
    }

    // Update progress
    const itemCount = this.currentCard.items.length;
    const progress = capacity ? Math.round((itemCount / capacity) * 100) : 0;
    document.querySelector('.progress-fill').style.width = `${progress}%`;
    document.querySelector('.progress-text').textContent = `${itemCount}/${capacity} items added`;

    // Update buttons
    const isFull = itemCount >= capacity;
    document.getElementById('item-input').disabled = isFull;
    document.getElementById('add-btn').disabled = isFull;
    document.getElementById('fill-empty-btn').disabled = isFull;
    const clearBtn = document.getElementById('clear-btn');
    if (clearBtn) clearBtn.disabled = itemCount === 0;
    const aiBtn = document.getElementById('ai-btn');
    if (aiBtn) aiBtn.disabled = isFull;
    const shuffleBtn = document.getElementById('shuffle-btn');
    if (shuffleBtn) shuffleBtn.disabled = itemCount === 0;
    const finalizeBtn = document.getElementById('finalize-btn');
    if (finalizeBtn) finalizeBtn.disabled = itemCount < capacity;

    // Update suggestions panel
    this.refreshSuggestionsList();

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
        const key = (item.content || '').toLowerCase();
        if (key) {
          const stillUsed = this.currentCard.items.some(i => (i.content || '').toLowerCase() === key);
          if (!stillUsed) this.usedSuggestions.delete(key);
        }
      }

      // Update grid
      const cell = document.querySelector(`[data-position="${position}"]`);
      cell.className = 'bingo-cell bingo-cell--empty';
      cell.removeAttribute('data-item-id');
      cell.removeAttribute('draggable');
      cell.innerHTML = '';

      // Update progress
      const itemCount = this.currentCard.items.length;
      const capacity = this.getCardCapacity(this.currentCard);
      const progress = capacity ? Math.round((itemCount / capacity) * 100) : 0;
      document.querySelector('.progress-fill').style.width = `${progress}%`;
      document.querySelector('.progress-text').textContent = `${itemCount}/${capacity} items added`;

      // Update buttons
      document.getElementById('item-input').disabled = false;
      document.getElementById('add-btn').disabled = false;
      document.getElementById('fill-empty-btn').disabled = false;
      const clearBtn = document.getElementById('clear-btn');
      if (clearBtn) clearBtn.disabled = itemCount === 0;
      const aiBtn = document.getElementById('ai-btn');
      if (aiBtn) aiBtn.disabled = itemCount >= capacity;
      const finalizeBtn = document.getElementById('finalize-btn');
      if (finalizeBtn) finalizeBtn.disabled = itemCount < capacity;
      if (itemCount === 0) {
        const shuffleBtn = document.getElementById('shuffle-btn');
        if (shuffleBtn) shuffleBtn.disabled = true;
      }

      // Update suggestions
      this.refreshSuggestionsList();

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

  async updateDraftConfig({ headerText = null, hasFreeSpace = null } = {}) {
    if (!this.currentCard || this.currentCard.is_finalized) return;

    const normalizedHeader = headerText !== null ? headerText.trim() : null;
    if (normalizedHeader !== null && normalizedHeader.length === 0) {
      this.toast('Header cannot be empty', 'error');
      const container = document.getElementById('main-container');
      if (container) this.renderCardEditor(container);
      return;
    }

    try {
      if (this.isAnonymousMode) {
        const updated = AnonymousCard.updateConfig({
          headerText: normalizedHeader,
          hasFreeSpace: typeof hasFreeSpace === 'boolean' ? hasFreeSpace : null,
        });
        if (!updated) {
          throw new Error('Unable to update card layout. Remove an item and try again.');
        }
        this.currentCard = this.convertAnonymousCardToAppFormat(updated);
      } else {
        const response = await API.cards.updateConfig(
          this.currentCard.id,
          normalizedHeader,
          typeof hasFreeSpace === 'boolean' ? hasFreeSpace : null
        );
        this.currentCard = response.card;
      }

      const container = document.getElementById('main-container');
      if (container) this.renderCardEditor(container);
    } catch (error) {
      this.toast(error.message, 'error');
      const container = document.getElementById('main-container');
      if (container) this.renderCardEditor(container);
    }
  },

  async showCloneCardModal() {
    if (!this.currentCard || this.isAnonymousMode) return;

    // Fetch categories
    let categories = [];
    try {
      const response = await API.cards.getCategories();
      categories = response.categories || [];
    } catch (error) {
      categories = this.getFallbackCategories();
    }

    const currentYear = new Date().getFullYear();
    const nextYear = currentYear + 1;

    const currentTitle = this.currentCard.title || '';
    const defaultTitle = currentTitle ? `${currentTitle} (Copy)` : `${this.currentCard.year} Bingo Card (Copy)`;
    const currentCategory = this.currentCard.category || '';

    const categoryOptions = categories.map(c => {
      const selected = c.id === currentCategory ? 'selected' : '';
      return `<option value="${this.escapeHtml(c.id)}" ${selected}>${this.escapeHtml(c.name)}</option>`;
    }).join('');

    const gridSize = this.getGridSize(this.currentCard);
    const headerText = this.getHeaderText(this.currentCard);
    const hasFree = this.getHasFreeSpace(this.currentCard);

    this.openModal('Clone Card', `
      <form data-action="clone-card">
        <div class="form-group">
          <label for="clone-card-year">Year</label>
          <select id="clone-card-year" class="form-input" required>
            <option value="${currentYear}" ${this.currentCard.year === currentYear ? 'selected' : ''}>${currentYear}</option>
            <option value="${nextYear}" ${this.currentCard.year === nextYear ? 'selected' : ''}>${nextYear}</option>
          </select>
        </div>

        <div class="form-group">
          <label for="clone-card-title">
            Title <span class="text-muted" style="font-weight: normal;">(optional)</span>
          </label>
          <input type="text" id="clone-card-title" class="form-input"
                 maxlength="100">
        </div>

        <div class="form-group">
          <label for="clone-card-category">
            Category <span class="text-muted" style="font-weight: normal;">(optional)</span>
          </label>
          <select id="clone-card-category" class="form-input">
            <option value="" ${!currentCategory ? 'selected' : ''}>None</option>
            ${categoryOptions}
          </select>
        </div>

        <div class="form-group">
          <label for="clone-card-grid-size">Grid Size</label>
          <select id="clone-card-grid-size" class="form-input">
            <option value="2" ${gridSize === 2 ? 'selected' : ''}>2x2</option>
            <option value="3" ${gridSize === 3 ? 'selected' : ''}>3x3</option>
            <option value="4" ${gridSize === 4 ? 'selected' : ''}>4x4</option>
            <option value="5" ${gridSize === 5 ? 'selected' : ''}>5x5</option>
          </select>
          <small class="text-muted">To change grid size, clone into a new card.</small>
        </div>

        <div class="form-group">
          <label class="checkbox-label" style="display: flex; align-items: center; gap: 0.5rem;">
            <input type="checkbox" id="clone-card-free-space" ${hasFree ? 'checked' : ''}>
            <span>Include FREE space</span>
          </label>
        </div>

        <div class="form-group">
          <label for="clone-card-header">Header</label>
          <input type="text" id="clone-card-header" class="form-input" maxlength="${gridSize}" required>
          <small class="text-muted" id="clone-card-header-help">1-${gridSize} characters.</small>
        </div>

        <div style="display: flex; gap: 0.5rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-ghost" style="flex: 1;" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary" style="flex: 1;">Clone</button>
        </div>
      </form>
    `);

    const gridSizeEl = document.getElementById('clone-card-grid-size');
    const headerEl = document.getElementById('clone-card-header');
    const headerHelpEl = document.getElementById('clone-card-header-help');
    const titleEl = document.getElementById('clone-card-title');
    if (titleEl) titleEl.value = defaultTitle;
    if (headerEl) headerEl.value = headerText;
    if (gridSizeEl && headerEl) {
      const apply = () => {
        const n = parseInt(gridSizeEl.value, 10) || 5;
        headerEl.maxLength = n;
        if (headerHelpEl) headerHelpEl.textContent = `1-${n} characters.`;
        if (headerEl.value.length > n) headerEl.value = Array.from(headerEl.value).slice(0, n).join('');
      };
      gridSizeEl.addEventListener('change', apply);
      apply();
    }
  },

  async handleCloneCard(event) {
    event.preventDefault();
    if (!this.currentCard) return;

    const year = parseInt(document.getElementById('clone-card-year').value, 10);
    const title = document.getElementById('clone-card-title').value.trim() || null;
    const category = document.getElementById('clone-card-category').value || null;
    const gridSize = parseInt(document.getElementById('clone-card-grid-size').value, 10);
    const hasFreeSpace = !!document.getElementById('clone-card-free-space').checked;
    const headerText = document.getElementById('clone-card-header').value.trim();

    try {
      const response = await API.cards.clone(this.currentCard.id, {
        year,
        title,
        category,
        grid_size: gridSize,
        has_free_space: hasFreeSpace,
        header_text: headerText,
      });

      this.closeModal();
      this.currentCard = response.card;
      window.location.hash = `#card/${response.card.id}`;
      if (response.message) this.toast(response.message, 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async showShareCardModal() {
    if (!this.currentCard || this.isAnonymousMode || this.isSharedView || !this.currentCard.is_finalized) return;

    this.openModal('Share Card', `
      <div id="share-modal-content">
        <div class="text-center"><div class="spinner" style="margin: 1rem auto;"></div></div>
      </div>
    `);

    await this.refreshShareModal();
  },

  async refreshShareModal() {
    const content = document.getElementById('share-modal-content');
    if (!content) return;

    content.innerHTML = '<div class="text-center"><div class="spinner" style="margin: 1rem auto;"></div></div>';
    try {
      const response = await API.cards.shareStatus(this.currentCard.id);
      this.currentShareStatus = response || { enabled: false };
      content.innerHTML = this.renderShareModalContent(this.currentShareStatus);
      this.bindShareExpiryControls();

      const shareUrl = this.currentShareStatus.url
        ? `${window.location.origin}/${this.currentShareStatus.url}`
        : '';
      const input = document.getElementById('share-link-input');
      if (input) input.value = shareUrl;
    } catch (error) {
      content.innerHTML = `<p class="text-muted" id="share-modal-error"></p>`;
      const errorEl = document.getElementById('share-modal-error');
      if (errorEl) errorEl.textContent = error.message;
    }
  },

  renderShareModalContent(status) {
    const expiresAt = status?.expires_at ? new Date(status.expires_at) : null;
    const expired = !!status?.expired;
    const isEnabled = !!status?.enabled;
    const now = new Date();
    const msInDay = 24 * 60 * 60 * 1000;
    const hasActiveExpiry = !!expiresAt && !expired;
    let daysLeft = 0;
    let expiresLabel = 'Never expires';
    if (hasActiveExpiry) {
      const expiresAtLabel = this.escapeHtml(expiresAt.toLocaleDateString());
      const startNow = new Date(now.getFullYear(), now.getMonth(), now.getDate());
      const startExpiry = new Date(expiresAt.getFullYear(), expiresAt.getMonth(), expiresAt.getDate());
      daysLeft = Math.round((startExpiry - startNow) / msInDay);
      if (daysLeft < 0) daysLeft = 0;
      if (daysLeft === 0) {
        expiresLabel = `Expires today (${expiresAtLabel})`;
      } else {
        expiresLabel = `Expires in ${daysLeft} day${daysLeft === 1 ? '' : 's'} (${expiresAtLabel})`;
      }
    }
    const statusLine = isEnabled
      ? `<p class="${expired ? 'text-muted' : 'share-expiration'}">${expired ? 'This link has expired.' : expiresLabel}</p>`
      : '<p class="text-muted">Share a read-only link to this card.</p>';

    const linkSection = isEnabled ? `
      <div class="form-group">
        <label class="form-label">Share Link</label>
        <div class="search-input-group">
          <input type="text" class="form-input" id="share-link-input" readonly>
          <button class="btn btn-secondary" data-action="copy-share-link" ${expired ? 'disabled' : ''}>Copy</button>
        </div>
      </div>
    ` : '';

    const primaryAction = isEnabled
      ? ''
      : `<button class="btn btn-primary" data-action="enable-share">Enable Sharing</button>`;

    const disableAction = isEnabled
      ? `<button class="btn btn-primary" data-action="disable-share">Disable Sharing</button>`
      : '';

    const expirationControls = isEnabled ? '' : `
      <div class="form-group">
        <label class="form-label" for="share-expiry-select">Link expiration</label>
        <select id="share-expiry-select" class="form-input">
          <option value="0">Never expires</option>
          <option value="7">7 days</option>
          <option value="30">30 days</option>
          <option value="90">90 days</option>
          <option value="custom">Custom…</option>
        </select>
        <div id="share-expiry-custom-group" style="margin-top: 0.75rem; display: none;">
          <input type="number" id="share-expiry-custom" class="form-input" min="1" max="3650" placeholder="Enter days">
        </div>
      </div>
    `;
    const expirationNote = isEnabled
      ? '<p class="text-muted" style="margin-top: 0.5rem;">Disable sharing to change the expiration.</p>'
      : '';

    return `
      ${statusLine}
      ${expirationNote}
      ${linkSection}
      ${expirationControls}
      <div style="display: flex; gap: 0.5rem; flex-wrap: wrap; justify-content: flex-end;">
        ${disableAction}
        ${primaryAction}
      </div>
    `;
  },

  bindShareExpiryControls() {
    const select = document.getElementById('share-expiry-select');
    const customGroup = document.getElementById('share-expiry-custom-group');
    if (!select || !customGroup) return;
    const toggle = () => {
      customGroup.style.display = select.value === 'custom' ? 'block' : 'none';
    };
    select.addEventListener('change', toggle);
    toggle();
  },

  getShareExpiryDays() {
    const select = document.getElementById('share-expiry-select');
    if (!select) return null;
    if (select.value === 'custom') {
      const input = document.getElementById('share-expiry-custom');
      const days = parseInt(input?.value || '', 10);
      if (!Number.isFinite(days) || days <= 0 || days > 3650) return null;
      return days;
    }
    const parsed = parseInt(select.value, 10);
    return Number.isFinite(parsed) ? parsed : null;
  },

  async enableShare() {
    const days = this.getShareExpiryDays();
    if (days === null) {
      this.toast('Enter a valid expiration in days', 'error');
      return;
    }
    try {
      await API.cards.shareEnable(this.currentCard.id, days);
      await this.refreshShareModal();
      this.toast('Share link created', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async regenerateShare() {
    if (!confirm('Regenerate this link? The old link will stop working.')) return;
    const days = this.getShareExpiryDays();
    if (days === null) {
      this.toast('Enter a valid expiration in days', 'error');
      return;
    }
    try {
      await API.cards.shareEnable(this.currentCard.id, days);
      await this.refreshShareModal();
      this.toast('Share link regenerated', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async disableShare() {
    if (!confirm('Disable sharing? The current link will stop working.')) return;
    try {
      await API.cards.shareDisable(this.currentCard.id);
      await this.refreshShareModal();
      this.toast('Sharing disabled', 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  copyShareLink() {
    const input = document.getElementById('share-link-input');
    if (!input?.value) return;
    this.copyToClipboard(input.value);
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
          <button class="btn btn-ghost" data-action="close-modal">Cancel</button>
          <button class="btn btn-primary" data-action="confirm-finalize">Finalize Card</button>
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
          <button class="btn btn-primary btn-lg" data-action="show-finalize-register-form">
            Create Account
          </button>
          <button class="btn btn-secondary btn-lg" data-action="show-finalize-login-form">
            I Already Have an Account
          </button>
          <button class="btn btn-ghost" data-action="close-modal">
            Cancel
          </button>
        </div>
      </div>
    `);
  },

  showAIAuthModal() {
    this.openModal('Use the AI Goal Wizard', `
      <div class="finalize-auth-modal">
        <p style="margin-bottom: 1.5rem;">
          AI-generated goals are available after you create an account.
          This helps prevent abuse and keeps AI costs under control.
        </p>
        <div style="display: flex; flex-direction: column; gap: 1rem;">
          <a class="btn btn-primary btn-lg" href="#register" data-action="close-modal">
            Create Account
          </a>
          <a class="btn btn-secondary btn-lg" href="#login" data-action="close-modal">
            I Already Have an Account
          </a>
          <button class="btn btn-ghost" data-action="close-modal">
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
      <form id="finalize-register-form" data-action="finalize-register">
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
          <button type="button" class="btn btn-ghost" style="flex: 1;" data-action="show-finalize-auth-modal">
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
      await this.refreshNotificationCount();
      this.startNotificationPolling();

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
      <form id="finalize-login-form" data-action="finalize-login">
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
          <button type="button" class="btn btn-ghost" style="flex: 1;" data-action="show-finalize-auth-modal">
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
      await this.refreshNotificationCount();
      this.startNotificationPolling();

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
          <button class="btn btn-secondary" data-action="conflict-keep-existing" data-card-id="${existingCard.id}">
            Keep Existing Card
          </button>
          <button class="btn btn-primary" data-action="conflict-save-as-new">
            Save as New Card (with different title)
          </button>
          <button class="btn btn-ghost" style="color: var(--color-danger);" data-action="conflict-replace" data-card-id="${existingCard.id}">
            Replace Existing Card
          </button>
          <button class="btn btn-ghost" data-action="close-modal">
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
    this.currentCard = null;
    this.currentView = null;
    this.closeModal();
    this._allowNextHashRoute = true;
    window.location.hash = `#card/${existingCardId}`;
    this.toast('Keeping your existing card. Anonymous card discarded.', 'success');
  },

  // Handle conflict: save with new title
  async handleConflictSaveAsNew() {
    const anonCard = AnonymousCard.get();
    const currentTitle = anonCard.title || `${anonCard.year} Bingo Card`;

    this.openModal('Save with New Title', `
      <form id="conflict-new-title-form" data-action="conflict-save-as-new-submit">
        <div class="form-group">
          <label class="form-label" for="conflict-new-title">New Title</label>
          <input type="text" id="conflict-new-title" class="form-input" required
                 maxlength="100">
          <small class="text-muted">Choose a different title for your new card</small>
        </div>
        <div id="conflict-new-title-error" class="form-error hidden"></div>
        <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-ghost" style="flex: 1;" data-action="import-anonymous-card">
            Back
          </button>
          <button type="submit" class="btn btn-primary" style="flex: 1;">
            Save Card
          </button>
        </div>
      </form>
    `);
    const titleInput = document.getElementById('conflict-new-title');
    if (titleInput) titleInput.value = `${currentTitle} (2)`;
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
      <button class="btn btn-secondary" data-action="create-conflict-go-to-existing" data-card-id="${existingCard.id}">
        Go to Existing Card
      </button>
      <button class="btn btn-primary" data-action="create-conflict-save-as-new">
        Create with Different Title
      </button>`;

    // Only offer replace for unfinalized cards
    if (!existingCard.is_finalized) {
      buttons += `
        <button class="btn btn-ghost" style="color: var(--color-danger);" data-action="create-conflict-replace" data-card-id="${existingCard.id}">
          Delete &amp; Create New
        </button>`;
    }

    buttons += `
      <button class="btn btn-ghost" data-action="close-modal">
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
      <form id="create-conflict-title-form" data-action="create-conflict-save-as-new-submit">
        <div class="form-group">
          <label class="form-label" for="create-conflict-title">Card Title</label>
          <input type="text" id="create-conflict-title" class="form-input" required
                 maxlength="100">
          <small class="text-muted">Choose a unique title for your new card</small>
        </div>
        <div id="create-conflict-error" class="form-error hidden"></div>
        <div style="display: flex; gap: 1rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-ghost" style="flex: 1;" data-action="close-modal">
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" style="flex: 1;">
            Create Card
          </button>
        </div>
      </form>
    `);
    const titleInput = document.getElementById('create-conflict-title');
    if (titleInput) titleInput.value = suggestedTitle;
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
    cells.forEach((cell) => {
      grid.push(cell.classList.contains('bingo-cell--completed') || cell.classList.contains('bingo-cell--free'));
    });

    const size = this.getGridSize(this.currentCard);

    // Check rows
    for (let row = 0; row < size; row++) {
      if (grid.slice(row * size, row * size + size).every(Boolean)) {
        this.toast('BINGO! Row complete! 🎉🎉🎉', 'success');
        this.confetti(100);
        return;
      }
    }

    // Check columns
    for (let col = 0; col < size; col++) {
      if (Array.from({ length: size }).map((_, row) => grid[row * size + col]).every(Boolean)) {
        this.toast('BINGO! Column complete! 🎉🎉🎉', 'success');
        this.confetti(100);
        return;
      }
    }

    // Check diagonals
    if (Array.from({ length: size }).map((_, i) => grid[i * size + i]).every(Boolean)) {
      this.toast('BINGO! Diagonal complete! 🎉🎉🎉', 'success');
      this.confetti(100);
      return;
    }
    if (Array.from({ length: size }).map((_, i) => grid[i * size + (size - 1 - i)]).every(Boolean)) {
      this.toast('BINGO! Diagonal complete! 🎉🎉🎉', 'success');
      this.confetti(100);
      return;
    }
  },

  // Friends page
  renderInviteGate(container, token) {
    if (!token) {
      container.innerHTML = `
        <div class="card text-center" style="padding: 3rem;">
          <h3>Invite link not found</h3>
          <p class="text-muted mb-lg">This invite link is missing or invalid.</p>
          <a href="#home" class="btn btn-primary">Go Home</a>
        </div>
      `;
      return;
    }

    this.storePendingInviteToken(token);
    container.innerHTML = `
      <div class="card text-center" style="padding: 3rem;">
        <h3>Accept Friend Invite</h3>
        <p class="text-muted mb-lg">Sign in or create an account to accept this invite.</p>
        <div style="display: flex; gap: 1rem; justify-content: center; flex-wrap: wrap;">
          <a href="#login" class="btn btn-primary">Sign In</a>
          <a href="#register" class="btn btn-secondary">Create Account</a>
        </div>
      </div>
    `;
  },

  async renderInviteAccept(container, token) {
    if (!token) {
      container.innerHTML = `
        <div class="card text-center" style="padding: 3rem;">
          <h3>Invite link not found</h3>
          <p class="text-muted mb-lg">This invite link is missing or invalid.</p>
          <a href="#friends" class="btn btn-primary">Back to Friends</a>
        </div>
      `;
      return;
    }

    container.innerHTML = `
      <div class="card text-center" style="padding: 3rem;">
        <div class="spinner" style="margin: 2rem auto;"></div>
        <p>Accepting invite...</p>
      </div>
    `;

    try {
      const response = await API.friends.acceptInvite(token);
      this.toast('Invite accepted!', 'success');
      container.innerHTML = `
        <div class="card text-center" style="padding: 3rem;">
          <h3>You're friends now!</h3>
          <p class="text-muted mb-lg">You are now connected with ${this.escapeHtml(response.inviter.username)}.</p>
          <a href="#friends" class="btn btn-primary">Go to Friends</a>
        </div>
      `;
    } catch (error) {
      container.innerHTML = `
        <div class="card text-center" style="padding: 3rem;">
          <h3>Invite Error</h3>
          <p class="text-muted mb-lg" id="invite-accept-error"></p>
          <a href="#friends" class="btn btn-primary">Back to Friends</a>
        </div>
      `;
      const errorEl = document.getElementById('invite-accept-error');
      if (errorEl) errorEl.textContent = error.message;
    }
  },

  async renderFriends(container) {
    container.innerHTML = `
      <div class="friends-page">
        <div class="friends-header">
          <h2>Friends</h2>
        </div>

        <div class="card">
          <h3>Invite Friends</h3>
          <p class="text-muted" style="margin-bottom: 1rem;">
            Share a private invite link. Anyone with the link can accept it.
            You can revoke invites at any time.
          </p>
          <div class="search-input-group" style="align-items: center;">
            <button class="btn btn-primary" id="create-invite-btn">Create Invite Link</button>
          </div>
          <div id="invite-result" class="mt-md"></div>
          <div id="invite-list" class="mt-md"></div>
        </div>

        <div class="friends-search card">
          <h3>Find Friends</h3>
          <p class="text-muted" style="margin-bottom: 1rem;">
            Search for friends by their username. Users must enable "Make my profile searchable"
            in their <a href="#profile">Profile settings</a> to appear in search results.
          </p>
          <div class="search-input-group">
            <input type="text" id="friend-search" class="form-input" placeholder="Search by username...">
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

        <div id="blocked-users" class="card" style="display: none;">
          <h3>Blocked Users</h3>
          <div id="blocked-list"></div>
        </div>
      </div>
    `;

    this.setupFriendsEvents();
    await this.loadFriends();
    await this.loadInvites();
    await this.loadBlockedUsers();
  },

  setupFriendsEvents() {
    const searchInput = document.getElementById('friend-search');
    const searchBtn = document.getElementById('search-btn');
    const createInviteBtn = document.getElementById('create-invite-btn');

    searchBtn.addEventListener('click', () => this.searchFriends());
    searchInput.addEventListener('keypress', (e) => {
      if (e.key === 'Enter') this.searchFriends();
    });

    let debounceTimer;
    searchInput.addEventListener('input', () => {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => this.searchFriends(), 300);
    });

    createInviteBtn.addEventListener('click', () => this.createFriendInvite());
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
            <button class="btn btn-primary btn-sm" data-action="send-friend-request" data-user-id="${user.id}">
              Add Friend
            </button>
          </div>
        `).join('');
      }
    } catch (error) {
      resultsEl.innerHTML = '<p class="text-muted" id="friend-search-error"></p>';
      const errorEl = document.getElementById('friend-search-error');
      if (errorEl) errorEl.textContent = error.message;
    }
  },

  async createFriendInvite() {
    const resultEl = document.getElementById('invite-result');
    resultEl.innerHTML = '<div class="spinner" style="margin: 1rem auto;"></div>';

    try {
      const response = await API.friends.createInvite(14);
      const inviteURL = `${window.location.origin}/${response.url}`;
      resultEl.innerHTML = `
        <div class="card" style="padding: 1rem;">
          <div class="form-group" style="margin-bottom: 0;">
            <label class="form-label">Invite Link</label>
            <div class="search-input-group">
              <input type="text" class="form-input" id="invite-link-input" readonly>
              <button class="btn btn-secondary" data-action="copy-invite-link">Copy</button>
            </div>
          </div>
        </div>
      `;
      const inviteInput = document.getElementById('invite-link-input');
      if (inviteInput) inviteInput.value = inviteURL;
      await this.loadInvites();
    } catch (error) {
      resultEl.innerHTML = '<p class="text-muted" id="invite-error"></p>';
      const errorEl = document.getElementById('invite-error');
      if (errorEl) errorEl.textContent = error.message;
    }
  },

  copyInviteLink(url) {
    navigator.clipboard.writeText(url).then(() => {
      this.toast('Invite link copied!', 'success');
    }).catch(() => {
      this.toast('Could not copy link', 'error');
    });
  },

  async loadInvites() {
    const listEl = document.getElementById('invite-list');
    if (!listEl) return;
    try {
      const response = await API.friends.listInvites();
      const invites = response.invites || [];
      if (invites.length === 0) {
        listEl.innerHTML = '<p class="text-muted">No active invites.</p>';
        return;
      }

      listEl.innerHTML = invites.map(invite => `
        <div class="friend-item">
          <div>
            <strong>Invite created</strong>
            <div class="text-muted">
              ${invite.expires_at ? `Expires ${new Date(invite.expires_at).toLocaleDateString()}` : 'No expiration'}
            </div>
          </div>
          <button class="btn btn-ghost btn-sm" data-action="revoke-invite" data-invite-id="${invite.id}">Revoke</button>
        </div>
      `).join('');
    } catch (error) {
      listEl.innerHTML = '<p class="text-muted" id="invite-list-error"></p>';
      const errorEl = document.getElementById('invite-list-error');
      if (errorEl) errorEl.textContent = error.message;
    }
  },

  async revokeInvite(inviteId) {
    try {
      await API.friends.revokeInvite(inviteId);
      this.toast('Invite revoked', 'success');
      await this.loadInvites();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async loadBlockedUsers() {
    const blockedEl = document.getElementById('blocked-users');
    const blockedListEl = document.getElementById('blocked-list');
    if (!blockedEl || !blockedListEl) return;
    try {
      const response = await API.friends.listBlocked();
      const blocked = response.blocked || [];
      if (blocked.length === 0) {
        blockedEl.style.display = 'none';
        return;
      }
      blockedEl.style.display = 'block';
      blockedListEl.innerHTML = blocked.map(user => `
        <div class="friend-item">
          <div>
            <strong>${this.escapeHtml(user.username)}</strong>
          </div>
          <button class="btn btn-ghost btn-sm" data-action="unblock-user" data-user-id="${user.id}">Unblock</button>
        </div>
      `).join('');
    } catch (error) {
      blockedEl.style.display = 'block';
      blockedListEl.innerHTML = '<p class="text-muted" id="blocked-error"></p>';
      const errorEl = document.getElementById('blocked-error');
      if (errorEl) errorEl.textContent = error.message;
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
            </div>
            <div class="friend-actions">
              <button class="btn btn-primary btn-sm" data-action="accept-request" data-request-id="${req.id}">Accept</button>
              <button class="btn btn-ghost btn-sm" data-action="reject-request" data-request-id="${req.id}">Reject</button>
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
            </div>
            <button class="btn btn-ghost btn-sm" data-action="cancel-request" data-request-id="${req.id}">Cancel</button>
          </div>
        `).join('');
      } else {
        sentEl.style.display = 'none';
      }

      // Friends list
      const friendsListEl = document.getElementById('friends-list');
      if (friends && friends.length > 0) {
        friendsListEl.innerHTML = friends.map(friend => {
          const otherUserId = friend.user_id === this.user.id ? friend.friend_id : friend.user_id;
          const friendName = this.escapeHtml(friend.friend_username);
          return `
            <div class="friend-item">
              <div>
                <strong>${friendName}</strong>
              </div>
              <div class="friend-actions">
                <a href="#friend-card/${friend.id}" class="btn btn-secondary btn-sm">View Card</a>
                <button class="btn btn-ghost btn-sm" data-action="remove-friend" data-friendship-id="${friend.id}">Remove</button>
                <button class="btn btn-ghost btn-sm" data-action="block-user" data-other-user-id="${otherUserId}">Block</button>
              </div>
            </div>
          `;
        }).join('');
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

  async blockUser(userId, friendName) {
    if (!confirm(`Block ${friendName}? This will remove the friendship and stop future requests.`)) {
      return;
    }
    try {
      await API.friends.block(userId);
      this.toast('User blocked', 'success');
      await this.loadFriends();
      await this.loadBlockedUsers();
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async unblockUser(userId, friendName) {
    if (!confirm(`Unblock ${friendName}? They will be able to send requests again.`)) {
      return;
    }
    try {
      await API.friends.unblock(userId);
      this.toast('User unblocked', 'success');
      await this.loadBlockedUsers();
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
          <p class="text-muted mb-lg" id="friend-card-error"></p>
          <a href="#friends" class="btn btn-primary">Back to Friends</a>
        </div>
      `;
      const errorEl = document.getElementById('friend-card-error');
      if (errorEl) errorEl.textContent = error.message;
    }
  },

  renderFriendCardView(container) {
    const completedCount = this.currentCard.items.filter(i => i.is_completed).length;
    const gridSize = this.getGridSize(this.currentCard);
    const capacity = this.getCardCapacity(this.currentCard);
    const progress = capacity ? Math.round((completedCount / capacity) * 100) : 0;
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
        <select id="friend-card-select" class="year-selector" data-change-action="friend-card-select">
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
          <div class="bingo-grid bingo-grid--finalized ${isArchived ? 'bingo-grid--archive' : ''}" id="bingo-grid" style="--grid-size: ${gridSize};">
            ${this.renderGrid(true)}
          </div>
        </div>

        <div class="finalized-card-progress">
          <div class="progress-bar">
            <div class="progress-fill" style="width: ${progress}%"></div>
          </div>
          <p class="progress-text">${completedCount}/${capacity} completed</p>
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
    return this.renderGrid(true);
  },

  setupFriendCardEvents() {
    document.getElementById('bingo-grid').addEventListener('click', async (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (!cell || cell.classList.contains('bingo-cell--free') || cell.classList.contains('bingo-cell--empty')) return;

      const itemId = cell.dataset.itemId;
      const item = this.currentCard.items?.find(i => i.id === itemId);
      const content = item?.content || cell.querySelector('.bingo-cell-content')?.textContent || '';
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
                    data-action="react-item" data-item-id="${itemId}" data-emoji="${emoji}">${emoji}</button>
          `).join('')}
          ${userReaction ? `<button class="emoji-btn emoji-btn--remove" data-action="remove-reaction" data-item-id="${itemId}">✕</button>` : ''}
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
        <button type="button" class="btn btn-secondary" style="width: 100%;" data-action="close-modal">
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
          <button class="btn btn-secondary btn-sm" data-action="resend-verification">Resend verification email</button>
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
              <p class="text-muted" style="margin-bottom: 1rem;">
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
        </div>
      </div>
    `;

    this.setupProfileEvents();
    this.loadNotificationSettings();
    this.loadReminderSettings();
    this.loadApiTokens();
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
          <p class="text-muted mb-lg" id="archive-card-error"></p>
          <a href="#dashboard" class="btn btn-primary">Back to Dashboard</a>
        </div>
      `;
      const errorEl = document.getElementById('archive-card-error');
      if (errorEl) errorEl.textContent = error.message;
    }
  },

  renderArchiveCardView(container) {
    const completedCount = this.currentCard.items.filter(i => i.is_completed).length;
    const gridSize = this.getGridSize(this.currentCard);
    const capacity = this.getCardCapacity(this.currentCard);
    const progress = capacity ? Math.round((completedCount / capacity) * 100) : 0;
    const stats = this.currentStats;
    const displayName = this.getCardDisplayName(this.currentCard);
    const categoryBadge = this.getCategoryBadge(this.currentCard);
    const visibilityIcon = this.currentCard.visible_to_friends ? 'eye' : 'eye-slash';
    const visibilityLabel = this.currentCard.visible_to_friends ? 'Visible' : 'Private';
    const showShare = this.user && !this.isAnonymousMode && this.currentCard.is_finalized;

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
            <button class="btn btn-ghost btn-sm" data-action="show-clone-card-modal" title="Clone card">📄</button>
            ${showShare ? '<button class="btn btn-ghost btn-sm" data-action="open-share-modal" title="Share card">🔗</button>' : ''}
            <button class="visibility-toggle-btn ${this.currentCard.visible_to_friends ? 'visibility-toggle-btn--visible' : 'visibility-toggle-btn--private'}" data-action="toggle-card-visibility" data-card-id="${this.currentCard.id}" data-visible="${!this.currentCard.visible_to_friends}" title="${visibilityLabel}">
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
          <div class="bingo-grid bingo-grid--finalized bingo-grid--archive" id="bingo-grid" style="--grid-size: ${gridSize};">
            ${this.renderArchiveGrid()}
          </div>
        </div>

        <div class="finalized-card-progress">
          <div class="progress-bar">
            <div class="progress-fill" style="width: ${progress}%"></div>
          </div>
          <p class="progress-text">${completedCount}/${capacity} completed</p>
        </div>
      </div>
    `;

    this.setupArchiveCardEvents();
  },

  renderArchiveGrid() {
    return this.renderGrid(true);
  },

  setupArchiveCardEvents() {
    document.getElementById('bingo-grid').addEventListener('click', (e) => {
      const cell = e.target.closest('.bingo-cell');
      if (!cell || cell.classList.contains('bingo-cell--free') || cell.classList.contains('bingo-cell--empty')) return;

      const position = parseInt(cell.dataset.position);
      const item = this.currentCard.items?.find(i => i.position === position);
      const content = item?.content || cell.querySelector('.bingo-cell-content')?.textContent || '';
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
        <button type="button" class="btn btn-secondary" style="width: 100%;" data-action="close-modal">
          Close
        </button>
      </div>
    `);
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
        <p style="margin-bottom: 1.5rem;">
          Your card is saved as a draft. If you log out now, you can pick up where you left off when you sign back in. Finalizing locks the layout so you can start tracking completion.
        </p>
        <div style="display: flex; gap: 1rem; justify-content: flex-end; flex-wrap: wrap;">
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
      this.notificationSettings = null;
      this.notificationUnreadCount = 0;
      this.stopNotificationPolling();
      this.setupNavigation();
      sessionStorage.removeItem('pendingInviteToken');
      this._allowNextHashRoute = true;
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

  async loadApiTokens() {
    const listEl = document.getElementById('api-tokens-list');
    if (!listEl) return;

    try {
      const response = await API.tokens.list();
      const tokens = response.tokens || [];

      if (tokens.length === 0) {
        listEl.innerHTML = '<p class="text-muted" style="margin-top: 1rem;">No active tokens.</p>';
        return;
      }

      listEl.innerHTML = tokens.map(token => `
        <div class="token-item" style="padding: 0.75rem; border: 1px solid var(--border-color); border-radius: 0.5rem; margin-top: 0.5rem; display: flex; justify-content: space-between; align-items: center;">
          <div class="token-info">
            <div style="font-weight: 500;">${this.escapeHtml(token.name)}</div>
            <div class="token-meta text-muted" style="font-size: 0.85rem;">
              <code>${this.escapeHtml(token.token_prefix)}...</code>
              <span>•</span>
              <span class="token-scope scope-${token.scope}">${token.scope.replace('_', ' & ')}</span>
              <span>•</span>
              <span>${token.expires_at ? 'Expires ' + new Date(token.expires_at).toLocaleDateString() : 'Never expires'}</span>
            </div>
            <div class="token-meta text-muted" style="font-size: 0.85rem;">
              Last used: ${token.last_used_at ? new Date(token.last_used_at).toLocaleString() : 'Never'}
            </div>
          </div>
          <button class="btn btn-ghost btn-sm" style="color: var(--color-danger);" data-action="delete-token" data-token-id="${token.id}" title="Revoke Token">
            <i class="fas fa-trash"></i>
          </button>
        </div>
      `).join('');

      // Add Revoke All button if tokens exist
      if (tokens.length > 1) {
          listEl.innerHTML += `
            <div style="margin-top: 1rem; text-align: right;">
                <button class="btn btn-ghost btn-sm" style="color: var(--color-danger);" data-action="revoke-all-tokens">Revoke All Tokens</button>
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
        <div style="display: flex; gap: 1rem; justify-content: flex-end;">
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
        <div class="token-display" style="background: var(--surface-2); padding: 1rem; border-radius: 0.5rem; margin: 1rem 0; display: flex; align-items: center; justify-content: space-between; gap: 1rem;">
          <code id="new-token" style="word-break: break-all;">${this.escapeHtml(token)}</code>
          <button class="btn btn-secondary btn-sm" data-action="copy-new-token">Copy</button>
        </div>
        <p class="text-muted" style="margin-top: 1rem; font-size: 0.9rem;">
          Use this token in the <code>Authorization</code> header:
          <br>
          <code style="display: block; background: var(--surface-2); padding: 0.5rem; margin-top: 0.5rem; border-radius: 0.25rem;">Authorization: Bearer ${this.escapeHtml(token.substring(0, 10))}...</code>
        </p>
        <div style="margin-top: 1.5rem; text-align: right;">
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
    if (text === null || text === undefined) return '';
    const map = {
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      '"': '&quot;',
      "'": '&#039;'
    };
    return String(text).replace(/[&<>"']/g, function(c) {
      return map[c];
    });
  },

  // About Page
  renderAbout(container) {
    container.innerHTML = `
      <div class="legal-page">
        <h1>About Year of Bingo</h1>

        <div class="legal-content about-content">
          <h2>The Origin Story</h2>
          <p>
            On New Year's Eve 2024, my wife and I were celebrating and the topic of New Year's Resolutions came up. I pitched an idea for something different: What if we created Bingo cards and tracked 24 goals throughout the year instead of just a single resolution destined to be unachieved?
          </p>
          <p>
            She loved the idea and quickly sketched out a Bingo card in a notebook and filled in the squares. I popped open Excel and filled out my card digitally.
          </p>
          <p>
            Our goals ranged from <em>"Read 52 Books"</em> to <em>"Drive a Tank"</em> and we've been having a blast tracking and completing the goals in 2025.
          </p>

          <h2>Why This Site Exists</h2>
          <p>
            We've told the story to a bunch of people and everyone loves the idea, so I decided to make a simple webapp for everyone to create and share cards.
          </p>
          <p>
            With the help of Claude Opus 4.5, <strong>yearofbingo.com</strong> was born.
          </p>

          <h2>Open Source</h2>
          <p>
            This project is open source and licensed under Apache 2. Check out the code on <a href="https://github.com/HammerMeetNail/yearofbingo" target="_blank" rel="noopener noreferrer">GitHub</a>.
          </p>

          <h2>Go Have Fun!</h2>
          <p>
            The site is free and easy to use. Create goals, share with your friends, do something you've always wanted to!
          </p>

          <div class="about-cta">
            <a href="#register" class="btn btn-primary">Create Your Card</a>
          </div>
        </div>
      </div>
    `;
  },

  renderFAQ(container) {
    container.innerHTML = `
      <div class="legal-page">
        <h1>Frequently Asked Questions</h1>

        <div class="legal-content faq-content">
          <div class="faq-section">
            <h2>Getting Started</h2>

            <div class="faq-item">
              <h3>What is Year of Bingo?</h3>
              <p>
                Year of Bingo is a fun way to track your annual goals! Instead of making a single New Year's resolution,
                you create a Bingo card (2x2–5x5) with personal goals (optionally including a FREE space). Throughout the year,
                you mark items complete and try to get Bingos!
              </p>
            </div>

            <div class="faq-item">
              <h3>How do I create a Bingo card?</h3>
              <p>
                Click "Get Started" or "Create New Card" to begin. You can type in your own goals, use our curated
                suggestions by category, or use the "Fill Empty Spaces" button to randomly fill remaining slots.
                Once you have filled all items, click "Finalize Card" to lock it in and start tracking!
              </p>
            </div>

            <div class="faq-item">
              <h3>Can I edit my card after finalizing it?</h3>
              <p>
                No, once a card is finalized, the layout is locked. This is intentional&mdash;it prevents moving items
                around to get easier Bingos! You can still add notes and mark items as complete.
              </p>
            </div>

            <div class="faq-item">
              <h3>What counts as a Bingo?</h3>
              <p>
                A Bingo is 5 completed items in a row&mdash;horizontally, vertically, or diagonally. The center FREE space
                counts as completed. With a full card, you can get up to 12 Bingos (5 rows + 5 columns + 2 diagonals).
              </p>
            </div>
          </div>

          <div class="faq-section">
            <h2>Friends & Sharing</h2>

            <div class="faq-item">
              <h3>How do I find friends on the site?</h3>
              <p>
                Go to the <a href="#friends">Friends page</a> and search for friends by their <strong>username</strong>.
                Note: Users must opt in to be searchable. If you can't find someone, ask them to enable
                "Make my profile searchable" in their <a href="#profile">Profile settings</a>.
              </p>
            </div>

            <div class="faq-item">
              <h3>How do I let friends find me?</h3>
              <p>
                By default, your profile is private. To let friends find you, go to your <a href="#profile">Profile</a>
                and enable "Make my profile searchable". Your username will then appear in search results.
              </p>
            </div>

            <div class="faq-item">
              <h3>Can I hide my card from friends?</h3>
              <p>
                Yes! Each card has a visibility setting. On the Dashboard, select cards and use Actions &rarr; "Make Private"
                to hide them from friends. Private cards are completely hidden&mdash;friends won't even know they exist.
              </p>
            </div>

            <div class="faq-item">
              <h3>What are reactions?</h3>
              <p>
                When viewing a friend's card, you can react to their completed items with emojis to cheer them on!
                It's a fun way to celebrate each other's accomplishments.
              </p>
            </div>
          </div>

          <div class="faq-section">
            <h2>Managing Cards</h2>

            <div class="faq-item">
              <h3>Can I have multiple cards?</h3>
              <p>
                Yes! You can create multiple cards for different years or different themes. All your cards appear
                on your Dashboard where you can sort, filter, and manage them.
              </p>
            </div>

            <div class="faq-item">
              <h3>What does archiving a card do?</h3>
              <p>
                Archiving is a way to organize your cards. Archived cards still appear on your Dashboard with an
                "Archived" badge. You can archive/unarchive cards anytime using the Actions menu.
              </p>
            </div>

            <div class="faq-item">
              <h3>How do I export my cards?</h3>
              <p>
                On the Dashboard, select the cards you want to export using the checkboxes, then click
                Actions &rarr; "Export Cards". You'll download a ZIP file containing CSV files for each selected card.
              </p>
            </div>

            <div class="faq-item">
              <h3>Can I delete a card?</h3>
              <p>
                Yes. On the Dashboard, select the card(s) you want to delete and use Actions &rarr; "Delete Cards".
                This action is permanent and cannot be undone, so be careful!
              </p>
            </div>
          </div>

          <div class="faq-section">
            <h2>Account & Privacy</h2>

            <div class="faq-item">
              <h3>Do I need to verify my email?</h3>
              <p>
                Email verification is optional but recommended. It allows you to use password reset and magic link login
                if you forget your password. You can verify your email anytime from your <a href="#profile">Profile</a>.
              </p>
            </div>

            <div class="faq-item">
              <h3>What data do you collect?</h3>
              <p>
                We only collect what's necessary to run the service: your email, username, and the content of your
                Bingo cards. We don't use tracking cookies or sell your data. See our <a href="#privacy">Privacy Policy</a>
                for full details.
              </p>
            </div>

            <div class="faq-item">
              <h3>Can I delete my account?</h3>
              <p>
                If you need to delete your account, please <a href="#support">contact support</a> and we'll help you out.
              </p>
            </div>
          </div>

          <div class="faq-section">
            <h2>Tips for Success</h2>

            <div class="faq-item">
              <h3>What makes a good Bingo card?</h3>
              <p>
                Mix it up! Include some easy wins (like "Try a new restaurant"), medium challenges
                (like "Read 12 books"), and stretch goals (like "Run a marathon"). The variety keeps things
                interesting all year long.
              </p>
            </div>

            <div class="faq-item">
              <h3>Any other tips?</h3>
              <ul>
                <li>Add notes to items to track your progress or memories</li>
                <li>Share your card with friends for accountability</li>
                <li>Check in monthly to review what you've accomplished</li>
                <li>Don't stress about getting every Bingo&mdash;have fun with it!</li>
              </ul>
            </div>
          </div>

          <div class="faq-cta">
            <p>Still have questions?</p>
            <a href="#support" class="btn btn-primary">Contact Support</a>
          </div>
        </div>
      </div>
    `;
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

    const emailInput = document.getElementById('support-email');
    if (emailInput) emailInput.value = userEmail;

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
  App.handleHashChange();
});
