# GAEB-Preisvorschlag nach gesetztem Material: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten technischen Folgeausbau nach der
abgeschlossenen kleinen Suchtreffer-Uebernahme zu.

Der Scope bleibt bewusst eng:

- erster kleiner Preisvorschlag direkt an einer Quote-Position mit bereits
  gesetztem `material_id`
- nur als Preis- und Uebernahmehilfe fuer genau eine Position
- ohne Ranking-, Bulk-, Historien- oder Automatiklogik

## 1. Ausgangslage

Der aktuelle Stand deckt bereits ab:

- persistenter Herkunftsanker `accepted import item -> created quote item`
- optionales `material_id` an Quote-Positionen
- `price_mapping_status = open/manual`
- read-only `material_candidate_status = none/available`
- explizite Uebernahme sichtbarer Kandidaten
- enge read-only Materialsuche an offenen Draft-Quote-Positionen
- explizite Uebernahme sichtbarer Suchtreffer

Damit ist der kleine manuelle Materialmapping-Pfad geloest. Noch nicht
moeglich ist ein kleiner kontrollierter Preis-Anschluss, wenn:

- ein Material bereits gesetzt wurde
- `unit_price` aber weiter roh leer oder manuell blind gepflegt werden muss

## 2. Kernentscheidung

Der naechste Block soll **nicht** automatisch kalkulieren und **nicht**
historische oder gewichtete Preislogik mitziehen.

Der kleinste sinnvolle technische Schnitt ist:

- ein kleiner read-only Preisanker oder einzelner Preisvorschlag direkt an
  genau einer Quote-Position mit gesetztem `material_id`
- mit weiterhin separater expliziter manueller Uebernahme des vorgeschlagenen
  Preises

## 3. Empfohlenes Minimalzielbild

Die erste Preisvorschlagsstufe soll nur diese Fragen beantwortbar machen:

- kann fuer eine Quote-Position mit gesetztem `material_id` ein kleiner
  Preisanker sichtbar gemacht werden?
- bleibt der Preisvorschlag auf genau diese eine Quote-Position begrenzt?
- bleibt das Ergebnis weiter ein explizit manuell gesetzter `unit_price`?

Bewusst noch nicht Teil dieses Schritts:

- Ranking oder Priorisierung mehrerer Preisquellen
- Historien- oder Durchschnittslogik
- automatische Preisuebernahme
- Preisvorschlag ueber mehrere Positionen gleichzeitig
- globale Preis- oder Kalkulationsansicht

## 4. Kleinster technischer Zuschnitt

Der kleinste saubere Zuschnitt ist:

- kleine Preisanfrage an der bestehenden Quote-Position
- kleiner read-only Preisanker oder einzelner Vorschlag
- separate explizite Uebernahmeaktion fuer genau diesen Vorschlag

Sinnvolle Minimalfelder fuer den Preisanker:

- `material_id`
- `suggested_unit_price`
- optional `currency`
- optional knapper `source_label`

Sinnvolle Minimalreaktion auf Uebernahme:

- aktualisierte Quote-Detailantwort oder aktualisiertes Quote-Item aus der
  Serverantwort

Damit bleibt der Preisvorschlag eine kleine Hilfe und noch keine neue
Kalkulationsarbeitsflaeche.

## 5. Warum Preisanzeige und Preisuebernahme getrennt bleiben sollten

Der erste Preispfad sollte bewusst zwei kleine Schritte behalten:

- erst Preisanker sichtbar machen
- dann den Preis explizit uebernehmen

Das ist wichtig, weil:

- sonst aus Preisvorschlag sofort Halbautomatik wird
- der Nutzer den Preisanker erst sehen und bewerten soll
- die bestehende manuelle Verantwortung klar erhalten bleibt

## 6. Warum der Preisvorschlag direkt an der Quote-Position liegen sollte

Die Quote-Position bleibt der richtige Ort, weil dort bereits zusammenlaufen:

- Herkunftsanker
- gesetztes `material_id`
- Mapping-Status
- bestehender `unit_price`
- manuelle Verantwortung des Nutzers

Eine separate Preisoberflaeche waere fuer diesen ersten Schritt unnoetig
gross und wuerde einen parallelen Workflow erzeugen.

## 7. Was der Preisvorschlag in diesem Schritt bewusst nicht bedeutet

Der Preisvorschlag ist in diesem Schritt nur:

- eine kleine manuelle Preisfindungshilfe nach gesetztem Material
- ein enger Folgeschritt des bestehenden Materialmappings

Der Preisvorschlag ist noch nicht:

- vollstaendige Kalkulation
- beste Preisempfehlung aus mehreren Quellen
- automatische Preiszuordnung
- Ausloeser fuer automatische Steuer-, Zuschlags- oder Margenlogik

## 8. Guard Rails

Die erste Preisvorschlagsstufe sollte mindestens diese Regeln einhalten:

- nur fuer genau eine Quote-Position pro Anfrage
- nur fuer bearbeitbare Draft-Quote-Positionen
- nur wenn bereits ein `material_id` gesetzt ist
- vorhandenen `unit_price` nicht stillschweigend ueberschreiben
- keine Bulk-Preisvorschlaege ueber ganze Quotes
- keine Ranking-, Historien- oder Scorelogik im selben Schritt
- keine automatische Aenderung von `tax_code`, `qty` oder anderen
  Kalkulationsfeldern

## 9. Einordnung in den bestehenden Mapping-Pfad

Der Preisvorschlag ist kein neuer paralleler Workflow, sondern die kleinste
Erweiterung des bestehenden manuellen Materialmappings nach erfolgreicher
Materialwahl.

Er baut sauber auf dem vorhandenen Pfad auf:

- Herkunftsanker
- offenes manuelles Mapping
- sichtbare Kandidaten oder Suchtreffer
- explizite manuelle Materialuebernahme
- anschliessend kleiner Preisvorschlag

Damit bleibt der Weg fachlich konsistent:

- erst Material finden und setzen
- dann Preisanker sichtbar machen
- dann Preis explizit manuell uebernehmen

## 10. Anschlussnutzen

Schon ohne Historie oder Automatik schafft dieser kleine Preisvorschlag:

- operativen Nutzen nach erfolgreichem Materialmapping
- weniger Abhaengigkeit von blindem manuellen Preiseintrag
- saubere Grundlage fuer spaetere Preis-, Historien- oder
  Kalkulationsstufen

## 11. Was bewusst ausserhalb dieses Blocks bleibt

Nicht in denselben Block ziehen:

- Ranking oder Match-Scores fuer Preise
- Preis-Historien oder Durchschnittslogik
- Bulk-Preisvorschlaege oder Bulk-Preisuebernahmen
- automatische Kalkulation aus Material, Zuschlaegen und Margen
- globale Preis- oder Kalkulationskonsole
- KI-gestuetzte Preis- oder Kalkulationslogik

## 12. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Umsetzungsschritt:

- erste kleine Backend-Stufe fuer den Preisvorschlag vorbereiten, bewusst
  noch ohne Ranking-, Bulk-, Historien- oder Automatiklogik
