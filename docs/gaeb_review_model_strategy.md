# GAEB-Review: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument beschreibt die kleinste technische Ausbauphase nach der
fachlichen Review-Inventur fuer `quote_import_items`.

Der Scope bleibt bewusst eng:

- pro Importposition nur `review_status` schreibbar machen
- optional `review_note` pflegbar machen
- die Rohpositionsdaten selbst unveraendert lassen

Bewusst nicht Teil dieses Schritts:

- Quote-Uebernahme
- Bulk-Review
- Importlauf auf `reviewed` setzen
- Editieren von `description`, `qty`, `unit` oder `position_no`
- Parser- oder Mapping-Logik

## 1. Kernentscheidung

Der erste schreibende Review-Pfad soll nur auf Ebene einzelner
`quote_import_items` arbeiten.

Das kleinste technische Zielbild ist:

- `PATCH /api/v1/quotes/imports/{id}/items/{itemID}/review`

mit einem sehr kleinen Body:

- `review_status`
- `review_note`

## 2. Warum dieser Schnitt klein genug ist

Nach Upload, Parse-Speicher und Read-only-Sicht ist die aktuelle Hauptluecke:

- Benutzer koennen Rohpositionen sehen, aber noch nicht fachlich bewerten

Ein erster Review-Pfad nur fuer Status und Notiz ist klein genug, weil er:

- keine Rohdaten mutiert
- keinen finalen Angebotsdatensatz erzeugt
- keine Importlauf-Freigabe voraussetzt
- keinen neuen komplexen UI-Flow erzwingt

## 3. Empfohlenes Datenmodell

Die Tabelle `quote_import_items` enthaelt bereits:

- `review_status TEXT NOT NULL DEFAULT 'pending'`
- `review_note TEXT NOT NULL DEFAULT ''`

Fuer den naechsten Schritt braucht es deshalb **keine neue Migration**,
sondern nur:

- klare Validierungsregeln
- einen kleinen Schreibpfad

## 4. Erlaubte Statuswerte

Der erste schreibende Review-Pfad soll genau diese Werte erlauben:

- `pending`
- `accepted`
- `rejected`

### 4.1 Semantik

- `pending`
  - noch nicht fachlich bewertet
- `accepted`
  - fuer spaetere Uebernahme prinzipiell geeignet
- `rejected`
  - soll nicht in eine spaetere Uebernahme einfliessen

Weitere Stati wie `edited` oder `mapped` bleiben bewusst ausserhalb dieses
Blocks.

## 5. Guard Rails

Der erste Review-Schreibpfad sollte bewusst eng abgesichert werden.

Empfohlene Minimalregeln:

- der Importlauf muss existieren
- das Item muss zum angegebenen Importlauf gehoeren
- der Importlauf muss im Status `parsed` sein
- `review_status` darf nur einer der erlaubten Werte sein
- `review_note` darf leer sein, soll aber serverseitig auf String normalisiert
  werden

Bewusst noch nicht erforderlich:

- Vollstaendigkeitspruefung aller Items eines Importlaufs
- automatische Statusfortschreibung des Importlaufs auf `reviewed`

## 6. Kleinstes Service-Zielbild

Im bestehenden `server/internal/quotes/imports.go` ist der passende Ort fuer
den ersten Review-Pfad.

Empfohlene kleine Service-Erweiterung:

- `UpdateImportItemReview(ctx, importID, itemID, status, note)`

Verantwortung dieser Methode:

- Importlauf und Item validieren
- Importstatus auf `parsed` pruefen
- `review_status` und `review_note` schreiben
- aktualisierte Item-Detailansicht zurueckgeben

## 7. Kleinstes HTTP-Zielbild

Empfohlene erste Route:

- `PATCH /api/v1/quotes/imports/{id}/items/{itemID}/review`

Minimaler Request-Body:

- `review_status`
- `review_note`

Minimaler Response-Body:

- aktualisierte Importposition

## 8. Kleine Integritaetsregeln

Bereits in diesem Schritt sinnvoll:

- `review_note` auf `''` normalisieren statt `null`
- Whitespace bei `review_note` trimmen
- bei ungueltigem Status `400`
- bei nicht passendem Import/Item ebenfalls fachlich sauber fehlschlagen

## 9. Bewusste Abgrenzung zu spaeteren Schritten

Nicht in denselben Block ziehen:

- Sammelaktionen wie "alle akzeptieren"
- automatische Importlauf-Freigabe
- Positionen im UI frei editieren
- Uebernahme akzeptierter Positionen in `quote_items`
- Mapping auf Material-, Leistungs- oder Preisstrukturen

## 10. Naechster sinnvoller technischer Schritt

Nach diesem Dokument ist der naechste kleine und saubere Umsetzungsschritt:

1. kleinen Review-Service in `server/internal/quotes/imports.go` einfuehren
2. `PATCH`-Route fuer `review_status` und `review_note` anbinden
3. 1-2 fokussierte Tests fuer Erfolg und Guard Rails ergaenzen

## 11. Entscheidung

Der naechste technische GAEB-Ausbauschritt wird bewusst auf genau einen kleinen
schreibenden Review-Pfad begrenzt:

- `review_status`
- `review_note`

pro einzelner `quote_import_item`, ohne Rohdatenedit, Importlauf-Freigabe oder
Quote-Uebernahme.
