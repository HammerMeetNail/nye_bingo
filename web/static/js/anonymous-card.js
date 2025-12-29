// Year of Bingo - Anonymous Card LocalStorage Management
// Allows users to create and edit a card without logging in

const AnonymousCard = {
  STORAGE_KEY: 'yearofbingo_anonymous_card',

  // Check if an anonymous card exists in localStorage
  exists() {
    return localStorage.getItem(this.STORAGE_KEY) !== null;
  },

  // Get the anonymous card from localStorage
  get() {
    const data = localStorage.getItem(this.STORAGE_KEY);
    if (!data) return null;
    try {
      return JSON.parse(data);
    } catch (e) {
      console.error('Failed to parse anonymous card:', e);
      return null;
    }
  },

  // Save/create anonymous card to localStorage
  save(card) {
    const now = new Date().toISOString();
    const size = card.grid_size || 5;
    const totalSquares = size * size;
    const hasFree = typeof card.has_free_space === 'boolean' ? card.has_free_space : true;
    const freePos = hasFree
      ? (typeof card.free_space_position === 'number'
        ? card.free_space_position
        : (size % 2 === 1 ? Math.floor(totalSquares / 2) : Math.floor(Math.random() * totalSquares)))
      : null;
    const data = {
      year: card.year,
      title: card.title || null,
      category: card.category || null,
      grid_size: size,
      header_text: card.header_text || 'BINGO',
      has_free_space: hasFree,
      free_space_position: freePos,
      items: card.items || [],
      createdAt: card.createdAt || now,
      updatedAt: now,
    };
    localStorage.setItem(this.STORAGE_KEY, JSON.stringify(data));
    return data;
  },

  // Create a new anonymous card (only if one doesn't exist)
  create(year, title = null, category = null, gridSize = 5, headerText = null, hasFreeSpace = true) {
    if (this.exists()) {
      return this.get();
    }
    const size = Number.isFinite(gridSize) ? gridSize : 5;
    const header = (headerText || 'BINGO').toString().trim().toUpperCase();
    const totalSquares = size * size;
    const freePos = hasFreeSpace
      ? (size % 2 === 1 ? Math.floor(totalSquares / 2) : Math.floor(Math.random() * totalSquares))
      : null;
    const card = {
      year,
      title,
      category,
      grid_size: size,
      header_text: header.slice(0, size),
      has_free_space: hasFreeSpace,
      free_space_position: freePos,
      items: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    localStorage.setItem(this.STORAGE_KEY, JSON.stringify(card));
    return card;
  },

  // Clear the anonymous card
  clear() {
    localStorage.removeItem(this.STORAGE_KEY);
  },

  // Clear all items on the anonymous card
  clearItems() {
    const card = this.get();
    if (!card) return false;
    card.items = [];
    this.save(card);
    return true;
  },

  // Add an item to the anonymous card
  addItem(text, position = null) {
    const card = this.get();
    if (!card) return null;

    const size = card.grid_size || 5;
    const totalSquares = size * size;
    const hasFree = typeof card.has_free_space === 'boolean' ? card.has_free_space : true;
    const freePos = hasFree ? card.free_space_position : null;

    const usedPositions = new Set(card.items.map(i => i.position));
    let targetPosition = position;

    if (targetPosition !== null) {
      if (targetPosition < 0 || targetPosition >= totalSquares) return null;
      if (freePos !== null && targetPosition === freePos) return null;
      if (usedPositions.has(targetPosition)) return null;
    } else {
      // Find next available position (0..N^2-1, excluding FREE if enabled)
      for (let i = 0; i < totalSquares; i++) {
        if (freePos !== null && i === freePos) continue;
        if (!usedPositions.has(i)) {
          targetPosition = i;
          break;
        }
      }
    }

    if (targetPosition === null) {
      return null; // Card is full
    }

    const item = {
      position: targetPosition,
      text,
      notes: '',
    };
    card.items.push(item);
    this.save(card);
    return item;
  },

  // Remove an item by position
  removeItem(position) {
    const card = this.get();
    if (!card) return false;

    const index = card.items.findIndex(i => i.position === position);
    if (index === -1) return false;

    card.items.splice(index, 1);
    this.save(card);
    return true;
  },

  // Update an item's text
  updateItem(position, text) {
    const card = this.get();
    if (!card) return false;

    const item = card.items.find(i => i.position === position);
    if (!item) return false;

    item.text = text;
    this.save(card);
    return true;
  },

  // Shuffle items (randomize positions)
  shuffle() {
    const card = this.get();
    if (!card || card.items.length === 0) return card;

    const size = card.grid_size || 5;
    const totalSquares = size * size;
    const hasFree = typeof card.has_free_space === 'boolean' ? card.has_free_space : true;
    const freePos = hasFree ? card.free_space_position : null;

    // Get all available positions excluding FREE (if enabled)
    const availablePositions = [];
    for (let i = 0; i < totalSquares; i++) {
      if (freePos !== null && i === freePos) continue;
      availablePositions.push(i);
    }

    // Fisher-Yates shuffle the positions
    for (let i = availablePositions.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [availablePositions[i], availablePositions[j]] = [availablePositions[j], availablePositions[i]];
    }

    // Assign new positions to items
    card.items.forEach((item, index) => {
      item.position = availablePositions[index];
    });

    this.save(card);
    return card;
  },

  // Swap two items' positions
  swapItems(pos1, pos2) {
    const card = this.get();
    if (!card) return false;

    const hasFree = typeof card.has_free_space === 'boolean' ? card.has_free_space : true;
    const freePos = hasFree ? card.free_space_position : null;

    if (freePos !== null && (pos1 === freePos || pos2 === freePos)) {
      const oldFree = freePos;
      const newFree = pos1 === oldFree ? pos2 : pos1;
      if (newFree === oldFree) return true;

      const displaced = card.items.find(i => i.position === newFree) || null;
      if (displaced) {
        displaced.position = oldFree;
      }
      card.free_space_position = newFree;
      this.save(card);
      return true;
    }

    const item1 = card.items.find(i => i.position === pos1);
    const item2 = card.items.find(i => i.position === pos2);

    if (item1 && item2) {
      // Both positions have items - swap them
      item1.position = pos2;
      item2.position = pos1;
    } else if (item1 && !item2) {
      // Only first position has item - move it to second
      item1.position = pos2;
    } else if (!item1 && item2) {
      // Only second position has item - move it to first
      item2.position = pos1;
    } else {
      // Neither position has item - nothing to do
      return false;
    }

    this.save(card);
    return true;
  },

  // Get count of items
  getItemCount() {
    const card = this.get();
    return card ? card.items.length : 0;
  },

  // Check if card has all 24 items (ready to finalize)
  isReady() {
    const card = this.get();
    if (!card) return false;
    const size = card.grid_size || 5;
    const totalSquares = size * size;
    const hasFree = typeof card.has_free_space === 'boolean' ? card.has_free_space : true;
    const capacity = hasFree ? totalSquares - 1 : totalSquares;
    return this.getItemCount() === capacity;
  },

  // Get item at a specific position
  getItemAt(position) {
    const card = this.get();
    if (!card) return null;
    return card.items.find(i => i.position === position) || null;
  },

  // Update card metadata (title, category)
  updateMeta(title, category) {
    const card = this.get();
    if (!card) return null;

    card.title = title || null;
    card.category = category || null;
    this.save(card);
    return card;
  },

  // Convert anonymous card format to API format for import
  toAPIFormat() {
    const card = this.get();
    if (!card) return null;

    return {
      year: card.year,
      title: card.title,
      category: card.category,
      grid_size: card.grid_size || 5,
      header_text: card.header_text || 'BINGO',
      has_free_space: typeof card.has_free_space === 'boolean' ? card.has_free_space : true,
      free_space_position: typeof card.free_space_position === 'number' ? card.free_space_position : null,
      items: card.items.map(item => ({
        position: item.position,
        content: item.text,
      })),
      finalize: true,
    };
  },

  updateConfig({ headerText = null, hasFreeSpace = null } = {}) {
    const card = this.get();
    if (!card) return null;

    const size = card.grid_size || 5;
    const totalSquares = size * size;

    if (headerText !== null) {
      const normalized = headerText.toString().trim().toUpperCase();
      if (!normalized || normalized.length > size) return null;
      card.header_text = normalized;
    }

    if (typeof hasFreeSpace === 'boolean' && hasFreeSpace !== card.has_free_space) {
      if (hasFreeSpace) {
        const used = new Set(card.items.map(i => i.position));
        const empties = [];
        for (let p = 0; p < totalSquares; p++) {
          if (!used.has(p)) empties.push(p);
        }
        if (empties.length === 0) return null;

        let desired;
        if (size % 2 === 1) {
          desired = Math.floor(totalSquares / 2);
        } else {
          desired = empties[Math.floor(Math.random() * empties.length)];
        }

        if (used.has(desired)) {
          const otherEmpties = empties.filter(p => p !== desired);
          if (otherEmpties.length === 0) return null;
          const relocateTo = otherEmpties[Math.floor(Math.random() * otherEmpties.length)];
          const displaced = card.items.find(i => i.position === desired);
          if (displaced) displaced.position = relocateTo;
        }

        card.has_free_space = true;
        card.free_space_position = desired;
      } else {
        card.has_free_space = false;
        card.free_space_position = null;
      }
    }

    this.save(card);
    return card;
  },
};
