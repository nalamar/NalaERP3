# GAEB: Folgeinventur nach dem getesteten Positionsdetailpfad

## Ziel

Subtask 3.1.60.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
dem abgesicherten read-only GAEB-Positionsdetailpfad. Dieser Leaf aendert keine
Runtime- oder Testlogik.

## Ausgangslage

Die read-only Kette ist inzwischen durchgaengig widgetgetestet:

```text
Importvorschau -> Importdialog -> Rohposition -> Positionsdetaildialog
```

Im Positionsdetaildialog existiert bereits die kleinste fachliche
Schreibaktion:

- `Review setzen` erscheint mit `quotes.write`.
- `_QuoteImportReviewDialog` bietet die Statuswerte `pending`, `accepted` und
  `rejected`.
- eine optionale Reviewnotiz wird vor der Rueckgabe getrimmt.
- `updateQuoteImportItemReview(...)` erhaelt Import-ID, Item-ID, Status und
  Notiz.
- die API-Antwort ersetzt lokal `detail` und aktualisiert den sichtbaren
  Reviewstatus.
- eine Snackbar bestaetigt die Speicherung.

Im vorhandenen Flutter-Harness wird dieser Pfad noch nicht ausgefuehrt. Der
zweistufige Positionsdetail-Fake ist vorhanden, aber es gibt keinen
Mutation-Override oder aufgezeichneten Review-Payload.

## Optionen

### A: Positionsreview-Mutation widgettesten

Ein isolierter Test oeffnet eine Rohposition mit `quotes.write`, startet
`Review setzen`, waehlt einen neuen Status, erfasst eine Notiz und speichert.
Der Fake zeichnet beide IDs und den normalisierten Payload auf und liefert ein
aktualisiertes Detail zurueck.

Vorteile:

- sichert die kleinste vorhandene GAEB-Fachmutation
- belegt Berechtigung, Formular und API-Payload gemeinsam
- prueft die lokale Aktualisierung ohne Seitenreload
- vorhandene Dialog- und Fake-Strukturen sind wiederverwendbar
- keine Runtime-, Backend- oder API-Aenderung erforderlich

Bewertung: kleinster Folgeausbau mit direktem Review-Nutzen.

### B: Gesamten Importlauf zur Uebernahme freigeben

`Zur Übernahme freigeben` koennte im aeusseren Importdialog getestet werden.

Bewertung: groesser. Der Pfad mutiert den Import, laedt Detail und Positionen
erneut, aktualisiert die Importvorschau und zeigt einen Fortschrittsdialog.
Ausserdem setzt er fachlich bereits abgeschlossene Einzelreviews voraus.

### C: Draft-Quote aus Importlauf erzeugen

`Draft-Quote erzeugen` koennte bis zur neuen Quote und Seitenaktualisierung
getestet werden.

Bewertung: zentral fuer GAEB, aber an Importstatus, Apply-Antwort,
Angebotsliste und Navigation gekoppelt. Deutlich groesser als eine
Einzelreview-Mutation.

### D: Quote-Verknuepfung einer uebernommenen Position oeffnen

Der Positionsdialog koennte bei gesetzter `linked_quote_id` zur Quote
navigieren.

Bewertung: read-only, aber erst nach Apply fachlich relevant und mit
Seitenreload verbunden. Der ungetestete Review-Schreibpfad liegt davor.

### E: KI-Reviewvorschlag

Eine KI koennte Status und Begruendung fuer Rohpositionen vorschlagen.

Bewertung: langfristig wichtig, erfordert aber Modellvertrag,
Nachvollziehbarkeit, menschliche Freigabe und Fehlerregeln. Die manuelle
Reviewmutation sollte zuerst belastbar getestet sein.

## Entscheidung

Der naechste Ausbau ist ein isolierter Widgettest fuer die vorhandene
Positionsreview-Mutation.

Der Zielumfang bleibt eng:

- neuer Fake-Override fuer `updateQuoteImportItemReview(...)`
- Aufzeichnung von Import-ID, Item-ID, Reviewstatus und Reviewnotiz
- Rueckgabe eines aktualisierten Positionsdetails
- Testberechtigungen `quotes.read` und `quotes.write`
- genau ein Import und genau eine Rohposition
- Statuswechsel von `pending` auf einen eindeutigen Zielstatus
- Notizeingabe mit umgebenden Leerzeichen und Pruefung des getrimmten Payloads
- sichtbarer aktualisierter Reviewstatus und Erfolgssnackbar

Der vorhandene read-only Test bleibt unveraendert, damit die Berechtigungsgrenze
weiter separat abgesichert ist.

## Naechster Leaf

Subtask 3.1.60.2 definiert das technische Minimalmodell fuer:

- Mutation-Recording und Rueckgabevertrag im Fake
- kleinsten Testpayload
- Dropdown- und Textfeldinteraktion
- stabile Finder bei drei Dialogebenen
- Erfolgskriterien nach dem Speichern

## Nicht-Ziele

- noch keine Implementierung
- keine Runtime-Aenderung
- keine Importfreigabe oder Draft-Quote-Erzeugung
- keine Quote-Navigation
- kein Fehlerpfad- oder Validierungstest
- keine Backend-, API-, Datenbank- oder Permission-Aenderung
- keine KI-Automatisierung

## Ergebnis

3.1.60.1 ist abgeschlossen. Der kleinste Folgeausbau ist ein isolierter
Widgettest fuer die vorhandene Positionsreview-Mutation. Subtask 3.1.60.2
definiert dafuer das technische Minimalmodell.
