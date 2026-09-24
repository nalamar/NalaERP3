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
