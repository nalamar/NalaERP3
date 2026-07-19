# GAEB: Folgeinventur nach dem getesteten Positionsreview

## Ziel

Subtask 3.1.61.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
dem abgesicherten Review einer einzelnen GAEB-Importposition. Dieser Leaf
aendert keine Runtime- oder Testlogik.

## Ausgangslage

Die clientseitige GAEB-Pruefkette ist inzwischen bis zur Einzelentscheidung
widgetgetestet:

```text
Importvorschau -> Importdetail -> Positionsdetail -> Reviewentscheidung
```

Der Mutationstest belegt Import-ID, Item-ID, normalisierten Status-/Notiz-
Payload, lokale Detailaktualisierung und Erfolgsfeedback. Der naechste bereits
vorhandene Fachschritt liegt im aeusseren Importdetaildialog:

- bei Importstatus `parsed` und `quotes.write` erscheint
  `Zur Übernahme freigeben`
- `markQuoteImportReviewed(importId)` setzt den Importlauf auf `reviewed`
- ein nicht schliessbarer Fortschrittsdialog begleitet die Mutation
- Detail und Rohpositionen werden danach neu geladen
- die Importvorschau wird aktualisiert
- `Importlauf wurde freigegeben` bestaetigt den Erfolg
- erst der Status `reviewed` schaltet `Draft-Quote erzeugen` frei

API- und Backendvertrag fuer diesen Schritt existieren bereits und sind auf
Integrationsebene abgedeckt. Im Flutter-Harness fehlt der Mutationspfad.

## Optionen

### A: Importlauf-Freigabe widgettesten

Ein isolierter Test oeffnet einen `parsed` Import ohne offene Positionen,
betaetigt `Zur Übernahme freigeben` und zeichnet die Import-ID im Fake auf.
Der Fake liefert beim anschliessenden Refresh den Status `reviewed`.

Vorteile:

- sichert den direkten Folgeschritt nach den Einzelreviews
- belegt Status- und Berechtigungsgrenze
- prueft Mutation, Fortschrittsdialog, Refresh und Erfolgsfeedback
- macht die nachfolgende Apply-Aktion sichtbar, ohne sie auszufuehren
- vorhandene Importdetail-Fakes und Finder sind wiederverwendbar
- keine Runtime-, Backend- oder API-Aenderung erforderlich

Bewertung: kleinster eigenstaendiger Folgeausbau mit neuem Prozessnutzen.

### B: Zweiten Einzelreviewstatus oder Abbruch testen

Der vorhandene Test koennte um `rejected`, `pending` oder `Abbrechen`
erweitert werden.

Bewertung: technisch kleiner, aber fachlich weitgehend redundant. ID-Bindung,
Dropdown, Notiznormalisierung und lokale Aktualisierung sind bereits belegt.

### C: Fehlerpfad der Einzelreviewmutation testen

Der Fake koennte einen API-Fehler ausloesen und die Fehlersnackbar pruefen.

Bewertung: sinnvoll fuer Robustheit, erschliesst aber keinen neuen GAEB-
Prozessschritt und erfordert einen zusaetzlichen Fake-Fehlermodus.

### D: Draft-Quote aus dem Importlauf erzeugen

`Draft-Quote erzeugen` koennte bis zur Apply-Antwort und aktualisierten
Angebotsliste getestet werden.

Bewertung: zentral, aber groesser. Der Pfad setzt einen bereits freigegebenen
Import voraus, verarbeitet Import- und Quote-Antwort, laedt die Angebotsseite
neu und besitzt eine eigene Navigation zur erzeugten Quote.

### E: Serverseitige Sammelentscheidung oder Review-Automatisierung

Mehrere Positionen koennten gesammelt oder ueber KI-Vorschlaege bewertet
werden.

Bewertung: langfristig relevant, benoetigt jedoch neue Fach-, API-,
Nachvollziehbarkeits- und Human-in-the-loop-Regeln. Die vorhandene sequenzielle
Freigabekette sollte zuerst vollstaendig getestet werden.

## Entscheidung

Der naechste Ausbau ist ein isolierter Widgettest fuer die vorhandene Freigabe
des vollstaendig geprueften Importlaufs.

Der Zielumfang bleibt eng:

- Fake-Aufzeichnung fuer `markQuoteImportReviewed(importId)`
- genau ein `parsed` Importdetail mit `pending_count: 0`
- Berechtigungen `quotes.read` und `quotes.write`
- Pruefung, dass `Zur Übernahme freigeben` sichtbar ist
- Ausfuehrung der Mutation mit der korrekten Import-ID
- deterministischer Refresh auf Status `reviewed`
- Pruefung des aktualisierten Status und der Erfolgssnackbar
- Pruefung, dass danach `Draft-Quote erzeugen` sichtbar wird
- keine Ausfuehrung der Apply-Mutation

Der bestehende Positionsreview-Test bleibt unveraendert.

## Naechster Leaf

Subtask 3.1.61.2 definiert das technische Minimalmodell fuer:

- Fake-Mutationsrecord und zustandsabhaengige Importdetail-Antwort
- kontrollierten Detail-/Positions-/Listenrefresh
- stabile Interaktion mit dem Fortschrittsdialog
- kleinsten Testpayload ohne offene Positionen
- Erfolgskriterien nach dem Statuswechsel

## Nicht-Ziele

- noch keine Implementierung
- keine Aenderung der Runtime
- keine Apply-Mutation oder Draft-Quote-Erzeugung
- keine Quote-Navigation
- kein Fehlerpfadtest
- keine Sammelreview- oder KI-Logik
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Ergebnis

3.1.61.1 ist abgeschlossen. Der kleinste fachlich wertvolle Folgeausbau ist
ein isolierter Widgettest fuer die vorhandene Importlauf-Freigabe von `parsed`
nach `reviewed`. Subtask 3.1.61.2 definiert dafuer das technische Minimalmodell.
