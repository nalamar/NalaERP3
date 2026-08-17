# GAEB: Minimalstrategie fuer Parseradapter und Statusuebergabe

## Ziel

Subtask 3.1.73.2 schneidet den kleinsten Serververtrag, durch den ein bereits
gespeicherter GAEB-Importlauf von uploaded nach parsed oder failed uebergehen
kann. Ein konkretes GAEB-Format wird nicht implementiert.

## Bestehende Grenze

CreateGAEBImport schreibt die Quelle nach GridFS und legt quote_imports mit
source_document_id und Status uploaded an. SaveImportParseResult uebernimmt
normalisierte QuoteImportItemInput-Werte nach parsed. MarkImportFailed leert
eventuelle Rohpositionen und uebernimmt eine Fehlermeldung nach failed.

Der Handoff muss deshalb nur drei Verantwortlichkeiten zusammenfuehren:

1. vorhandenen Importlauf und seine GridFS-Quelle lesen;
2. einen austauschbaren Parseradapter aufrufen;
3. genau einen der bestehenden Statusschreibpfade verwenden.

## Adaptervertrag

Der Contract gehoert in das quotes-Package, damit der Importlauf Eigentum der
Verarbeitungsstrecke bleibt:

    type GAEBImportParser interface {
        ParseGAEB(ctx context.Context, source io.Reader, filename string) (
            GAEBImportParseResult, error,
        )
    }

    type GAEBImportParseResult struct {
        ParserVersion string
        DetectedFormat string
        Items         []QuoteImportItemInput
    }

Der Adapter erhaelt nur einen lesbaren Quelldatenstrom und den urspruenglichen
Dateinamen. Er kennt weder Postgres, GridFS, Import-IDs noch Review- oder
Quote-Zustaende. Diese Trennung ermoeglicht spaeter X83-, X84-, D83-, P83-
oder XML-spezifische Adapter sowie KI-gestuetzte Erweiterungen ohne
Ausweitung des Importservice.

## Servicevertrag

Der Service bekommt einen optionalen, kettenfaehigen Konfigurator:

    WithGAEBImportParser(parser GAEBImportParser) *Service

und genau eine explizite Verarbeitungseinstiegsmethode:

    ProcessGAEBImport(ctx context.Context, importID string) (*QuoteImport, error)

Sie wird in diesem Block noch nicht an einen HTTP-Endpunkt, Uploadcallback,
Worker oder Retrymechanismus angeschlossen. Der Aufrufer bestimmt damit
bewusst den Zeitpunkt der synchronen Verarbeitung.

## Verarbeitungsfolge

ProcessGAEBImport muss:

1. eine konfigurierte Parserinstanz verlangen;
2. den Importlauf laden und nur source_kind gaeb sowie Status uploaded
   akzeptieren;
3. source_document_id als Mongo ObjectID validieren;
4. den zugehoerigen GridFS-Stream oeffnen und nach dem Parse schliessen;
5. ParseGAEB genau einmal aufrufen;
6. bei Erfolg SaveImportParseResult mit ParserVersion, DetectedFormat und
   Items aufrufen;
7. bei Parser- oder Quelllesefehler MarkImportFailed aufrufen und dessen
   Ergebnis zurueckgeben.

Die Fehleruebergabe nutzt im ersten Schnitt eine sichere technische
Parserkennung wie adapter-v1 und einen aus Dateiendung oder leerem Wert
ableitbaren detected_format-Wert. Ein kuenftiger konkreter Parser kann diese
Werte ohne Serviceumbau praezisieren.

Nicht als failed markiert werden lokale Vorbedingungen vor dem Leseversuch:
fehlende Parserinstanz, ungueltige ID, nicht vorhandener Importlauf, falscher
source_kind oder ein Status ungleich uploaded. Sie bleiben direkte Fehler,
damit bereits verarbeitete oder fachlich fremde Importlaeufe nicht durch einen
erneuten Aufruf mutiert werden.

## Minimaltests fuer 3.1.73.3

Ein Fake-Parser implementiert den Adaptervertrag und zeichnet Dateiname sowie
Quellbytes auf.

- Erfolgsfall: Ein uploadter GAEB-Import mit gespeicherter Quelle wird genau
  einmal geparst; Status, Parsermetadaten und zwei Itempositionen sind parsed.
- Fehlerfall: Derselbe Ablauf liefert einen Parserfehler; der Import wird
  failed, enthaelt die Fehlermeldung und keine Items.
- Guard: Ein nicht-uploadter Import oder fehlender Adapter beendet sich ohne
  Statusmutation und ohne Parseraufruf.

Die Tests liegen bei server/internal/quotes/imports_test.go und verwenden die
bestehende Postgres-/Mongo-Integrationstestumgebung.

## Nicht-Ziele

- kein konkreter GAEB-Parser und keine Formatheuristik;
- kein HTTP-Endpunkt, Uploadcallback, Worker, Queue, Retry oder UI-Ausbau;
- keine KI-, Mapping-, Preis-, Kalkulations- oder Review-Aenderung;
- keine neue Persistenz oder Migration;
- keine GridFS-Kompensation.

## Naechster Leaf

Subtask 3.1.73.3 implementiert ausschliesslich Adaptertypen, die explizite
Serviceverarbeitungsnaht und die drei beschriebenen Integrationstests.

## Ergebnis

3.1.73.2 ist abgeschlossen, sobald der Adapter- und Statusvertrag
dokumentiert ist. Die nachfolgende Implementierung bleibt auf eine
deterministische synchrone Servicegrenze begrenzt.
