-- Premium Templates

CREATE TABLE card_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    category VARCHAR(50),

    grid_size SMALLINT NOT NULL DEFAULT 5,
    header_text VARCHAR(5) NOT NULL DEFAULT 'BINGO',
    has_free_space BOOLEAN NOT NULL DEFAULT true,

    default_visible_to_friends BOOLEAN NOT NULL DEFAULT true,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT card_templates_valid_grid_size CHECK (grid_size IN (2,3,4,5)),
    CONSTRAINT card_templates_header_len CHECK (char_length(header_text) >= 1 AND char_length(header_text) <= grid_size)
);

CREATE INDEX idx_card_templates_user_id ON card_templates(user_id);

CREATE TABLE card_template_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL REFERENCES card_templates(id) ON DELETE CASCADE,
    sort_order INT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(template_id, sort_order)
);

CREATE INDEX idx_card_template_items_template_id ON card_template_items(template_id);

CREATE TRIGGER update_card_templates_updated_at
  BEFORE UPDATE ON card_templates
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();

