package apihttp

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"nalaerp3/internal/accounting"
)

// buildXRechnungFile bündelt die für E.4.3.3 nötigen Schritte: Rechnung
// laden und auf das CII-Modell abbilden (E.4.3.2), daraus die CII-XML
// serialisieren (E.4.3.1), Dateinamen bestimmen. Reine Orchestrierung ohne
// eigene Geschäftslogik - dient dem
// GET /invoices-out/{id}/xrechnung-Handler in v1.go.
//
// Dieselbe XML wird in E.4.3.4 unverändert als factur-x.xml in die
// Rechnungs-PDF eingebettet (ZUGFeRD) - deshalb liefert diese Funktion die
// Bytes und nicht direkt eine HTTP-Antwort.
func buildXRechnungFile(
	ctx context.Context,
	invoiceID uuid.UUID,
	companyID string,
	eInvoiceSvc *accounting.EInvoiceService,
) (content []byte, filename string, err error) {
	model, err := eInvoiceSvc.BuildCIIInvoice(ctx, invoiceID, companyID)
	if err != nil {
		return nil, "", err
	}

	xml, err := accounting.BuildCrossIndustryInvoice(model)
	if err != nil {
		return nil, "", err
	}

	return xml, fmt.Sprintf("xrechnung_%s.xml", sanitizeFilenamePart(model.Number)), nil
}

// sanitizeFilenamePart entfernt aus der Rechnungsnummer alles, was in einem
// Content-Disposition-Dateinamen Probleme macht (Pfadtrenner,
// Anführungszeichen, Steuerzeichen). Rechnungsnummern sind über
// settings.NumberingService frei konfigurierbar, es gibt also keine Garantie
// für ein unkritisches Format.
func sanitizeFilenamePart(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "rechnung"
	}
	return out
}
