// Year of Bingo - API Client

const API = {
  csrfToken: null,
  retryCount: 0,
  maxRetries: 2,

  async init() {
    await this.fetchCSRFToken();
  },

  async fetchCSRFToken() {
    try {
      const response = await fetch('/api/csrf');
      if (!response.ok) {
        throw new Error('Failed to fetch CSRF token');
      }
      const data = await response.json();
      this.csrfToken = data.token;
    } catch (error) {
      console.error('Failed to fetch CSRF token:', error);
      // Retry once after a short delay
      if (this.retryCount < 1) {
        this.retryCount++;
        await new Promise(resolve => setTimeout(resolve, 1000));
        return this.fetchCSRFToken();
      }
    }
  },

  async request(method, path, body = null, options = {}) {
    const headers = {
      'Content-Type': 'application/json',
    };

    if (this.csrfToken && ['POST', 'PUT', 'DELETE', 'PATCH'].includes(method)) {
      headers['X-CSRF-Token'] = this.csrfToken;
    }

    const fetchOptions = {
      method,
      headers,
      credentials: 'same-origin',
    };

    if (body && method !== 'GET') {
      fetchOptions.body = JSON.stringify(body);
    }

    // Add timeout support
    const timeout = options.timeout || 30000;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);
    fetchOptions.signal = controller.signal;

    try {
      const response = await fetch(path, fetchOptions);
      clearTimeout(timeoutId);

      // Handle empty responses
      const contentType = response.headers.get('content-type');
      let data = null;
      if (contentType && contentType.includes('application/json')) {
        const text = await response.text();
        data = text ? JSON.parse(text) : {};
      }

      if (!response.ok) {
        // Handle specific status codes
        if (response.status === 401) {
          const message = data?.error || 'Session expired. Please log in again.';
          throw new APIError(message, response.status, data);
        }
        if (response.status === 403) {
          const isCSRFError = typeof data?.error === 'string' && data.error.toLowerCase().includes('csrf token');
          // CSRF token might be invalid - refresh and retry once
          if (isCSRFError && !options.retried) {
            await this.fetchCSRFToken();
            return this.request(method, path, body, { ...options, retried: true });
          }
          throw new APIError(data?.error || 'Access denied.', response.status, data);
        }
        if (response.status === 409) {
          // Conflict: by default treat as an error, but allow specific callers to handle it.
          if (options.allowConflictResponse) {
            return data;
          }
          throw new APIError(data?.error || 'Conflict', response.status, data);
        }
        if (response.status >= 500) {
          throw new APIError(data?.error || 'Server error. Please try again later.', response.status, data);
        }
        throw new APIError(data?.error || 'Request failed', response.status, data);
      }

      return data;
    } catch (error) {
      clearTimeout(timeoutId);

      if (error.name === 'AbortError') {
        throw new APIError('Request timed out. Please check your connection.', 0);
      }
      if (error instanceof APIError) {
        throw error;
      }
      // Network error
      if (!navigator.onLine) {
        throw new APIError('No internet connection. Please check your network.', 0);
      }
      throw new APIError('Connection error. Please try again.', 0);
    }
  },

  async requestBlob(method, path, body = null, options = {}) {
    const headers = {};

    if (this.csrfToken && ['POST', 'PUT', 'DELETE', 'PATCH'].includes(method)) {
      headers['X-CSRF-Token'] = this.csrfToken;
    }

    if (body && method !== 'GET') {
      headers['Content-Type'] = 'application/json';
    }

    const fetchOptions = {
      method,
      headers,
      credentials: 'same-origin',
    };

    if (body && method !== 'GET') {
      fetchOptions.body = JSON.stringify(body);
    }

    const timeout = options.timeout || 30000;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);
    fetchOptions.signal = controller.signal;

    try {
      const response = await fetch(path, fetchOptions);
      clearTimeout(timeoutId);

      if (!response.ok) {
        const contentType = response.headers.get('content-type') || '';
        let data = null;
        if (contentType.includes('application/json')) {
          const text = await response.text();
          data = text ? JSON.parse(text) : {};
        }

        if (response.status === 401) {
          const message = data?.error || 'Session expired. Please log in again.';
          throw new APIError(message, response.status, data);
        }
        if (response.status === 403) {
          const isCSRFError = typeof data?.error === 'string' && data.error.toLowerCase().includes('csrf token');
          if (isCSRFError && !options.retried) {
            await this.fetchCSRFToken();
            return this.requestBlob(method, path, body, { ...options, retried: true });
          }
          throw new APIError(data?.error || 'Access denied.', response.status, data);
        }
        if (response.status >= 500) {
          throw new APIError(data?.error || 'Server error. Please try again later.', response.status, data);
        }
        throw new APIError(data?.error || 'Request failed', response.status, data);
      }

      return response.blob();
    } catch (error) {
      clearTimeout(timeoutId);

      if (error.name === 'AbortError') {
        throw new APIError('Request timed out. Please check your connection.', 0);
      }
      if (error instanceof APIError) {
        throw error;
      }
      if (!navigator.onLine) {
        throw new APIError('No internet connection. Please check your network.', 0);
      }
      throw new APIError('Connection error. Please try again.', 0);
    }
  },

  // Auth endpoints
  auth: {
    async register(email, password, username, searchable = false) {
      return API.request('POST', '/api/auth/register', {
        email,
        password,
        username,
        searchable,
      });
    },

    async login(email, password) {
      return API.request('POST', '/api/auth/login', { email, password });
    },

    async logout() {
      return API.request('POST', '/api/auth/logout');
    },

    async me() {
      return API.request('GET', '/api/auth/me');
    },

    async changePassword(currentPassword, newPassword) {
      return API.request('POST', '/api/auth/password', {
        current_password: currentPassword,
        new_password: newPassword,
      });
    },

    async verifyEmail(token) {
      return API.request('POST', '/api/auth/verify-email', { token });
    },

    async resendVerification() {
      return API.request('POST', '/api/auth/resend-verification');
    },

    async requestMagicLink(email) {
      return API.request('POST', '/api/auth/magic-link', { email });
    },

    async verifyMagicLink(token) {
      return API.request('GET', `/api/auth/magic-link/verify?token=${encodeURIComponent(token)}`);
    },

    async forgotPassword(email) {
      return API.request('POST', '/api/auth/forgot-password', { email });
    },

    async resetPassword(token, password) {
      return API.request('POST', '/api/auth/reset-password', { token, password });
    },

    async updateSearchable(searchable) {
      return API.request('PUT', '/api/auth/searchable', { searchable });
    },
  },

  account: {
    async export() {
      return API.requestBlob('GET', '/api/account/export');
    },

    async delete(confirmUsername, password) {
      return API.request('DELETE', '/api/account', {
        confirm_username: confirmUsername,
        password,
        confirm: true,
      });
    },
  },

  // Card endpoints
  cards: {
    async create(year, title = null, category = null, options = {}) {
      const body = { year };
      if (title) body.title = title;
      if (category) body.category = category;
      if (options && typeof options.gridSize === 'number') body.grid_size = options.gridSize;
      if (options && typeof options.headerText === 'string') body.header_text = options.headerText;
      if (options && typeof options.hasFreeSpace === 'boolean') body.has_free_space = options.hasFreeSpace;
      return API.request('POST', '/api/cards', body, { allowConflictResponse: true });
    },

    async list() {
      return API.request('GET', '/api/cards');
    },

    async get(id) {
      return API.request('GET', `/api/cards/${id}`);
    },

    async deleteCard(id) {
      return API.request('DELETE', `/api/cards/${id}`);
    },

    async updateMeta(cardId, title = null, category = null) {
      const body = {};
      if (title !== null) body.title = title;
      if (category !== null) body.category = category;
      return API.request('PUT', `/api/cards/${cardId}/meta`, body);
    },

    async getCategories() {
      return API.request('GET', '/api/cards/categories');
    },

    async addItem(cardId, content, position = null) {
      const body = { content };
      if (position !== null) {
        body.position = position;
      }
      return API.request('POST', `/api/cards/${cardId}/items`, body);
    },

    async updateItem(cardId, position, updates) {
      return API.request('PUT', `/api/cards/${cardId}/items/${position}`, updates);
    },

    async removeItem(cardId, position) {
      return API.request('DELETE', `/api/cards/${cardId}/items/${position}`);
    },

    async shuffle(cardId) {
      return API.request('POST', `/api/cards/${cardId}/shuffle`);
    },

    async swap(cardId, position1, position2) {
      return API.request('POST', `/api/cards/${cardId}/swap`, { position1, position2 });
    },

    async finalize(cardId, visibleToFriends = null) {
      const body = visibleToFriends !== null ? { visible_to_friends: visibleToFriends } : null;
      return API.request('POST', `/api/cards/${cardId}/finalize`, body);
    },

    async shareStatus(cardId) {
      return API.request('GET', `/api/cards/${cardId}/share`);
    },

    async shareEnable(cardId, expiresInDays = null) {
      const body = typeof expiresInDays === 'number' ? { expires_in_days: expiresInDays } : null;
      return API.request('POST', `/api/cards/${cardId}/share`, body);
    },

    async shareDisable(cardId) {
      return API.request('DELETE', `/api/cards/${cardId}/share`);
    },

    async updateConfig(cardId, headerText = null, hasFreeSpace = null) {
      const body = {};
      if (headerText !== null) body.header_text = headerText;
      if (hasFreeSpace !== null) body.has_free_space = hasFreeSpace;
      return API.request('PUT', `/api/cards/${cardId}/config`, body);
    },

    async clone(cardId, params = {}) {
      return API.request('POST', `/api/cards/${cardId}/clone`, params);
    },

    async updateVisibility(cardId, visibleToFriends) {
      return API.request('PUT', `/api/cards/${cardId}/visibility`, {
        visible_to_friends: visibleToFriends,
      });
    },

    async bulkUpdateVisibility(cardIds, visibleToFriends) {
      return API.request('PUT', '/api/cards/visibility/bulk', {
        card_ids: cardIds,
        visible_to_friends: visibleToFriends,
      });
    },

    async bulkDelete(cardIds) {
      return API.request('DELETE', '/api/cards/bulk', {
        card_ids: cardIds,
      });
    },

    async bulkUpdateArchive(cardIds, isArchived) {
      return API.request('PUT', '/api/cards/archive/bulk', {
        card_ids: cardIds,
        is_archived: isArchived,
      });
    },

    async completeItem(cardId, position, notes = null, proofUrl = null) {
      const body = {};
      if (notes) body.notes = notes;
      if (proofUrl) body.proof_url = proofUrl;
      return API.request('PUT', `/api/cards/${cardId}/items/${position}/complete`, body);
    },

    async uncompleteItem(cardId, position) {
      return API.request('PUT', `/api/cards/${cardId}/items/${position}/uncomplete`);
    },

    async updateNotes(cardId, position, notes, proofUrl) {
      return API.request('PUT', `/api/cards/${cardId}/items/${position}/notes`, {
        notes,
        proof_url: proofUrl,
      });
    },

    async getArchive() {
      return API.request('GET', '/api/cards/archive');
    },

    async getStats(cardId) {
      return API.request('GET', `/api/cards/${cardId}/stats`);
    },

    async getExportable() {
      return API.request('GET', '/api/cards/export');
    },

    async import(cardData) {
      return API.request('POST', '/api/cards/import', cardData, { allowConflictResponse: true });
    },
  },

  // Suggestion endpoints
  suggestions: {
    async getAll() {
      return API.request('GET', '/api/suggestions');
    },

    async getGrouped() {
      return API.request('GET', '/api/suggestions?grouped=true');
    },

    async getByCategory(category) {
      return API.request('GET', `/api/suggestions?category=${encodeURIComponent(category)}`);
    },

    async getCategories() {
      return API.request('GET', '/api/suggestions/categories');
    },
  },

  // Friend endpoints
  friends: {
    async list() {
      return API.request('GET', '/api/friends');
    },

    async search(query) {
      return API.request('GET', `/api/friends/search?q=${encodeURIComponent(query)}`);
    },

    async sendRequest(friendId) {
      return API.request('POST', '/api/friends/requests', { friend_id: friendId });
    },

    async acceptRequest(friendshipId) {
      return API.request('PUT', `/api/friends/requests/${friendshipId}/accept`);
    },

    async rejectRequest(friendshipId) {
      return API.request('PUT', `/api/friends/requests/${friendshipId}/reject`);
    },

    async remove(friendshipId) {
      return API.request('DELETE', `/api/friends/${friendshipId}`);
    },

    async cancelRequest(friendshipId) {
      return API.request('DELETE', `/api/friends/requests/${friendshipId}/cancel`);
    },

    async getCard(friendshipId) {
      return API.request('GET', `/api/friends/${friendshipId}/card`);
    },

    async getCards(friendshipId) {
      return API.request('GET', `/api/friends/${friendshipId}/cards`);
    },

    async block(userId) {
      return API.request('POST', '/api/blocks', { user_id: userId });
    },

    async unblock(userId) {
      return API.request('DELETE', `/api/blocks/${userId}`);
    },

    async listBlocked() {
      return API.request('GET', '/api/blocks');
    },

    async createInvite(expiresInDays) {
      return API.request('POST', '/api/friends/invites', { expires_in_days: expiresInDays });
    },

    async listInvites() {
      return API.request('GET', '/api/friends/invites');
    },

    async revokeInvite(inviteId) {
      return API.request('DELETE', `/api/friends/invites/${inviteId}/revoke`);
    },

    async acceptInvite(token) {
      return API.request('POST', '/api/friends/invites/accept', { token });
    },
  },

  // Notification endpoints
  notifications: {
    async list({ unread = false, limit = 50, before = null } = {}) {
      const params = new URLSearchParams();
      if (unread) params.set('unread', '1');
      if (limit) params.set('limit', String(limit));
      if (before) params.set('before', before);
      const query = params.toString();
      const path = query ? `/api/notifications?${query}` : '/api/notifications';
      return API.request('GET', path);
    },

    async markRead(id) {
      return API.request('POST', `/api/notifications/${id}/read`);
    },

    async markAllRead() {
      return API.request('POST', '/api/notifications/read-all');
    },

    async delete(id) {
      return API.request('DELETE', `/api/notifications/${id}`);
    },

    async deleteAll() {
      return API.request('DELETE', '/api/notifications');
    },

    async unreadCount() {
      return API.request('GET', '/api/notifications/unread-count');
    },

    async getSettings() {
      return API.request('GET', '/api/notifications/settings');
    },

    async updateSettings(patch) {
      return API.request('PUT', '/api/notifications/settings', patch);
    },
  },

  // Reminder endpoints
  reminders: {
    async getSettings() {
      return API.request('GET', '/api/reminders/settings');
    },

    async updateSettings(patch) {
      return API.request('PUT', '/api/reminders/settings', patch);
    },

    async listCards() {
      return API.request('GET', '/api/reminders/cards');
    },

    async upsertCardCheckin(cardId, payload) {
      return API.request('PUT', `/api/reminders/cards/${cardId}`, payload);
    },

    async deleteCardCheckin(cardId) {
      return API.request('DELETE', `/api/reminders/cards/${cardId}`);
    },

    async listGoals(cardId = null) {
      const path = cardId ? `/api/reminders/goals?card_id=${encodeURIComponent(cardId)}` : '/api/reminders/goals';
      return API.request('GET', path);
    },

    async upsertGoalReminder(payload) {
      return API.request('POST', '/api/reminders/goals', payload);
    },

    async deleteGoalReminder(id) {
      return API.request('DELETE', `/api/reminders/goals/${id}`);
    },

    async sendTestEmail(cardId) {
      return API.request('POST', '/api/reminders/test', { card_id: cardId });
    },
  },

  // Reaction endpoints
  reactions: {
    async add(itemId, emoji) {
      return API.request('POST', `/api/items/${itemId}/react`, { emoji });
    },

    async remove(itemId) {
      return API.request('DELETE', `/api/items/${itemId}/react`);
    },

    async get(itemId) {
      return API.request('GET', `/api/items/${itemId}/reactions`);
    },

    async getAllowedEmojis() {
      return API.request('GET', '/api/reactions/emojis');
    },
  },

  // Token endpoints
  tokens: {
    async list() {
      return API.request('GET', '/api/tokens');
    },

    async create(name, scope, expiresInDays) {
      return API.request('POST', '/api/tokens', {
        name,
        scope,
        expires_in_days: parseInt(expiresInDays, 10),
      });
    },

    async delete(id) {
      return API.request('DELETE', `/api/tokens/${id}`);
    },

    async deleteAll() {
      return API.request('DELETE', '/api/tokens');
    },
  },

  // Support endpoint
  support: {
    async submit(email, category, message) {
      return API.request('POST', '/api/support', { email, category, message });
    },
  },

  // Share endpoints
  share: {
    async get(token) {
      return API.request('GET', `/api/share/${encodeURIComponent(token)}`);
    },
  },

  // AI endpoints
  ai: {
    async generate(category, focus, difficulty, budget, context, count = 24) {
      return API.request('POST', '/api/ai/generate', {
        category,
        focus,
        difficulty,
        budget,
        context,
        count,
      }, { timeout: 100000 });
    },
    async guide(mode, currentGoal, hint, count, avoid) {
      return API.request('POST', '/api/ai/guide', {
        mode,
        current_goal: currentGoal,
        hint,
        count,
        avoid,
      }, { timeout: 100000 });
    },
  },
};

class APIError extends Error {
  constructor(message, status, data = null) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.data = data;
  }
}

// Initialize API on load
document.addEventListener('DOMContentLoaded', () => {
  API.init();
});
