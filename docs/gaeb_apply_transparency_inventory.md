# Apply-Transparenz auf Importlauf-Ebene: Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Ausbau nach dem
abgeschlossenen Post-Apply-Navigationsschritt.

Fokus ist bewusst eng:

- Transparenz des bereits ausgefuehrten Apply-Vorgangs auf Importlauf-Ebene
- ohne Einstieg in Preis-/Material-/Steuermapping
- ohne Re-Apply oder Ruecknahme-Mechanik

## 1. Ausgangslage nach Navigation

Der aktuelle Stand deckt bereits ab:

- `parsed -> reviewed -> applied`
- Erzeugung genau einer neuen Draft-Quote aus `accepted`-Positionen
- Rueckverknuepfung ueber `created_quote_id`
- direkte Anschlussfuehrung per `Quote oeffnen` aus dem Importdialog

Damit ist die operative Uebergabe in die Angebotsbearbeitung vorhanden.

## 2. Verbleibende Transparenzluecke

Was im aktuellen Dialog noch fehlt, ist eine knappe fachliche Antwort auf:

- wie viele Positionen wurden aus dem Importlauf uebernommen
- wie viele Positionen wurden bewusst nicht uebernommen (`rejected`)
- ob beim Apply noch `pending` offen waren (fachlich nein, aber als Kontext
  nicht sichtbar)

Diese Luecke ist kein Mapping-Thema, sondern reine Nachvollziehbarkeit des
bereits ausgefuehrten Apply-Schritts.

## 3. Bestehende Datenbasis fuer kleinen Transparenzschritt

Die benoetigten Informationen sind bereits implizit vorhanden:

- `quote_import_items.review_status` (`accepted`, `rejected`, `pending`)
- Importstatus auf `quote_imports`
- `created_quote_id` als Apply-Anker

Damit kann ein kleiner Transparenzschnitt eingefuehrt werden, ohne neue
fachliche Kernlogik zu bauen.

## 4. Was jetzt bewusst nicht der naechste Schritt ist

Nicht Teil dieses Blocks:

- Preis-/Material-/Steuermapping
- Re-Apply nach spaeteren Review-Aenderungen
- Ruecknahme (`applied -> reviewed`)
- Uebernahme in bestehende Quotes
- globale Freigabe-/Apply-Konsole

Diese Themen sind groesser und gehoeren in spaetere Stränge.

## 5. Kleinster sinnvoller Einstiegspunkt

Der kleinste sinnvolle Folgepunkt ist:

- eine kompakte Apply-Zusammenfassung direkt im bestehenden Importdetail

Minimaler fachlicher Inhalt:

- `accepted_count`
- `rejected_count`
- `pending_count`
- optional `applied_item_count` (gleich `accepted_count` im MVP)

## 6. Warum dieser Schritt den besten Signalwert hat

- verbessert sofort die Nachvollziehbarkeit des bereits bestehenden Apply-Pfads
- reduziert Rueckfragen im operativen Alltag
- bleibt strikt read-only
- erfordert keine Erweiterung der Geschaeftslogik

## 7. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- minimales technisches Zielmodell fuer eine schlanke Apply-Transparenz
  festlegen (kleiner Backend-Read-Schnitt plus kleine Dialog-Anzeige),
  bevor groesseres Mapping gestartet wird.
