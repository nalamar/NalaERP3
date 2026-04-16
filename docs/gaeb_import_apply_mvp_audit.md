# GAEB-Importlauf-Freigabe und kontrollierte Quote-Uebernahme: Abschluss-Audit

## Ziel dieses Dokuments

Dieses Dokument bewertet den nach `3.1.8.4` erreichten Stand des ersten
GAEB-Freigabe-/Apply-MVP.

Geprueft wird bewusst nur, ob innerhalb dieses eng geschnittenen Blocks noch
genau ein weiterer kleiner Haertungsschritt mit gutem Signal uebrig ist oder
ob der Block fachlich und technisch sauber abgeschlossen werden sollte.

## 1. Abgedeckter Scope

Der aktuelle Stand deckt jetzt den kleinsten kontrollierten Uebergang von
bewerteten Importpositionen in die Angebotsdomaene ab:

- Importlauf kann von `parsed` auf `reviewed` gesetzt werden
- Freigabe ist nur ohne offene `pending`-Positionen moeglich
- kontrollierte Uebernahme arbeitet nur auf `reviewed`-Importlaeufen
- nur `accepted`-Positionen werden uebernommen
- Ziel ist immer genau eine neue Quote im Status `draft`
- `created_quote_id` verknuepft Importlauf und erzeugte Quote
- der Importlauf endet danach auf `applied`
- der bestehende GAEB-Importdialog macht Freigabe und Apply direkt sichtbar

Damit ist die Strecke

- `parse -> positionen reviewen -> importlauf freigeben -> neue draft-quote erzeugen`

erstmals durchgaengig vorhanden.

## 2. Bereits vorhandene Guard Rails

Der aktuelle MVP enthaelt bereits die fachlich wichtigsten Schutzmechanismen:

- keine Freigabe bei offenen `pending`-Positionen
- kein Apply ausserhalb des Status `reviewed`
- kein Apply ohne `accepted`-Positionen
- keine zweite Quote-Erzeugung bei bereits gesetztem `created_quote_id`
- keine Mutation bestehender Quotes
- keine implizite Preis- oder Steuerlogik
- klare Rueckverfolgung von Importlauf zu erzeugter Quote

Das ist fuer den ersten kontrollierten Apply-Pfad bereits ein sauberer und
vertretbarer Sicherungsrahmen.

## 3. Was noch fehlt, aber kein kleiner Haertungsschritt mehr ist

Die verbleibenden sinnvollen Folgepunkte liegen jetzt bereits ausserhalb eines
kleinen MVP-Haertungsschritts:

- direkte Navigation in die neu erzeugte Quote
- Importlauf-Aggregation auf Listenebene
- globale Freigabe-/Apply-Uebersicht statt nur Dialogeinbettung
- Re-Apply- oder Ruecknahme-Logik
- Preis-, Steuer- oder Material-Mapping
- Uebernahme in bestehende Quotes
- feinere Apply-Transparenz wie Preview oder Delta-Sicht

Diese Themen veraendern den Bedienfluss oder die fachliche Reichweite bereits
spuerbar und sind deshalb keine kleine Resthaertung mehr.

## 4. Bewertung moeglicher Resthaertungen

Ein moeglicher Kandidat waere gewesen:

- zusaetzliche clientseitige Vorabpruefungen fuer Freigabe oder Apply

Das liefert hier aber nur schwaches Signal, weil die entscheidenden Regeln
bereits serverseitig abgesichert sind und der Client Fehler bereits ueber die
bestehenden Dialogpfade sichtbar macht.

Ein weiterer Kandidat waere:

- automatische Oeffnung der neu erzeugten Quote

Das verbessert die Nutzerfuehrung, ist aber keine Haertung des Apply-Pfads,
sondern bereits ein neuer UX-Ausbauschritt.

## 5. Entscheidung

Innerhalb des Blocks

- Importlauf-Freigabe
- kontrollierte Quote-Uebernahme

bleibt nach Backend- und Client-MVP **kein weiterer kleiner
Haertungsschritt mit gutem Signal** uebrig.

Der Block ist damit bewusst sauber beendet.

## 6. Naechster sinnvoller Schritt

Der naechste sinnvolle Ausbau liegt nicht mehr in weiterem Apply-Feinschliff,
sondern in einem neuen Folgeabschnitt nach der ersten Quote-Erzeugung.

Der naechste fachlich sinnvolle Strang ist:

- die Zielstrecke nach erzeugter Quote inventarisieren und entscheiden, ob
  zuerst Quote-Navigation, Apply-Transparenz oder spaeteres Mapping den
  kleinsten sinnvollen Ausbau bildet

Fuer den aktuellen Block gilt jedoch:

- **abgeschlossen**
