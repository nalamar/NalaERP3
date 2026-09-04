# ADR 0012 — Abschlags-/Schlussrechnung nach VOB/B §16

Datum: 2026-08-18
Status: entschieden (Subtask B.4.1)
Bezug: Backlog B.4 (Epic B — Angebots- & Auftragswesen). `docs/01-gap-analysis.md:37`
("kein Feld/Status für Abschlagsrechnung vs. Schlussrechnung, keine
VOB-§16-spezifische Logik gefunden").

## Kontext

**Wichtigster Befund**: Teilrechnungsstellung gegen einen `sales_order`
existiert bereits vollständig und produktiv — `invoice_out_items.source_sales_order_item_id`
(`server/internal/migrate/migrations/038_sales_order_partial_invoicing.sql`)
verknüpft jede Rechnungsposition mit ihrer Auftragsposition,
`remainingQtyByItemTx`/`selectInvoiceQuantities`
(`server/internal/sales/service.go:1071-1135`) berechnen bereits korrekt
die noch offene Menge über ALLE bisherigen Rechnungen hinweg, und
`ConvertToInvoice` kann bereits mehrfach gegen denselben Auftrag
aufgerufen werden (`loadForInvoiceTx` erlaubt explizit erneute Konvertierung
im Status `invoiced`, `service.go:798-802`). B.4 baut daher NICHT die
Teilrechnungs-Mechanik selbst — die existiert schon —, sondern **nur die
VOB/B-§16-spezifische Typisierung und Geschäftsregel** obendrauf.

**Kein Feld für den Rechnungstyp existiert** — `invoices_out.status` ist
Buchungs-/Zahlungs-/Storno-Lebenszyklus (`draft`/`booked`/`partial`/`paid`/
`storniert`), nicht Rechnungsart. Es gibt keine Spalte, die
Abschlagsrechnung von Schlussrechnung unterscheidet.

**Wichtiger, bereits vorbestehender Fund** (nicht Teil dieser ADR zu
beheben, aber bei der Regelgestaltung zu berücksichtigen):
`ConvertToInvoice` setzt bei JEDEM Aufruf, auch bei einer kleinen
Teilrechnung, `sales_orders.status='invoiced'`
(`server/internal/sales/service.go:747`) — dieser Status bedeutet also
faktisch nur "mindestens eine Rechnung existiert", nicht "vollständig
abgerechnet". B.4 darf `status='invoiced'` daher NICHT als Signal für
"Schlussrechnung bereits gestellt" verwenden — ein neues, eindeutiges
Signal ist nötig.

**Fachliche Erkenntnis, die das Design vereinfacht**: da jede Rechnung
(Abschlag oder Schluss) laut der bestehenden Mechanik ohnehin nur die noch
NICHT abgerechnete Restmenge je Position enthält (nie doppelt), ist eine
Schlussrechnung automatisch bereits "um alle vorherigen
Abschlagszahlungen bereinigt" — es ist schlicht die Rechnung, die die
GESAMTE verbleibende Restmenge abdeckt. Eine separate monetäre
Verrechnungslogik ("Gesamtsumme minus bereits gezahlte Abschläge") ist
NICHT nötig, das leistet die bestehende Mengenlogik bereits.

**Bestehender GoBD-Storno-Mechanismus** (Backlog 0.3.1,
`062_invoices_out_storno.sql`) darf nicht angefasst werden: `Book()`/
`Storno()` bleiben unverändert, Storno mutiert nie die Original-Buchung.

## Optionen

**Option A — `invoices_out.status` um neue Werte erweitern** (z. B.
`'abschlag_booked'`, `'schluss_booked'`).
Verworfen: `status` ist bereits der Buchungs-/Zahlungs-/Storno-Lebenszyklus
(`draft → booked → partial/paid`, `storniert`) — eine Vermischung mit dem
Rechnungstyp würde die ohnehin schon komplexe Statuslogik in `Book()`/
`Storno()`/`payments.go` weiter verschachteln und widerspricht der
etablierten Trennung.

**Option B — separate, additive Spalte `invoice_type` auf `invoices_out`**,
Geschäftsregeln rein im Anwendungscode (`sales.Service.ConvertToInvoice`),
keine Änderung an der bestehenden Teilrechnungs-Mechanik oder am
Storno-Mechanismus. Gewählt.

## Entscheidung

**Option B.**

- **`invoices_out.invoice_type`**: neue Spalte `text NOT NULL DEFAULT
  'rechnung'`, `CHECK (invoice_type IN ('rechnung', 'abschlagsrechnung',
  'schlussrechnung'))`. Default `'rechnung'` — JEDE bereits bestehende und
  jede künftige, nicht auftragsgebundene Rechnung bleibt unverändert
  neutral, 100% rückwärtskompatibel, keine Datenmigration nötig.
- **Minimal-invasive Umsetzung**: `accounting.ARService.createTx`/
  `CreateFromSalesOrderTx`/`InvoiceOutInput` bleiben UNVERÄNDERT (kein
  Eingriff in den gemeinsamen, auch vom Quote-Rechnungspfad genutzten
  Low-Level-Insert-Pfad). Stattdessen setzt `sales.Service.ConvertToInvoice`
  den Typ per einer zusätzlichen `UPDATE invoices_out SET invoice_type=...`
  INNERHALB derselben, bereits offenen Transaktion, direkt nach dem
  bestehenden `arSvc.CreateFromSalesOrderTx(...)`-Aufruf — kein neuer
  Parameter in der `accounting`-Paket-Schnittstelle, minimale Blast-Radius
  in einer GoBD-sensiblen Datei. Lesepfad (`ARService.Get`/`List`) wird um
  `invoice_type` ergänzt (rein additiv, ein Feld mehr in SELECT/Scan).
- **`sales.ConvertToInvoiceInput` bekommt `InvoiceType`**: beim Aufruf über
  einen Auftrag MUSS explizit `'abschlagsrechnung'` oder
  `'schlussrechnung'` angegeben werden (kein stiller Default auf
  `'rechnung'` — eine auftragsgebundene Rechnung ist unter VOB/B immer das
  eine oder das andere, ein unbenannter Typ wäre eine unbelegte Annahme,
  aufgabe.md §7.10).
- **Geschäftsregel 1 — keine weitere Rechnung nach Schlussrechnung**: vor
  jeder Konvertierung wird geprüft, ob für denselben `sales_order_id`
  bereits eine NICHT stornierte Rechnung mit `invoice_type=
  'schlussrechnung'` existiert; falls ja, wird JEDE weitere Konvertierung
  (Abschlag ODER erneute Schlussrechnung) abgelehnt. Eine stornierte
  Schlussrechnung blockiert bewusst NICHT (GoBD-Storno-Pfad bleibt nutzbar,
  z. B. bei einer fehlerhaft gestellten Schlussrechnung).
- **Geschäftsregel 2 — Schlussrechnung muss vollständig sein**: wird
  `invoice_type='schlussrechnung'` angefordert, muss die daraus
  resultierende Rechnung JEDE Position vollständig bis zur Restmenge 0
  abrechnen (keine Teilmengen-Auswahl erlaubt) — sonst Ablehnung, BEVOR
  irgendetwas geschrieben wird. Keine separate Verrechnungslogik nötig
  (siehe Kontext: das leistet die bestehende Mengenlogik bereits
  automatisch).
- **`sales_orders.status='invoiced'` wird NICHT als Signal
  wiederverwendet** — die neue Prüfung fragt direkt `invoices_out.invoice_type`
  ab, unabhängig vom (bekanntermaßen ungenauen) Auftragsstatus. Der
  bestehende, dokumentierte Status-Bug wird NICHT im Rahmen von B.4
  behoben (separate, mögliche künftige Backlog-Position).
- **Kein Sicherheitseinbehalt (Retention/§17 VOB/B)**: bewusst außerhalb
  des Scopes — der Backlog-Titel nennt explizit nur "Abschlags-/
  Schlussrechnung nach VOB/B §16", Sicherheitseinbehalt ist ein separates,
  in §17 geregeltes Thema.

## Konsequenzen

- **B.4.2** (Folge-Subtask): additive Migration `074_...sql`
  (eine neue Spalte + CHECK-Constraint), reversibel.
- **B.4.3** (Folge-Subtask): Anwendungscode nur in `sales/service.go`
  (`ConvertToInvoice`-Erweiterung, neue Validierungsfunktionen) und
  lesend in `accounting/ar.go` (`invoice_type` in `Get`/`List`), HTTP-Wiring,
  Tests. **Hohe Sorgfalt** (wie B.1.3/B.3.3): `accounting.ARService`s
  Buchungs-/Storno-Kernpfade bleiben komplett unangetastet.
- Neue, mögliche künftige Backlog-Positionen (nicht Teil von B.4):
  Behebung des `sales_orders.status='invoiced'`-bei-Teilrechnung-Bugs;
  Sicherheitseinbehalt/Fälligkeit-nach-Abnahme (VOB/B §17 bzw. weitere
  §16-Absätze).
- Kein Einfluss auf bestehende Daten, den Storno-Mechanismus, oder den
  Quote-Rechnungspfad.
