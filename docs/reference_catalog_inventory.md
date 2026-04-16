# Referenzdaten-Inventur fuer Einheiten, Materialgruppen und Statuskataloge

## Kontext
Subtask `2.2.2.2` zielt darauf, Referenzdaten administrierbar zu machen. Fuer den ersten Micro-Subtask wird der Ist-Zustand zwischen Schema, Backend und Client festgehalten, damit die folgenden Implementierungsschritte nicht auf impliziten Annahmen beruhen.

## Ist-Zustand

### 1. Einheiten
- Persistenz vorhanden: Tabelle `units` in `server/internal/migrate/migrations/014_material_dimensions_and_units.sql`.
- Backend vorhanden: `server/internal/settings/units.go` mit `List`, `Upsert`, `Delete`.
- HTTP vorhanden: `/api/v1/settings/units` in `server/internal/http/v1.go`.
- UI vorhanden: Verwaltung in `client/lib/pages/settings_page.dart`.
- Nutzung vorhanden:
  - Materialien verwenden `einheit`.
  - Einkauf, Angebote, Auftraege, Projektimporte und Lagernahe Flows verwenden Einheitenfelder.
- Bewertung:
  - Einheiten sind bereits administrierbar.
  - Es fehlt eher Konsistenz in der Wiederverwendung derselben Stammdaten in allen Fachmasken als die Grundfunktion selbst.

### 2. Materialgruppen
- Persistenz heute nicht als Stammdatenkatalog, sondern als Freitextfeld `materials.kategorie`.
- Backend nur indirekt:
  - Filterung in `materials.Service.List(...)`.
  - Distinct-Liste per `ListCategories()` in `server/internal/materials/service.go`.
- Keine eigene Settings-API fuer Materialgruppen.
- Keine eigene Tabelle mit Sortierung, Aktiv-Flag, Beschreibung oder Loeschschutz.
- Client nutzt `kategorie` bereits in Materialien und Projektnavigation, aber ohne zentrale Administration.
- Bewertung:
  - Materialgruppen sind aktuell nur faktisch vorhandene Werte aus dem Materialbestand.
  - Hier besteht die groesste Luecke fuer echte Administrierbarkeit.

### 3. Statuskataloge
- Statuslisten sind heute ueberwiegend hart codiert in Domain-Services:
  - Kontakte: `contacts.Statuses()`
  - Kontaktaufgaben: `contacts.TaskStatuses()`
  - Bestellungen: `purchasing.Statuses()`
  - Auftraege: `sales.Statuses()`
- Teilweise werden diese Listen bereits per Read-API an den Client ausgeliefert:
  - `/contacts/statuses`
  - `/purchase-orders/statuses`
  - `/sales-orders/statuses`
- Weitere Statusfelder existieren im Schema ebenfalls als freie Textspalten mit fachlich implizitem Katalog:
  - `projects.status`
  - `quotes.status`
  - `invoices_out.status`
  - `hr_leave_requests.status`
- Keine zentrale Katalogtabelle, keine Settings-Verwaltung, keine Mandanten- oder Aktiv-/Inaktiv-Steuerung.
- Bewertung:
  - Statuskataloge sind aktuell codezentriert statt datengetrieben.
  - Das ist fuer kleine Kernprozesse tragbar, skaliert aber schlecht fuer ERP-weite Konfiguration.

## Gap-Analyse

### Bereits geloest
- Einheiten: technische und UI-seitige Grundverwaltung ist vorhanden.

### Teilweise geloest
- Statuskataloge: lesbar, aber nicht administrierbar.
- Materialgruppen: nutzbar, aber nicht als echter Stammdatenkatalog verwaltet.

### Ungeloest
- Zentrale Referenzdatenstrategie fuer:
  - Materialgruppen
  - Statuskataloge pro Domäne
  - gemeinsame Metadaten wie Sortierung, Aktiv-Flag, Systemeintrag, Mandantenfaehigkeit

## Architekturentscheidung fuer die Folgearbeit

### Einheiten
- Nicht neu erfinden.
- Folgefokus: vorhandene Units-Verwaltung spaeter als Referenz fuer weitere Kataloge nutzen.

### Materialgruppen
- Von Freitext auf echten Stammdatenkatalog umstellen.
- Empfohlene Zielstruktur:
  - eigene Tabelle, z. B. `material_groups`
  - Felder mindestens: `code`, `name`, `description`, `sort_order`, `is_active`, `created_at`
- Materialien sollten mittelfristig auf `material_group_code` oder aehnliches referenzieren.
- Fuer die Migration wird vorerst ein Parallelbetrieb sinnvoll sein:
  - bestehendes `kategorie` nicht sofort hart entfernen
  - zuerst Katalog aufbauen und bestehende Werte ueberfuehren

### Statuskataloge
- Nicht alle Statusfelder gleichzeitig datengetrieben machen.
- Sinnvolle Einfuehrungsreihenfolge:
  1. Katalog-Leseschicht vereinheitlichen
  2. Settings-Administration fuer einzelne Domänen einfuehren
  3. Validierung in Domain-Services schrittweise von Hardcode auf Katalogdaten umstellen
- Empfohlene Zielstruktur:
  - zentrale Tabelle, z. B. `status_catalog_entries`
  - Schluessel: `catalog`, `code`
  - weitere Felder: `label`, `sort_order`, `is_active`, `is_terminal`, `color`, `system_locked`

## Empfohlene Micro-Subtasks fuer `2.2.2.2`
1. Ist-Zustand und Zielmodell dokumentieren.
2. Materialgruppen als eigenen administrierbaren Katalog modellieren.
3. Read-API fuer Referenzdatenkataloge vereinheitlichen.
4. Statuskataloge fuer die ersten Domänen aus Hardcode in Konfiguration ueberfuehren.
5. Settings-UI fuer Materialgruppen und ausgewaehlte Statuskataloge erweitern.

## Empfehlung fuer den naechsten Implementierungsschritt
- Zuerst Materialgruppen angreifen.
- Grund:
  - dort ist die Luecke am klarsten,
  - die Domänenkopplung ist geringer als bei Statusmaschinen,
  - und es gibt bereits reale Bestandsdaten (`materials.kategorie`), die in einen Katalog ueberfuehrt werden koennen.
