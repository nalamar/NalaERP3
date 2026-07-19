# GAEB-Preisentscheidungs-Transparenz: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit schliesst den engen Block der read-only
Preisentscheidungs-Transparenz ab.

Geprueft wird bewusst nur:

- ob Backend und Client die gleiche kleine Transparenzfrage beantworten
- ob der Pfad read-only geblieben ist
- ob vor neuer Persistenz, Marge, Zuschlag, Rabatt, Freigabe, Bulk oder
  Automatik noch ein weiterer kleiner Haertungsschritt mit gutem Signal
  uebrig ist

## 1. Ausgangspunkt

Vor diesem Block waren bereits vorhanden:

- Materialzuordnung an einer Quote-Position
- sichtbare Preisquellen
- Quellen-Priorisierung
- read-only Preisbewertung gegen die primaere Quelle
- explizite Uebernahme der primaeren Quelle als Positionspreis

Die danach identifizierte Luecke war:

- der aktuelle Positionspreis war sichtbar
- die Bewertung gegen die primaere Quelle war sichtbar
- die explizite Uebernahme war moeglich
- aber der aktuelle Preis wurde noch nicht als kleine
  Preisentscheidungs-Transparenz eingeordnet

## 2. Umgesetzter enger Scope

Der umgesetzte Scope bleibt bewusst klein:

- genau eine Quote-Position
- read-only Anfrage
- Wiederverwendung der bestehenden Preisbewertung
- keine Veraenderung der Quote-Position
- keine neue Tabelle
- keine Entscheidungshistorie
- keine kommerzielle Regel ueber Uebereinstimmung oder Abweichung hinaus

Der Status beantwortet nur:

- entspricht der aktuelle Positionspreis der aktuellen primaeren Quelle?
- oder weicht er davon ab?

## 3. Ergebnis im Backend

Das Backend bildet den Block eng ab:

- `PriceDecisionTransparency` als kleine Response-Struktur
- `PriceDecisionTransparencyForQuoteItem(...)` als Semantikschicht ueber
  `PriceEvaluationForQuoteItem(...)`
- `GET /api/v1/quotes/{id}/items/{itemID}/price-decision-transparency`
- Integrationstest fuer:
  - Abweichung vor der Primaerpreis-Uebernahme
  - Uebereinstimmung nach der Primaerpreis-Uebernahme
  - fachlichen Fehler ohne Material

Die Methode erfindet keine zweite Primarquellenlogik. Das ist korrekt, weil
der Transparenzblock fachlich auf derselben Quelle stehen muss wie Bewertung
und Primaerpreis-Uebernahme.

## 4. Ergebnis im Client

Der Client spiegelt den Block positionsnah:

- `getQuoteItemPriceDecisionTransparency(...)`
- separater Ladezustand `_loadingPriceDecisionTransparencyItemId`
- Handler `_loadPriceDecisionTransparency(...)`
- Draft `_QuotePriceDecisionTransparencyDraft`
- read-only Block `Preisentscheidung` im bestehenden Quote-Editor

Der Block zeigt:

- kleinen Status
- absolute und relative Abweichung
- aktuellen Positionspreis
- primaere Quelle
- Quellenmeta
- Entscheidungsgrund

Damit entsteht keine neue Arbeitsflaeche und kein verdeckter Schreibpfad.

## 5. Bewusst nicht umgesetzt

Weiterhin ausserhalb dieses Blocks bleiben:

- neue Persistenz fuer Preisentscheidungen
- Nutzer-/Zeitpunkt-Historie
- Marge
- Zuschlag
- Rabatt
- Freigabe
- Eskalation
- Bulk-Bewertung
- automatische Preisentscheidung
- KI-gestuetzte Preisoptimierung

Diese Themen waeren groesser als der aktuelle Transparenzanker.

## 6. Audit-Entscheidung

Der Block ist abgeschlossen.

Innerhalb dieses engen Transparenzblocks gibt es keinen weiteren kleinen
Haertungsschritt mit gutem Signal, der vor neuer Persistenz, Marge, Zuschlag,
Rabatt, Freigabe, Bulk oder Automatik noch sinnvoll waere.

Moegliche Zusatzschritte wie andere Label, weitere Hilfstexte oder ein
zusaetzlicher Bestandsblock wuerden die fachliche Aussage nicht wesentlich
haerten. Die naechsten sinnvollen Schritte liegen in einem neuen Block.

## 7. Naechster sinnvoller Schritt

Nach Preisbewertung, expliziter Primaerpreis-Uebernahme und
Preisentscheidungs-Transparenz ist der naechste Schritt eine neue fachliche
Inventur:

- welcher minimale kommerzielle Folgeausbau hat jetzt den hoechsten
  Signalwert?

Die naheliegenden Kandidaten sind:

- ein kalkulationsnaher Margen-/Zuschlagsanker
- eine kleine Preisentscheidungs-Persistenz
- ein minimaler Abweichungs- oder Freigabeanker

Die Inventur muss entscheiden, ob jetzt erstmals Kalkulationslogik sinnvoll
ist oder ob vorher eine persistierte Entscheidungsgrundlage benoetigt wird.

