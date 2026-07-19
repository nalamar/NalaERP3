# GAEB: Folgeinventur nach der getesteten Importlauf-Freigabe

## Ziel

Subtask 3.1.62.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
dem abgesicherten Prozessuebergang eines GAEB-Importlaufs von `parsed` nach
`reviewed`. Dieser Leaf aendert keine Runtime- oder Testlogik.

## Ausgangslage

Die clientseitige GAEB-Kette ist inzwischen bis zur Apply-Bereitschaft
widgetgetestet:

```text
Importvorschau
  -> Importdetail
  -> Positionsreview
  -> Importlauf-Freigabe
  -> Status reviewed
```

Bei `reviewed` und `quotes.write` zeigt der Importdetaildialog bereits
`Draft-Quote erzeugen`. Der vorhandene Apply-Pfad:

- ruft `applyQuoteImport(importId)` auf
- erwartet eine Antwort mit `import` und `quote`
- setzt zunaechst das zurueckgegebene Importdetail
- laedt Importdetail und Positionen erneut
- aktualisiert die Angebotsliste
- zeigt eine Snackbar mit der erzeugten Angebotsnummer
- zeigt bei `created_quote_id` die erzeugte Quote und `Quote öffnen`

API- und Backendvertrag existieren bereits und sind auf Integrationsebene
abgedeckt. Im Flutter-Harness fehlt der Apply-Mutationspfad.

## Optionen

### A: Draft-Quote-Erzeugung widgettesten

Ein isolierter Test oeffnet einen `reviewed` Import, betaetigt
`Draft-Quote erzeugen` und zeichnet die Import-ID im Fake auf. Die Fake-Antwort
enthaelt einen `applied` Import mit `created_quote_id` sowie eine Draft-Quote
mit eindeutiger Nummer. Der anschliessende Detailrefresh bleibt
zustandsgebunden auf diesem Ergebnis.

Vorteile:

- sichert den zentralen GAEB-Ausgabeschritt
- belegt Import-ID und Form der Apply-Antwort
- prueft Fortschrittsroute, Refresh und Angebotslistenaktualisierung
- prueft das sichtbare Erzeugungsergebnis und Erfolgsfeedback
- vorhandene Importdetail- und Zustandsfake-Strukturen sind wiederverwendbar
- keine Runtime-, Backend- oder API-Aenderung erforderlich

Bewertung: kleinster eigenstaendiger Folgeausbau mit direktem Angebotsnutzen.

### B: Erzeugte Quote aus einem vorkonfigurierten Import oeffnen

Ein `applied` Import mit `created_quote_id` koennte `Quote öffnen` testen.

Bewertung: technisch kleiner und read-only, ueberspringt aber die noch
ungetestete Draft-Quote-Erzeugung. Die Navigation ist erst nach einem
belastbaren Apply-Pfad fachlich sinnvoll.

### C: Fehlerpfad der Importlauf-Freigabe testen

Der Fake koennte `markQuoteImportReviewed` fehlschlagen lassen und die
Fehlersnackbar pruefen.

Bewertung: sinnvoll fuer Robustheit, erschliesst aber keinen neuen
GAEB-Prozessschritt.

### D: Berechtigungsnegativtest fuer Apply

Ein `reviewed` Import mit nur `quotes.read` koennte das Fehlen von
`Draft-Quote erzeugen` belegen.

Bewertung: klein, aber die analoge Read-only-Grenze ist bereits bei
Positionsreview und Importdetail abgesichert. Geringerer Prozessnutzen als A.

### E: KI-gestuetzte automatische Angebotserzeugung

Nach GAEB-Import koennten KI-Vorschlaege Material, Preise und Reviewentscheid
vorbereiten oder den gesamten Draft automatisiert aufbauen.

Bewertung: strategisches Kernziel, aber deutlich groesser. Es benoetigt
Modellvertrag, Quellenbelege, Konfidenz, Kostenkontrolle, Auditierbarkeit und
menschliche Freigabe. Der deterministische Apply-Pfad sollte zuerst im Client
vollstaendig abgesichert sein.

## Entscheidung

Der naechste Ausbau ist ein isolierter Widgettest fuer die vorhandene
Draft-Quote-Erzeugung aus einem freigegebenen GAEB-Importlauf.

Der Zielumfang bleibt eng:

- Fake-Aufzeichnung fuer `applyQuoteImport(importId)`
- genau ein `reviewed` Importdetail
- Berechtigungen `quotes.read` und `quotes.write`
- deterministische Apply-Antwort mit Importstatus `applied`
- eindeutige `created_quote_id`
- eindeutige Draft-Quote-ID und Angebotsnummer
- zustandsgebundener `getQuoteImport`-Refresh auf das Apply-Ergebnis
- Pruefung des aktualisierten Importstatus und der erzeugten Quote
- Pruefung der Snackbar mit Angebotsnummer
- Pruefung, dass `Quote öffnen` danach sichtbar ist
- keine Ausfuehrung der Quote-Navigation

Der bestehende Importlauf-Freigabe-Test bleibt unveraendert.

## Naechster Leaf

Subtask 3.1.62.2 definiert das technische Minimalmodell fuer:

- Apply-Aufzeichnung und Antwortvertrag im Fake
- zustandsgebundenes Importdetail nach der Mutation
- kleinsten Quote- und Importpayload
- Refreshfolge von Importdetail, Positionen und Angebotsliste
- stabile Finder und Erfolgskriterien

## Nicht-Ziele

- noch keine Implementierung
- keine Runtime-Aenderung
- keine Navigation zur erzeugten Quote
- kein Apply-Fehler- oder Berechtigungsnegativtest
- keine Preis-, Material- oder Kalkulationspruefung der erzeugten Quote
- keine Backend-, API-, Datenbank- oder Permission-Aenderung
- keine KI-Automatisierung

## Ergebnis

3.1.62.1 ist abgeschlossen. Der kleinste fachlich wertvolle Folgeausbau ist
ein isolierter Widgettest fuer die vorhandene Draft-Quote-Erzeugung aus einem
`reviewed` GAEB-Import. Subtask 3.1.62.2 definiert dafuer das technische
Minimalmodell.
