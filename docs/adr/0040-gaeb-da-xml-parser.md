# ADR 0040: Echter GAEB-DA-XML-Parser

- Status: Entwurf (2026-11-13)
- Bezug: Backlog I.2 (Entscheidung Blocker-Frage 3: ersetzt
  `gaeb_xml_subset_parser.go`), ADR 0039 (Evaluationsset), ADR 0009
  (LV-Hierarchie im Angebot), `aufgabe.md` §4 (deterministischer Parser,
  nie ein Sprachmodell zum Interpretieren von GAEB)

## Bestandsaufnahme

- **Schnittstelle:** `GAEBImportParser.ParseGAEB(ctx, io.Reader, filename)`
  liefert `GAEBImportParseResult{ParserVersion, DetectedFormat, Items}`
  (`server/internal/quotes/imports.go`). Eingebunden in
  `internal/http/router.go` und `internal/http/v1.go` mit
  `quotes.GAEBXMLSubsetParser{}`. Ablauf: Upload → GridFS → `ProcessGAEBImport`
  → `SaveImportParseResult` oder `MarkImportFailed`.
- **Der Subset-Parser liest kein GAEB**, sondern ein eigenes Format
  `<gaeb><item position_no qty unit optional><description>`. Verwendet in
  `gaeb_xml_subset_parser_test.go` und in `quotes_integration_test.go`
  (Inline-XML im eigenen Format, Erwartung `gaeb-xml-subset-v1`); weitere
  Integrationstests nutzen einen Test-Parser.
- **Gespeichert wird je Position** (`quote_import_items`, Migration 045):
  OZ, Gliederungsnummer, **ein** Text, Menge, Einheit, `is_optional`,
  Parser-Hinweis, Prüfstatus. Es fehlen Kurz- und Langtext getrennt,
  Positionsart, Pauschale, Bezug Alternative → Grundposition, die
  Gliederung (Los/Titel/Untertitel) und die Vorbemerkungen.
- **Übernahme ins Angebot** (`ApplyImportToDraftQuote`) überträgt nur Text,
  Menge und Einheit der angenommenen Positionen nach `quote_items`. Die
  Gliederung im Angebot gibt es schon (`quote_item_groups`, Migration 071,
  ADR 0009 — ausdrücklich als Ziel für I.2 gebaut), der Import nutzt sie
  nicht.
- **Endungen:** `.x83`, `.x84`, `.d83`, `.p83`, `.gaeb`, `.xml`. `.d83` und
  `.p83` sind GAEB 90/2000 (kein XML) und scheitern heute erst beim Parsen.
- **Fund Upload-Grenze:** `POST /quotes/imports/gaeb` ruft
  `ParseMultipartForm(32 << 20)` ohne `http.MaxBytesReader` auf; größere
  Dateien landen in temporären Dateien, eine harte Obergrenze fehlt (andere
  Uploads wie Nachweise und E-Rechnungen haben eine). AGENTS.md verlangt
  einheitliche Grenzen.
- **Client** (`quotes_page.dart`) zeigt OZ, Gliederung, „Optional“ und
  Parser-Hinweis je Position.
- **Messbasis:** Korpus aus I.1 — 10 LVs, 250 Positionen, je LV das
  erwartete Parse-Ergebnis (Bereiche, Vorbemerkungen, OZ, Kurz-/Langtext,
  Menge, Einheit, Art, Pauschale, `zu`). Die X83-Dateien sind aus der
  Erwartung erzeugt; die Elementnamen außer `Provis` und `LumpSumItem`
  sind nicht belegt (ADR 0039, Folgen).

## Zu entscheiden

1. **Phasen.** (a) Zuerst X83 (DP 83); X81/X86 nehmen dieselbe Struktur
   und folgen, sobald die Elementnamen geprüft sind; X84 erst mit der
   Rückschreibung (I.6). (b) X81, X83 und X86 sofort.
   Empfehlung: **(a)** — das Korpus deckt nur X83 ab.
2. **Unbelegte Elementnamen.** (a) Parser nach den Namen des Korpus,
   tolerant gegen den Namespace (nur lokale Namen, Version aus Namespace
   oder `GAEBInfo/Version`), mit klarer Fehlermeldung bei Unbekanntem;
   Abgleich, sobald eine echte X83 oder das Schema vorliegt (dann
   Korrektur an einer Stelle in Generator und Parser, Korpus neu
   erzeugen). (b) Erst nach einer echten Datei bzw. dem Schema beginnen.
   Empfehlung: **(a)** — dazu die Bitte um eine echte X83 (lokal, nicht
   ins Repo) oder die Fachdokumentation/das Schema.
3. **Datenmodell.** (a) `quote_import_items` erweitern um Kurztext,
   Langtext, Art, Pauschale, Grundposition (OZ) und Bezug zum Bereich;
   neue Tabelle `quote_import_groups` (Los/Titel/Untertitel mit Eltern,
   analog `quote_item_groups`); Vorbemerkungen am Importlauf. `description`
   bleibt (Kurztext + Langtext) für den heutigen Client und die Übernahme.
   (b) Parse-Ergebnis als JSON am Importlauf. (c) Alles in `description`
   zusammenfassen wie heute. Empfehlung: **(a)** — I.4 (Matching), I.5
   (Prüfliste) und I.6 (D84) brauchen die Felder einzeln und
   abfragbar. `is_optional` wird aus der Art abgeleitet (Bedarfs- und
   Alternativpositionen) und bleibt für den Client.
4. **Übernahme ins Angebot.** (a) Eigener Schritt in I.2: Gliederung nach
   `quote_item_groups`, Kurz-/Langtext, Art übernehmen. (b) Übernahme
   bleibt wie heute, Gliederung später. Empfehlung: **(a)** als letzter
   Teilpunkt von I.2 — das Zielmodell aus ADR 0009 steht bereit.
5. **Subset-Parser und Endungen.** (a) Der neue Parser ersetzt den
   Subset-Parser (Entscheidung Blocker-Frage 3); das eigene Format entfällt
   ersatzlos, seine Tests werden auf echte DA-XML-Fixtures umgestellt;
   `.d83`/`.p83` werden beim Upload mit klarer Meldung abgelehnt, bis
   GAEB 90/2000 ein eigener Punkt ist; `.gaeb`/`.xml` werden am Inhalt
   (Wurzelelement, Namespace) erkannt. (b) Subset-Parser als Rückfall
   behalten. Empfehlung: **(a)** — das eigene Format ist kein GAEB und wird
   von keinem Programm erzeugt. Dieser ADR ist die nach den Regeln nötige
   Grundlage für das Entfernen.
6. **Grenzen.** (a) Upload hart begrenzt (`MaxBytesReader`, Vorschlag
   20 MB), im Parser zusätzlich höchstens 20.000 Positionen, Texte wie im
   Evaluationsset (Langtext ≤ 20.000 Zeichen), Gliederungstiefe ≤ 5; Fehler
   statt Kürzen. (b) Nur die Upload-Grenze. Empfehlung: **(a)**. Go's
   `encoding/xml` löst keine externen Entitäten auf; die Grenzen schützen
   vor sehr großen oder tief verschachtelten Dateien.

## Vorschlag zur Umsetzung (bei Empfehlung)

- **Paket:** `server/internal/quotes/gaeb` (deterministisch, ohne
  Datenbank), Ergebnis als Typ mit Bereichen, Vorbemerkungen und
  Positionen; Adapter auf `GAEBImportParser`.
- **Messung:** Test liest alle X83 des Korpus und vergleicht mit
  `lv-NN.erwartet.json` (Bereiche, Vorbemerkungen, je Position alle
  Parse-Felder); Ziel 100 % Übereinstimmung. Danach liefert der Parser die
  Eingabe für das Evaluationsset (Einschränkung aus I.1.4a entfällt).
- **Teilung:** I.2.1a Kern (Kopf, Gliederung, Positionen, Texte, Art) gegen
  das Korpus; I.2.1b Grenzen, Fehlerfälle, Adapter; I.2.2 Migration 116
  und Speichern; I.2.3 Upload-Grenze, Endungen, Ablösung Subset-Parser mit
  Umstellung der Tests; I.2.4 Übernahme der Gliederung ins Angebot; I.2.5
  Client zeigt Kurztext, Art und Gliederung.

## Folgen

- Kein Sprachmodell im Parser (`aufgabe.md` §4).
- Migration 116 erst in I.2.2, mit DOWN-Pfad und Hinweis auf
  Datenverlust (neue Spalten und Tabelle).
- Das heutige Subset-Format entfällt — Importe darin gibt es nur in Tests.
