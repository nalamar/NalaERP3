# GAEB-Margenanker: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit schliesst den engen Block des read-only Margenankers ab.

Geprueft wird bewusst nur:

- ob der aktuelle Positionspreis gegen die letzte gespeicherte
  Preisentscheidung gelesen werden kann
- ob Backend, API und Client dieselbe read-only Semantik abbilden
- ob innerhalb dieses Blocks noch ein kleiner Haertungsschritt mit gutem Signal
  uebrig ist

## 1. Ausgangspunkt

Vor diesem Block waren bereits vorhanden:

- explizite Primaerpreis-Uebernahme
- persistierter Snapshot in `quote_item_price_decisions`
- read-only Preisentscheidungs-Transparenz
- read-only Historie gespeicherter Preisentscheidungen

Die danach identifizierte Luecke war:

- Nutzer sahen die gespeicherte Preisentscheidung
- Nutzer sahen den aktuellen Positionspreis
- Nutzer sahen aber noch kein kalkulationsnahes Verhaeltnis zwischen beiden

## 2. Umgesetzter enger Scope

Der umgesetzte Scope bleibt bewusst klein:

- read-only Service fuer genau eine Quote-Position
- read-only API-Endpunkt
- Client-API und positionsnahes Draft-Modell
- Anzeige im bestehenden Quote-Editor
- keine neue Persistenz
- keine Schreibaktion
- keine Zielmarge
- kein Zuschlagsmodell
- keine Rabattlogik
- keine Freigabe
- keine Bulk-Aktion
- keine Automatik

Der Margenanker beantwortet nur:

- was ist der aktuelle Positionspreis?
- was ist die letzte gespeicherte Kostenbasis?
- wie hoch ist die absolute und prozentuale Marge?
- welcher einfache Status ergibt sich daraus?

## 3. Ergebnis im Backend

Das Backend bildet den Block eng ab:

- DTO `QuoteItemMarginAnchor`
- Service `MarginAnchorForQuoteItem(...)`
- Route `GET /api/v1/quotes/{id}/items/{itemID}/margin-anchor`
- Kostenbasis aus `quote_item_price_decisions.applied_unit_price`
- Sortierung nach `created_at desc`
- Begrenzung auf die letzte gespeicherte Entscheidung
- kein Live-Fallback auf Material- oder Bestellhistorie

Der Integrationstest prueft:

- erfolgreichen Margenanker nach Primaerpreis-Uebernahme
- Kostenbasis, aktuellen Positionspreis, Marge, Prozentwert und Status
- Entscheidungsmetadaten aus dem gespeicherten Snapshot
- fachlichen Fehler ohne gespeicherte Preisentscheidung

## 4. Ergebnis im Client

Der Client spiegelt den Block positionsnah:

- `getQuoteItemMarginAnchor(...)`
- `_QuoteItemMarginAnchorDraft`
- Felder `marginAnchor` und `marginAnchorPerformed` an der Position
- Ladezustand `_loadingMarginAnchorItemId`
- Handler `_loadMarginAnchor(...)`
- read-only Block `Marge` im bestehenden Quote-Editor

Der Block zeigt:

- Status
- absolute und prozentuale Marge
- aktuellen Positionspreis
- Kostenbasis
- Entscheidungstyp
- Quelle
- Referenz und Quelldatum
- Entscheidungszeit

## 5. Bewusst nicht umgesetzt

Weiterhin ausserhalb dieses Blocks bleiben:

- Bearbeiten der Kostenbasis
- Setzen einer Zielmarge
- Zuschlagsregel
- Rabattlogik
- Freigabe- oder Eskalationsworkflow
- Aggregation ueber mehrere Positionen
- automatische Preisveraenderung
- KI-gestuetzte Kalkulation

Diese Themen waeren jeweils eigenstaendige Folgeausbauten.

## 6. Audit-Entscheidung

Der Block ist abgeschlossen.

Innerhalb dieses engen Margenankerblocks gibt es keinen weiteren kleinen
Haertungsschritt mit gutem Signal, der vor Zielmarge, Zuschlag, Rabatt,
Freigabe, Bulk oder Automatik noch sinnvoll waere.

Eine zusaetzliche Schreibaktion, Zielmarge oder Regeldefinition wuerde bereits
einen neuen Kalkulations- oder Freigabeblock starten. Eine zusaetzliche
Detailhistorie wuerde wieder in den Auditstrang wechseln. Das Minimalziel ist
erfuellt: die gespeicherte Preisentscheidung ist als Kostenbasis
kalkulationsnah sichtbar.

## 7. Verifikation

Verifiziert wurden:

- `go test ./internal/quotes ./internal/http`
- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

Beide Pruefungen waren gruen.

## 8. Naechster sinnvoller Schritt

Nach sichtbarem read-only Margenanker ist der naechste Schritt wieder eine
fachliche Inventur:

- ist jetzt ein minimaler Abweichungs-/Freigabeanker sinnvoll?
- oder braucht es zuerst Zielmarge/Zuschlagslogik?
- oder soll der Block in Richtung Angebotskalkulation auf Positionsebene
  erweitert werden?

Die Inventur muss entscheiden, welcher Folgeausbau nach sichtbarer Marge den
hoechsten Signalwert hat.
