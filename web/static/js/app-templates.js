// Year of Bingo - Templates Module (scaffold)
// SCAFFOLD: Not yet loaded in production. See plans/refactor.md for extraction status.

window.App = window.App || {};
var App = window.App;

if (!App._moduleTemplatesLoaded) {
  App._moduleTemplatesLoaded = true;

Object.assign(App, {
  async renderTemplates(container) {
    this.currentView = 'templates';
    const canUseTemplates = this.hasFeature('templates');

    container.innerHTML = `
      <div class="templates-page">
        <div class="flex justify-between items-center mb-lg flex-wrap gap-sm">
          <div>
            <h1 class="m-0">Templates</h1>
            <p class="text-muted mt-sm">Save reusable templates and create a new year’s card in one click.</p>
          </div>
          <div class="flex gap-sm flex-wrap">
            ${canUseTemplates ? `
              <button class="btn btn-primary" data-action="show-create-template-modal">New template</button>
            ` : `
              <a href="/premium" class="btn btn-primary">Upgrade</a>
            `}
          </div>
        </div>

        ${canUseTemplates ? '' : `
          <div class="card mb-lg">
            <h3 class="mt-0">Premium feature</h3>
            <p class="text-muted mb-md">You can view existing templates, but creating, editing, and using templates requires Premium.</p>
            <div class="flex gap-sm flex-wrap">
              <a href="/premium" class="btn btn-primary">See Premium</a>
              <button class="btn btn-secondary" data-action="open-upgrade-modal">Upgrade</button>
            </div>
          </div>
        `}

        <div id="templates-list">
          <div class="text-center"><div class="spinner spinner--small"></div></div>
        </div>
      </div>
    `;

    const listEl = document.getElementById('templates-list');
    if (!listEl) return;

    try {
      const response = await API.templates.list();
      const templates = response?.templates || [];
      if (templates.length === 0) {
        listEl.innerHTML = `
          <div class="card text-center p-2xl">
            <h3>No templates yet</h3>
            <p class="text-muted mb-lg">Save a template to reuse it year after year.</p>
            ${canUseTemplates ? `
              <button class="btn btn-primary" data-action="show-create-template-modal">Create your first template</button>
            ` : `
              <a href="/premium" class="btn btn-primary">Upgrade to create templates</a>
            `}
          </div>
        `;
        return;
      }

      listEl.innerHTML = templates.map((t) => {
        const name = this.escapeHtml(t.name || 'Untitled');
        const size = `${parseInt(t.grid_size, 10) || 5}x${parseInt(t.grid_size, 10) || 5}`;
        const freeLabel = t.has_free_space ? ' • FREE' : '';
        const categoryLabel = t.category ? ` • ${this.escapeHtml(t.category)}` : '';
        const updated = t.updated_at ? new Date(t.updated_at).toLocaleDateString() : '';
        const updatedLabel = updated ? ` • Updated ${this.escapeHtml(updated)}` : '';
        const canEdit = canUseTemplates;
        const useAction = canUseTemplates ? 'use-template' : 'open-upgrade-modal';
        const editAction = canUseTemplates ? 'edit-template' : 'open-upgrade-modal';
        const deleteAction = canUseTemplates ? 'delete-template' : 'open-upgrade-modal';

        return `
          <div class="card">
            <div class="flex justify-between items-start gap-md flex-wrap">
              <div>
                <h3 class="mt-0 mb-sm">${name}</h3>
                <p class="text-muted m-0">${this.escapeHtml(size)}${freeLabel}${categoryLabel}${updatedLabel}</p>
              </div>
              <div class="flex gap-sm flex-wrap">
                <button class="btn btn-secondary" data-action="view-template" data-template-id="${this.escapeHtml(t.id)}">View</button>
                <button class="btn btn-primary" data-action="${useAction}" data-template-id="${this.escapeHtml(t.id)}">${canEdit ? 'Use' : 'Use (Premium)'}</button>
                <button class="btn btn-ghost" data-action="${editAction}" data-template-id="${this.escapeHtml(t.id)}">${canEdit ? 'Edit' : 'Edit (Premium)'}</button>
                <button class="btn btn-ghost btn-danger-outline" data-action="${deleteAction}" data-template-id="${this.escapeHtml(t.id)}">${canEdit ? 'Delete' : 'Delete (Premium)'}</button>
              </div>
            </div>
          </div>
        `;
      }).join('');
    } catch (error) {
      listEl.innerHTML = `
        <div class="card text-center p-2xl">
          <h3>Couldn’t load templates</h3>
          <p class="text-muted mb-lg" id="templates-error"></p>
          <a href="/templates" class="btn btn-secondary">Retry</a>
        </div>
      `;
      const errEl = document.getElementById('templates-error');
      if (errEl) errEl.textContent = error.message;
    }
  },

  async showCreateTemplateModal() {
    if (!this.hasFeature('templates')) {
      this.openUpgradeModal();
      return;
    }
    this.openModal('New template', `<div class="text-center"><div class="spinner spinner--small"></div></div>`);

    let cards = [];
    try {
      const res = await API.cards.list();
      cards = res?.cards || [];
    } catch (error) {
      cards = [];
    }

    let categories = [];
    try {
      const res = await API.cards.getCategories();
      categories = res.categories || [];
    } catch (error) {
      categories = this.getFallbackCategories();
    }

    const cardOptions = cards.map((card) => {
      const displayName = this.getCardDisplayNameRaw ? this.getCardDisplayNameRaw(card) : this.getCardDisplayName(card);
      const label = `${this.escapeHtml(displayName)} (${this.escapeHtml(String(card.year))})`;
      return `<option value="${this.escapeHtml(card.id)}">${label}</option>`;
    }).join('');

    const categoryOptions = [
      `<option value="">(no category)</option>`,
      ...categories.map((c) => `<option value="${this.escapeHtml(c.id)}">${this.escapeHtml(c.name)}</option>`),
    ].join('');

    this.openModal('New template', `
      <form data-action="create-template">
        <div class="form-error hidden mb-md" id="template-create-error" role="alert"></div>

        <div class="form-group">
          <label for="template-create-mode">Create from</label>
          <select id="template-create-mode" class="form-input">
            <option value="from_card" selected>Existing card</option>
            <option value="blank">Blank template</option>
          </select>
        </div>

        <div class="form-group">
          <label for="template-create-name">Template name</label>
          <input id="template-create-name" class="form-input" type="text" maxlength="100" placeholder="e.g., 2026 Goals" required />
        </div>

        <div id="template-create-from-card">
          <div class="form-group">
            <label for="template-create-card-id">Card</label>
            <select id="template-create-card-id" class="form-input" ${cards.length ? '' : 'disabled'}>
              ${cards.length ? cardOptions : '<option value="">No cards found</option>'}
            </select>
            <small class="text-muted">Copies the current items from the selected card.</small>
          </div>
        </div>

        <div id="template-create-blank" class="hidden">
          <div class="form-group">
            <label for="template-create-category">Category <span class="text-muted fw-normal">(optional)</span></label>
            <select id="template-create-category" class="form-input">${categoryOptions}</select>
          </div>

          <div class="form-group">
            <label for="template-create-grid-size">Grid size</label>
            <select id="template-create-grid-size" class="form-input">
              <option value="2">2x2</option>
              <option value="3">3x3</option>
              <option value="4">4x4</option>
              <option value="5" selected>5x5</option>
            </select>
          </div>

          <div class="form-group">
            <label class="checkbox-label">
              <input type="checkbox" id="template-create-free-space" checked />
              <span>Include FREE space</span>
            </label>
          </div>

          <div class="form-group">
            <label class="checkbox-label">
              <input type="checkbox" id="template-create-visible" checked />
              <span>Default: visible to friends</span>
            </label>
          </div>

          <div class="form-group">
            <label for="template-create-header">Header</label>
            <input type="text" id="template-create-header" class="form-input" maxlength="5" value="BINGO" required />
            <small class="text-muted" id="template-create-header-help">1-5 characters.</small>
          </div>

          <div class="form-group">
            <label for="template-create-items">Items</label>
            <textarea id="template-create-items" class="form-input" rows="8" placeholder="One item per line"></textarea>
            <small class="text-muted">Each item must be 1-500 characters.</small>
          </div>
        </div>

        <div class="flex gap-sm mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary flex-1">Create</button>
        </div>
      </form>
    `);

    const modeEl = document.getElementById('template-create-mode');
    const fromEl = document.getElementById('template-create-from-card');
    const blankEl = document.getElementById('template-create-blank');
    const applyMode = () => {
      const mode = modeEl?.value || 'from_card';
      if (mode === 'blank') {
        fromEl?.classList.add('hidden');
        blankEl?.classList.remove('hidden');
      } else {
        blankEl?.classList.add('hidden');
        fromEl?.classList.remove('hidden');
      }
    };
    modeEl?.addEventListener('change', applyMode);
    applyMode();

    const gridSizeEl = document.getElementById('template-create-grid-size');
    const headerEl = document.getElementById('template-create-header');
    const headerHelpEl = document.getElementById('template-create-header-help');
    if (gridSizeEl && headerEl) {
      const applyHeader = () => {
        const n = parseInt(gridSizeEl.value, 10) || 5;
        headerEl.maxLength = n;
        if (headerHelpEl) headerHelpEl.textContent = `1-${n} characters.`;
        if (headerEl.value.length > n) headerEl.value = Array.from(headerEl.value).slice(0, n).join('');
        if (!headerEl.dataset.touched) headerEl.value = Array.from('BINGO').slice(0, n).join('');
      };
      headerEl.addEventListener('input', () => { headerEl.dataset.touched = 'true'; });
      gridSizeEl.addEventListener('change', applyHeader);
      applyHeader();
    }
  },

  async showCreateTemplateFromCardModal(cardId) {
    if (!this.hasFeature('templates')) {
      this.openUpgradeModal();
      return;
    }
    if (!cardId) return;

    let card = null;
    try {
      const res = await API.cards.get(cardId);
      card = res?.card || null;
    } catch (error) {
      card = this.currentCard && this.currentCard.id === cardId ? this.currentCard : null;
    }
    const displayName = card ? (this.getCardDisplayNameRaw ? this.getCardDisplayNameRaw(card) : this.getCardDisplayName(card)) : '';
    const suggestedName = card ? `${displayName} Template` : 'New template';

    this.openModal('Save as template', `
      <form data-action="create-template-from-card" data-card-id="${this.escapeHtml(cardId)}">
        <div class="form-error hidden mb-md" id="template-from-card-error" role="alert"></div>
        <div class="form-group">
          <label for="template-from-card-name">Template name</label>
          <input id="template-from-card-name" class="form-input" type="text" maxlength="100" value="${this.escapeHtml(suggestedName)}" required />
        </div>
        <div class="flex gap-sm mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary flex-1">Save</button>
        </div>
      </form>
    `);
    document.getElementById('template-from-card-name')?.focus?.();
  },

  async showTemplateModal(templateId) {
    if (!templateId) return;
    this.openModal('Template', `<div class="text-center"><div class="spinner spinner--small"></div></div>`);
    try {
      const tpl = await API.templates.get(templateId);
      const t = tpl?.template || {};
      const items = tpl?.items || [];

      const title = this.escapeHtml(t.name || 'Template');
      const size = `${parseInt(t.grid_size, 10) || 5}x${parseInt(t.grid_size, 10) || 5}`;
      const freeLabel = t.has_free_space ? 'Yes' : 'No';
      const categoryLabel = t.category ? this.escapeHtml(t.category) : '(none)';
      const defaultVisible = t.default_visible_to_friends ? 'Yes' : 'No';

      const itemsHtml = items.length ? `
        <ol class="mt-md">
          ${items.map((it) => `<li>${this.escapeHtml(it.content || '')}</li>`).join('')}
        </ol>
      ` : `<p class="text-muted mt-md">No items saved in this template.</p>`;

      const canUseTemplates = this.hasFeature('templates');
      const actions = canUseTemplates ? `
        <div class="flex gap-sm flex-wrap mt-lg">
          <button type="button" class="btn btn-primary" data-action="use-template" data-template-id="${this.escapeHtml(templateId)}">Use template</button>
          <button type="button" class="btn btn-secondary" data-action="edit-template" data-template-id="${this.escapeHtml(templateId)}">Edit</button>
          <button type="button" class="btn btn-ghost btn-danger-outline" data-action="delete-template" data-template-id="${this.escapeHtml(templateId)}">Delete</button>
          <button type="button" class="btn btn-ghost" data-action="close-modal">Close</button>
        </div>
      ` : `
        <div class="flex gap-sm flex-wrap mt-lg">
          <a href="/premium" class="btn btn-primary">Upgrade to use</a>
          <button type="button" class="btn btn-secondary" data-action="open-upgrade-modal">Upgrade</button>
          <button type="button" class="btn btn-ghost" data-action="close-modal">Close</button>
        </div>
      `;

      this.openModal('Template', `
        <div class="card">
          <h3 class="mt-0">${title}</h3>
          <p class="text-muted m-0">${this.escapeHtml(size)} • FREE: ${this.escapeHtml(freeLabel)} • Category: ${categoryLabel} • Default visible: ${this.escapeHtml(defaultVisible)}</p>
          ${itemsHtml}
          ${actions}
        </div>
      `);
    } catch (error) {
      this.openModal('Template', `
        <div class="card text-center p-2xl">
          <h3>Couldn’t load template</h3>
          <p class="text-muted mb-lg" id="template-load-error"></p>
          <button class="btn btn-ghost" data-action="close-modal">Close</button>
        </div>
      `);
      const errEl = document.getElementById('template-load-error');
      if (errEl) errEl.textContent = error.message;
    }
  },

  async showEditTemplateModal(templateId) {
    if (!this.hasFeature('templates')) {
      this.openUpgradeModal();
      return;
    }
    if (!templateId) return;
    this.openModal('Edit template', `<div class="text-center"><div class="spinner spinner--small"></div></div>`);

    let tpl = null;
    try {
      tpl = await API.templates.get(templateId);
    } catch (error) {
      this.toast(error.message, 'error');
      return;
    }

    let categories = [];
    try {
      const res = await API.cards.getCategories();
      categories = res.categories || [];
    } catch (error) {
      categories = this.getFallbackCategories();
    }

    const t = tpl?.template || {};
    const items = tpl?.items || [];
    const itemsText = items.map((it) => it.content || '').join('\n');

    const categoryOptions = [
      `<option value="">(no category)</option>`,
      ...categories.map((c) => `<option value="${this.escapeHtml(c.id)}" ${t.category === c.id ? 'selected' : ''}>${this.escapeHtml(c.name)}</option>`),
    ].join('');

    const gridSize = parseInt(t.grid_size, 10) || 5;
    const headerText = t.header_text || 'BINGO';

    this.openModal('Edit template', `
      <form data-action="update-template"
            data-template-id="${this.escapeHtml(templateId)}"
            data-original-grid-size="${this.escapeHtml(String(gridSize))}"
            data-original-has-free-space="${t.has_free_space ? 'true' : 'false'}">
        <div class="form-error hidden mb-md" id="template-edit-error" role="alert"></div>

        <div class="form-group">
          <label for="template-edit-name">Template name</label>
          <input id="template-edit-name" class="form-input" type="text" maxlength="100" value="${this.escapeHtml(t.name || '')}" required />
        </div>

        <div class="form-group">
          <label for="template-edit-category">Category <span class="text-muted fw-normal">(optional)</span></label>
          <select id="template-edit-category" class="form-input">${categoryOptions}</select>
        </div>

        <div class="form-group">
          <label for="template-edit-grid-size">Grid size</label>
          <select id="template-edit-grid-size" class="form-input">
            <option value="2" ${gridSize === 2 ? 'selected' : ''}>2x2</option>
            <option value="3" ${gridSize === 3 ? 'selected' : ''}>3x3</option>
            <option value="4" ${gridSize === 4 ? 'selected' : ''}>4x4</option>
            <option value="5" ${gridSize === 5 ? 'selected' : ''}>5x5</option>
          </select>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" id="template-edit-free-space" ${t.has_free_space ? 'checked' : ''} />
            <span>Include FREE space</span>
          </label>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" id="template-edit-visible" ${t.default_visible_to_friends ? 'checked' : ''} />
            <span>Default: visible to friends</span>
          </label>
        </div>

        <div class="form-group">
          <label for="template-edit-header">Header</label>
          <input type="text" id="template-edit-header" class="form-input" maxlength="${gridSize}" value="${this.escapeHtml(headerText)}" required />
          <small class="text-muted" id="template-edit-header-help">1-${gridSize} characters.</small>
        </div>

        <div class="form-group">
          <label for="template-edit-items">Items</label>
          <textarea id="template-edit-items" class="form-input" rows="10" placeholder="One item per line">${this.escapeHtml(itemsText)}</textarea>
        </div>

        <div class="flex gap-sm mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary flex-1">Save</button>
        </div>
      </form>
    `);

    const gridSizeEl = document.getElementById('template-edit-grid-size');
    const headerEl = document.getElementById('template-edit-header');
    const headerHelpEl = document.getElementById('template-edit-header-help');
    if (gridSizeEl && headerEl) {
      const applyHeader = () => {
        const n = parseInt(gridSizeEl.value, 10) || 5;
        headerEl.maxLength = n;
        if (headerHelpEl) headerHelpEl.textContent = `1-${n} characters.`;
        if (headerEl.value.length > n) headerEl.value = Array.from(headerEl.value).slice(0, n).join('');
      };
      gridSizeEl.addEventListener('change', applyHeader);
      applyHeader();
    }
  },

  async deleteTemplate(templateId) {
    if (!this.hasFeature('templates')) {
      this.openUpgradeModal();
      return;
    }
    if (!templateId) return;
    if (!confirm('Delete this template? This cannot be undone.')) return;
    try {
      await API.templates.del(templateId);
      this.toast('Template deleted', 'success');
      const container = document.getElementById('main-container');
      if (container) this.renderTemplates(container);
    } catch (error) {
      this.toast(error.message, 'error');
    }
  },

  async showCreateCardFromTemplateModal(templateId) {
    if (!this.hasFeature('templates')) {
      this.openUpgradeModal();
      return;
    }
    if (!templateId) return;
    this.openModal('Use template', `<div class="text-center"><div class="spinner spinner--small"></div></div>`);

    let tpl = null;
    try {
      tpl = await API.templates.get(templateId);
    } catch (error) {
      this.toast(error.message, 'error');
      return;
    }

    let categories = [];
    try {
      const res = await API.cards.getCategories();
      categories = res.categories || [];
    } catch (error) {
      categories = this.getFallbackCategories();
    }

    const currentYear = new Date().getFullYear();
    const nextYear = currentYear + 1;
    const t = tpl?.template || {};
    const defaultTitle = `${nextYear} Bingo Card`;

    const categoryOptions = [
      `<option value="">(use template category)</option>`,
      ...categories.map((c) => `<option value="${this.escapeHtml(c.id)}">${this.escapeHtml(c.name)}</option>`),
    ].join('');

    this.openModal('Use template', `
      <form data-action="create-card-from-template" data-template-id="${this.escapeHtml(templateId)}">
        <div class="form-error hidden mb-md" id="template-card-create-error" role="alert"></div>

        <div class="form-group">
          <label for="template-card-year">Year</label>
          <select id="template-card-year" class="form-input" required>
            <option value="${currentYear}">${currentYear}</option>
            <option value="${nextYear}" selected>${nextYear}</option>
          </select>
        </div>

        <div class="form-group">
          <label for="template-card-title">Title <span class="text-muted fw-normal">(optional)</span></label>
          <input id="template-card-title" class="form-input" type="text" maxlength="100" placeholder="${this.escapeHtml(defaultTitle)}" />
          <small class="text-muted">Leave blank for a default title.</small>
        </div>

        <div class="form-group">
          <label for="template-card-category">Category <span class="text-muted fw-normal">(optional)</span></label>
          <select id="template-card-category" class="form-input">${categoryOptions}</select>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" id="template-card-shuffle" checked />
            <span>Shuffle layout</span>
          </label>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" id="template-card-visible" ${t.default_visible_to_friends ? 'checked' : ''} />
            <span>Visible to friends</span>
          </label>
        </div>

        <div class="flex gap-sm mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary flex-1">Create card</button>
        </div>
      </form>
    `);
    document.getElementById('template-card-title')?.focus?.();
  },

  async showRolloverCardModal(cardId) {
    if (!this.hasFeature('templates')) {
      this.openUpgradeModal();
      return;
    }
    if (!cardId) return;

    let card = this.currentCard && this.currentCard.id === cardId ? this.currentCard : null;
    if (!card) {
      try {
        const res = await API.cards.get(cardId);
        card = res?.card || null;
      } catch (error) {
        this.toast(error.message, 'error');
        return;
      }
    }

    const currentYear = new Date().getFullYear();
    const maxYear = currentYear + 1;
    const suggestedYear = Math.min(maxYear, (parseInt(card.year, 10) || currentYear) + 1);
    const defaultTitle = `${suggestedYear} Bingo Card`;

    this.openModal('New Year rollover', `
      <form data-action="rollover-card" data-card-id="${this.escapeHtml(cardId)}">
        <div class="form-error hidden mb-md" id="rollover-error" role="alert"></div>

        <div class="form-group">
          <label for="rollover-year">Year</label>
          <input id="rollover-year" class="form-input" type="number" min="2020" max="${maxYear}" value="${suggestedYear}" required />
        </div>

        <div class="form-group">
          <label for="rollover-carry">Carry over</label>
          <select id="rollover-carry" class="form-input">
            <option value="all" selected>All items (reset completion)</option>
            <option value="incomplete_only">Incomplete items only (reset completion)</option>
          </select>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" id="rollover-shuffle" checked />
            <span>Shuffle layout</span>
          </label>
        </div>

        <div class="form-group">
          <label for="rollover-title">Title <span class="text-muted fw-normal">(optional)</span></label>
          <input id="rollover-title" class="form-input" type="text" maxlength="100" placeholder="${this.escapeHtml(defaultTitle)}" value="${this.escapeHtml(card.title || '')}" />
          <small class="text-muted">Leave blank to keep the same title (or use a default).</small>
        </div>

        <div class="flex gap-sm mt-lg">
          <button type="button" class="btn btn-ghost flex-1" data-action="close-modal">Cancel</button>
          <button type="submit" class="btn btn-primary flex-1">Create new card</button>
        </div>
      </form>
    `);
    document.getElementById('rollover-title')?.focus?.();
  },

  async handleCreateTemplate(event, form) {
    event.preventDefault();
    if (!this.hasFeature('templates')) {
      this.openUpgradeModal();
      return;
    }

    const errorEl = document.getElementById('template-create-error');
    if (errorEl) {
      errorEl.textContent = '';
      errorEl.classList.add('hidden');
    }

    const mode = document.getElementById('template-create-mode')?.value || 'from_card';
    const name = document.getElementById('template-create-name')?.value?.trim?.() || '';

    try {
      if (mode === 'from_card') {
        const fromCardId = document.getElementById('template-create-card-id')?.value || '';
        if (!fromCardId) throw new Error('Select a card');
        await API.templates.create({ from_card_id: fromCardId, name });
      } else {
        const categoryValue = document.getElementById('template-create-category')?.value || '';
        const category = categoryValue ? categoryValue : null;
        const gridSize = parseInt(document.getElementById('template-create-grid-size')?.value || '5', 10);
        const hasFreeSpace = !!document.getElementById('template-create-free-space')?.checked;
        const headerText = document.getElementById('template-create-header')?.value?.trim?.() || '';
        const defaultVisible = !!document.getElementById('template-create-visible')?.checked;
        const items = this.parseItemsFromTextarea('template-create-items');
        await API.templates.create({
          name,
          category,
          grid_size: gridSize,
          header_text: headerText,
          has_free_space: hasFreeSpace,
          default_visible_to_friends: defaultVisible,
          items,
        });
      }

      this.closeModal();
      this.toast('Template created', 'success');
      const container = document.getElementById('main-container');
      if (container) this.renderTemplates(container);
    } catch (error) {
      if (errorEl) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      } else {
        this.toast(error.message, 'error');
      }
    }
  },

  async handleCreateTemplateFromCard(event, form) {
    event.preventDefault();
    if (!this.hasFeature('templates')) {
      this.openUpgradeModal();
      return;
    }

    const cardId = form?.dataset?.cardId || '';
    const name = document.getElementById('template-from-card-name')?.value?.trim?.() || '';
    const errorEl = document.getElementById('template-from-card-error');
    if (errorEl) {
      errorEl.textContent = '';
      errorEl.classList.add('hidden');
    }

    try {
      await API.templates.create({ from_card_id: cardId, name });
      this.closeModal();
      this.toast('Template saved', 'success');
      this.navigate('/templates', { skipWarning: true });
    } catch (error) {
      if (errorEl) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      } else {
        this.toast(error.message, 'error');
      }
    }
  },

  async handleUpdateTemplate(event, form) {
    event.preventDefault();
    if (!this.hasFeature('templates')) {
      this.openUpgradeModal();
      return;
    }

    const templateId = form?.dataset?.templateId || '';
    const errorEl = document.getElementById('template-edit-error');
    if (errorEl) {
      errorEl.textContent = '';
      errorEl.classList.add('hidden');
    }

    try {
      const name = document.getElementById('template-edit-name')?.value?.trim?.() || '';
      const categoryValue = document.getElementById('template-edit-category')?.value || '';
      const category = categoryValue ? categoryValue : null;
      const gridSize = parseInt(document.getElementById('template-edit-grid-size')?.value || '5', 10);
      const hasFreeSpace = !!document.getElementById('template-edit-free-space')?.checked;
      const headerText = document.getElementById('template-edit-header')?.value?.trim?.() || '';
      const defaultVisible = !!document.getElementById('template-edit-visible')?.checked;
      const items = this.parseItemsFromTextarea('template-edit-items');

      const originalGridSize = parseInt(form?.dataset?.originalGridSize || '5', 10);
      const originalHasFreeSpace = form?.dataset?.originalHasFreeSpace !== 'false';
      const oldCapacity = this.getCardCapacity({ grid_size: originalGridSize, has_free_space: originalHasFreeSpace });
      const newCapacity = this.getCardCapacity({ grid_size: gridSize, has_free_space: hasFreeSpace });
      if (items.length > newCapacity) throw new Error('Too many items for this grid size');

      const updatePayload = {
        name,
        category,
        grid_size: gridSize,
        header_text: headerText,
        has_free_space: hasFreeSpace,
        default_visible_to_friends: defaultVisible,
      };

      // `ReplaceItems` validates against the template's current grid config, so
      // when increasing capacity we must update first.
      if (items.length > oldCapacity) {
        await API.templates.update(templateId, updatePayload);
        await API.templates.replaceItems(templateId, items);
      } else {
        await API.templates.replaceItems(templateId, items);
        await API.templates.update(templateId, updatePayload);
      }

      this.closeModal();
      this.toast('Template updated', 'success');
      const container = document.getElementById('main-container');
      if (container) this.renderTemplates(container);
    } catch (error) {
      if (errorEl) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      } else {
        this.toast(error.message, 'error');
      }
    }
  },

  async handleCreateCardFromTemplate(event, form) {
    event.preventDefault();
    if (!this.hasFeature('templates')) {
      this.openUpgradeModal();
      return;
    }

    const templateId = form?.dataset?.templateId || '';
    const errorEl = document.getElementById('template-card-create-error');
    if (errorEl) {
      errorEl.textContent = '';
      errorEl.classList.add('hidden');
    }

    const year = parseInt(document.getElementById('template-card-year')?.value || '0', 10);
    const titleRaw = document.getElementById('template-card-title')?.value?.trim?.() || '';
    const title = titleRaw ? titleRaw : null;
    const categoryValue = document.getElementById('template-card-category')?.value || '';
    const category = categoryValue ? categoryValue : null;
    const shuffle = !!document.getElementById('template-card-shuffle')?.checked;
    const visible = !!document.getElementById('template-card-visible')?.checked;

    try {
      const response = await API.templates.createCard(templateId, {
        year,
        title,
        category,
        shuffle_layout: shuffle,
        visible_to_friends: visible,
      });

      if (response?.error === 'Card conflict') {
        const suggested = response?.suggested_title || '';
        if (errorEl) {
          errorEl.textContent = `You already have a card named "${response?.conflict?.title || ''}" for ${response?.conflict?.year || year}.`;
          errorEl.classList.remove('hidden');
        }
        if (suggested) {
          const titleEl = document.getElementById('template-card-title');
          if (titleEl) {
            titleEl.value = suggested;
            titleEl.focus();
          }
        }
        return;
      }

      if (!response?.card?.id) {
        throw new Error('Unexpected response');
      }
      this.closeModal();
      this.toast('Card created', 'success');
      this.navigate(`/card/${response.card.id}`);
    } catch (error) {
      if (errorEl) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      } else {
        this.toast(error.message, 'error');
      }
    }
  },

  async handleRolloverCard(event, form) {
    event.preventDefault();
    if (!this.hasFeature('templates')) {
      this.openUpgradeModal();
      return;
    }

    const cardId = form?.dataset?.cardId || '';
    const errorEl = document.getElementById('rollover-error');
    if (errorEl) {
      errorEl.textContent = '';
      errorEl.classList.add('hidden');
    }

    const year = parseInt(document.getElementById('rollover-year')?.value || '0', 10);
    const carryOver = document.getElementById('rollover-carry')?.value || 'all';
    const shuffle = !!document.getElementById('rollover-shuffle')?.checked;
    const titleRaw = document.getElementById('rollover-title')?.value?.trim?.() || '';
    const title = titleRaw ? titleRaw : null;

    try {
      const response = await API.templates.rollover(cardId, {
        year,
        carry_over: carryOver,
        shuffle_layout: shuffle,
        title,
      });

      if (response?.error === 'Card conflict') {
        const suggested = response?.suggested_title || '';
        if (errorEl) {
          errorEl.textContent = `You already have a card named "${response?.conflict?.title || ''}" for ${response?.conflict?.year || year}.`;
          errorEl.classList.remove('hidden');
        }
        if (suggested) {
          const titleEl = document.getElementById('rollover-title');
          if (titleEl) {
            titleEl.value = suggested;
            titleEl.focus();
          }
        }
        return;
      }

      if (!response?.card?.id) {
        throw new Error('Unexpected response');
      }
      this.closeModal();
      this.toast('New card created', 'success');
      this.navigate(`/card/${response.card.id}`);
    } catch (error) {
      if (errorEl) {
        errorEl.textContent = error.message;
        errorEl.classList.remove('hidden');
      } else {
        this.toast(error.message, 'error');
      }
    }
  },

  parseItemsFromTextarea(id) {
    const el = document.getElementById(id);
    if (!el) return [];
    const raw = String(el.value || '');
    const lines = raw.split('\n').map(s => s.trim()).filter(s => s.length > 0);
    return lines;
  },
});
}
