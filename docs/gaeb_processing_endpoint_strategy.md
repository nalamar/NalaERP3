# GAEB: API- und Bereitstellungsstrategie fuer den Process-Endpunkt

## Ziel

Subtask 3.1.74.2 definiert den kleinsten rückwärtskompatiblen Routeranschluss
für den expliziten GAEB-Verarbeitungsauslöser. Ein konkreter GAEB-Parser bleibt
ausdrücklich ausserhalb des Leafs.

## Routerbereitstellung

NewV1Router bleibt als bestehender Aufrufpunkt unveraendert und delegiert an
einen neuen, optionsbasierten Konstruktor:

    type V1RouterOptions struct {
        GAEBImportParser quotes.GAEBImportParser
    }

    NewV1RouterWithOptions(pg, mg, rd, cfg, V1RouterOptions)

Der neue Konstruktor baut den Quote-Service wie bisher auf und reicht nur den
optionalen Parser mit WithGAEBImportParser weiter. Leere Optionen bewahren
vollstaendig das bisherige Verhalten der API und aller bestehenden Tests.
Produktiv kann ein konkreter Parser später ausschliesslich am
Zusammensetzungspunkt bereitgestellt werden; der Router implementiert ihn
nicht selbst.

## Endpunktvertrag

Der neue Endpunkt wird innerhalb der bestehenden Quotes-Import-Routen
registriert:

    POST /api/v1/quotes/imports/{id}/process

Vertrag:

- erforderlich: authentifizierter Benutzer mit quotes.write;
- Request: kein Body und keine neuen Queryparameter;
- Aktion: genau ein Aufruf von ProcessGAEBImport mit der Pfad-ID;
- Erfolg: HTTP 200 und der aktualisierte QuoteImport;
- keine neue Responseform und keine Clientänderung.

## Fehlergrenzen

Der Endpunkt verwendet writeDomainError wie die benachbarten Review- und
Apply-Aktionen.

- kein konfigurierter Parser: HTTP 500, code internal_error; der Import bleibt
  unveraendert uploaded;
- unbekannte Import-ID: HTTP 404, code not_found;
- Import nicht mehr uploaded: HTTP 409, code conflict, da ein erneuter
  Prozessversuch eine Konfliktsituation und keinen Infrastrukturfehler bildet;
- Parser- oder Quelldownloadfehler: HTTP 200 mit dem bereits als failed
  persistierten Importlauf. Der Service behandelt diesen fachlichen
  Endzustand, nicht der Router.
- fehlendes quotes.write: bestehendes Auth-Middleware-Ergebnis, HTTP 403.

Fuer den Statuskonflikt wird classifyDomainError eng um den spezifischen
Text fuer nicht mehr hochgeladene Importlaeufe ergänzt. Andere bestehende
Domainfehler bleiben unveraendert.

## Minimaltests fuer 3.1.74.3

Ein lokaler Fake-Parser wird nur über V1RouterOptions für die
HTTP-Integrationstestinstanz injiziert.

1. Ein autorisierter POST verarbeitet einen hochgeladenen Import, antwortet
   mit 200, Status parsed und persistierten Itemdaten.
2. Ein POST ohne quotes.write wird vor dem Service mit 403 abgewiesen.
3. Ein zweiter POST auf den bereits parsed Import liefert 409 conflict und
   ruft den Parser nicht erneut auf.
4. Ein Router ohne Parseroption liefert 500 internal_error und der Import
   bleibt uploaded.

Die vorhandenen Upload-, Review- und Apply-Endpunkte werden nicht angepasst.

## Nicht-Ziele

- kein konkreter Parser, keine Formaterkennung und keine Konfiguration über
  Environmentvariablen;
- kein automatischer Uploadcallback, Worker, Queue, Retry oder UI-Ausbau;
- keine neue Persistenz, Migration, KI-, Mapping-, Preis-, Kalkulations- oder
  Reviewlogik.

## Naechster Leaf

Subtask 3.1.74.3 implementiert nur Optionskonstruktor, Process-Route,
eng begrenzte Konfliktklassifikation und die vier HTTP-Integrationstests.

## Ergebnis

3.1.74.2 ist abgeschlossen. Der produktive Ausloeser bleibt testbar,
autorisierbar und vom späteren konkreten Parser sauber getrennt.
