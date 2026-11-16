# ADR 0042: Auswahl von Alternativ- und Bedarfspositionen bei der Beauftragung

- Status: Entwurf (2026-11-16)
- Bezug: Backlog I.2.6, ADR 0041 (Frage 5 (a): Auftrag und Rechnung aus dem
  Angebot nur mit Normal- und Grundpositionen, Auswahl als eigener Punkt),
  ADR 0010 (Nachtragsmanagement)

## Bestandsaufnahme

- **Auftrag aus Angebot** (`sales.CreateFromQuote`, `POST
  /quotes/{id}/convert-to-sales-order`): ohne Eingaben, nur für angenommene
  Angebote, übernimmt seit I.2.4e2 nur Normal- und Grundpositionen (Text,
  Menge, Einheit, Preis, Steuer). Summe = Angebotssumme.
- **Rechnung aus Angebot** (`quotes.ConvertToInvoice`): ebenso nur Normal- und
  Grundpositionen; der Regelweg ist Rechnung aus Auftrag.
- **Auftragspositionen** (`sales_order_items`): Position, Text, Menge,
  Einheit, Preis, Steuer — keine OZ, keine Art, kein Bezug zur
  Angebotsposition.
- **Nachträge** (ADR 0010, `sales/addenda.go`): eigener Ablauf am Auftrag
  (Entwurf → beantragt → angenommen/abgelehnt) mit eigenen Positionen; die
  effektive Auftragssumme wird beim Lesen aus Grundauftrag und angenommenen
  Nachträgen berechnet. Positionen werden frei erfasst, ohne Vorbelegung.
- **Fachlich (GAEB):** über Alternativpositionen entscheidet der Auftraggeber
  meist bei der Vergabe (Wahl statt Grundposition); Bedarfspositionen werden
  erst während der Ausführung „auf Anordnung“ abgerufen, oft in anderer Menge.
- **Client:** die Umwandlung ist ein Knopf ohne Dialog.

## Zu entscheiden

1. **Zeitpunkt.** (a) Alternativen beim Umwandeln in den Auftrag wählen,
   Bedarfspositionen später als Abruf über einen Nachtrag; (b) beides beim
   Umwandeln; (c) beides nur über Nachträge. Empfehlung: **(a)** — entspricht
   dem Ablauf auf der Baustelle; der Nachtrag hat schon Freigabe und Summe.
2. **Wirkung einer gewählten Alternative.** (a) sie ersetzt ihre
   Grundposition (die Grundposition fällt im Auftrag weg), je Grundposition
   höchstens eine Alternative; (b) sie kommt zusätzlich dazu. Empfehlung:
   **(a)** — so ist die Wahlposition gemeint.
3. **Abruf einer Bedarfsposition.** (a) Nachtrag mit aus dem Angebot
   vorbelegter Position (Text, Einheit, Preis; Menge änderbar) und Bezug auf
   die Angebotsposition; mehrfacher Abruf erlaubt, die abgerufene Menge wird
   angezeigt; (b) freie Nachträge wie heute. Empfehlung: **(a)**.
4. **Herkunft am Auftrag.** (a) Auftrags- und Nachtragspositionen bekommen
   OZ und Bezug zur Angebotsposition (Migration 118); (b) nichts davon.
   Empfehlung: **(a)** — sonst ist nicht nachvollziehbar, welche Alternative
   beauftragt wurde.
5. **Rechnung direkt aus dem Angebot.** (a) bleibt ohne Auswahl (nur Normal-
   und Grundpositionen), wer auswählen will, geht über den Auftrag; (b)
   dieselbe Auswahl wie beim Auftrag. Empfehlung: **(a)** — kleinster
   Eingriff, der Auftrag ist der Regelweg.
6. **Angebot nach der Auswahl.** (a) bleibt unverändert, die Auswahl steht am
   Auftrag; (b) wird per Revision angepasst. Empfehlung: **(a)**.

## Vorschlag zur Umsetzung (bei Empfehlung)

- **I.2.6a** Migration 118 (`outline_no` und `source_quote_item_id` an
  `sales_order_items` und den Nachtragspositionen, DOWN-Pfad,
  Datenverlusthinweis) und Auswahl der Alternativen beim Umwandeln (Eingabe
  `alternatives` am Endpunkt, Prüfung: Alternative gehört zum Angebot, höchstens
  eine je Grundposition).
- **I.2.6b** Abruf einer Bedarfsposition als vorbelegter Nachtrag, abgerufene
  Menge je Bedarfsposition.
- **I.2.6c** Client: Auswahldialog beim Umwandeln, Abruf am Auftrag.

## Folgen

- Ohne Auswahl bleibt alles wie seit I.2.4e2.
- Die Auftragssumme kann von der Angebotssumme abweichen, wenn eine
  Alternative mit anderem Preis gewählt wird; das ist gewollt und sichtbar.
