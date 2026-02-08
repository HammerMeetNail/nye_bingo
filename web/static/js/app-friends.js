// Year of Bingo - Friends/Invites Module (scaffold)

window.App = window.App || {};
var App = window.App;

Object.assign(App, {
  // Friends page
  renderInviteGate(container, token) {
    if (!token) {
      container.innerHTML = `
        <div class="card text-center p-2xl">
          <h3>Invite link not found</h3>
          <p class="text-muted mb-lg">This invite link is missing or invalid.</p>
          <a href="/" class="btn btn-primary">Go Home</a>
        </div>
      `;
      return;
    }

    this.storePendingInviteToken(token);
    container.innerHTML = `
      <div class="card text-center p-2xl">
        <h3>Accept Friend Invite</h3>
        <p class="text-muted mb-lg">Sign in or create an account to accept this invite.</p>
        <div class="flex gap-md justify-center flex-wrap">
          <a href="/login" class="btn btn-primary">Sign In</a>
          <a href="/register" class="btn btn-secondary">Create Account</a>
        </div>
      </div>
    `;
  },

  async renderInviteAccept(container, token) {
    if (!token) {
      container.innerHTML = `
        <div class="card text-center p-2xl">
          <h3>Invite link not found</h3>
          <p class="text-muted mb-lg">This invite link is missing or invalid.</p>
          <a href="/friends" class="btn btn-primary">Back to Friends</a>
        </div>
      `;
      return;
    }

    container.innerHTML = `
      <div class="card text-center p-2xl">
        <div class="spinner spinner--spaced"></div>
        <p>Accepting invite...</p>
      </div>
    `;

    try {
      const response = await API.friends.acceptInvite(token);
      this.toast('Invite accepted!', 'success');
      container.innerHTML = `
        <div class="card text-center p-2xl">
          <h3>You're friends now!</h3>
          <p class="text-muted mb-lg">You are now connected with ${this.escapeHtml(response.inviter.username)}.</p>
          <a href="/friends" class="btn btn-primary">Go to Friends</a>
        </div>
      `;
    } catch (error) {
      container.innerHTML = `
        <div class="card text-center p-2xl">
          <h3>Invite Error</h3>
          <p class="text-muted mb-lg" id="invite-accept-error"></p>
          <a href="/friends" class="btn btn-primary">Back to Friends</a>
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
          <p class="text-muted mb-md">
            Share a private invite link. Anyone with the link can accept it.
            You can revoke invites at any time.
          </p>
          <div class="search-input-group items-center">
            <button class="btn btn-primary" id="create-invite-btn">Create Invite Link</button>
          </div>
          <div id="invite-result" class="mt-md"></div>
          <div id="invite-list" class="mt-md"></div>
        </div>

        <div class="friends-search card">
          <h3>Find Friends</h3>
          <p class="text-muted mb-md">
            Search for friends by their username. Users must enable "Make my profile searchable"
            in their <a href="/profile">Profile settings</a> to appear in search results.
          </p>
          <div class="search-input-group">
            <input type="text" id="friend-search" class="form-input" placeholder="Search by username...">
            <button class="btn btn-primary" id="search-btn">Search</button>
          </div>
          <div id="search-results" class="search-results"></div>
        </div>

        <div id="friend-requests" class="card hidden">
          <h3>Friend Requests</h3>
          <div id="requests-list"></div>
        </div>

        <div id="sent-requests" class="card hidden">
          <h3>Sent Requests</h3>
          <div id="sent-list"></div>
        </div>

        <div class="card">
          <h3>My Friends</h3>
          <div id="friends-list">
            <div class="text-center"><div class="spinner"></div></div>
          </div>
        </div>

        <div id="blocked-users" class="card hidden">
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
    resultEl.innerHTML = '<div class="spinner spinner--compact"></div>';

    try {
      const response = await API.friends.createInvite(14);
      const inviteURL = `${window.location.origin}/${response.url}`;
      resultEl.innerHTML = `
        <div class="card p-md">
          <div class="form-group mb-0">
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
        blockedEl.classList.add('hidden');
        return;
      }
      blockedEl.classList.remove('hidden');
      blockedListEl.innerHTML = blocked.map(user => `
        <div class="friend-item">
          <div>
            <strong>${this.escapeHtml(user.username)}</strong>
          </div>
          <button class="btn btn-ghost btn-sm" data-action="unblock-user" data-user-id="${user.id}">Unblock</button>
        </div>
      `).join('');
    } catch (error) {
      blockedEl.classList.remove('hidden');
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
        requestsEl.classList.remove('hidden');
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
        requestsEl.classList.add('hidden');
      }

      // Sent requests
      const sentEl = document.getElementById('sent-requests');
      const sentListEl = document.getElementById('sent-list');
      if (sent && sent.length > 0) {
        sentEl.classList.remove('hidden');
        sentListEl.innerHTML = sent.map(req => `
          <div class="friend-item">
            <div>
              <strong>${this.escapeHtml(req.friend_username)}</strong>
            </div>
            <button class="btn btn-ghost btn-sm" data-action="cancel-request" data-request-id="${req.id}">Cancel</button>
          </div>
        `).join('');
      } else {
        sentEl.classList.add('hidden');
      }

      // Friends list
      const friendsListEl = document.getElementById('friends-list');
      if (friends && friends.length > 0) {
        friendsListEl.innerHTML = friends.map(friend => {
          const otherUserId = friend.user_id === this.user.id ? friend.friend_id : friend.user_id;
          const friendName = this.escapeHtml(friend.friend_username);
          const premiumBadge = friend.friend_is_premium ? '<span class="badge badge-premium badge--sm">Premium</span>' : '';
          return `
            <div class="friend-item">
              <div>
                <strong>${friendName} ${premiumBadge}</strong>
              </div>
              <div class="friend-actions">
                <a href="/friend-card/${friend.id}" class="btn btn-secondary btn-sm">View Card</a>
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
      <div class="text-center"><div class="spinner spinner--spaced"></div></div>
    `;

    try {
      const response = await API.friends.getCards(friendshipId);

      if (!response.cards || response.cards.length === 0) {
        container.innerHTML = `
          <div class="card text-center p-2xl">
            <h3>No Cards Available</h3>
            <p class="text-muted mb-lg">This friend has no finalized cards yet.</p>
            <a href="/friends" class="btn btn-primary">Back to Friends</a>
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
        <div class="card text-center p-2xl">
          <h3>Error</h3>
          <p class="text-muted mb-lg" id="friend-card-error"></p>
          <a href="/friends" class="btn btn-primary">Back to Friends</a>
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
	    const currentYear = new Date().getFullYear();
	    const isArchived = this.currentCard.year < currentYear;
	    const displayName = this.getCardDisplayName(this.currentCard);
	    const categoryBadge = this.getCategoryBadge(this.currentCard);
	    const ownerPremiumBadge = this.friendCardOwner?.is_premium ? '<span class="badge badge-premium badge--sm">Premium</span>' : '';

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
          <a href="/friends" class="btn btn-ghost">&larr; Friends</a>
          <div class="friend-card-title">
            <div class="flex items-center gap-sm flex-wrap justify-center">
              <h2 class="m-0">${this.escapeHtml(this.friendCardOwner?.username || 'Friend')}'s ${displayName}</h2>
              ${ownerPremiumBadge}
              <span class="year-badge">${this.currentCard.year}</span>
              ${categoryBadge}
              ${isArchived ? '<span class="archive-badge">Archived</span>' : ''}
            </div>
          </div>
          ${cardSelector || '<div></div>'}
        </div>

        <div class="bingo-container bingo-container--finalized">
          <div class="bingo-grid bingo-grid--finalized bingo-grid--size-${gridSize} ${isArchived ? 'bingo-grid--archive' : ''}" id="bingo-grid">
            ${this.renderGrid(true)}
          </div>
	        </div>

	        <div class="finalized-card-progress">
	          <progress class="progress-bar" value="${completedCount}" max="${capacity}"></progress>
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
        ${!isCompleted ? '<p class="text-muted mt-md">This goal hasn\'t been completed yet.</p>' : ''}
      </div>
      <div class="mt-lg">
        <button type="button" class="btn btn-secondary btn-full" data-action="close-modal">
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
});
