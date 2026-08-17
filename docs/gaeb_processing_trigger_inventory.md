# GAEB: Folgeinventar fuer den produktiven Verarbeitungsausloeser

## Ziel

Subtask 3.1.74.1 bestimmt den kleinsten sicheren produktiven Anschluss an den
auditierten ProcessGAEBImport-Service. Dieser Leaf aendert keine Runtime- oder
Testlogik.

## Ausgangslage

Die Servicegrenze kann einen gespeicherten, uploadten GAEB-Import nach parsed
oder failed ueberfuehren. Der v1-Router erzeugt den Quote-Service derzeit nur
mit Postgres, Numbering und MongoDB; kein konkreter Parseradapter wird
konfiguriert. Folglich existiert weder ein produktiver Aufruf noch eine
Parserbereitstellung.

Die beiden Luecken sind bewusst getrennt:

- Der Ausloeser bestimmt Berechtigung, Request/Response und den Zeitpunkt.
- Der Parseradapter bestimmt die konkrete GAEB-Formatverarbeitung.

Ohne diese Trennung wuerde ein automatischer Uploadcallback entweder eine
unbegründete Erfolgssemantik behaupten oder eine nicht konfigurierte
Parserinstanz als versteckten Laufzeitfehler erzeugen.

## Optionen im Vergleich

### Option A: Expliziter, geschuetzter Process-Endpunkt

Ein autorisierter Benutzer startet die Verarbeitung eines einzelnen
uploaded-Imports ueber einen POST-Endpunkt. Der Router delegiert nur an
ProcessGAEBImport und gibt dessen aktualisierten Importlauf zurueck.

Vorteile:

- passt zu den bestehenden importbezogenen Review- und Apply-Endpunkten;
- klare Berechtigung mit quotes.write;
- keine implizite Synchronität im Uploadrequest;
- Parserkonfiguration kann getrennt eingefuehrt und integrationstestbar
  injiziert werden;
- Wiederholung ist durch den uploaded-Guard eindeutig abweisbar.

Bewertung: kleinster produktiver Ausloeser mit klarer Auditgrenze.

### Option B: Verarbeitung direkt nach Upload starten

Der Uploadendpunkt ruft nach dem Speichern automatisch ProcessGAEBImport auf.

Bewertung: setzt sofort einen produktiv konfigurierten Parser voraus, koppelt
Uploadlatenz an Parsing und vermischt das bestätigte Speichern mit der
fachlichen Verarbeitung. Fuer den aktuellen Stand zu gross und zu frueh.

### Option C: Hintergrundworker oder Queue

Ein Worker verarbeitet uploaded-Importe asynchron.

Bewertung: langfristig passend, erfordert aber Jobmodell, Sperren, Retry,
Monitoring und Ausfallsemantik. Kein minimaler Anschluss.

### Option D: Nur Parser im Router konfigurieren

Ein konkreter Parseradapter wird ohne HTTP-Ausloeser bereitgestellt.

Bewertung: unverzichtbar für echte Verarbeitung, aber ohne kontrollierten
Aufruf noch nicht produktiv erreichbar. Die konkrete Parserentscheidung
bleibt ein eigener Block.

## Entscheidung

Der kleinste produktive Anschluss ist ein expliziter, berechtigter
Verarbeitungsendpunkt fuer einen einzelnen Import:

    POST /api/v1/quotes/imports/{id}/process

Der Endpunkt bekommt keinen Body. Er darf nur mit quotes.write erreichbar
sein, ruft genau einmal ProcessGAEBImport auf und liefert den resultierenden
QuoteImport mit HTTP 200. Servicefehler durch fehlenden Parser, falschen
Status oder unbekannte Quelle werden ueber den bestehenden Domain-Fehlerpfad
transparent zurückgegeben.

Der Endpoint wird erst dann in der Produktkonfiguration wirksam verarbeitet,
wenn ein konkreter Parseradapter über die getrennte Bereitstellung
konfiguriert ist. Diese Klarheit ist besser als ein stilles Fallback oder
eine scheinbar erfolgreiche Verarbeitung ohne Parser.

## Zerlegung

Task 3.1.74 wird in drei weitere Leaves zerlegt:

1. **3.1.74.2** – API- und Parserbereitstellungsstrategie fuer den
   expliziten Process-Endpunkt definieren.
2. **3.1.74.3** – Router-Anschluss, injizierbare Parserbereitstellung und
   kleinste HTTP-Integrationstests implementieren.
3. **3.1.74.4** – Endpunkt, Berechtigung, Fehlergrenzen und Nachweise
   auditieren.

## Nicht-Ziele

- kein konkreter GAEB-Parser und keine Formatheuristik;
- kein automatischer Uploadcallback, Worker, Queue, Retry oder UI-Ausbau;
- keine KI-, Mapping-, Preis-, Kalkulations- oder Review-Aenderung;
- keine neue Datenbankstruktur, Migration oder GridFS-Kompensation.

## Naechster Leaf

Subtask 3.1.74.2 definiert ausschliesslich die Abhaengigkeitsbereitstellung,
den bodylosen Endpunktvertrag, Statuscodes und gezielte Erfolgs- und
Fehlernachweise.

## Ergebnis

3.1.74.1 ist abgeschlossen. Ein expliziter geschuetzter Process-Endpunkt ist
der kleinste sichere produktive Ausloeser; die konkrete Parserimplementierung
bleibt davon getrennt.
