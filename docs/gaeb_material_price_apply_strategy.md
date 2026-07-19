# GAEB-Preisuebernahme aus sichtbarem Preisanker: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten technischen Folgeausbau nach dem
abgeschlossenen read-only Preisvorschlag zu.

Der Scope bleibt bewusst eng:

- explizite manuelle Preisuebernahme aus einem bereits sichtbaren Preisanker
- nur fuer genau eine Quote-Position
- ohne Ranking-, Bulk-, Historien- oder Automatiklogik

## 1. Ausgangslage

Der aktuelle Stand deckt bereits ab:

- gesetztes `material_id` an der Quote-Position
- read-only Preisanker aus dem bestehenden Materialstamm
- positionsnahe Anzeige im bestehenden Quote-Editor
- weiterhin unveraenderten `unit_price`, solange der Nutzer nichts uebernimmt

Damit ist die Sichtbarkeit des Preisankers geloest.

Noch offen ist nur der naechste kleine operative Schritt:

- ein bereits sichtbarer Preisanker soll kontrolliert auf genau diese
  Quote-Position uebernommen werden koennen

## 2. Kernentscheidung

Der naechste Block soll **nicht** in Preisstrategie oder Kalkulation
ausufern.

Der kleinste sinnvolle technische Schnitt ist:

- eine enge explizite Uebernahmeaktion fuer den bereits sichtbaren
  Preisanker
- auf genau einer bereits gemappten Draft-Quote-Position
- mit unmittelbarer Rueckspiegelung des aktualisierten Preiszustands aus der
  Serverantwort

## 3. Empfohlenes Minimalzielbild

Die erste Preisuebernahmestufe soll nur diese Fragen loesen:

- kann ein sichtbarer Preisanker kontrolliert auf genau eine Position
  uebernommen werden?
- bleibt die Entscheidung explizit beim Nutzer?
- bleibt der Rest der Quote unveraendert?

Bewusst noch nicht Teil dieses Schritts:

- Auswahl zwischen mehreren Preisquellen
- Historien- oder Durchschnittsvergleich
- automatische Preisuebernahme
- Preisuebernahme ueber mehrere Positionen gleichzeitig
- Margen-, Zuschlags- oder Steuerlogik

## 4. Kleinster technischer Zuschnitt

Der kleinste saubere Zuschnitt ist:

- read-only Preisanker bleibt wie bisher sichtbar
- pro sichtbarem Preisanker gibt es genau eine explizite Aktion
  `Preis uebernehmen`
- die Uebernahme setzt nur den `unit_price` an genau einer Quote-Position

Sinnvolle Minimalreaktion der Serverantwort:

- aktualisierte Quote-Detailantwort oder aktualisiertes Quote-Item

Damit kann die UI nach erfolgreicher Uebernahme sofort den neuen
Positionszustand anzeigen, ohne einen parallelen Preisworkflow zu oeffnen.

## 5. Warum die Preisuebernahme explizit bleiben muss

Die erste Preisuebernahme sollte bewusst kein stiller Seiteneffekt des
Preisvorschlags sein.

Das ist wichtig, weil:

- der Preisvorschlag nur eine Hilfe und noch keine Entscheidung ist
- `unit_price` ein kommerziell relevantes Feld bleibt
- die manuelle Verantwortung des Nutzers klar erhalten bleiben soll

## 6. Was diese Uebernahme in diesem Schritt genau bedeuten sollte

In diesem Schritt bedeutet Preisuebernahme nur:

- sichtbaren Preisanker auf den `unit_price` dieser einen Position setzen

In diesem Schritt bedeutet sie bewusst noch nicht:

- Preisstatus- oder Kalkulationsautomatik
- Neuberechnung oder Ableitung weiterer Felder ausser der ohnehin
  nachgelagerten Quote-Summenaktualisierung
- Preisentscheidung fuer andere Positionen

## 7. Guard Rails

Die erste Preisuebernahme sollte mindestens diese Regeln einhalten:

- nur fuer genau eine Quote-Position pro Anfrage
- nur fuer bearbeitbare Draft-Quote-Positionen
- nur wenn bereits ein `material_id` gesetzt ist
- nur wenn fuer diese Position ein sichtbarer Preisanker verfuegbar ist
- keine Bulk-Preisuebernahme ueber ganze Quotes
- keine Ranking-, Historien- oder Scorelogik im selben Schritt
- keine automatische Aenderung von `tax_code`, `qty` oder `material_id`

## 8. Warum dieser Schritt der richtige naechste Block ist

Diese Preisuebernahme ist jetzt der richtige naechste Block, weil sie:

- den noch offenen Bruch zwischen Sichtbarkeit und Nutzung des Preisankers
  schliesst
- ohne neue Preisstrategie auskommt
- direkt am bestehenden Quote-Editor anschliesst
- eine saubere Grundlage fuer spaetere Preis- oder Kalkulationsstufen schafft

## 9. Was bewusst ausserhalb dieses Blocks bleibt

Nicht in denselben Block ziehen:

- mehrere konkurrierende Preisquellen
- Preis-Ranking oder Preis-Scores
- Preis-Historien oder Zeitbezug
- automatische Preisuebernahme
- Bulk-Preisuebernahme
- Margen-, Zuschlags- oder KI-Kalkulationslogik

## 10. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Umsetzungsschritt:

- erste kleine Backend-Stufe fuer die enge Preisuebernahme aus dem sichtbaren
  Preisanker vorbereiten, weiterhin ohne Ranking-, Bulk-, Historien- oder
  Automatiklogik
