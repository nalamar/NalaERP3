# ADR 0024 — Zeiterfassung (revisionssicher, ArbZG/BAG-konform)

Datum: 2026-10-13
Status: entschieden (Subtask F.1.1)
Bezug: Backlog F.1 (Epic F — Personal & HR, erste Task).

## Kontext

`aufgabe.md` §2 fordert: "Arbeitszeit: Erfassung revisionssicher,
Anforderungen aus ArbZG und BAG-Rechtsprechung berücksichtigen", dazu
DSGVO (Zweckbindung, Löschfristen, keine Personaldaten in Logs).

**Ist-Stand im Repo (belegt):**

- Es gibt keine Zeiterfassung. Suche nach `time_entr`/`zeiterfass`/
  `arbeitszeit` über `server/internal` liefert keinen Treffer;
  `docs/01-gap-analysis.md:93` führt sie als "offen".
- `server/internal/hr/service.go` kennt nur `EmployeeService` (Stammdaten,
  Teams) und `LeaveService` (Urlaubsanträge). Tabellen: `hr_employees`,
  `hr_teams`, `hr_leave_requests`, `hr_absences`, `hr_holidays`
  (`020_hr_base.sql`), Mandanten-Scoping über `hr_employees.company_id`
  (`059_hr_scope.sql`); Kindtabellen erben den Scope über ihr
  verpflichtendes `employee_id` (ADR 0002).
- **Das Paket `hr` ist an keinen HTTP-Handler angebunden.** Weder
  `internal/hr` wird außerhalb des Pakets importiert, noch gibt es in
  `server/internal/http/v1.go` eine Route mit HR-Bezug. Es existiert auch
  keine einzige HR-Permission — nur die Rolle `role-hr`
  (`017_auth.sql:70`) ohne zugeordnete Rechte.
- **Es gibt keine Verknüpfung zwischen Login-Benutzer und Mitarbeiter.**
  `users` (`017_auth.sql:1`) und `hr_employees` sind unverbunden. Ohne
  diese Verknüpfung kann das System nicht wissen, für wen ein
  angemeldeter Benutzer stempelt.
- Das Repo verwendet keine DB-Trigger (kein `CREATE TRIGGER`/
  `CREATE FUNCTION` in den Migrationen). Unveränderbarkeit ist bisher
  durchgängig Sache des Anwendungscodes (z. B. Storno von
  Ausgangsrechnungen, `062_invoices_out_storno.sql`).

**Rechtlicher Rahmen (nur was das Datenmodell bestimmt):**

- BAG, Beschluss vom 13.09.2022 – 1 ABR 22/21, im Anschluss an EuGH,
  Urteil vom 14.05.2019 – C-55/18: der Arbeitgeber muss ein System
  einführen, mit dem Beginn, Ende und damit Dauer der **gesamten**
  Arbeitszeit einschließlich Überstunden erfasst wird; das System muss
  objektiv, verlässlich und zugänglich sein.
- ArbZG §3: werktäglich 8 Stunden, verlängerbar auf 10 Stunden bei
  Ausgleich im Schnitt von 6 Kalendermonaten bzw. 24 Wochen.
- ArbZG §4: Ruhepause mindestens 30 Minuten bei mehr als 6 bis 9
  Stunden, 45 Minuten bei mehr als 9 Stunden; Abschnitte von mindestens
  15 Minuten; nicht länger als 6 Stunden ohne Pause.
- ArbZG §5: ununterbrochene Ruhezeit von mindestens 11 Stunden.
- ArbZG §16 Abs. 2: Aufzeichnungen mindestens zwei Jahre aufbewahren.

## Optionen

### 1. Was wird erfasst — Dauer oder Beginn/Ende?

**A — nur Tagesdauer** (Datum + Stunden). Verworfen: die
BAG-Entscheidung verlangt Beginn und Ende; Pausen- und
Ruhezeitprüfung (§§4, 5 ArbZG) sind aus einer reinen Dauer nicht
ableitbar.

**B — Beginn und Ende als Zeitpunkte, Pause als Minuten.** Gewählt.

Ein Eintrag darf offen sein (`ended_at IS NULL`), damit Kommen/Gehen in
der Werkstatt gestempelt werden kann; derselbe Datensatz deckt den
manuellen Nachtrag (Beginn und Ende zusammen) ab. Zwei getrennte
Modelle für Stempeln und Nachtrag wären doppelter Code für dieselbe
Aussage.

Pausen werden auf zwei Wegen berücksichtigt: als `break_minutes` am
Eintrag (entspricht der Zettelpraxis und dem Nachtrag) **und** als Lücke
zwischen zwei Einträgen desselben Tages (entspricht dem Ausstempeln zur
Pause). Bekannte Grenze: für `break_minutes` ist die
15-Minuten-Abschnittsregel aus §4 Satz 2 nicht prüfbar, weil nur die
Summe vorliegt.

### 2. Revisionssicherheit — wie werden Korrekturen abgebildet?

**A — Einträge frei änderbar, Änderungen in `entity_change_log`
protokollieren.** Verworfen: der Originalwert läge dann nur noch als
JSON im Protokoll, die Fachtabelle zeigte stets nur den letzten Stand.
Eine Auswertung "was war ursprünglich erfasst" wäre eine Protokollsuche
statt einer Abfrage. Zudem hat 0.3.3.5 HR bewusst nicht an das
Änderungsprotokoll angebunden.

**B — unveränderliche Einträge, Korrektur durch Storno und
Ersatzeintrag.** Gewählt. Dasselbe Prinzip wie überall sonst im Repo
("Storno statt Änderung", `aufgabe.md` §2). Ein abgeschlossener Eintrag
wird nie geändert und nie gelöscht. Eine Korrektur storniert den alten
Eintrag (Pflichtbegründung, wer, wann) und legt in derselben
Transaktion einen neuen an, der über `replaces_entry_id` auf den alten
zeigt. Original, Korrektur und Urheber bleiben als Zeilen sichtbar.

Genau zwei Zustandsübergänge sind erlaubt, beide nur einmal:
offen → abgeschlossen (`ended_at` wird gesetzt) und aktiv → storniert.

**C — wie B, zusätzlich per DB-Trigger erzwungen.** Nicht gewählt, aber
ausdrücklich nicht als schlechter bewertet: ein Trigger schützt auch
gegen Direkt-SQL. Er wäre jedoch der erste Trigger im Repo und würde
ausgerechnet Zeitdaten strenger schützen als Rechnungen. Wenn
DB-seitige Unveränderbarkeit gewünscht ist, gehört sie als eigene,
domänenübergreifende Entscheidung getroffen (→ neue Backlog-Position
0.45), nicht nebenbei in F.1.

### 3. ArbZG-Verstöße — ablehnen oder ausweisen?

**A — Einträge ablehnen, die gegen das ArbZG verstoßen** (z. B. mehr
als 10 Stunden). Verworfen, und zwar aus rechtlichen Gründen: erfasst
werden muss die **tatsächlich geleistete** Arbeitszeit. Ein System, das
einen 11-Stunden-Tag nicht annimmt, erzwingt eine falsche Aufzeichnung
— der Verstoß verschwände aus den Daten, nicht aus der Welt. Das wäre
das Gegenteil von "objektiv und verlässlich".

**B — alles Tatsächliche annehmen, Verstöße als Hinweise ausweisen.**
Gewählt. Abgelehnt wird nur, was nicht stimmen **kann**: Ende vor
Beginn, Pause länger als die Anwesenheit, Überschneidung mit einem
anderen aktiven Eintrag desselben Mitarbeiters, zweiter offener Eintrag,
Beginn in der Zukunft. Alles andere wird gespeichert und in der
Tagesauswertung gekennzeichnet:

| Hinweis | Regel |
|---|---|
| `ueber_8h` | Nettoarbeitszeit des Tages > 8 h (§3 Satz 1, ausgleichspflichtig) |
| `ueber_10h` | Nettoarbeitszeit des Tages > 10 h (§3 Satz 2) |
| `pause_zu_kurz` | > 6 h mit < 30 min Pause bzw. > 9 h mit < 45 min (§4) |
| `ruhezeit_unterschritten` | < 11 h zwischen Ende und nächstem Beginn (§5) |

Die Hinweise werden bei jedem Lesen berechnet und nicht gespeichert —
sie sind eine Ableitung aus den Einträgen, kein eigener Sachverhalt.
Zugrunde liegen ausschließlich die gesetzlichen Grundwerte. Der
Ausgleichszeitraum nach §3 Satz 2, tarifliche Abweichungen (§7),
Sonn-/Feiertagsarbeit (§9) und das JArbSchG sind nicht Teil von F.1.

### 4. Für wen darf ein Benutzer erfassen?

Entscheidung: additive Spalte `hr_employees.user_id` (nullable, UNIQUE,
FK auf `users`). Drei neue Permissions:

- `time_entries.self` — eigene Zeiten erfassen und lesen. Setzt voraus,
  dass dem Benutzer ein Mitarbeiter zugeordnet ist. Zugewiesen an alle
  Systemrollen.
- `time_entries.read` — Zeiten aller Mitarbeiter des Mandanten lesen.
- `time_entries.write` — für andere erfassen, stornieren, korrigieren.

`read`/`write` gehen an `role-hr` und `role-admin`. Wiederverwendung
einer bestehenden Permission scheidet aus: es gibt keine mit HR-Bezug.

Eine Korrektur eigener Zeiten ist mit `time_entries.self` möglich
(sonst könnte ein Tippfehler nur über die Personalabteilung behoben
werden); sie bleibt durch die Storno-Kette vollständig nachvollziehbar.

### 5. Projektbezug

Entscheidung: `project_id` als optionale Spalte (nullable FK auf
`projects`, `ON DELETE SET NULL`). ADR 0020 hält ausdrücklich fest,
dass Projekten die Ist-Lohnseite fehlt, weil es keine Zeiterfassung
gibt; Epic H (BDE) braucht denselben Bezug. Die Spalte jetzt
mitzuführen kostet nichts, sie später nachzurüsten hieße, bereits
erfasste Zeiten ohne Projekt zu haben. **Nicht** Teil von F.1: jede
Bewertung in Geld, Lohnbuchung oder Einbindung ins Projektcontrolling.

## Entscheidung

**`hr_time_entries`** (neue Tabelle):

| Spalte | Typ | Bemerkung |
|---|---|---|
| `id` | uuid PK | |
| `employee_id` | uuid NOT NULL → `hr_employees` | **`ON DELETE RESTRICT`**, siehe unten |
| `work_date` | date NOT NULL | Kalendertag des Beginns |
| `started_at` | timestamptz NOT NULL | |
| `ended_at` | timestamptz NULL | NULL = läuft |
| `break_minutes` | integer NOT NULL DEFAULT 0 | CHECK ≥ 0 |
| `project_id` | uuid NULL → `projects` | `ON DELETE SET NULL` |
| `note` | text NOT NULL DEFAULT '' | |
| `status` | text NOT NULL DEFAULT 'active' | CHECK in (`active`, `storniert`) |
| `storno_reason` | text NOT NULL DEFAULT '' | Pflicht bei Storno |
| `storniert_at` | timestamptz NULL | |
| `storniert_by_user_id` | text NULL → `users` | `ON DELETE SET NULL` |
| `replaces_entry_id` | uuid NULL → `hr_time_entries` | Korrekturkette |
| `created_by_user_id` | text NULL → `users` | `ON DELETE SET NULL` |
| `created_at` | timestamptz NOT NULL DEFAULT now() | |

Zusätzlich: CHECK `ended_at IS NULL OR ended_at > started_at`;
partieller UNIQUE-Index auf `(employee_id) WHERE ended_at IS NULL AND
status='active'` (höchstens ein laufender Eintrag je Mitarbeiter, von
der Datenbank garantiert, nicht nur vom Code).

- **`ON DELETE RESTRICT` statt `CASCADE`.** `hr_leave_requests` und
  `hr_absences` hängen mit `CASCADE` an `hr_employees`. Für Zeitdaten
  wäre das falsch: das Löschen eines Mitarbeiters würde
  aufbewahrungspflichtige Aufzeichnungen (§16 Abs. 2 ArbZG) still
  mitlöschen.
- **Kein eigenes `company_id`**: Scope über `employee_id`, wie
  `hr_leave_requests` (ADR 0002). Alle Lese- und Schreibpfade prüfen
  die Mandantenzugehörigkeit des Mitarbeiters.
- **`work_date` = Kalendertag des Beginns in `Europe/Berlin`.** Eine
  Nachtschicht über Mitternacht gehört zum Tag, an dem sie beginnt.
  **Annahme**: eine Zeitzone für das ganze System (wie `TZ` in
  `docker-compose.yml`); es gibt kein Zeitzonenfeld je Mandant oder
  Standort.
- **Kein Löschpfad.** F.1 implementiert weder `DELETE` noch eine
  automatische Löschung nach Fristablauf. Die Löschfrist ist fachlich
  ungeklärt (→ `docs/open-questions.md`); bis dahin wird nichts
  gelöscht, was nicht wiederherstellbar wäre.
- **Keine Zeitdaten in Logs.** Fehlermeldungen nennen weder Namen noch
  Zeiten, höchstens die Eintrags-ID.
- **Implementierung im Paket `hr`**, neue Datei `hr/time_entries.go`
  (`TimeEntryService`), analog zu `accounting/cost_centers.go`.
  `EmployeeService`/`LeaveService` bleiben unangetastet bis auf die
  neue Spalte `user_id`.

## Konsequenzen

Der Umfang überschreitet §6.3 deutlich. Zerlegung:

- **F.1.2** Migration `086_hr_time_entries.sql`: Tabelle,
  `hr_employees.user_id`, drei Permissions samt Rollenzuordnung.
  Additiv, reversibel.
- **F.1.3** Fachregeln ohne Datenbank: Validierung eines Eintrags und
  die ArbZG-Tagesauswertung als reine Funktionen, mit Tests für jede
  Regel der Tabelle oben einschließlich der Grenzwerte.
- **F.1.4** `TimeEntryService`: Beginnen, Beenden, Nachtragen,
  Stornieren, Korrigieren, Lesen — mit Integrationstests, darunter
  Mandantentrennung und der Nachweis, dass kein Pfad einen
  abgeschlossenen Eintrag verändert.
- **F.1.5** HTTP-Anbindung samt Rechteprüfung (`self` gegen
  `read`/`write`), End-to-End-Integrationstest.

Neue Backlog-Positionen aus der Recherche (nicht Teil von F.1):

- **F.4** `EmployeeService`/`LeaveService` haben keinen HTTP-Endpunkt
  und keine Permissions; `client/lib/pages/employees_page.dart:29` ruft
  `api.listEmployees` auf, das es in `client/lib/api.dart` nicht gibt
  (die Seite wird nirgends eingebunden).
- **F.5** Client-Oberfläche für die Zeiterfassung.
- **F.6** Erweiterte ArbZG-Auswertung: Ausgleichszeitraum §3 Satz 2,
  Sonn-/Feiertagsarbeit, tarifliche Abweichungen.
- **0.45** DB-seitige Unveränderbarkeit (Trigger) für festgeschriebene
  Belege und Zeitdaten als domänenübergreifende Entscheidung.

Kein Einfluss auf bestehende Daten oder Lesepfade: alles ist additiv,
`hr_employees.user_id` ist nullable.
