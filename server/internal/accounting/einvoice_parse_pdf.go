package accounting

// E-Rechnung Eingang, ZUGFeRD-/Factur-X-PDF (ADR 0023, Backlog E.5.4).
//
// Eine ZUGFeRD-Rechnung ist eine PDF mit eingebetteter EN-16931-XML. Zum
// Lesen wird ein PDF-PARSER gebraucht - gofpdf (unsere Ausgangsseite)
// kann ausschliesslich schreiben und keine bestehende PDF oeffnen.
// Deshalb die Abhaengigkeit github.com/pdfcpu/pdfcpu (Apache-2.0, aktiv
// gepflegt, siehe ADR 0023 fuer die Begruendung).
//
// WARNUNG gegen eine naheliegende Scheinloesung: in
// internal/http/zugferd_export_integration_test.go gibt es eine Funktion,
// die einen Anhang aus einer PDF herausloest. Sie ist ein TESTHELFER und
// NICHT wiederverwendbar - sie sucht naiv nach "/Type /EmbeddedFile" und
// einem unmittelbar folgenden zlib-Strom und funktioniert nur, weil sie
// PDFs prueft, die wir selbst mit gofpdf erzeugt haben. Fremde PDFs
// nutzen Objekt-Streams, komprimierte Cross-Reference-Streams,
// abweichende Filter oder Verschluesselung.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// ZugferdAttachmentName ist der von ZUGFeRD 2.1+/Factur-X vorgegebene
// Dateiname der eingebetteten XML - derselbe, den unser eigener Ausgang
// schreibt (E.4.3.4). Er wird beim Lesen BEVORZUGT, aber nicht verlangt:
// siehe ExtractEInvoiceFromPDF.
const ZugferdAttachmentName = "factur-x.xml"

// maxEInvoiceAttachmentSize begrenzt, wie viel aus einem einzelnen
// PDF-Anhang gelesen wird. Der Anhang stammt von einem Dritten; ohne
// Grenze koennte eine praeparierte PDF den Speicher fuellen. 16 MiB ist
// fuer eine EN-16931-XML sehr grosszuegig (reale Rechnungen liegen im
// niedrigen dreistelligen Kilobyte-Bereich).
const maxEInvoiceAttachmentSize = 16 << 20

// ExtractEInvoiceFromPDF liest die in einer ZUGFeRD-/Factur-X-PDF
// eingebettete Rechnung und gibt sie geparst zurueck.
//
// **Auswahl des Anhangs** (die in docs/open-questions.md offengehaltene
// Detailfrage, hier entschieden): es wird NICHT auf einen festen
// Dateinamen bestanden. Verifiziert ist nur `factur-x.xml` (ZUGFeRD
// 2.1+/Factur-X); aeltere ZUGFeRD-Staende verwenden andere Namen, fuer
// die hier keine belastbare Quelle vorliegt - eine geratene Namensliste
// waere genau die Art Annahme, die ADR 0023 vermeiden will. Stattdessen:
// der Anhang mit dem bekannten Namen wird ZUERST versucht, danach alle
// uebrigen; der erste, der sich als EN-16931-XML lesen laesst, gewinnt.
// Das Parsen selbst ist die belastbarste Pruefung - ein Anhang, der als
// gueltige CII- oder UBL-Rechnung durchgeht, IST die Rechnung, unabhaengig
// davon, wie die Datei heisst.
func ExtractEInvoiceFromPDF(pdf []byte) (*ParsedEInvoice, error) {
	if len(pdf) == 0 {
		return nil, fmt.Errorf("E-Rechnung Eingang: leere PDF-Datei")
	}

	conf := model.NewDefaultConfiguration()
	// Fremde PDFs halten sich nicht immer streng an die Spezifikation.
	// Eine Rechnung wegen eines formalen PDF-Mangels abzulehnen, obwohl
	// die eingebetteten Daten einwandfrei sind, waere unverhaeltnismaessig.
	conf.ValidationMode = model.ValidationRelaxed

	attachments, err := api.ExtractAttachmentsRaw(bytes.NewReader(pdf), "", nil, conf)
	if err != nil {
		if isPdfcpuNoAttachmentsError(err) {
			return nil, errNoEInvoiceAttachment
		}
		return nil, fmt.Errorf("E-Rechnung Eingang: PDF konnte nicht gelesen werden: %w", err)
	}
	if len(attachments) == 0 {
		return nil, errNoEInvoiceAttachment
	}

	var (
		firstErr error
		versucht []string
	)
	for _, a := range eInvoiceAttachmentsInPreferredOrder(attachments) {
		data, err := readEInvoiceAttachment(a)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			versucht = append(versucht, a.FileName)
			continue
		}

		parsed, err := ParseEInvoiceXML(data)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			versucht = append(versucht, a.FileName)
			continue
		}
		return parsed, nil
	}

	return nil, fmt.Errorf(
		"E-Rechnung Eingang: keiner der %d eingebetteten Anhänge (%s) ist eine lesbare E-Rechnung: %w",
		len(attachments), strings.Join(versucht, ", "), firstErr)
}

// eInvoiceAttachmentsInPreferredOrder stellt den Anhang mit dem bekannten
// ZUGFeRD-Dateinamen nach vorn, behaelt aber alle uebrigen als Kandidaten.
func eInvoiceAttachmentsInPreferredOrder(attachments []model.Attachment) []model.Attachment {
	out := make([]model.Attachment, 0, len(attachments))
	for _, a := range attachments {
		if strings.EqualFold(strings.TrimSpace(a.FileName), ZugferdAttachmentName) {
			out = append(out, a)
		}
	}
	for _, a := range attachments {
		if !strings.EqualFold(strings.TrimSpace(a.FileName), ZugferdAttachmentName) {
			out = append(out, a)
		}
	}
	return out
}

// readEInvoiceAttachment liest den Inhalt eines Anhangs mit Groessengrenze.
func readEInvoiceAttachment(a model.Attachment) ([]byte, error) {
	if a.Reader == nil {
		return nil, fmt.Errorf("Anhang %q hat keinen Inhalt", a.FileName)
	}
	data, err := io.ReadAll(io.LimitReader(a.Reader, maxEInvoiceAttachmentSize+1))
	if err != nil {
		return nil, fmt.Errorf("Anhang %q konnte nicht gelesen werden: %w", a.FileName, err)
	}
	if len(data) > maxEInvoiceAttachmentSize {
		return nil, fmt.Errorf("Anhang %q ist größer als die zulässigen %d MiB", a.FileName, maxEInvoiceAttachmentSize>>20)
	}
	return data, nil
}

// errNoEInvoiceAttachment ist der Fall "PDF ohne eingebettete Datei" - die
// haeufigste Fehlbedienung (eine gewoehnliche Rechnungs-PDF statt einer
// ZUGFeRD-PDF) und deshalb mit eigener, verstaendlicher Meldung.
var errNoEInvoiceAttachment = errors.New("E-Rechnung Eingang: die PDF enthält keine eingebettete Rechnungsdatei - handelt es sich wirklich um eine ZUGFeRD-/Factur-X-Rechnung?")

// isPdfcpuNoAttachmentsError erkennt, dass pdfcpu eine PDF ganz ohne
// Anhaenge gemeldet hat.
//
// WORKAROUND, bewusst gekennzeichnet (§7.2): pdfcpu liefert fuer diesen
// Fall KEINEN typisierten Fehler, sondern
// errors.New("EmbeddedFiles name tree: no attachments available")
// (pkg/pdfcpu/model/attach.go). Ein Abgleich auf den Meldungstext ist die
// einzige Moeglichkeit, diesen Normalfall von einer echt kaputten PDF zu
// unterscheiden - ohne ihn bekaeme der Anwender fuer die haeufigste
// Fehlbedienung (eine gewoehnliche Rechnungs-PDF statt einer ZUGFeRD-PDF)
// eine englische Bibliotheksmeldung statt eines Hinweises.
//
// RECHERCHESTAND 2026-09-25 (Backlog E.7): die saubere Loesung waere ein
// Sentinel-Fehler in pdfcpu - den gibt es NICHT, auch nicht im aktuellen
// Upstream-Stand. Geprueft wurden alle veroeffentlichten Versionen bis
// v0.16.0-rc.1 sowie der master-Branch: dort existieren zwar Sentinels
// fuer ANDERE Anhang-Faelle (ErrNoAttachmentAdded, ErrNoAttachmentRemoved
// in pkg/api/attach.go), aber fuer "keine Anhaenge vorhanden" weiterhin
// nur das blanke errors.New. Eine Umstellung ist also nicht moeglich,
// nicht bloss noch nicht gemacht. Nicht erneut recherchieren, ohne dass
// sich Upstream bewegt hat.
//
// WER DIE pdfcpu-VERSION ANHEBT, muss hier nichts pruefen: ein
// geaenderter Meldungstext faellt sofort auf, weil
// TestExtractEInvoiceFromPDFRejectsBrokenInput/"PDF ohne Anhang" gegen
// die echte Bibliothek laeuft und dann fehlschlaegt. Das wurde
// experimentell bestaetigt (Abgleich absichtlich gebrochen -> Test rot).
// Die frueher an dieser Stelle stehende Annahme, ein Bruch bliebe
// unbemerkt, war falsch.
func isPdfcpuNoAttachmentsError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no attachments available")
}
