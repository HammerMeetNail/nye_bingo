// Year of Bingo - Notifications Module (scaffold)

window.App = window.App || {};
const App = window.App;

Object.assign(App, {
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
          <a href="/dashboard" class="btn btn-ghost">&larr; Back</a>
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
          <div class="text-center"><div class="spinner spinner--spaced"></div></div>
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
          <div class="card text-center p-xl">
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
        return `/friend-card/${notification.friendship_id}`;
      }
    }
    return '/friends';
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
});
