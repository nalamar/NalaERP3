# ADR 0011 — Vollständiges Kalkulationsschema (Material/Lohn/Fremdleistung/Zuschläge getrennt)

Datum: 2026-08-18
Status: entschieden (Subtask B.3.1)
Bezug: Backlog B.3 (Epic B — Angebots- & Auftragswesen). `anweisung.md:94`
(Ursprungs-Vorgabe: "Kalkulationsschema für Material, Lohn, Fremdleistung
und Zuschläge definieren") und `docs/01-gap-analysis.md:34` ("einfaches
Zielmargen-Modell, kein vollständiges Kalkulationsschema").

## Kontext

`quote_items.unit_price` ist seit der Basismigration (`032_quotes.sql:29`)
eine einzelne, flache `numeric(18,4)`-Spalte — keine Migration hat je eine
Kostenart-Aufschlüsselung ergänzt. Die bestehende Preisfindungskette
(`primaryPriceSourceForMaterialTx` → `ApplyPrimaryPriceSourceForQuoteItem`
→ `ApplyTargetUnitPriceForQuoteItem`, `server/internal/quotes/service.go`)
ist **ausschließlich materialkostenbasiert** (letzter Bestellpreis oder
`materials.avg_purchase_price`) und wendet **eine einzige, pauschale**
Zielmarge (`quote_calculation_settings.target_margin_percent`, aktuell
20 %) auf diese eine Kostenbasis an. Eine Position ganz ohne `material_id`
(z. B. eine reine Lohn- oder Fremdleistungsposition) kann diese Kette
überhaupt nicht durchlaufen (`primaryPriceSourceForMaterialTx` verlangt
`material_id`, sonst Fehler "Angebotsposition hat kein Material").

**Es existiert im gesamten Repo keine Lohnkosten-Quelle** (kein
Stundensatz-Feld in `hr.Employee`, `CostCenter` ist reiner Freitext) und
**keine Fremdleistungskosten-Quelle** (keine Nachunternehmer-/
Fremdleistungs-Struktur in `purchasing`). Beide Kostenarten müssen daher
in dieser ADR als manuell erfasste Werte modelliert werden — es gibt
nichts, worauf B.3 automatisch zurückgreifen könnte (anders als bei
Material, wo bereits ein Lookup-Mechanismus existiert).

**Bewusste Abgrenzung**: die bestehende Zielmargen-Kette
(`quote_calculation_settings.target_margin_percent`,
`ApplyTargetUnitPriceForQuoteItem`) wird NICHT entfernt oder umgebaut —
sie ist ein bereits produktiv genutzter, getesteter, unabhängiger
Preisfindungsweg für materialbasierte Positionen (einfache pauschale
Marge). B.3 fügt einen **zweiten, parallelen, detaillierteren**
Kalkulationsweg hinzu, der Nutzer optional statt der pauschalen Marge
verwenden können, wenn sie Material/Lohn/Fremdleistung getrennt
kalkulieren wollen — kein Breaking Change am bestehenden Mechanismus.

## Optionen

**Option A — `quote_items.unit_price` durch mehrere Kostenart-Spalten
ersetzen** (Material/Lohn/Fremdleistung direkt auf `quote_items`).
Verworfen: `unit_price` wird an sehr vielen Stellen im bestehenden,
fragilen `quotes/service.go` gelesen/geschrieben (Summenbildung,
Rechnungsstellung, Preisentscheidungs-Historie, GAEB-Import-Übernahme,
Revise-Kopie) — ein Ersatz durch mehrere Spalten wäre ein tiefgreifender,
riskanter Umbau des gesamten bestehenden Preispfads und würde JEDE
bestehende Position zwingen, das neue Schema zu nutzen, auch wenn sie es
gar nicht braucht (z. B. GAEB-Importe ohne Kalkulationsbedarf).

**Option B — separate, optionale 1:1-Tabelle `quote_item_calculations`**
(Kalkulationsdetails je Position, unabhängig von `unit_price`; ein
expliziter "Anwenden"-Schritt schreibt das Ergebnis in `unit_price`,
analog zum bereits etablierten Muster `ApplyPrimaryPriceSourceForQuoteItem`/
`ApplyTargetUnitPriceForQuoteItem`). Gewählt.

## Entscheidung

**Option B.**

- **`quote_item_calculations`**: `id` (UUID), `quote_item_id` (UUID,
  `UNIQUE` FK → `quote_items(id) ON DELETE CASCADE` — genau eine
  Kalkulation je Position, 1:1), getrennte Kostenarten mit jeweils
  eigenem Zuschlagssatz (klassische deutsche
  "Zuschlagskalkulation mit Kostenartentrennung", nicht eine gemeinsame
  Marge auf eine Gesamtsumme):
  - `material_cost` (numeric(18,4), Materialkosten je Einheit),
    `material_zuschlag_percent` (numeric(6,2))
  - `lohn_stunden` (numeric(18,4), Arbeitsstunden je Einheit),
    `lohn_stundensatz` (numeric(18,4), €/h — manuell erfasst, keine
    Quelle vorhanden, s. Kontext), `lohn_zuschlag_percent` (numeric(6,2))
  - `fremdleistung_cost` (numeric(18,4), Fremdleistungskosten je Einheit
    — manuell erfasst, keine Quelle vorhanden), `fremdleistung_zuschlag_percent`
    (numeric(6,2))
  - `created_at`/`updated_at`
  - Alle Kosten-/Stunden-/Satz-Felder `CHECK (... >= 0)` (keine negativen
    Kalkulationswerte).
- **Berechnung** (Anwendungsschicht, B.3.3, nicht in der DB gespeichert —
  reine Ableitung, um Inkonsistenzen zwischen gespeicherten und
  berechneten Werten auszuschließen):
  ```
  material_total = material_cost * (1 + material_zuschlag_percent/100)
  lohn_cost      = lohn_stunden * lohn_stundensatz
  lohn_total     = lohn_cost * (1 + lohn_zuschlag_percent/100)
  fremdleistung_total = fremdleistung_cost * (1 + fremdleistung_zuschlag_percent/100)
  calculated_unit_price = material_total + lohn_total + fremdleistung_total
  ```
- **"Anwenden"-Schritt**: ein expliziter Aufruf schreibt
  `calculated_unit_price` in `quote_items.unit_price` (inkl.
  Neuberechnung von `net_amount`/`tax_amount`, identisches Muster wie
  `ApplyPrimaryPriceSourceForQuoteItem`) und protokolliert die
  Entscheidung in der bereits bestehenden `quote_item_price_decisions`
  (neuer `decision_type`-Wert `'calculation_scheme_applied'`, CHECK-
  Constraint-Erweiterung analog zu Migration 066) — Kalkulation und
  bestehende Preisentscheidungs-Historie bleiben EINE gemeinsame
  Quelle der Wahrheit statt eines zweiten, unabhängigen Protokolls.
- **Standardwerte für Zuschlagssätze/Stundensatz**: `quote_calculation_settings`
  (bereits bestehende Singleton-Tabelle, ADR-Konsequenz statt neuer
  Tabelle) wird additiv um `default_material_zuschlag_percent`,
  `default_lohn_zuschlag_percent`, `default_fremdleistung_zuschlag_percent`,
  `default_lohn_stundensatz` erweitert — alle `NOT NULL DEFAULT 0`.
  **Bewusst 0 als Default, nicht ein geschätzter Praxiswert** (z. B.
  "80 % Lohnzuschlag" wäre eine unbelegte fachliche Annahme, aufgabe.md
  §0/§7.10) — Mandanten müssen ihre tatsächlichen Sätze selbst
  hinterlegen, bevor das Schema sinnvoll nutzbar ist. Das bestehende
  `target_margin_percent` (Default 20, aus einer früheren, nicht von
  dieser ADR zu hinterfragenden Entscheidung) bleibt unverändert.
- **Kein eigenes `company_id`** auf `quote_item_calculations` — Scope wird
  über `quote_item_id` → `quote_id` → `quotes.company_id` geerbt
  (identisches Muster wie `quote_item_groups` aus ADR 0009).
- **Material-Kostenermittlung bewusst nicht automatisch verknüpft**: `B.3`
  baut keine automatische Kopplung an `primaryPriceSourceForMaterialTx`
  beim Anlegen einer Kalkulation (das würde die ohnehin schon große
  Preisfindungskette weiter verzahnen und das Risiko für die fragile
  Datei erhöhen). Der bestehende Preisvorschlags-Endpunkt bleibt
  weiterhin separat nutzbar; ein Nutzer/Client kann den dort gelieferten
  Wert manuell als `material_cost` übernehmen. Eine engere Kopplung ist
  als mögliche spätere, eigenständige Backlog-Position denkbar.

## Konsequenzen

- **B.3.2** (Folge-Subtask): additive Migration `073_...sql` (neue Tabelle
  `quote_item_calculations`, additive Spalten auf
  `quote_calculation_settings`, CHECK-Erweiterung auf
  `quote_item_price_decisions.decision_type`), reversibel.
- **B.3.3** (Folge-Subtask): CRUD + Berechnungs-/Anwenden-Logik im
  `quotes`-Paket (nur additiv, `quotes/service.go` bleibt strukturell
  unangetastet außer der CHECK-Erweiterung und einem neuen
  `decision_type`-Wert), HTTP-Wiring, Tests. **Hohe Sorgfalt geboten**
  (wie schon bei B.1.3): keine bestehende Funktion umbauen.
- Kein Einfluss auf bestehende Daten, Endpunkte, oder die bestehende
  Zielmargen-Kette — beide Preisfindungswege koexistieren.
