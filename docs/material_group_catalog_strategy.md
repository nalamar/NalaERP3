# Materialgruppen-Katalogstrategie

## Ziel
- Materialgruppen sollen von einem Freitextfeld zu einem administrierbaren Referenzdatenkatalog werden.
- Bestehende Materials-CRUDs und Filter sollen waehrend der Umstellung weiter funktionieren.
- Die erste Ausbaustufe soll klein bleiben: Katalog-Persistenz, minimale Settings-API und klare Migrationsstrategie.

## Ausgangslage
- Heute liegt die Gruppierung in `materials.kategorie` als Freitext.
- `materials.Service.ListCategories()` liefert nur `SELECT DISTINCT kategorie FROM materials`.
- Der Client nutzt diese Werte bereits fuer Filter und Bearbeitung, hat aber keine echte Katalogverwaltung.
- Es gibt noch keinen Loeschschutz, keine Sortierung, kein Aktiv-Flag und keine Beschreibung.

## Zielmodell

### Tabelle
Empfohlene neue Tabelle: `material_groups`

Empfohlene Felder:
- `code text primary key`
- `name text not null`
- `description text not null default ''`
- `sort_order integer not null default 0`
- `is_active boolean not null default true`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`

### Identifikationsregel
- `code` ist der stabile technische Schluessel.
- `name` ist die fachliche Bezeichnung fuer UI und Auswahl.
- In der ersten Ausbaustufe darf `code` identisch zu heutigen `kategorie`-Werten sein, um die Ueberfuehrung einfach zu halten.

## Migrationsstrategie

### Stufe 1: Katalog aufbauen, Materialschema noch nicht brechen
- Neue Tabelle `material_groups` anlegen.
- Bestehende nicht-leere `materials.kategorie`-Werte in `material_groups` uebernehmen.
- `materials.kategorie` bleibt vorerst unveraendert bestehen.
- Vorteil:
  - keine sofortige harte Migration aller Materials-CRUDs,
  - bestehende Filter und JSON-Payloads bleiben kompatibel,
  - Katalog kann bereits administriert werden.

### Stufe 2: Materialeingaben gegen Katalog validieren
- Bei Material-Create/Update wird `kategorie` nur noch akzeptiert, wenn sie leer ist oder auf einen aktiven `material_groups.code` zeigt.
- Die Distinct-Liste in `ListCategories()` wird durch Katalogdaten ersetzt.

### Stufe 3: Optionales hartes Referenzmodell
- Spaeter kann `materials.kategorie` durch `material_group_code` ersetzt oder intern umbenannt werden.
- Diese dritte Stufe ist fuer den aktuellen Subtask nicht noetig und sollte erst nach stabiler UI- und API-Nutzung erfolgen.

## Minimale API fuer die erste Ausbaustufe

### Settings-Verwaltung
Neue Route unter `settings.manage`:
- `GET /api/v1/settings/material-groups`
- `POST /api/v1/settings/material-groups`
- `DELETE /api/v1/settings/material-groups/{code}`

Minimales DTO:
- `code`
- `name`
- `description`
- `sort_order`
- `is_active`

Verhalten:
- `GET` liefert aktive und inaktive Gruppen sortiert nach `sort_order`, dann `name`.
- `POST` arbeitet als Upsert.
- `DELETE` entfernt nur Gruppen, die von keinem Material verwendet werden.
  Falls die Gruppe noch referenziert ist, wird ein fachlicher Fehler zurueckgegeben.

### Materials-Lesepfad
- Bestehende Material-Endpoints bleiben in Stufe 1 unveraendert.
- `listMaterialCategories()` soll spaeter statt Distinct-Werten die aktiven Katalogcodes oder Katalognamen liefern.
- Fuer den ersten API-Ausbau reicht die neue Settings-Route, damit die Administration entkoppelt vorbereitet ist.

## Service-Schnittstelle

Empfohlener neuer Backend-Service:
- Datei: `server/internal/settings/material_groups.go`

Minimale Operationen:
- `List(ctx) ([]MaterialGroup, error)`
- `Upsert(ctx, in MaterialGroup) error`
- `Delete(ctx, code string) error`

Empfohlenes Modell:
- `type MaterialGroup struct { Code, Name, Description string; SortOrder int; IsActive bool }`

## Validierungsregeln
- `code` erforderlich, getrimmt, case-stabil speichern.
- `name` erforderlich.
- `sort_order` optional, Standard `0`.
- `is_active=false` statt Delete ist spaeter fuer schon verwendete Gruppen oft sinnvoller.
- Fuer die erste Ausbaustufe bleibt physisches Delete erlaubt, aber nur ohne bestehende Materialreferenzen.

## UI-Folgepfad
- Settings-Seite um einen kleinen Bereich `Materialgruppen` erweitern.
- Analoger Bedienpfad zu Einheiten:
  - Liste bestehender Gruppen
  - einfaches Anlegen/Bearbeiten
  - Loeschen nur wenn unverwendet
- Materials-Seite spaeter auf Katalogdaten statt Distinct-Liste umstellen.

## Empfohlene Reihenfolge der Umsetzung
1. Migration fuer `material_groups` anlegen und bestehende Kategorien einsammeln.
2. `settings.MaterialGroupService` implementieren.
3. HTTP-Route unter `/settings/material-groups` anbinden.
4. Settings-UI fuer Materialgruppen ergaenzen.
5. Materials-Facets und Material-Validierung auf Katalogdaten umstellen.

## Bewusste Nicht-Ziele fuer diesen Schritt
- Noch keine Vollmigration von `materials.kategorie` auf Foreign Key.
- Noch keine generische Referenzdaten-Engine fuer alle Katalogarten.
- Noch keine Statuskatalog-Migration; diese bleibt ein spaeterer Strang.
