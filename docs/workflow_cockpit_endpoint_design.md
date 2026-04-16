# Workflow-Cockpit Endpoint-Design

## Ziel

Dieses Dokument begrenzt den ersten technischen Einstieg fuer das
 kommerzielle Workflow-Cockpit auf genau eine kleine Read-only-Route.

## Erster Endpoint

- `GET /api/v1/workflow/commercial`

Der Endpoint liefert ausschliesslich offene Folgeaktionen zwischen Angebot,
 Auftrag und Rechnung. Er ist keine allgemeine Belegsuche und keine Timeline.

## Scope des MVP

Der MVP soll nur diese Faelle ausliefern:

- `quote_sent_pending`
- `quote_accepted_pending_followup`
- `sales_order_pending_invoice`
- `sales_order_partially_invoiced`

Nicht Teil des MVP:

- abgeschlossene Ketten ohne offene Aktion
- rein informative Rechnungslisten
- Revisionsfamilien-Navigation
- Workflow-Events oder Statushistorie
- Dashboard-Kennzahlen ueber Zeitraeume

## Response-Form

Empfohlenes Antwortformat:

```json
{
  "items": [
    {
      "kind": "quote_accepted_pending_followup",
      "stage": "quote",
      "priority": "high",
      "project_id": "...",
      "project_name": "...",
      "contact_id": "...",
      "contact_name": "...",
      "quote_id": "...",
      "quote_number": "AN-2026-0012",
      "sales_order_id": "",
      "sales_order_number": "",
      "invoice_id": "",
      "invoice_number": "",
      "document_date": "2026-04-03T00:00:00Z",
      "status": "accepted",
      "gross_total": 12500,
      "open_gross_total": 12500,
      "next_action_label": "Folgebeleg erzeugen"
    }
  ]
}
```

## Query-Parameter

Zulaessige optionale Filter im MVP:

- `project_id`
- `contact_id`
- `kind`

Bewusst noch nicht vorgesehen:

- Sortierungsparameter
- freie Statusfilter
- Paging
- kombinierte Suchsyntax

## Ableitungslogik

### 1. `quote_sent_pending`

Quelle:

- `quotes.status = sent`
- `superseded_by_quote_id IS NULL`
- `linked_sales_order_id IS NULL`
- `linked_invoice_out_id IS NULL`

### 2. `quote_accepted_pending_followup`

Quelle:

- `quotes.status = accepted`
- `superseded_by_quote_id IS NULL`
- `linked_sales_order_id IS NULL`
- `linked_invoice_out_id IS NULL`

### 3. `sales_order_pending_invoice`

Quelle:

- aktueller Auftrag ohne ersichtliche Rechnung
- offene Fakturierungssumme entspricht noch dem Auftragsvolumen

### 4. `sales_order_partially_invoiced`

Quelle:

- aktueller Auftrag mit mindestens einer Rechnung
- offener Restbetrag groesser `0`

## Priorisierung

Serverseitige Priorisierung im MVP:

- `high`
  - `quote_accepted_pending_followup`
  - `sales_order_pending_invoice`
- `normal`
  - `quote_sent_pending`
  - `sales_order_partially_invoiced`

Diese Priorisierung ist bewusst statisch und spaeter austauschbar.

## Technische Richtung

Der erste Implementierungsschritt sollte in der HTTP-Schicht bleiben, analog zu
 den bereits eingefuehrten kommerziellen Kontextaggregationen fuer Kontakt und
 Projekt.

Begruendung:

- schneller Einstieg ohne neue persistente Projektion
- vorhandene Leseservices fuer Quotes und Sales Orders koennen direkt genutzt
  werden
- Faktura-Logik fuer offene oder teilweise fakturierte Auftraege kann zunaechst
  als kleine spezialisierte Read-only-Ableitung in derselben Schicht bleiben

## Kleinstes sinnvolles Testset

Der MVP sollte mindestens diese Faelle absichern:

1. `sent` Quote ohne Folgebeleg erscheint als `quote_sent_pending`
2. `accepted` Quote ohne Folgebeleg erscheint als
   `quote_accepted_pending_followup`
3. Auftrag ohne Rechnung erscheint als `sales_order_pending_invoice`
4. Auftrag mit Teilrechnung erscheint als `sales_order_partially_invoiced`
5. Quote mit `superseded_by_quote_id` erscheint nicht
6. Quote oder Auftrag mit vollstaendiger Folgebelegkette erscheint nicht
