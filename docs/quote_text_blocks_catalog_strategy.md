# Angebots-Textbaustein-Katalog: Minimales technisches Zielmodell

## Ziel

Dieses Dokument ueberfuehrt den fachlichen Einstiegspunkt aus
`quote_templates_text_blocks_inventory.md` in ein kleines technisches
Zielmodell fuer einen administrierbaren Angebots-Textbaustein-Katalog.

Der Fokus liegt bewusst auf dem ersten Katalogpfad nach vorhandenem
Settings-Muster, nicht schon auf dem Quote-Editor-Einbau.

## Technische Leitplanken

Der erste Ausbau soll sich an den bereits vorhandenen administrierbaren
Settings-Katalogen orientieren:

- `units`
- `material_groups`

Das bedeutet fuer den ersten Schritt:

- eigene kleine Tabelle
- kleines Service-Objekt unter `server/internal/settings`
- kleine HTTP-Route unter `/api/v1/settings/...`
- spaeter minimale Client-Anbindung in `client/lib/api.dart`
- spaeter kleine Pflege-UI in `client/lib/pages/settings_page.dart`

Bewusst noch nicht Teil dieses Schritts:

- Quote-Editor-Einbau
- automatische Textuebernahme
- Platzhalter-Engine
- Versionierung
- Mehrsprachigkeit

## Empfohlenes Datenmodell

Empfohlene neue Tabelle:

- `quote_text_blocks`

### Minimalfeldset

- `id UUID PRIMARY KEY`
- `code TEXT NOT NULL UNIQUE`
- `name TEXT NOT NULL`
- `category TEXT NOT NULL`
- `body TEXT NOT NULL`
- `sort_order INTEGER NOT NULL DEFAULT 0`
- `is_active BOOLEAN NOT NULL DEFAULT TRUE`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`

### Begründung der Felder

- `id`
  - stabiler interner Identifikator fuer spaetere Referenzen aus dem
    Quote-Editor
- `code`
  - maschinenfreundlicher, stabiler Schluessel fuer Seeds und Exporte
- `name`
  - sprechender Anzeigename im UI
- `category`
  - einfache fachliche Gruppierung ohne neue Referenztabelle
- `body`
  - eigentlicher Textbausteininhalt
- `sort_order`
  - steuert Reihenfolge im Settings-UI und spaeter im Quote-Editor
- `is_active`
  - ermoeglicht kontrollierte Deaktivierung ohne Datenverlust

## Kategorien im MVP

Als erste zugelassene Kategorien:

- `intro`
- `scope`
- `closing`
- `legal`

Die Kategorien sollen im MVP noch nicht als eigener Katalog modelliert werden.
Sie bleiben ein begrenzter, serverseitig validierter Stringraum.

## Service-Schnittstelle

Empfohlene neue Datei:

- `server/internal/settings/quote_text_blocks.go`

Empfohlenes Minimal-API:

- `List(ctx) ([]QuoteTextBlock, error)`
- `Upsert(ctx, in QuoteTextBlock) error`
- `Delete(ctx, id string) error`

Empfohlenes Go-Modell:

```go
type QuoteTextBlock struct {
    ID        string `json:"id"`
    Code      string `json:"code"`
    Name      string `json:"name"`
    Category  string `json:"category"`
    Body      string `json:"body"`
    SortOrder int    `json:"sort_order"`
    IsActive  bool   `json:"is_active"`
}
```

## Validierungsregeln

### `Upsert`

- `code` erforderlich, getrimmt, normalisiert
- `name` erforderlich
- `category` erforderlich und auf erlaubte Werte begrenzt
- `body` erforderlich
- `sort_order` frei numerisch
- `is_active` standardmaessig `true`

### `Delete`

Im ersten MVP soll `Delete` hart loeschen duerfen.

Begruendung:

- Es gibt noch keine Referenzen aus Quotes
- dadurch entsteht noch kein historischer Integritaetskonflikt

Sobald spaeter echte Quote-Referenzen oder Textsnapshot-Mechaniken existieren,
kann `Delete` auf Soft-Delete oder Referenzschutz angehoben werden.

## HTTP-Pfad

Empfohlene neue Route:

- `GET /api/v1/settings/quote-text-blocks`
- `POST /api/v1/settings/quote-text-blocks`
- `DELETE /api/v1/settings/quote-text-blocks/{id}`

Berechtigung:

- `settings.manage`

Verhalten:

- `GET` liefert vollstaendige Liste
- `POST` arbeitet im Stil der bestehenden Settings-Kataloge als Upsert
- `DELETE` loescht nach `id`

## Seeds und Migration

Fuer den ersten technischen Schritt ist eine leere Tabelle ausreichend.

Bewusst **kein** Default-Seed im ersten Wurf:

- noch keine stabile Fachsprache fuer Standardtexte beschlossen
- vermeidet vorschnelle Systemtexte, die spaeter migriert werden muessen

Falls spaeter Startdaten noetig werden, sollen sie gemaess
`reference_data_seed_strategy.md` als bewusste Default-Startdaten und nicht als
stille Runtime-Erzeugung eingefuehrt werden.

## Bewusste MVP-Grenzen

Nicht Teil dieses Modells:

- `quote_template_sets`
- Reihenfolge mehrerer Bausteine innerhalb eines Vorlagensatzes
- Platzhalter wie `{project_name}` oder `{contact_name}`
- Markdown-/HTML-Renderer
- Revisionshistorie fuer Bausteine
- Freigabe- oder Mandantenlogik

## Naechster technischer Schritt

Der naechste sinnvolle Schritt ist jetzt kein Editor-Umbau, sondern zuerst die
erste technische Backend-Stufe:

1. Migration fuer `quote_text_blocks`
2. Settings-Service `quote_text_blocks.go`
3. HTTP-Route unter `/api/v1/settings/quote-text-blocks`
4. kleine Integrationstests fuer `List/Upsert/Delete`

Erst danach sollte die minimale Client-Anbindung in Settings und Quote-Editor
folgen.
