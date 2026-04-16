# Workflow-Cockpit Datenmodell

## Ziel

Dieses Dokument definiert das kleinste sinnvolle Read-only-Datenmodell fuer ein
 operatives Workflow-Cockpit zwischen Angebot, Auftrag und Ausgangsrechnung.
Der Fokus liegt bewusst nicht auf vollstaendiger Historie, sondern auf offenen
 Folgeaktionen.

## Leitprinzip

Das MVP soll keine neue Fachpersistenz einfuehren. Es soll offene
 Workflow-Situationen aus den bereits vorhandenen Belegtabellen und
 Referenzfeldern ableiten.

## Kleinste sinnvolle Einheit

Die kleinste operative Einheit ist nicht ein einzelner Beleg, sondern ein
 `WorkflowItem`, das eine ausstehende Folgeaktion repraesentiert.

Beispiele:

- Angebot wurde versendet, aber noch nicht entschieden.
- Angebot wurde angenommen, aber noch nicht in Auftrag oder Rechnung
  ueberfuehrt.
- Auftrag existiert, aber noch nicht fakturiert.
- Auftrag ist nur teilweise fakturiert und benoetigt weitere Folgeaktion.

## Minimales WorkflowItem

Empfohlenes Read-only-MVP-Feldset:

- `kind`
  - Werte im MVP:
    - `quote_sent_pending`
    - `quote_accepted_pending_followup`
    - `sales_order_pending_invoice`
    - `sales_order_partially_invoiced`
- `stage`
  - `quote`
  - `sales_order`
- `priority`
  - einfache serverseitige Priorisierung:
    - `high`
    - `normal`
- `project_id`
- `project_name`
- `contact_id`
- `contact_name`
- `quote_id`
- `quote_number`
- `sales_order_id`
- `sales_order_number`
- `invoice_id`
  - im MVP optional, nur wenn bereits eine Teil- oder Direktrechnung als
    Kontext angezeigt werden soll
- `invoice_number`
  - ebenfalls optional
- `document_date`
  - Belegdatum des ausloesenden Dokuments
- `status`
  - Originalstatus des ausloesenden Dokuments
- `gross_total`
  - Bruttogesamt des ausloesenden Dokuments
- `open_gross_total`
  - nur fuer fakturarelevante Faelle
- `next_action_label`
  - z. B. `Entscheidung ausstehend`, `Folgebeleg erzeugen`,
    `Restbetrag fakturieren`

## Ableitungsregeln fuer das MVP

### 1. `quote_sent_pending`

Quelle:

- `quotes.status = sent`
- keine Folgebelege vorhanden

Zweck:

- zeigt Angebote, die noch auf Entscheidung oder aktive Nachverfolgung warten

### 2. `quote_accepted_pending_followup`

Quelle:

- `quotes.status = accepted`
- `linked_sales_order_id IS NULL`
- `linked_invoice_out_id IS NULL`

Zweck:

- zeigt angenommene Angebote, fuer die noch kein Folgebeleg erzeugt wurde

### 3. `sales_order_pending_invoice`

Quelle:

- Auftrag existiert
- keine verknuepfte oder aggregierte Rechnung vorhanden

Zweck:

- zeigt Auftraege, die kaufmaennisch noch nicht fakturiert wurden

### 4. `sales_order_partially_invoiced`

Quelle:

- Auftrag existiert
- mindestens eine Rechnung vorhanden
- offener fakturierbarer Restbetrag groesser `0`

Zweck:

- zeigt Auftraege mit weiterem Fakturapotenzial

## Bewusste MVP-Grenzen

Nicht Teil dieses Modells:

- revisionsspezifische Familiennavigation
- mehrstufige Timeline
- mehrdeutige Parallelpfade oder konkurrierende Folgebelege
- frei konfigurierbare Priorisierungsregeln
- eigene Cockpit-Filter-DSL
- globale KPI- und Trendaggregation

## Kleinste Aggregationsroute

Fuer den ersten technischen Einstieg ist eine einzige route ausreichend:

- `GET /api/v1/workflow/commercial`

Empfohlene Query-Parameter im MVP:

- `project_id` optional
- `contact_id` optional
- `kind` optional

Die Route soll zunaechst nur eine flache Liste von `WorkflowItem` liefern.
Keine Pagination- oder Gruppierungslogik ist fuer den ersten Schritt zwingend.

## Priorisierte technische Richtung

Der erste Implementierungsschritt sollte serverseitig nur zwei Quellen
 zusammenfassen:

1. `quotes` fuer offene Angebots-Folgeaktionen
2. `sales_orders` plus vorhandene Rechnungsreferenzen fuer Faktura-Faelle

`invoices_out` dient im MVP primär als Nachweis, ob ein Folgebeleg bereits
 existiert oder ein Restbetrag offen bleibt. Die Rechnung selbst ist noch nicht
 der operative Primärtreiber der Queue.
