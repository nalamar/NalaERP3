# GAEB-Zielmargen-/Zuschlagsanker: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit prueft den engen Block des read-only
Zielmargen-/Zuschlagsankers nach Backend- und Client-Umsetzung.

Geprueft wird bewusst nur:

- ob der Zielmargenanker technisch Ende-zu-Ende vorhanden ist
- ob Backend, API und Client dieselbe read-only Semantik abbilden
- ob vor Zielwert-Persistenz, Preisautomatik oder Freigabe-Workflow noch ein
  kleiner Haertungsschritt mit gutem Signal uebrig ist

## 1. Ausgangspunkt

Vor diesem Block waren bereits vorhanden:

- persistierte Preisentscheidung als Kostenbasis
- read-only Margenanker
- read-only Approval-Hint
- Entscheidung, vor echtem Freigabe-Workflow zuerst eine Zielwertbewertung
  einzufuehren

Die fachliche Luecke war:

- negative Marge war bereits erkennbar
- positive Marge war aber noch nicht gegen einen Zielwert bewertet
- ein echter Freigabe-Workflow haette ohne Ziel-/Schwellwertlogik ein zu
  duennes Signal

## 2. Umgesetzter enger Scope

Der umgesetzte Scope bleibt bewusst klein:

- Backend-DTO `QuoteItemTargetMarginAnchor`
- Service `TargetMarginAnchorForQuoteItem(...)`
- Route `GET /api/v1/quotes/{id}/items/{itemID}/target-margin-anchor`
- Client-API `getQuoteItemTargetMarginAnchor(...)`
- Draft `_QuoteItemTargetMarginAnchorDraft`
- positionsnaher Ladehandler `_loadTargetMarginAnchor(...)`
- read-only UI-Block `Zielmarge`

Der Block verwendet einen technischen Default:

- `target_margin_percent = 20.00`

Weiterhin nicht enthalten:

- Zielwert speichern
- Zielwert je Kunde, Projekt, Materialgruppe oder Mandant
- Preis automatisch setzen
- Zielpreis uebernehmen
- Freigabe anfordern
- Freigabe erteilen oder ablehnen
- Rollenmodell
- Angebotsweite Aggregation
- Bulk-Aktion
- Automatik

## 3. Ergebnis im Backend

Das Backend bildet den Anker eng ab:

- `TargetMarginAnchorForQuoteItem(...)` baut auf
  `MarginAnchorForQuoteItem(...)` auf
- vorhandene Kostenbasis und aktuelle Position werden nicht doppelt berechnet
- fehlende Preisentscheidung wird als
  `target_blocked_until_margin_available` mit HTTP 200 dargestellt
- echte Positions- oder Statusfehler bleiben Domain-Fehler
- Zielpreis wird aus Kostenbasis und Default-Zielwert berechnet
- Zielabweichung wird absolut und prozentual ausgegeben, wenn moeglich

Aktuell getestete Backend-Pfade:

- `below_target` nach gespeicherter Preisentscheidung und aktuellem Preis
  unter Zielpreis
- `on_target` nach bewusst gesetztem Positionspreis auf den Zielpreis
- `target_blocked_until_margin_available` ohne gespeicherte Preisentscheidung

Nicht explizit getestet, aber aus derselben Statusableitung ableitbar:

- `below_cost` bei negativer Marge
- `above_target` bei Preis oberhalb des Zielpreises

Diese beiden Pfade waeren fachlich sinnvoll, aber fuer den Abschluss dieses
ersten read-only Zielwertankers nicht zwingend, weil die zentralen neuen
Semantiken bereits abgedeckt sind:

- Zielwert wird berechnet
- unter Ziel wird erkannt
- Ziel erreicht wird erkannt
- fehlende Kostenbasis blockiert read-only

## 4. Ergebnis im Client

Der Client spiegelt den Anker positionsnah:

- `getQuoteItemTargetMarginAnchor(...)`
- `_QuoteItemTargetMarginAnchorDraft`
- Felder `targetMarginAnchor` und `targetMarginAnchorPerformed`
- Ladezustand `_loadingTargetMarginAnchorItemId`
- Handler `_loadTargetMarginAnchor(...)`
- read-only Block `Zielmarge`

Der Block zeigt:

- Zielstatus
- Grundtext
- Zielwert
- aktuellen Positionspreis
- Zielpreis
- Kostenbasis
- Zielabweichung
- aktuelle Marge
- Entscheidungstyp
- Quelle
- Entscheidungszeit

Der Client fuehrt keine Preisuebernahme aus und veraendert keine
Angebotsdaten.

## 5. Bewusst nicht umgesetzt

Weiterhin ausserhalb dieses Blocks bleiben:

- Zielwert-Konfiguration
- Zielwert-Persistenz
- Zielwert-Historie
- Zielwert je Kunde, Projekt, Materialgruppe oder Angebot
- Uebernahme des Zielpreises
- Rabatt- oder Nachlassregel
- echter Freigabe-Workflow
- Rollen und Berechtigungen
- Kommentierung
- Angebotsweite Zielmargenpruefung
- Bulk-Bewertung
- KI-gestuetzte Preis- oder Zielwertlogik

Diese Themen waeren eigene Folgeausbauten.

## 6. Audit-Entscheidung

Der read-only Zielmargen-/Zuschlagsanker ist abgeschlossen.

Begruendung:

- Backend, API und Client bilden dieselbe read-only Semantik ab
- der Anker nutzt den bestehenden Margenanker als gemeinsame Kostenbasis
- der technische Default-Zielwert ist sichtbar und nicht versteckt
- die wichtigsten Statuspfade sind per Integrationstest abgesichert
- der Block veraendert keine Angebotsdaten
- weitere sinnvolle Schritte sind neue Feature-Bloecke, keine Haertung dieses
  Ankers

## 7. Warum kein weiterer kleiner Haertungsschritt folgt

Ein zusaetzlicher Test fuer `below_cost` oder `above_target` waere moeglich,
liefert aber weniger Signal als der jetzt erreichte Ende-zu-Ende-Abschluss:

- `below_cost` ist fachlich bereits ueber Margenanker und Approval-Hint
  abgesichert
- `above_target` ist die symmetrische Gegenrichtung zu `below_target` und
  nutzt dieselbe Zielpreisberechnung
- die naechsten relevanten Entscheidungen liegen nicht in weiteren read-only
  Statusvarianten, sondern in Zielwertquelle, Preisuebernahme oder Freigabe

Damit bleibt der Block bewusst geschlossen.

## 8. Verifikation

Fuer den Abschluss waren gruen:

- `go test ./internal/quotes ./internal/http`
- `flutter analyze lib/api.dart lib/pages/quotes_page.dart`

Zusaetzlich wurden die betroffenen Go- und Dart-Dateien formatiert.

## 9. Naechster sinnvoller Schritt

Nach dem abgeschlossenen Zielmargenanker sollte die naechste Inventur
entscheiden, welcher Folgeblock den hoechsten Signalwert hat:

- Zielwert-Konfiguration und Persistenz
- kontrollierte Zielpreis-Uebernahme
- echter Freigabe-Workflow auf Basis von Zielabweichung

Der naechste Schritt soll wieder nur eine dieser Richtungen waehlen und
technisch eng zuschneiden.
