# Angebotsvorlagen und Textbausteine: Ist-Stand und Einstiegspunkt

## Ziel

Dieses Dokument inventarisiert den aktuellen Stand fuer wiederverwendbare
Angebotsinhalte und legt den kleinsten sinnvollen operativen Einstiegspunkt
fest.

## Ist-Stand im Repo

### 1. PDF-Templates sind vorhanden, aber nur fuer Layout

Vorhanden sind bereits konfigurierbare PDF-Templates fuer Angebots-,
Auftrags- und Rechnungsdokumente.

Relevante Bausteine:

- `server/internal/migrate/migrations/006_pdf_templates.sql`
- `server/internal/migrate/migrations/031_pdf_templates_quote.sql`
- `server/internal/settings/pdf.go`
- `server/internal/pdfgen/renderer.go`
- `client/lib/pages/settings_page.dart`

Diese Templates decken heute vor allem:

- Kopf- und Fusszeilen
- Seitenabstaende
- Farben
- Logo- und Hintergrundgrafiken

Nicht abgedeckt sind heute:

- fachliche Angebotsvorlagen
- wiederverwendbare Standardtexte fuer Angebotsinhalt
- Textmodule pro Gewerk, Situation oder Angebotstyp
- strukturierte Vorbelegung des Quote-Editors

### 2. Quote-Erfassung ist heute voll manuell

Der Quote-Editor in `client/lib/pages/quotes_page.dart` arbeitet aktuell
manuell mit:

- `project_id`
- `contact_id`
- `currency`
- freiem Feld `note`
- frei gepflegten Positionen

Es gibt derzeit keine:

- Auswahl einer Angebotsvorlage
- Auswahl eines Textbausteins
- Vorbelegung von Einleitung, Leistungsbeschreibung oder Schlusstext
- Katalogverwaltung fuer Standardformulierungen

### 3. Backend kennt heute kein fachliches Vorlagenmodell

In `server/internal/quotes/service.go` besteht das fachliche Angebotsmodell
aktuell aus Kopf- und Positionsdaten:

- `project_id`
- `contact_id`
- `quote_date`
- `valid_until`
- `currency`
- `note`
- `items`

Es existiert kein eigenes Datenmodell fuer:

- Angebotsvorlagen
- Textbausteine
- Textblock-Kategorien
- Vorbelegungsregeln
- Platzhalterersetzung innerhalb fachlicher Texte

## Fachliche Luecke

Die aktuelle Luecke liegt nicht im PDF-Layout, sondern in der
Inhaltserstellung vor der PDF-Erzeugung.

Heute muessen Angebotsinhalte immer wieder neu formuliert oder aus externen
Vorlagen kopiert werden. Dadurch fehlen:

- Wiederverwendbarkeit
- konsistente Standardsprache
- schnellere Angebotserstellung
- sauberer spaeterer Einstieg fuer GAEB- und KI-generierte Inhalte

## Kleinster sinnvoller Einstiegspunkt

Der kleinste operative Einstiegspunkt ist bewusst **nicht** ein komplettes
Vorlagensystem fuer ganze Angebotsdokumente.

Der erste sinnvolle MVP ist:

- ein administrierbarer Katalog fuer Angebots-Textbausteine
- mit kleiner fachlicher Kategorisierung
- und rein manueller Uebernahme in den Quote-Editor

## Empfohlenes MVP-Zielbild

### 1. Einheit: Textbaustein statt Vollvorlage

Die erste technische Einheit soll ein **Textbaustein** sein, nicht eine
komplette Angebotsvorlage.

Vorgesehenes Minimalfeldset:

- `id`
- `code`
- `name`
- `category`
- `body`
- `sort_order`
- `is_active`

### 2. MVP-Kategorien

Als kleinste sinnvolle Kategorien:

- `intro`
- `scope`
- `closing`
- `legal`

Damit lassen sich spaeter Einleitung, Leistungsbeschreibung, Schlusstext und
optionale Standardklauseln getrennt weiterentwickeln.

### 3. Erster Nutzerfluss

Der erste Nutzerfluss soll bewusst klein bleiben:

1. Textbaustein in den Einstellungen pflegen
2. Im Quote-Editor einen oder mehrere Bausteine auswaehlen
3. Inhalt in das bestehende Feld `note` uebernehmen
4. Danach weiter normal manuell bearbeiten

Bewusst noch nicht Teil des MVP:

- eigene neue Quote-Textstruktur mit mehreren Textsektionen
- automatische Platzhalterersetzung
- kombinierte Vollvorlagen
- Versionierung fuer Textbausteine
- PDF-spezifische Sonderlogik

## Warum dieser Einstiegspunkt

Dieser Einstieg hat das beste Signal-Risiko-Verhaeltnis:

- klein genug fuer einen schnellen Nutzwert
- anschlussfaehig an bestehende Settings-Muster
- spaeter erweiterbar zu Vorlagenbibliotheken
- gute Vorstufe fuer GAEB- und KI-generierte Textvorschlaege

Er zwingt noch keinen grossen Umbau des Quote-Datenmodells.

## Technische Richtung fuer den naechsten Schritt

Der naechste sinnvolle technische Schritt ist noch **kein** Quote-Editor-Umbau,
sondern zuerst die Inventur und Zieldefinition fuer einen administrierbaren
Textbaustein-Katalog.

Empfohlene Reihenfolge:

1. Referenzdaten- und Settings-Muster fuer diesen Katalog festlegen
2. minimale Tabelle und Settings-API fuer Quote-Textbausteine einfuehren
3. erst danach kleine Client-Anbindung im Quote-Editor

## Abgrenzung

Nicht Teil dieses Blocks:

- GAEB-Parsing
- KI-Textableitung
- Dokumentvarianten pro Kunde
- Bedingungslogik je Land oder Vertragsart
- komplexe Placeholder-Engine
- PDF-Renderer-Refactor

Diese Themen bauen spaeter auf dem Textbaustein-Katalog auf, gehoeren aber
nicht in den ersten operativen Einstiegspunkt.
