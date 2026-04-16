# GAEB-Importlauf-Freigabe und kontrollierte Quote-Uebernahme: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument beschreibt die kleinste technische Ausbauphase nach der
fachlichen Inventur fuer Importlauf-Freigabe und kontrollierte
Quote-Uebernahme.

Der Scope bleibt bewusst eng:

- Importlauf erstmals kontrolliert auf `reviewed` freigeben
- aus einem freigegebenen Importlauf genau **eine** neue Quote erzeugen
- nur `accepted`-Importpositionen uebernehmen
- Importlauf danach auf `applied` setzen und mit `created_quote_id`
  referenzieren

Bewusst nicht Teil dieses Schritts:

- Uebernahme in bestehende Quotes
- Preis- oder Material-Mapping
- Rohdatenedit waehrend der Uebernahme
- Re-Apply oder Ruecknahme einer bereits erzeugten Quote
- Bulk-Freigabe oder Sammeluebernahme mehrerer Importlaeufe

## 1. Kernentscheidung

Der erste kontrollierte Apply-Pfad soll auf Importlauf-Ebene arbeiten, aber
fachlich so klein wie moeglich bleiben.

Das minimale technische Zielbild ist:

1. `parsed`-Importlauf wird explizit auf `reviewed` gesetzt
2. nur danach darf eine kontrollierte Quote-Uebernahme stattfinden
3. die Uebernahme erzeugt genau **eine neue Quote im Status `draft`**
4. der Importlauf wird danach auf `applied` gesetzt und mit der neuen Quote
   verknuepft

## 2. Warum dieser Schnitt klein genug ist

Nach Upload, Parse-Speicher, Read-only-Sicht und Positions-Review liegt die
aktuelle Hauptluecke nicht mehr im Parser oder UI-Detail, sondern in der
kontrollierten Uebergabe an die Angebotsdomaene.

Ein kleiner Apply-Pfad ist klein genug, weil er:

- nur ein bereits vorhandenes Zielobjekt nutzt: `quotes`
- keine bestehenden Quotes mutiert
- nur auf freigegebenen Importlaeufen arbeitet
- die erste Rueckverfolgbarkeit `Importlauf -> Quote` sauber schliesst

## 3. Vorhandene Bausteine, die genutzt werden sollen

Bereits vorhanden sind:

- `quote_imports.status`
- `quote_imports.created_quote_id`
- `quote_import_items.review_status`
- `quotes.Service.Create(...)`

Das bedeutet:

- es braucht fuer den naechsten Schritt **keine neue Grundarchitektur**
- sondern nur einen kleinen orchestrierenden Pfad zwischen vorhandenem
  Importlauf und bestehender Quote-Erzeugung

## 4. Empfohlene Importlauf-Freigabe

### 4.1 Statuspfad

Der erste technisch aktiv genutzte Freigabe-/Apply-Pfad soll sein:

- `parsed -> reviewed -> applied`

`failed` bleibt Fehlerpfad.

`uploaded` bleibt vorgelagerter Zustand vor Parse.

### 4.2 Kleinste Freigaberegel

Ein Importlauf darf nur auf `reviewed` gesetzt werden, wenn:

- der Importlauf existiert
- der Importlauf im Status `parsed` ist
- keine `quote_import_items.review_status = 'pending'` mehr vorhanden sind
- mindestens eine Position `accepted` oder `rejected` bewertet wurde

Bewusst noch nicht notwendig:

- weitere Aggregationsfelder auf Importlauf-Ebene
- Review-Zaehler in der Datenbank

## 5. Empfohlenes Minimalzielbild fuer die Apply-Logik

### 5.1 Zielobjekt

Der erste Apply-Schritt darf nur dieses Ziel beschreiben:

- **eine neue Quote im Status `draft`**

Keine Mutation:

- bestehender Quotes
- bestehender Quote-Items

### 5.2 Quellmenge

Nur Positionen mit

- `review_status = 'accepted'`

duerfen in die neue Quote einfliessen.

Positionen mit

- `pending`
- `rejected`

duerfen nicht uebernommen werden.

### 5.3 Guard Rails

Der erste Apply-Pfad sollte mindestens diese Regeln erzwingen:

- Importlauf muss existieren
- Importlauf muss im Status `reviewed` sein
- `created_quote_id` darf noch nicht gesetzt sein
- es muss mindestens eine `accepted`-Position geben
- es darf keine `pending`-Position mehr geben

## 6. Erste sichere Feldabbildung in die Quote

Die erste Uebernahme soll bewusst nur das bestehende Quote-Modell befuellen,
ohne Preislogik vorzutaeuschen.

Empfohlene Abbildung:

- `project_id` des Importlaufs -> `QuoteInput.project_id`
- `contact_id` des Importlaufs -> `QuoteInput.contact_id` falls vorhanden
- `description` -> `QuoteItemInput.description`
- `qty` -> `QuoteItemInput.qty`
- `unit` -> `QuoteItemInput.unit`
- `unit_price = 0`
- `tax_code = ''`

Zusaetzlich sinnvoll:

- `note` kann einen kleinen Herkunftshinweis enthalten, z. B. dass das Angebot
  aus einem GAEB-Importlauf erzeugt wurde

Nicht Teil dieses Blocks:

- Preisvorschlaege
- Steuercodeableitung
- Materialmapping

## 7. Kleinstes Service-Zielbild

Im bestehenden `server/internal/quotes/imports.go` liegt der passende Ort fuer
den ersten Freigabe-/Apply-Orchestrator.

Empfohlene kleine Service-Erweiterungen:

- `MarkImportReviewed(ctx, importID)`
- `ApplyImportToDraftQuote(ctx, importID)`

### 7.1 Verantwortung von `MarkImportReviewed(...)`

- Importlauf validieren
- Status `parsed` pruefen
- offene `pending`-Positionen ausschliessen
- Importlauf auf `reviewed` setzen
- aktualisierte Importansicht zurueckgeben

### 7.2 Verantwortung von `ApplyImportToDraftQuote(...)`

- Importlauf validieren
- Status `reviewed` pruefen
- `created_quote_id` auf Leerheit pruefen
- akzeptierte Positionen laden
- neue Draft-Quote ueber bestehenden Quote-Service erzeugen
- `created_quote_id` schreiben
- Importlauf auf `applied` setzen
- Import- und Quote-Referenz zurueckgeben

## 8. Kleinstes HTTP-Zielbild

Der erste technische Zuschnitt sollte bewusst nur zwei kleine Importlauf-Routen
einfuehren:

- `PATCH /api/v1/quotes/imports/{id}/review`
- `POST /api/v1/quotes/imports/{id}/apply`

### 8.1 `PATCH /review`

Verantwortung:

- Importlauf auf `reviewed` setzen

Kein Body erforderlich oder maximal ein spaeterer Leer-Body.

### 8.2 `POST /apply`

Verantwortung:

- neue Draft-Quote aus dem freigegebenen Importlauf erzeugen

Minimaler Response:

- aktualisierter Importlauf
- erzeugte Quote oder Quote-Referenz

## 9. Integritaets- und Idempotenzregeln

Bereits im ersten Apply-Block sinnvoll:

- `reviewed` nur einmalig aus `parsed`
- `applied` nur einmalig aus `reviewed`
- `created_quote_id` verhindert doppelte Quote-Erzeugung
- bei bereits gesetztem `created_quote_id` fachlich sauber fehlschlagen

Damit bleibt der erste Apply-Schnitt technisch kontrollierbar und fachlich
nachvollziehbar.

## 10. Bewusste Abgrenzung zu spaeteren Schritten

Nicht in denselben Block ziehen:

- globale Importlauf-Uebersicht mit Freigabe-Workflow
- Batch-Freigabe
- Apply in bestehende Quotes
- Ruecknahme eines `applied`-Importlaufs
- partielle Re-Apply-Logik nach spaeteren Review-Aenderungen
- Preis-, Material- oder KI-Anreicherung

## 11. Naechster sinnvoller technischer Schritt

Nach diesem Dokument ist der naechste kleine und saubere Umsetzungsschritt:

1. kleine Importlauf-Service-Erweiterung fuer `reviewed` und `applied`
2. Verbindung zum bestehenden Quote-Service fuer neue Draft-Quotes
3. 1-2 fokussierte Tests fuer Freigabe und erfolgreiche Apply-Guard-Rails

## 12. Entscheidung

Der naechste technische GAEB-Ausbauschritt wird bewusst auf genau einen kleinen
Freigabe-/Apply-Pfad begrenzt:

- Importlauf von `parsed` auf `reviewed`
- danach kontrollierte Erzeugung genau einer neuen Draft-Quote aus
  `accepted`-Positionen
- Abschluss mit `created_quote_id` und Status `applied`

Ohne Preislogik, ohne Quote-Mutation und ohne Re-Apply-Komplexitaet.
