# ADR 0030 — Rohe Datenbankfehler weder ins Log noch an den Aufrufer

Datum: 2026-10-23
Status: entschieden (Backlog 0.47; "fortsetzen" des Nutzers als Zustimmung
zur Empfehlung gewertet, nicht ausdrücklich bestätigt)
Bezug: `aufgabe.md` §2 ("Keine Personaldaten in Logs"), Fund aus 0.46
(ADR 0026).

## Kontext

Alle Fehlerantworten der API laufen über drei Funktionen
(`server/internal/http`):

- `writeDomainError` ordnet einen Fehler über `classifyDomainError` nach
  deutschen Teilstrings einem Status zu und lieferte **immer**
  `err.Error()` an den Aufrufer, auch bei 500 für nicht zugeordnete
  Fehler.
- `writeHTTPError` (113 Aufrufstellen) liefert `msg` als Klartext; an 68
  Stellen ist `msg` schlicht `err.Error()` (35× bei 400, 23× bei 500,
  10× bei 404).
- `logAPIError` schrieb `message=%q err=%v` — also wieder den rohen Text.

Ein Postgres-Fehler trägt Datenwerte: im `Detail`
(`Key (company_id, personalnummer)=(default, 1001) already exists`) und
teils schon in der `Message` (`invalid input syntax for type uuid:
"…"`). In F.4.3 wurde das für Personalnummer und Benutzer-Verknüpfung
durch fachliche Meldungen umgangen, allgemein war es offen.

Dazu ein zweiter Weg: `classifyDomainError` sucht Teilstrings im ganzen
Fehlertext. Ein Datenwert im `Detail`, der zufällig "nicht gefunden"
enthält, hätte einen Postgres-Fehler als fachlichen 404 samt Rohtext
ausgeliefert.

## Optionen

**A — Jede Aufrufstelle umstellen** (113 + alle `writeDomainError`).
Verworfen: großer Eingriff in alle Domänen, und jede neue Aufrufstelle
könnte den Fehler wieder einführen.

**B — Zentral in den drei Funktionen** (gewählt). Die Aufrufstellen
bleiben unverändert; die Regel gilt für jeden Weg zur Antwort.

**C — Fehlertexte im Log komplett weglassen.** Verworfen: bei einem
nicht zugeordneten 500 wäre die Ursache dann nirgends mehr zu finden.

## Entscheidung

1. **Postgres-Fehler (`*pgconn.PgError` irgendwo in der Kette) werden
   nur mit Kennungen aus dem Schema geloggt**: `postgres code=<SQLSTATE>
   constraint=… table=… column=…` (`describeError`). Weder `Message` noch
   `Detail` noch der Text, den der Code darum gewickelt hat (er enthält
   den Postgres-Text).
2. **Ein Postgres-Fehler ist nie fachlich**: `classifyDomainError` gibt
   für ihn 500 zurück, bevor Teilstrings geprüft werden.
3. **Bei 500 bekommt der Aufrufer `Interner Fehler (Request-ID …)`**
   statt des Fehlertexts (`writeDomainError`; bei `writeHTTPError`, wenn
   `msg == err.Error()`). Die Request-ID steht auch im Kopf
   `X-Request-ID` und in der Logzeile.
4. **`writeHTTPError` reicht einen Postgres-Fehler bei keinem Status
   durch**: 404 → "Nicht gefunden", sonst 4xx → "Ungültige Eingabe" —
   auch dann, wenn die Aufrufstelle eine eigene Meldung übergab, weil
   die aus dem Fehler zusammengesetzt sein kann.
5. Selbst formulierte Meldungen ohne Postgres-Fehler bleiben, auch bei
   500 (z. B. "GridFS nicht verfügbar"). Fachliche 4xx-Meldungen bleiben
   unverändert.
6. `logAPIError` schreibt das Fehlerfeld mit `%q` (ein Zeilenumbruch im
   Fehlertext erzeugt keine zweite Logzeile mehr).

## Konsequenzen

- Der Client zeigt bei internen Fehlern "Interner Fehler (Request-ID …)"
  statt technischer Texte; die Ursache findet sich über die Request-ID im
  Log.
- **Nicht gelöst** (bewusst): ein nicht zugeordneter Fehler OHNE
  Postgres-Bezug steht weiterhin im Klartext im Log (z. B. ein
  `strconv`-Fehler mit der Eingabe). Das ist der Preis der
  Fehlersuche (Option C verworfen); er erreicht den Aufrufer nicht mehr.
- **Nicht gelöst**: IDs im Pfad stehen weiter in Request- und Fehler-Log
  (Auslegungsfrage aus ADR 0026, `docs/open-questions.md`).
- **Bereits geschriebene Logs** enthalten die alten Texte weiterhin; die
  Änderung wirkt nur nach vorn.
- Andere Ausgaben an `log` außerhalb dieser drei Funktionen sind nicht
  Teil dieser Entscheidung: der Panic-Handler (`panic=%v`) und die
  Protokollzeilen des Asset-Imports in `v1.go` (Dateinamen/Pfade der
  hochgeladenen Projektdateien und GridFS-/Dateifehler, keine
  Postgres-Fehler). Durchgesehen am 2026-10-23.
