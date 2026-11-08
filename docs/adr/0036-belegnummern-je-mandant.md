# ADR 0036 — Belegnummern je Mandant

Datum: 2026-11-08
Status: **Angenommen** (2026-11-07, ANNAHME) — "fortsetzen" nach den
Fragen 1–3 mit Empfehlung (`docs/open-questions.md`, Abschnitt
2026-11-07 H.5.5): 1a, 2a, 3 ja.
Bezug: Backlog H.5.5 (Befund aus H.5.2), H.5.5a (Migration 111),
H.5.5b (Ersatzmuster), H.5.6 (Material im Logikal-Import je Mandant).

## Kontext

- Nummernkreise sind seit Migration 060/061 je Mandant
  (`number_sequences`, Schlüssel Mandant + Entität).
- Die Nummern selbst waren über alle Mandanten eindeutig:
  `invoices_out.nummer`, `purchase_orders.number`, `rfqs.nummer`,
  `sales_orders.nummer`, dazu `materials.nummer`. Zwei Mandanten mit
  gleichem Muster erzeugten dieselbe Nummer, der zweite Beleg scheiterte
  mit 500. Eine fremde Materialnummer ließ sich durch Ablehnung erraten.
- `NumberingService.Next` legte für einen Mandanten ohne Nummernkreis
  immer "PO-{YYYY}-{NNNN}" an, auch für Rechnung, Angebot, Auftrag und
  Anfrage. Das Onboarding legt keine Nummernkreise an.

## Entscheidung

1. **Eindeutig je Mandant** (Migration 111, H.5.5a): eindeutige Indizes
   auf (`COALESCE(company_id,'')`, Nummer), bei `rfqs` (company_id
   NOT NULL) auf (company_id, nummer). Belege ohne Mandant bilden einen
   eigenen Bereich. Rechnungsentwürfe ohne Nummer bleiben mehrfach
   möglich. `production_orders` war schon je Mandant (Migration 107),
   `quotes` und `projects` haben keinen eindeutigen Index.
2. **Ersatzmuster aus Mandant `default`** (H.5.5b): fehlt einem Mandanten
   der Nummernkreis, legt `Next` ihn mit dem Muster des Mandanten
   `default` für dieselbe Entität an (z. B. "RE-{YYYY}-{NNNN}" für
   Rechnungen), beginnend bei 1; nur ohne solches Muster weiter
   "PO-{YYYY}-{NNNN}". Der Zähler von `default` wird nicht übernommen
   und zählt nicht mit. Vorhandene Nummernkreise bleiben unverändert;
   der Mandant ändert sein Muster wie bisher in den Einstellungen.
3. **Materialnummern ebenfalls je Mandant** (in Migration 111);
   Voraussetzung H.5.6: Material wird beim Verknüpfen nur im Mandanten
   gesucht.

## Folgen

- Gleiche Nummern in verschiedenen Mandanten sind gewollt. Wer Belege
  über Mandanten hinweg zusammenführt (Export, Auswertung), muss den
  Mandanten mitführen.
- Fortlaufende Rechnungsnummern sind je Aussteller eindeutig (UStG § 14),
  nicht über Firmen hinweg.
- Ändert `default` sein Muster, wirkt das nur für Nummernkreise, die
  danach neu entstehen.
- Rückweg von Migration 111 siehe DOWN-Kommentar dort; er scheitert,
  sobald zwei Mandanten dieselbe Nummer haben. Rechnungsnummern gebuchter
  Rechnungen dürfen dann nicht umbenannt werden (GoBD).

## Verworfen

- 1b eigenes Präfix je Mandant erzwingen (Nummern ändern ihr Aussehen),
  1c global lassen (Fehler bleibt).
- 2b alle Nummernkreise beim Onboarding anlegen (deckt schon angelegte
  Mandanten nicht ab, zweite Stelle), 2c so lassen.
