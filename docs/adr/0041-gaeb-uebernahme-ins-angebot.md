# ADR 0041: Übernahme eines GAEB-Imports ins Angebot

- Status: Entwurf (2026-11-16)
- Bezug: Backlog I.2.4, ADR 0040 (Entscheidung 4 (a): Gliederung, Kurz-/Langtext
  und Art als eigener Teilpunkt von I.2), ADR 0009 (LV-Hierarchie im Angebot),
  Backlog I.6 (D84-Rückschreibung braucht die OZ je Angebotsposition)

## Bestandsaufnahme

- **Übernahme heute** (`ApplyImportToDraftQuote`, `quotes/imports.go`): nur
  angenommene Positionen, je Position Text (`description` = Kurz- und
  Langtext), Menge, Einheit, Preis 0. Verknüpfung Importposition ↔
  Angebotsposition in `quote_import_item_links`. Gliederung, OZ, Art,
  Pauschale und Vorbemerkungen gehen verloren.
- **Gliederung im Angebot** (`quote_item_groups`, Migration 071, ADR 0009):
  Stufen `los`, `titel`, `untertitel`, Bezeichnung Pflicht, keine
  Ordnungszahl. Regeln im Code (`validateGroupHierarchy`): Los nie mit
  Eltern, Titel nur oben oder unter Los, **Untertitel nur unter Titel**.
  `quote_items.group_id` (SET NULL). Lesen über `GET /quotes/{id}/item-tree`;
  der Client nutzt den Baum noch nicht, das PDF ist flach.
- **GAEB ist tiefer:** der Parser erlaubt bis zu 5 Stufen von Bereichen; ab der
  dritten Stufe ohne Los bzw. der vierten mit Los entstehen Untertitel unter
  Untertiteln. Im Korpus kommt das nicht vor (höchstens Los/Titel/Untertitel).
- **Angebotspositionen** (`quote_items`, 032 ff.): Position (laufende Nummer),
  Text, Menge, Einheit, Preise, Steuer, Material, Gruppe. Keine OZ, kein
  Kurz-/Langtext, keine Art, keine Pauschale.
- **Summen:** `calcTotals` und zwei SQL-Neuberechnungen (`service.go`)
  summieren **alle** Positionen. Das PDF (`renderQuoteItemsTable`) zeigt alle
  Positionen in einer Tabelle mit laufender Nummer.
- **Auftrag aus Angebot** (`sales/service.go`): übernimmt alle Positionen
  (Text, Menge, Einheit, Preis, Steuer).
- **Fund Revision:** `Revise` kopiert Positionen **ohne** `group_id` und legt
  keine Gruppen an — eine Revision verliert die Gliederung schon heute (bei
  von Hand angelegten Gruppen). Mit der GAEB-Übernahme träfe das jedes
  importierte LV.

## Zu entscheiden

1. **Tiefere Gliederung.** (a) Untertitel dürfen auch unter Untertiteln stehen
   (Regel aus ADR 0009 erweitert, nur Anwendungscode, keine Migration);
   (b) tiefere Stufen werden in den nächsten Untertitel zusammengezogen
   (Bezeichnung mit OZ-Präfix); (c) Übernahme lehnt tiefere LVs ab.
   Empfehlung: **(a)** — GAEB erlaubt die Tiefe, die Gliederung bleibt
   vollständig.
2. **Ordnungszahl.** (a) Neue Spalte `outline_no` an `quote_item_groups` und
   `quote_items` (Migration 117); leere Bezeichnung einer Gruppe wird durch die
   OZ ersetzt; (b) OZ nur in der Bezeichnung bzw. im Text; (c) OZ nur über
   `quote_import_item_links`. Empfehlung: **(a)** — I.6 (D84) braucht die OZ je
   Position, auch nach einer Revision, und die Verknüpfung zeigt nur auf die
   erste Version.
3. **Kurz-/Langtext, Art, Pauschale an der Angebotsposition.** (a) Neue Spalten
   an `quote_items` (Kurztext, Langtext, Art mit Standard `normal`, Pauschale,
   Grundposition als Verweis auf die Angebotsposition), `description` bleibt
   (Kurz- und Langtext) für PDF, Auftrag und bestehende Oberflächen;
   (b) nichts davon am Angebot, nur am Import. Empfehlung: **(a)**.
4. **Summen.** (a) Alternativ- und Bedarfspositionen (mit und ohne
   Gesamtbetrag) zählen **nicht** zur Angebotssumme, bekommen aber Preis und
   Gesamtbetrag; das PDF kennzeichnet sie („Alternative zu …“, „Bedarf“) und
   weist sie getrennt aus; (b) alles zählt wie heute, nur Kennzeichnung;
   (c) je Position ein änderbares Kennzeichen „in der Summe“. Empfehlung:
   **(a)** — so verlangt es das LV; eine Summe mit Alternativen wäre zu hoch.
5. **Auftrag aus Angebot.** (a) Normal- und Grundpositionen werden übernommen,
   Alternativ- und Bedarfspositionen nicht; die Auswahl bei der Beauftragung
   (Alternative statt Grundposition, beauftragte Bedarfsposition) wird ein
   eigener Backlog-Punkt; (b) alle Positionen mit Kennzeichen übernehmen
   (braucht dieselben Spalten am Auftrag); (c) Auswahl gleich in I.2.4.
   Empfehlung: **(a)** — gleiche Summe wie im Angebot, kleinster Eingriff.
6. **Revision.** (a) `Revise` kopiert Gruppen und alle neuen Felder mit (behebt
   den Fund); (b) bleibt wie heute. Empfehlung: **(a)**.

Weiter gilt: Vorbemerkungen bleiben am Importlauf; ins Angebot (Notiz oder
Textbaustein) gehen sie erst mit einem eigenen Punkt.

## Vorschlag zur Umsetzung (bei Empfehlung)

- **I.2.4a** Migration 117 (Spalten an `quote_item_groups` und `quote_items`,
  DOWN-Pfad, Datenverlusthinweis) und Übernahme: Gruppen aus
  `quote_import_groups` in Reihenfolge, Positionen mit Gruppe, OZ, Texten, Art,
  Pauschale, Grundposition; Hierarchie-Regel nach Frage 1.
- **I.2.4b** Summen und PDF nach Frage 4 (`calcTotals`, SQL-Neuberechnungen,
  Kennzeichnung im PDF).
- **I.2.4c** Revision kopiert Gruppen und Felder; Auftrag aus Angebot nach
  Frage 5.

## Folgen

- Bestehende Angebote bleiben unverändert (Art `normal`, keine OZ).
- Die Summenregel betrifft nur Angebote mit Alternativ- oder
  Bedarfspositionen, also nur übernommene GAEB-LVs.
