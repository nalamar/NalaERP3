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
  ist in aufgabe.md §2 fix vorgegeben) — wird bei Erreichen von Epic E erneut
  geprüft.
- **0.1.3.1 (hr: Default-Wert für `Employee.Active` bei Anlage)**: Beim Beheben
  des No-Op-Bugs (`server/internal/hr/service.go:98-100`) aufgefallen: neue
  Mitarbeitende erhalten `Active=false`, sofern der Aufrufer es nicht explizit
  auf `true` setzt (Go-Zero-Value für `bool`). Ob ein neu angelegter
  Mitarbeiter fachlich standardmäßig aktiv sein soll, ist unklar und wurde
  NICHT geändert (reine Bugfix-Subtask, kein Verhaltenswechsel ohne Freigabe).
  Kein Blocker für die laufende Subtask-Kette, aber vor Abschluss von Task
  0.1.3 zu klären.
