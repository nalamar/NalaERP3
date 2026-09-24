# ADR 0023 — E-Rechnung Eingang (Parsen von XRechnung und ZUGFeRD)

Datum: 2026-09-24
Status: entschieden (Subtask E.5.1)
Bezug: Backlog E.5 (Epic E — Finanzwesen, letzte Task, nach E.1-E.4).

## Kontext

`aufgabe.md` §2 fordert: "E-Rechnung: Ausgang **XRechnung und ZUGFeRD
2.x**, Eingang **mindestens Parsen**." Der Ausgang ist mit Task E.4
vollständig abgeschlossen (ADR 0022). E.5 deckt den Eingang ab.

Datenbasis für die Zielseite: `invoices_in`/`invoice_in_items`
(`accounting/ap.go`, seit Migration 080/ADR 0018) — Kopf (Lieferant,
Bestellbezug, Rechnungsnummer, Datum, Währung, Status, Notiz) und
Positionen (Bestellpositionsbezug, Bezeichnung, Menge, Preis, Währung).

**Entscheidender Unterschied zum Ausgang**: Beim Ausgang bestimmen WIR
die Syntax — deshalb konnte ADR 0022 sich auf CII allein festlegen und
UBL sparen. Beim Eingang bestimmt der **Absender** die Syntax. Wir können
nicht vorschreiben, was ein Lieferant schickt.

**Recherche (Primärquelle, wie in ADR 0021/0022)**: Aus dem offiziellen
KoSIT-Testsuite-Repository `itplr-kosit/xrechnung-testsuite` wurde für
denselben Geschäftsvorfall (01.01a) sowohl die CII- als auch die
UBL-Variante im Rohtext geladen und verglichen. Beide tragen dieselbe
`CustomizationID`
(`urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_3.0`)
und dieselbe `ProfileID` — sie sind nachweislich gleichwertige,
gleichermaßen konforme XRechnungen. Die UBL-Struktur ist dabei eine
komplett andere: Wurzelelement `ubl:Invoice` (Namensraum
`urn:oasis:names:specification:ubl:schema:xsd:Invoice-2`) mit flachen
`cbc:`-Feldern (`cbc:ID`, `cbc:IssueDate`, `cbc:InvoiceTypeCode`,
`cbc:DocumentCurrencyCode`, `cbc:BuyerReference`) und `cac:`-Gruppen
(`cac:AccountingSupplierParty`, `cac:LegalMonetaryTotal` mit
`cbc:PayableAmount`) — kein einziges Element ist mit CII gemeinsam.

Daraus folgt unmittelbar: **ein Eingangsparser, der nur CII versteht,
würde einen erheblichen Teil vollkommen konformer XRechnungen ablehnen.**
Das wäre kein Randfall, sondern ein funktionaler Mangel.

**Werkzeuglücke ZUGFeRD-Eingang**: Eine ZUGFeRD-/Factur-X-Rechnung ist
eine PDF mit eingebetteter CII-XML. Das Projekt hat mit `gofpdf` einen
PDF-*Schreiber*, aber **keinen PDF-Leser** (`go.mod` geprüft: keine
einzige PDF-lesende Abhängigkeit). Ohne einen solchen ist der Anhang aus
einer fremden PDF nicht herauszulösen.

> **Ausdrückliche Warnung gegen eine naheliegende Scheinlösung**: In
> `zugferd_export_integration_test.go` (E.4.3.4) existiert bereits eine
> Funktion, die einen eingebetteten Anhang aus einer PDF herauslöst. Sie
> ist **nicht** wiederverwendbar: sie sucht naiv nach `/Type
> /EmbeddedFile` und einem unmittelbar folgenden, zlib-komprimierten
> Stream und funktioniert nur deshalb, weil sie PDFs prüft, die wir
> selbst mit `gofpdf` erzeugt haben. Fremde PDFs nutzen
> Objekt-Streams, komprimierte Cross-Reference-Streams, abweichende
> Filter oder Verschlüsselung. Diese Funktion auf Lieferanten-PDFs
> loszulassen hieße, einen Testhelfer als Produktivparser auszugeben.

## Optionen

**Syntaxumfang**
- Nur CII (verworfen — würde konforme UBL-XRechnungen ablehnen, siehe
  Kontext; die Ersparnis beim Ausgang lässt sich nicht übertragen).
- CII **und** UBL (gewählt).
- Zusätzlich ältere/andere Formate wie ZUGFeRD 1.0 oder EDIFACT
  (verworfen für E.5 — nicht von `aufgabe.md` gefordert, deutlich
  größerer Aufwand; bei Bedarf eigene Backlog-Position).

**ZUGFeRD-PDF-Eingang**
- Eigener PDF-Parser (verworfen — ein robuster PDF-Parser ist ein
  Projekt für sich; die naive Variante wäre ein als Lösung getarnter
  Workaround, siehe Warnung oben).
- Externe Bibliothek `pdfcpu` (gewählt). Geprüft statt angenommen:
  Lizenz **Apache-2.0** (nach §2 zulässig), aktiv gepflegt (Releases
  `v0.14.0` 2026-08-03, `v0.15.0` 2026-08-11, `v0.16.0-rc.1`
  2026-09-20), und die benötigte Funktion existiert tatsächlich mit
  passender Signatur — im Quellcode `pkg/api/attach.go` verifiziert:
  `ExtractAttachmentsRaw(c context.Context, rs io.ReadSeeker, outDir
  string, fileNames []string, conf *model.Configuration) ([]model.Attachment, error)`.
  Keine Micro-Dependency, sondern das etablierte PDF-Werkzeug im
  Go-Ökosystem.
- ZUGFeRD-Eingang ganz weglassen (verworfen — `aufgabe.md` nennt
  ZUGFeRD 2.x ausdrücklich; auch wenn "mindestens Parsen" sich auf den
  Eingang allgemein bezieht, wäre eine Eingangsverarbeitung, die an der
  in Deutschland verbreitetsten Form scheitert, praktisch wertlos).

**Verarbeitungstiefe**
- Parsen **und automatisch** `invoices_in` anlegen (verworfen, siehe
  Entscheidung).
- Parsen und strukturierte Vorschau inkl. Lieferanten-Zuordnungs-
  *Vorschlag* (gewählt).

## Entscheidung

1. **Beide EN-16931-Syntaxen werden gelesen: CII und UBL.** Das Format
   wird anhand des Wurzelelement-Namensraums erkannt, nicht anhand des
   Dateinamens oder der Dateiendung (beide sind beim Eingang nicht
   vertrauenswürdig). Unbekannte Wurzelelemente werden mit klarer
   Meldung abgelehnt.

2. **Gemeinsames Zielmodell.** Beide Parser liefern dieselbe interne
   Struktur (Rechnungsnummer, Datum, Fälligkeit, Währung, Typ-Code,
   Käuferreferenz, Verkäufer inkl. USt-IdNr./Anschrift, Käufer,
   Positionen, Steueraufschlüsselung, Summen), damit alles Nachgelagerte
   syntaxunabhängig bleibt.

3. **Beim Lesen wird über echte Namensräume gematcht, nicht über
   Präfixe.** Der in `einvoice_cii.go` (E.4.3.1) genutzte Trick,
   Präfixe fest in die Elementnamen zu schreiben, funktioniert
   ausschließlich beim SCHREIBEN. Die dortigen Structs sind für das
   Lesen **nicht** wiederverwendbar: ein Absender darf beliebige Präfixe
   wählen (`rsm:`, `ram:` sind Konvention, nicht Vorschrift). Die
   Eingangs-Structs werden daher mit vollqualifizierten
   `xml:"<namespace> <local>"`-Tags definiert.

4. **ZUGFeRD-PDF**: Anhang per `pdfcpu` extrahieren, dann durch
   denselben CII-Parser schicken. Neue Abhängigkeit
   `github.com/pdfcpu/pdfcpu` (Apache-2.0), begründet wie oben.

5. **Umfang von E.5 ist Parsen und Vorschau — KEINE automatische
   Anlage einer Eingangsrechnung.** Begründung: `invoices_in.supplier_id`
   ist ein Pflicht-Fremdschlüssel auf `contacts`. Ein automatisches
   Anlegen hieße, Stammdaten aus einer von außen zugestellten Datei zu
   erzeugen, oder die Rechnung einem per Textähnlichkeit geratenen
   Lieferanten zuzuordnen — beides ist in einem buchungsrelevanten
   Kontext nicht vertretbar. Der Endpunkt liefert stattdessen die
   geparsten Daten plus einen **Zuordnungsvorschlag** für den Lieferanten
   (Abgleich über USt-IdNr., ersatzweise Name) mit klarer Kennzeichnung,
   ob ein eindeutiger Treffer vorliegt. Die Übernahme nach `invoices_in`
   bestätigt ein Mensch. Das entspricht dem Human-in-the-Loop-Prinzip,
   das `aufgabe.md` §4 für die KI-Zuordnung fordert und das im
   GAEB-Import bereits durchgängig umgesetzt ist. Die eigentliche
   Übernahme wird als **neue Backlog-Position** geführt, nicht
   stillschweigend in E.5 hineingezogen.

6. **Keine Berechnungen aus geparsten Werten ableiten.** Beträge werden
   übernommen wie geliefert; es wird nichts nachgerechnet und nichts
   korrigiert. Weicht die Summe der Positionen von der gelieferten
   Gesamtsumme ab, wird das als Hinweis ausgewiesen, aber weder
   stillschweigend repariert noch als Fehler behandelt — es ist die
   Rechnung des Absenders.

7. **Eingangsdaten sind nicht vertrauenswürdig.** Es gilt eine
   Größenbegrenzung für den Upload, und der Parser darf keine externen
   Entitäten auflösen. Beides ist im umsetzenden Subtask durch einen
   Test **nachzuweisen** (bewusst als Nachweispflicht formuliert, nicht
   als Behauptung über das Verhalten von `encoding/xml`).

8. **Neuer Endpunkt** `POST /invoices-in/parse-e-invoice`
   (Multipart-Upload, Antwort JSON mit geparster Rechnung +
   Lieferantenvorschlag), Permission `invoices_in.write` — bestehende
   Permission, kein neues Recht: es entsteht ein Arbeitsergebnis im
   Erfassungsprozess für Eingangsrechnungen, genau der Zweck dieses
   Rechts. Bewusst `write` und nicht `read`, obwohl (noch) nichts
   persistiert wird: der Vorgang gehört zum Erfassen, nicht zum Ansehen.

## Konsequenzen

- **E.5.2**: gemeinsames Zielmodell + CII-Eingangsparser (DB-los),
  namensraumbasiert, inkl. Negativtests.
- **E.5.3**: UBL-Eingangsparser (DB-los) + Formaterkennung über den
  Wurzelelement-Namensraum, auf dasselbe Zielmodell.
- **E.5.4**: ZUGFeRD-PDF — Abhängigkeit `pdfcpu` aufnehmen und
  begründen, Anhang extrahieren, an den CII-Parser übergeben. Beim
  Umsetzen ist die exakte API der gepinnten Version erneut gegen den
  Quellcode zu prüfen (die Signatur hat sich zwischen Versionen
  geändert — die oben verifizierte Form stammt aus `master`).
- **E.5.5**: HTTP-Endpunkt inkl. Lieferanten-Zuordnungsvorschlag,
  Größenbegrenzung, Entitäten-Nachweis, Integrationstest.
- **Neue Backlog-Position** (nicht Teil von E.5): Übernahme einer
  geparsten Eingangsrechnung nach `invoices_in` inkl.
  Lieferantenbestätigung und Bestellzuordnung.
- **Offene Detailfrage** (kein Blocker, in `docs/open-questions.md`):
  welche Dateinamen der eingebettete Anhang tragen darf. Verifiziert ist
  nur `factur-x.xml` (Factur-X/ZUGFeRD 2.1+, von uns selbst geschrieben);
  ältere ZUGFeRD-Stände verwenden abweichende Namen. Statt eine Liste zu
  raten, wird beim Umsetzen von E.5.4 entschieden, ob der Anhang
  überhaupt nach Namen gesucht oder schlicht der erste Anhang genommen
  wird, der sich als EN-16931-XML parsen lässt.
- Bekannte, bewusst nicht abgedeckte Fälle: ZUGFeRD 1.0, EDIFACT,
  signierte/verschlüsselte PDFs, Rechnungskorrekturen (UNTDID-1001-Typ
  `381`) auf der Eingangsseite.
