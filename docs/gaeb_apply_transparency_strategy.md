# Schlanke Apply-Transparenz: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet den kleinsten technischen Folgeausbau nach
abgeschlossenem Apply- und Navigations-MVP zu:

- kleine, read-only Apply-Transparenz auf Importlauf-Ebene
- basierend auf bereits vorhandenen Review-Daten
- ohne Mapping, Re-Apply oder globale Workflow-Erweiterung

## 1. Ausgangslage

Der aktuelle Stand deckt bereits ab:

- `parsed -> reviewed -> applied`
- Erzeugung genau einer neuen Draft-Quote aus `accepted`-Positionen
- Rueckverknuepfung ueber `created_quote_id`
- direkte Navigation zur erzeugten Quote aus dem Importdialog

Was noch fehlt, ist eine knappe fachliche Zusammenfassung des bereits
ausgefuehrten Import-/Review-/Apply-Ergebnisses.

## 2. Kernentscheidung

Der kleinste sinnvolle technische Zuschnitt ist:

- bestehende Importlauf-Responses um kleine Summary-Felder erweitern
- diese Felder direkt aus `quote_import_items.review_status` ableiten
- spaeter im bestehenden Importdialog read-only anzeigen

Es wird bewusst kein neuer Endpoint eingefuehrt.

## 3. Minimales Datenmodell

`QuoteImport` soll um genau diese Transparenzfelder erweitert werden:

- `accepted_count`
- `rejected_count`
- `pending_count`

Optional denkbar, aber im MVP nicht noetig:

- `applied_item_count`

Im kleinen ersten Schritt reicht:

- `accepted_count` als Zahl der uebernommenen Positionen
- `rejected_count` als Zahl der bewusst nicht uebernommenen Positionen
- `pending_count` als Rest-/Offenheitsindikator

## 4. Ableitungslogik

Die Werte werden nicht persistent gespeichert, sondern bei bestehenden
Leseoperationen direkt aggregiert:

- `accepted_count`: Anzahl `quote_import_items` mit `review_status='accepted'`
- `rejected_count`: Anzahl `quote_import_items` mit `review_status='rejected'`
- `pending_count`: Anzahl `quote_import_items` mit `review_status='pending'`

Damit bleibt der Schritt:

- konsistent zur aktuellen Review-Quelle
- frei von Synchronisationslogik
- ohne neue Migrations- oder Schreibpfade

## 5. Technischer Zuschnitt

Backend:

- `server/internal/quotes/imports.go`
  - `QuoteImport` um die drei Count-Felder erweitern
  - bestehende Importlauf-Lesepfade (`ListImports(...)`, `GetImport(...)`)
    um kleine Aggregation aus `quote_import_items` erweitern

HTTP:

- keine neue Route
- bestehende Importlauf-Responses liefern die neuen Felder automatisch mit

Client:

- noch kein groesserer Ausbau in diesem Schritt
- Folgeschritt kann die neuen Felder read-only im bestehenden Importdialog
  sichtbar machen

## 6. Guard Rails und Abgrenzung

Bewusst nicht Teil dieses Blocks:

- neue Apply- oder Re-Apply-Logik
- Ruecknahme von `applied`
- Mapping auf Preise, Steuern oder Materialien
- globale Dashboard-/Cockpit-Sicht
- Persistenz oder Caching der Summary-Werte

Der Schritt bleibt rein lesend und risikoarm.

## 7. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Umsetzungsschritt:

- kleine Backend-Erweiterung fuer Importlauf-Responses, die
  `accepted_count`, `rejected_count` und `pending_count` liefert.
