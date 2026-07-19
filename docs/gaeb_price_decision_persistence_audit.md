# GAEB-Preisentscheidungs-Persistenz: Abschluss-Audit

## Ziel dieses Audits

Dieses Audit schliesst den engen Block der Preisentscheidungs-Persistenz ab.

Geprueft wird bewusst nur:

- ob die explizite Primaerpreis-Uebernahme als kleiner Snapshot persistiert
  wird
- ob die Persistenz innerhalb derselben Transaktion wie die Preisuebernahme
  liegt
- ob vor Historienanzeige, Marge, Zuschlag, Rabatt, Freigabe, Bulk oder
  Automatik noch ein weiterer kleiner Haertungsschritt mit gutem Signal
  uebrig ist

## 1. Ausgangspunkt

Vor diesem Block waren bereits vorhanden:

- read-only Preisquellen
- Quellen-Priorisierung
- read-only Preisbewertung
- explizite Primaerpreis-Uebernahme
- read-only Preisentscheidungs-Transparenz

Die danach identifizierte Luecke war:

- die Uebernahme aenderte den Positionspreis
- die Transparenz konnte Uebereinstimmung oder Abweichung live zeigen
- aber die konkrete Entscheidung wurde nicht als stabiler Snapshot
  festgehalten

## 2. Umgesetzter enger Scope

Der umgesetzte Scope bleibt bewusst klein:

- neue Tabelle `quote_item_price_decisions`
- genau ein Snapshot beim bestehenden
  `ApplyPrimaryPriceSourceForQuoteItem(...)`
- Entscheidungstyp `primary_source_applied`
- Snapshot von Quote, Position, Material, Quelle, Preis, Waehrung, Referenz,
  Quelldatum und Entscheidungszeit
- Insert in derselben Transaktion wie Positionspreis- und Summenupdate

Der Block fuehrt bewusst nicht ein:

- neue API
- Client-Aenderung
- Historienanzeige
- Marge
- Zuschlag
- Rabatt
- Freigabe
- Bulk-Entscheidung
- Automatik

## 3. Ergebnis in der Migration

Die Migration `048_quote_item_price_decisions.sql` erstellt:

- `quote_item_price_decisions`
- Foreign Keys auf `quotes`, `quote_items` und optional `materials`
- Constraint auf `decision_type = primary_source_applied`
- Preis-Constraint fuer nichtnegative Preise
- Waehrungs-Constraint gegen leere Werte
- Index auf `(quote_item_id, created_at desc)`
- Index auf `(quote_id, created_at desc)`

Die Tabelle hat bewusst keine Unique-Constraint auf `quote_item_id`, damit
spaetere erneute explizite Uebernahmen als weitere Entscheidungen erhalten
bleiben koennen.

## 4. Ergebnis im Service

Der bestehende Service-Pfad bleibt der einzige Schreib-Einstieg:

- `ApplyPrimaryPriceSourceForQuoteItem(...)`

Der neue Helper:

- `insertPrimarySourceAppliedDecisionTx(...)`

schreibt den Snapshot nach der Preis- und Summenaktualisierung und vor dem
Commit. Das ist korrekt, weil Position, Quote-Summen und Snapshot gemeinsam
gelingen oder gemeinsam zurueckgerollt werden.

## 5. Ergebnis im Test

Der Integrationstest prueft:

- erfolgreiche Primaerpreis-Uebernahme erzeugt genau einen Snapshot fuer die
  Position
- `decision_type = primary_source_applied`
- Material, Quelle, Referenz, Preis, Waehrung, Quelldatum und
  Entscheidungszeit sind gesetzt
- fehlendes Material erzeugt keinen Snapshot

Damit ist der fachlich wichtigste Persistenzpfad abgedeckt.

## 6. Audit-Entscheidung

Der Block ist abgeschlossen.

Innerhalb dieses engen Persistenzblocks gibt es keinen weiteren kleinen
Haertungsschritt mit gutem Signal, der vor Historienanzeige, Marge, Zuschlag,
Rabatt, Freigabe, Bulk oder Automatik noch sinnvoll waere.

Moegliche Zusatzschritte wie eine sofortige Read-API, eine UI-Historie oder
Kommentare waeren bereits eigene Folgeausbauten. Die erste Persistenzstufe
ist bewusst nur der stabile Snapshot beim vorhandenen Entscheidungsereignis.

## 7. Naechster sinnvoller Schritt

Nach persistierter Preisentscheidung ist der naechste Schritt eine neue
fachliche Inventur:

- soll zuerst eine kleine Historienanzeige fuer Preisentscheidungen entstehen?
- oder ist jetzt ein kalkulationsnaher Margen-/Zuschlagsanker sinnvoll?
- oder braucht es zuerst einen minimalen Abweichungs-/Freigabeanker?

Die Inventur muss entscheiden, welcher Folgeausbau nach dem stabilen Snapshot
den hoechsten Signalwert hat.

