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
    const data = {
      year: card.year,
      title: card.title || null,
      category: card.category || null,
      items: card.items || [],
      createdAt: card.createdAt || now,
      updatedAt: now,
    };
    localStorage.setItem(this.STORAGE_KEY, JSON.stringify(data));
    return data;
  },

  // Create a new anonymous card (only if one doesn't exist)
  create(year, title = null, category = null) {
    if (this.exists()) {
      return this.get();
    }
    const card = {
      year,
      title,
      category,
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

  // Add an item to the anonymous card
  addItem(text) {
    const card = this.get();
    if (!card) return null;

    // Find next available position (0-24, excluding 12 which is FREE space)
    const usedPositions = new Set(card.items.map(i => i.position));
    let position = null;
    for (let i = 0; i <= 24; i++) {
      if (i !== 12 && !usedPositions.has(i)) {
        position = i;
        break;
      }
    }

    if (position === null) {
      return null; // Card is full
    }

    const item = {
      position,
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

    // Get all available positions (0-24 excluding 12)
    const availablePositions = [];
    for (let i = 0; i <= 24; i++) {
      if (i !== 12) availablePositions.push(i);
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
    return this.getItemCount() === 24;
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
      items: card.items.map(item => ({
        position: item.position,
        content: item.text,
      })),
      finalize: true,
    };
  },
};
