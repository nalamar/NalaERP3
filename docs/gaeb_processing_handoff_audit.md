# GAEB: Audit des Parseradapter-Verarbeitungshandoffs

## Gegenstand

Subtask 3.1.73.4 auditiert die in 3.1.73.3 implementierte synchrone
Service-Naht fuer bereits hochgeladene GAEB-Importe. Dieser Leaf aendert
weder Produktions- noch Testcode.

## Ergebnis

Das Audit ist ohne fachlichen oder technischen Befund abgeschlossen. Die
Implementierung entspricht dem Vertrag aus gaeb_processing_handoff_strategy.md
und erweitert keinen der bewusst ausgeschlossenen Ausloeser oder Parser.

## Vertragsabgleich

- GAEBImportParser erhaelt nur context, lesbare Quelle und Dateiname.
- GAEBImportParseResult transportiert Parserkennung, erkanntes Format und
  normalisierte QuoteImportItemInput-Werte, aber keine Infrastrukturdetails.
- WithGAEBImportParser bleibt kettenfaehig und ist optional.
- ProcessGAEBImport verlangt einen konfigurierten Parser und MongoDB, laedt
  danach den Importlauf und akzeptiert nur source_kind gaeb sowie Status
  uploaded.
- source_document_id wird als ObjectID validiert; die GridFS-Quelle wird
  ausschliesslich lesend geoeffnet und geschlossen.
- Ein erfolgreicher Adapterlauf ruft ausschliesslich SaveImportParseResult
  auf und fuehrt damit nach parsed.
- Parser- oder Quelldownloadfehler verwenden ausschliesslich
  MarkImportFailed und fuehren damit nach failed.
- Lokale Vorbedingungen wie fehlender Adapter mutieren den Importlauf nicht.

## Testabgleich

Die drei Integrationstests belegen:

1. Parsererfolg liest exakt Quelle und Dateiname und persistiert zwei
   Positionen samt Parsermetadaten nach parsed.
2. Parserfehler fuehrt nach failed, speichert die technische Fehlermeldung
   und hinterlaesst keine Positionen.
3. Ein fehlender Adapter liefert einen direkten Fehler; der urspruengliche
   Import bleibt uploaded und leer.

Der Parser wird in den beiden Verarbeitungsfaellen genau einmal aufgerufen.
Die Testnaht ist damit auf den Serververtrag begrenzt und simuliert kein
reales GAEB-Format.

## Regressionsnachweis

Folgende gezielte Verifikation ist gruen:

    go test ./internal/quotes -run TestProcessGAEBImport -count=1

gofmt wurde vor dem Audit ausgefuehrt; git diff --check ist sauber.

## Scope-Pruefung

Der Leaf fuegt nur dieses Audit-Artefakt und den fortgeschriebenen State hinzu.
Nicht erweitert wurden:

- konkreter GAEB-Parser oder Formaterkennung;
- HTTP-Endpunkt, Uploadcallback, Worker, Queue oder Retry;
- Client, Berechtigungen, Datenbank oder Migration;
- KI-, Mapping-, Preis-, Kalkulations- oder Reviewlogik.

## Abschluss

Subtask 3.1.73.4 ist abgeschlossen. Die gespeicherte Quelle kann jetzt ueber
eine explizite, testbare Servicegrenze in parsed oder failed ueberfuehrt
werden. Der naechste Leaf inventarisiert den kleinsten produktiven Anschluss
an diese bewusst noch nicht ausgeloeste Verarbeitungsnaht.
