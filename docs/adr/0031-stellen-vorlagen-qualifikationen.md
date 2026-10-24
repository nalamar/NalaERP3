# ADR 0031 — Stellen als Vorlagen für erforderliche Qualifikationen

Datum: 2026-10-24
Status: **Entwurf — wartet auf Entscheidungen des Nutzers**
(`docs/open-questions.md`, 2026-10-24). Kein Produktivcode vor der Antwort.
Bezug: Backlog F.11; Wunsch des Nutzers vom 2026-10-13: "Vorlagen für
Mitarbeiter-Stellen generieren, die bestimmte Qualifikationen erfordern";
ADR 0027 (Qualifikationen, Anforderungen je Mitarbeiter).

## Kontext (Ist-Stand, belegt)

- Anforderungen gibt es nur **je Mitarbeiter**:
  `hr_employee_qualification_requirements (employee_id, type_id, …)`,
  Migration 088. Sie sind Planung, kein Beleg (ADR 0027, Ergänzung 2).
- Der Stand eines Mitarbeiters (`fehlt`/`abgelaufen`/`läuft ab`/`gültig`)
  zählt jede Art, die ihm als Anforderung zugeordnet **oder** für alle
  Pflicht ist (`pflicht_fuer_alle`), `server/internal/hr/
  qualification_requirements.go` (`Standing`) und `qualification_due.go`
  (`DueList`, betriebsweite Fälligkeitsliste).
- Die Stelle ist im Mitarbeiterstamm **Freitext** (`hr_employees.role`,
  im Client "Stelle / Rolle"); nichts im Code wertet sie aus.
- Seit F.10.5 werden Anforderungen und vorhandene Nachweise beim Anlegen
  in einem Aufruf gespeichert (`POST /employees/with-qualifications`);
  die Personalabteilung wählt dabei jede Art einzeln.
- Der Qualifikationskatalog hat einen im Code gepflegten
  Vorschlagskatalog, den ein Mandant ausdrücklich übernimmt
  (`qualification_defaults.go`, `AdoptDefaults`, ergänzt nur Fehlendes).
- ADR 0027 hat eine Anforderungsmatrix je **Stelle oder Tätigkeit**
  ausdrücklich offengelassen (→ F.11 bzw. Epic H).

## Zu entscheiden

### 1. Stellen als Stammdaten

**Empfehlung**: eigene Stammdaten je Mandant — Stelle (Name, eindeutig
je Mandant, aktiv/stillgelegt, nie gelöscht, solange ein Mitarbeiter sie
hat) und Zuordnung Stelle → Qualifikationsarten. Pflege mit
`qualifications.write`, Lesen mit `qualifications.read` (wie der
Katalog).

### 2. Kopieren oder dauerhaft gelten?

- **A — Kopieren.** Beim Zuweisen werden die Arten der Stelle in die
  Anforderungen des Mitarbeiters übernommen. Spätere Änderungen an der
  Stelle wirken nicht auf bestehende Mitarbeiter. Einfach; nichts am
  Stand ändert sich. **Nachteil**: verlangt die Fachkraft für
  Arbeitssicherheit für alle Schweißer eine neue Unterweisung, erscheint
  sie bei den vorhandenen Schweißern **nicht** als "fehlt" — die
  Personalabteilung müsste jeden einzeln nachpflegen. Diese Lücke fällt
  niemandem auf.
- **B — Dauerhaft gelten (verknüpft).** Ein Mitarbeiter hat eine oder
  mehrere Stellen; erforderlich ist, was seine Stellen verlangen, dazu
  seine eigenen Zusatzanforderungen und `pflicht_fuer_alle`. Eine
  Änderung an der Stelle wirkt sofort auf alle mit dieser Stelle — im
  Stand, in der Fälligkeitsliste, in der Anzeige. Mehr Umbau
  (`Standing`, `DueList`, Anforderungsanzeige, Anlage aus F.10.5).
- **C — Kopieren mit ausdrücklichem Nachziehen** ("Änderung an alle
  Mitarbeiter dieser Stelle übertragen?"). Mittelweg, aber zwei
  Wahrheiten, die auseinanderlaufen können.

**Empfehlung: B.** Begründung: die Stelle ist die fachliche Ursache der
Anforderung. Eine neue Pflicht für eine Stelle muss bei allen Inhabern
sofort sichtbar werden; bei A bliebe sie still aus — bei
Sicherheitsunterweisungen der gefährlichere Fehler. Individuelle
Zusatzanforderungen bleiben über die bestehende Tabelle möglich.
Entfernt man eine Art aus der Stelle, ist sie für deren Inhaber nicht
mehr erforderlich (es sei denn, sie ist ihnen einzeln zugeordnet);
Nachweise bleiben unberührt.

### 3. Eine oder mehrere Stellen je Mitarbeiter?

"Monteur" und "Staplerfahrer" sind oft dieselbe Person. **Empfehlung:
mehrere** — die Anforderungen ergeben sich aus der Vereinigung. Bei nur
einer Stelle müsste man Mischstellen ("Monteur mit Stapler") anlegen.

### 4. Verhältnis zum Freitext `role`

**Empfehlung**: `role` bleibt unverändert als freie Berufsbezeichnung
(keine Datenänderung, keine Migration der Werte). Die Stellen sind ein
eigenes Feld; die Anzeige zeigt beide. Eine spätere Zusammenführung
wäre eine eigene Entscheidung.

### 5. Was heißt "generieren"?

- **a — Vorschlagskatalog** typischer Metallbau-Stellen, den ein
  Mandant übernimmt wie den Qualifikationskatalog, z. B. Monteur
  (Arbeitsschutz, PSA gegen Absturz, Leitern, Hubarbeitsbühne),
  Schweißer (Schweißerprüfung, Brandschutzhelfer, Gefahrstoffe),
  Lagerist (Stapler: Ausbildung, Unterweisung, Beauftragung;
  Ladungssicherung), Staplerfahrer, Kranführer/Anschläger. Zuordnung
  über die Codes des Vorschlagskatalogs; nur Arten, die der Mandant hat.
  **Nicht recherchiert** — abgeleitet aus dem Katalog aus ADR 0027, vom
  Betrieb abzugleichen.
- **b — Aus vorhandenen Mitarbeitern ableiten**: Mitarbeiter mit
  gleichem `role`-Text gruppieren, Vorschlag = Arten, die alle (oder die
  meisten) von ihnen als Anforderung haben; die Personalabteilung
  bestätigt.
- **c — Von Hand anlegen.**

**Empfehlung**: c und a zuerst; b als eigener, späterer Schritt (lohnt
erst, wenn genug Mitarbeiter mit Anforderungen erfasst sind).

### 6. Schnittstelle zu Epic H (Einsatzfähigkeit je Tätigkeit)

Eine Stelle sagt, was ein Mitarbeiter **dauerhaft** vorweisen muss. Die
Einsatzfähigkeit fragt, ob er **eine bestimmte Arbeit** übernehmen darf
("darf er morgen die Bühne fahren?"). **Empfehlung**: F.11 baut nur
Stellen; Epic H entscheidet, ob es dieselbe Zuordnung Stelle → Arten
nutzt oder Tätigkeiten eigens führt. Das Datenmodell aus F.11 legt
dafür nichts fest.

## Folgen bei Entscheidung wie empfohlen (Vorschau, nicht beschlossen)

Grobe Teilung, je mit eigener Migration und Tests: Stellen und
Zuordnung (Server) · Stellen am Mitarbeiter und Stand/Fälligkeit als
Vereinigung (Server) · Pflege der Stellen und Auswahl beim Anlegen/
Bearbeiten (Client) · Vorschlagskatalog. Die Anlage aus F.10.5 nimmt
dann zusätzlich Stellen an.
