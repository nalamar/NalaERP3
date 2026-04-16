# Referenzdaten: Seed- und Migrationsstrategie

## Ziel

Referenzdaten sollen im Repo kuenftig eindeutig in drei Klassen getrennt werden:

1. `System-Referenzdaten`
   Daten, ohne die das System fachlich oder technisch nicht korrekt startet.
   Beispiele: Rollen, Permissions, systemische Nummernkreis-Entities, Default-Company-Row.

2. `Default-Startdaten`
   Sinnvolle Anfangsbelegung fuer neue Installationen, die aber fachlich spaeter durch den Kunden gepflegt oder ersetzt werden kann.
   Beispiele: Einheiten, erste Materialgruppen, optionale Standard-Layouts.

3. `Mandanten-/Betriebsdaten`
   Daten, die aus dem konkreten Kundenbetrieb stammen oder sich aus Bestandsdaten ableiten.
   Beispiele: aus `materials.kategorie` uebernommene Materialgruppen, individuelle Niederlassungen, kundenspezifische Statusvarianten.

Die aktuelle Codebasis mischt diese Klassen teilweise in denselben SQL-Migrationen. Das ist fuer die fruehe Projektphase akzeptabel, skaliert aber schlecht, sobald mehr administrierbare Kataloge hinzukommen.

## Ist-Zustand

Aktuell werden Referenzdaten an mehreren Stellen eingebracht:

- per idempotenter SQL-Migration direkt im Schema-Layer
  Beispiele:
  - `014_material_dimensions_and_units.sql` seeded `units`
  - `017_auth.sql` seeded `roles`, `permissions`, `role_permissions`
  - `005_numbering.sql`, `017_accounting_basics.sql`, `032_quotes.sql`, `034_sales_orders.sql` seeded `number_sequences`
  - `025_company_profile.sql`, `027_company_localization.sql`, `028_company_branding.sql` legen Default-Singletons an

- per datenabhaengiger Uebernahme aus Bestandsdaten
  Beispiel:
  - `039_material_groups.sql` uebernimmt `DISTINCT TRIM(materials.kategorie)` in `material_groups`

- per Runtime-Upsert in Services
  Beispiele:
  - `settings.NumberingService.UpdatePattern()` legt fehlende Entities on demand an
  - Settings-Services pflegen katalogartige Daten ueber APIs nach

Das funktioniert, fuehrt aber zu drei Risiken:

1. Migrationsdateien werden implizit zu Seed-Containern.
2. On-demand-Upserts koennen stillschweigend neue Referenzdaten erzeugen.
3. Es ist nicht klar, welche Daten bei Neuinstallation, Update oder Bestandsmigration verpflichtend sein muessen.

## Zielprinzipien

### 1. Migrationen duerfen weiterhin idempotente System-Seeds enthalten

Zulaessig in SQL-Migrationen sind nur Daten, die fuer jede Installation gleich und systemisch notwendig sind:

- Rollen und Permissions
- technische Singleton-Zeilen
- verpflichtende Nummernkreis-Initialwerte fuer fest definierte Entities
- Default-Tabellenzeilen, ohne die APIs nicht sinnvoll arbeiten

Regel:
Diese Seeds muessen deterministisch, idempotent und repo-versioniert sein.

### 2. Default-Startdaten bleiben zunaechst in Migrationen, aber klar markiert

Solange es noch keinen separaten Bootstrap-Mechanismus gibt, duerfen Default-Startdaten weiter per Migration kommen, wenn sie:

- klein bleiben
- fuer neue Installationen nuetzlich sind
- per Settings-UI spaeter gepflegt werden koennen
- nicht kundenspezifisch sind

Beispiele:

- `units`
- spaeter ggf. erste Materialgruppen wie `profil`, `blech`, `beschlag`

Regel:
Diese Seeds muessen in SQL-Dateien explizit als `Default-Startdaten` kommentiert sein und duerfen keine produktiven Kundendaten voraussetzen.

### 3. Bestandsableitungen sind nur als Uebergangs-Migration erlaubt

Datenuebernahmen aus vorhandenen Business-Tabellen bleiben erlaubt, aber nur fuer einmalige Migrationsuebergaenge.

Beispiel:

- `039_material_groups.sql` darf bestehende `materials.kategorie`-Werte in den neuen Katalog ueberfuehren

Regel:
Solche Migrationen sind kein Seed-Mechanismus fuer die Zukunft, sondern einmalige Datenkonvertierung.

### 4. Neue administrierbare Kataloge werden nicht mehr per Runtime-Erfindung erzeugt

Sobald ein Referenzdatenbereich administrierbar ist, soll Runtime-Code fehlende Werte nicht mehr stillschweigend anlegen.

Konsequenzen:

- `NumberingService.Next()` sollte spaeter keine unbekannten Entities mehr automatisch mit einem Fallback-Pattern anlegen
- neue Status- oder Katalogwerte sollen ueber definierte Seeds, Admin-UI oder explizite Datenmigration entstehen

## Konkrete Strategie pro Referenzdatenklasse

### A. System-Referenzdaten

Pfad:

- bleiben in `server/internal/migrate/migrations/*.sql`

Regeln:

- immer `INSERT ... ON CONFLICT DO NOTHING` oder funktional gleichwertig
- nie aus Kundendaten ableiten
- nie durch UI loeschbar machen
- optional durch `is_system`-Flag technisch absichern

Kandidaten:

- `roles`
- `permissions`
- feste `number_sequences` fuer definierte Belegarten

### B. Default-Startdaten

Pfad:

- kurzfristig weiter in SQL-Migrationen
- mittelfristig in separaten Bootstrap-Schritt verschieben

Regeln:

- nur fuer neue Installationen gedacht
- keine Abhaengigkeit von bestehenden Bewegungsdaten
- im UI aenderbar, aber nicht zwingend loeschbar
- spaeter bevorzugt ueber dedizierte Seed-Dateien oder einen Bootstrap-Runner verwalten

Kandidaten:

- `units`
- spaetere Default-Materialgruppen
- evtl. Standard-Statuskataloge, falls diese bewusst kundenseitig anpassbar werden

### C. Bestandsmigrationen

Pfad:

- einmalige SQL-Migration

Regeln:

- transformieren Altbestand in neuen Katalog
- muessen idempotent sein
- muessen nach dem initialen Uebergang nicht erneut erweitert werden

Kandidaten:

- `materials.kategorie` -> `material_groups`

## Praktische Regeln fuer die naechsten Referenzdatenarbeiten

### Materialgruppen

Gilt ab sofort:

- Tabelle und Settings-UI sind der fuehrende Katalog
- `039_material_groups.sql` bleibt die einmalige Uebernahme aus Altbestand
- neue fachliche Default-Gruppen sollten, falls noetig, in einer eigenen spaeteren Migration als klar kommentierte `Default-Startdaten` kommen
- keine weitere implizite Generierung aus Freitext ausserhalb definierter Migrationspfade

### Einheiten

Gilt vorerst:

- bestehender Seed in `014_material_dimensions_and_units.sql` bleibt akzeptiert
- kuenftige Erweiterungen nur mit klarer Kommentierung als Default-Startdaten
- keine kundenspezifischen Einheiten in Repo-Migrationen

### Statuskataloge

Empfehlung fuer den naechsten Ausbau:

- zunaechst unterscheiden zwischen
  - `harte Prozessstatus` mit Business-Logik-Kopplung
  - `weiche Auswahlkataloge` fuer Anzeige oder Klassifikation

Nur die zweite Klasse eignet sich kurzfristig fuer echte Administrierbarkeit.
Harte Prozessstatus sollten vorerst codebasiert bleiben oder spaeter als geschuetzte Systemkataloge modelliert werden.

## Zielbild fuer einen spaeteren sauberen Seed-Layer

Wenn weitere Kataloge hinzukommen, sollte ein eigener Bootstrap-Layer eingefuehrt werden:

- `server/internal/bootstrap/system_reference_data.go`
- Aufruf nach `migrate.Run(...)`
- strikt idempotent
- getrennt nach:
  - `EnsureSystemReferenceData()`
  - `EnsureDefaultCatalogData()`

Vorteile:

- Schema-Migration und Default-Belegung werden konzeptionell getrennt
- Tests koennen Seeds gezielter an- oder ausschalten
- Referenzdatenregeln werden in Go statt in verstreuten SQL-Dateien zentral lesbar

## Entscheidung fuer den aktuellen Projektstand

Fuer den aktuellen Repo-Stand gilt folgende operative Linie:

1. Systemische Referenzdaten bleiben weiter in SQL-Migrationen erlaubt.
2. Default-Startdaten bleiben kurzfristig ebenfalls in SQL erlaubt, muessen aber klar als solche erkennbar sein.
3. Bestandsableitungen wie bei Materialgruppen sind nur als einmalige Migrationsuebergaenge erlaubt.
4. Neue administrierbare Kataloge sollen nicht mehr stillschweigend im Runtime-Code entstehen.
5. Sobald der zweite oder dritte groessere Katalog nach Materialgruppen folgt, sollte ein separater Bootstrap-Layer eingefuehrt werden.

## Konkrete Folgepunkte

1. `NumberingService.Next()` spaeter haerten, damit unbekannte Entities nicht mehr implizit entstehen.
2. Bei kuenftigen Statuskatalogen vorab trennen, welche Status systemisch und welche administrierbar sein sollen.
3. Neue Referenzdaten-Migrationen mit Kommentarkopf versehen:
   - `System-Referenzdaten`
   - `Default-Startdaten`
   - `Bestandsmigration`
4. Spaeter einen dedizierten Bootstrap-Layer fuer Default-Kataloge einfuehren, bevor weitere groessere Referenzdatenbereiche administrierbar gemacht werden.
