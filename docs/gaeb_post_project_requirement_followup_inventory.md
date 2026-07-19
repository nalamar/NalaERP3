# GAEB: Folgeinventar nach abgesicherter Projektpflicht

## Ziel

Subtask 3.1.69.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
dem belegten Abbruch eines GAEB-Imports ohne Projektkontext. Dieser Leaf
aendert keine Runtime- oder Testlogik.

## Ausgangslage

Der Client-Harness deckt die Prozesskette ab dem geladenen Import inzwischen
vollstaendig fuer die zentralen Erfolgs- und Fehlerausgaenge ab. Auch der Guard
vor dem Dateipicker ist getestet.

Nicht widgetgetestet ist damit vor allem der positive Einstieg:

```text
Projekt-ID vorhanden
  -> GAEB-Datei auswaehlen
  -> Dateimetadaten und Bytes hochladen
  -> Importliste neu laden
  -> Erfolg rueckmelden
```

Der Server-Integrationstest belegt den Uploadvertrag bereits. Die offene Luecke
liegt in der Client-Orchestrierung und nicht in einem neuen Backend-Endpunkt.

## Technische Blockade

`QuotesPage._importGAEB()` ruft `browser.pickFile(...)` derzeit direkt auf.
Der bedingte Browseradapter ist korrekt fuer die Runtime, aber im Widgettest
nicht deterministisch steuerbar. `_FakeApiClient` kann den Upload erst
beobachten, nachdem eine Datei ausgewaehlt wurde.

Die kleinste saubere Testnaht ist eine optionale Picker-Funktion am
`QuotesPage`-Konstruktor:

- nullable Callback mit demselben fachlichen Ergebnis `browser.PickedFile?`
- Produktion verwendet bei `null` weiterhin `browser.pickFile`
- Tests liefern eine feste Datei ohne globale Overrides
- kein neuer Service, Provider oder Zustandscontainer

## Folgeoptionen im Vergleich

### Option A: Picker-Callback und erfolgreichen Upload widgettesten

Eine optionale, standardmaessig nicht gesetzte Picker-Funktion wird in
`QuotesPage` injiziert. Der Fake zeichnet genau das Uploadpayload auf und gibt
einen erzeugten Import zurueck. Der Widgettest belegt Accept-Filter,
Dateimetadaten, Projektbindung, Reload, geschlossenen Fortschrittsdialog und
Erfolgshinweis.

Bewertung: kleinster eigenstaendiger Folgeausbau, der einen noch unbelegten
produktiven Prozessschritt end-to-end im Client absichert.

### Option B: nur die Picker-Testnaht einfuehren

Der Callback koennte ohne Widgettest implementiert werden.

Bewertung: kleinerer Codeumfang, aber kein eigenstaendiger fachlicher Nutzen
und kein Nachweis, dass die Naht den Uploadfluss tatsaechlich abbildet.

### Option C: Uploadfehler zuerst testen

Nach injizierter Datei koennte `uploadGAEBQuoteImport(...)` eine strukturierte
Exception werfen.

Bewertung: sinnvoll als direkter Folgeblock, aber der erfolgreiche Hauptpfad
liefert zuerst das groessere Prozesssignal und definiert den Fake-Vertrag.

### Option D: globalen Browserhook einbauen

`browser.pickFile` koennte ueber einen global austauschbaren Handler laufen.

Bewertung: technisch moeglich, aber globaler Testzustand birgt Leckage zwischen
Tests und betrifft alle Dateipicker der Anwendung. Eine lokale
Widget-Abhaengigkeit ist enger und sicherer.

### Option E: Mapping oder KI-Anreicherung beginnen

Material-, Preis- oder KI-Vorschlaege koennten weiter ausgebaut werden.

Bewertung: fachlich zentral, aber groesser. Der noch offene reale Uploadpfad
sollte zuerst deterministisch abgesichert werden, bevor nachgelagerte
Automatisierung auf ihm aufbaut.

## Entscheidung

Der kleinste fachlich wertvolle Folgeausbau ist eine lokale, optionale
Picker-Testnaht in `QuotesPage` zusammen mit genau einem Widgettest fuer den
erfolgreichen projektgebundenen GAEB-Upload.

Der geplante Umfang:

- optionaler Picker-Callback nur an `QuotesPage`
- unveraenderter Produktionsfallback auf `browser.pickFile`
- fester `browser.PickedFile` mit Dateiname, Bytes und Content-Type im Test
- `_FakeApiClient` zeichnet versuchte erfolgreiche Uploadpayloads auf
- vorhandene Projekt-ID wird getrimmt und weitergereicht
- bestehender Accept-Filter wird an den Picker gegeben
- Fortschrittsdialog erscheint waehrend des Uploads und ist danach geschlossen
- Importliste wird nach Erfolg erneut geladen
- Erfolgssnackbar nennt den Dateinamen

## Zerlegung des Folgeblocks

Der Folgeblock ist fuer einen einzelnen Leaf zu gross und wird in vier
Micro-Subtasks zerlegt:

1. **3.1.69.2** – Minimalstrategie fuer Callback-, Fake- und Widgettestvertrag
   definieren.
2. **3.1.69.3** – optionale Picker-Testnaht und kleinsten Fake-Uploadvertrag
   implementieren.
3. **3.1.69.4** – genau einen erfolgreichen Upload-Widgettest implementieren
   und gezielt verifizieren.
4. **3.1.69.5** – Implementierung und Nachweise auditieren.

Damit bleibt jede Antwort auf genau eine aktuelle Subtask begrenzt.

## Nicht-Ziele

- noch keine Implementierung
- kein Uploadfehler-Test
- keine globale Browsermutation
- keine neue Architektur- oder State-Management-Schicht
- keine Backend-, API-, DB- oder Permission-Aenderung
- keine Dateigroessen- oder Inhaltsvalidierung
- keine Mapping-, Kalkulations- oder KI-Logik

## Naechster Leaf

Subtask 3.1.69.2 definiert ausschliesslich:

- Signatur und Fallback des lokalen Picker-Callbacks
- minimalen Fake-Uploadversuch und Rueckgabevertrag
- festen Datei- und Projektpayload
- stabile UI- und Orchestrierungsassertions
- gezielte Verifikationskommandos

## Ergebnis

3.1.69.1 ist abgeschlossen. Der kleinste fachlich wertvolle Folgeausbau ist
die lokal injizierbare Dateiauswahl samt erfolgreichem Upload-Widgettest; die
Umsetzung wird wegen ihres Umfangs in vier getrennte Leaves zerlegt.
