# ADR 0039 — Epic I: Evaluationsset für das GAEB-Matching

Datum: 2026-11-12
Status: **Entwurf** — Fragen in `docs/open-questions.md`, Abschnitt
2026-11-12 I.1. Kein Produktivcode.
Bezug: Backlog I.1 "Evaluationsset zuerst" (muss vor I.3 stehen);
`aufgabe.md` §4 (Sonderregeln Epic I); Blocker-Frage 3 vom 2026-08-11
(echter DA-XML-3.x-Parser ersetzt den Subset-Parser, Backlog I.2); ADR
zu A.1 (Profilattribute am Material, Migration 068); offene Detailfrage
Brandschutzklassen (DIN 4102 oder DIN EN 13501, `open-questions.md`).

## Kontext (Ist-Stand, belegt)

- **GAEB-Import gibt es nur als Gerüst.** `quotes/imports.go`: Upload je
  Projekt (Endungen `.x83`, `.x84`, `.d83`, `.p83`, `.gaeb`, `.xml`), Datei
  in GridFS, Parser über das Interface `GAEBImportParser`, Positionen in
  `quote_import_items` (OZ, Gliederungsnummer, ein Text, Menge, Einheit,
  `is_optional`), Prüfung je Position (`review_status`), Übernahme der
  angenommenen Positionen in einen Angebotsentwurf (`quote_import_item_links`).
- **Der Parser liest kein GAEB.** `gaeb_xml_subset_parser.go` erwartet ein
  eigenes Format `<gaeb><item position_no=… qty=… unit=…>` — weder
  DA XML 3.x noch GAEB 90/2000. Es fehlen Hierarchie (Los/Titel/Untertitel),
  Kurz- und Langtext getrennt, Positionsarten außer "optional",
  Vorbemerkungen. Ersatz ist entschieden (Blocker-Frage 3 → I.2).
- **Tests und Testdaten:** `gaeb_xml_subset_parser_test.go` (ein Inline-
  Beispiel), `imports_test.go` (Ablauf mit Fake-Parser). Im Repo liegt
  **keine einzige GAEB-Datei** und kein Leistungsverzeichnis mit erwarteten
  Ergebnissen. Die rund 225 Dateien `docs/gaeb_*.md` sind Alt-Artefakte
  (Blocker-Frage 2: unangetastet).
- **Es gibt kein Zuordnungsziel "interne Leistung".** Vorhanden sind:
  - `materials` mit `profilserie`, `rc_klasse`, `u_wert`,
    `brandschutzklasse` (Migration 068, A.1) — Artikel, keine Leistungen;
  - `price_lists`/`price_list_items` je Material (069);
  - `quote_text_blocks` (043: Code, Name, Kategorie, Text) — **ohne
    `company_id`, Code global eindeutig** (Fund: Mandantenlücke, siehe
    Folgen);
  - `quote_items` mit Kalkulation `quote_item_calculations` (073: Material,
    Lohnstunden und -satz, Fremdleistung mit Zuschlägen) — frühere
    Angebotspositionen, aber ohne Bezug zu einer Leistung oder Stückliste;
  - Logikal-Projekte mit Positionen, Varianten und Artikeln
    (`single_elevation_articles`) — projektgebunden, kein Katalog.
  Für das Matching "LV-Position → interne Leistung/Stückliste" (§4) fehlt
  also der Katalog, gegen den gemessen werden kann.
- **Systemvorgaben** nennt §4: Profilserie, Verglasung, RC-Klasse, U-Wert,
  Brandschutzklasse. Vier davon stehen am Material; Verglasung nicht. Die
  Klassierung beim Brandschutz ist fachlich offen.

## Recherche (Sekundärquellen)

- **Positionsarten in DA XML** werden über Kindelemente des `Item`
  ausgedrückt (z. B. `Provis` für Bedarfspositionen), Grund- und
  Wahlpositionen über eine Zuordnungsnummer, deren letzte Stelle Grund (0)
  und Alternative (1–9) unterscheidet; Alternativen stehen nur mit
  Einheitspreis und ohne Gesamtbetrag im LV (pyGAEB-Referenz; Schemadruck
  TU Dresden 2009). Die exakten Element- und Attributnamen sind in I.2
  gegen das offizielle Schema zu prüfen — hier nicht belegt.
- **Bedarfsposition ohne Gesamtbetrag** verlangt nur einen Einheitspreis
  (Bauprofessor). Für das Evaluationsset heißt das: die Positionsart gehört
  zum erwarteten Ergebnis, weil sie die Rückschreibung (D84, I.6) und die
  Summen bestimmt.
- **Phasen** erkennt man an der Endung (X81 … X86; X83 Angebotsaufforderung,
  X84 Angebotsabgabe). DA XML 3.3 ändert gegenüber 3.2 vor allem BIM-Bezüge;
  öffentliche Auftraggeber nutzen teils noch 3.2 (AVA-Pflichtenheft SBN 2024).
  → Korpus in 3.2 und 3.3.
- Offizielle Musterdateien des GAEB wurden nicht gefunden; frei nutzbare
  Prüfprogramme (Viewer) gibt es von Drittanbietern. → eigene, synthetische
  LVs sind der verlässliche Weg; ein Prüfprogramm kann sie vor dem Einchecken
  auf Schemakonformität prüfen.

## Zu entscheiden

1. **Herkunft des Korpus.** Echte LVs enthalten Auftraggeber, Bauvorhaben,
   Adressen, teils Namen — nach den Vorgaben nicht ins Repo.
   - (a) nur synthetische LVs im Repo;
   - (b) anonymisierte echte LVs im Repo;
   - (c) synthetische im Repo, echte oder anonymisierte nur lokal außerhalb
     des Repos (Pfad per Umgebungsvariable), im Repo nur deren Kennzahlen.
   Empfehlung: **(c)**, Beginn mit (a). Anonymisieren ist bei LVs
   fehleranfällig (Bauort und Objekt stehen im Langtext).
2. **Umfang** (Platzhalter `<<Anzahl>>` in §4).
   - (a) 10 synthetische LVs mit zusammen rund 250–300 Positionen aus
     Fenster, Türen, Fassade, Stahl-/Metallbau; alle Positionsarten und
     Hierarchiestufen; rund 20 % Positionen ohne passende Leistung
     (Fremdgewerk, Baustelleneinrichtung); gezielte Fallen (ähnlicher Text,
     andere Einheit, abweichende Systemvorgabe);
   - (b) 3 LVs als Minimum;
   - (c) 30 und mehr.
   Empfehlung: **(a)**; erweitern, wenn echte LVs lokal dazukommen.
3. **Zuordnungsziel.**
   - (a) ein Leistungskatalog je Mandant (Code, Kurztext, Einheit,
     Systemvorgaben, Kalkulationsansatz aus Material, Lohn und
     Fremdleistung; Preise nur daraus) — im Korpus zuerst als Datei mit
     festen Codes, in der Datenbank als eigener Punkt vor I.4;
   - (b) direkt Materialien und Textbausteine;
   - (c) frühere Angebotspositionen (Ähnlichkeitssuche).
   Empfehlung: **(a)**; (c) später als zusätzliche Quelle für Vorschläge.
   Begründung: eine LV-Position ("Fenster liefern und montieren") ist eine
   Leistung aus Material, Lohn und Fremdleistung, kein einzelner Artikel;
   §4 verlangt Preise nur aus Artikelstamm, Preisliste oder
   Kalkulationsschema.
4. **Kennzahlen und Zielwerte.** Je Position erwartet: Positionsart,
   Leistungscode oder "kein Ziel", Systemvorgaben. Kennzahlen:
   - Trefferquote = richtig automatisch zugeordnete / Positionen mit
     erwartetem Ziel;
   - Fehlzuordnungsrate = falsch automatisch zugeordnete (auch bei "kein
     Ziel") / alle automatisch zugeordneten (Score über der Schwelle);
   - Prüflistenquote = Positionen unter der Schwelle / alle;
   - Systemvorgaben je Merkmal: Genauigkeit und Vollständigkeit.
   - (a) Zielwerte als Bedingung für den Einsatz von I.4:
     Fehlzuordnungsrate ≤ 2 %, Trefferquote ≥ 70 %;
   - (b) nur messen, Zielwerte später.
   Empfehlung: **(a)** — eine falsche automatische Zuordnung ist teurer als
   eine Position mehr in der Prüfliste; die Schwelle wird so gewählt, dass
   die Fehlzuordnungsrate hält.
5. **Formate im Korpus.**
   - (a) DA XML 3.2 und 3.3, Phase X83 (dazu je ein X81 und X86);
     GAEB 90/2000 (D83/P83) später als eigener Punkt;
   - (b) auch GAEB 90/2000 sofort.
   Empfehlung: **(a)**; die erlaubten Endungen `.d83`/`.p83` bleiben, bis
   I.2 sie bewusst ablehnt oder unterstützt.

Auskunft (kein Blocker): **Liest der Betrieb GAEB heute in Logikal ein und
kalkuliert dort?** Dann ist zu klären, ob das ERP-Matching Logikal ergänzt
(Fremdgewerk, Montage, Nebenleistungen) oder ersetzt. Ein synthetisches
Beispiel-LV im Stil der üblichen Ausschreibungen wäre hilfreich.

## Vorschlag zur Umsetzung (bei Empfehlung)

- **Ablage:** `server/internal/quotes/eval/testdata/` mit `katalog.json`
  (synthetischer Leistungskatalog), je LV `lv-NN.x83` und
  `lv-NN.erwartet.json`. Die erwartete Datei enthält je Position auch das
  erwartete Parse-Ergebnis (OZ, Hierarchie, Kurz-/Langtext, Menge,
  Einheit, Positionsart) — sie ist damit zugleich das Prüforakel für den
  Parser in I.2.
- **Auswertung:** Go-Paket `quotes/eval` mit Kennzahlen als reinen
  Funktionen (tabellengetrieben getestet) und einem Interface `Matcher`
  (Position → Code, Score, Begründung). Lauf mit `go test` hinter einer
  Umgebungsvariable, Bericht als Tabelle je LV und gesamt. Zwei
  Basis-Matcher als Referenz: "ordnet nie zu" und ein einfacher
  deterministischer Wortabgleich — so sind die Kennzahlen belegt, bevor ein
  Modell eingebunden wird.
- **Reihenfolge:** I.1 (Korpus, Erwartung, Kennzahlen) → I.2 Parser wird am
  Korpus gemessen → I.3 Provider-Interface → I.4 Matching gegen die
  Zielwerte.
- **Teilung I.1** (jeweils unter 8 Dateien und 400 Zeilen): I.1.1 Format
  der Erwartung, Katalog und Kennzahlen mit Tests; I.1.2 und I.1.3
  synthetische LVs (je etwa fünf); I.1.4 Auswertung mit Basis-Matchern und
  Bericht.

## Folgen

- Kein Produktivcode, keine Migration in I.1. Ein Leistungskatalog in der
  Datenbank (Frage 3 (a)) wird ein eigener Backlog-Punkt vor I.4.
- **Fund Mandantenlücke:** `quote_text_blocks` hat kein `company_id`, der
  Code ist mandantenübergreifend eindeutig. Relevant für I.4 (Vorschlag
  von Textbausteinen); als eigener Punkt nachzuziehen, nicht Teil von I.1.
- Die offene Frage zur Brandschutzklassierung bestimmt die erwarteten
  Werte; bis zur Klärung speichert die Erwartung die Angabe wie im LV und,
  falls eindeutig, die Klasse.
- Kein Modellzugriff in I.1; Datenschutzfragen zum Versand von LVs an einen
  Modellanbieter gehören zu I.3.
