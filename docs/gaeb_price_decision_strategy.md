# GAEB-Preisentscheidung nach Preisbewertung:
# Minimalzielbild und technischer Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet das Minimalzielbild fuer die naechste kleine
Ausbaustufe nach der abgeschlossenen read-only Preisbewertung zu.

Der Fokus bleibt bewusst eng:

- eine explizite Nutzerentscheidung ermoeglichen, die primaere priorisierte
  Preisquelle als Positionspreis zu uebernehmen
- den Write-Pfad auf genau eine bereits gemappte Draft-Quote-Position
  begrenzen
- Bulk, Margen-, Zuschlags-, Rabatt-, Freigabe- und Automatiklogik weiterhin
  strikt ausklammern

## 1. Ausgangspunkt

Der aktuelle Stand bietet bereits:

- gesetztes `material_id` an der Quote-Position
- aktuellen `unit_price` an der Quote-Position
- read-only Preisquellenanzeige
- read-only Quellen-Priorisierung
- read-only Preisbewertung gegen die primaere Quelle

Die naechste kleine Luecke ist damit nicht mehr Sichtbarkeit oder Bewertung,
sondern eine kontrollierte Entscheidung:

- soll die primaere priorisierte Quelle jetzt explizit als Positionspreis
  uebernommen werden?

## 2. Fachliches Minimalziel

Das Minimalziel ist:

- eine enge explizite Preisuebernahme aus der primaeren priorisierten Quelle
  fuer genau eine bereits gemappte Draft-Quote-Position

Diese Aktion soll nur beantworten:

- kann der Nutzer den primaeren Preisanker bewusst auf die Position setzen?
- wird die Quote danach mit aktualisiertem Positionspreis und aktualisierten
  Summen zurueckgegeben?

Sie soll bewusst noch nicht beantworten:

- welche Marge oder welcher Zuschlag anzuwenden ist
- ob eine Freigabe erforderlich ist
- ob mehrere Positionen gemeinsam aktualisiert werden sollen
- ob die Preisentscheidung automatisch ausgeloest wird

## 3. Bewusst enger Scope

Innerhalb dieses Blocks soll es nur geben:

- einen expliziten Button bzw. eine explizite Aktion pro Position
- genau eine Position pro Anfrage
- serverseitige Neubestimmung der primaeren Quelle
- Update von `quote_items.unit_price`
- Setzen bzw. Halten von `price_mapping_status = manual`
- Rueckgabe der aktualisierten Quote ueber den bestehenden Editorfluss

Bewusst ausserhalb des Scopes bleiben:

- Bulk-Uebernahme ueber mehrere Positionen
- Margen-, Zuschlags-, Rabatt- oder Gemeinkostenlogik
- Preisberechnung aus mehreren Quellen
- Freigabe- oder Eskalationsworkflow
- Persistierung einer separaten Entscheidungsakte
- globale Preisarbeitsflaeche

## 4. Zielbild im Backend

Das Backend soll einen engen Write-Pfad bereitstellen, der:

- genau eine bereits gemappte Draft-Quote-Position adressiert
- dieselben Guard Rails wie die bestehenden Preis-Write-Pfade verwendet
- die Quote fuer das Update sperrt
- die Position fuer das Update sperrt
- die primaere Quelle aus der bestehenden Quellen-Priorisierung bestimmt
- `unit_price` auf den `unit_price` dieser primaeren Quelle setzt
- `price_mapping_status` auf `manual` setzt
- die aktualisierte Quote ueber `Get(...)` zurueckgibt

Der Pfad soll bewusst keine neue Tabelle, keinen neuen Status und keine
separate Entscheidungshistorie einfuehren.

## 5. Vorgeschlagene Service-Methode

Der enge Service-Einstieg kann lauten:

- `ApplyPrimaryPriceSourceForQuoteItem(ctx, quoteID, itemID) (*Quote, error)`

Die Methode soll sich am bestehenden Muster von
`ApplyPriceSuggestionForQuoteItem(...)` orientieren, aber nicht
`materials.avg_purchase_price` direkt verwenden.

Stattdessen soll sie:

- Quote und Position mit den bestehenden Guard Rails pruefen
- die primaere Preisquelle serverseitig erneut aus der bestehenden
  Quellen-Priorisierung bestimmen
- nur den Preis dieser primaeren Quelle uebernehmen

## 6. Vorgeschlagener API-Endpunkt

Der minimale API-Endpunkt kann lauten:

- `POST /api/v1/quotes/{id}/items/{itemID}/apply-primary-price-source`

Eigenschaften:

- kein Request-Body erforderlich
- Permission analog zu bestehenden Preisaktionen: `quotes.write`
- Response: aktualisierte Quote
- Fehler werden ueber bestehende Domain-Error-Behandlung ausgegeben

## 7. Guard Rails

Die Guard Rails sollen bewusst eng zu den bestehenden Preisuebernahme- und
Preisbewertungspfaden passen:

- Quote muss existieren
- keine historischen Quote-Versionen
- nur Draft-Quotes
- Position muss existieren und zur Quote gehoeren
- Position muss ein gesetztes `material_id` besitzen
- mindestens eine primaere priorisierte Quelle muss sichtbar sein
- es wird nur genau diese eine Position geaendert

Wenn keine primaere Quelle bestimmbar ist, soll der Pfad mit einem fachlichen
Fehler abbrechen.

## 8. Zielbild fuer die Aktualisierung

Die Aktualisierung bleibt minimal:

- `quote_items.unit_price = primary_source.unit_price`
- `quote_items.price_mapping_status = 'manual'`

Wichtig:

- Quote-Summen muessen konsistent mit dem aktualisierten Positionspreis sein.
- Falls der bestehende Update-Pfad Positionen neu schreibt und Summen neu
  berechnet, soll die Implementierung das lokale Muster nutzen.
- Falls direkt auf `quote_items` aktualisiert wird, muessen `net_amount`,
  `tax_amount` und Quote-Kopf-Summen im gleichen transaktionalen Schritt
  konsistent gehalten werden.

## 9. Zielbild im Client

Der Client soll die Aktion klein und positionsnah spiegeln:

- weiterhin direkt an genau einer Quote-Position
- weiterhin im bestehenden Quote-Editor
- sichtbar nur, wenn eine Preisbewertung sichtbar ist oder geladen werden kann
- explizite Aktion wie `Primaerpreis uebernehmen`
- nach Erfolg Aktualisierung des Dialogs aus der Serverantwort

Es soll bewusst noch nicht geben:

- Bulk-Button
- automatische Ausfuehrung nach Laden der Bewertung
- Margen- oder Zuschlagseingaben
- Freigabehinweise oder Eskalationsdialoge

## 10. Warum diese Stufe noch keine Automatik ist

Diese Stufe bleibt bewusst kleiner als Automatik, weil:

- der Nutzer explizit klickt
- die primaere Quelle nur als Quelle fuer genau diese Entscheidung dient
- kein Preis ohne Aktion veraendert wird
- keine Regel mehrere Positionen oder ganze Angebote veraendert

## 11. Warum diese Stufe noch keine Kalkulation ist

Diese Stufe bleibt bewusst vor Kalkulationslogik, weil:

- kein neuer Verkaufspreis berechnet wird
- keine Marge oder kein Zuschlag angewendet wird
- nur ein vorhandener primaerer Kostenanker uebernommen wird
- keine Angebotsgesamtbewertung entsteht

## 12. Minimaler Nutzen

Schon in diesem engen Zuschnitt schafft die Preisentscheidung:

- Abschluss des kleinen manuellen Bewertungs- und Entscheidungsflusses
- weniger manuelle Eingabefehler beim Positionspreis
- konsistentere Nutzung der primaeren Preisquelle
- klaren Anschluss fuer spaetere Margen-, Zuschlags- und Freigabelogik

## 13. Entscheidung

Das minimale technische Zielmodell fuer die naechste Stufe ist:

- ein enger expliziter Write-Pfad fuer genau eine bereits gemappte
  Draft-Quote-Position, der den Preis der primaeren Quelle aus der bestehenden
  Quellen-Priorisierung uebernimmt

## 14. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Schritt:

- die erste Backend-Stufe fuer diese explizite Preisuebernahme vorbereiten,
  bewusst noch ohne Bulk, Margen-, Zuschlags-, Rabatt- oder Freigabelogik
