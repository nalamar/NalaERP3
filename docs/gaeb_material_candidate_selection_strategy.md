# GAEB-Kleine Kandidatenauswahl: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten technischen Folgeausbau nach der
abgeschlossenen read-only Materialkandidatenliste zu.

Der Scope bleibt bewusst eng:

- erste kleine Kandidatenauswahl-Aktion
- direkt aus bereits sichtbaren Materialkandidaten
- ohne Materialsuche, Ranking oder automatische Uebernahme

## 1. Ausgangslage

Der aktuelle Stand deckt bereits ab:

- persistenter Herkunftsanker `accepted import item -> created quote item`
- optionales `material_id` an Quote-Positionen
- `price_mapping_status = open/manual`
- read-only `material_candidate_status = none/available`
- kleine read-only Materialkandidatenliste direkt an der Quote-Position

Damit ist bereits sichtbar, welche wenigen Kandidaten konkret in Frage
kommen. Noch nicht moeglich ist die direkte Uebernahme eines bereits
sichtbaren Kandidaten.

## 2. Kernentscheidung

Der naechste Block soll **nicht** nach Materialien suchen und **nicht**
eigenstaendig den besten Kandidaten bestimmen.

Der kleinste sinnvolle technische Schnitt ist:

- eine kleine explizite Auswahlaktion direkt an einem bereits sichtbaren
  Kandidaten
- mit Uebernahme von `material_id` an genau einer Quote-Position
- nur als manueller Mapping-Schritt, nicht als Matching- oder
  Automatiksystem

## 3. Empfohlenes Minimalzielbild

Die erste Kandidatenauswahl soll nur diese Fragen beantwortbar machen:

- kann ein bereits sichtbarer Kandidat direkt uebernommen werden?
- bleibt die Aktion auf genau eine Quote-Position begrenzt?
- bleibt das Ergebnis ein klar manuell gesetztes Materialmapping?

Bewusst noch nicht Teil dieses Schritts:

- Materialsuche ausserhalb sichtbarer Kandidaten
- Ranking oder Sortierlogik unter Kandidaten
- automatische Vorbelegung eines Kandidaten
- Bulk-Uebernahme ueber mehrere Positionen
- Preisvorschlag oder Preislogik aus Kandidaten

## 4. Kleinster technischer Zuschnitt

Der kleinste saubere Zuschnitt ist:

- Auswahlaktion direkt an einem read-only Kandidateneintrag
- Uebernahme von `candidate.material_id` nach `quote_items.material_id`
- Statuswechsel von offenem Kandidatenfall zu manuell gesetztem Mapping

Sinnvolle Minimalwirkung:

- `material_id` wird gesetzt
- `price_mapping_status` bleibt im engen manuellen Statusraum konsistent
- read-only Kandidatenanzeige kann danach entfallen oder als erledigt gelten

## 5. Warum die Aktion direkt am sichtbaren Kandidaten liegen sollte

Die erste Interaktion sollte bewusst klein bleiben, weil:

- sonst sofort ein neuer Such- oder Auswahldialog mitgezogen wird
- die vorhandene Kandidatenliste bereits genug Kontext fuer den ersten
  Uebernahmeschritt liefert
- der Nutzer ohne Medienbruch von Transparenz zu Aktion wechseln kann

Die vorhandene Quote-Position bleibt damit die kleinste sinnvolle
Arbeitsflaeche.

## 6. Was die Aktion in diesem Schritt bewusst nicht bedeutet

Die Kandidatenauswahl ist in diesem Schritt nur:

- eine explizite manuelle Uebernahme eines sichtbaren Kandidaten
- ein kleiner Folgeschritt des bestehenden manuellen Mappings

Die Aktion ist noch nicht:

- Bestaetigung eines fachlich besten Treffers
- Suchergebnis mit Rangfolge
- automatische Materialzuordnung
- Ausloeser fuer automatische Preislogik

## 7. Guard Rails

Die erste Kandidatenauswahl sollte mindestens diese Regeln einhalten:

- nur sichtbare Kandidaten duerfen uebernommen werden
- nur eine Quote-Position pro Aktion
- vorhandenes `material_id` darf nicht stillschweigend ueberschrieben werden
- kein neuer Such- oder Rankingpfad im selben Schritt
- keine Bulk-Aktion ueber mehrere Positionen
- keine automatische Ableitung von Preis oder Steuer

## 8. Einordnung in den bestehenden Mapping-Pfad

Die kleine Kandidatenauswahl ist kein neuer paralleler Workflow, sondern eine
engere Bedienform des bestehenden manuellen Materialmappings.

Sie setzt auf denselben Positionskontext auf:

- Herkunftsanker
- offenes manuelles Mapping
- sichtbare Kandidaten

Damit bleibt der Pfad fachlich konsistent und leicht verstaendlich.

## 9. Anschlussnutzen

Schon ohne Suche oder Ranking schafft diese kleine Aktion:

- sofort nutzbaren operativen Wert aus der Kandidatenliste
- schnelleren Abschluss einfacher Mapping-Faelle
- saubere Trennung zwischen manueller Auswahl und spaeterer Suchlogik
- bessere Grundlage fuer spaetere Ranking- oder Automatikstufen

## 10. Was bewusst ausserhalb dieses Blocks bleibt

Nicht in denselben Block ziehen:

- Materialsuche
- Kandidatenranking
- automatische Vorbelegung
- Bulk-Uebernahme
- Preislogik aus Kandidaten
- globale Mapping- oder Kandidatenansicht

## 11. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Umsetzungsschritt:

- erste kleine Backend-Stufe fuer die Kandidatenauswahl vorbereiten,
  bewusst noch ohne Such-, Ranking- oder Automatiklogik
