# Epic 3 Commercial Workflow Inventory

## Ziel
Diese Inventur markiert den tatsaechlichen Startpunkt fuer Epic 3. Sie trennt bereits vorhandene Vertriebs- und Folgebeleg-Funktion von echten Ausbauluecken.

## Bereits vorhanden

### Angebote
- Backend-Service in `server/internal/quotes/service.go`
- API-Routen in `server/internal/http/v1.go`
  - `GET/POST /api/v1/quotes`
  - `GET/PATCH /api/v1/quotes/{id}`
  - `POST /api/v1/quotes/{id}/status`
  - `POST /api/v1/quotes/{id}/accept`
  - `POST /api/v1/quotes/{id}/convert-to-invoice`
  - `POST /api/v1/quotes/{id}/convert-to-sales-order`
  - `GET /api/v1/quotes/{id}/pdf`
- Client-UI in `client/lib/pages/quotes_page.dart`
- Vorhandene Fachpunkte
  - Angebotsnummer ueber zentralen Nummernkreis
  - Kontakt- und Projektbezug
  - Positionen, Summen und Steuercodes
  - Statuswechsel
  - Annahme mit optionalem Projektstatus-Update
  - Folgebeleg-Erzeugung

### Auftraege
- API-Routen in `server/internal/http/v1.go` unter `/api/v1/sales-orders`
- Client-UI in `client/lib/pages/sales_orders_page.dart`
- Vorhandene Fachpunkte
  - Erzeugung aus Angebot
  - Statusmodell
  - Auftragspositionen
  - PDF-Ausgabe
  - Sicht auf Quellangebot
  - Teilfaktura und Mehrfachrechnungen aus einem Auftrag

### Ausgangsrechnungen
- API-Routen in `server/internal/http/v1.go` unter `/api/v1/invoices-out`
- Vorhandene Fachpunkte
  - Liste, Entwurf und Detail
  - Buchen mit finaler Rechnungsnummer
  - Zahlungen
  - PDF
  - Herkunftsbezug von Angebot und Auftrag

### Uebergreifende kommerzielle Navigation
- Client-Helfer in
  - `client/lib/commercial_navigation.dart`
  - `client/lib/commercial_context.dart`
  - `client/lib/commercial_destinations.dart`
- Projektseite laedt bereits Quotes und Sales Orders
- Settings- und Dashboard-Pfade verlinken auf kommerzielle Bereiche

### Testbasis
- Integrationstests fuer Quote-/Auftrag-/Rechnungsfluesse in `server/internal/http/quotes_integration_test.go`
- Tests decken Folgebelegbeziehungen und Teilfaktura bereits in relevanten Pfaden ab

## Bereits vorhandene Architekturbausteine
- Nummernkreise fuer `quote`, `sales_order` und `invoice_out`
- PDF-Templates fuer Angebot, Auftrag und Rechnung
- Kontakt-, Projekt- und Belegverknuepfungen sind technisch bereits vorhanden

## Klar erkennbare Luecken fuer Epic 3

### 1. Gap-Matrix statt Basis-CRUD
Der Kernengpass ist nicht fehlendes CRUD, sondern fehlende Priorisierung der verbleibenden kommerziellen Luecken.

### 2. Angebotsrevisionen
- Es gibt aktuell kein explizites Revisionsmodell fuer Angebote
- Noch offen ist, ob Revisionen als Suffix, Snapshots oder eigene Entitaet modelliert werden

### 3. End-to-end Workflow-Cockpit
- Einzelne Folgebelegbeziehungen sind sichtbar
- Ein durchgaengiges Workflow-Bild Quote → Auftrag → Rechnung ueber Kontakt/Projekt hinweg ist aber noch nicht als eigener Fokus umgesetzt

### 4. Kontakt- und Projektsicht auf Belege
- Technische Beleglisten existieren an Einzelstellen
- Eine gezielte kommerzielle Kontextsicht pro Kontakt ist noch kein klar abgeschlossener Strang

### 5. GAEB-/KI-Angebotserzeugung
- Im Repo gibt es aktuell keinen belegten GAEB-Importpfad
- Im Repo gibt es aktuell keine KI-gestuetzte Angebotsableitung aus Leistungsverzeichnissen
- Das ist der spaetere fachliche Schluesselpfad fuer das ERP-Zielbild

## Konsequenz fuer die weitere Arbeit
Epic 3 sollte mit einer priorisierten Gap-Matrix fortgesetzt werden, nicht mit erneutem Validieren bereits vorhandener Flows.

Empfohlene Reihenfolge:
1. Bestehenden kommerziellen Workflow gegen Zielbild mappen
2. Ersten echten Ausbaukandidaten priorisieren
3. Erst danach gezielt in Implementierung gehen
