# ADR 0026 — Request-Log ohne Werte von Query-Parametern

Datum: 2026-10-13
Status: entschieden (Backlog 0.46, vom Nutzer am 2026-10-13 vor F.2 gezogen)
Bezug: `aufgabe.md` §2 ("Keine Personaldaten in Logs"), Fund aus F.4.6.

## Kontext

`requestLoggerMiddleware` (`server/internal/http/observability.go`)
schrieb für jeden Request den vollständigen Query-String ins Log
(`query=%q` mit `req.URL.RawQuery`).

Der Fund entstand bei den Listenrouten der Zeiterfassung und der
Urlaubsanträge (`employee_id=<uuid>`). Die Durchsicht aller Handler
zeigt, dass das der harmlosere Teil ist:

- **Freitextsuche `q`** an neun Stellen in `v1.go`, darunter die
  Kontaktliste (`contacts.ContactFilter{Q: q.Get("q")}`). Wer nach einem
  Kunden sucht, schreibt dessen **Namen im Klartext** ins Log.
- Fachliche Bezüge als IDs: `employee_id`, `contact_id`, `project_id`,
  `lieferant_id`, `supplier_id` und weitere.
- Datumsgrenzen von Abfragen (`from`/`to`, `von`/`bis`, `datum`).

Insgesamt 32 verschiedene Parameternamen. Kein Test und keine
Dokumentation im Repo bezieht sich auf das Format der Logzeile.

## Optionen

**A — Query-String gar nicht mehr loggen.** Sicher und am einfachsten.
Verworfen: bei der Fehlersuche ("warum ist diese Listenabfrage langsam,
warum leer?") ist es nützlich zu sehen, WELCHE Filter gesetzt waren —
dafür braucht es die Werte nicht.

**B — einzelne Parameter schwärzen** (z. B. `q` und `employee_id`).
Verworfen: eine Sperrliste ist in die falsche Richtung offen. Jeder
neue Parameter wäre geloggt, bis jemand daran denkt, ihn einzutragen.

**C — nur die Namen der Parameter loggen, alle.** Verworfen wegen eines
unscheinbaren Falls: `?Mueller` ist formal ein Parameter mit dem
**Namen** "Mueller" und leerem Wert. "Alle Namen" würde also wieder
Eingaben loggen.

**D — nur Namen aus einer festen Liste, der Rest wird gezählt.**
Gewählt. Geloggt wird `query_keys="employee_id,from,to"` (bekannte
Namen, sortiert, je einmal) und `query_other=<n>` (Anzahl aller
übrigen Parameter; ein nicht lesbarer Query-String zählt als einer).
Der Rohtext wird in keinem Fall geschrieben.

Die Liste ist in die sichere Richtung offen: fehlt ein neuer Parameter,
erscheint er als Zähler statt mit Namen. Das ist weniger Auskunft, aber
nie ein Leck — deshalb braucht die Liste keinen Wächter, der sie mit den
Handlern abgleicht.

## Entscheidung

Option D. Das Feld heißt bewusst nicht mehr `query`, damit niemand in
einer Auswertung den neuen Inhalt für den alten hält.

Die Handler sind nicht betroffen: sie sehen den Query-String weiterhin
vollständig.

## Was damit NICHT gelöst ist

Diese ADR schließt genau die Query-Werte. Beim Durchsehen sind drei
verwandte Punkte aufgefallen, die hier nur festgehalten werden:

1. **IDs im Pfad.** `/employees/<uuid>`, `/time-entries/<uuid>` usw.
   stehen weiterhin im Log (`path=`). Das sind pseudonyme technische
   Bezeichner, keine Namen. Sie aus dem Pfad zu entfernen hieße, das
   Request-Log für die Zuordnung eines Fehlers zu einem Datensatz
   unbrauchbar zu machen. Ob das mit "keine Personaldaten in Logs"
   vereinbar ist, ist eine Auslegungsfrage → `docs/open-questions.md`.
2. **`remote_ip` und `user_agent`** stehen in jeder Logzeile. Eine
   IP-Adresse ist ein personenbezogenes Datum; zugleich ist sie für die
   Nachverfolgung von Angriffen (u. a. das Login-Rate-Limit aus 0.5.2)
   der entscheidende Wert. → dieselbe offene Frage, zusammen mit der
   Aufbewahrungsdauer der Logs.
3. **Rohe Datenbankfehler im Fehler-Log.** `logAPIError` schreibt bei
   nicht klassifizierten Fehlern den Fehlertext mit (`err=%v`). Ein
   roher Postgres-Fehler kann im `DETAIL` Datenwerte tragen (z. B.
   `Key (personalnummer)=(1001)`). In F.4.3 wurde das für die
   Personalnummer durch eine fachliche Meldung vermieden; allgemein
   gelöst ist es nicht. → Backlog 0.47.

## Konsequenzen

- Logzeilen haben ein neues Format: `query=…` entfällt, `query_keys=…`
  und `query_other=…` kommen hinzu. Wer Logs auswertet, muss das
  berücksichtigen (im Repo gibt es keine solche Auswertung).
- **Bereits geschriebene Logs enthalten die alten Werte weiterhin.**
  Diese Änderung wirkt nur nach vorn. Ob und wie bestehende Logdateien
  bereinigt oder gelöscht werden, ist eine betriebliche Entscheidung
  außerhalb des Repos → `docs/open-questions.md`.
- Neue Query-Parameter sollten in `loggedQueryKeys` eingetragen werden,
  wenn ihr Name im Log erscheinen soll.
