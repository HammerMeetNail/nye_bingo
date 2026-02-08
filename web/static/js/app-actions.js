// Year of Bingo - App Action Delegation Module (scaffold)

window.App = window.App || {};
var App = window.App;

Object.assign(App, {
  setupActionDelegation() {
    document.addEventListener('click', (event) => {
      const stopEl = event.target.closest ? event.target.closest('[data-stop-propagation]') : null;
      if (stopEl) event.stopPropagation();

      const actionEl = event.target.closest ? event.target.closest('[data-action]') : null;
      if (actionEl) {
        if (actionEl.classList.contains('dropdown-item--disabled')) return;
        const ariaDisabled = actionEl.getAttribute('aria-disabled');
        const ariaDisabledProp = actionEl.ariaDisabled;
        if (actionEl.disabled || ariaDisabled === 'true' || ariaDisabledProp === 'true') return;
        const action = actionEl.dataset.action;
        if (action) {
          this.handleActionClick(action, actionEl, event);
        }
      }
      if (event.defaultPrevented) return;
      this.handleNavClick(event);
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
      case 'export-account':
        this.exportAccountData(target);
        break;
      case 'open-delete-account-modal':
        this.openDeleteAccountModal();
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
        this.navigate(`/check-email?type=verification&email=${encodeURIComponent(this.user?.email || '')}`, { skipWarning: true });
        break;
      case 'open-upgrade-modal':
        this.openUpgradeModal();
        break;
      case 'select-upgrade-premium':
        this.selectUpgradePremium(target);
        break;
      case 'select-upgrade-tip':
        this.selectUpgradeTip(target);
        break;
      case 'billing-checkout-selected':
        this.startSelectedCheckout(target);
        break;
      case 'open-premium-code-modal':
        this.openPremiumCodeModal();
        break;
      case 'show-create-template-modal':
        this.showCreateTemplateModal();
        break;
      case 'view-template':
        if (target.dataset.templateId) this.showTemplateModal(target.dataset.templateId);
        break;
      case 'edit-template':
        if (target.dataset.templateId) this.showEditTemplateModal(target.dataset.templateId);
        break;
      case 'delete-template':
        if (target.dataset.templateId) this.deleteTemplate(target.dataset.templateId);
        break;
      case 'use-template':
        if (target.dataset.templateId) this.showCreateCardFromTemplateModal(target.dataset.templateId);
        break;
      case 'save-template-from-card': {
        const cardId = target.dataset.cardId || this.currentCard?.id;
        if (cardId) this.showCreateTemplateFromCardModal(cardId);
        break;
      }
      case 'show-rollover-card-modal': {
        const cardId = target.dataset.cardId || this.currentCard?.id;
        if (cardId) this.showRolloverCardModal(cardId);
        break;
      }
      case 'set-post-auth-next':
        this.storePostAuthNextPath(target?.dataset?.next || '');
        break;
      case 'open-billing-portal':
        this.openBillingPortal();
        break;
      case 'billing-checkout-subscription':
        this.startSubscriptionCheckout(target);
        break;
      case 'billing-checkout-lifetime':
        this.startLifetimeCheckout();
        break;
      case 'billing-checkout-tip':
        this.startTipCheckout(target);
        break;
      case 'billing-redeem-code':
        this.redeemPremiumCode(target);
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
      case 'show-edit-finalized-card-modal':
        this.showEditFinalizedCardModal();
        break;
      case 'open-share-modal':
        this.showShareCardModal();
        break;
      case 'enable-share':
        this.enableShare();
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
      case 'ai-premium-assist': {
        const position = parseInt(target.dataset.position, 10);
        if (!Number.isNaN(position)) this.handleAIPremiumAssist(position);
        break;
      }
      case 'ai-fill-empty-premium':
        this.fillEmptyWithAI();
        break;
      case 'ai-regenerate-goal': {
        const index = parseInt(target.dataset.index, 10);
        if (!Number.isNaN(index)) AIWizard.regenerateGoal(index, target);
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
        if (!Number.isNaN(position)) this.saveItemEdit(event, position, form);
        break;
      }
      case 'clone-card':
        this.handleCloneCard(event);
        break;
      case 'edit-finalized-card':
        this.handleEditFinalizedCard(event);
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
      case 'create-template':
        this.handleCreateTemplate(event, form);
        break;
      case 'create-template-from-card':
        this.handleCreateTemplateFromCard(event, form);
        break;
      case 'update-template':
        this.handleUpdateTemplate(event, form);
        break;
      case 'create-card-from-template':
        this.handleCreateCardFromTemplate(event, form);
        break;
      case 'rollover-card':
        this.handleRolloverCard(event, form);
        break;
      case 'ai-generate':
        AIWizard.handleGenerate(event);
        break;
      case 'delete-account':
        this.handleDeleteAccount(event, form);
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
});
