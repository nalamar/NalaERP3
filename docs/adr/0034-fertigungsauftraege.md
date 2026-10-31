# ADR 0034 — Produktionssteuerung: Fertigungsaufträge aus Projektpositionen

Datum: 2026-10-30
Status: **Angenommen** (2026-10-31) — der Nutzer hat alle sechs
Empfehlungen angenommen ("Alle deine Fragen mit deinen Empfehlungen
umsetzen"). Zu Frage 5 also: Ausführungsklasse optional, ohne Wirkung in
H.1; ob der Betrieb nach EN 1090 fertigt, bleibt offen.
Bezug: `aufgabe.md` §1 Domäne H; Backlog H.1–H.3; `docs/01-gap-analysis.md`
§H; ADR 0032 (Muster: Beleg in geschlossener Form, Storno).

## Kontext (Ist-Stand, belegt)

- **Keine Fertigungssteuerung.** `docs/01-gap-analysis.md:108`: keine
  Tabellen für Fertigungsaufträge, Stücklisten, Arbeitsgänge, Kapazitäten,
  BDE oder Kommissionierung. Im Repo gibt es keine Spur von
  `fertigung`/`production` außer der Rolle.
- **Projektstruktur aus Logikal** (`projects/import_logikal.go`):
  Projekt → Phase (`project_phases`, Logikal "ElevationsGroup", z. B.
  "Haus A") → **Position** (`project_elevations`: Nummer, Name, Menge,
  Breite/Höhe, `external_guid`) → **Variante** (`project_single_elevations`,
  Menge, `selected`) → Material je Variante: Profile mit Länge
  (`single_elevation_profiles`), Artikel (`single_elevation_articles`),
  Glas mit Maßen (`single_elevation_glass`). Mengen sind `REAL`, Preise
  Kalkulationsdaten.
- **Ein erneuter Import ersetzt das Material** einer Variante und löscht
  Varianten (`project_imports.materials_replaced_variants`,
  `deleted_variants`); alle Ebenen hängen mit `ON DELETE CASCADE` am
  Projekt. Die Projektdaten sind also **veränderlich** — auf sie kann ein
  Fertigungsauftrag nicht dauerhaft verweisen.
- **Lager** (`materials`): Bewegungen (`stock_movements` mit Charge
  `batches`), **Reservierungen je Projekt** (`stock_reservations`,
  `project_id` RESTRICT), **Reststücke** von Profilen (`profile_offcuts`,
  Länge, verbraucht mit genutzter Länge), Inventur.
- **Projekte**: `status` als Freitext (`'neu'`), Kostenstelle;
  Kundenaufträge (`sales_orders.project_id`, optional, SET NULL).
- **Personal**: Zeiterfassung mit Projektbezug (`hr_time_entries.project_id`),
  Teams, Qualifikationen und Stellen (F.2, F.11) — Grundlage für spätere
  Kapazität und Einsatzfähigkeit (H.2/H.3).
- **Rolle `role-production`** "Arbeitsvorbereitung und Fertigung"
  (`017_auth.sql:71`) mit `materials.read`, `projects.read/write`,
  `documents.read` — keine eigenen Fertigungsrechte.

## Recherche (Sekundärquellen, nicht am Normtext geprüft)

| Thema | Befund | Folge für das Modell |
|---|---|---|
| EN 1090 (tragende Bauteile aus Stahl/Aluminium) | Werkseigene Produktionskontrolle (WPK) ist Pflicht; ab **EXC2** sind die Prüfbescheinigungen der Konstruktionsmaterialien (Halbzeuge, Schweißzusätze, Schrauben) **auftragsbezogen rückverfolgbar** zu archivieren; in EXC1 keine Rückverfolgbarkeit gefordert. Ohne Vereinbarung gilt EXC2 (österreichische Quelle) | Ausführungsklasse am Fertigungsauftrag; Bezug Auftrag ↔ Charge/Zeugnis vorsehen (H.3 Kommissionierung), nicht schon in H.1 |
| Fenster, Türen, Fassaden | fallen meist nicht unter EN 1090, sondern unter die eigenen Produktnormen (z. B. EN 14351-1, EN 13830) — in den Quellen nur am Rand | Ausführungsklasse optional, nicht Pflicht |
| Logikal → ERP | Orgadata beschreibt die Übergabe von Auftrags- und Materialbedarfsdaten per SQLite (genau der vorhandene Import); Bearbeitungsdaten für Maschinen erzeugt Logikal selbst | Zuschnitt- und Maschinendaten **nicht** im ERP nachbauen; das ERP steuert Auftrag, Material und Termine |

Quellen: [TÜV Austria Checkliste WPK EN 1090](https://tr.tuv.at/fileadmin/user_upload/docs/industry/EN1090-WPK-Rev-5-2022.pdf),
[Stahlbauverband Österreich: EN 1090-2 Schwerpunktthemen](https://www.stahlbauverband.at/download/348/EN_1090-2_4._Teil_Schwerpunktthemen.pdf),
[WKO Leitfaden EN 1090](https://www.wko.at/ooe/gewerbe-handwerk/metalltechniker/leitfaden-en-1090-bundesinnung.pdf),
[GSI SLV Flyer EN 1090](https://www.gsi-slv.de/fileadmin/user_upload/Ueber_Uns/Downloads/DIN_EN_1090/Flyer_1090_2011-01.pdf),
[Orgadata: Logikal-Projektdaten im ERP](https://help.orgadata.com/us/?p=7220),
[Logikal Modulübersicht](https://www.logikal-software.com/en/product/module-overview).

## Zu entscheiden

### 1. Was ist ein Fertigungsauftrag?

**Empfehlung**: ein Auftrag **je Projektposition** (Logikal-Position, z. B.
"Pos. 3 – 10 Fenster") mit **Stückzahl** und gewählter **Variante**;
mehrere Aufträge je Position erlaubt (Teillose), die Summe darf die
Positionsmenge nicht überschreiten (Hinweis, keine Sperre — Ersatzteile,
Nachfertigung). Nummer aus dem vorhandenen Nummernkreis
(`number_sequences`), z. B. `FA-2026-0001`.
Alternative: Auftrag je Phase/Haus — gröber, passt schlecht zu
Rückmeldung und Teillieferung.

### 2. Kopie oder Verweis auf die Projektdaten?

**Empfehlung**: beim **Freigeben** werden Position, Maße und die
Materialliste der Variante (Profile mit Länge, Artikel, Glas) **in den
Auftrag kopiert** (Fertigungsstückliste), danach unveränderlich. Grund:
ein erneuter Logikal-Import ersetzt Material und löscht Varianten. Ändert
sich die Position später, zeigt der Auftrag einen **Hinweis**
"Position nach Freigabe geändert" (Vergleich über `external_guid` und
Inhalt); eine Änderung braucht Storno und neuen Auftrag.

### 3. Ablauf und Status

**Empfehlung**: `entwurf` → `freigegeben` → `in_arbeit` → `fertig`, dazu
`storniert` (mit Begründung, nur vor `fertig`). Status mit Zeitpunkt und
Benutzer festgehalten; kein Löschen. Arbeitsgänge, Kapazitäten (H.2) und
Rückmeldungen/BDE (H.3) kommen später und hängen am Auftrag.
Liefer-/Wunschtermin am Auftrag; Liste nach Termin und Status.

### 4. Bezug zu Kundenauftrag und Lager

**Empfehlung**: Freigabe **ohne** Kundenauftrag möglich, aber mit Hinweis
"Kein Kundenauftrag zum Projekt" (Vorfertigung, Musterfenster). Bei der
Freigabe **optional** Reservierungen für das Material anlegen
(vorhandene `stock_reservations` je Projekt) — nur für Artikel, die einem
Lagermaterial zugeordnet sind; Fehlmengen als Hinweis. Entnahme,
Reststücke und Chargen in H.3 (Kommissionierung).

### 5. Ausführungsklasse (EN 1090)

**Empfehlung**: optionales Feld `ausfuehrungsklasse` (EXC1–EXC4) am
Auftrag, ohne Wirkung in H.1. Ab H.3 wird bei EXC2+ bei der Entnahme die
Charge Pflicht (Rückverfolgbarkeit der Zeugnisse). Frage an den Nutzer:
fertigt der Betrieb tragende Bauteile nach EN 1090, und in welcher Klasse?

### 6. Rechte

**Empfehlung**: neue Rechte `production.read` und `production.write` für
`role-production` und Administrator; Vertrieb und Projektleitung lesen
(`production.read`). Freigabe und Storno mit `production.write`.

## Folgen bei Entscheidung wie empfohlen (Vorschau, nicht beschlossen)

- Neues Paket `internal/production`, Migration 107 (Aufträge,
  Fertigungsstückliste, Statusverlauf, Rechte).
- Teilung: H.1.1 Migration und Regeln, H.1.2 Service (Entwurf, Freigabe
  mit Kopie, Status, Storno, Hinweis "geändert"), H.1.3 Routen, H.1.4
  Client (Auftrag aus der Position anlegen, Liste, Detail mit Stückliste).
- Reservierungen bei Freigabe als eigene Teilaufgabe H.1.5.

## Präzisierung H.1.1 (2026-10-31, Migration 107)

- **Verweise mit SET NULL**: der Logikal-Import löscht Positionen und
  Varianten (`import_logikal.go`); `elevation_id` und
  `single_elevation_id` werden dann leer, der Auftrag bleibt. Das Projekt
  mit RESTRICT. Ohne Verweis zeigt der Service später
  `position_entfernt`.
- **Statusverlauf als Spalten** (`freigegeben_at/_by`, `begonnen_at/_by`,
  `fertig_at/_by`, `storniert_at/_by` mit Grund) statt eigener Tabelle:
  der Ablauf ist linear, jeder Übergang passiert höchstens einmal. Die
  Datenbank prüft Zeitpunkte je Status und ihre Reihenfolge.
- **Fingerabdruck** (`inhalt_hash`, SHA-256) über Phase, Position,
  Variante, Maße und Materialliste; gerundet wie die Spalten und
  unabhängig von der Reihenfolge. Abweichung → Hinweis
  `position_geaendert`.
- **Stückzahl ganzzahlig** (1–10 000) — ANNAHME: gefertigt werden ganze
  Elemente; die Positionsmenge aus Logikal ist `REAL`, der Vergleich
  toleriert Rundungsrauschen.
- **Menge je Stück** in der Fertigungsstückliste wie aus Logikal für ein
  Element der Variante — ANNAHME; der Bedarf ist Menge × Stückzahl. Ohne
  Preise (Kalkulationsdaten).
- **Rechte**: `production.read` an Administrator, Produktion und Vertrieb;
  `production.write` an Administrator und Produktion. Eine Rolle
  "Projektleitung" gibt es nicht (`017_auth.sql`).
- **Nummernkreis** `production_order`, Muster `FA-{YYYY}-{NNNN}`, nur für
  den Mandanten `default` angelegt. Der bekannte Fallback "PO-" in
  `settings.NumberingService.Next` (siehe 079) betrifft weitere Mandanten
  — der Service in H.1.2 muss das berücksichtigen.
