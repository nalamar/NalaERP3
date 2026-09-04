# ADR 0010 — Nachtragsmanagement für bestehende Aufträge

Datum: 2026-08-18
Status: entschieden (Subtask B.2.1)
Bezug: Backlog B.2 (Epic B — Angebots- & Auftragswesen), aufgabe.md §1
(Domäne B: "Nachträge"). `docs/01-gap-analysis.md:33` hatte die Lücke
bereits präzise benannt: "kein Konzept 'Nachtrag zu bestehendem Auftrag' im
Schema (nur Angebots-Revisionierung `root_quote_id`, das ist
Angebotsversionierung vor Zuschlag, kein Nachtragsmanagement nach
Auftragserteilung)".

## Kontext

`sales_orders`/`sales_order_items`
(`server/internal/migrate/migrations/034_sales_orders.sql`) haben keinerlei
Versionierungs-/Änderungsspalten (kein `revision_no`, kein
`superseded_by_*`). Der einzige Analog-Mechanismus im Repo ist
`quotes.Service.Revise()` (`server/internal/quotes/service.go:3271-3459`) —
aber das ist strukturell und rechtlich etwas anderes: eine
**Vorvertrags**-Versionierung (neues Angebot ersetzt das alte, VOR
Auftragserteilung, nur bis Status `accepted`/Konvertierung möglich). Ein
VOB/B-**Nachtrag** ist das Gegenteil: eine zusätzliche Vereinbarung
(Leistungsänderung nach VOB/B §1 Abs. 3, zusätzliche Leistung nach §1 Abs.
4/§2 Abs. 6) zu einem **bereits erteilten, weiterhin gültigen** Auftrag —
der ursprüngliche Auftrag bleibt unverändert bestehen, der Nachtrag kommt
additiv hinzu und durchläuft eine eigene Vereinbarungs-Historie (beantragt
→ angenommen/abgelehnt).

`sales_orders` hat bereits einen etablierten Schreibschutz-Mechanismus aus
Backlog 0.3.2 (`isEditableStatus`, `server/internal/sales/service.go:833-840`
— nur `open`/`released` sind editierbar) sowie eine bestehende, aber
**unvollständige** Audit-Anbindung (`s.audit`, aus Backlog 0.3.3.3): nur
`UpdateStatus` und `ConvertToInvoice` protokollieren, Header-/Item-Änderungen
NICHT. Diese vorbestehende Lücke wird durch B.2 NICHT generell geschlossen
(wäre Scope-Creep) — aber die Nachtrag-**Entscheidung** selbst (das
fachlich zentrale, GoBD-relevante Ereignis eines Nachtragsmanagements) wird
bewusst von Anfang an protokolliert, da das der eigentliche Zweck dieses
Features ist.

`sales_order_items` hat kein `material_id` und keine LV-Hierarchie
(B.1s `quote_item_groups` wurde bewusst nicht auf `sales_orders` übertragen,
siehe B.1.3-Notiz) — Nachtragspositionen bilden dasselbe, bewusst schlanke
Spaltenset nach, keine Erweiterung über das Bestehende hinaus.

## Optionen

**Option A — Nachtrag als neue Version des gesamten Auftrags** (analog zu
`quotes.Revise()`: neue `sales_orders`-Zeile, alte wird
`superseded_by`-markiert).
Verworfen: passt fachlich nicht. Ein VOB/B-Nachtrag ersetzt NICHT den
Grundauftrag — der Grundauftrag bleibt exakt gültig, der Nachtrag kommt
additiv hinzu. Eine "neue Vollversion pro Nachtrag" würde bei mehreren
Nachträgen zu einem Auftrag unnötig viele vollständige Auftragskopien
erzeugen und widerspricht dem Prinzip "ursprünglicher Auftrag bleibt
unverändert" (GoBD-Festschreibung, aufgabe.md §2).

**Option B — Nachtrag als zusätzliche Positionen direkt in
`sales_order_items`** (z. B. mit einem `ist_nachtrag boolean`-Flag).
Verworfen: ein Nachtrag ist fachlich mehr als eine Positionsliste — er hat
einen eigenen Status (beantragt/angenommen/abgelehnt), eine eigene
Begründung, ein eigenes Entscheidungsdatum. Das in eine einzelne
Boolean-Spalte auf `sales_order_items` zu pressen würde diese Informationen
verlieren oder erzwingt Umwege (z. B. Status auf Auftragsebene statt
Nachtragsebene). Zusätzlich: ein abgelehnter Nachtrag dürfte NIE in die
Auftragssumme einfließen — mit Option B wäre das nur über zusätzliche,
fehleranfällige Filterlogik an jeder Summierungsstelle sicherzustellen.

**Option C — separate Kopf/Positionen-Tabellen `sales_order_addenda` +
`sales_order_addendum_items`**, analog zum bereits mehrfach etablierten
Muster (Preislisten aus ADR 0007, LV-Hierarchie aus ADR 0009). Gewählt.

## Entscheidung

**Option C.**

- **`sales_order_addenda`** (Nachtrag-Header): `id` (UUID, Konvention der
  `sales`/`quotes`-Domäne), `sales_order_id` (FK →
  `sales_orders(id) ON DELETE CASCADE`), `nachtrag_no` (int, sequentiell
  PRO Auftrag — nicht global über `number_sequences`, da ein Nachtrag
  fachlich eine Unternummerierung des Auftrags ist, `UNIQUE
  (sales_order_id, nachtrag_no)`), `status` (text, `entwurf` |
  `beantragt` | `angenommen` | `abgelehnt`), `begruendung` (text NOT NULL —
  VOB/B verlangt eine Begründung für Leistungsänderungen/Zusatzleistungen),
  `currency` (char(3), beim Anlegen automatisch vom Auftrag übernommen,
  kein Nutzereingabefeld — verhindert Mehrwährungs-Inkonsistenzen
  innerhalb eines Auftrags), `net_amount`/`tax_amount`/`gross_amount`
  (numeric(18,4), aus den eigenen Positionen berechnet, analog
  `sales_orders`/`quotes`), `beantragt_am`/`entschieden_am` (timestamptz,
  nullable), `entschieden_von` (text, User-ID — intern erfasste
  Entscheidung, keine externe Kundenportal-Freigabe, da keine
  Kunden-Logins im System existieren), `ablehnungsgrund` (text DEFAULT ''),
  `created_at`.
- **`sales_order_addendum_items`** (Nachtrag-Positionen): identisches,
  bewusst schlankes Spaltenset wie `sales_order_items` (`id`, `addendum_id`
  FK → `sales_order_addenda(id) ON DELETE CASCADE`, `position`,
  `description`, `qty`, `unit`, `unit_price`, `net_amount`, `tax_amount`,
  `tax_code`) — keine Erweiterung (kein `material_id`, keine
  Hierarchie), da `sales_order_items` selbst keine dieser Eigenschaften
  hat.
- **Statuslogik**: nur `entwurf` ist editierbar (Positionen
  hinzufügen/ändern/löschen) — analog `isEditableStatus` für Aufträge
  selbst. `beantragt` friert die Positionen ein (Festschreibung ab
  Antragstellung, GoBD-Geist). `angenommen`/`abgelehnt` sind terminal —
  KEIN Zurücksetzen auf `beantragt`/`entwurf` (analog Storno-statt-Änderung-
  Prinzip: ein neu zu verhandelnder Nachtrag wird als NEUER Nachtrag
  angelegt, nicht durch Wiederbeleben eines abgelehnten).
- **Auftragsberechtigung**: Nachtrag-Anlage nur für Aufträge mit Status
  `open`/`released` (identisch zu `isEditableStatus`) — ein Nachtrag zu
  einem bereits `invoiced`/`completed`/`canceled`-Auftrag wird in dieser
  ersten Ausbaustufe bewusst nicht unterstützt (kann bei Bedarf später als
  eigene, informierte Entscheidung erweitert werden, siehe Konsequenzen).
- **Effektive Auftragssumme wird NICHT in `sales_orders` selbst
  geschrieben.** Der ursprüngliche Auftrag bleibt fachlich und GoBD-seitig
  unverändert (keine stille Mutation bereits festgeschriebener Summen).
  Stattdessen berechnet die Anwendungsschicht (Subtask B.2.3) die
  effektive Summe zur Lesezeit als Grundauftrag + Summe aller `angenommen`-
  Nachträge — additiv, nachvollziehbar, ohne Update-Anomalien.
- **Berechtigungen**: keine neue Permission-Infrastruktur — Nachtrag-CRUD
  und -Entscheidung nutzen die bestehende `sales_orders.write`/
  `sales_orders.read`, da die Entscheidung intern getroffen wird (kein
  gesondertes Freigabe-Rollenkonzept wie bei `quotes.approve, das für einen
  mehrstufigen Preisfreigabe-Workflow gebaut wurde` — hier reicht ein
  einfacher Statuswechsel).
- **Audit-Log**: die Nachtrag-**Entscheidung** (beantragt→angenommen bzw.
  beantragt→abgelehnt) wird von Anfang an über das bestehende, generische
  `entity_change_log`/`auditlog`-System protokolliert (`entity_type =
  'sales_order_addendum'`) — das ist der fachliche Kern eines
  Nachtragsmanagements ("wer hat wann welchen Nachtrag entschieden").
  Die bereits bestehende, unabhängige Lücke (Header-/Item-Änderungen an
  `sales_orders` selbst sind nicht auditiert) wird NICHT im Rahmen von B.2
  geschlossen — dokumentiert als separate, mögliche künftige
  Backlog-Position.

## Konsequenzen

- **B.2.2** (Folge-Subtask): additive Migration `072_...sql` (zwei neue
  Tabellen, keine bestehende Tabelle verändert, reversibel via `DROP
  TABLE`).
- **B.2.3** (Folge-Subtask): CRUD + Statusworkflow im `sales`-Paket
  (Anlegen/Positionen pflegen/beantragen/entscheiden), Effektiv-Summen-
  Helfer, Audit-Anbindung für Entscheidungen, HTTP-Wiring, Tests.
- **Neue, separate, mögliche Backlog-Position** (nicht Teil von B.2):
  generelle Audit-Anbindung für `sales_orders`-Header-/Item-Mutationen
  (vorbestehende Lücke, bei der Recherche zu B.2 bestätigt).
- **Neue, separate, mögliche Backlog-Position**: Nachträge auch für
  Aufträge mit Status `invoiced` zulassen, falls sich fachlicher Bedarf
  zeigt (aktuell bewusst nicht Teil von B.2).
- Kein Einfluss auf bestehende Daten, Tabellen oder Endpunkte. Kein
  Einfluss auf `quotes.Revise()` oder die B.1-LV-Hierarchie (beide bleiben
  unverändert, strukturell getrennte Konzepte).
