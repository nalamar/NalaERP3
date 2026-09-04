# ADR 0008 — Systemlieferanten-Konzept (Bindung Lieferant↔Profilserie)

Datum: 2026-08-18
Status: entschieden (Subtask A.3.1)
Bezug: Backlog A.3 (Epic A — Stammdaten). ADR 0006 (Materials-Profilattribute,
`profilserie` bewusst als freier Text: "die Zuordnung Lieferant↔Profilserie
ist ausdrücklich Gegenstand von Backlog A.3") und ADR 0007 (Preislisten,
`price_lists.lieferant` ebenfalls bewusst freier Text mit demselben
Verweis) — diese ADR baut die dort angekündigte Struktur.

## Kontext

Repo-Recherche: es existiert **keine** separate "Lieferanten"-Tabelle.
`contacts` (`server/internal/migrate/migrations/003_contacts.sql:3-15`) hat
bereits eine `rolle`-Spalte mit den Werten `customer | supplier | partner |
both | other` (`server/internal/contacts/service.go:30`,
`Roles()`-Funktion) — ein Lieferant ist im bestehenden Datenmodell also
schlicht ein `contacts`-Datensatz mit `rolle IN ('supplier','both')`.
Bestätigt durch `purchase_orders.supplier_id text NOT NULL REFERENCES
contacts(id) ON DELETE RESTRICT`
(`server/internal/migrate/migrations/004_purchase_orders.sql:5`) — Bestellungen
referenzieren bereits direkt `contacts`, keine eigene Lieferantentabelle.

**Ein "Systemlieferant"** ist im Metallbau ein Lieferant, der ein
bestimmtes Profilsystem (eine oder mehrere Profilserien, z. B. "Schüco AWS
75") exklusiv oder zertifiziert vertreibt. Diese Zuordnung
(Lieferant↔Profilserie) fehlt bisher komplett — weder `contacts` noch
`materials` noch `price_lists` haben eine strukturierte Verbindung
zwischen einem Lieferanten und den von ihm geführten Profilserien.

## Optionen

**Option A — neue, eigenständige `system_suppliers`-Tabelle** (Duplikat der
Lieferanten-Stammdaten, losgelöst von `contacts`).
Verworfen: `contacts` ist bereits die etablierte, produktiv genutzte
Lieferanten-Quelle (`purchase_orders.supplier_id`, `role IN
('supplier','both')`) — eine zweite, parallele Lieferantentabelle würde
Datenredundanz und Inkonsistenzrisiko schaffen (welcher Datensatz ist
"der" Lieferant X: der in `contacts` oder der in `system_suppliers`?) und
widerspricht dem in ADR 0002/0006/0007 durchgehend verfolgten Prinzip,
bestehende Kopf-Tabellen wiederzuverwenden statt zu duplizieren.

**Option B — Profilserie als eigene Katalog-Tabelle mit M:N-Bindung zu
`contacts`** (`profile_series` Stammdatentabelle + `contact_profile_series`
Junction-Tabelle).
Erwogen, aber verworfen für A.3: würde eine zentrale
Profilserien-Katalogpflege voraussetzen (wer legt "Schüco AWS 75" als
offiziellen Katalogeintrag an, bevor ein Lieferant ihn binden kann?) — das
ist zusätzliche Verwaltungslast ohne fachliche Notwendigkeit, da
`materials.profilserie` laut ADR 0006 bewusst freier Text bleibt (keine
Enum-/Katalog-Pflicht). Eine Katalogtabelle würde zwei parallele Quellen
für "gültige Profilserien" schaffen (Katalog vs. bereits in `materials`
verwendete freie Texte) und A.1 nachträglich einschränken, was Backlog A.1
explizit vermeiden wollte.

**Option C — Junction-Tabelle `supplier_profile_series` direkt zwischen
`contacts` und einem freien Profilserie-Textfeld** (keine eigene
Profilserie-Katalogtabelle; die Menge der tatsächlich gebundenen
Profilserien ergibt sich implizit aus den vorhandenen Bindungen). Gewählt.

## Entscheidung

**Option C.**

- **`supplier_profile_series`**: `id` (text), `contact_id` (FK →
  `contacts(id) ON DELETE CASCADE` — Bindung verliert ihre Bedeutung, wenn
  der Lieferanten-Kontakt gelöscht wird, harte statt weiche Kaskade,
  analog zu `contact_addresses`/`contact_persons`), `profilserie` (text NOT
  NULL, freier Text — bewusst KEINE FK auf eine Katalogtabelle, siehe
  Option B), `notiz` (text DEFAULT '', optionaler Freitext z. B. für
  Zertifizierungsstatus/Gebietsbeschränkung), `created_at`. `UNIQUE
  (contact_id, profilserie)` verhindert doppelte Bindungen.
- **Kein eigenes `company_id`** auf `supplier_profile_series` — Scope wird
  über `contact_id` → `contacts.company_id` geerbt (identisches Muster wie
  `contact_addresses`/`contact_persons`, ADR 0002).
- **Fachliche Regel**: eine Bindung darf nur zu einem Kontakt mit `rolle IN
  ('supplier','both')` angelegt werden — im Anwendungscode geprüft
  (Subtask A.3.3), nicht per DB-CHECK (eine CHECK-Bedingung über eine
  andere Tabelle ist in Postgres ohne Trigger nicht abbildbar; ein Trigger
  wäre hier unverhältnismäßig, die Anwendungsschicht prüft das bereits bei
  jedem anderen kontaktbezogenen Fall wie `rolle`-Validierung selbst).
- **Ort im Code**: neue Datei im bestehenden `server/internal/contacts`-Paket
  (nicht `materials`) — die Bindung ist konzeptionell eine Erweiterung des
  Lieferanten-Kontakts, nicht des Materialstamms; folgt damit demselben
  Paket-Zuordnungsprinzip wie A.2 (Preislisten im `materials`-Paket, weil
  materialpreis-nah) und A.1 (Profilattribute im `materials`-Paket, weil
  materialstamm-nah).
- **Bewusst NICHT Teil von A.3**: `materials.profilserie` und
  `price_lists.lieferant` bleiben freier Text — sie werden NICHT
  rückwirkend auf eine harte FK gegen `supplier_profile_series`/`contacts`
  umgestellt. Das wäre ein Breaking Change für bereits erfasste Freitext-
  Werte (Migrationsrisiko, uneindeutige Zuordnung bei Tippfehlern/
  Schreibvarianten) und ist fachlich nicht zwingend Teil des
  "Konzepts" (Backlog-Titel: "Systemlieferanten-**Konzept**", nicht
  "-Migration"). Als mögliche spätere, eigenständige Backlog-Position
  vermerkt (Konsequenzen).

## Konsequenzen

- **A.3.2** (Folge-Subtask): additive Migration `070_...sql`
  (eine neue Tabelle, keine bestehende Tabelle geändert, reversibel via
  `DROP TABLE`).
- **A.3.3** (Folge-Subtask): CRUD im `contacts`-Paket (neue Datei
  `system_suppliers.go` o. ä.), inkl. Rollen-Validierung bei Bindungsanlage
  und einer Reverse-Lookup-Funktion ("welche Lieferanten führen Profilserie
  X" — die eigentliche Motivation hinter "Bindung", nicht nur die
  Vorwärtsrichtung "welche Serien führt Lieferant Y"), HTTP-Wiring
  (bestehende `contacts.read`/`contacts.write`-Berechtigungen, keine neue
  Permission-Infrastruktur, analog zu A.2.3), Tests.
- **Neue, separate, mögliche Backlog-Position** (nicht Teil von A.3, nur
  bei Bedarf später anzulegen): `materials.profilserie`/
  `price_lists.lieferant` optional gegen `supplier_profile_series`
  validieren oder per Autovervollständigung vorschlagen (Anwendungsschicht,
  keine Schemaänderung nötig, da beide Felder bereits als freier Text
  bestehen bleiben).
- Kein Einfluss auf bestehende Daten, Tabellen oder Endpunkte.
