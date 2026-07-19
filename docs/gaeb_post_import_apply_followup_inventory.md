# GAEB: Folgeinventur nach der getesteten Draft-Quote-Erzeugung

## Ziel

Subtask 3.1.63.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
der abgesicherten Draft-Quote-Erzeugung aus einem GAEB-Importlauf. Dieser Leaf
aendert keine Runtime- oder Testlogik.

## Ausgangslage

Die clientseitige GAEB-Kette ist inzwischen bis zum sichtbaren
Erzeugungsergebnis widgetgetestet:

```text
Importvorschau
  -> Positionsreview
  -> Importlauf-Freigabe
  -> Draft-Quote erzeugen
  -> Status applied + created_quote_id
```

Bei gesetzter `created_quote_id` und `quotes.read` zeigt der Importdetaildialog
bereits `Quote öffnen`. Die vorhandene Aktion:

- schliesst den Importdetaildialog
- setzt Status- und Folgebelegfilter zurueck
- ruft `getQuote(createdQuoteId)` auf
- setzt die geladene Quote als `_selected`
- laedt die Angebotsseite neu
- zeigt `Erzeugte Quote wurde geöffnet`

Der Draft-Quote-Erzeugungstest prueft die Sichtbarkeit dieser Aktion, fuehrt
sie aber bewusst nicht aus. Im Flutter-Harness fehlt damit nur noch der
Navigationstest fuer den direkten Uebergang in die erzeugte Quote.

## Optionen

### A: Erzeugte Draft-Quote aus dem Importdetail oeffnen

Ein isolierter Test oeffnet einen bereits `applied` Import mit
`created_quote_id`, betaetigt `Quote öffnen` und zeichnet die angefragte
Quote-ID im Fake auf. Das eindeutige Quote-Detail wird danach in der rechten
Angebotsansicht sichtbar.

Vorteile:

- schliesst die GAEB-Prozesskette bis zur Angebotsbearbeitung
- belegt `created_quote_id` als Navigationsvertrag
- prueft Dialogschluss, Detailauswahl, Reload und Erfolgsfeedback
- verwendet nur bestehende Client- und API-Vertraege
- kann als separater read-only Widgettest umgesetzt werden
- keine Runtime-, Backend- oder API-Aenderung erforderlich

Bewertung: kleinster eigenstaendiger Folgeausbau mit direktem Arbeitsfluss.

### B: Quote-Navigation an den Apply-Test anhaengen

Der bestehende Apply-Test koennte nach dem Sichtbarwerden von `Quote öffnen`
weiter navigieren.

Bewertung: weniger Testcode, vermischt aber Apply-Mutation und Navigation.
Ein Fehler waere schlechter lokalisierbar und der bisher enge Scope ginge
verloren.

### C: Apply-Fehlerpfad testen

Der Fake koennte `applyQuoteImport` fehlschlagen lassen und die
Fehlersnackbar pruefen.

Bewertung: sinnvoll fuer Robustheit, schliesst aber nicht den erfolgreichen
GAEB-Arbeitsfluss zur erzeugten Quote.

### D: Inhalt der erzeugten Angebotspositionen validieren

Der Test koennte GAEB-Positionen, Mengen, Preise oder Materialbezug in der
erzeugten Quote pruefen.

Bewertung: fachlich wichtig, aber deutlich groesser. Es benoetigt einen
realistischen Quote-Detailpayload und konkrete Transformationsregeln. Die
Navigation sollte zuerst isoliert abgesichert sein.

### E: KI-gestuetzte Angebotsanreicherung

Die erzeugte Draft-Quote koennte automatisch Material-, Preis- oder
Textvorschlaege aus KI erhalten.

Bewertung: strategisches Kernziel, benoetigt aber Modellvertrag,
Quellenbelege, Konfidenz, Auditierbarkeit und menschliche Freigabe. Der
deterministische End-to-End-Clientpfad sollte zuerst geschlossen werden.

## Entscheidung

Der naechste Ausbau ist ein separater read-only Widgettest fuer die Navigation
vom angewendeten GAEB-Import zur erzeugten Draft-Quote.

Der Zielumfang bleibt eng:

- genau ein `applied` Importdetail
- `created_quote_id: quote-gaeb-open-1`
- Berechtigung `quotes.read`
- Aufzeichnung der an `getQuote` uebergebenen ID
- eindeutiges Quote-Detail mit Nummer `ANG-GAEB-OPEN-0001`
- Oeffnung ueber `Quote öffnen`
- Pruefung, dass der Importdialog geschlossen ist
- Pruefung der ausgewaehlten Quote in der Detailansicht
- Pruefung der Snackbar `Erzeugte Quote wurde geöffnet`
- keine Bearbeitung, Freigabe oder weitere Konvertierung der Quote

Der bestehende Apply-Test bleibt unveraendert.

## Naechster Leaf

Subtask 3.1.63.2 definiert das technische Minimalmodell fuer:

- Quote-ID-Aufzeichnung im Fake
- kleinsten Import- und Quote-Detailpayload
- Verhalten von `listQuotes` und `_load()` nach der Auswahl
- stabile Finder fuer Importdialog und Quote-Detailansicht
- Erfolgskriterien fuer Dialogschluss und Navigation

## Nicht-Ziele

- noch keine Implementierung
- keine Runtime-Aenderung
- keine erneute Apply-Mutation
- keine Quote-Bearbeitung oder Freigabe
- keine Positions-, Preis-, Material- oder Kalkulationspruefung
- kein Fehler- oder Berechtigungsnegativtest
- keine Backend-, API-, Datenbank- oder Permission-Aenderung
- keine KI-Automatisierung

## Ergebnis

3.1.63.1 ist abgeschlossen. Der kleinste fachlich wertvolle Folgeausbau ist
ein isolierter Widgettest fuer die Navigation vom `applied` GAEB-Import zur
erzeugten Draft-Quote. Subtask 3.1.63.2 definiert dafuer das technische
Minimalmodell.
