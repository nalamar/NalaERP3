# ADR 0025 — HR-Bestand an HTTP anbinden (Mitarbeiterstamm, Urlaubsanträge, Benutzer-Verknüpfung)

Datum: 2026-10-13
Status: entschieden (Subtask F.4.1); der offene Punkt wurde am 2026-10-13 freigegeben
Bezug: Backlog F.4. Vom Nutzer am 2026-10-13 vor F.2/F.3 gezogen.

## Kontext

`EmployeeService` und `LeaveService` (`server/internal/hr/service.go`)
existieren seit dem Ursprungsstand, wurden in Epic 0 getestet und
gehärtet (0.10-0.12) und sind von außen nicht erreichbar: bis F.1.5 gab
es keine einzige HTTP-Route mit HR-Bezug. Seit F.1 hängt daran ein
konkreter Schaden: die Zeiterfassung ist fertig, aber kein Mitarbeiter
kann selbst stempeln, weil sich `hr_employees.user_id` nur per
Direkt-SQL setzen lässt.

**Bestandsaufnahme (belegt). Die Services sind nicht HTTP-reif** — sie
einfach durchzureichen würde mehrere Fehler nach außen tragen:

| # | Befund | Beleg | Folge über HTTP |
|---|---|---|---|
| 1 | `EmployeeService.Get` reicht `pgx.ErrNoRows` roh durch | `service.go:109-113` | unbekannte ID → 500 statt 404 |
| 2 | `EmployeeService.Update` prüft `RowsAffected` nicht | `service.go:150-151` | Änderung an fremdem/unbekanntem Mitarbeiter → 200, nichts passiert |
| 3 | `personalnummer` ist GLOBAL eindeutig, nicht je Mandant | `020_hr_base.sql:10`, Constraint `hr_employees_personalnummer_key` | zwei Mandanten können nicht beide "1001" vergeben; der Mandant erfährt so auch, welche Nummern ein anderer nutzt |
| 4 | keine Vorabprüfung auf doppelte Personalnummer | `service.go:94-99` | roher UNIQUE-Fehler → 500, und die Fehlermeldung von Postgres (mit der Personalnummer im `DETAIL`) landet im Log |
| 5 | `team_id` wird nur per FK geprüft | `service.go:94`, `:150` | Mitarbeiter kann einem Team eines ANDEREN Mandanten zugeordnet werden |
| 6 | `LeaveService.Approve` hat keine Statusprüfung | `service.go:233-234` | ein entschiedener Antrag lässt sich beliebig oft neu entscheiden, die erste Entscheidung wird überschrieben |
| 7 | `approver_id` ist ein MITARBEITER (FK auf `hr_employees`), `Approve` verlangt ihn zwingend | `020_hr_base.sql:49`, `service.go:227` | wer entscheidet, muss selbst Mitarbeiter sein — ein HR-Login ohne Mitarbeiterdatensatz kann nicht entscheiden |
| 8 | Genehmigung erzeugt keinen Eintrag in `hr_absences` | Suche: `hr_absences`/`hr_holidays` kommen in keinem Go-Code vor | die Tabellen sind angelegt und ungenutzt |
| 9 | Neuer Mitarbeiter ist ohne ausdrückliches `active` INAKTIV | Go-Nullwert, `service.go:27`, `:94`; offene Frage seit 0.1.3.1 | über HTTP angelegte Mitarbeiter wären standardmäßig inaktiv |
| 10 | Es gibt keine HR-Permission außer den dreien aus F.1.2 | `017_auth.sql`, `086_hr_time_entries.sql` | — |
| 11 | `client/lib/pages/employees_page.dart` ruft `listEmployees`, `createEmployee`, `getEmployee`, `updateEmployee`, `listTeams` auf; keine davon existiert in `client/lib/api.dart`, die Seite wird nirgends eingebunden; die Git-Historie kennt weder die Methoden noch eine Server-Route dazu | Suche, `git log -S` | die Seite hat in diesem Repo nie funktioniert |

Weitere, hier nur festgehaltene Beobachtungen: `Employee.Role` und
`Employee.CostCenter` sind Freitext (für Kostenstellen gibt es seit E.1
eine Stammdatentabelle); `EmployeeService` kann Teams nur lesen, nicht
anlegen; `hire_date`/`termination_date`/`personalnummer` sind nach der
Anlage nicht änderbar (`employeeUpdatableFields`).

## Optionen

### 1. Umfang

**A — nur die Benutzer-Verknüpfung** (kleinster Schritt, der das
Stempeln freischaltet). Verworfen: ohne Lese-Endpunkt für Mitarbeiter
kann niemand die ID nachschlagen, die er verknüpfen will; das wäre ein
Endpunkt, der nur mit Datenbankzugriff benutzbar ist.

**B — Mitarbeiterstamm und Verknüpfung, Urlaubsanträge getrennt
danach.** Gewählt, als Reihenfolge innerhalb von F.4: zuerst der Stamm
(Voraussetzung für alles andere in Epic F), dann die Urlaubsanträge.
Beides bleibt F.4, aber als eigene Subtasks mit eigenem Abschluss.

**C — alles inklusive Client-Seite, Teamverwaltung, `hr_absences`.**
Verworfen als Scope-Creep: die Client-Seite ist ein eigenes Thema
(→ Backlog F.7), Team-Anlage und Abwesenheiten sind neue Funktionen,
keine Anbindung des Bestands (→ Backlog F.8).

### 2. Rechte

Wiederverwendung scheidet aus (Befund 10). Neu:

- `employees.read`, `employees.write` — an `role-hr` und `role-admin`.
  Bewusst NICHT an alle Rollen: der Mitarbeiterstamm enthält
  personenbezogene Daten (Name, E-Mail, Telefon, Eintritts- und
  Austrittsdatum), Lesezugriff braucht einen Zweck (DSGVO Art. 5 Abs. 1
  lit. b). Module, die später Namen anzeigen müssen (z. B. Einsatz-
  planung), bekommen dafür einen eigenen, schmalen Lesepfad — nicht
  dieses Recht.
- `leave_requests.self` — eigene Anträge stellen und lesen; an alle
  Systemrollen, wie `time_entries.self`.
- `leave_requests.read` — Anträge aller Mitarbeiter lesen.
- `leave_requests.decide` — genehmigen und ablehnen.
  Beide an `role-hr` und `role-admin`.

Das Muster "self gegen read/write" und die Auflösung
Benutzer→Mitarbeiter stammen aus F.1.5.1 und werden wiederverwendet,
nicht neu gebaut.

### 3. Pflege von `hr_employees.user_id`

**A — als weiteres Feld im generischen Patch von `Update`.** Verworfen:
die Verknüpfung entscheidet, wessen Zeiten ein Login bucht. Sie braucht
eigene Prüfungen (Benutzer gehört zum selben Mandanten; Benutzer ist
noch mit keinem anderen Mitarbeiter verknüpft) und eine eigene, klare
Fehlermeldung — in einer Schleife über Patch-Keys wäre das ein
Fremdkörper.

**B — eigene, enge Funktion `SetUser` mit eigener Route
`PUT /employees/{id}/user`.** Gewählt, analog zu
`projects.SetKostenstelle` (ADR 0020). Recht: `employees.write`. Ein
leerer Wert löst die Verknüpfung.

Ein eigenes, drittes Recht nur für die Verknüpfung wurde erwogen und
verworfen: wer `employees.write` hat, ist nach der Rollenvergabe
zugleich Inhaber von `time_entries.write` und kann ohnehin für jeden
buchen. Ein zusätzliches Recht würde nichts schützen.

### 4. Wer hat einen Urlaubsantrag entschieden?

Befund 7: `approver_id` verlangt einen Mitarbeiter. Die Zeiterfassung
hält den Handelnden dagegen als Login-Benutzer fest
(`created_by_user_id`, `storniert_by_user_id`).

**A — `approver_id` beibehalten, Entscheider muss Mitarbeiter sein.**
Verworfen: ein Administrator ohne Mitarbeiterdatensatz könnte nicht
entscheiden, und der Nachweis hinge an einer Stammdatenzeile statt am
Login, der die Aktion ausgelöst hat.

**B — neue Spalte `hr_leave_requests.decided_by_user_id` (FK auf
`users`), `approver_id` wird optional mitgefüllt, wenn der Entscheider
einen verknüpften Mitarbeiter hat.** Gewählt. Der Nachweis "wer hat
entschieden" liegt beim Login, konsistent zur Zeiterfassung;
`approver_id` bleibt erhalten und lesbar (keine Löschung bestehender
Funktionalität), ist aber nicht mehr Voraussetzung.

Dazu: eine Entscheidung ist **einmalig** (Befund 6). Entschieden werden
kann nur ein Antrag im Status `pending`; ein zweiter Versuch wird
abgelehnt. Eine Rücknahme wäre eine neue Funktion mit eigener
Nachvollziehbarkeit, nicht ein Überschreiben.

### 5. Personalnummer

Befund 3 und 4 zusammen: Eindeutigkeit je Mandant statt global
(`UNIQUE (company_id, personalnummer)`), dazu eine Vorabprüfung im
Service mit fachlicher Meldung. Die Migration ist für bestehende Daten
ungefährlich — was global eindeutig war, ist es auch je Mandant.

## Entscheidung

- **Routen Mitarbeiterstamm** (`/api/v1/employees`): Liste, Anlegen,
  Einzelabruf, Ändern (bestehender Patch mit seinen neun Feldern),
  `PUT /{id}/user` für die Verknüpfung, `GET /teams` (nur lesen).
- **Routen Urlaubsanträge** (`/api/v1/leave-requests`): Anlegen, Liste,
  Entscheiden.
- **Vor der Anbindung werden die Services gehärtet** (Befunde 1, 2, 4,
  5, 6). Das ist kein Aufräumen nebenbei, sondern Voraussetzung: jeder
  dieser Punkte wäre über HTTP ein sichtbarer Fehler.
- **Migration `087`**: fünf Permissions samt Rollenzuordnung;
  Personalnummer je Mandant eindeutig; `decided_by_user_id`.
- **Keine Personaldaten in Logs**: Fehlertexte nennen keine Namen,
  E-Mail-Adressen oder Personalnummern. Befund 4 ist auch deshalb zu
  schließen — der rohe Postgres-Fehler enthielte die Personalnummer.
- **Kein Löschpfad für Mitarbeiter.** `EmployeeService` hat keinen und
  bekommt keinen; Ausscheiden wird über `active=false` und
  `termination_date` abgebildet. Seit F.1.2 verhindert ohnehin
  `ON DELETE RESTRICT`, dass ein Mitarbeiter mit Zeitdaten gelöscht
  wird. DSGVO-Löschung nach Fristablauf ist fachlich offen
  (→ `docs/open-questions.md`).
- **Nicht Teil von F.4**: Client-Seite (F.7); Team-Anlage, Abwesenheiten
  aus genehmigten Anträgen, Feiertage, Urlaubskonto (F.8);
  Änderbarkeit von Personalnummer/Ein-/Austrittsdatum; Ablösung der
  Freitextfelder `role`/`cost_center`.

## Freigegeben am 2026-10-13 (war: braucht Freigabe vor F.4.3)

**Befund 9, Standardwert von `active` bei der Anlage.** Heute ist ein
neuer Mitarbeiter inaktiv, wenn der Aufrufer `active` nicht setzt. Das
ist seit 0.1.3.1 als offene Frage vermerkt und wurde dort bewusst nicht
geändert. Mit der HTTP-Anbindung wird es sichtbar: ein Anlegen ohne
`active` ergäbe einen inaktiven Mitarbeiter.

Vorschlag: **neu angelegte Mitarbeiter sind aktiv, wenn nichts anderes
angegeben ist** (entspricht dem Spalten-Default `true` in
`020_hr_base.sql:21`, den der Service derzeit überschreibt). Das ist
eine Verhaltensänderung an bestehendem Code und wird deshalb nicht ohne
Freigabe umgesetzt.

**Entscheidung des Nutzers (2026-10-13): "Ja, neue Mitarbeiter sollten
aktiv sein."** Umsetzung in F.4.3: ein ohne Angabe angelegter
Mitarbeiter ist aktiv; wer ausdrücklich `active: false` angibt, bekommt
einen inaktiven. Dafür muss die Anlage "nicht angegeben" von "false"
unterscheiden können.

Nachtrag F.4.2 (2026-10-13): die Eindeutigkeit der Personalnummer ist
als Index auf `(COALESCE(company_id, ''), personalnummer)` umgesetzt,
nicht als schlichtes `UNIQUE (company_id, personalnummer)` — `company_id`
ist noch nullable, und NULLs gelten in einem UNIQUE-Index als
verschieden.

Nachtrag F.4.3 (2026-10-13): Befund 4 ist nicht über eine Vorabprüfung
geschlossen, sondern durch Übersetzen des UNIQUE-Fehlers in die
fachliche Meldung (ebenso bei der Benutzer-Verknüpfung). Das ist auch
bei gleichzeitigen Aufrufen richtig. `EmployeeService.Create` nimmt
jetzt `EmployeeInput` mit `Active *bool`.

Nachtrag F.4.5 (2026-10-13): `LeaveService.Approve` ist entfallen,
`Decide` ist die eine Funktion für Genehmigen und Ablehnen. `Approve`
hieß so, lehnte aber auch ab; `Decide` rief nur `Approve` auf; es gab
keinen Aufrufer außerhalb der Tests. Zusätzlich geprüft werden jetzt
der Antragstyp (vier Werte laut `020_hr_base.sql`) und die Tageszahl
(höchstens die Länge des Zeitraums).

## Konsequenzen

Zerlegung von F.4:

- **F.4.2** Migration `087_hr_http_permissions_and_hardening.sql`.
- **F.4.3** `EmployeeService` härten (Befunde 1, 2, 4, 5; `active`
  je nach Freigabe) und `SetUser` ergänzen, mit Tests.
- **F.4.4** HTTP-Routen Mitarbeiterstamm samt Verknüpfung, mit
  Integrationstests. **Danach kann ein Mitarbeiter stempeln.**
- **F.4.5** `LeaveService` härten (Befund 6, `decided_by_user_id`,
  optionaler `approver_id`), mit Tests.
- **F.4.6** HTTP-Routen Urlaubsanträge mit `self`/`read`/`decide`.

Neue Backlog-Positionen: **F.7** Client-Anbindung der
Mitarbeiterverwaltung (`employees_page.dart` und fehlende
`api.dart`-Methoden); **F.8** Abwesenheiten, Feiertage, Urlaubskonto,
Team-Anlage.
