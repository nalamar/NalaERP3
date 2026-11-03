# ADR 0035 — Nachweise der Zulieferer nach EN 1090

Datum: 2026-11-03
Status: **Angenommen** (2026-11-03, ANNAHME) — "fortsetzen" nach den
Fragen 1–6 mit Empfehlung (`docs/open-questions.md`, Abschnitt
2026-11-03): alle sechs wie empfohlen.
Bezug: Backlog H.4; offene Fragen 2026-10-30 Nr. 5 und 7 (ANNAHME
2026-11-02: eigenes ADR nach H.1, kein Bezug zum Fertigungsauftrag);
ADR 0034, Präzisierung EN 1090; ADR 0027 (Muster: Nachweis mit Datei,
Storno statt Löschung).

Anlass, Antwort des Nutzers (2026-11-02): "Wir fertigen selbst nicht in
EN 1090, haben aber Zulieferer, die das tun."

## Kontext (Ist-Stand, belegt)

- **Lieferant = Kontakt** mit Rolle `supplier`/`both`
  (`contacts/service.go` `Roles()`). Am Kontakt können beliebige Dateien
  hängen (`contact_documents`, Migration 024, Datei in GridFS), ohne Art,
  Gültigkeit oder Storno. Ein Zertifikat mit Ablaufdatum gibt es nicht.
- **Bestellung** (`purchase_orders`, Migration 004/056): Lieferant,
  Nummer, Status `draft | ordered | received | canceled`, Mandant und
  Niederlassung; **kein Projektbezug**. Positionen
  (`purchase_order_items`) immer mit Material.
- **Wareneingang ist kein eigener Beleg.** Der Client
  (`client/lib/purchase_order_receipt_flow.dart`,
  `pages/purchase_order_detail_page.dart` "Wareneingang buchen") bucht je
  Position eine Lagerbewegung `purchase` mit der vollen Bestellmenge und
  Referenz = Bestellnummer, danach (Fehler verschluckt) Status
  `received`. Er setzt dabei **weder** `purchase_order_item_id` (Spalte
  seit Migration 080, vom Server geprüft) **noch** eine Charge, obwohl
  der Server beides annimmt (`materials/service.go`, Upsert in `batches`).
  Teillieferungen sind so nicht abbildbar.
- **Chargen** (`batches`, Migration 001): Material + Code, Freitext
  `supplier`; keine Datei, kein Zeugnis.
- **Material** hat kein Kennzeichen "tragend / Nachweis erforderlich".
- **Rechte** (Migration 017): Einkauf `purchase_orders.read/write`,
  `contacts.read`, `materials.read`; Lager `stock_movements.write`, aber
  **kein** `purchase_orders.read` — die Lager-Rolle erreicht den
  Wareneingang an der Bestellung heute gar nicht.
- **Muster im Repo**: Qualifikationen der Mitarbeiter (ADR 0027,
  Migration 096) — Nachweis mit "gültig bis", Datei in GridFS, Storno
  statt Löschung, Dokumente nicht über den allgemeinen Download.

## Recherche (Sekundärquellen, nicht am Normtext geprüft)

| Thema | Befund | Folge für das Modell |
|---|---|---|
| Zertifikat des Zulieferers | Für die CE-Kennzeichnung nach EN 1090-1 braucht der Hersteller eine von einer benannten Stelle zertifizierte werkseigene Produktionskontrolle (WPK), System 2+ nach BauPVO 305/2011. Das Zertifikat nennt die höchste Ausführungsklasse (EXC) und hat eine Gültigkeit mit Überwachung | Am Lieferanten: Zertifikat mit Nummer, Stelle, EXC bis, gültig bis, Datei |
| Lieferung | Üblich (laut bauforumstahl "die meisten Hersteller"): Zulieferer liefern Produkte **mit CE-Kennzeichen und Leistungserklärung** auf Grundlage ihrer zertifizierten WPK; alternativ führt der Hersteller sie in seiner eigenen WPK — das entfällt hier, der Betrieb hat keine | Je Lieferung die Leistungserklärung ablegen |
| Werkstoffzeugnisse | Prüfbescheinigungen nach EN 10204 (2.2, 3.1, 3.2); 3.1/3.2 u. a. bei höheren Festigkeiten gefordert (Leitfaden WKO) | Art des Lieferpapiers als feste Liste, nicht nur "Datei" |
| Wareneingang | Material wird beim Wareneingang geprüft und gekennzeichnet, möglichst schon beim Einkauf einem Auftrag zugeordnet (EWM) | Bezug Lieferpapier → Bestellposition, optional Charge und Projekt |

Quellen: [bauforumstahl: CE-Kennzeichnung nach EN 1090](https://bauforumstahl.de/wp-content/uploads/2024/02/bfs-CE-Kennzeichnung_nach_EN-1090.pdf),
[Kiwa: EN 1090-1](https://www.kiwa.com/de/de/serviceart/zertifizieren/en-1090-1-ce-marking-steel-and-aluminum-constructions/),
[WIFI Zertifizierungsprogramm EN 1090 (2026)](https://zertifizierung.wifi.at/images/PR2010_00_Zertifizierungsprogramm_EN1090_ISO3834_DIN2303_20260123.pdf),
[WKO Leitfaden EN 1090](https://www.wko.at/ooe/gewerbe-handwerk/metalltechniker/leitfaden-en-1090-bi-2-.pdf),
[EWM: Der Weg zur Zertifizierung](https://www.ewm-group.com/images/pdf/wpqr/WM087600_EN1090_Weg_zur_Zertifizierung_05-2012.pdf).
Rechtlich verbindlich sind nur Norm und benannte Stelle; das ERP
**dokumentiert**, es bewertet nicht.

## Zu entscheiden

### 1. Was wird am Lieferanten erfasst?
Optionen: (a) Datei am Kontakt wie bisher (`contact_documents`) —
kein Ablaufdatum, keine Prüfung möglich; (b) eigener Nachweis
"Lieferantenzertifikat" mit Art (feste Liste: `en1090`, `iso3834`,
`sonstiges`), Nummer, ausstellende Stelle, EXC bis (nur `en1090`,
EXC1–EXC4), gültig von/bis, Dateien; Storno mit Begründung statt
Löschung (Beweisfunktion, wie ADR 0027).
**Empfehlung: (b).**

### 2. Wann gilt ein Material als nachweispflichtig?
Optionen: (a) Kennzeichen am Material "Nachweis EN 1090 erforderlich";
(b) Kennzeichen je Bestellung von Hand; (c) alles, was ein Lieferant mit
Zertifikat liefert.
**Empfehlung: (a)** — einmal am Materialstamm gepflegt, wirkt auf jede
Bestellung; ohne Kennzeichen ändert sich nichts.

### 3. Hinweis oder Sperre bei fehlendem / abgelaufenem Zertifikat?
Eine Bestellung mit nachweispflichtigem Material bei einem Lieferanten
ohne gültiges `en1090`-Zertifikat (am Bestelldatum, gültig bis ≥
spätester Liefertermin der Positionen, sonst Bestelldatum):
(a) Hinweis an der Bestellung, keine Sperre; (b) Bestellen (`ordered`)
sperren.
**Empfehlung: (a)** wie die Hinweise in H.1 — der Einkauf entscheidet;
dazu ein Hinweis am Lieferanten, wenn ein Zertifikat in 60 Tagen
abläuft. Keine Mail in H.4.

### 4. Wo hängen Leistungserklärung und Zeugnis der Lieferung?
Es gibt keinen Wareneingangsbeleg. Optionen: (a) Lieferpapier an der
**Bestellung**, optional mit Bestellposition und Chargencode, Art aus
fester Liste (`leistungserklaerung`, `zeugnis_2_2`, `zeugnis_3_1`,
`zeugnis_3_2`, `lieferschein`, `sonstiges`), Storno statt Löschung;
(b) neuer Beleg "Wareneingang" mit Teillieferungen, Dokumente daran;
(c) an der Lagerbewegung.
**Empfehlung: (a)** — klein, passt zum heutigen Ablauf; Hinweis an der
Bestellung im Status `received`, wenn eine nachweispflichtige Position
keine Leistungserklärung hat. (b) bleibt als eigener Punkt möglich, wenn
Teillieferungen gebraucht werden; (a) ließe sich dann übernehmen.

### 5. Bezug zum Projekt?
Bestellungen haben kein Projekt. Optionen: (a) optionales Projekt an der
Bestellung (`purchase_orders.project_id`, im Mandanten geprüft), damit
die Nachweise eines Projekts auffindbar sind; (b) Projekt je Lieferpapier;
(c) kein Projektbezug.
**Empfehlung: (a)**; eine Liste "Nachweise des Projekts" im Projekt ist
ein späterer Schritt, kein Teil von H.4.

### 6. Rechte
Optionen: (a) bestehende Rechte — Zertifikate und Lieferpapiere lesen
mit `purchase_orders.read`, schreiben/stornieren mit
`purchase_orders.write`; Kennzeichen am Material mit `materials.write`;
Dateien **nicht** über den allgemeinen Download `/documents/{id}`;
(b) neue Rechte `supplier_certificates.read/write`; (c) zusätzlich die
Rolle Lager Lieferpapiere ablegen lassen.
**Empfehlung: (a)** — Einkauf pflegt, keine neuen Rollen. Die Rolle
Lager kommt heute nicht an die Bestellung (kein `purchase_orders.read`);
(c) erst, wenn der Wareneingang ein eigener Beleg wird.

## Befund außerhalb der Fragen

Der Wareneingang im Client setzt `purchase_order_item_id` nicht und
verschluckt den Fehler beim Statuswechsel. Das betrifft den Abgleich mit
Eingangsrechnungen (Migration 080). Vorschlag: eigener kleiner Punkt
H.4.x "Wareneingang setzt die Bestellposition" (Client, keine Migration)
— unabhängig von der Entscheidung oben.

## Folgen bei Entscheidung wie empfohlen (Vorschau, nicht beschlossen)

- H.4.1 Migration 110: `supplier_certificates` (+ Dokumente),
  `materials.nachweis_en1090`, `purchase_order_documents`,
  `purchase_orders.project_id`; Fachregeln (Gültigkeit am Stichtag,
  Hinweise) mit Tests.
- H.4.2 Service und Routen Zertifikate am Lieferanten.
- H.4.3 Service und Routen Lieferpapiere an der Bestellung, Hinweise an
  der Bestellung, Projekt an der Bestellung.
- H.4.4 Client Lieferant (Zertifikate, Hinweis Ablauf), Material
  (Kennzeichen).
- H.4.5 Client Bestellung (Hinweise, Lieferpapiere, Projekt).
- H.4.x Wareneingang setzt die Bestellposition (siehe Befund).

## Präzisierung H.4.1a (2026-11-03, Migration 110)

- Tabellen `supplier_certificates` (+ `supplier_certificate_documents`),
  `purchase_order_documents`; Spalten `materials.nachweis_en1090`
  (Vorgabe false) und `purchase_orders.project_id` (optional, RESTRICT).
- ANNAHME: `gueltig_bis` leer heißt "ohne Enddatum" (die Stelle
  überwacht laufend); gesetzt gilt das Zertifikat bis einschließlich
  diesem Tag. `exc_bis` genau bei `en1090` Pflicht, sonst leer.
- Nie löschen: Zertifikat, seine Dateien und Lieferpapiere hängen mit
  RESTRICT an Lieferant bzw. Bestellung; Storno mit Grund. Eine im
  Entwurf gelöschte Position setzt den Bezug des Lieferpapiers auf leer,
  das Papier bleibt an der Bestellung.
- Mandant: dass Lieferant, Bestellung, Position und Projekt zum selben
  Mandanten gehören, prüft der Service (H.4.2/H.4.3) —
  `contacts.company_id` ist nullable (Migration 054), ein Fremdschlüssel
  über den Mandanten geht daher nicht.
- Fund beim Test: `exc_bis IN (...)` ist bei NULL selbst NULL, und ein
  CHECK lässt NULL durch; die Bedingung prüft darum ausdrücklich
  `exc_bis IS NOT NULL`.

## Präzisierung H.4.1b (2026-11-03, Fachregeln)

`purchasing/evidence.go`. ANNAHMEN (aus den Empfehlungen abgeleitet):

- **Zeitraum**: ein Zertifikat muss vom Bestelldatum bis zum Stichtag
  gelten; Stichtag ist der späteste Liefertermin der Positionen, ohne
  Liefertermin das Bestelldatum. Tagesgenau, Grenzen einschließlich.
- Zwei Hinweise statt einem: `zertifikat_fehlt` (kein nicht storniertes
  EN-1090-Zertifikat) und `zertifikat_ungueltig` (vorhanden, deckt den
  Zeitraum aber nicht — abgelaufen **oder noch nicht gültig**). Nur
  EN-1090-Zertifikate zählen für die Bestellung.
- Die **Ausführungsklasse** wird nicht verglichen: das Kennzeichen am
  Material hat keine Klasse (Frage 2). EXC bis wird nur dokumentiert.
- `zertifikat_laeuft_ab` am Lieferanten für jede Art: endet heute bis in
  60 Tagen und kein anderes Zertifikat derselben Art gilt am Tag danach.
  Schon abgelaufene geben diesen Hinweis nicht.
- `leistungserklaerung_fehlt` nur im Status `received`; eine
  Leistungserklärung ohne Position gilt für die ganze Bestellung;
  Zeugnisse ersetzen sie nicht.
- Eine stornierte Bestellung (`canceled`) hat keine Hinweise.

## Präzisierung H.4.2 (2026-11-03, Service und Routen der Zertifikate)

- Pfad `/supplier-certificates` (Liste mit `lieferant_id`) statt
  `/contacts/{id}/certificates`: die Kontaktrouten hängen an
  `contacts.*`, die Nachweise am Einkauf (§6).
- Lieferant heißt: Kontakt im Mandanten mit Rolle `supplier` oder
  `both`; sonst "Lieferant nicht gefunden".
- Die Liste zeigt stornierte Zertifikate nur auf Wunsch; der Hinweis
  "läuft ab" rechnet sie nie mit.
- Meldungen der Regeln beginnen mit "Ungültig…", damit sie als 400
  ankommen (die Fehlerzuordnung arbeitet über Textbausteine).

## Präzisierung H.4.3a (2026-11-03, Service der Lieferpapiere)

- ANNAHME: Lieferpapiere sind in jedem Status der Bestellung erlaubt
  außer `canceled` (Papiere kommen vor, mit oder nach der Ware); zu einer
  stornierten Bestellung bleiben vorhandene Papiere lesbar.
- Die Position ist optional und muss zu genau dieser Bestellung gehören;
  leer heißt "gilt für die ganze Bestellung" (wie in H.4.1b).
- Ein storniertes Lieferpapier bleibt mit Datei lesbar (Nachweis, warum
  es ersetzt wurde).

## Präzisierung H.4.3b (2026-11-03, Routen der Lieferpapiere)

- Pfad `/purchase-orders/{id}/papers` unter der Bestellung (Rechte des
  Einkaufs wie die Bestellung selbst). Upload als Multipart: Datei im
  Feld `file`, Angaben `art`, `position_id`, `charge`, `notiz` als
  Formularfelder.

## Präzisierung H.4.3c (2026-11-03, Projekt und Lieferant an der Bestellung)

- ANNAHME: das Projekt ist eine interne Zuordnung und kein Inhalt des
  Belegs an den Lieferanten; es lässt sich darum in jedem Status ändern
  (die Festschreibung ab `ordered` gilt dafür nicht). Leer entfernt es.
- Das Projekt muss zum Mandanten gehören (§5); der Lieferant einer neuen
  Bestellung ist ein Kontakt im Mandanten mit Rolle `supplier` oder
  `both` (wie bei den Zertifikaten, H.4.2).
