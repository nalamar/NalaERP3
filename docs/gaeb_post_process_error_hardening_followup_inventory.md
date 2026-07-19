# GAEB: Folgeinventar nach abgeschlossener Prozessfehlerabsicherung

## Ziel

Subtask 3.1.67.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
der Widgettest-Absicherung der zentralen Freigabe-, Apply- und
Navigationsfehler. Dieser Leaf aendert keine Runtime- oder Testlogik.

## Ausgangslage

Die zentrale GAEB-Clientkette ist fuer ihre wichtigsten Ausgaenge abgedeckt:

```text
Einzelposition erfolgreich reviewen
  -> Importlauf erfolgreich freigeben oder Freigabefehler erhalten
  -> Draft-Quote erfolgreich erzeugen oder Apply-Fehler erhalten
  -> erzeugte Quote erfolgreich oeffnen oder Ladefehler erhalten
```

Auf Backend-Integrationsebene ist zudem bereits belegt, dass beim Apply:

- nur akzeptierte Importpositionen uebernommen werden
- Beschreibung, Menge und Einheit in der Quote ankommen
- der initiale Preis- und Mappingzustand korrekt gesetzt wird
- Importposition und Quote-Position miteinander verknuepft werden

Ein neuer Transformations-Test waere deshalb nicht der kleinste offene Punkt.

## Verbleibende unmittelbare Luecke

Der vorgelagerte Einzelpositionsreview besitzt einen vorhandenen, aber im
Flutter-Harness noch nicht getesteten Fehlerausgang:

- eine Importposition steht auf `pending`
- der Nutzer waehlt beispielsweise `accepted` und erfasst eine Notiz
- `updateQuoteImportItemReview(...)` wird serverseitig abgewiesen
- der Entscheidungsdialog ist nach dem Speichern geschlossen
- der Positionsdetaildialog muss geoeffnet bleiben
- Status und Review-Notiz muessen unveraendert bleiben
- eine fachliche Fehlermeldung muss sichtbar sein
- es darf kein Erfolgssignal erscheinen

Dieser Fall schliesst die letzte unmittelbare Mutationsfehlerluecke vor der
bereits abgesicherten Importlauf-Freigabe.

## Folgeoptionen im Vergleich

### Option A: Einzelpositionsreview-Fehler widgettesten

Ein `parsed` Import mit einer `pending`-Position wird mit `quotes.read` und
`quotes.write` geoeffnet. Nach Auswahl von `accepted` und Eingabe einer Notiz
wirft `updateQuoteImportItemReview` fuer genau diese Import-/Item-Kombination
eine strukturierte `ApiException`. Der Test belegt Versuchspayload,
ausbleibenden Erfolg, erhaltenes Positionsdetail und unveraenderten Zustand.

Vorteile:

- schliesst den Fehlervertrag der ersten fachlichen Schreibmutation
- verhindert falschen Status- oder Notizeindruck nach einer Ablehnung
- nutzt ausschliesslich bestehende Runtime- und API-Vertraege
- ist auf eine Fake-Fehlerkonfiguration und einen Widgettest begrenzbar
- keine Backend-, API-, DB- oder Permission-Aenderung erforderlich

Bewertung: kleinster eigenstaendiger Folgeausbau mit direktem
Datenintegritaetssignal.

### Option B: Berechtigungsnegativtest fuer Positionsreview

Ohne `quotes.write` koennte `Review setzen` als verborgen geprueft werden.

Bewertung: kleiner, aber mit geringerem Signal. Das Permission-Gate ist eine
direkte boolesche Bedingung; der Mutationsfehler prueft Zustandserhalt und
Feedback nach einem realen Laufzeitfehler.

### Option C: weiteren erfolgreichen Reviewstatus testen

Der bestehende Erfolgsfall koennte `rejected` statt `accepted` verwenden oder
um einen zweiten Test ergaenzt werden.

Bewertung: weitgehend redundant. ID-Bindung, normalisierte Notiz,
Detailaktualisierung und Erfolgssnackbar sind bereits fuer denselben
Schreibpfad belegt.

### Option D: Transformations- oder Mappingtests ausbauen

Weitere Preis-, Material- oder Steuerregeln koennten zwischen Import und Quote
geprueft werden.

Bewertung: fachlich wichtig, aber ein eigener groesserer Block. Die aktuelle
Integration deckt den deterministischen Basisvertrag bereits ab.

### Option E: KI-gestuetzte Angebotsanreicherung beginnen

GAEB-Positionen oder erzeugte Draft-Quotes koennten Text-, Material- oder
Preisvorschlaege erhalten.

Bewertung: strategisches Kernziel, aber kein kleiner Restpunkt. Erforderlich
sind Modellvertrag, Quellenbelege, Konfidenz, Auditierbarkeit und menschliche
Freigabe.

## Entscheidung

Der kleinste fachlich wertvolle Folgeausbau ist ein separater Widgettest fuer
einen serverseitig abgewiesenen Einzelpositionsreview.

Der Zielumfang bleibt eng:

- genau ein `parsed` Import mit einer `pending`-Position
- Berechtigungen `quotes.read` und `quotes.write`
- Benutzerwahl `accepted` mit normalisierter Review-Notiz
- import-/item-spezifische `ApiException` aus
  `updateQuoteImportItemReview`
- separate Aufzeichnung von Versuch und erfolgreicher Mutation
- Entscheidungsdialog ist nach `Speichern` geschlossen
- Positionsdetaildialog bleibt geoeffnet
- `Review-Status: pending` und leere Notizdarstellung bleiben erhalten
- fachliche Fehlermeldung ist sichtbar
- kein `accepted`-Zustand und keine Erfolgssnackbar

## Technische Minimalgrenze

Der vorhandene `_FakeApiClient` besitzt bereits:

- `updatedQuoteImportItemReviews`
- ein Override fuer `updateQuoteImportItemReview(...)`
- import-/item-spezifische Detailpayloads

Analog zur Importlauf-Freigabe sollte die bestehende Liste weiterhin nur
erfolgreiche Mutationen repraesentieren. Fuer den Folgeblock sind daher
voraussichtlich erforderlich:

- eine standardmaessig leere, schluesselbasierte Fehler-Map
- eine separate Liste fuer versuchte Review-Payloads
- Fehlerpruefung vor dem Eintrag in `updatedQuoteImportItemReviews`

Runtime, `ApiClient` und Serververtrag muessen nicht erweitert werden.

## Nicht-Ziele

- noch keine Implementierung
- keine Runtime-Aenderung
- kein Importlauf-Freigabe-, Apply- oder Navigationstest
- kein Berechtigungsnegativtest
- kein zweiter erfolgreicher Reviewstatus
- kein Retry- oder Dialog-Redesign
- keine Transformations-, Mapping-, Kalkulations- oder KI-Logik
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Naechster Leaf

Subtask 3.1.67.2 definiert die Minimalstrategie fuer:

- import-/item-spezifische Reviewfehler-Konfiguration im Fake
- getrennte Versuch-/Erfolgsaufzeichnung des Payloads
- kleinsten `pending`-Positionspayload
- strukturierte `ApiException`
- stabile Finder fuer den geschlossenen Entscheidungs- und erhaltenen
  Positionsdetaildialog
- Negativassertions fuer Status, Notiz und Erfolgssignal

## Ergebnis

3.1.67.1 ist abgeschlossen. Der kleinste fachlich wertvolle Folgeausbau ist
ein isolierter Widgettest fuer einen fehlgeschlagenen Einzelpositionsreview,
der den bestehenden `pending`-Zustand unveraendert erhaelt.
