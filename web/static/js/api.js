// NYE Bingo - API Client

const API = {
  csrfToken: null,

  async init() {
    await this.fetchCSRFToken();
  },

  async fetchCSRFToken() {
    try {
      const response = await fetch('/api/csrf');
      const data = await response.json();
      this.csrfToken = data.token;
    } catch (error) {
      console.error('Failed to fetch CSRF token:', error);
    }
  },

  async request(method, path, body = null) {
    const headers = {
      'Content-Type': 'application/json',
    };

    if (this.csrfToken && ['POST', 'PUT', 'DELETE', 'PATCH'].includes(method)) {
      headers['X-CSRF-Token'] = this.csrfToken;
    }

    const options = {
      method,
      headers,
      credentials: 'same-origin',
    };

    if (body && method !== 'GET') {
      options.body = JSON.stringify(body);
    }

    const response = await fetch(path, options);
    const data = await response.json();

    if (!response.ok) {
      throw new APIError(data.error || 'Request failed', response.status);
    }

    return data;
  },

  // Auth endpoints
  auth: {
    async register(email, password, displayName) {
      return API.request('POST', '/api/auth/register', {
        email,
        password,
        display_name: displayName,
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
  },

  // Card endpoints
  cards: {
    async create(year) {
      return API.request('POST', '/api/cards', { year });
    },

    async list() {
      return API.request('GET', '/api/cards');
    },

    async get(id) {
      return API.request('GET', `/api/cards/${id}`);
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

    async finalize(cardId) {
      return API.request('POST', `/api/cards/${cardId}/finalize`);
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
      return API.request('POST', '/api/friends/request', { friend_id: friendId });
    },

    async acceptRequest(friendshipId) {
      return API.request('PUT', `/api/friends/${friendshipId}/accept`);
    },

    async rejectRequest(friendshipId) {
      return API.request('PUT', `/api/friends/${friendshipId}/reject`);
    },

    async remove(friendshipId) {
      return API.request('DELETE', `/api/friends/${friendshipId}`);
    },

    async cancelRequest(friendshipId) {
      return API.request('DELETE', `/api/friends/${friendshipId}/cancel`);
    },

    async getCard(friendshipId) {
      return API.request('GET', `/api/friends/${friendshipId}/card`);
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
};

class APIError extends Error {
  constructor(message, status) {
    super(message);
    this.name = 'APIError';
    this.status = status;
  }
}

// Initialize API on load
document.addEventListener('DOMContentLoaded', () => {
  API.init();
});
