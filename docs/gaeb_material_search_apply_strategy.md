# GAEB-Suchtreffer-Uebernahme: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten technischen Folgeausbau nach der
abgeschlossenen kleinen Materialsuche zu.

Der Scope bleibt bewusst eng:

- erste kleine explizite Uebernahme aus bereits sichtbaren Suchtreffern
- nur als manueller Write-Pfad fuer genau eine Quote-Position
- ohne Ranking-, Bulk-, Preis- oder Automatiklogik

## 1. Ausgangslage

Der aktuelle Stand deckt bereits ab:

- persistenter Herkunftsanker `accepted import item -> created quote item`
- optionales `material_id` an Quote-Positionen
- `price_mapping_status = open/manual`
- read-only `material_candidate_status = none/available`
- explizite Uebernahme bereits sichtbarer Kandidaten
- kleine read-only Materialsuche an offenen Draft-Quote-Positionen

Damit ist die Auffindbarkeit fuer Nicht-Trefferfaelle geloest. Noch nicht
moeglich ist ein kleiner kontrollierter Write-Pfad, wenn:

- ein passendes Material in der Suchtrefferliste sichtbar ist
- dieses Material direkt auf genau eine Quote-Position uebernommen werden soll

## 2. Kernentscheidung

Der naechste Block soll **nicht** den besten Suchtreffer bestimmen und
**nicht** Preis- oder Kalkulationslogik mitziehen.

Der kleinste sinnvolle technische Schnitt ist:

- eine explizite Uebernahmeaktion direkt an einem sichtbaren Suchtreffer
- mit engem Backend-Write-Pfad fuer genau eine Quote-Position
- mit unmittelbarer Rueckgabe des aktualisierten Quote-Zustands

## 3. Empfohlenes Minimalzielbild

Die erste Suchtreffer-Uebernahme soll nur diese Fragen beantwortbar machen:

- kann ein bereits sichtbarer Suchtreffer kontrolliert uebernommen werden?
- bleibt die Uebernahme auf genau diese eine Quote-Position begrenzt?
- bleibt das Ergebnis weiter ein explizit manuell gesetztes Materialmapping?

Bewusst noch nicht Teil dieses Schritts:

- Ranking oder Relevanzbewertung unter Suchtreffern
- automatische Vorbelegung eines Treffers
- Uebernahme ueber mehrere Positionen gleichzeitig
- Preisvorschlag oder Preislogik aus dem uebernommenen Material
- globale Mapping- oder Suchansicht

## 4. Kleinster technischer Zuschnitt

Der kleinste saubere Zuschnitt ist:

- bestehende sichtbare Suchtreffer als read-only Ausgangspunkt behalten
- separate explizite Aktion `Uebernehmen` pro Suchtreffer
- enger Backend-Write-Pfad fuer genau einen sichtbaren Suchtreffer auf genau
  einer offenen Draft-Quote-Position

Sinnvolle Minimalangaben fuer die Uebernahmeanfrage:

- `quote_id`
- `quote_item_id`
- `material_id`

Sinnvolle Minimalreaktion:

- aktualisierte Quote-Detailantwort oder aktualisiertes Quote-Item aus der
  Serverantwort

Damit bleibt die Uebernahme ein kleiner Abschluss des bestehenden Suchpfads
und noch keine neue Arbeitsflaeche.

## 5. Warum Suche und Uebernahme getrennt bleiben sollten

Der Suchpfad sollte bewusst zwei kleine Schritte behalten:

- erst Treffer sichtbar machen
- dann einen Treffer explizit uebernehmen

Das ist wichtig, weil:

- sonst aus Suche sofort Halbautomatik wird
- der Nutzer den Suchkontext erst sehen soll
- die bestehende manuelle Verantwortung klar erhalten bleibt

## 6. Warum die Uebernahme direkt an der Quote-Position liegen sollte

Die Quote-Position bleibt der richtige Ort, weil dort bereits zusammenlaufen:

- Herkunftsanker
- Mapping-Status
- sichtbare Kandidaten
- sichtbare Suchtreffer
- bestehende manuelle Materialzuordnung

Eine separate Uebernahmeoberflaeche waere fuer diesen ersten Schritt unnoetig
gross und wuerde einen parallelen Workflow erzeugen.

## 7. Was die Uebernahme in diesem Schritt bewusst nicht bedeutet

Die Suchtreffer-Uebernahme ist in diesem Schritt nur:

- ein kleiner manueller Abschluss des vorhandenen Suchpfads
- ein enger Folgeschritt des bestehenden Materialmappings

Die Uebernahme ist noch nicht:

- fachliches Matching mit Rangfolge
- Empfehlung des besten Materials
- automatische Materialzuordnung
- Ausloeser fuer automatische Preis-, Steuer- oder Kalkulationslogik

## 8. Guard Rails

Die erste Suchtreffer-Uebernahme sollte mindestens diese Regeln einhalten:

- nur fuer genau eine Quote-Position pro Anfrage
- nur fuer bearbeitbare Draft-Quote-Positionen
- nur fuer aktuell sichtbare Suchtreffer der Position
- vorhandenes `material_id` nicht stillschweigend ueberschreiben
- kein Bulk-Mapping ueber ganze Quotes
- keine Ranking- oder Scorelogik im selben Schritt
- keine automatische Aenderung von `unit_price`, `tax_code` oder anderen
  Kalkulationsfeldern

## 9. Einordnung in den bestehenden Mapping-Pfad

Die Suchtreffer-Uebernahme ist kein neuer paralleler Workflow, sondern die
kleinste Erweiterung des bestehenden manuellen Materialmappings fuer
Nicht-Treffer-Faelle.

Sie baut sauber auf dem vorhandenen Pfad auf:

- Herkunftsanker
- offenes manuelles Mapping
- sichtbare Kandidaten, falls vorhanden
- sonst enge Suche
- anschliessend explizite manuelle Uebernahme

Damit bleibt der Weg fachlich konsistent:

- erst sichtbaren Kandidaten nutzen, wenn vorhanden
- sonst eng suchen
- dann explizit manuell uebernehmen

## 10. Anschlussnutzen

Schon ohne Ranking oder Automatik schafft diese kleine Uebernahme:

- einen durchgaengigen kleinen Mapping-Pfad fuer Nicht-Trefferfaelle
- weniger Abhaengigkeit von direkter Material-ID-Eingabe
- saubere Grundlage fuer spaetere Matching-, Ranking- oder
  Preisvorschlagsstufen

## 11. Was bewusst ausserhalb dieses Blocks bleibt

Nicht in denselben Block ziehen:

- Ranking oder Match-Scores
- automatische Vorbelegung
- Bulk-Uebernahme oder Bulk-Mapping
- Preislogik aus Suchtreffern
- globale Mapping- oder Suchkonsole
- KI-gestuetzte Such- oder Matchinglogik

## 12. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Umsetzungsschritt:

- erste kleine Backend-Stufe fuer die Suchtreffer-Uebernahme vorbereiten,
  bewusst noch ohne Ranking-, Bulk-, Preis- oder Automatiklogik
