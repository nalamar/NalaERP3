# GAEB: Folgeinventar nach abgeschlossener Apply- und Navigationsabsicherung

## Ziel

Subtask 3.1.66.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
der getrennten Widgettest-Absicherung von erfolgreichem und fehlgeschlagenem
Apply sowie erfolgreicher und fehlgeschlagener Navigation zur erzeugten
Draft-Quote. Dieser Leaf aendert keine Runtime- oder Testlogik.

## Ausgangslage

Die Clientstrecke deckt inzwischen folgende kontrollierte Uebergaenge ab:

```text
Importpositionen bewerten
  -> Importlauf erfolgreich freigeben
  -> Draft-Quote erfolgreich erzeugen oder Apply-Fehler erhalten
  -> erzeugte Quote erfolgreich oeffnen oder Ladefehler erhalten
```

Erfolgs- und Fehlerausgang nach dem Status `reviewed` sind damit
deterministisch getestet.

## Verbleibende unmittelbare Luecke

Die unmittelbar vorgelagerte Freigabe besitzt ebenfalls einen vorhandenen,
aber noch nicht widgetgetesteten Fehlerpfad:

- ein Importlauf steht auf `parsed`
- mindestens eine Position ist noch `pending`
- `Zur Übernahme freigeben` ist im Client sichtbar
- `markQuoteImportReviewed(importId)` wird serverseitig abgewiesen
- der Fortschrittsdialog muss geschlossen werden
- der Importdetaildialog und Status `parsed` muessen erhalten bleiben
- die Review-Summary muss weiterhin die offene Position zeigen
- die fachliche Fehlermeldung muss sichtbar sein
- `Draft-Quote erzeugen` darf nicht erscheinen

Dieser Fall bildet eine zentrale fachliche Guard Rail ab: Ein Import darf erst
nach vollstaendiger Positionsentscheidung freigegeben werden.

## Folgeoptionen im Vergleich

### Option A: Freigabefehler bei offener Position widgettesten

Ein `parsed` Import mit `pending_count: 1` wird mit `quotes.read` und
`quotes.write` geoeffnet. `markQuoteImportReviewed` wirft fuer genau diese
Import-ID eine strukturierte 409-`ApiException`. Der Test belegt den
Freigabeaufruf, den erhaltenen Importzustand, die unveraenderte Summary, das
Fehlerfeedback und das Ausbleiben der Apply-Aktion.

Vorteile:

- sichert eine zentrale serverseitige Freigaberegel im Nutzerfluss ab
- verhindert einen falschen `reviewed`- oder Apply-Eindruck
- verwendet ausschliesslich bestehende Runtime- und API-Vertraege
- ist auf eine Fake-Fehler-Map und einen Widgettest begrenzbar
- keine Backend-, API-, DB- oder Permission-Aenderung erforderlich

Bewertung: kleinster Folgeausbau mit direktem fachlichem Schutzsignal.

### Option B: einfacher Berechtigungsnegativtest

Ohne `quotes.write` koennte die verborgene Freigabeaktion geprueft werden.

Bewertung: kleiner, aber mit geringerem Signal. Das Permission-Gate ist eine
direkte boolesche Bedingung; die abgewiesene Freigabe prueft dagegen eine
echte fachliche Guard Rail und den Fehlerzustand des Dialogs.

### Option C: Fehler beim Review einer einzelnen Importposition

`updateQuoteImportItemReview` koennte fehlschlagen und den unveraenderten
Positionszustand belegen.

Bewertung: sinnvoll, aber tiefer im Positionsdialog. Die Importlauf-Freigabe
ist der naehere Prozessanschluss an den gerade abgeschlossenen Apply-Block und
entscheidet direkt ueber dessen Erreichbarkeit.

### Option D: Transformationskorrektheit zwischen Import und Quote pruefen

Akzeptierte GAEB-Positionen koennten gegen erzeugte Quote-Positionen auf
Beschreibung, Menge, Einheit und Preis verglichen werden.

Bewertung: hoher fachlicher Wert, aber ein eigener Backend-/Mappingblock mit
realistischen Payloads und Regeln. Er ist groesser als die noch offene
Freigabehaertung.

### Option E: KI-gestuetzte Angebotsanreicherung beginnen

Die Draft-Quote koennte automatisch Text-, Material- oder Preisvorschlaege
erhalten.

Bewertung: strategisches Ziel, aber kein kleiner Anschluss. Modellvertrag,
Quellenbelege, Konfidenz, Auditierbarkeit und menschliche Freigabe muessen
separat zugeschnitten werden.

## Entscheidung

Der kleinste fachlich wertvolle Folgeausbau ist ein separater Widgettest fuer
die serverseitig abgewiesene Importlauf-Freigabe bei einer offenen Position.

Der Zielumfang bleibt eng:

- genau ein `parsed` Import
- `accepted_count: 0`, `rejected_count: 0`, `pending_count: 1`
- Berechtigungen `quotes.read` und `quotes.write`
- import-ID-spezifische 409-`ApiException` aus
  `markQuoteImportReviewed`
- Aufzeichnung der Freigabe-Import-ID
- Fortschrittsdialog wird beendet
- Importdetaildialog bleibt geoeffnet
- Status und Review-Summary bleiben unveraendert
- `Zur Übernahme freigeben` bleibt sichtbar
- fachliche Fehlermeldung ist sichtbar
- `Draft-Quote erzeugen` und Erfolgsfeedback bleiben verborgen

## Technische Minimalgrenze

Der vorhandene `_FakeApiClient` besitzt bereits:

- `reviewedQuoteImportIds`
- ein Override fuer `markQuoteImportReviewed`
- import-ID-spezifische Detailpayloads

Fuer den Folgeblock reicht voraussichtlich eine standardmaessig leere,
import-ID-spezifische Review-Fehler-Map analog zu
`quoteImportApplyErrors`. Runtime, `ApiClient` und Serververtrag muessen nicht
erweitert werden.

## Nicht-Ziele

- noch keine Implementierung
- keine Runtime-Aenderung
- kein Apply- oder Navigationstest
- kein Berechtigungsnegativtest
- kein Positionsreview-Fehler
- kein Retry- oder Dialog-Redesign
- keine Transformations-, Mapping-, Kalkulations- oder KI-Logik
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Naechster Leaf

Subtask 3.1.66.2 definiert die Minimalstrategie fuer:

- import-ID-spezifische Freigabefehler-Konfiguration im Fake
- kleinsten `parsed` Import mit einer offenen Position
- strukturierte 409-`ApiException`
- stabile Finder fuer erhaltenen Dialog, Status und Review-Summary
- Negativassertions fuer `reviewed`, Apply-Aktion und Erfolgssignal
- klare Abgrenzung gegen Positionsreview-, Backend- und Mappingtests

## Ergebnis

3.1.66.1 ist abgeschlossen. Der kleinste fachlich wertvolle Folgeausbau ist
ein isolierter Widgettest fuer eine wegen offener Positionen serverseitig
abgewiesene Importlauf-Freigabe.
