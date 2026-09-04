# ADR 0015 — Verschnitt-/Reststückverwaltung für Profile

Datum: 2026-08-23
Status: entschieden (Subtask C.3.1)
Bezug: Backlog C.3 (Epic C — Waren- & Lagerwirtschaft, letzte Task).

## Kontext

Profile (Aluminium-/Stahl-Profilstäbe für Fenster/Türen) werden als
Standardlängen eingekauft und je Projekt auf Maß abgelängt
(`materials.profilserie`, seit A.1 — `docs/adr/0006-metallbau-artikel-profilattribute.md`).
Beim Ablängen bleibt häufig ein Reststück übrig, das für den aktuellen
Zuschnitt zu kurz, für einen künftigen kleineren Zuschnitt aber noch
brauchbar ist. Aktuell gibt es dafür keinerlei Abbildung — der bestehende
Lagerbestand (`stock_movements`, C.1/C.2) kennt nur additive Mengen
(`quantity`), keine Länge einzelner Stücke; zwei Stäbe à 1,5 m und ein
Stab à 3 m sind im aktuellen Modell ununterscheidbar (beide ergeben
`quantity=3`, wenn `einheit='m'`).

`materials` hat bereits `length_mm numeric(18,6)` (seit
`014_material_dimensions_and_units.sql`) — dort beschreibt es aber die
Standardlänge des ARTIKELS (Stammdatum), nicht die Länge eines konkreten,
einzelnen Lagerstücks. Für C.3 wird eine neue, davon unabhängige
Längenangabe je EINZELSTÜCK benötigt.

Der Backlog-Titel unterscheidet explizit "Verschnitt" (Abfall, zu kurz für
Wiederverwendung) von "Reststück" (noch brauchbarer Rest) — im Unterschied
zu C.1/C.2 gibt es dafür in `aufgabe.md` oder den bisherigen Recon-
Dokumenten keine Vorgabe einer konkreten Mindestlänge, ab der ein Rest als
wiederverwendbar gilt. Eine feste Zahl (z. B. "500 mm") wäre eine
unbelegte Annahme (aufgabe.md §7.1).

Etabliertes Muster dieser Session: additive, nie mutierte Historie
(`stock_movements`), Scope-Vererbung über `warehouse_id` (ADR 0002,
zuletzt C.1/C.2), Wiederverwendung von `stock_movements.read/write` statt
neuer Permission-Infrastruktur (C.1/C.2), Implementierung in
`materials.Service`.

## Optionen

**Option A — feste, konfigurierbare Mindestlänge** (globale oder
Mandanten-Einstellung `min_reststueck_length_mm`), unterhalb derer ein
Rest automatisch als Verschnitt gilt und nicht registrierbar ist.
Verworfen: keine belegte fachliche Vorgabe für einen Zahlenwert oder
überhaupt die Notwendigkeit einer globalen Schwelle — Metallbaubetriebe
unterscheiden sich stark darin, was für sie noch "brauchbar" ist (abhängig
von Profiltyp, üblichen Auftragsgrößen). Eine erfundene Konstante wäre
eine unbelegte Annahme.

**Option B — Registrierung ist die Entscheidung**: JEDES Reststück, das
ein Nutzer explizit registriert, GILT damit als Reststück (wiederverwend-
bar); alles, was nicht registriert wird, ist implizit Verschnitt (Abfall)
— ohne dass das System selbst eine Länge bewertet. Gewählt.

**Zu Option B — Verbrauch eines Reststücks, das nur teilweise
gebraucht wird**: bestehende Zeile mutieren (Länge reduzieren) vs.
bestehende Zeile abschließen + neue Zeile für den neuen Rest anlegen?
Entscheidung: Zeile abschließen + neue Zeile anlegen. Konsistent mit dem
in dieser Session durchgängig etablierten Muster "Storno/Korrektur statt
Mutation" (`stock_movements` additiv, `invoices_out` Storno-statt-Änderung,
ADR 0012) — ein Reststück-Datensatz beschreibt IMMER ein einzelnes,
physisches Stück mit fixer Ursprungslänge; wird ein Teil davon verwendet,
entsteht ein NEUES, kleineres Stück mit eigener Historie
(`source_offcut_id`-Verkettung), das ursprüngliche Stück wird als
verbraucht abgeschlossen, nie nachträglich verändert.

**Zu Option B — Beschränkung auf Profil-Materialien**: Registrierung für
jedes Material erlauben vs. nur für Materialien mit gesetztem
`profilserie`?
Entscheidung: nur für Materialien mit gesetztem `profilserie`
(`length_mm`/Stückzahl-Logik ergibt für nicht-Profil-Artikel — z. B.
Schrauben, Beschläge — fachlich keinen Sinn). Direkte Nutzung des bereits
in A.1 verifizierten Feldes, keine neue Annahme.

## Entscheidung

**Option B.**

- **Neue Tabelle `profile_offcuts`**: `id`, `material_id` (FK, Prüfung im
  Anwendungscode: `materials.profilserie` muss gesetzt sein), `warehouse_id`
  (FK, Scope-Vererbung wie `stock_movements`/`stock_reservations`/
  `inventories` — kein eigenes `company_id`), `location_id` (FK, nullable,
  wie bei den anderen Lagertabellen), `length_mm numeric(18,6) NOT NULL
  CHECK (length_mm > 0)` (Länge DIESES Einzelstücks, nicht die
  Artikel-Standardlänge), `status text DEFAULT 'verfügbar' CHECK (status
  IN ('verfügbar', 'verbraucht'))`, `source_offcut_id` (FK auf dieselbe
  Tabelle, nullable — gesetzt, wenn dieses Stück beim Verbrauch eines
  anderen, registrierten Reststücks entstanden ist; NULL, wenn es direkt
  beim Ablängen eines frischen, nicht separat erfassten Stabs entstand),
  `note text DEFAULT ''` (z. B. Projekt-/Auftragsbezug, freier Text wie
  bei `stock_movements.reason`), `created_at timestamptz DEFAULT now()`,
  `consumed_at timestamptz` (NULL bis Verbrauch), `used_length_mm
  numeric(18,6)` (NULL bis Verbrauch, dann die tatsächlich entnommene
  Länge).
- **`RegisterOffcut`**: legt ein neues, `verfügbar`es Reststück an. Prüft
  Material-/Lager-Zugehörigkeit zum Mandanten UND dass
  `materials.profilserie` gesetzt ist ("Material ist kein Profil").
- **`ListOffcuts`**: Filter nach Material/Lager/Status UND Mindestlänge
  (`length_mm >= ?`) — das ist die eigentliche fachliche Kernfunktion
  ("finde ein verfügbares Reststück von Material X mit mindestens Y mm"),
  ohne die C.3 nur eine Ablage, aber kein Verschnitt-Management wäre.
- **`ConsumeOffcut`**: nimmt `used_length_mm` entgegen, lehnt ab, wenn
  `used_length_mm > length_mm` des Zielstücks oder das Stück bereits
  `verbraucht` ist. Setzt das Zielstück auf `status='verbraucht'`,
  `consumed_at=now()`, `used_length_mm=<Wert>` (Werte bleiben stehen,
  keine Löschung — Nachvollziehbarkeit). Ist `length_mm - used_length_mm
  > 0`, wird transaktional EIN neues `profile_offcuts`-Stück mit dieser
  Restlänge angelegt (`source_offcut_id` = verbrauchtes Stück,
  `status='verfügbar'`) — der Nutzer entscheidet in einem Folgeschritt
  (erneutes `ConsumeOffcut` oder schlicht Nichtbeachtung in `ListOffcuts`-
  Suchen), ob dieser neue, kleinere Rest noch brauchbar ist; das System
  bewertet die Länge nicht (siehe Optionen).
- **Keine Kopplung an `stock_movements`/`stock_reservations`/
  `inventories`** — Profil-Reststücke sind eine eigenständige, stückzahl-
  basierte Nebenbuchführung neben der mengenbasierten Hauptbuchführung;
  keine der drei bestehenden Tabellen wird verändert oder gelesen.
- **Implementierung in `materials.Service`** (kein neues Paket) — exakt
  der bestehende Ort für alles Lager-/Bestandsbezogene.
- **Keine neue Permission-Infrastruktur** — Wiederverwendung von
  `stock_movements.read`/`stock_movements.write`, analog zu C.1/C.2.

## Konsequenzen

- **C.3.2** (Folge-Subtask): additive Migration `077_profile_offcuts.sql`
  (eine neue Tabelle, keine bestehende Tabelle geändert), reversibel.
- **C.3.3** (Folge-Subtask): Anwendungscode nur in `materials/service.go`
  (`RegisterOffcut`/`ListOffcuts`/`ConsumeOffcut`), HTTP-Wiring, Tests.
  `CreateMovement`/`StockByMaterial`/`CreateReservation`/`StartInventory`
  bleiben komplett unangetastet.
- Neue, mögliche künftige Backlog-Positionen (nicht Teil von C.3):
  automatische Verschnittoptimierung (bestmögliche Zuschnittkombination
  aus verfügbaren Reststücken vorschlagen), konfigurierbare
  Mindestlänge je Mandant/Material, Verknüpfung von Reststücken mit
  konkreten Projekt-/Auftragspositionen.
- Kein Einfluss auf bestehende `stock_movements`-/`stock_reservations`-/
  `inventories`-Daten oder deren Lesepfade.
