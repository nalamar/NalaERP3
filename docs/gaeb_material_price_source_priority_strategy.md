# GAEB-Quellen-Priorisierung nach Preis-Historien-/Quellenstufe:
# Minimalzielbild und technischer Zuschnitt

## Ziel dieses Dokuments

Dieses Dokument schneidet das Minimalzielbild fuer die naechste kleine
Ausbaustufe nach der abgeschlossenen Preis-Historien-/Quellenstufe zu.

Der Fokus bleibt bewusst eng:

- sichtbare Preisquellen an genau einer bereits gemappten Draft-Quote-Position
  in eine kleine nachvollziehbare Reihenfolge bringen
- die Priorisierung read-only und positionsnah im bestehenden Quote-Editor
  halten
- Ranking-, Bulk-, Kalkulations- und Automatiklogik weiterhin strikt
  ausklammern

## 1. Ausgangspunkt

Der aktuelle Stand bietet bereits:

- gesetztes `material_id` an der Quote-Position
- sichtbaren Preisanker
- explizite Preisuebernahme
- kleine read-only Preisquellenanzeige mit:
  - `Durchschnittlicher Einkaufspreis`
  - optional `Letzter Bestellpreis`

Die naechste kleine Luecke ist damit nicht mehr Sichtbarkeit, sondern
Einordnung.

## 2. Fachliches Minimalziel

Das Minimalziel ist:

- eine enge read-only Quellen-Priorisierung fuer genau eine bereits gemappte
  Draft-Quote-Position

Diese Priorisierung soll nur beantworten:

- welche der bereits sichtbaren Quellen fuer diese Position zuerst relevant
  ist
- welche Quelle danach folgt

Sie soll bewusst noch nicht beantworten:

- welcher Preis automatisch uebernommen werden soll
- ob aus mehreren Quellen ein neuer Preis berechnet wird
- wie mehrere Positionspreise gegeneinander optimiert werden

## 3. Bewusst enger Scope

Innerhalb dieses Blocks soll es nur geben:

- kleine read-only Priorisierungsinformation direkt an bereits sichtbaren
  Quellen
- genau eine Position pro Anfrage
- weiterhin Arbeit im bestehenden Quote-Editor

Bewusst ausserhalb des Scopes bleiben:

- komplexe Scores oder gewichtete Rankingmodelle
- Bulk-Priorisierung ueber ganze Quotes
- automatische Preisneusetzung
- Margen-, Zuschlags-, Rabatt- oder Kalkulationslogik
- globale Preis- oder Quellenoberflaechen

## 4. Zielbild im Backend

Das Backend soll einen engen read-only Pfad bereitstellen, der:

- genau eine bereits gemappte Draft-Quote-Position adressiert
- dieselben Guard Rails wie der bestehende Preisquellenpfad verwendet
- die vorhandenen sichtbaren Quellen in einer kleinen priorisierten Form
  zurueckgibt

Das Ziel ist nicht ein neues allgemeines Ranking-System, sondern nur:

- bereits sichtbare Quellen mit einer kleinen nachvollziehbaren
  Prioritaetsinformation ausliefern

## 5. Zielbild in der Rueckgabe

Die Rueckgabe soll bewusst klein bleiben. Pro sichtbarer Quelle reicht:

- `source_label`
- `unit_price`
- `currency`
- optionale Herkunftsdetails wie `reference` und `date`
- kleine Priorisierungsinformation wie:
  - `priority_rank`
  - optional kurzer `priority_reason`

Wichtig ist:

- die Priorisierung bleibt erklaerbar und knapp
- sie beschreibt nur die Reihenfolge innerhalb der bereits sichtbaren Quellen
- sie ist noch keine automatische Entscheidung

## 6. Erste enge Priorisierungsregel

Fuer diesen Minimalblock reicht eine kleine, feste, gut erklaerbare Regel.

Das enge Zielmodell ist:

- `Letzter Bestellpreis` wird vor `Durchschnittlicher Einkaufspreis`
  priorisiert, wenn er sichtbar ist
- fehlt ein letzter Bestellpreis, bleibt der durchschnittliche Einkaufspreis
  die erste sichtbare Quelle

Begruendung fuer diese enge Regel:

- sie baut direkt auf den schon sichtbaren Quellen auf
- sie ist fachlich leicht erklaerbar
- sie vermeidet komplexe Ranking- oder Gewichtungslogik

## 7. Zielbild im Client

Der Client soll die Priorisierung klein und positionsnah spiegeln:

- weiterhin direkt an genau einer Quote-Position
- weiterhin im bestehenden Quote-Editor
- weiterhin read-only

Pro sichtbarer Quelle reicht eine kleine Prioritaetsanzeige wie:

- `Primaer`
- `Sekundaer`

oder eine kleine numerische Reihenfolge.

Es soll bewusst noch nicht geben:

- neue Uebernahmeaktion aus der Priorisierung
- Filter, Sortierdialoge oder globale Quellenvergleiche
- automatische Hervorhebung mit stiller Preisanpassung

## 8. Warum diese Stufe noch keine echte Rankinglogik ist

Diese Stufe bleibt bewusst kleiner als spaeteres Ranking, weil:

- sie nur mit den bereits sichtbaren zwei kleinen Quellen arbeitet
- die Regel fest und transparent ist
- keine Scores, keine Gewichtungen und keine lernende Logik entstehen
- die Ausgabe nur eine kleine Reihenfolge und keine automatische Entscheidung
  liefert

## 9. Warum diese Stufe noch keine Kalkulationsnaehe ist

Diese Stufe bleibt bewusst vor Kalkulationsnaehe, weil:

- kein neuer Preis berechnet wird
- keine Marge oder kein Zuschlag angewendet wird
- keine Angebotslogik aus der Quellenpriorisierung abgeleitet wird
- nur die Reihenfolge sichtbarer Quellen geklaert wird

## 10. Guard Rails

Die Guard Rails sollen bewusst eng zu den bestehenden Preisquellenpfaden
passen:

- keine historischen Quote-Versionen
- nur Draft-Quotes
- Position muss existieren
- Position muss ein gesetztes `material_id` besitzen
- nur bereits sichtbare Quellen duerfen priorisiert zurueckgegeben werden

## 11. Minimaler Nutzen

Schon in diesem engen Zuschnitt schafft die Quellen-Priorisierung:

- bessere Einordnung sichtbarer Preisquellen
- schnellere manuelle Preisentscheidung
- belastbarere Grundlage fuer spaetere kalkulationsnahe Ausbaustufen

## 12. Entscheidung

Das minimale technische Zielmodell fuer die naechste Stufe ist:

- ein enger read-only Priorisierungspfad fuer bereits sichtbare Preisquellen
  an genau einer bereits gemappten Draft-Quote-Position

## 13. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Schritt:

- die erste Backend-Stufe fuer diese enge Quellen-Priorisierung vorbereiten,
  bewusst noch ohne Ranking-, Bulk-, Kalkulations- oder Automatiklogik
