# Quote Revision Strategy

## Ziel

Angebote sollen kuenftig kontrolliert versionierbar sein, ohne den bereits
vorhandenen Quote-, Sales-Order- und Invoice-Flow zu destabilisieren.
Revisionen sind dabei kein rein kosmetisches Nummern-Suffix, sondern ein
fachlicher Zustand:

- ein Angebot kann mehrere Versionen haben
- genau eine Version ist die aktuelle Arbeitsversion
- Folgebelege duerfen nur aus einer freigegebenen bzw. aktuell gueltigen
  Version entstehen
- alte Versionen bleiben lesbar und nachvollziehbar

## Ist-Stand im Repo

Die vorhandene Angebotsfunktion ist bereits deutlich ausgebaut:

- Quotes haben CRUD, Status, PDF und Konvertierungen
- zulaessige Stati sind aktuell `draft`, `sent`, `accepted`, `rejected`
- `accepted_at` ist vorhanden
- Folgebelege sind bereits verknuepft:
  - `quotes.linked_sales_order_id`
  - `quotes.linked_invoice_out_id`
  - `sales_orders.source_quote_id`
  - `invoices_out.source_quote_id`
- Quote -> Auftrag und Quote -> Rechnung sind bereits produktiv im Flow

Revisionslogik fehlt aber noch vollstaendig:

- keine `revision_of`- oder `root_quote_id`-Beziehung
- keine `revision_no`
- kein Status oder Marker fuer "durch Revision ersetzt"
- keine Historie ueber Versionen desselben Angebots
- keine UI fuer Wechsel zwischen Versionen

## Fachliche Kernentscheidung

Revisionen sollen als neue Quote-Datensaetze modelliert werden, nicht als
Mutation derselben Zeile.

Begruendung:

- bestehende Folgebelege referenzieren heute direkt eine konkrete Quote-ID
- so bleibt die Herkunft von Auftrag und Rechnung revisionssicher
- PDFs und Snapshots koennen pro Version stabil bleiben
- bestehender List-/Detail-Code laesst sich eher erweitern als komplett
  austauschen

## Zielmodell

Minimaler Zielzustand:

- jede Quote gehoert zu einer Revisionsfamilie
- eine Familie hat genau eine Wurzelquote
- jede Quote hat eine laufende Revisionsnummer innerhalb der Familie
- genau eine Quote ist die aktuelle Version
- ersetzte Versionen bleiben sichtbar, aber nicht mehr konvertierbar

Empfohlene neue Felder auf `quotes`:

- `root_quote_id UUID`
  - zeigt auf die erste Quote der Familie
  - bei der ersten Version identisch zur eigenen `id`
- `revision_no integer not null default 1`
- `superseded_by_quote_id UUID null`
  - zeigt auf die naechste Version

Bewusst **nicht** im MVP:

- separater Revisionstisch
- diff-basierte Speicherung
- automatische Positions-Historisierung auf Feldebene
- verzweigte Revisionsbaeume

## Statusmodell im Revisionskontext

Die bestehenden Statuswerte koennen im ersten Schritt erhalten bleiben.
Revision ist zunaechst eine orthogonale Achse.

Das bedeutet:

- `draft`, `sent`, `accepted`, `rejected` bleiben bestehen
- zusaetzlich entscheidet `superseded_by_quote_id`, ob eine Version fachlich
  ueberholt ist

Fachregel fuer das MVP:

- Quotes mit `superseded_by_quote_id IS NOT NULL` sind read-only Historie
- nur die aktuelle Version einer Familie darf bearbeitet, versendet oder
  konvertiert werden
- eine bereits in Folgebelege ueberfuehrte Version darf nicht mehr revidiert
  werden, wenn dadurch Widerspruch zur Herkunft entstuende

## Nummerierung

Die bestehende Quote-Nummer bleibt die kaufmaennische Hauptnummer.
Die Revision wird im MVP zunaechst getrennt davon modelliert.

Empfehlung:

- `number` bleibt z. B. `ANG-2026-0042`
- Anzeige im UI/PDF: `ANG-2026-0042 / Rev. 2`

Bewusst nicht im ersten Schritt:

- neuer eigener Nummernkreis `quote_revision`
- neue physische Belegnummer pro Revision

Grund:

- das wuerde den bestehenden Folgebeleg- und PDF-Pfad unnoetig frueh vergroessern
- die eigentliche fachliche Luecke ist Versionierbarkeit, nicht Nummernformat

## Minimaler technischer Ausbaupfad

### Phase 1: Datenmodell und Lesbarkeit

- Migration fuer `root_quote_id`, `revision_no`, `superseded_by_quote_id`
- Rueckfuellung:
  - bestehende Quotes bekommen `root_quote_id = id`
  - `revision_no = 1`
- Quote-List- und Detail-Responses um Revisionsmetadaten erweitern

### Phase 2: Revision erzeugen

Neuer Endpoint:

- `POST /api/v1/quotes/{id}/revise`

Verhalten:

- kopiert Kopf- und Positionsdaten in neue Quote
- neue Quote erhaelt:
  - neue `id`
  - gleiches `root_quote_id`
  - `revision_no = alte revision_no + 1`
  - Status `draft`
- alte Quote bekommt `superseded_by_quote_id = neue id`

### Phase 3: Guard Rails

- nur aktuelle Version konvertierbar
- nur aktuelle Version manuell statusaenderbar
- Historienversionen im UI klar kennzeichnen

### Phase 4: UI-MVP

- Quotes-Liste zeigt Revisionshinweis
- Quote-Detail zeigt:
  - aktuelle Revision
  - Wurzel-/Vorgaenger-/Nachfolgerbezug
  - Aktion `Neue Revision erzeugen`

## Wichtige Guard Rails

- keine Revision, wenn Quote bereits in Auftrag oder Rechnung ueberfuehrt wurde
  und die Herkunft dadurch fachlich undeutlich wuerde
- keine stille Mutation historischer Versionen
- PDF einer alten Revision muss stabil reproduzierbar bleiben
- kommerzielle Kontextsicht pro Kontakt/Projekt sollte spaeter nur aktuelle
  Versionen standardmaessig prominent zeigen, Historie aber optional einblendbar

## Auswirkungen auf Folgebelege

Bestehende Folgebelege sollen weiter auf die konkrete Ursprungsquote zeigen.

Das heisst:

- `sales_orders.source_quote_id` bleibt auf einer konkreten Revision
- `invoices_out.source_quote_id` bleibt auf einer konkreten Revision
- es gibt im MVP keine automatische Umbuchung alter Folgebelege auf neuere
  Revisionen

Das ist fachlich korrekt, weil Auftrag oder Rechnung genau aus einer bestimmten
Angebotsversion entstanden sind.

## Nicht Teil des MVP

- Freigabe-Workflow fuer Revisionen
- Parallelrevisionen
- PDF-Versionsarchiv mit explizitem Snapshot-Objekt
- Preis-/Positions-Diff-Ansicht zwischen zwei Revisionen
- eigene Workflow-Kachel fuer "Revision erforderlich"

## Nächster sinnvoller technischer Schritt

Der naechste Umsetzungsschritt sollte klein und kontrolliert bleiben:

1. Migration und Lesefelder fuer Revisionsmetadaten auf `quotes`
2. Rueckfuellung aller Bestandsquotes auf `revision_no = 1`
3. noch **kein** UI zum Erzeugen von Revisionen

Damit wird zuerst das Datenmodell stabilisiert, bevor neue Schreiblogik in den
bestehenden Quote->Auftrag->Rechnung-Flow eingreift.
