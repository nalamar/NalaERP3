# GAEB: Folgeinventar nach abgeschlossener Erzeugte-Quote-Navigation

## Ziel

Subtask 3.1.65.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
der abgeschlossenen Erfolgs- und Fehlerabsicherung der direkten Navigation von
einem `applied` GAEB-Import zur erzeugten Draft-Quote. Dieser Leaf aendert
keine Runtime- oder Testlogik.

## Ausgangslage

Die clientseitige GAEB-Kette ist inzwischen fuer folgende Schritte gezielt
widgetgetestet:

```text
Positionsreview
  -> Importlauf freigeben
  -> Draft-Quote erfolgreich erzeugen
  -> created_quote_id anzeigen
  -> erzeugte Quote erfolgreich oeffnen
  -> nicht ladbare erzeugte Quote kontrolliert melden
```

Damit sind der erfolgreiche Prozessuebergang und beide unmittelbaren
Navigationsausgaenge abgesichert.

## Verbleibende unmittelbare Luecke

Vor der Navigation bleibt ein bereits vorhandener, aber im Flutter-Harness
nicht gezielt belegter Fehlerpfad:

- ein Importlauf steht auf `reviewed`
- `Draft-Quote erzeugen` ist verfuegbar
- `applyQuoteImport(importId)` wird serverseitig abgewiesen
- der Fortschrittsdialog muss geschlossen werden
- der Importdetaildialog und Status `reviewed` muessen erhalten bleiben
- die fachliche Fehlermeldung muss sichtbar sein
- es darf weder `created_quote_id` noch Erfolgssignal oder
  `Quote öffnen` erscheinen

Dieser Fall ist operativ relevant, weil serverseitige Guard Rails unter
anderem doppelte Anwendung, fehlende akzeptierte Positionen oder einen
ungueltigen Status ablehnen koennen.

## Folgeoptionen im Vergleich

### Option A: Apply-Fehlerpfad als isolierten Widgettest absichern

Ein `reviewed` Import wird mit `quotes.read` und `quotes.write` geoeffnet.
`applyQuoteImport` wirft fuer genau diese Import-ID eine strukturierte
`ApiException`. Der Test belegt Mutationaufruf, erhaltenen Dialogzustand,
Fehlerfeedback und das Ausbleiben jedes Erzeugungsergebnisses.

Vorteile:

- sichert den letzten unmittelbaren Fehlerausgang des Erzeugungspfads ab
- verhindert falschen Status- oder Erfolgseindruck nach fehlgeschlagener
  Mutation
- nutzt ausschliesslich bestehende Runtime- und API-Vertraege
- bleibt auf einen Fake-Zweig und einen Widgettest begrenzbar
- keine Backend-, API-, DB- oder Permission-Aenderung erforderlich

Bewertung: kleinster eigenstaendiger Folgeausbau mit hohem Robustheitssignal.

### Option B: Berechtigungsnegativtest fuer Apply oder Navigation

Die Sichtbarkeit von Aktionen koennte ohne `quotes.write` beziehungsweise
`quotes.read` geprueft werden.

Bewertung: klein, aber mit geringerem neuem Signal. Die Aktionsbedingungen
sind direkte boolesche Permission-Gates; ohne `quotes.read` wird zudem bereits
die gesamte Importliste nicht geladen. Der Apply-Fehler prueft einen
realistischeren Laufzeitausgang.

### Option C: Import- und Quote-Positionen fachlich vergleichen

Der erzeugte Quote-Payload koennte gegen akzeptierte GAEB-Positionen auf
Beschreibung, Menge, Einheit und Preis geprueft werden.

Bewertung: fachlich wertvoll, aber ein eigener Transformations- und
Mappingblock. Er benoetigt realistische Payloads, Regeln und wahrscheinlich
Backend-Integrationstests statt nur eines kleinen Clienttests.

### Option D: Retry- oder Recovery-UX fuer Apply entwickeln

Nach einem Fehler koennten Retry, Fehlerklassifikation oder detaillierte
Handlungsempfehlungen hinzukommen.

Bewertung: erst sinnvoll, nachdem das bestehende Verhalten stabil getestet
ist. Eine neue UX waere groesser als der aktuelle Folgepunkt.

### Option E: KI-gestuetzte Angebotsanreicherung beginnen

Die erzeugte Draft-Quote koennte Text-, Material- oder Preisvorschlaege aus KI
erhalten.

Bewertung: strategisches Ziel, aber kein kleiner Anschluss. Modellvertrag,
Quellenbelege, Konfidenz, Auditierbarkeit und menschliche Freigabe muessen
zuvor fachlich zugeschnitten werden.

## Entscheidung

Der kleinste fachlich wertvolle Folgeausbau ist ein isolierter Widgettest fuer
einen serverseitig abgewiesenen Apply-Versuch.

Der Zielumfang bleibt eng:

- genau ein `reviewed` Import
- Berechtigungen `quotes.read` und `quotes.write`
- ziel-ID-spezifische `ApiException` aus `applyQuoteImport`
- Aufzeichnung der angewendeten Import-ID
- Fortschrittsdialog wird beendet
- Importdetaildialog bleibt geoeffnet
- Status bleibt `reviewed`
- `Draft-Quote erzeugen` bleibt verfuegbar
- strukturierte Fehlermeldung ist sichtbar
- kein `created_quote_id`, kein `Quote öffnen`, keine Erfolgssnackbar

## Technische Minimalgrenze

Der vorhandene `_FakeApiClient` besitzt bereits:

- `quoteImportApplyResults`
- `appliedQuoteImportIds`
- ein Override fuer `applyQuoteImport`

Fuer den Folgeblock reicht voraussichtlich eine standardmaessig leere,
ziel-ID-spezifische Fehler-Map analog zu `quoteDetailErrors`. Runtime,
`ApiClient` und Serververtrag muessen nicht erweitert werden.

## Nicht-Ziele

- noch keine Implementierung
- keine Runtime-Aenderung
- kein Navigationstest
- kein Berechtigungsnegativtest
- kein Retry- oder Dialog-Redesign
- keine Positions-, Preis-, Material- oder Kalkulationspruefung
- kein Mapping- oder KI-Ausbau
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Naechster Leaf

Subtask 3.1.65.2 definiert die Minimalstrategie fuer:

- ziel-ID-spezifische Apply-Fehlerkonfiguration im Fake
- kleinsten `reviewed` Importpayload
- strukturierte `ApiException`
- stabile Finder fuer erhaltenen Importdialog und Status
- Negativassertions fuer Erzeugungsergebnis, Navigation und Erfolgssignal
- klare Abgrenzung gegen Runtime-, Backend- und Mappingtests

## Ergebnis

3.1.65.1 ist abgeschlossen. Der kleinste fachlich wertvolle Folgeausbau ist
ein separater Widgettest fuer einen fehlgeschlagenen Apply-Versuch, der den
bestehenden Importzustand erhaelt und ein fachliches Fehlerfeedback zeigt.
