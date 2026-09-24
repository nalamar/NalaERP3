# ADR 0022 — E-Rechnung Ausgang (XRechnung + ZUGFeRD 2.x)

Datum: 2026-09-09
Status: entschieden (Subtask E.4.1)
Bezug: Backlog E.4 (Epic E — Finanzwesen, vierte Task, nach E.1/E.2/E.3).

## Kontext

`aufgabe.md` §2 fordert fix: "E-Rechnung: Ausgang **XRechnung und ZUGFeRD
2.x**, Eingang mindestens Parsen." E.4 deckt den Ausgang ab (Eingang ist
E.5). Datenbasis: `invoices_out`/`invoice_out_items` (`accounting/ar.go`,
seit 018/033/036/062/074) — Rechnungskopf (Nummer, Status, Typ, Kontakt,
Datum, Fälligkeit, Währung, Netto/Steuer/Brutto, Storno-Felder) und
-positionen (Beschreibung, Menge, Einzelpreis, Steuerkennzeichen,
Konto). `contacts`/`contact_addresses` (003) für den Käufer,
`company_profiles` (025, inkl. der in E.3 ergänzten DATEV-Felder) für den
Verkäufer.

**Fehlende Recherchegrundlage im Repo, geschlossen per Websuche** (analog
zu ADR 0021/E.3.1 — auch hier gilt: eine falsch geratene Struktur würde
eine für Empfänger-Systeme unbrauchbare oder rechtlich angreifbare Datei
erzeugen): Es gibt keine lokale Kopie der EN-16931-/XRechnung-/ZUGFeRD-
Spezifikation. Recherchiert und gegen eine autoritative Primärquelle
verifiziert: das offizielle Testsuite-Repository
[`itplr-kosit/xrechnung-testsuite`](https://github.com/itplr-kosit/xrechnung-testsuite)
(gepflegt von KoSIT, der für die deutsche XRechnung-Spezifikation
zuständigen Stelle) — daraus zwei vollständige, für denselben
Geschäftsvorfall deckungsgleiche Beispieldateien geladen:
`01.01a-INVOICE_ubl.xml` (UBL-2.1-Syntax) und
`01.01a-INVOICE_uncefact.xml` (CII-Syntax), beide mit
`CustomizationID = urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_3.0`
(aktuelle Version: **XRechnung 3.0**) und
`ProfileID = urn:fdc:peppol.eu:2017:poacc:billing:01:1.0` (Peppol BIS
Billing 3.0, das von XRechnung verwendete Basisprofil).

**Wichtigster Recherchebefund**: XRechnung ist syntaxoffen — es erlaubt
sowohl UBL 2.1 als auch UN/CEFACT Cross Industry Invoice (CII) als
gleichwertige, standardkonforme Ausdrucksformen desselben EN-16931-
Datenmodells. ZUGFeRD 2.x (technisch identisch mit dem internationalen
Factur-X-Standard) hingegen **verlangt zwingend CII** (in ein PDF/A-3
eingebettet). Da beide vom Aufgabentext geforderten Formate dasselbe
Quelldatenmodell (`invoices_out`) abbilden müssen, folgt daraus eine
zentrale Design-Entscheidung: **CII als einzige zu implementierende
XML-Syntax** — sie deckt XRechnung (als eigenständige, unquotierte
CII-Datei, für Empfänger, die CII-Syntax akzeptieren — laut o. g.
Testsuite ausdrücklich ein offiziell unterstützter, gleichwertiger Weg)
UND ZUGFeRD (dieselbe CII-Datei, eingebettet in die bereits bestehende
Rechnungs-PDF) mit EINER Mapping-/Serialisierungslogik ab, statt UBL und
CII parallel zu pflegen.

**Zweiter wichtiger Recherchebefund (Werkzeug-Grenze, kein
Formatproblem)**: `server/internal/pdfgen` nutzt `github.com/jung-kurt/gofpdf`
(bereits Projektabhängigkeit). Der Quellcode
(`gofpdf@v1.16.2/attachments.go`) bestätigt eingebettete Dateianhänge
über `Fpdf.SetAttachments([]Attachment)` (schreibt ein PDF-`/EmbeddedFile`-
Objekt plus `/Filespec`-Eintrag) — das technische Grundwerkzeug für
ZUGFeRD ist also vorhanden. Was `gofpdf` NICHT anbietet: `/AFRelationship`
auf dem Filespec, XMP-Metadaten-Stream, `/OutputIntent` mit eingebettetem
ICC-Profil und die übrigen ISO-19005-3(PDF/A-3)-Konformitätsanforderungen
— echte, formale PDF/A-3-Zertifizierungskonformität ist mit der
aktuellen PDF-Erzeugungs-Infrastruktur NICHT erreichbar, ohne die
PDF-Engine grundlegend zu ersetzen (out of scope für E.4). **Bewusste,
offengelegte Einschränkung** (kein verschwiegener Workaround, siehe
`docs/open-questions.md`): das Ergebnis ist eine PDF mit eingebetteter,
inhaltlich vollständiger ZUGFeRD-/Factur-X-CII-XML als benannter Anhang
(`factur-x.xml`) — von den meisten E-Rechnungs-Extraktionswerkzeugen
lesbar —, aber ohne formale ISO-PDF/A-3-Zertifizierung.

**Dritter Recherchebefund — Datenlücken im bestehenden Modell**:
- **BuyerReference (BT-10, Pflichtfeld laut o. g. Beispieldatei UND
  EN-16931-Kernregel)** existiert in keiner Tabelle. Erfüllt bei
  B2G-Rechnungen die gesetzlich vorgeschriebene Leitweg-ID, bei
  B2B-Rechnungen typischerweise eine Kundenreferenz/Bestellnummer — in
  beiden Fällen ein Wert, den nur der Rechnungsersteller je Rechnung
  kennt, keine ableitbare Stammdaten-Eigenschaft des Kontakts.
- **Käufer-Postanschrift**: Der bestehende `GET /invoices-out/{id}/pdf`-
  Handler (`v1.go`) übergibt an `pdfgen.InvoiceOutData` aktuell NUR
  `ContactName`/`ContactID` — KEINE Adressfelder (überraschender, aber
  bestätigter Bestand: die heutige PDF-Rechnung druckt gar keine
  Käuferadresse). Für CII ist die Käuferadresse (`BuyerTradeParty` /
  `PostalTradeAddress`) aber ein EN-16931-Pflichtblock (BG-8) — eine neue
  Abfrage gegen `contact_addresses` ist nötig (Präferenz `art='billing'`,
  sonst `is_primary=true`, sonst irgendeine vorhandene Adresse; keine
  Adresse vorhanden → Export wird mit klarer Fehlermeldung abgelehnt,
  keine erfundene Adresse).
- **Mengeneinheit je Rechnungsposition**: `invoice_out_items` hat kein
  `unit`-Feld (anders als z. B. `materials`/`quote_items`). CII verlangt
  einen `unitCode` (BT-130, UN/ECE-Rec.-20-Code) je Position. Entscheidung:
  fester Fallback-Code `"C62"` ("Stück/Eins", UN/ECE-Rec.-20-Standardcode
  für "nicht näher spezifizierte Einheit") für ALLE Positionen, da
  `invoice_out_items` aktuell keine Einheit trackt — bewusste, offengelegte
  Vereinfachung (keine neue `unit`-Spalte in dieser Task, das wäre ein
  eigenständiges, größeres Rechnungs-Datenmodell-Feature, siehe
  Konsequenzen).

## Optionen

**Syntax**: CII (gewählt, siehe oben) vs. UBL (verworfen — würde ZUGFeRD
nicht abdecken, zwei parallele Serialisierer nötig) vs. beide parallel
(verworfen — unverhältnismäßiger Aufwand für denselben Informationsgehalt,
kein Mehrwert laut Recherche: beide Syntaxen sind für XRechnung
gleichwertig zugelassen).

**Steuerkategorie-Zuordnung** (UNTDID-5305-Code je `ClassifiedTaxCategory`/
`ApplicableTradeTax`, aus den bestehenden `tax_codes` abgeleitet — keine
neue Stammdatenpflege): `rate > 0` → `"S"` (Standard rate); `rate = 0 AND
reverse_charge = true` → `"AE"` (VAT Reverse Charge); `rate = 0 AND
reverse_charge = false` → `"E"` (Exempt from tax, mit fester
Befreiungsbegründung "Steuerbefreit"). Deckt alle vier bestehenden
Steuerkennzeichen (`DE19`, `DE7`, `DE0`, `EU-RC`, `RC`) ab, ohne die
`tax_codes`-Tabelle selbst zu ändern.

**Rechnungstyp-Code (BT-3, UNTDID 1001)**: `invoices_out.invoice_type`
(B.4.2, seit 074) wird direkt abgebildet — `'abschlagsrechnung'` → `386`
(Prepayment invoice, exakte semantische Entsprechung); `'rechnung'`/
`'schlussrechnung'` → `380` (Commercial invoice). Storno-Rechnungen
(`status='storniert'`) werden vom Export ausgeschlossen (siehe
Entscheidung unten) — ein echter Rechnungskorrektur-Typ (`381` Credit
note) ist nicht Teil dieser Task.

**Exportierbarer Status**: nur `status IN ('booked','paid')` (analog zur
bereits etablierten GoBD-Festschreibung aus Epic 0.3 — ein `draft` ist per
Definition noch änderbar und darf nicht als rechtsverbindliche E-Rechnung
das Haus verlassen; ein `storniert`es Dokument ist keine gültige neue
Rechnung mehr). `draft`/`storniert` → Export wird mit klarer
Fehlermeldung abgelehnt.

**BuyerReference-Quelle**: neue Spalte auf `invoices_out` (gewählt, da
Rechnungs-spezifisch, siehe Kontext) vs. Feld auf `contacts` (verworfen —
eine Leitweg-ID/Kundenreferenz ist typischerweise je Rechnung/Vorgang
unterschiedlich, keine stabile Stammdaten-Eigenschaft des Kontakts) vs.
Pflichtfeld bei JEDER Rechnungserstellung erzwingen (verworfen — würde
ALLE bisherigen Rechnungs-Erstellungspfade brechen bzw. eine
rückwirkende Datenmigration für Bestandsrechnungen erfordern; stattdessen
nullable, mit klarer Fehlermeldung erst beim tatsächlichen
E-Rechnungs-Export, wenn der Wert fehlt — analog zur bereits etablierten
"Pflicht-Prüfung erst bei Verwendung, nicht bei Anlage"-Konvention aus
E.3 für DATEV-Beraternummer/Mandantennummer).

## Entscheidung

- **CII (UN/CEFACT Cross Industry Invoice) als alleinige XML-Syntax**,
  für XRechnung als eigenständige `.xml`-Datei und für ZUGFeRD 2.x als
  in die bestehende Rechnungs-PDF eingebetteter Anhang (`factur-x.xml`,
  via `gofpdf.SetAttachments`) — EINE Mapping-Logik für beide
  geforderten Formate.
  - **CustomizationID**: `urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_3.0`
    (XRechnung 3.0, für beide Ausgabewege identisch — ZUGFeRD 2.x ist
    seinerseits EN-16931-konform und verwendet für den
    XRechnung-kompatiblen Teil dieselbe Kennung).
  - **ProfileID**: `urn:fdc:peppol.eu:2017:poacc:billing:01:1.0`.
  - Steuerkategorie- und Rechnungstyp-Mapping wie oben entschieden.
- **Offengelegte Einschränkung**: ZUGFeRD-Ausgabe ist eine PDF mit
  eingebetteter, vollständiger CII-XML, OHNE formale ISO-19005-3(PDF/A-3)-
  Zertifizierungskonformität (Werkzeuggrenze von `gofpdf`, siehe Kontext).
  Als offene Frage in `docs/open-questions.md` vermerkt.
- **Neue additive Spalte `invoices_out.buyer_reference`** (nullable text),
  Pflicht-Prüfung erst beim tatsächlichen E-Rechnungs-Export.
- **Käufer-Postanschrift** wird zum Exportzeitpunkt frisch aus
  `contact_addresses` aufgelöst (Präferenz `billing` > `is_primary` >
  irgendeine vorhandene) — keine Änderung an der bestehenden
  `GET /invoices-out/{id}/pdf`-Logik (die bleibt wie sie ist, das ist ein
  bestehendes, hier nicht zu behebendes Verhalten, siehe
  `docs/open-questions.md`).
- **Mengeneinheit** fest `"C62"` für alle Positionen (dokumentierte
  Vereinfachung, siehe Kontext).
- **Export nur für `status IN ('booked','paid')`**, `draft`/`storniert`
  werden mit klarer Fehlermeldung abgelehnt.
- **Neue Endpunkte**, beide mit der bereits bestehenden
  `invoices_out.read`-Permission (kein neues Recht nötig — Charakter
  entspricht dem bereits vorhandenen `/pdf`-Export dieser Rechnung, keine
  neue, sensiblere Fähigkeit wie beim DATEV-Ledger-Export):
  - `GET /invoices-out/{id}/xrechnung` → `application/xml`, Dateiname
    `xrechnung_<Nummer>.xml`.
  - `GET /invoices-out/{id}/zugferd` → `application/pdf`, Dateiname
    `zugferd_<Nummer>.pdf` (bestehende PDF-Renderpipeline +
    eingebettete CII-XML).

## Konsequenzen

- **E.4.2** (Folge-Subtask): additive Migration `invoices_out.buyer_reference`
  (nullable, reversibel).
- **E.4.3** (Folge-Subtask): Anwendungscode — CII-XML-Builder,
  DB-Abfrage/Mapping (inkl. Käuferadress-Auflösung, Steuerkategorie-/
  Rechnungstyp-Mapping), HTTP-Wiring (XRechnung-Endpunkt + ZUGFeRD-PDF-
  Embedding-Endpunkt), Tests. Umfang (verschachtelte CII-Struktur mit
  mehreren Ebenen, zwei Endpunkte, PDF-Embedding-Logik, mehrere
  Testfälle inkl. Negativfälle) wird voraussichtlich §6.3 überschreiten
  — bei Erreichen von E.4.3 in Micro-Subtasks zerlegen (analog D.3.3/
  E.2.3/E.3.3): CII-XML-Builder (DB-los) → DB-Abfrage/Mapping →
  XRechnung-HTTP-Wiring → ZUGFeRD-PDF-Embedding-HTTP-Wiring.
- **Bekannte, bewusst nicht abgedeckte Fälle** (dokumentiert, kein
  Blocker): echte Rechnungskorrektur-E-Rechnungen (UNTDID-1001-Typ `381`
  Credit note) für stornierte Rechnungen; UBL-Syntax-Ausgabe (CII deckt
  laut Recherche denselben Empfängerkreis ab); formale PDF/A-3-
  Zertifizierung; Mengeneinheiten-Tracking je Rechnungsposition
  (`invoice_out_items.unit`, eigenständiges künftiges Feature); Peppol-
  Netzwerk-Versand (nur Datei-Erzeugung, kein Netzwerk-Transport ist
  Teil von "Ausgang" laut aufgabe.md-Wortlaut).
- Kein Einfluss auf bestehende `invoices_out`/`invoice_out_items`/
  `contacts`/`contact_addresses`/`company_profiles`-Daten oder deren
  Lesepfade (rein additive/lesende Erweiterung, die neue Spalte ist
  nullable).
