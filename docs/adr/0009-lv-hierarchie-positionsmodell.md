# ADR 0009 — LV-Hierarchie (Los/Titel/Untertitel) im Positionsmodell

Datum: 2026-08-18
Status: entschieden (Subtask B.1.1)
Bezug: Backlog B.1 (Epic B — Angebots- & Auftragswesen), aufgabe.md §1
(Domäne B: "LV-Verwaltung, Positionen"). `docs/01-gap-analysis.md:32,121`
hatte die Lücke bereits dokumentiert: "`quote_items`/`quote_import_items`
vorhanden, aber flach — keine Los/Titel/Untertitel-Hierarchie". Wichtige
Abgrenzung zu **Backlog I.2.1** ("D81/D83/D86-Einlesen mit voller Hierarchie
(Los/Titel/Untertitel), Positionsart, Vorbemerkungen") — siehe Kontext.

## Kontext

`quote_items` (`server/internal/migrate/migrations/032_quotes.sql:22-33`,
später um `material_id`/`price_mapping_status` ergänzt) ist vollständig
flach: eine einzige `position int`-Spalte als reine Sortierreihenfolge,
keine Gruppierungs-/Elternspalte. Der Go-Typ `QuoteItemInput`
(`server/internal/quotes/service.go:22-35`) spiegelt das exakt — keine
Hierarchiefelder.

**Wichtiger Recherchebefund**: eine echte GAEB-Hierarchie-Extraktion
existiert im gesamten Repo NICHT — auch nicht teilweise. Der einzige
vorhandene Parser (`server/internal/quotes/gaeb_xml_subset_parser.go`) liest
ein bewusst minimales Nicht-Standard-Dialekt-Format und schließt
"LV-Hierarchie" laut eigener Strategiedokumentation
(`docs/gaeb_xml_subset_parser_strategy.md:45`) explizit aus. Selbst das
einzige hierarchienahe Feld, das er erfasst (`outline_no`), wird beim
tatsächlichen Übernehmen eines Imports in ein Angebot
(`ApplyImportToDraftQuote`, `server/internal/quotes/imports.go:606-632`)
NICHT mit übernommen.

**Scope-Abgrenzung zu Backlog I.2.1**: I.2.1 ("echter GAEB-DA-XML-3.x-Parser
... mit voller Hierarchie") ist explizit für den **Parser** zuständig, der
diese Struktur aus echten GAEB-Dateien EXTRAHIERT. B.1 ist davon
unabhängig — es baut das **Zielmodell** (Datenstruktur + CRUD), in das I.2.1
später hineinschreiben wird, UND ermöglicht schon jetzt die manuelle
Strukturierung eines Angebots durch einen Nutzer (unabhängig von jedem
GAEB-Import), analog zu aufgabe.md §1: "Alles davor wird so gebaut, dass I
ohne Umbau darauf aufsetzen kann, insbesondere: Positionsmodell". B.1 fasst
daher bewusst NICHT den GAEB-Parser selbst an (`gaeb_xml_subset_parser.go`,
`imports.go`) — das bleibt vollständig I.2s Aufgabe.

**Weitere bewusste Abgrenzung**: "Positionsart" (Normal-/Alternativ-/
Eventual-/Bedarfsposition) und "Vorbemerkungen" werden zusammen mit der
Hierarchie in Backlog I.2.1 genannt, stehen aber NICHT im Titel von B.1
("LV-Hierarchie (Los/Titel/Untertitel)") — beide bleiben bewusst außerhalb
des Scopes dieser ADR/dieses Tasks, um Scope-Creep zu vermeiden (aufgabe.md
§0, §7.5). Sie werden bei Bedarf als eigene Backlog-Positionen behandelt,
spätestens wenn I.2.1 ansteht.

## Optionen

**Option A — Hierarchie direkt in `quote_items` selbst** (`parent_id`
Selbstreferenz + `kind`-Spalte, die zwischen Gruppenknoten
(los/titel/untertitel) und echten Positionen unterscheidet, eine
Baumstruktur in einer Tabelle).
Erwogen, aber verworfen: vermischt zwei fachlich unterschiedliche Dinge in
einer Tabelle — bepreiste Positionen (mit `qty`/`unit_price`/`tax_code`/
`net_amount` etc.) und rein strukturelle Überschriften (Los/Titel/
Untertitel, die selbst nie bepreist werden). Für Gruppenknoten blieben alle
Preisspalten bedeutungslos/NULL — unsauberes Modell, widerspricht dem im
gesamten Repo etablierten Kopf/Positionen-Trennungsprinzip (z. B.
`price_lists`/`price_list_items` aus ADR 0007, `quotes`/`quote_items`
selbst).

**Option B — separate Tabelle `quote_item_groups`** (eigener Baum für
Los/Titel/Untertitel-Knoten, `quote_items` bekommt nur eine zusätzliche,
NULLABLE `group_id`-Fremdschlüsselspalte). Gewählt.

## Entscheidung

**Option B.**

- **`quote_item_groups`**: `id` (UUID, konsistent mit `quotes`/`quote_items`
  — anders als die `text`-IDs im `materials`/`contacts`-Paket, da diese neue
  Tabelle Teil der `quotes`-Domäne ist und deren ID-Konvention folgt),
  `quote_id` (FK → `quotes(id) ON DELETE CASCADE`), `parent_group_id`
  (FK → `quote_item_groups(id) ON DELETE CASCADE`, NULLABLE — oberste
  Ebene hat keinen Elternknoten), `kind` (text, `CHECK (kind IN ('los',
  'titel', 'untertitel'))` — bewusst genau diese drei, laut aufgabe.md §1/§4
  explizit benannten Ebenen, keine beliebige Tiefe, um nicht über die
  gestellte Anforderung hinauszubauen), `bezeichnung` (text NOT NULL, die
  Überschrift des Knotens), `sort_order` (int NOT NULL DEFAULT 0,
  Geschwisterreihenfolge), `created_at`.
- **`quote_items.group_id`**: neue NULLABLE Spalte, `REFERENCES
  quote_item_groups(id) ON DELETE SET NULL` (bewusst SET NULL statt CASCADE
  — das Löschen eines Gruppenknotens darf NIE bepreiste Positionen
  mitlöschen, nur deren Gruppierung aufheben, aufgabe.md §7.8-Geist:
  Datenverlust vermeiden). `group_id IS NULL` bedeutet "ungruppierte
  Position" — der bestehende, komplett flache Ist-Zustand bleibt für JEDES
  bereits existierende Angebot unverändert gültig, keine Datenmigration
  nötig (100% rückwärtskompatibel).
- **Kein eigenes `company_id`** auf `quote_item_groups` — Scope wird über
  `quote_id` → `quotes.company_id` geerbt (identisches Muster wie
  `quote_items` selbst, ADR 0002: "Item-/Kind-Tabellen ... erben Scope über
  FK zur Kopf-Tabelle").
- **Hierarchie-Wohlgeformtheit** (ein `titel` darf nur unter `los` oder
  oberster Ebene stehen, ein `untertitel` nur unter `titel`) wird bewusst
  NICHT per DB-Trigger erzwungen, sondern im Anwendungscode geprüft
  (Subtask B.1.3) — identisches Prinzip wie in ADR 0008 (Cross-Table-Regeln
  ohne Trigger sind in Postgres nur über CHECK-Constraints, die andere
  Zeilen lesen, nicht abbildbar; ein Trigger wäre hier unverhältnismäßig).
- **Anzeige-/Lesereihenfolge**: keine neue, vereinheitlichte
  Sortierspalte über Gruppen UND Positionen hinweg. Stattdessen baut die
  Anwendungsschicht (B.1.3) den Baum zur Lesezeit aus `quote_item_groups`
  (sortiert nach `sort_order` je Elternknoten) und den zugehörigen
  `quote_items` (weiterhin nach ihrer bestehenden `position`-Spalte
  sortiert) zusammen — kein Schemaeingriff in die bestehende, bereits an
  vielen Stellen verwendete `position`-Spalte nötig.
- Bewusst NICHT Teil dieser ADR: `gaeb_xml_subset_parser.go`/`imports.go`
  bleiben unverändert (siehe Kontext) — die Anbindung an einen echten
  GAEB-Parser ist Backlog I.2.

## Konsequenzen

- **B.1.2** (Folge-Subtask): additive Migration `071_...sql` (eine neue
  Tabelle + eine NULLABLE `ALTER TABLE quote_items ADD COLUMN group_id`,
  keine bestehende Zeile betroffen, reversibel via `DROP TABLE`/`DROP
  COLUMN`).
- **B.1.3** (Folge-Subtask): CRUD für `quote_item_groups` im `quotes`-Paket
  (Anlegen/Umbenennen/Löschen/Verschieben von Los/Titel/Untertitel-Knoten,
  inkl. Wohlgeformtheits-Validierung), `quote_items` optional einer Gruppe
  zuordnen (Erweiterung von `QuoteItemInput`/`Create`/`Update`), ein
  Baum-Assemblierungs-Helfer für die Leseseite, HTTP-Wiring, Tests. **Hohe
  Sorgfalt geboten**: `quotes/service.go` ist die größte und fragilste
  Datei der Session (bekannte, dokumentierte Vorbefunde wie Backlog 0.34
  "conn busy" bei `Revise`) — B.1.3 darf NUR additiv erweitern, keine
  bestehende Funktion umbauen, die nicht direkt Teil der neuen Funktionalität
  ist.
- Kein Einfluss auf bestehende Daten, Tabellen, Endpunkte oder den
  GAEB-Import-Pfad.
