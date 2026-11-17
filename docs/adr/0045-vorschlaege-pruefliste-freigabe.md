# ADR 0045: Zuordnungsvorschläge am GAEB-Import, Prüfliste und Freigabe

- Status: **Angenommen** (2026-11-16, vom Nutzer: „Deine Annahme kann so
  umgesetzt werden“): 1–7 jeweils (a). **Frage 4 angepasst** durch ADR 0046
  (angenommen 2026-11-17): mit mehreren Modellprofilen gehört die Schwelle
  ans Profil.
  Fragen in `docs/open-questions.md`, Abschnitt 2026-11-16 I.5
- Bezug: Backlog I.5, `aufgabe.md` §4 (Score und Begründung je Zuordnung,
  unterhalb der Schwelle in die Prüfliste, Angebot erst nach Freigabe durch
  einen Menschen, Preise nie vom Modell), ADR 0040 (GAEB-Import), ADR 0043
  (Modellzugriff, Datenschutz, Grenzen), ADR 0044 (Matching mit Modell)

## Bestandsaufnahme (belegt)

- **Importablauf** (`server/internal/quotes/imports.go`): Hochladen
  (`POST /quotes/imports/gaeb`) → Verarbeiten (`POST …/{id}/process`,
  synchron, Status `parsed`) → je Position `review_status`
  `pending`/`accepted`/`rejected` mit `review_note`
  (`PATCH …/items/{itemID}/review`) → Freigabe des Laufs
  (`PATCH …/{id}/review`, nur ohne offene Positionen, Status `reviewed`) →
  Übernahme ins Angebot (`POST …/{id}/apply`, Status `applied`, Preis 0,
  Verknüpfung in `quote_import_item_links`). Rechte: Lesen `quotes.read`,
  alles andere `quotes.write`.
- **`review_status` ist keine Zuordnung.** Er sagt nur, ob eine LV-Position
  ins Angebot übernommen wird. Code, Score und Begründung haben keinen Platz;
  `parser_hint` ist ein Hinweis des Parsers.
- **`quote_import_items` hat kein `company_id`**; der Mandant kommt über
  `quote_imports.project_id → projects.company_id` (wie bei den Gruppen,
  Migration 116).
- **`quote_items`** kennt `material_id` und `price_mapping_status`
  (Migration 047) für die Preisfindung, aber keinen Bezug zu einer Leistung.
  Die Leistung (Migration 120) hat bewusst **keinen Preis**.
- **Matcher liegt in `eval`** (`ModellMatcher`, `Kandidaten`,
  `MitSystemvorgaben`, `ErkenneSystemvorgaben`) und arbeitet auf `gaeb.LV`
  und `eval.Katalog`. Im Betrieb liegen LV und Katalog in der Datenbank
  (`quote_import_items`/`_groups`, `leistungen`).
- **Dauer:** ein LV mit 250 Positionen braucht etwa 15 Aufrufe, je Aufruf bis
  300 s Zeitlimit (einstellbar bis 30 min). Der HTTP-Server bricht nach 300 s
  ab (`WriteTimeout`, `internal/app/server.go`). Es gibt keine Warteschlange
  für Hintergrundarbeit; das einzige Muster ist die Zeitsteuerung der
  Sammelmails (`noticemail/schedule.go`, Goroutine im API-Prozess).
- **Ohne Schlüssel** (I.4.3 offen) gibt es im Betrieb nur den Test-Anbieter;
  ohne eingeschaltetes Sprachmodell läuft nach ADR 0044 nur der
  deterministische Teil (Wortabgleich und Systemvorgaben).

## Zu entscheiden

1. **Speicherort der Vorschläge.**
   (a) neue Tabelle `quote_import_item_matches`, eine Zeile je Importposition:
   Leistung (Verweis, `SET NULL`) und Code als Abschrift, Score, Begründung,
   erkannte Systemvorgaben, Quelle (`modell`/`wortabgleich`), Anbieter und
   Modell, Zeitpunkt des Laufs, Entscheidung mit Person und Zeitpunkt;
   ein neuer Lauf ersetzt nur unentschiedene Vorschläge;
   (b) Spalten an `quote_import_items`;
   (c) mehrere Vorschläge je Position (Rangliste).
   Empfehlung: **(a)** — trennt Parserdaten von Vorschlägen, ein erneuter Lauf
   berührt keine Entscheidung; (c) erst, wenn die Prüfliste Alternativen
   zeigen soll (Kandidaten lassen sich dort auch frisch berechnen).
2. **Auslösen und Dauer.**
   (a) eigener Endpunkt `POST …/{id}/match`, läuft im Hintergrund
   (Goroutine im API-Prozess, Status am Importlauf: `läuft`/`fertig`/
   `Fehler` mit Fortschritt; je Bündel gespeichert, ein Neustart des
   Prozesses lässt den Lauf als abgebrochen stehen und er kann neu gestartet
   werden); der Client fragt den Status ab;
   (b) synchron im Endpunkt (bricht bei großen LVs nach 300 s ab);
   (c) automatisch nach dem Verarbeiten.
   Empfehlung: **(a)** — ein Mensch entscheidet, wann Daten nach außen gehen
   und das Tageslimit belastet wird; (c) später als Schalter je Mandant.
3. **Ohne eingeschaltetes Sprachmodell.**
   (a) derselbe Endpunkt läuft deterministisch (Wortabgleich mit Schwelle 0,7
   und Systemvorgaben), Quelle `wortabgleich`;
   (b) Endpunkt antwortet 409.
   Empfehlung: **(a)** — nach ADR 0039 ohne Fehlzuordnung am Korpus, und der
   Ablauf ist ohne Schlüssel prüfbar.
4. **Schwelle.**
   (a) je Mandant in `company_llm_settings` (Standard 0,9, Bereich 0–1),
   einstellbar auf der Seite „Sprachmodell“; für den Wortabgleich fest 0,7;
   (b) fest im Code;
   (c) je Anbieter und Modell in einer eigenen Tabelle.
   Empfehlung: **(a)** — die Messung aus I.4.3 setzt den Wert, (c) erst, wenn
   ein Mandant mehrere Modelle gleichzeitig nutzt.
5. **Prüfliste und Freigabe.**
   (a) jeder Vorschlag bleibt offen, bis ein Mensch ihn bestätigt, ändert
   (andere Leistung) oder verwirft; die Prüfliste zeigt zuerst Vorschläge
   unter der Schwelle und Positionen ohne Vorschlag; über der Schwelle gibt
   es „alle über der Schwelle bestätigen“ als bewusste Sammelaktion; die
   Freigabe des Laufs (`…/review`) verlangt, dass kein Vorschlag mehr offen
   ist; Positionen ohne Vorschlag bleiben zulässig (manuelle Kalkulation);
   (b) Vorschläge über der Schwelle gelten ohne Zutun als bestätigt;
   (c) Vorschläge sind nur Hinweise, die Freigabe des Laufs prüft sie nicht.
   Empfehlung: **(a)** — erfüllt §4 („erst gültig, wenn ein Mensch
   freigegeben hat“) und ADR 0044 („bleibt ein Vorschlag bis zur Freigabe“);
   die Sammelaktion hält den Aufwand klein. Recht: `quotes.write` wie die
   übrige Prüfung.
6. **Übernahme ins Angebot.**
   (a) neue Spalte `quote_items.leistung_id` (`SET NULL`) aus der bestätigten
   Zuordnung; Preis bleibt 0 (Preise nur aus Artikelstamm, Preisliste oder
   Kalkulation — der Kalkulationsansatz je Leistung ist ein eigener Punkt);
   (b) nur die Zuordnung am Import, nichts am Angebot.
   Empfehlung: **(a)** — der Bezug überlebt die Übernahme, und die spätere
   Kalkulation und die D84-Ausgabe (I.6) können ihn nutzen.
7. **Ort des Matchers.**
   (a) neues Paket `internal/matching` (Kandidaten, Systemvorgaben,
   Wortabgleich, Matcher mit Modell, auf eigenen Typen für Position und
   Leistung); `eval` nutzt es und behält Korpus, Kennzahlen und Bericht;
   (b) im Paket `quotes`;
   (c) in `eval` lassen und aus dem Produktivcode importieren.
   Empfehlung: **(a)** — `quotes` ist schon groß (über 4.900 Zeilen in
   `service.go`/`imports.go`), und Testcode gehört nicht in den Produktivpfad.

Unabhängig davon (aus §4 und ADR 0043/0044, nicht zur Wahl): an das Modell
gehen nur LV- und Katalogfelder, nie Kunde, Projekt oder Personen; das
Protokoll enthält keine Inhalte; keine Preise im Schema; Vorschläge und
Entscheidungen bleiben je Mandant getrennt (Scope über den Importlauf).

## Entscheidung (2026-11-16)

Alle sieben Fragen mit (a): eigene Tabelle für Vorschläge, Lauf im
Hintergrund über einen eigenen Endpunkt, ohne Sprachmodell deterministisch,
Schwelle einstellbar (Ort nach ADR 0046: am Modellprofil), jeder Vorschlag
offen bis zur Entscheidung eines Menschen mit Sammelbestätigung über der
Schwelle, `quote_items.leistung_id` bei der Übernahme, Matcher in
`internal/matching`. Reihenfolge und Migrationsnummern siehe ADR 0046.

## Vorschlag zur Umsetzung (bei Empfehlung)

- **I.5.1** Umzug des Matchers nach `internal/matching` mit eigenen Typen;
  `eval` nutzt ihn, Kennzahlen unverändert (Gegenprobe: gleicher Bericht).
- **I.5.2** Migration 122 (`quote_import_item_matches`, Status des Laufs am
  Import, Schwelle in `company_llm_settings`) und Service: Lauf aus der
  Datenbank (Positionen, Gruppen, Vorbemerkungen, aktive Leistungen),
  Speichern je Bündel, Entscheidungen.
- **I.5.3** HTTP: Lauf starten (Hintergrund), Status, Vorschläge lesen,
  entscheiden, Sammelbestätigung; Freigabe des Laufs prüft offene Vorschläge.
- **I.5.4** Übernahme: `quote_items.leistung_id` (Migration 123).
- **I.5.5** Client: Prüfliste im Importdialog, Schwelle auf der Seite
  „Sprachmodell“.

## Folgen

- Ein Importlauf kann ohne Zuordnung wie bisher freigegeben werden, solange
  kein Lauf gestartet wurde (kein Bruch bestehender Abläufe).
- Hintergrundarbeit im API-Prozess ist neu; mehrere Instanzen bräuchten eine
  Sperre je Importlauf (Status in der Datenbank, nur ein Lauf je Import).
- Bis zur Antwort ändert sich kein Produktivcode.
