CREATE TABLE IF NOT EXISTS company_branding_settings (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL DEFAULT '',
    claim TEXT NOT NULL DEFAULT '',
    primary_color TEXT NOT NULL DEFAULT '#1F4B99',
    accent_color TEXT NOT NULL DEFAULT '#6B7280',
    document_header_text TEXT NOT NULL DEFAULT '',
    document_footer_text TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO company_branding_settings (
    id, display_name, claim, primary_color, accent_color, document_header_text, document_footer_text
)
VALUES (
    'default', '', '', '#1F4B99', '#6B7280', '', ''
)
ON CONFLICT (id) DO NOTHING;
