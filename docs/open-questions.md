# Open Questions

> Format je Eintrag: Datum, Frage, Status (offen/beantwortet), Entscheidung
> (falls beantwortet), Bezug zu Backlog-Nummer.

## 2026-08-11 — Blocker-Fragen Phase 0 (beantwortet)

1. **Mandantenfähigkeit**: DB-Schema ist durchgängig Single-Tenant, aufgabe.md
   §2 fordert Mehrmandantenfähigkeit von Anfang an. Status: **beantwortet**.
   Entscheidung: Nachrüsten als Breaking-Change-Migration. → Backlog 0.2.
2. **Umgang mit Alt-Artefakten** (codex.md/anweisung.md-Vorgänger-Backlogs,
   >150 `docs/gaeb_*.md`-Dateien). Status: **beantwortet**. Entscheidung:
   unangetastet lassen, `docs/backlog.md` beginnt neu mit A–I-Nummerierung,
   keine Migration des Codex-Fortschritts.
3. **GAEB-Parser-Strategie**: bestehender `gaeb_xml_subset_parser.go` liest
   kein echtes GAEB-Format. Status: **beantwortet**. Entscheidung: wird durch
   echten DA-XML-3.x/D81-D86-Parser ersetzt (nicht parallel aufgebaut). →
   Backlog I.2.
4. **Priorisierung nächste Arbeit**: Compliance-Fundament vs. Domäne I vs.
   Testlücken. Status: **beantwortet**. Entscheidung: Testlücken in
   `accounting`/`sales`/`hr` zuerst (höchstes akutes Risiko bei bereits
   produktivem Code). → Backlog 0.1, vor 0.2/0.3.

## 2026-08-11 — Blocker: Migrationen nicht laufzeitverifizierbar (offen)

Bei Subtask 0.2.1.2.1 festgestellt: `docker compose -f docker-compose.test.yml
up` schlägt fehl, weil der Docker-Desktop-Daemon in dieser Arbeitsumgebung
nicht läuft (`docker info` bestätigt: Named-Pipe `dockerDesktopLinuxEngine`
nicht erreichbar). Damit lässt sich keine neue Migration gegen echtes
Postgres ausführen — Definition of Done "Migration reversibel"/"Verifikation:
tatsächlich ausführen" (aufgabe.md §6.6) ist für `054_contacts_company_branch_scope.sql`
und alle folgenden Schema-Migrationen dieser Session **nicht erfüllbar**,
solange dieser Zustand anhält. Status: **offen**, Frage an den Nutzer: wie
fortfahren — (a) Docker Desktop lokal starten und Session fortsetzen lassen,
(b) unverifizierte Migrationen vorerst akzeptieren und später gebündelt
verifizieren, oder (c) Schema-Arbeit pausieren und mit nicht-DB-abhängigen
Backlog-Positionen fortfahren, bis Docker verfügbar ist.

## Offene Detailfragen (aus Phase-0-Recherche, keine Blocker für Backlog-Start)

- **0.2.1.1 (Mandanten-Scoping-Design)**: Auf welcher Ebene wird gescoped —
  `company_id` (Mandant) oder zusätzlich `branch_id` (Standort) je Fachtabelle?
  Betrifft auch, ob `number_sequences` pro Mandant oder pro Mandant+Standort
  laufen. Wird in der ADR zu Subtask 0.2.1.1 geklärt, sobald diese Subtask
  ansteht — kein Blocker für den Start von Epic 0.1.
- **E.3/E.4 (DATEV/E-Rechnung)**: Keine Angabe, ob ein konkretes
  Buchhaltungssystem als DATEV-Exportziel vorgegeben ist (Format-Version 13
  ist in aufgabe.md §2 fix vorgegeben). Status: **beantwortet** bei Erreichen
  von E.3 (2026-09-04) — EXTF ("Extern-Format") ist laut DATEV-Definition
  explizit für den Import in JEDES DATEV-kompatible System gedacht, ein
  konkretes Zielsystem ist keine Voraussetzung. → `docs/adr/0021-datev-export.md`.
- **E.3.1 (DATEV-Wirtschaftsjahresbeginn-Tag)**: `company_profiles.datev_fiscal_year_start_month`
  (ADR 0021) bildet nur den Monat des Wirtschaftsjahresbeginns ab, der Tag
  wird fix als der 1. angenommen. Ein abweichender Beginn mitten im Monat
  (in der deutschen Praxis extrem selten) kann damit nicht abgebildet werden.
  Status: **offen**, kein Blocker für E.3.2/E.3.3 (Default deckt den
  Standardfall Kalenderjahr ab), aber vor einer produktiven Nutzung mit
  einem Mandanten mit tatsächlich abweichendem Wirtschaftsjahr zu klären.
- **E.4.1 (ZUGFeRD/PDF-A-3-Konformität)**: `docs/adr/0022-e-rechnung-ausgang.md`
  legt fest, dass der ZUGFeRD-Export eine PDF mit eingebetteter,
  inhaltlich vollständiger CII-XML (`factur-x.xml`) liefert, OHNE formale
  ISO-19005-3(PDF/A-3)-Zertifizierungskonformität (kein XMP-Metadaten-
  Stream, kein `/OutputIntent` mit eingebettetem ICC-Profil) — die
  bestehende PDF-Erzeugung (`github.com/jung-kurt/gofpdf`) bietet dafür
  keine native Unterstützung, eine vollständige PDF/A-3-Konformität würde
  einen Wechsel der PDF-Engine erfordern. Status: **offen**, kein Blocker
  für E.4.2/E.4.3 (der Anhang ist inhaltlich vollständig und von den
  meisten E-Rechnungs-Extraktionswerkzeugen lesbar), aber vor einer
  Nutzung mit Empfängern zu klären, die strikte PDF/A-3-Validierung
  voraussetzen (z. B. manche öffentliche Auftraggeber-Portale).
- **E.4.1 (Mengeneinheit je Rechnungsposition)**: `invoice_out_items` hat
  kein `unit`-Feld; der E-Rechnungs-Export verwendet fest den
  UN/ECE-Rec.-20-Fallback-Code `"C62"` ("Stück/nicht näher spezifizierte
  Einheit") für alle Positionen. Status: **offen**, kein Blocker für
  E.4.2/E.4.3, aber vor einer Erweiterung um echtes Einheiten-Tracking
  bei Rechnungspositionen (eigenständiges, größeres Feature) zu klären.
- **0.1.3.1 (hr: Default-Wert für `Employee.Active` bei Anlage)**: Beim Beheben
  des No-Op-Bugs (`server/internal/hr/service.go:98-100`) aufgefallen: neue
  Mitarbeitende erhalten `Active=false`, sofern der Aufrufer es nicht explizit
  auf `true` setzt (Go-Zero-Value für `bool`). Ob ein neu angelegter
  Mitarbeiter fachlich standardmäßig aktiv sein soll, ist unklar und wurde
  NICHT geändert (reine Bugfix-Subtask, kein Verhaltenswechsel ohne Freigabe).
  Kein Blocker für die laufende Subtask-Kette, aber vor Abschluss von Task
  0.1.3 zu klären.
- **A.1.1 (Brandschutzklasse-Klassifikationssystem)**: `docs/adr/0006-metallbau-artikel-profilattribute.md`
  legt für `materials.brandschutzklasse` bewusst freien Text statt einer
  Enum-Validierung fest, weil im deutschen/europäischen Baurecht mehrere,
  nicht deckungsgleiche Systeme parallel existieren (DIN 4102
  Feuerwiderstandsklassen `F30`/`F60`/`F90`/`F120`/`F180` für Bauteile,
  `T30`/`T90` für Feuerschutzabschlüsse/Türen; DIN EN 13501-2 z. B.
  `EI30`/`EI60`/`REI90`), die je nach Bauteilart unterschiedlich greifen.
  Frage an die Fachseite: welches System (oder welche Kombination) soll das
  Metallbau-ERP tatsächlich abbilden? Status: **offen**, kein Blocker für
  A.1.2/A.1.3 (freier Text funktioniert bis zur Klärung), aber vor einer
  künftigen Enum-Verschärfung oder vor Epic I (GAEB-Systemvorgaben-Erkennung)
  zu klären.

- **E.5.1 (Dateiname des eingebetteten ZUGFeRD-Anhangs)**:
  `docs/adr/0023-e-rechnung-eingang.md` legt fest, dass ZUGFeRD-PDFs beim
  Eingang gelesen werden. Verifiziert ist nur der Anhangname `factur-x.xml`
  (Factur-X/ZUGFeRD 2.1+, den wir im Ausgang selbst schreiben); ältere
  ZUGFeRD-Stände verwenden abweichende Namen, für die im Repo keine
  belastbare Quelle vorliegt. Statt eine Namensliste zu raten, wird beim
  Umsetzen von E.5.4 entschieden, ob überhaupt nach Namen gesucht oder
  schlicht der erste Anhang genommen wird, der sich als EN-16931-XML parsen
  lässt. Status: **beantwortet** in E.5.4 (2026-09-24): es wird NICHT auf einen
  festen Namen bestanden. Der Anhang mit dem verifizierten Namen
  `factur-x.xml` wird zuerst versucht, danach alle übrigen; der erste,
  der sich als EN-16931-XML lesen lässt, gewinnt. Das Parsen ist die
  belastbarste Prüfung — ein Anhang, der als gültige CII- oder
  UBL-Rechnung durchgeht, IST die Rechnung, unabhängig vom Dateinamen.
  Damit war keine geratene Namensliste nötig.

## 2026-10-13 — Zeiterfassung (F.1.1, ADR 0024) — alle offen, keine blockiert F.1.2-F.1.5

- **F.1 (Löschfrist für Zeitdaten)**: §16 Abs. 2 ArbZG verlangt
  mindestens zwei Jahre Aufbewahrung. Dienen die Zeiten zugleich als
  Grundlage der Lohnabrechnung, gelten für Lohnunterlagen längere
  Fristen. Welche Frist soll das System anwenden, und soll nach Ablauf
  gelöscht oder anonymisiert werden? Bis zur Klärung implementiert F.1
  keinen Löschpfad. Vor produktiver Nutzung zu klären (DSGVO Art. 5
  Abs. 1 lit. e).
- **F.1 (MiLoG §17 / Branchenzuordnung)**: Für Betriebe der in §2a
  SchwarzArbG genannten Branchen (u. a. Baugewerbe) und für
  geringfügig Beschäftigte müssen Beginn, Ende und Dauer spätestens
  sieben Tage nach der Arbeitsleistung aufgezeichnet sein. Fällt der
  Betrieb (insbesondere mit Montage auf Baustellen) darunter? Wenn ja,
  sollte ein verspäteter Nachtrag als Hinweis ausgewiesen werden
  (→ Backlog F.6). F.1 speichert `created_at` je Eintrag, die
  Information geht also nicht verloren.
- **F.1 (abweichende Regelungen)**: Die Hinweise in F.1 verwenden nur
  die gesetzlichen Grundwerte (8 h/10 h, 30/45 min, 11 h). Gibt es
  einen Tarifvertrag oder eine Betriebsvereinbarung mit Abweichungen
  nach §7 ArbZG, und werden Jugendliche beschäftigt (JArbSchG)?
  → Backlog F.6.
- **F.1 (Zeitzone)**: ADR 0024 nimmt eine einzige Zeitzone
  (`Europe/Berlin`) für die Zuordnung eines Eintrags zum Arbeitstag an.
  Gibt es Standorte oder Montageeinsätze außerhalb dieser Zeitzone?

## 2026-10-13 — HR-Bestand an HTTP (F.4.1, ADR 0025)

- **F.4.3 (Standardwert `active` bei Mitarbeiteranlage)**: Fortsetzung
  der seit 0.1.3.1 offenen Frage weiter oben — heute ist ein neuer
  Mitarbeiter inaktiv, wenn `active` nicht ausdrücklich gesetzt wird.
  Status: **beantwortet** am 2026-10-13. Entscheidung des Nutzers:
  "Ja, neue Mitarbeiter sollten aktiv sein." → Umsetzung in F.4.3.
  Damit ist auch die Detailfrage zu 0.1.3.1 erledigt.
- **F.4 (Löschung von Mitarbeiterdaten nach Ausscheiden)**: Es gibt
  keinen Löschpfad und F.4 baut keinen. Nach welcher Frist und in
  welcher Form (Löschen, Anonymisieren) sollen Stammdaten
  ausgeschiedener Mitarbeiter entfernt werden? Hängt mit der offenen
  Löschfrist für Zeitdaten zusammen (F.1). Status: **offen**, kein
  Blocker für F.4.
- **F.4.5 (Entscheidung über den eigenen Urlaubsantrag)**: Darf jemand
  mit `leave_requests.decide` seinen eigenen Antrag genehmigen? ADR
  0025 erzwingt dazu nichts, weil keine Vorgabe vorliegt. Status:
  **offen**, kein Blocker (lässt sich nachträglich als Prüfung
  ergänzen).

## 2026-10-13 — Personaldaten in Logs (0.46, ADR 0026)

- **0.46 (Auslegung "keine Personaldaten in Logs")**: `aufgabe.md` §2
  verlangt "Keine Personaldaten in Logs". Nach 0.46 stehen keine Werte
  von Query-Parametern mehr im Request-Log. Weiterhin geloggt werden
  (a) IDs im Pfad wie `/employees/<uuid>` — pseudonyme technische
  Bezeichner, ohne die sich ein Fehler keinem Datensatz zuordnen
  lässt — und (b) `remote_ip` und `user_agent` — die IP ist
  personenbezogen, aber für die Nachverfolgung von Angriffen der
  entscheidende Wert. Frage: sind (a) und (b) mit der Vorgabe
  vereinbar, und wie lange dürfen Logs aufbewahrt werden? Status:
  **offen**, kein Blocker.
- **0.46 (bestehende Logdateien)**: Bis zum 2026-10-13 geschriebene
  Logs enthalten Query-Werte, darunter Suchbegriffe aus der
  Kontaktsuche (Namen). Sollen vorhandene Logdateien bereinigt oder
  gelöscht werden? Betriebliche Entscheidung außerhalb des Repos.
  Status: **offen**, kein Blocker — aber vor einem Produktivbetrieb
  mit Echtdaten zu klären, falls schon Logs mit Echtdaten existieren.

## 2026-10-13 — Qualifikationen und Unterweisungen (F.2.1, ADR 0027) — keine blockiert

- **F.2 (Bestätigung durch den Mitarbeiter)**: Status: **beantwortet**
  am 2026-10-13. Entscheidung des Nutzers: "Die Personalabteilung
  trägt ein, welche Qualifikationen erreicht sind und wann sie
  erreicht wurden. Eine anstehende Erneuerung sollte vom System
  angemahnt werden." → kein Selbsteintrag, keine Bestätigung durch den
  Mitarbeiter; Anmahnen über Status und Fälligkeitsliste (ADR 0027,
  Ergänzung).
- **F.2 ("anmahnen" = aktive Benachrichtigung?)**: Umgesetzt wird der
  Status `laeuft_ab`/`abgelaufen` und eine Liste der anstehenden
  Erneuerungen. Eine E-Mail oder andere Zustellung gibt es nicht — das
  Repo hat dafür keine Infrastruktur. Genügt die Liste (sichtbar,
  sobald es eine Oberfläche gibt), oder soll das System von sich aus
  benachrichtigen? Status: **offen**, kein Blocker; → Backlog F.10.
- **F.2 (Fristen und Umfang des Vorschlagskatalogs)**: Die Fristen
  stammen aus einer Websuche über Sekundärquellen, sechs Themen sind
  gar nicht recherchiert. Vor produktiver Nutzung mit der Fachkraft
  für Arbeitssicherheit bzw. der Berufsgenossenschaft abgleichen. Die
  Fristen sind je Mandant änderbar; ein falscher Vorschlagswert ist
  also korrigierbar, aber er wäre zunächst unbemerkt im Einsatz.
  Status: **offen**, kein Blocker.
- **F.2 (Pflicht je Tätigkeit)**: Status: **beantwortet** am
  2026-10-13. Entscheidung des Nutzers: "Beim Anlegen eines neuen
  Mitarbeiters soll die Personalabteilung festlegen können, welche
  Qualifikationen für die Stelle nötig sind und ob sie schon
  vorhanden sind." → Anforderungen je Mitarbeiter (Tabelle seit
  Migration 088), `fehlt` gilt dafür und für `pflicht_fuer_alle`.
  Über die API in drei Schritten (Mitarbeiter, Anforderungen,
  Nachweise), die die Oberfläche in einem Dialog zusammenfasst.

## 2026-10-15 — Asset-Zuordnung (F.3.1, ADR 0028) — alle beantwortet am 2026-10-15

- **F.3 (Prüfungen der Arbeitsmittel)**: Sollen Prüffristen (DGUV
  Vorschrift 3 für Elektrowerkzeug, Leitern, PSA gegen Absturz) Teil
  von F.3 sein, mit Anmahnen wie bei den Qualifikationen? Vorschlag:
  ja; nicht bestandene Prüfung sperrt die Ausgabe, überfällige Prüfung
  gibt nur einen Hinweis. Status: **beantwortet** — "Ja, soll Teil von F.3 sein".
- **F.3 (Verbrauchs-PSA)**: Handschuhe, Brillen, Schuhe ohne Rückgabe
  — in F.3 oder später über das Lager? Vorschlag: später über das
  Lager; F.3 nur Einzelstücke mit Inventarnummer. Status: **beantwortet** —
  "Vorschlag umsetzen" → Backlog F.12.
- **F.3 (wer gibt aus)**: Vorschlag Personalabteilung, Lager,
  Administrator. Status: **beantwortet** — "Erstmal nur Personalabteilung,
  später eventuell dem Lager das Recht aber einräumen können."
- **F.3 (Empfangsbestätigung)**: Vorschlag wie F.2 — die ausgebende
  Stelle erfasst, keine Bestätigung im System. Status: **beantwortet** —
  "Nein, keine Bestätigung vorerst."

## 2026-10-15 — Prüfungen der Betriebsmittel (F.3.5.1) — keine blockiert

- **F.3.5 (Ergebnis "Mängel")**: Annahme im Code — ein Stück mit dem
  Prüfergebnis "Mängel" bleibt ausgebbar und wird nur als Hinweis
  angezeigt; gesperrt ist nur "nicht bestanden". Soll "Mängel" ebenfalls
  sperren? Status: **beantwortet** am 2026-10-19 — "ja, Mängel sollen
  Ausgabe sperren" → umgesetzt in F.3.7.
- **F.3.5 (Vorwarnzeit)**: Annahme 30 Tage für "Prüfung bald fällig"
  (Qualifikationen: 60). Der Wert ist beim Abruf änderbar. Passt 30?
  Status: **beantwortet** am 2026-10-19 — "30 Tage reichen" (keine
  Änderung).

## 2026-10-19 — F.8 Abwesenheiten und Urlaubskonto (offen)

Details und Begründung in `docs/adr/0029-abwesenheiten-urlaubskonto.md`.

1. Feiertage berechnen, Bundesland am Mandanten (am Mitarbeiter
   überschreibbar), betriebliche Zusatztage pflegbar? Empfehlung: ja.
   Status: offen.
2. Jahreskonto mit Jahresanspruch am Mitarbeiter, Teilurlaub als
   Vorschlag; Vorgabe für neue Mitarbeiter? Empfehlung: ja, Vorgabe als
   Mandanten-Einstellung mit Startwert 30. Status: offen.
3. Kein automatischer Verfall, Übertrag durch die Personalabteilung,
   Nachweisfeld "Hinweis erteilt am"? Empfehlung: ja. Status: offen.
4. Krankheit direkt als Abwesenheit ohne Diagnose, "Attest ja/nein",
   Rückbuchung nach §9 BUrlG; Selbstmeldung durch den Mitarbeiter?
   Empfehlung: ja, Selbstmeldung erlaubt. Status: offen.
5. Teamleiter genehmigt Anträge seines Teams; Kollegen sehen
   Abwesenheiten im Team? Empfehlung: beides vorerst nein. Status: offen.
