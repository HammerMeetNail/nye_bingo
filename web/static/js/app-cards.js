// Year of Bingo - Cards Module (scaffold)

window.App = window.App || {};
var App = window.App;

Object.assign(App, {
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
      <div class="text-center mb-lg section-divider">
        <button class="btn btn-secondary btn-lg btn-full flex items-center justify-center gap-sm" data-action="open-ai-wizard-from-modal">
            <span>✨</span> Generate with AI Wizard
        </button>
        <p class="text-muted mt-sm text-sm">Let AI create a custom card for you!</p>
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
            Title <span class="text-muted fw-normal">(optional)</span>
          </label>
          <input type="text" id="modal-card-title" class="form-input"
                 placeholder="e.g., Life Goals, Foods to Try"
                 maxlength="100">
          <small class="text-muted">Leave blank for default "${currentYear} Bingo Card"</small>
        </div>

        <div class="form-group">
          <label for="modal-card-category">
            Category <span class="text-muted fw-normal">(optional)</span>
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
          <label class="checkbox-label">
            <input type="checkbox" id="modal-card-free-space" checked>
            <span>Include FREE space</span>
          </label>
        </div>

        <div class="form-group">
          <label for="modal-card-header">Header</label>
          <input type="text" id="modal-card-header" class="form-input" maxlength="5" value="BINGO" required>
          <small class="text-muted" id="modal-card-header-help">1-5 characters.</small>
        </div>

        <div class="flex gap-sm mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary flex-1">Create Card</button>
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
      this.navigate(`/card/${response.card.id}`);
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

        <div class="flex gap-md justify-end">
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


  async renderDashboard(container) {
    this.selectedCards = [];

    container.innerHTML = `
      ${this.renderEmailVerificationBanner()}
      <div class="dashboard-page">
        <div class="dashboard-header">
          <h2>My Bingo Cards</h2>
        </div>
        <div id="cards-list">
          <div class="text-center"><div class="spinner spinner--spaced"></div></div>
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
        <div class="card text-center p-2xl">
          <div class="status-icon">🎯</div>
          <h3>No cards yet</h3>
          <p class="text-muted mb-lg">Create your first bingo card and start tracking your goals!</p>
          <button class="btn btn-primary btn-lg" data-action="show-create-card-modal">Create Your First Card</button>
        </div>
      `;
      return;
    }

    const hasSelection = this.selectedCards.length > 0;

    listEl.innerHTML = `
      <div class="dashboard-controls">
        <div class="dashboard-sort">
          <label for="sort-select" class="text-muted text-sm">Sort:</label>
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
	    const progressValue = card.is_finalized ? completedCount : itemCount;
	    const displayName = this.getCardDisplayName(card);
	    const categoryBadge = this.getCategoryBadge(card);
	    const visibilityIcon = card.visible_to_friends ? 'eye' : 'eye-slash';
	    const visibilityLabel = card.visible_to_friends ? 'Visible to friends' : 'Private';
	    const isSelected = this.selectedCards.includes(card.id);
	    const cardLink = card.is_archived ? `/archive-card/${card.id}` : `/card/${card.id}`;

    return `
      <div class="card dashboard-card-preview">
        <div class="dashboard-card-preview-header">
          <div class="dashboard-card-preview-main">
            <label class="dashboard-checkbox-label" data-stop-propagation="true">
              <input type="checkbox" class="dashboard-card-checkbox" data-card-id="${card.id}" ${isSelected ? 'checked' : ''} data-change-action="dashboard-selection">
            </label>
            <a href="${cardLink}" class="dashboard-card-preview-link">
              <div class="dashboard-card-preview-title-row">
                <h3 class="m-0">${displayName}</h3>
                <span class="year-badge">${card.year}</span>
                ${categoryBadge}
              </div>
              <p class="text-muted dashboard-card-preview-meta">
                ${card.is_finalized
                  ? `${completedCount}/${capacity} completed`
                  : `${itemCount}/${capacity} items added`}
              </p>
            </a>
          </div>
          <div class="dashboard-card-preview-actions">
            <span class="visibility-badge visibility-badge--${card.visible_to_friends ? 'visible' : 'private'}" title="${visibilityLabel}">
              <i class="fas fa-${visibilityIcon}"></i> ${card.visible_to_friends ? 'Visible' : 'Private'}
            </span>
            ${card.is_archived ? '<div class="archive-badge">Archived</div>' : ''}
            <button class="btn btn-ghost btn-sm dashboard-delete-btn" data-action="delete-card" data-card-id="${card.id}" data-stop-propagation="true" aria-label="Delete card" title="Delete card">
              <i class="fas fa-trash"></i>
            </button>
          </div>
	        </div>
	        <a href="${cardLink}" class="no-underline block">
	          <progress class="progress-bar mt-md" value="${progressValue}" max="${capacity}"></progress>
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
      <div class="card create-card-shell">
        <div class="card-header text-center">
          <h2 class="card-title">Create Your Bingo Card</h2>
          <p class="card-subtitle">Set up your bingo card - no account needed to start!</p>
        </div>

        <div class="card ai-upsell">
          <div class="ai-upsell-content">
            <div class="ai-upsell-icon">🧙</div>
            <div>
              <div class="ai-upsell-title">Want AI-generated goals?</div>
              <div class="text-muted ai-upsell-text">
                The AI Goal Wizard is available after you create an account.
              </div>
              <div class="ai-upsell-actions">
                <button type="button" class="btn btn-secondary btn-sm ai-upsell-btn" data-action="show-ai-auth-modal">
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
              Title <span class="text-muted fw-normal">(optional)</span>
            </label>
            <input type="text" id="card-title" class="form-input"
                   placeholder="e.g., Life Goals, Foods to Try"
                   maxlength="100">
            <small class="text-muted">Leave blank for default "${currentYear} Bingo Card"</small>
          </div>

          <div class="form-group">
            <label for="card-category">
              Category <span class="text-muted fw-normal">(optional)</span>
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
            <label class="checkbox-label">
              <input type="checkbox" id="card-free-space" checked>
              <span>Include FREE space</span>
            </label>
          </div>

          <div class="form-group">
            <label for="card-header">Header</label>
          <input type="text" id="card-header" class="form-input" maxlength="5" value="BINGO" required>
          <small class="text-muted" id="card-header-help">1-5 characters.</small>
          </div>

          <div class="flex gap-sm mt-md">
            <a href="/" class="btn btn-ghost btn-lg flex-1 text-center">Cancel</a>
            <button type="submit" class="btn btn-primary btn-lg flex-1">Create Card</button>
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
      <div class="card create-card-shell">
        <div class="text-center mb-lg section-divider">
            <button class="btn btn-secondary btn-lg btn-full flex items-center justify-center gap-sm" data-action="open-ai-wizard">
                <span>✨</span> Generate with AI Wizard
            </button>
            <p class="text-muted mt-sm text-sm">Let AI create a custom card for you!</p>
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
              Title <span class="text-muted fw-normal">(optional)</span>
            </label>
            <input type="text" id="card-title" class="form-input"
                   placeholder="e.g., Life Goals, Foods to Try"
                   maxlength="100">
            <small class="text-muted">Leave blank for default "${currentYear} Bingo Card"</small>
          </div>

          <div class="form-group">
            <label for="card-category">
              Category <span class="text-muted fw-normal">(optional)</span>
            </label>
            <select id="card-category" class="form-input">
              <option value="">None</option>
              ${categoryOptions}
            </select>
          </div>

          <div class="flex gap-sm mt-md">
            <a href="/dashboard" class="btn btn-ghost btn-lg flex-1 text-center">Cancel</a>
            <button type="submit" class="btn btn-primary btn-lg flex-1">Create Card</button>
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
      this.navigate(`/card/${response.card.id}`);
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
      this.navigate(`/card/${response.card.id}`);
      this.toast(`${year} card created!`, 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async renderCard(container, cardId, itemId = null) {
    container.innerHTML = `
      <div class="text-center"><div class="spinner spinner--spaced"></div></div>
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
        <div class="card text-center p-2xl">
          <h3>Card not found</h3>
          <p class="text-muted mb-lg" id="card-error-message"></p>
          <a href="/dashboard" class="btn btn-primary">Back to Dashboard</a>
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
              <a href="/register" class="anonymous-card-banner-link">Create an account</a> to save it permanently and unlock the AI Goal Wizard.
            </span>
          </div>
        </div>
      ` : ''}

      <div class="flex justify-between items-center mb-md">
        <a href="${isAnon ? '/' : '/dashboard'}" class="btn btn-ghost">&larr; Back</a>
        <div class="flex items-center gap-sm flex-wrap justify-center">
          <h2 class="m-0">${displayName}</h2>
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

	      <progress class="progress-bar" value="${itemCount}" max="${capacity}"></progress>
	      <p class="progress-text mb-lg">${itemCount}/${capacity} items added</p>

      <div class="card-editor-layout">
        <div class="bingo-container editor-grid">
          <div class="bingo-grid bingo-grid--size-${gridSize}" id="bingo-grid">
            ${this.renderGrid()}
          </div>
        </div>

        <div class="editor-sidebar">
          <div class="input-area editor-input">
            <input type="text" id="item-input" class="form-input" placeholder="Type your goal..." maxlength="500" ${itemCount >= capacity ? 'disabled' : ''}>
            <button class="btn btn-primary" id="add-btn" ${itemCount >= capacity ? 'disabled' : ''}>Add</button>
          </div>

          <div class="card-config-panel mt-075">
            <div class="form-group mb-075">
              <label class="form-label">Header</label>
              <input type="text" id="card-header-input" class="form-input" maxlength="${gridSize}">
              <small class="text-muted">1-${gridSize} characters.</small>
            </div>
            <label class="checkbox-label no-select">
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
              <button class="btn btn-secondary" data-action="save-template-from-card" data-card-id="${this.currentCard.id}" ${itemCount === 0 ? 'disabled' : ''}>
                ⭐ Save Template
              </button>
              <button class="btn btn-secondary" data-action="show-rollover-card-modal" data-card-id="${this.currentCard.id}" ${itemCount === 0 ? 'disabled' : ''}>
                📅 Rollover
              </button>
            ` : ''}
            <button class="btn btn-primary" id="finalize-btn" data-action="finalize-card" ${itemCount < capacity ? 'disabled' : ''}>
              ✓ Finalize Card
            </button>
          </div>

          <div class="suggestions-panel editor-suggestions">
            <div class="suggestions-header">
              <h3 class="suggestions-title">Suggestions</h3>
              <div class="flex gap-sm flex-wrap">
                ${isAnon ? `
                  <button class="btn btn-secondary btn-sm" data-action="show-ai-auth-modal" title="Create an account to use AI features">
                    🧙 AI
                  </button>
                ` : `
                  <button class="btn btn-secondary btn-sm" id="ai-btn" data-action="open-ai-wizard" data-card-id="${this.currentCard.id}" data-desired-count="${capacity - itemCount}" title="Generate goals with AI" ${itemCount >= capacity ? 'disabled' : ''}>
                    🧙 AI
                  </button>
                  ${this.hasFeature('ai_enhancements') ? `
                    <button class="btn btn-secondary btn-sm" id="ai-fill-empty-btn" data-action="ai-fill-empty-premium" title="Fill empty squares with Premium AI" ${itemCount >= capacity ? 'disabled' : ''}>
                      ✨ AI Fill
                    </button>
                  ` : ''}
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
              <button class="btn btn-ghost btn-ghost-danger" data-action="confirm-delete-anonymous-card">
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
        <p class="mb-lg">
          Clear all ${itemCount} items from this card? This can't be undone.
        </p>
        <div class="flex gap-md justify-end">
          <button class="btn btn-ghost" data-action="close-modal">Cancel</button>
          <button class="btn btn-danger" data-action="clear-card-items">Clear All</button>
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
      this.navigate('/create', { replace: true, skipWarning: true });
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

        <div class="flex gap-md justify-end">
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
      this.navigate('/', { skipWarning: true });
      this.toast('Card deleted', 'success');
    }
  },

	  renderFinalizedCard(container, options = {}) {
	    const readOnly = !!options.readOnly;
	    this.currentView = readOnly ? 'shared-card' : 'finalized-card';
	    document.querySelector('.page')?.classList.add('page--compact-main');
	    const completedCount = this.currentCard.items.filter(i => i.is_completed).length;
	    const gridSize = this.getGridSize(this.currentCard);
	    const capacity = this.getCardCapacity(this.currentCard);
	    const displayName = this.getCardDisplayName(this.currentCard);
	    const categoryBadge = this.getCategoryBadge(this.currentCard);

    const sharedView = !!options.shared;
    const showActions = !readOnly && this.user && !this.isAnonymousMode;
    const backLink = sharedView ? '/' : '/dashboard';
    const backLabel = sharedView ? 'Home' : 'Back';
    const sharedBadge = sharedView ? '<span class="badge badge-warning">Shared view</span>' : '';

    let actionsHtml = '';
    if (showActions) {
      const visibilityIcon = this.currentCard.visible_to_friends ? 'eye' : 'eye-slash';
      const visibilityLabel = this.currentCard.visible_to_friends ? 'Visible to friends' : 'Private';
      actionsHtml = `
        <button class="btn btn-ghost btn-sm" data-action="edit-card-meta" title="Edit card name">✏️</button>
        <button class="btn btn-ghost btn-sm" data-action="show-clone-card-modal" title="Clone card">📄</button>
        <button class="btn btn-ghost btn-sm" data-action="show-edit-finalized-card-modal" title="Edit finalized card (Premium)">📝</button>
        <button class="btn btn-ghost btn-sm" data-action="save-template-from-card" data-card-id="${this.currentCard.id}" title="Save as template">⭐</button>
        <button class="btn btn-ghost btn-sm" data-action="show-rollover-card-modal" data-card-id="${this.currentCard.id}" title="New Year rollover">📅</button>
        <button class="btn btn-ghost btn-sm" data-action="open-share-modal" title="Share card">🔗</button>
        <button class="visibility-toggle-btn ${this.currentCard.visible_to_friends ? 'visibility-toggle-btn--visible' : 'visibility-toggle-btn--private'}" data-action="toggle-card-visibility" data-card-id="${this.currentCard.id}" data-visible="${!this.currentCard.visible_to_friends}" title="${visibilityLabel}" aria-label="${visibilityLabel}">
          <i class="fas fa-${visibilityIcon}"></i>
          <span>${visibilityLabel}</span>
        </button>
      `;
    }

    container.innerHTML = `
      <div class="finalized-card-view">
        <div class="finalized-card-header">
          <a href="${backLink}" class="btn btn-ghost">&larr; ${backLabel}</a>
          <div class="finalized-card-title">
            <h2>${displayName}</h2>
            <span class="year-badge">${this.currentCard.year}</span>
            ${categoryBadge}
            ${sharedBadge}
          </div>
          <div class="card-header-actions">
            ${actionsHtml}
          </div>
        </div>

        <div class="bingo-container bingo-container--finalized">
          <div class="bingo-grid bingo-grid--finalized bingo-grid--size-${gridSize}" id="bingo-grid">
            ${this.renderGrid(true)}
          </div>
	        </div>

	        <div class="finalized-card-progress">
	          <progress class="progress-bar" value="${completedCount}" max="${capacity}"></progress>
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
        <div class="card text-center p-2xl">
          <h3>Invalid Share Link</h3>
          <p class="text-muted mb-lg">This share link is missing or malformed.</p>
          <a href="/" class="btn btn-primary">Back to Home</a>
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
        <div class="card text-center p-2xl">
          <h3>Share Link Not Found</h3>
          <p class="text-muted mb-lg" id="share-error"></p>
          <a href="/" class="btn btn-primary">Back to Home</a>
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
        <p class="mt-md"><span class="${statusClass}">${statusText}</span></p>
      </div>
      <div class="mt-lg">
        <button type="button" class="btn btn-secondary btn-full" data-action="close-modal">
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
        <div class="flex gap-md mt-lg">
          <button type="button" class="btn btn-secondary flex-1" data-action="close-modal">
            Close
          </button>
          <button type="button" class="btn btn-ghost flex-1" data-action="uncomplete-item" data-position="${position}">
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
          <div class="form-group mt-md">
            <label class="form-label">Notes (optional)</label>
            <textarea id="complete-notes" class="form-input" rows="3" placeholder="How did you accomplish this?"></textarea>
          </div>
          <div class="flex gap-md">
            <button type="button" class="btn btn-secondary flex-1" data-action="close-modal">
              Cancel
            </button>
            <button type="submit" class="btn btn-primary flex-1">
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
	      const progressEl = document.querySelector('progress.progress-bar');
	      if (progressEl) {
	        progressEl.max = capacity;
	        progressEl.value = completedCount;
	      }
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
	      const progressEl = document.querySelector('progress.progress-bar');
	      if (progressEl) {
	        progressEl.max = capacity;
	        progressEl.value = completedCount;
	      }
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
    const canUsePremiumAI = !isEmpty && !this.isAnonymousMode && this.hasFeature('ai_enhancements');
    const premiumMeter = this.formatPremiumAIStatusLine(this.premiumAIStatus);
    const premiumSection = canUsePremiumAI ? `
        <div class="form-group ai-guide-section">
          <label class="form-label">Goal Assistant (Premium)</label>
          ${premiumMeter ? `<small class="text-muted">${this.escapeHtml(premiumMeter)}</small>` : ''}
          <select id="ai-premium-mode" class="form-input form-input--sm mt-sm">
            <option value="breakdown">Break it down</option>
            <option value="next_step">Next step</option>
            <option value="obstacles">Obstacles</option>
            <option value="schedule">Schedule</option>
            <option value="ideas">Ideas</option>
            <option value="motivation">Motivation</option>
          </select>
          <textarea id="ai-premium-notes" class="form-input form-input--sm mt-sm" rows="2" maxlength="500" placeholder="Constraints / notes (optional)"></textarea>
          <button type="button" class="btn btn-secondary btn-sm" id="ai-premium-generate" data-action="ai-premium-assist" data-position="${position}">
            ✨ Ask Goal Assistant
          </button>
          <div id="ai-premium-results" class="ai-guide-results"></div>
        </div>
    ` : '';

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
          <button type="button" class="btn btn-danger flex-1" data-action="remove-item" data-position="${position}" ${isEmpty ? 'disabled aria-disabled="true" title="No goal to remove"' : ''}>
            Remove
          </button>
    `;

    this.openModal(modalTitle, `
      <form data-action="save-item-edit" data-position="${position}">
        <div class="form-group">
          <label class="form-label" for="edit-item-content-${position}">Goal</label>
          <textarea id="edit-item-content-${position}" class="form-input" rows="4" maxlength="500" autofocus>${this.escapeHtml(content)}</textarea>
        </div>
        ${premiumSection}
        ${aiSection}
        <div class="flex gap-md mt-lg">
          <button type="button" class="btn btn-secondary flex-1" data-action="close-modal">
            Cancel
          </button>
          ${removeButton}
          <button type="submit" class="btn btn-primary flex-1">
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

  async handleAIPremiumAssist(position) {
    if (this.isAnonymousMode || !this.user) {
      this.showAIAuthModal();
      return;
    }
    if (!this.hasFeature('ai_enhancements')) {
      this.navigate('/premium?upgrade=1', { skipWarning: true });
      return;
    }
    if (!this.currentCard?.id) {
      this.toast('No active card found', 'error');
      return;
    }

    const mode = (document.getElementById('ai-premium-mode')?.value || 'breakdown').trim();
    const notes = (document.getElementById('ai-premium-notes')?.value || '').trim();
    if (notes.length > 500) {
      this.toast('Notes must be 500 characters or less', 'error');
      return;
    }

    const resultsEl = document.getElementById('ai-premium-results');
    const button = document.getElementById('ai-premium-generate');
    const originalLabel = button ? button.textContent.trim() : '';
    if (button) {
      button.disabled = true;
      button.textContent = 'Thinking...';
    }
    if (resultsEl) {
      resultsEl.textContent = 'Generating guidance...';
    }

    try {
      const response = await API.ai.assistGoal(this.currentCard.id, position, mode, notes);
      if (resultsEl) {
        resultsEl.textContent = response?.reply || '';
      }
      this.applyPremiumAIUsageUpdate(response);
    } catch (error) {
      if (error?.status === 403 && /premium required/i.test(error?.message || '')) {
        this.navigate('/premium?upgrade=1', { skipWarning: true });
        return;
      }
      if (resultsEl && error?.data?.resets_at) {
        const reset = new Date(error.data.resets_at);
        resultsEl.textContent = `No AI Enhancements left this month. Resets ${reset.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}.`;
      }
      this.toast(error.message, 'error');
    } finally {
      if (button) {
        button.disabled = false;
        button.textContent = originalLabel || '✨ Ask Goal Assistant';
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
	      const progressEl = document.querySelector('progress.progress-bar');
	      if (progressEl) {
	        progressEl.max = capacity;
	        progressEl.value = itemCount;
	      }
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
      const aiFillBtn = document.getElementById('ai-fill-empty-btn');
      if (aiFillBtn) aiFillBtn.disabled = isFull;
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

  async saveItemEdit(event, position, form = null) {
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

    if (this._itemEditInFlightPositions.has(position)) return;
    this._itemEditInFlightPositions.add(position);
    const submitBtn = form ? form.querySelector('button[type="submit"]') : null;
    const submitLabel = submitBtn ? submitBtn.textContent : '';
    if (submitBtn) {
      submitBtn.disabled = true;
      submitBtn.textContent = 'Saving...';
    }

    const item = this.currentCard.items?.find(i => i.position === position);
    try {
      if (!item) {
        await this.addItemAtPosition(position, newContent);
        return;
      }
      const oldContent = item.content || '';

      if (newContent === oldContent) {
        this.closeModal();
        return;
      }

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
    } finally {
      this._itemEditInFlightPositions.delete(position);
      if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.textContent = submitLabel || 'Save';
      }
    }
  },

  async addItem() {
    const input = document.getElementById('item-input');
    const content = input.value.trim();

    if (!content) {
      this.toast('Please enter a goal', 'error');
      return;
    }

    if (this._addItemInFlight) return;
    this._addItemInFlight = true;
    const addBtn = document.getElementById('add-btn');
    const addLabel = addBtn ? addBtn.textContent : '';
    if (addBtn) {
      addBtn.disabled = true;
      addBtn.textContent = 'Adding...';
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
	      const progressEl = document.querySelector('progress.progress-bar');
	      if (progressEl) {
	        progressEl.max = capacity;
	        progressEl.value = itemCount;
	      }
	      document.querySelector('.progress-text').textContent = `${itemCount}/${capacity} items added`;

      // Update buttons
      if (itemCount >= capacity) {
        input.disabled = true;
        document.getElementById('add-btn').disabled = true;
        document.getElementById('fill-empty-btn').disabled = true;
        const aiFillBtn = document.getElementById('ai-fill-empty-btn');
        if (aiFillBtn) aiFillBtn.disabled = true;
        const finalizeBtn = document.getElementById('finalize-btn');
        if (finalizeBtn) finalizeBtn.disabled = false;
      }
      const clearBtn = document.getElementById('clear-btn');
      if (clearBtn) clearBtn.disabled = itemCount === 0;
      const aiBtn = document.getElementById('ai-btn');
      if (aiBtn) aiBtn.disabled = itemCount >= capacity;
      const aiFillBtn = document.getElementById('ai-fill-empty-btn');
      if (aiFillBtn) aiFillBtn.disabled = itemCount >= capacity;
      const shuffleBtn = document.getElementById('shuffle-btn');
      if (shuffleBtn) shuffleBtn.disabled = false;

      // Update suggestions
      this.refreshSuggestionsList();

      this.confetti();
    } catch (error) {
      this.toast(error.message, 'error');
    } finally {
      this._addItemInFlight = false;
      const itemCount = this.currentCard?.items ? this.currentCard.items.length : 0;
      const capacity = this.getCardCapacity(this.currentCard);
      const isFull = capacity > 0 && itemCount >= capacity;
      if (addBtn) {
        addBtn.disabled = isFull;
        addBtn.textContent = addLabel || 'Add';
      }
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
	    const progressEl = document.querySelector('progress.progress-bar');
	    if (progressEl) {
	      progressEl.max = capacity;
	      progressEl.value = itemCount;
	    }
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
    const aiFillBtn = document.getElementById('ai-fill-empty-btn');
    if (aiFillBtn) aiFillBtn.disabled = isFull;
    const shuffleBtn = document.getElementById('shuffle-btn');
    if (shuffleBtn) shuffleBtn.disabled = itemCount === 0;
    const finalizeBtn = document.getElementById('finalize-btn');
    if (finalizeBtn) finalizeBtn.disabled = itemCount < capacity;

    // Update suggestions panel
    this.refreshSuggestionsList();

    this.toast(`Added ${added} item${added !== 1 ? 's' : ''} to your card`, 'success');
  },

  async fillEmptyWithAI() {
    if (this.isAnonymousMode || !this.user) {
      this.showAIAuthModal();
      return;
    }
    if (!this.hasFeature('ai_enhancements')) {
      this.navigate('/premium?upgrade=1', { skipWarning: true });
      return;
    }
    if (!this.currentCard?.id) {
      this.toast('No active card found', 'error');
      return;
    }
    if (this.currentCard.is_finalized) {
      this.toast('Card must be a draft', 'error');
      return;
    }

    const currentItemCount = this.currentCard.items ? this.currentCard.items.length : 0;
    const capacity = this.getCardCapacity(this.currentCard);
    if (currentItemCount >= capacity) {
      this.toast('Card is already full', 'info');
      return;
    }

    const button = document.getElementById('ai-fill-empty-btn');
    const originalLabel = button ? button.textContent.trim() : '';
    if (button) {
      button.disabled = true;
      button.textContent = 'Filling...';
    }

    try {
      const response = await API.ai.fillEmpty(this.currentCard.id, 'mix', '', 'medium', 'free', '');
      if (response?.card) {
        this.currentCard = response.card;
      } else {
        const refreshed = await API.cards.get(this.currentCard.id);
        this.currentCard = refreshed.card;
      }

      this.usedSuggestions = new Set(
        (this.currentCard.items || [])
          .map(item => (item.content || '').toLowerCase())
          .filter(Boolean)
      );

      const grid = document.getElementById('bingo-grid');
      if (grid) grid.innerHTML = this.renderGrid();

      const itemCount = this.currentCard.items ? this.currentCard.items.length : 0;
      const isFull = itemCount >= capacity;
      const progressEl = document.querySelector('progress.progress-bar');
      if (progressEl) {
        progressEl.max = capacity;
        progressEl.value = itemCount;
      }
      const progressText = document.querySelector('.progress-text');
      if (progressText) {
        progressText.textContent = `${itemCount}/${capacity} items added`;
      }

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
      this.applyPremiumAIUsageUpdate(response);
      this.toast('Filled empty squares with AI', 'success');
    } catch (error) {
      if (error?.status === 403 && /premium required/i.test(error?.message || '')) {
        this.navigate('/premium?upgrade=1', { skipWarning: true });
        return;
      }
      this.toast(error.message, 'error');
    } finally {
      if (button) {
        const itemCount = this.currentCard?.items ? this.currentCard.items.length : 0;
        button.disabled = itemCount >= capacity;
        button.textContent = originalLabel || '✨ AI Fill';
      }
    }
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
	      const progressEl = document.querySelector('progress.progress-bar');
	      if (progressEl) {
	        progressEl.max = capacity;
	        progressEl.value = itemCount;
	      }
	      document.querySelector('.progress-text').textContent = `${itemCount}/${capacity} items added`;

      // Update buttons
      document.getElementById('item-input').disabled = false;
      document.getElementById('add-btn').disabled = false;
      document.getElementById('fill-empty-btn').disabled = false;
      const clearBtn = document.getElementById('clear-btn');
      if (clearBtn) clearBtn.disabled = itemCount === 0;
      const aiBtn = document.getElementById('ai-btn');
      if (aiBtn) aiBtn.disabled = itemCount >= capacity;
      const aiFillBtn = document.getElementById('ai-fill-empty-btn');
      if (aiFillBtn) aiFillBtn.disabled = itemCount >= capacity;
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
            Title <span class="text-muted fw-normal">(optional)</span>
          </label>
          <input type="text" id="clone-card-title" class="form-input"
                 maxlength="100">
        </div>

        <div class="form-group">
          <label for="clone-card-category">
            Category <span class="text-muted fw-normal">(optional)</span>
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
          <label class="checkbox-label">
            <input type="checkbox" id="clone-card-free-space" ${hasFree ? 'checked' : ''}>
            <span>Include FREE space</span>
          </label>
        </div>

        <div class="form-group">
          <label for="clone-card-header">Header</label>
          <input type="text" id="clone-card-header" class="form-input" maxlength="${gridSize}" required>
          <small class="text-muted" id="clone-card-header-help">1-${gridSize} characters.</small>
        </div>

        <div class="flex gap-sm mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary flex-1">Clone</button>
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
      this.navigate(`/card/${response.card.id}`);
      if (response.message) this.toast(response.message, 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  showEditFinalizedCardModal() {
    if (!this.currentCard || this.isAnonymousMode || !this.currentCard.is_finalized) return;
    if (!this.hasFeature('edit_after_finalize')) {
      this.openUpgradeModal();
      return;
    }

    const displayName = this.getCardDisplayName(this.currentCard);
    const defaultTitle = `${displayName} (Edit)`;

    this.openModal('Edit Finalized Card', `
      <form data-action="edit-finalized-card">
        <div class="form-group">
          <label for="edit-finalized-card-title">
            Title <span class="text-muted fw-normal">(optional)</span>
          </label>
          <input type="text" id="edit-finalized-card-title" class="form-input" maxlength="100">
          <small class="text-muted">Defaults to "${this.escapeHtml(defaultTitle)}".</small>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" id="edit-finalized-card-shuffle">
            <span>Shuffle layout</span>
          </label>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" id="edit-finalized-card-reset" checked>
            <span>Reset completion progress</span>
          </label>
          <small class="text-muted">Recommended for a fresh draft.</small>
        </div>

        <div class="form-error hidden" id="edit-finalized-card-error" role="alert"></div>

        <div class="flex gap-sm mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary flex-1">Create Edit Draft</button>
        </div>
      </form>
    `);

    const titleEl = document.getElementById('edit-finalized-card-title');
    if (titleEl) titleEl.value = defaultTitle;
  },

  async handleEditFinalizedCard(event) {
    event.preventDefault();
    if (!this.currentCard || this.isAnonymousMode || !this.currentCard.is_finalized) return;

    const titleEl = document.getElementById('edit-finalized-card-title');
    const shuffleEl = document.getElementById('edit-finalized-card-shuffle');
    const resetEl = document.getElementById('edit-finalized-card-reset');
    const errorEl = document.getElementById('edit-finalized-card-error');

    if (errorEl) {
      errorEl.classList.add('hidden');
      errorEl.textContent = '';
    }

    const title = titleEl?.value?.trim() || null;
    const shuffleLayout = !!shuffleEl?.checked;
    const resetProgress = resetEl?.checked !== false;

    try {
      const response = await API.cards.editFinalized(this.currentCard.id, {
        title,
        shuffle_layout: shuffleLayout,
        reset_progress: resetProgress,
      });

      if (response?.card) {
        this.closeModal();
        this.currentCard = response.card;
        this.navigate(`/card/${response.card.id}`);
        this.toast('Editable draft created', 'success');
        return;
      }

      if (response?.error === 'Card conflict') {
        if (titleEl && response.suggested_title) {
          titleEl.value = response.suggested_title;
        }
        if (errorEl) {
          errorEl.textContent = response.suggested_title
            ? `A card with that title already exists. Suggested: ${response.suggested_title}`
            : 'A card with that title already exists for this year.';
          errorEl.classList.remove('hidden');
        }
        return;
      }

      throw new Error(response?.error || 'Unable to create editable draft.');
    } catch (error) {
      if (error?.status === 403 && /premium required/i.test(error?.message || '')) {
        this.closeModal();
        this.openUpgradeModal();
        return;
      }

      if (errorEl) {
        errorEl.textContent = error?.message || 'Unable to create editable draft.';
        errorEl.classList.remove('hidden');
      } else {
        this.toast(error?.message || 'Unable to create editable draft.', 'error');
      }
    }
  },

  async showShareCardModal() {
    if (!this.currentCard || this.isAnonymousMode || this.isSharedView || !this.currentCard.is_finalized) return;

    this.openModal('Share Card', `
      <div id="share-modal-content">
        <div class="text-center"><div class="spinner spinner--compact"></div></div>
      </div>
    `);

    await this.refreshShareModal();
  },

  async refreshShareModal() {
    const content = document.getElementById('share-modal-content');
    if (!content) return;

    content.innerHTML = '<div class="text-center"><div class="spinner spinner--compact"></div></div>';
    try {
      const response = await API.cards.shareStatus(this.currentCard.id);
      this.currentShareStatus = response || { enabled: false };
      content.innerHTML = this.renderShareModalContent(this.currentShareStatus);
      this.bindShareExpiryControls();

      const rawUrl = this.currentShareStatus.url || '';
      let shareUrl = '';
      if (rawUrl) {
        if (/^https?:\/\//i.test(rawUrl)) {
          shareUrl = rawUrl;
        } else if (rawUrl.startsWith('#')) {
          shareUrl = `${window.location.origin}${rawUrl}`;
        } else if (rawUrl.startsWith('/')) {
          shareUrl = `${window.location.origin}${rawUrl}`;
        } else {
          shareUrl = `${window.location.origin}/s/${rawUrl}`;
        }
      }
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
      const startNowUtc = Date.UTC(
        now.getUTCFullYear(),
        now.getUTCMonth(),
        now.getUTCDate(),
      );
      const startExpiryUtc = Date.UTC(
        expiresAt.getUTCFullYear(),
        expiresAt.getUTCMonth(),
        expiresAt.getUTCDate(),
      );
      daysLeft = Math.round((startExpiryUtc - startNowUtc) / msInDay);
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
        <div id="share-expiry-custom-group" class="mt-075 hidden">
          <input type="number" id="share-expiry-custom" class="form-input" min="1" max="3650" placeholder="Enter days">
        </div>
      </div>
    `;
    const expirationNote = isEnabled
      ? '<p class="text-muted mt-sm">Disable sharing to change the expiration.</p>'
      : '';

    return `
      ${statusLine}
      ${expirationNote}
      ${linkSection}
      ${expirationControls}
      <div class="flex gap-sm flex-wrap justify-end">
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
      customGroup.classList.toggle('hidden', select.value !== 'custom');
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
      if (!Number.isFinite(days) || days < 0 || days > 3650) return null;
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
        <p class="mb-lg">
          Are you sure you want to finalize this card? You won't be able to change the items after this.
        </p>
        <div class="mb-lg">
          <label class="checkbox-label">
            <input type="checkbox" id="finalize-visibility" checked>
            <span>Visible to friends</span>
          </label>
          <p class="text-muted mt-sm text-sm">
            If unchecked, friends won't be able to see this card.
          </p>
        </div>
        <div class="flex gap-md justify-end">
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
        <p class="mb-lg">
          Your bingo card is ready! Create an account to save and finalize it.
        </p>
        <div class="flex flex-col gap-md">
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
        <p class="mb-lg">
          AI-generated goals are available after you create an account.
          This helps prevent abuse and keeps AI costs under control.
        </p>
        <div class="flex flex-col gap-md">
          <a class="btn btn-primary btn-lg" href="/register" data-action="close-modal">
            Create Account
          </a>
          <a class="btn btn-secondary btn-lg" href="/login" data-action="close-modal">
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
        <div class="flex gap-md mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="show-finalize-auth-modal">
            Back
          </button>
          <button type="submit" class="btn btn-primary flex-1">
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
	      this.applyAuthEntitlements(response);
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
        <div class="flex gap-md mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="show-finalize-auth-modal">
            Back
          </button>
          <button type="submit" class="btn btn-primary flex-1">
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
	      this.applyAuthEntitlements(response);
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
        <p class="mb-md">
          You already have a <strong>${existingCard.year}</strong> card:
        </p>
        <div class="card p-md mb-lg">
          <strong>${this.escapeHtml(existingTitle)}</strong>
          <p class="text-muted m-0 mt-xs">
            ${itemCount} items, ${isFinalized}
          </p>
        </div>
        <p class="mb-lg">What would you like to do?</p>
        <div class="flex flex-col gap-075">
          <button class="btn btn-secondary" data-action="conflict-keep-existing" data-card-id="${existingCard.id}">
            Keep Existing Card
          </button>
          <button class="btn btn-primary" data-action="conflict-save-as-new">
            Save as New Card (with different title)
          </button>
          <button class="btn btn-ghost btn-ghost-danger" data-action="conflict-replace" data-card-id="${existingCard.id}">
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
    this.navigate(`/card/${existingCardId}`, { skipWarning: true });
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
        <div class="flex gap-md mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="import-anonymous-card">
            Back
          </button>
          <button type="submit" class="btn btn-primary flex-1">
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
        <button class="btn btn-ghost btn-ghost-danger" data-action="create-conflict-replace" data-card-id="${existingCard.id}">
          Delete &amp; Create New
        </button>`;
    }

    buttons += `
      <button class="btn btn-ghost" data-action="close-modal">
        Cancel
      </button>`;

    this.openModal('Card Already Exists', `
      <div class="conflict-modal">
        <p class="mb-md">
          You already have a <strong>${existingCard.year}</strong> card:
        </p>
        <div class="card p-md mb-lg">
          <strong>${this.escapeHtml(existingTitle)}</strong>
          <p class="text-muted m-0 mt-xs">
            ${itemCount} items, ${isFinalized}
          </p>
        </div>
        <p class="mb-lg">What would you like to do?</p>
        <div class="flex flex-col gap-075">
          ${buttons}
        </div>
      </div>
    `);
  },

  // Handle create conflict: go to existing card
  handleCreateConflictGoToExisting(existingCardId) {
    this.closeModal();
    this.navigate(`/card/${existingCardId}`, { skipWarning: true });
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
        <div class="flex gap-md mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="close-modal">
            Cancel
          </button>
          <button type="submit" class="btn btn-primary flex-1">
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
      this.navigate(`/card/${response.card.id}`);
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
      this.navigate(`/card/${response.card.id}`);
      const cardName = `${ctx.year} Bingo Card`;
      this.toast(`${cardName} created!`, 'success');
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  checkForBingo() {

  async renderArchiveCard(container, cardId) {
    container.innerHTML = `
      <div class="text-center"><div class="spinner spinner--spaced"></div></div>
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
        <div class="card text-center p-2xl">
          <h3>Card not found</h3>
          <p class="text-muted mb-lg" id="archive-card-error"></p>
          <a href="/dashboard" class="btn btn-primary">Back to Dashboard</a>
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
	    const stats = this.currentStats;
	    const displayName = this.getCardDisplayName(this.currentCard);
	    const categoryBadge = this.getCategoryBadge(this.currentCard);
    const visibilityIcon = this.currentCard.visible_to_friends ? 'eye' : 'eye-slash';
    const visibilityLabel = this.currentCard.visible_to_friends ? 'Visible' : 'Private';
    const showShare = this.user && !this.isAnonymousMode && this.currentCard.is_finalized;

    container.innerHTML = `
      <div class="archive-card-view">
        <div class="archive-card-header">
          <a href="/dashboard" class="btn btn-ghost">&larr; Back</a>
          <div class="flex items-center gap-sm flex-wrap justify-center">
            <h2 class="m-0">${displayName}</h2>
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
          <div class="bingo-grid bingo-grid--finalized bingo-grid--archive bingo-grid--size-${gridSize}" id="bingo-grid">
            ${this.renderArchiveGrid()}
          </div>
	        </div>

	        <div class="finalized-card-progress">
	          <progress class="progress-bar" value="${completedCount}" max="${capacity}"></progress>
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
          ${completedAt ? `<p class="text-muted mt-sm">Completed on ${completedAt}</p>` : ''}
          ${notes ? `<p class="item-detail-notes"><strong>Notes:</strong> ${this.escapeHtml(notes)}</p>` : ''}
        ` : `
          <p class="text-muted mt-md">This goal was not completed.</p>
        `}
      </div>
      <div class="mt-lg">
        <button type="button" class="btn btn-secondary btn-full" data-action="close-modal">
          Close
        </button>
      </div>
    `);
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

});
