# Workflow-Cockpit Inventur

## Ziel

Diese Inventur beschreibt den Ist-Stand fuer einen operativen Workflow zwischen
Angebot, Auftrag und Ausgangsrechnung. Sie grenzt bewusst ab, was bereits im
System sichtbar ist und was fuer ein echtes Workflow-Cockpit noch fehlt.

## Bereits vorhandene Workflow-Bausteine

### Angebotskontext

- `client/lib/pages/quotes_page.dart` zeigt bereits Folgebelegbezug pro Angebot.
- Aktionen fuer `Annehmen`, `In Auftrag umwandeln` und `Direkt fakturieren`
  sind bereits vorhanden.
- Angebotsdetails koennen verlinkte Auftraege und verlinkte Rechnungen laden.
- Das Backend besitzt dafuer bereits stabile Referenzen wie
  `linked_sales_order_id` und `linked_invoice_out_id`.

### Auftragskontext

- `client/lib/pages/sales_orders_page.dart` zeigt bereits Quellangebot,
  verknuepfte Rechnungen und Workflow-Hinweise im Detailbereich.
- Auftraege koennen auf ein Angebotsdokument zurueckverweisen
  (`source_quote_id`).
- Teilfaktura und Rechnungsbezug sind fachlich bereits vorhanden.

### Rechnungskontext

- Ausgangsrechnungen koennen bereits auf Angebot oder Auftrag zurueckverweisen
  (`source_quote_id`, `source_sales_order_id`).
- In der Rechnungs-UI existieren bereits Folgebeleghinweise und
  Quellreferenzen.

### Kontextsicht pro Kontakt und Projekt

- `GET /api/v1/contacts/{id}/commercial-context` liefert Quotes, Auftraege und
  Rechnungen pro Kontakt.
- `GET /api/v1/projects/{id}/commercial-context` liefert dasselbe pro Projekt.
- Kontakt- und Projektdetail zeigen diese Aggregationen bereits read-only an.

## Was trotz dieser Bausteine fehlt

Die vorhandenen Funktionen sind detailzentriert. Sie beantworten bereits:

- "Welche Folgebelege hat dieses konkrete Angebot?"
- "Welche Rechnungen haengen an diesem Auftrag?"
- "Welche Belege gehoeren zu diesem Kontakt oder Projekt?"

Sie beantworten aber noch nicht gut die operative Frage:

- "Welche Belege warten gerade auf den naechsten Schritt?"

Fuer ein echtes Workflow-Cockpit fehlen derzeit vor allem:

- eine einheitliche Queue ueber mehrere Belegarten
- eine normalisierte Stufe im Flow `quote -> sales_order -> invoice_out`
- eine priorisierte Sicht auf offene Folgeaktionen
- ein kleiner operativer Einstiegspunkt ausserhalb einzelner Detailseiten

## Kleinstes sinnvolles MVP

Der kleinste operative Einstiegspunkt ist kein grosses Cockpit mit Timeline,
Kanban oder Drilldown-Logik, sondern eine read-only Workflow-Liste mit klaren
Handlungsfaellen.

Empfohlene erste Queue:

- `Angebot gesendet, aber noch nicht angenommen oder abgelehnt`
- `Angebot angenommen, aber noch kein Auftrag und keine Direktrechnung`
- `Auftrag vorhanden, aber noch keine Rechnung erzeugt`
- `Auftrag vorhanden, aber noch nicht vollstaendig fakturiert`

Bewusst noch nicht Teil des MVP:

- Timeline ueber Statuswechsel
- Revisionsdarstellung im Cockpit
- Navigation zwischen allen Folgebelegen in beide Richtungen
- freie Filter- und Segmentierungslogik
- Dashboard-KPIs ueber Zeitraeume

## Empfohlene technische Richtung

Fuer den ersten operativen Schritt sollte keine neue Fachpersistenz eingefuehrt
werden. Stattdessen ist eine kleine Read-only-Aggregation ueber vorhandene
Belegtabellen und Referenzfelder sinnvoll.

Minimaler Zielpunkt fuer den naechsten Schritt:

- ein normalisiertes Workflow-Item fuer Quote- und Sales-Order-basierte
  Folgeaktionen
- ein einfacher Endpoint fuer offene Workflow-Faelle
- zuerst nur serverseitig priorisierte Standardregeln, keine generische
  Query-Sprache

## Priorisierte Folgerung

Der kleinste sinnvolle Einstiegspunkt fuer `Epic 3 / Feature 3.1 / Task 3.1.3`
ist daher:

1. Ein read-only Workflow-Cockpit fuer offene Folgeaktionen.
2. Start nur mit Quotes und Sales Orders als operative Treiber.
3. Rechnungen zunaechst als Ergebnis- oder Blockierungszustand derselben Queue,
   nicht als eigener vollwertiger Arbeitskorb.
