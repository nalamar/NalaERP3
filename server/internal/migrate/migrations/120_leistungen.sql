-- Leistungskatalog je Mandant (ADR 0044 Frage 1, ADR 0039 Frage 3; Backlog
-- I.4.4a): Ziel des Matchings von LV-Positionen. Eine Leistung hat einen
-- Code (je Mandant eindeutig), Kurz- und Langtext, Einheit, Gewerk und die
-- Systemvorgaben aus aufgabe.md §4. Preise stehen hier nicht: der
-- Kalkulationsansatz (Material, Lohn, Fremdleistung) kommt mit I.5/I.6.
-- Statt Loeschen wird eine Leistung abgeschaltet (aktiv = false), weil
-- Zuordnungen (I.5) auf sie verweisen werden.
CREATE TABLE IF NOT EXISTS leistungen (
    id text PRIMARY KEY,
    company_id text NOT NULL REFERENCES company_profiles(id),
    code text NOT NULL,
    kurztext text NOT NULL,
    langtext text NOT NULL DEFAULT '',
    einheit text NOT NULL,
    gewerk text NOT NULL DEFAULT '',
    profilserie text NOT NULL DEFAULT '',
    verglasung text NOT NULL DEFAULT '',
    rc_klasse text NOT NULL DEFAULT '',
    u_wert numeric(6,3),
    brandschutzklasse text NOT NULL DEFAULT '',
    aktiv boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by_user_id text REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT leistungen_code_unique UNIQUE (company_id, code),
    -- Code ohne Leerraum, hoechstens 50 Zeichen
    CONSTRAINT leistungen_code_check CHECK (code ~ '^[^[:space:]]{1,50}$'),
    CONSTRAINT leistungen_kurztext_check CHECK (btrim(kurztext) <> '' AND char_length(kurztext) <= 500),
    CONSTRAINT leistungen_langtext_check CHECK (char_length(langtext) <= 20000),
    CONSTRAINT leistungen_einheit_check CHECK (btrim(einheit) <> '' AND char_length(einheit) <= 20),
    CONSTRAINT leistungen_merkmale_check CHECK (
        char_length(gewerk) <= 100 AND char_length(profilserie) <= 200 AND char_length(verglasung) <= 200
        AND char_length(brandschutzklasse) <= 200),
    CONSTRAINT leistungen_rc_klasse_check CHECK (rc_klasse IN ('', 'RC1', 'RC2', 'RC3', 'RC4', 'RC5', 'RC6')),
    CONSTRAINT leistungen_u_wert_check CHECK (u_wert IS NULL OR (u_wert > 0 AND u_wert <= 10))
);

CREATE INDEX IF NOT EXISTS leistungen_company_aktiv_idx ON leistungen (company_id, aktiv, code);

COMMENT ON TABLE leistungen IS
    'Leistungskatalog je Mandant (ADR 0044): Ziel des Matchings, ohne Preise.';

-- DOWN (manuell auszufuehren; der Migrationsrunner kennt keine
-- automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS leistungen;
-- Datenverlustrisiko: mittel - der gepflegte Leistungskatalog geht verloren
-- (vorher als JSON exportieren bzw. die Importdatei aufbewahren). Andere
-- Tabellen verweisen in diesem Stand noch nicht auf leistungen.
