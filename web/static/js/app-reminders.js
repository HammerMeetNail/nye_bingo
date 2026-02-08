// Year of Bingo - Reminders Module (scaffold)
// SCAFFOLD: Not yet loaded in production. See plans/refactor.md for extraction status.

window.App = window.App || {};
var App = window.App;

if (!App._moduleRemindersLoaded) {
  App._moduleRemindersLoaded = true;

Object.assign(App, {
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
          <button class="btn btn-ghost btn-sm" data-action="delete-goal-reminder" data-reminder-id="${this.escapeHtml(reminder.id)}">Stop</button>
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
});
}
