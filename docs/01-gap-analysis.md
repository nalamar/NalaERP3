# 01 — Gap-Analyse: Ist-Stand vs. Domänen A–I

> Grundlage: `docs/00-recon.md`. Bewertungsskala je Domäne: **vorhanden** (funktional,
> testbelegt) · **teilweise** (Kernteile da, wesentliche Lücken) · **rudimentär**
> (Grundgerüst ohne Tiefe) · **offen** (kein Code gefunden).

---

## A — Stammdaten

**Status: teilweise.**

| Kernumfang | Status | Beleg / Lücke |
|---|---|---|
| Kunden/Lieferanten | vorhanden | `contacts`, `contact_addresses`, `contact_persons`, CRUD-API (`v1.go:293-609`) |
| Artikel/Profile | teilweise | `materials` vorhanden, aber generisch (kein Metallbau-spezifisches Attributschema für Profile/Bleche/Beschläge über Freitext-`attribute` hinaus, laut `anweisung.md`-Zielbild Subtask 4.1.1.1 offen) |
| Systemlieferanten | offen | kein Konzept "Systemlieferant"/Profilserie-Bindung an Lieferanten im Schema gefunden |
| Preislisten | rudimentär | nur `materials.avg_purchase_price` + `quote_item_price_decisions`-Historie; keine eigenständige Preisliste-Entität mit Gültigkeitszeiträumen/Staffelpreisen |
| Einheiten | vorhanden | `units`-Tabelle (`014_material_dimensions_and_units.sql`), API `/settings/units` |
| Steuersätze | vorhanden | `tax_codes` (`017_accounting_basics.sql`) |

**Lücke zu I**: Domäne I braucht laut aufgabe.md §1 ein Positions-/Preisfindungs-/Artikelstammschema, das "ohne Umbau" trägt. Das generische `attribute`-JSON-Feld an `materials` ist zwar flexibel, aber ohne festgelegtes Metallbau-Schema (Profilserie, RC-Klasse, U-Wert, Brandschutzklasse) — diese Attribute existieren aktuell nirgends strukturiert, sondern wären laut GAEB-Recon (`docs/00-recon.md` Abschnitt 8) erst mit Domäne I einzuführen.

---

## B — Angebots- & Auftragswesen

**Status: vorhanden (mit Lücken bei VOB-Spezifika).**

| Kernumfang | Status | Beleg / Lücke |
|---|---|---|
| LV-Verwaltung, Positionen | teilweise | `quote_items`/`quote_import_items` vorhanden, aber flach — keine Los/Titel/Untertitel-Hierarchie (s. GAEB-Recon) |
| Nachträge | offen | kein Konzept "Nachtrag zu bestehendem Auftrag" im Schema (nur Angebots-Revisionierung `root_quote_id`, das ist Angebotsversionierung vor Zuschlag, kein Nachtragsmanagement nach Auftragserteilung) |
| Kalkulation | teilweise | `quote_calculation_settings` (Zielmarge), `quote_item_price_decisions` — einfaches Zielmargen-Modell, kein vollständiges Kalkulationsschema (Material/Lohn/Fremdleistung/Zuschläge getrennt, wie in `anweisung.md` Subtask 3.1.2.1 vorgesehen) |
| Auftragsbestätigung | vorhanden | `sales_orders`, Konvertierung aus Quote |
| Rechnungsstellung | vorhanden | `invoices_out`, Teilrechnung (`invoice_out_items.source_sales_order_item_id`) |
| Abschlags-/Schlussrechnung nach VOB | offen | kein Feld/Status für Abschlagsrechnung vs. Schlussrechnung, keine VOB-§16-spezifische Logik (Sicherheitseinbehalt, Fälligkeit nach Abnahme) gefunden |

---

## C — Waren- & Lagerwirtschaft

**Status: teilweise.**

| Kernumfang | Status | Beleg / Lücke |
|---|---|---|
| Bestände | vorhanden | `stock_movements`, `batches` |
| Chargen/Längen | rudimentär | `batches`-Tabelle existiert, aber kein erkennbares Längen-/Verschnitt-Attribut (z. B. Reststück-Verwaltung für Profile) |
| Lagerorte | vorhanden | `warehouses`, `locations` |
| Reservierung | offen | keine `reservations`/`reserved_qty`-Struktur gefunden |
| Inventur | offen | kein `inventory_count`/Stichtagsinventur-Konzept gefunden |
| Verschnittverwaltung | offen | kein Code/Schema-Bezug gefunden |

---

## D — Bestellwesen

**Status: teilweise.**

| Kernumfang | Status | Beleg / Lücke |
|---|---|---|
| Bedarfsermittlung | offen | keine automatische Bedarfsermittlung aus Angebot/Mindestbestand gefunden |
| Anfragen | offen | kein Anfrage-Vorlauf-Prozess (RFQ) vor Bestellung im Schema |
| Bestellungen | vorhanden | `purchase_orders`/`purchase_order_items`, PDF-Export |
| Wareneingang | vorhanden | `purchase_order_receipt_flow.dart` (Client), Statusfluss `ordered→received` |
| Rechnungsprüfung | offen | kein `/invoices-in`-Pendant, keine 3-Way-Match-Logik (PO↔Wareneingang↔Eingangsrechnung) gefunden (s. Top-10-Risiko #-, Recon Abschnitt 4) |

---

## E — Finanzwesen

**Status: teilweise, aber ungetestet.**

| Kernumfang | Status | Beleg / Lücke |
|---|---|---|
| OP-Verwaltung | teilweise | `invoice_out_payments`, `bank_statements`-Matching — nur Debitorenseite, keine Kreditoren-OP |
| Zahlungsverkehr | rudimentär | Bankauszugs-Import/Matching vorhanden (`accounting/bank.go`), kein SEPA-Export/-Import für ausgehende Zahlungen gefunden |
| Kostenstellen | offen | kein `cost_center`-Konzept im Schema |
| Projektcontrolling | rudimentär | nur `commercial_context.dart`/`commercial_navigation.dart` als Aggregations-/Anzeige-Layer im Client, kein Soll-Ist-Vergleich im Server |
| Export an Buchhaltung (DATEV) | offen | kein Treffer für DATEV/EXTF im gesamten Repo |

**Risiko-Verweis**: `accounting`- und `sales`-Pakete sind funktional am weitesten ausgebaut in dieser Domäne, aber komplett ungetestet (`docs/00-recon.md` Risiko #3).

---

## F — Personal & HR

**Status: rudimentär.**

| Kernumfang | Status | Beleg / Lücke |
|---|---|---|
| Stammdaten | rudimentär | `hr_employees`, `hr_teams` — Basisfelder |
| Zeiterfassung | offen | keine `time_entries`/Stempeluhr-Struktur gefunden |
| Urlaub/Abwesenheit | teilweise | `hr_leave_requests`, `hr_absences`, `hr_holidays` vorhanden, aber ungetestet, kein Überschneidungscheck |
| Weiterbildung/Qualifikationen/Unterweisungen | offen | kein Schema-Bezug gefunden |
| Asset-Zuordnung | offen | kein Schema-Bezug gefunden (auch keine generische Asset-Tabelle für Werkzeuge/IT/PSA) |

---

## G — Fuhrpark

**Status: offen.** Kein Package, keine Migration, keine Client-Seite mit Fahrzeug-/Fuhrpark-Bezug gefunden. Einzige Spur: die Auth-Rolle `fleet` (`017_auth.sql:71`) — zeigt auf keine Domänenlogik. HU/AU/UVV/Fahrtenbuch/Führerscheinkontrolle: komplett offen.

---

## H — Produktionssteuerung

**Status: offen.** Keine Fertigungsauftrags-, Stücklisten-, Arbeitsgang-, Kapazitäts-, BDE- oder Kommissionierungs-Tabellen gefunden. `project_single_elevations`/`single_elevation_profiles` bilden LogiKal-Kalkulationsdaten ab, keine Fertigungssteuerung. Montageplanung: kein Schema-Bezug.

---

## I — KI-gestützte Angebotserzeugung aus GAEB (Zielwert des Systems)

**Status: nicht begonnen — nur das Fundament aus B existiert.**

Detaillierter Abgleich gegen aufgabe.md §4:

| Anforderung §4 | Status | Beleg |
|---|---|---|
| GAEB DA XML 3.x + D81/D83/D84/D86 | **offen** | Parser liest eigenes Mini-XML-Schema (`server/internal/quotes/gaeb_xml_subset_parser.go:19-33`), keine Phasenunterscheidung |
| Deterministischer, verlustfreier Parser | **teilweise** | regelbasiert (kein LLM im Pfad), aber verlustbehaftet: keine Kurz-/Langtext-Trennung, keine Positionsart, keine Los/Titel/Untertitel-Hierarchie, keine Vorbemerkungen |
| LLM nur für Matching/Textarbeit | **offen** | kein LLM-Provider im Repo (0 Treffer go.mod/pubspec.yaml/Volltextsuche) |
| Preise nie vom Modell erfunden | **strukturell erfüllbar, aktuell trivial** | Preisfindung ausschließlich aus Artikelstamm/Historie (`service.go:979-1024`) — aber es existiert noch kein Modell, das etwas "erfinden" könnte |
| Confidence + Human-in-the-Loop | **teilweise** | HITL-Blockade existiert und ist real erzwungen (Freigabe-Workflow blockiert Konvertierung), aber ausschließlich margen-basiert — kein Confidence-Score für automatisches LV→Leistung-Matching (0 Treffer für `confidence`/`match_score` im `quotes`-Paket) |
| Rückschreibefähigkeit (GAEB D84) | **offen** | keine Export-Funktion, kein Treffer für D84/Export im Code |
| Evaluationsset zuerst | **offen** | kein Testkorpus, keine Kennzahlen (Trefferquote/Fehlzuordnungsrate) |
| Austauschbares Modellzugriff (Provider-Interface) | **offen** | kein Interface-Typ für ein Sprachmodell; einziges Interface ist `GAEBImportParser` (`imports.go:92-94`), das nur den Parser kapselt |

**Bewertung**: Das, was aufgabe.md §1 als Vorbedingung beschreibt ("Positionsmodell, Preisfindung, Artikelstamm, Kalkulationsschema" sollen so gebaut sein, dass I ohne Umbau aufsetzen kann), ist in Ansätzen vorhanden (Positionsmodell flach, Preisfindung mit Historie, einfaches Margenschema) — aber jede der acht in §4 explizit geforderten Eigenschaften von Domäne I selbst ist entweder offen oder nur teilweise erfüllt. Der bestehende "GAEB-Import" ist ein Platzhalter-Format ohne echten GAEB-Bezug, kein Fortschritt in Richtung des eigentlichen Zielwerts.

**Wichtiger Hinweis zur Erwartungssteuerung**: `docs/` enthält über 150 Dateien mit Präfix `gaeb_*` (`*_strategy.md`, `*_audit.md`, `*_inventory.md`), die den Eindruck eines weit fortgeschrittenen GAEB-Features erwecken könnten. Der tatsächliche Code-Stand (siehe Tabelle oben) zeigt: diese Dokumente protokollieren iterative Ausbauschritte des **Angebotswesens (Domäne B)** — Import-Pipeline, Preisfindung, Freigabe-Workflow — nicht Domäne I. Diese Diskrepanz ist einer der Blocker-Punkte (siehe Statusblock am Ende der Session).

---

## Zusammenfassung — Priorisierung für Backlog (nicht Teil dieser Phase, nur Einordnung)

Reihenfolge nach Näherung an Domäne I und Risiko:

1. **Compliance-Fundament nachziehen, bevor B/I weiter wächst**: GoBD-Storno/Festschreibung, Mandantenfähigkeit — beides sind Datenmodell-Änderungen, die rückwirkend teuer werden, wenn sie erst nach mehr Feature-Ausbau nachgezogen werden (aufgabe.md §2: "von Anfang an im Datenmodell, nicht nachgerüstet").
2. **Domäne I in Teilschritten**: echter GAEB-DA-XML-3.x-Parser (ersetzt/erweitert das XML-Subset), verlustfreie Hierarchie, Provider-Interface, Confidence-Modell, Evaluationsset — in dieser Reihenfolge, da Evaluationsset laut §4 explizit *zuerst* vor Modell-Logik verlangt wird.
3. **Test- und Absicherungslücken in bereits gebauten Domänen** (`accounting`, `sales`, `hr`, Client) schließen, bevor weitere Domänen (F–H) im gleichen dünnen Stil aufgesetzt werden.
4. **F/G/H (HR-Tiefe, Fuhrpark, Produktion)** sind vollständig neu zu bauen — hoher Aufwand, aber laut aufgabe.md-Tabelle nicht das Endziel; Priorisierung gegenüber I ist eine fachliche Entscheidung, kein technischer Blocker (→ Blocker-Frage).
