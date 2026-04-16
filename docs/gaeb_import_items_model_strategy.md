# GAEB-Importpositionen: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument beschreibt die kleinste technische Ausbauphase nach dem
abgeschlossenen parserfreien GAEB-Importlauf.

Der Scope bleibt bewusst eng:

- geparste Rohpositionen als eigene Datenstruktur speichern
- Importlaeufe von `uploaded` nach `parsed` oder `failed` bringen
- einen spaeteren Review-Pfad vorbereiten

Bewusst nicht Teil dieses Schritts:

- echter GAEB-Parser
- Review-UI
- Quote-Erzeugung aus Importpositionen
- Preislogik
- Material- oder Artikel-Matching
- KI-Unterstuetzung

## 1. Kernentscheidung

Die naechste technische Einheit nach `quote_imports` soll sein:

- `quote_import_items`

`quote_import_items` repraesentieren bewusst noch keine finalen
`quote_items`, sondern parserseitig erzeugte Rohpositionen, die spaeter
reviewt und erst danach in die Angebotsdomaene uebernommen werden.

Parallel dazu wird der bisher nur vorbereitete Importstatusraum erstmals
aktiv genutzt:

- `uploaded -> parsed`
- `uploaded -> failed`

## 2. Warum jetzt `quote_import_items`

Nach dem parserfreien Importlauf ist die Datei- und Metadatenebene bereits
sauber vorhanden:

- `quote_imports` existiert
- Quelldatei liegt in GridFS
- Projekt- und optionaler Kontaktbezug sind vorhanden
- eine kleine List-/Detail-Sicht existiert bereits

Die eigentliche naechste technische Luecke ist deshalb:

- wohin geparste GAEB-Positionen geschrieben werden
- wie ein Importlauf als erfolgreich oder fehlgeschlagen markiert wird

Ohne `quote_import_items` gaebe es keinen stabilen Anker fuer spaetere
Review-, Mapping- oder Quote-Uebernahmeschritte.

## 3. Empfohlenes Tabellenmodell

### 3.1 Neue Tabelle `quote_import_items`

Empfohlenes Minimalfeldset:

- `id UUID PRIMARY KEY`
- `import_id UUID NOT NULL REFERENCES quote_imports(id) ON DELETE CASCADE`
- `position_no TEXT NOT NULL`
- `outline_no TEXT NOT NULL DEFAULT ''`
- `description TEXT NOT NULL`
- `qty NUMERIC(15,3) NOT NULL DEFAULT 0`
- `unit TEXT NOT NULL DEFAULT ''`
- `is_optional BOOLEAN NOT NULL DEFAULT false`
- `parser_hint TEXT NOT NULL DEFAULT ''`
- `review_status TEXT NOT NULL DEFAULT 'pending'`
- `review_note TEXT NOT NULL DEFAULT ''`
- `sort_order INT NOT NULL DEFAULT 0`

### 3.2 Bedeutung der Felder

- `import_id`
  - bindet jede Rohposition eindeutig an einen bestehenden Importlauf
- `position_no`
  - sichtbare Positionsnummer aus der Quelldatei
- `outline_no`
  - flacher Anker fuer Gliederungs-/OZ-Kontext, ohne schon einen Baum zu bauen
- `description`
  - normalisierte Kurz-/Langtextbasis der Position
- `qty`
  - importierte Menge
- `unit`
  - importierte Einheit in Rohform
- `is_optional`
  - Markierung fuer Bedarfs-/Alternativpositionen, sofern parserseitig
    ableitbar
- `parser_hint`
  - Platz fuer kleine technische Hinweise wie Formatvariante oder
    Auffaelligkeiten
- `review_status`
  - spaetere manuelle Freigabe oder Verwerfung je Position
- `review_note`
  - kleine manuelle Notiz fuer spaetere Review-Stufen
- `sort_order`
  - stabile Reihenfolge fuer Anzeige und spaetere Quote-Uebernahme

## 4. Warum dieses Modell klein genug ist

Das Modell verzichtet bewusst auf:

- Eltern-/Kind-Beziehungen fuer komplette LV-Baeume
- Preis- und Zuschlagsfelder
- Steuer- oder Materiallogik
- parserinterne JSON-Rohbloecke
- Aenderungsprotokolle pro Position

Damit bleibt `quote_import_items` nur ein flacher Rohpositionscontainer und
nicht schon eine vollstaendige GAEB-Engine.

## 5. Minimaler Statuspfad auf `quote_imports`

Der in `quote_imports` bereits angelegte Statusraum soll nun erstmals
technisch genutzt werden.

### 5.1 Aktiv zu verwendende Stati im naechsten Block

- `uploaded`
- `parsed`
- `failed`

### 5.2 Bewusst noch nicht aktiv

- `reviewed`
- `applied`

Diese beiden Stati bleiben fuer spaetere Review- bzw. Quote-Uebernahmephasen
reserviert.

### 5.3 Minimale Regeln

- neuer Upload startet immer in `uploaded`
- erfolgreicher Parserlauf setzt den Import auf `parsed`
- Fehler im Parserlauf setzen den Import auf `failed`
- bei `failed` wird `error_message` auf dem Importlauf gepflegt
- bei `parsed` bleibt `error_message` leer

## 6. Minimale Review-Idee fuer Positionen

Auch wenn noch keine Review-UI gebaut wird, sollte das Datenmodell bereits
einen kleinen Review-Raum vorbereiten.

Empfohlene Anfangswerte:

- `pending`
- `accepted`
- `rejected`

MVP-Regel fuer den naechsten technischen Schritt:

- neu erzeugte `quote_import_items` starten immer mit `review_status='pending'`

## 7. Kleinstes Service-Zielbild

Nach vorhandenem Muster im Repo sollte die naechste technische Stufe noch
keinen grossen Parser-Service aufspalten.

Kleinster sinnvoller Backend-Schnitt:

- bestehendes `quotes/imports.go` erweitern oder kleines Schwesterfile
  daneben einfuehren
- eine interne Methode fuer:
  - Importlauf auf `parsed` setzen
  - Importlauf auf `failed` setzen
  - vorhandene Items eines Importlaufs ersetzen
  - neue `quote_import_items` geordnet schreiben

Wichtig:

- Parserlauf sollte idempotent auf einem einzelnen Importlauf arbeiten koennen
- bei erneutem Parse-Versuch fuer denselben Lauf muessen alte Rohpositionen
  sauber ersetzt werden koennen

## 8. Minimales HTTP-Zielbild fuer die Folgestufe

Fuer diese Strategiestufe wird bewusst noch kein finaler Review-Endpoint
festgelegt. Der kleinste sinnvolle erste Ausbau bleibt intern bzw. sehr klein.

Empfohlene naechste Zielrichtung:

- noch kein allgemeiner Edit-Endpoint fuer einzelne Importpositionen
- zuerst nur technische Parser-/Statuspfade vorbereiten
- spaeter read-only Item-List/Detail fuer Review-Sicht

Das vermeidet, dass ein halbfertiger Review-Flow entsteht, bevor der
Rohpositionsspeicher stabil ist.

## 9. Kleine Integritaetsregeln

Bereits in der naechsten technischen Stufe sinnvoll:

- `quote_import_items` nur fuer bestehende `quote_imports`
- bei Loeschung eines Importlaufs fallen Items per `ON DELETE CASCADE` mit weg
- `sort_order` pro Importlauf stabil und lueckenarm vergeben
- `review_status` auf kleinen erlaubten Stringraum begrenzen
- kein Schreiben von Items, wenn der Importlauf fachlich nicht mehr existiert

## 10. Bewusste Abgrenzung zu spaeteren Schritten

Nicht in denselben Block ziehen:

- echte Formatabdeckung fuer X83/X84/GAEB-XML
- Hierarchieabbildung mit Losen/Titeln/Untertiteln
- manuelle Positionsbearbeitung im UI
- Uebernahme akzeptierter Positionen in `quote_items`
- Preis- oder Materialvorschlaege
- KI-Unterstuetzung fuer Mapping oder Textbereinigung

## 11. Naechster sinnvoller technischer Schritt

Nach diesem Dokument ist der naechste kleine und saubere Umsetzungsschritt:

1. Migration fuer `quote_import_items`
2. kleine Service-Erweiterung fuer Rohpositionsspeicherung
3. Importstatuspfad `uploaded -> parsed/failed` aktivieren
4. Tests fuer erfolgreichen und fehlgeschlagenen Parserlauf auf
   Importlauf-Ebene

Noch nicht erforderlich:

5. Review-UI
6. Quote-Uebernahme

## 12. Entscheidung

Der naechste technische GAEB-Ausbauschritt wird bewusst auf zwei Dinge
begrenzt:

- `quote_import_items` als flacher Rohpositionsspeicher
- Aktivierung des minimalen Importstatuspfads `uploaded -> parsed/failed`

Damit entsteht ein sauberer technischer Unterbau fuer spaetere Parser-,
Review- und Uebernahmelogik, ohne diese schon in denselben Schritt
hineinzuziehen.
