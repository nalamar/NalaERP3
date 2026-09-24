-- Steuer- und Summenangaben eingehender E-Rechnungen (Backlog E.8.1,
-- Folge aus E.6/ADR 0023).
--
-- Ausgangslage: invoices_in trug bisher UEBERHAUPT KEINE Summenfelder und
-- invoice_in_items weder Steuerkennzeichen noch Mengeneinheit. Die beim
-- Eingang geparsten Angaben (Steueraufschluesselung, ausgewiesene Summen,
-- Faelligkeit) wurden deshalb in E.6 als Klartext in invoices_in.note
-- gerettet - eine dokumentierte Notloesung: so sind sie weder auswertbar
-- noch fuer eine Rechnungspruefung nutzbar.
--
-- WICHTIG zur Semantik: gespeichert wird, was der LIEFERANT ausweist.
-- Es wird nichts nachgerechnet und nichts korrigiert (ADR 0023) - die
-- Rechnung gehoert dem Absender. Deshalb bekommen die Summen eigene
-- Spalten, statt sie aus den Positionen abzuleiten: eine abweichende
-- Summe ist eine Tatsache der Rechnung und darf nicht wegrationalisiert
-- werden.

-- 1) Kopfsummen und Faelligkeit.
--
-- Alle drei Betragsspalten NOT NULL DEFAULT 0: bestehende, manuell
-- erfasste Eingangsrechnungen haben diese Angaben schlicht nicht, und 0
-- ist dort die ehrliche Aussage "nicht erfasst" - ein NULL waere
-- gleichbedeutend, wuerde aber jede Auswertung mit COALESCE belasten.
-- due_date bleibt nullable: eine Faelligkeit KANN fehlen, und ein
-- Default waere ein erfundenes Datum.
ALTER TABLE invoices_in
    ADD COLUMN IF NOT EXISTS net_amount numeric(18,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tax_amount numeric(18,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gross_amount numeric(18,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS due_date date;

COMMENT ON COLUMN invoices_in.net_amount IS
    'Vom Lieferanten ausgewiesener Nettobetrag. Wird uebernommen, nicht nachgerechnet (ADR 0023).';
COMMENT ON COLUMN invoices_in.gross_amount IS
    'Vom Lieferanten ausgewiesener Bruttobetrag. Kann von net_amount+tax_amount abweichen - das ist dann eine Tatsache der Rechnung, kein Fehler.';

-- 2) Steuerangaben und Mengeneinheit je Position.
--
-- tax_category ist der UNTDID-5305-Code des LIEFERANTEN (S/AE/E/...),
-- NICHT unser internes tax_codes-Kennzeichen. Bewusst KEIN Fremdschluessel
-- auf tax_codes: unsere Stammdaten bilden unsere eigene Steuerlogik ab,
-- die Kategorie einer fremden Rechnung ist eine Angabe des Absenders. Sie
-- auf einen internen Code zu mappen waere ein Rateschritt - und ein
-- Fremdschluessel wuerde jede Rechnung mit einer bei uns nicht gepflegten
-- Kategorie unannehmbar machen.
--
-- Ebenso KEIN CHECK auf eine feste Werteliste: UNTDID 5305 kennt mehr
-- Kategorien als die drei, die unser Ausgang schreibt (ADR 0022), und
-- eine eingehende Rechnung darf nicht daran scheitern, dass wir eine
-- zulaessige Kategorie nicht vorgesehen haben.
--
-- unit_code ist der UN/ECE-Rec.-20-Code (z. B. MTR, HUR, C62). Leer
-- erlaubt: nicht jede Rechnung nennt eine Einheit.
ALTER TABLE invoice_in_items
    ADD COLUMN IF NOT EXISTS tax_category text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS tax_rate numeric(6,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS unit_code text NOT NULL DEFAULT '';

COMMENT ON COLUMN invoice_in_items.tax_category IS
    'UNTDID-5305-Steuerkategorie des Lieferanten (S/AE/E/...), NICHT unser internes tax_codes-Kennzeichen.';
COMMENT ON COLUMN invoice_in_items.tax_rate IS
    'Steuersatz in PROZENT (19.00), nicht als Bruchteil - anders als tax_codes.rate.';

-- 3) Steueraufschluesselung auf Belegebene (EN 16931 BG-23).
--
-- Eigene Tabelle statt weiterer Spalten auf invoices_in: eine Rechnung
-- kann mehrere Steuersatz-Gruppen haben (z. B. 19 % und 7 %), das ist
-- eine 1:n-Beziehung. Die Gruppe traegt die vom Lieferanten
-- ausgewiesenen Betraege; sie ist NICHT aus den Positionen abgeleitet,
-- sondern uebernommen.
CREATE TABLE IF NOT EXISTS invoice_in_taxes (
    id text PRIMARY KEY,
    invoice_in_id text NOT NULL REFERENCES invoices_in(id) ON DELETE CASCADE,
    tax_category text NOT NULL DEFAULT '',
    tax_rate numeric(6,2) NOT NULL DEFAULT 0,
    basis_amount numeric(18,4) NOT NULL DEFAULT 0,
    calculated_amount numeric(18,4) NOT NULL DEFAULT 0,
    exemption_reason text NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_invoice_in_taxes_invoice ON invoice_in_taxes(invoice_in_id);

COMMENT ON TABLE invoice_in_taxes IS
    'Steueraufschluesselung einer Eingangsrechnung je Kategorie/Satz (EN 16931 BG-23), uebernommen vom Lieferanten.';

-- Keine neue Permission: die Felder gehoeren zu Eingangsrechnungen und
-- sind mit invoices_in.read/write abgedeckt.

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   DROP TABLE IF EXISTS invoice_in_taxes;
--   ALTER TABLE invoice_in_items
--       DROP COLUMN IF EXISTS tax_category,
--       DROP COLUMN IF EXISTS tax_rate,
--       DROP COLUMN IF EXISTS unit_code;
--   ALTER TABLE invoices_in
--       DROP COLUMN IF EXISTS net_amount,
--       DROP COLUMN IF EXISTS tax_amount,
--       DROP COLUMN IF EXISTS gross_amount,
--       DROP COLUMN IF EXISTS due_date;
-- DATENVERLUSTRISIKO: Sobald Eingangsrechnungen mit Steuer- und
-- Summenangaben erfasst wurden, gehen diese bei einem Downgrade
-- unwiderruflich verloren - inklusive der kompletten
-- Steueraufschluesselung (die Tabelle wird gedroppt). Die
-- Eingangsrechnungen selbst, ihre Positionen, Lieferanten- und
-- Bestellbezuege bleiben unberuehrt: invoice_in_taxes wird von keiner
-- anderen Tabelle referenziert, und die neuen Spalten werden von keinem
-- Constraint ausserhalb ihrer eigenen Tabelle verwendet.
