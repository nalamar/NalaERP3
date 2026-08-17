# GAEB: Folgeinventar nach abgesichertem Uploadfehler

## Ziel

Subtask 3.1.71.1 bestimmt den kleinsten noch offenen GAEB-Upload-Risikobereich
nach den belegten Pfaden fuer fehlendes Projekt, erfolgreichen Upload und
strukturierte serverseitige Ablehnung. Dieser Leaf aendert keine Runtime- oder
Testlogik.

## Abgesicherter Stand

Der Client-Harness belegt inzwischen:

- Abbruch vor dem Picker, wenn keine Projekt-ID gesetzt ist
- korrekten Accept-Filter und normalisierte Uploadparameter
- erfolgreichen Upload mit Listen-Reload und Erfolgsmeldung
- strukturierten Uploadfehler ohne Reload und Erfolgshinweis
- geschlossenen Fortschrittsdialog nach Erfolg und Fehler
- weiterhin verfuegbare Importaktion nach einer Ablehnung

Die lokale `QuoteImportFilePicker`-Testnaht deckt damit die beiden mutierenden
Ausgaenge ab. Ein unmittelbarer nicht mutierender Ausgang bleibt offen.

## Unmittelbare offene Luecke

`QuotesPage._importGAEB()` beendet den Ablauf, wenn der Dateipicker `null`
zurueckgibt:

```text
Projekt vorhanden
  -> Picker wird geoeffnet
  -> Benutzer bricht ab
  -> kein Fortschrittsdialog
  -> kein Uploadversuch
  -> kein Listen-Reload
  -> keine Erfolgs- oder Fehlermeldung
```

Dieser fruehe Return ist einfach, aber noch nicht widgetgetestet. Eine
Regression koennte beim Abbruch einen leeren Upload ausloesen, Feedback
anzeigen oder unnoetig die Importliste neu laden.

## Folgeoptionen im Vergleich

### Option A: Picker-Abbruch widgettesten

Der vorhandene lokale Picker gibt `null` zurueck. Der Test belegt einen
Pickeraufruf mit dem bestehenden Accept-Filter, keinen Uploadversuch,
unveraendert einen initialen Listenabruf und das Ausbleiben von Dialog sowie
Feedback.

Bewertung: kleinster deterministischer Rest im bereits vorhandenen
Uploadvertrag. Keine Fake- oder Runtime-Erweiterung erforderlich.

### Option B: sichtbaren Fortschritts-Zwischenzustand testen

Ein angehaltener Future koennte den nicht schliessbaren Dialog waehrend des
Uploads belegen.

Bewertung: asynchron fragiler und technisch groesser. Erfolg und Fehler
belegen bereits, dass der Dialog in beiden Endzustaenden geschlossen wird.

### Option C: unbekannten technischen Fehler testen

Ein Nicht-`ApiException`-Fehler koennte die Fallbackmeldung
`GAEB-Upload fehlgeschlagen` absichern.

Bewertung: moeglich, aber nach dem strukturierten Fachfehler weniger wertvoll
als die noch gaenzlich ungedeckte Abbruchgrenze. Der gleiche Catch- und
Dialogpfad ist bereits belegt.

### Option D: Dateiinhalt oder Dateigroesse validieren

Leere oder zu grosse Dateien koennten clientseitig abgewiesen werden.

Bewertung: erfordert neue Fachregeln und Abstimmung mit Server- und
Proxygrenzen. Kein kleiner Testrest.

### Option E: Parsing, Mapping oder KI-Anreicherung erweitern

Die Verarbeitung importierter Positionen koennte fachlich vertieft werden.

Bewertung: strategisch relevant, aber ein eigener groesserer Funktionsblock.
Er gehoert nicht in die Absicherung der Picker-Grenze.

## Entscheidung

Der kleinste noch offene GAEB-Upload-Risikobereich ist der explizite Abbruch
der Dateiauswahl bei vorhandener Projekt-ID.

Der Zielumfang bleibt eng:

- `quotes.read` und `quotes.write`
- feste Projekt-ID
- lokaler Picker gibt `null` zurueck
- Picker wird genau einmal mit dem bestehenden Accept-Filter aufgerufen
- `attemptedQuoteImportUploads` bleibt leer
- `quoteImportListRequestCount` bleibt nach dem initialen Abruf bei eins
- kein Fortschrittsdialog und kein `AlertDialog`
- keine Upload-Erfolgs- oder Fehlermeldung

## Zerlegung

Task 3.1.71 wird in drei weitere Leaves zerlegt:

1. **3.1.71.2** – Minimalstrategie fuer den Picker-Abbruch-Widgettest
   definieren.
2. **3.1.71.3** – Genau einen Picker-Abbruch-Widgettest implementieren und
   verifizieren.
3. **3.1.71.4** – Implementierung und Nachweise auditieren.

## Nicht-Ziele

- noch keine Implementierung
- kein angehaltener Upload-Future
- kein weiterer Uploadfehler-Test
- keine Runtime-, Backend-, API-, DB- oder Permission-Aenderung
- keine Dateiinhalt- oder Groessenregel
- keine Mapping-, Kalkulations- oder KI-Logik

## Naechster Leaf

Subtask 3.1.71.2 definiert ausschliesslich Testdaten, Pickervertrag,
Negativassertions und gezielte Verifikationskommandos fuer den Abbruch der
Dateiauswahl.

## Ergebnis

3.1.71.1 ist abgeschlossen. Der Picker-Abbruch ist der kleinste isolierte
Folgeleaf und kann vollstaendig ueber die bestehende Testnaht abgesichert
werden.
