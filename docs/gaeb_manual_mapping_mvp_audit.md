# GAEB-Manueller Preis-/Material-Mapping-Einstieg: Abschluss-Audit

## Ziel dieses Dokuments

Dieses Dokument bewertet, ob der erste kleine manuelle Preis-/
Material-Mapping-Einstieg nach Backend- und Client-Anbindung sauber
abgeschlossen ist oder ob innerhalb dieses engen Scopes noch genau ein
weiterer kleiner Haertungsschritt mit gutem Signal verbleibt.

## 1. Gepruefter Umfang

Innerhalb dieses Blocks wurden abgeschlossen:

- fachliche Inventur des naechsten Mapping-Schritts nach vorhandenem
  Herkunftsanker
- technischer Zuschnitt auf einen kleinen manuellen Mapping-Einstieg statt
  frueher Vorschlags- oder Automatiklogik
- Backend-Erweiterung der `quote_items` um optionales `material_id` und
  engen `price_mapping_status`
- Guard Rails fuer gueltiges `materials(id)` und den Statusraum
  `open/manual`
- minimale Client-Anbindung dieser Felder direkt im bestehenden
  Quote-Editor

Bewusst ausserhalb des Blocks geblieben:

- Materialsuche oder Materialauswahl-Dialog
- Preisvorschlaege
- Materialvorschlaege
- Steuer- oder Kontierungsvorschlaege
- automatische Statusfortschreibung
- globale Mapping-Seite oder Mapping-Cockpit

## 2. Audit-Ergebnis

Der erste manuelle Preis-/Material-Mapping-Einstieg ist innerhalb seines
bewusst kleinen Zuschnitts sauber abgeschlossen.

Die urspruengliche Luecke wurde geschlossen:

- uebernommene Quote-Positionen koennen jetzt manuell mit einem Materialbezug
  versehen werden
- der Mapping-Zustand ist mit einem kleinen, kontrollierten Statusraum
  sichtbar und schreibbar
- die Eingabe ist bereits im bestehenden Quote-Editor erreichbar, ohne neuen
  Workflow-Screen

## 3. Warum kein weiterer kleiner Haertungsschritt uebrig ist

Innerhalb dieses Blocks bleibt kein einzelner kleiner Haertungsschritt mit
gutem Signal uebrig.

Gruende:

- die zentrale Minimalfaehigkeit des Blocks ist bereits vollstaendig:
  manueller Mapping-Anker direkt an der Quote-Position
- Backend und Client sprechen denselben bewusst engen Feldsatz
- Guard Rails fuer Materialbezug und Statusraum sind bereits vorhanden
- die UI ist direkt dort sichtbar, wo Quote-Positionen ohnehin bearbeitet
  werden

Naheliegende Folgeideen waeren bereits neue Ausbaustufen und nicht mehr
kleine Haertung:

- Materialsuche statt freier `material_id`
- Vorschlagslogik aus Beschreibung oder Herkunftsdaten
- automatische Preisuebernahme aus Material oder Kalkulation
- Sammelsicht offener Mapping-Faelle

## 4. Risiko- und Signalbewertung

Der Block hat gutes Signal, weil er:

- den vorhandenen Herkunftsanker sinnvoll nutzbar macht
- keine Preisautomatik vorwegnimmt
- keine neue Prozesskomplexitaet einfuehrt
- das Datenmodell fuer spaetere Vorschlags- oder Automatiklogik vorbereitet

Ein weiterer Mini-Schritt innerhalb desselben Blocks wuerde dagegen entweder
nur kosmetisch sein oder bereits in Materialsuche, Vorschlaege oder
Automatik kippen.

## 5. Entscheidung

Der Block `manueller Preis-/Material-Mapping-Einstieg` kann vor spaeterer
Vorschlags- oder Automatiklogik bewusst als sauber beendet betrachtet
werden.

## 6. Naechster sinnvoller Schritt

Der naechste sinnvolle Ausbau liegt nicht in weiterer Feinhaertung dieses
manuellen Einstiegs, sondern in der naechsten fachlichen Inventur fuer
spaetere Vorschlags- oder Automatiklogik auf Basis des nun vorhandenen
Mapping-Ankers.

Der beste Kandidat mit neuem Signal ist:

- den kleinsten sinnvollen Einstiegspunkt fuer spaetere Material- oder
  Preisvorschlaege fachlich inventarisieren

Fuer den aktuellen Block gilt jedoch:

- **abgeschlossen**
