# GAEB: Folgeinventar nach abgesicherter Picker-Grenze

## Ziel

Subtask 3.1.72.1 inventarisiert den kleinsten verbleibenden Risikobereich
am bereits abgesicherten GAEB-Upload-Eingang. Dieser Leaf aendert weder
Produktiv- noch Testlogik.

## Abgesicherter Stand

Die Widgettests fuer `QuotesPage._importGAEB()` belegen inzwischen getrennt:

- fehlende Projekt-ID beendet den Ablauf vor dem Dateipicker;
- ein vom Benutzer abgebrochener Picker beendet ihn ohne Mutation oder
  Feedback;
- eine ausgewaehlte Datei wird mit normalisiertem Projekt-, Datei- und
  Content-Type-Payload hochgeladen, danach wird die Importliste neu geladen;
- eine strukturierte `ApiException` beendet den Fortschrittsdialog, behaelt
  die Liste bei und zeigt die fachliche Servermeldung.

Damit sind beide nicht mutierenden Eintrittsgrenzen sowie der fachliche
Erfolgs- und Ablehnungsausgang abgedeckt.

## Kleinste verbleibende Luecke

Der Catch-Pfad behandelt neben `ApiException` auch technische oder unerwartete
Fehler ueber den Fallback `GAEB-Upload fehlgeschlagen`:

```text
Datei ausgewaehlt
  -> Fortschrittsdialog geoeffnet
  -> uploadGAEBQuoteImport wirft keinen ApiException-Fehler
  -> Fortschrittsdialog wird geschlossen
  -> generische Fehlermeldung wird angezeigt
  -> kein Importlisten-Reload und kein Erfolgshinweis
```

Dieser Fallback ist gegenueber dem bereits belegten 422-Pfad der kleinste
noch ungetestete Endzustand des vorhandenen Uploadvertrags. Ohne Test koennte
eine Regression den Dialog offen lassen, unerwartet einen Erfolg melden oder
die Importliste trotz fehlgeschlagener Mutation neu laden.

## Optionen im Vergleich

### Option A: Generischen technischen Uploadfehler widgettesten

Der vorhandene `_FakeApiClient.quoteImportUploadErrors`-Vertrag akzeptiert
bereits `Object`; eine dateinamenspezifische `StateError`-Instanz reicht daher
ohne Fake- oder Runtime-Ausbau. Ein Test kann denselben finalen
Zustandserhalt wie beim 422-Fall gegen die feste Fallbackmeldung pruefen.

Bewertung: kleinster vollstaendiger Rest des vorhandenen Catch-Vertrags.

### Option B: Fortschrittsdialog im Zwischenzustand pruefen

Ein angehaltener Upload-Future koennte den sichtbaren Dialog vor einem
Endzustand pruefen.

Bewertung: asynchron fragiler; Erfolg und beide Fehlerendzustaende geben der
fachlichen Korrektheit mehr Signal.

### Option C: Clientseitige Dateiinhalt- oder Groessenvalidierung einfuehren

Leere oder grosse Dateien koennten vor dem Upload abgewiesen werden.

Bewertung: benoetigt neue, mit API und Proxy abgestimmte Fachgrenzen und ist
kein isolierter Testrest.

### Option D: Parsing, Mapping oder KI-Vorschlaege erweitern

Die Verarbeitung nach dem Upload kann fachlich vertieft werden.

Bewertung: wichtig, aber ein eigener wesentlich groesserer Ausbaustrang;
der existierende Eingang sollte zuerst in allen Endzustaenden belastbar sein.

## Entscheidung und Zuschnitt

Der naechste Leaf bleibt bei einem einzigen generischen Fehlerfall im
existierenden Widget-Harness:

- `quotes.read` und `quotes.write` sowie feste Projekt-ID;
- lokaler Picker mit einer eindeutigen X83-Datei;
- vorhandene `quoteImportUploadErrors`-Map mit einer dateinamenspezifischen
  `StateError`-Instanz;
- vollstaendig aufgezeichneter Uploadversuch vor dem Fehler;
- genau ein initialer Importlistenabruf;
- geschlossener Fortschrittsdialog und kein `AlertDialog`;
- sichtbarer Fallback `GAEB-Upload fehlgeschlagen`;
- kein Erfolgshinweis und keine zweite Listenabfrage.

`_quoteErrorMessage` und der bestehende Fake-Vertrag werden dabei nur
verwendet, nicht veraendert.

## Zerlegung

Task 3.1.72 wird in drei weitere Leaves zerlegt:

1. **3.1.72.2** – Minimalstrategie fuer den generischen Uploadfehler und
   seine stabilen Assertions definieren.
2. **3.1.72.3** – Genau einen Widgettest fuer den generischen
   Uploadfehler implementieren und verifizieren.
3. **3.1.72.4** – Implementierung und Nachweise auditieren.

## Nicht-Ziele

- keine Runtime-, API-, Backend-, DB- oder Permission-Aenderung;
- kein weiterer strukturierter `ApiException`-Fall;
- kein angehaltener Future oder Zwischenframe-Test;
- keine Dateiinhalt- oder Groessenregel;
- kein Parsing-, Mapping-, Kalkulations- oder KI-Ausbau.

## Naechster Leaf

Subtask 3.1.72.2 definiert ausschliesslich Fehlerobjekt, Testdaten,
Zustands- und Negativassertions sowie die gezielten Verifikationskommandos.

## Ergebnis

3.1.72.1 ist abgeschlossen. Der generische technische Uploadfehler ist der
kleinste verbliebene Endzustand am bestehenden GAEB-Upload-Eingang und kann
ohne Produktivcodeaenderung separat abgesichert werden.
