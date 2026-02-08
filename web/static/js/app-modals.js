// Year of Bingo - App Modal Module (scaffold)

window.App = window.App || {};
var App = window.App;

Object.assign(App, {
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
});
