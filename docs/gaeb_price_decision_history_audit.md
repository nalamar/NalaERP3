# GAEB-Preisentscheidungs-Historie: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit schliesst den engen Block der read-only
Preisentscheidungs-Historie ab.

Geprueft wird bewusst nur:

- ob gespeicherte Preisentscheidungs-Snapshots fuer genau eine Quote-Position
  sichtbar werden
- ob Backend und Client dieselbe read-only Semantik abbilden
- ob vor Historienbearbeitung, Marge, Zuschlag, Rabatt, Freigabe, Bulk oder
  Automatik noch ein weiterer kleiner Haertungsschritt mit gutem Signal
  uebrig ist

## 1. Ausgangspunkt

Vor diesem Block waren bereits vorhanden:

- explizite Primaerpreis-Uebernahme
- persistierter Snapshot in `quote_item_price_decisions`
- read-only Preisentscheidungs-Transparenz

Die danach identifizierte Luecke war:

- der Snapshot existierte technisch
- Nutzer konnten ihn im bestehenden Quote-Editor aber noch nicht lesen

## 2. Umgesetzter enger Scope

Der umgesetzte Scope bleibt bewusst klein:

- read-only Service fuer genau eine Quote-Position
- read-only API-Endpunkt
- Anzeige im bestehenden Quote-Editor
- keine neue Persistenz
- keine Schreibaktion
- keine Historienbearbeitung
- keine Kalkulations- oder Workflowlogik

Die Historie beantwortet nur:

- welche Preisentscheidung wurde gespeichert?
- welche Quelle und welcher Preis wurden gespeichert?
- wann wurde die Entscheidung gespeichert?

## 3. Ergebnis im Backend

Das Backend bildet den Block eng ab:

- `PriceDecisionHistoryEntry`
- `PriceDecisionHistoryForQuoteItem(...)`
- `GET /api/v1/quotes/{id}/items/{itemID}/price-decision-history`
- Query auf `quote_item_price_decisions` fuer genau eine Position
- Sortierung nach `created_at desc`
- enge Begrenzung auf maximal 10 Eintraege

Der Integrationstest prueft:

- einen History-Eintrag nach Primaerpreis-Uebernahme
- eine leere Historie fuer eine Position ohne Snapshot

## 4. Ergebnis im Client

Der Client spiegelt den Block positionsnah:

- `getQuoteItemPriceDecisionHistory(...)`
- separater Ladezustand `_loadingPriceDecisionHistoryItemId`
- Handler `_loadPriceDecisionHistory(...)`
- Draft `_QuotePriceDecisionHistoryEntryDraft`
- read-only Block `Preisentscheidungen`

Der Block zeigt:

- Entscheidungsart
- uebernommenen Preis
- Quelle
- Quellpreis
- Referenz und Quelldatum
- Entscheidungszeit

Damit ist die gespeicherte Entscheidung im Editor sichtbar, ohne eine neue
Arbeitsflaeche zu schaffen.

## 5. Bewusst nicht umgesetzt

Weiterhin ausserhalb dieses Blocks bleiben:

- Historienbearbeitung
- Kommentare
- Storno oder Korrektur von Preisentscheidungen
- Margen- oder Zuschlagsberechnung
- Rabattlogik
- Freigabe- oder Eskalationsworkflow
- Bulk-Historie ueber mehrere Positionen
- automatische Preisentscheidung

Diese Themen waeren jeweils eigenstaendige Folgeausbauten.

## 6. Audit-Entscheidung

Der Block ist abgeschlossen.

Innerhalb dieses engen Historienblocks gibt es keinen weiteren kleinen
Haertungsschritt mit gutem Signal, der vor Historienbearbeitung, Marge,
Zuschlag, Rabatt, Freigabe, Bulk oder Automatik noch sinnvoll waere.

Eine zusaetzliche Detailansicht, Filterung oder Kommentarlogik wuerde bereits
eine groessere Historien- oder Auditfunktion starten. Die aktuelle Stufe
erfuellt das Minimalziel: gespeicherte Entscheidungen sind sichtbar.

## 7. Naechster sinnvoller Schritt

Nach sichtbarer Preisentscheidungs-Historie ist der naechste Schritt wieder
eine fachliche Inventur:

- ist jetzt erstmals ein kalkulationsnaher Margen-/Zuschlagsanker sinnvoll?
- oder braucht es zuerst einen kleinen Abweichungs-/Freigabeanker?
- oder ist eine engere Historien-/Auditfunktion als naechstes wertvoller?

Die Inventur muss entscheiden, welcher Folgeausbau nach sichtbarer
Entscheidungshistorie den hoechsten Signalwert hat.

