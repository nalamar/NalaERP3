# Contact and Project Commercial Context API

## Ziel
Der erste Umsetzungsschritt fuer die kommerzielle Kontextsicht soll keine neue Schreiblogik einfuehren, sondern eine kleine Leseschicht fuer bereits vorhandene Belege bereitstellen.

## Ist-Stand

### Kontaktkontext
- `client/lib/pages/contact_detail_screen.dart` laedt aktuell:
  - Kontaktstammdaten
  - Adressen
  - Ansprechpartner
  - Notizen
  - Aufgaben
  - Dokumente
  - Activity-Feed
- Es werden aktuell **keine** Angebote, Auftraege oder Rechnungen fuer den Kontakt geladen.
- Die benoetigten Daten sind fachlich bereits vorhanden:
  - `quotes` haben `contact_id`
  - `sales_orders` haben `contact_id`
  - `invoices_out` haben `contact_id`

### Projektkontext
- `client/lib/pages/projects_page.dart` laedt in `ProjectDetailPage._load()` aktuell:
  - Projektlose/Phasen
  - `listQuotes(projectId: ...)`
  - `listSalesOrders(projectId: ...)`
- Daraus wird nur ein kaufmaennischer Snapshot erzeugt.
- Rechnungen werden im Projektkontext aktuell nicht direkt aggregiert.
- Folgebelegbeziehungen sind aber bereits vorhanden:
  - Angebot -> Auftrag via `linked_sales_order_id`
  - Angebot -> Rechnung via `linked_invoice_out_id`
  - Auftrag -> Rechnung via `linked_invoice_out_id`
  - Rechnung -> Herkunft via `source_quote_id`, `source_sales_order_id`

## Ziel fuer Micro-Subtask 1
- Noch **keine** Implementierung einer Aggregationsroute
- Zuerst eine minimale Ziel-API definieren, die mit den vorhandenen Services erreichbar ist
- Die API soll nur lesen und vorhandene Informationen konsistent zusammenfassen

## Minimaler API-Zuschnitt

### Kontaktkontext
`GET /api/v1/contacts/{id}/commercial-context`

Antwort:

```json
{
  "contact_id": "string",
  "quotes": [],
  "sales_orders": [],
  "invoices_out": [],
  "stats": {
    "quote_count": 0,
    "sales_order_count": 0,
    "invoice_count": 0,
    "open_invoice_count": 0,
    "quote_gross_total": 0,
    "sales_order_gross_total": 0,
    "invoice_gross_total": 0,
    "invoice_open_total": 0
  }
}
```

### Projektkontext
`GET /api/v1/projects/{id}/commercial-context`

Antwort:

```json
{
  "project_id": "string",
  "quotes": [],
  "sales_orders": [],
  "invoices_out": [],
  "stats": {
    "quote_count": 0,
    "sales_order_count": 0,
    "invoice_count": 0,
    "open_invoice_count": 0,
    "quote_gross_total": 0,
    "sales_order_gross_total": 0,
    "invoice_gross_total": 0,
    "invoice_open_total": 0
  }
}
```

## Warum keine generische Universalroute
- Kontakt- und Projektkontext sind zwar aehnlich, aber nicht identisch
- Projektrechnungen muessen zunaechst aus Quote- und Auftragbeziehungen abgeleitet werden, waehrend Kontaktrechnungen direkt ueber `contact_id` filterbar sind
- Zwei kleine zielgerichtete Routen sind fuer das MVP klarer als eine abstrakte Kontext-Entitaet

## Empfohlene Wiederverwendung vorhandener Services

### Kontakt
- `quotes.Service.List(... ContactID: id ...)`
- `sales.Service.List(... ContactID: id ...)`
- `ARService.List(... ContactID: id ...)`

### Projekt
- `quotes.Service.List(... ProjectID: id ...)`
- `sales.Service.List(... ProjectID: id ...)`
- Rechnungen zunaechst aus den bereits geladenen Quotes und Sales Orders ableiten:
  - direkte Rechnungen aus `quotes.linked_invoice_out_id`
  - Auftragsrechnungen aus `sales_orders.linked_invoice_out_id`
  - spaeter optional echte Projekt-Rechnungsaggregation

## Bewusste MVP-Grenzen
- Keine neuen Datenbanktabellen
- Kein neuer Materialized View
- Keine Timeline-Aggregation in diesem Schritt
- Keine neuen Schreiboperationen
- Keine direkte GAEB- oder KI-Logik

## Konsequenz fuer den naechsten technischen Schritt
Der naechste Implementierungsschritt sollte zuerst den Kontaktkontext bedienen. Er ist technisch einfacher, weil Quotes, Auftraege und Rechnungen direkt ueber `contact_id` filterbar sind. Der Projektkontext kann danach auf dieselbe Leselogik aufbauen und nur bei Rechnungen einen kleinen Ableitungspfad benoetigen.
