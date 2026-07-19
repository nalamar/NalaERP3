# GAEB-Materialsuche: Minimales technisches Zielmodell fuer den ersten kleinen Suchpfad

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten technischen Folgeausbau nach der
abgeschlossenen kleinen Kandidatenauswahl zu.

Der Scope bleibt bewusst eng:

- erste kleine manuelle Materialsuche direkt an offenen Quote-Positionen
- nur als Such- und Auswahlhilfe fuer genau eine Position
- ohne Ranking-, Bulk- oder Automatiklogik

## 1. Ausgangslage

Der aktuelle Stand deckt bereits ab:

- persistenter Herkunftsanker `accepted import item -> created quote item`
- optionales `material_id` an Quote-Positionen
- `price_mapping_status = open/manual`
- read-only `material_candidate_status = none/available`
- kleine read-only Materialkandidatenliste
- explizite Uebernahme bereits sichtbarer Kandidaten

Damit ist der einfache Trefferfall geloest. Noch nicht moeglich ist ein
kleiner kontrollierter Suchpfad, wenn:

- keine sichtbaren Kandidaten vorhanden sind
- sichtbare Kandidaten vorhanden sind, aber fachlich nicht passen

## 2. Kernentscheidung

Der naechste Block soll **nicht** den besten Treffer bestimmen und **nicht**
automatisch etwas uebernehmen.

Der kleinste sinnvolle technische Schnitt ist:

- eine kleine manuelle Suche direkt an genau einer Quote-Position
- mit enger Trefferliste aus bestehenden Materialien
- mit weiterhin expliziter manueller Uebernahme des gewaehlten Treffers

## 3. Empfohlenes Minimalzielbild

Die erste Materialsuche soll nur diese Fragen beantwortbar machen:

- kann fuer eine offene Quote-Position ein Material gesucht werden?
- bleibt die Suche auf genau diese eine Quote-Position begrenzt?
- bleibt das Ergebnis weiter ein explizit manuell gesetztes Materialmapping?

Bewusst noch nicht Teil dieses Schritts:

- Ranking oder Relevanzbewertung unter Suchtreffern
- automatische Vorbelegung eines Treffers
- Suche ueber mehrere Positionen gleichzeitig
- Preisvorschlag oder Preislogik aus Suchtreffern
- globale Such- oder Mappingansicht

## 4. Kleinster technischer Zuschnitt

Der kleinste saubere Zuschnitt ist:

- kleine Suchanfrage an der bestehenden Quote-Position
- kleine read-only Trefferliste mit wenigen Materialankern
- separate explizite Uebernahmeaktion pro Suchtreffer

Sinnvolle Minimalfelder pro Suchtreffer:

- `material_id`
- optional `material_no`
- optional `material_label`

Sinnvolle Minimalgrenze:

- nur wenige Treffer, z. B. ein enges Limit

Damit bleibt die Suche eine kleine Hilfe und noch keine neue Arbeitsflaeche.

## 5. Warum Suche und Uebernahme getrennt bleiben sollten

Der erste Suchpfad sollte bewusst zwei kleine Schritte behalten:

- erst Treffer sichtbar machen
- dann einen Treffer explizit uebernehmen

Das ist wichtig, weil:

- sonst aus Suche sofort Halbautomatik wird
- der Nutzer den Suchkontext erst sehen soll
- die bestehende manuelle Verantwortung klar erhalten bleibt

## 6. Warum die Suche direkt an der Quote-Position liegen sollte

Die Quote-Position bleibt der richtige Ort, weil dort bereits zusammenlaufen:

- Herkunftsanker
- Mapping-Status
- sichtbare Kandidaten
- bestehende manuelle Materialzuordnung

Eine separate Suchoberflaeche waere fuer diesen ersten Schritt unnoetig
gross und wuerde einen parallelen Workflow erzeugen.

## 7. Was die Suche in diesem Schritt bewusst nicht bedeutet

Die Materialsuche ist in diesem Schritt nur:

- eine kleine manuelle Auffindungshilfe
- ein enger Folgeschritt des bestehenden Materialmappings

Die Suche ist noch nicht:

- fachliches Matching mit Rangfolge
- Empfehlung des besten Materials
- automatische Materialzuordnung
- Ausloeser fuer automatische Preis- oder Steuerlogik

## 8. Guard Rails

Die erste Materialsuche sollte mindestens diese Regeln einhalten:

- nur fuer genau eine Quote-Position pro Anfrage
- nur fuer bearbeitbare Draft-Quote-Positionen
- vorhandenes `material_id` nicht stillschweigend ueberschreiben
- keine Bulk-Suche ueber ganze Quotes
- keine Ranking- oder Scorelogik im selben Schritt
- keine automatische Aenderung von `unit_price`, `tax_code` oder anderen
  Kalkulationsfeldern

## 9. Einordnung in den bestehenden Mapping-Pfad

Die Materialsuche ist kein neuer paralleler Workflow, sondern die kleinste
Erweiterung des bestehenden manuellen Materialmappings fuer Nicht-Treffer-
Faelle.

Sie baut sauber auf dem vorhandenen Pfad auf:

- Herkunftsanker
- offenes manuelles Mapping
- sichtbare Kandidaten, falls vorhanden
- explizite manuelle Uebernahme

Damit bleibt der Weg fachlich konsistent:

- erst sichtbaren Kandidaten nutzen, wenn vorhanden
- sonst eng suchen
- dann explizit manuell uebernehmen

## 10. Anschlussnutzen

Schon ohne Ranking oder Automatik schafft diese kleine Suche:

- operativen Nutzen fuer offene Mapping-Faelle ohne Kandidatentreffer
- weniger Abhaengigkeit von direkter Material-ID-Eingabe
- saubere Grundlage fuer spaetere Matching-, Ranking- oder
  Preisvorschlagsstufen

## 11. Was bewusst ausserhalb dieses Blocks bleibt

Nicht in denselben Block ziehen:

- Ranking oder Match-Scores
- automatische Vorbelegung
- Bulk-Suche oder Bulk-Mapping
- Preislogik aus Suchtreffern
- globale Mapping- oder Suchkonsole
- KI-gestuetzte Such- oder Matchinglogik

## 12. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Umsetzungsschritt:

- erste kleine Backend-Stufe fuer die Materialsuche vorbereiten, bewusst
  noch ohne Ranking-, Bulk- oder Automatiklogik
