package apihttp

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"

	"nalaerp3/internal/accounting"
	"nalaerp3/internal/pdfgen"
	"nalaerp3/internal/settings"
)

// zugferdAttachmentName ist der von ZUGFeRD 2.x / Factur-X fest
// vorgegebene Dateiname der eingebetteten CII-XML.
const zugferdAttachmentName = "factur-x.xml"

// buildXRechnungFile bündelt die für E.4.3.3 nötigen Schritte: Rechnung
// laden und auf das CII-Modell abbilden (E.4.3.2), daraus die CII-XML
// serialisieren (E.4.3.1), Dateinamen bestimmen. Reine Orchestrierung ohne
// eigene Geschäftslogik - dient dem
// GET /invoices-out/{id}/xrechnung-Handler in v1.go.
//
// Dieselbe XML wird von buildZugferdFile unverändert als factur-x.xml in
// die Rechnungs-PDF eingebettet - deshalb liefert diese Funktion die Bytes
// und nicht direkt eine HTTP-Antwort.
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

	return xml, fmt.Sprintf("xrechnung_%s.xml", sanitizeFilename(model.Number)), nil
}

// buildInvoiceOutPDF rendert die Rechnungs-PDF. Wortgleich aus dem
// GET /invoices-out/{id}/pdf-Handler herausgezogen (Backlog E.4.3.4),
// damit der neue ZUGFeRD-Endpunkt dieselbe PDF erzeugen kann, statt rund
// 70 Zeilen Template-, Branding- und Mapping-Logik zu duplizieren.
// Verhalten unveraendert - der einzige Unterschied ist der zusaetzliche
// attachments-Parameter, den der bestehende PDF-Endpunkt mit nil belegt.
//
// Bewusst NICHT mitgeaendert: dass hier keine Kaeuferadresse an
// pdfgen.InvoiceOutData uebergeben wird (nur Name und ID). Das ist
// vorgefundenes Verhalten der gedruckten Rechnung, in ADR 0022
// dokumentiert und ausdruecklich nicht Teil dieser Task - die E-Rechnung
// loest die Adresse separat aus contact_addresses auf.
func buildInvoiceOutPDF(
	ctx context.Context,
	invoiceID uuid.UUID,
	companyID string,
	arSvc *accounting.ARService,
	pdfSvc *settings.PDFService,
	brandingSvc *settings.BrandingService,
	mg *mongo.Client,
	mongoDB string,
	attachments []pdfgen.Attachment,
) (content []byte, number string, err error) {
	inv, err := arSvc.Get(ctx, invoiceID, companyID)
	if err != nil {
		return nil, "", err
	}

	t, err := pdfSvc.Get(ctx, "invoice_out")
	if err != nil {
		return nil, "", err
	}
	effectiveTemplate := *t
	primaryColor := "#1F4B99"
	accentColor := "#6B7280"
	if branding, berr := brandingSvc.Get(ctx); berr == nil {
		effectiveTemplate = settings.ApplyBrandingDefaults(effectiveTemplate, branding)
		primaryColor = branding.PrimaryColor
		accentColor = branding.AccentColor
	}

	number = invoiceID.String()
	if inv.Number != nil && strings.TrimSpace(*inv.Number) != "" {
		number = *inv.Number
	}
	dueDate := ""
	if inv.DueDate != nil {
		dueDate = inv.DueDate.Format("02.01.2006")
	}
	data := pdfgen.InvoiceOutData{
		Number:      number,
		InvoiceDate: inv.InvoiceDate.Format("02.01.2006"),
		DueDate:     dueDate,
		Currency:    inv.Currency,
		Status:      inv.Status,
		ContactName: inv.ContactName,
		ContactID:   inv.ContactID,
		NetAmount:   inv.NetAmount,
		TaxAmount:   inv.TaxAmount,
		GrossAmount: inv.GrossAmount,
		PaidAmount:  inv.PaidAmount,
		Items:       make([]pdfgen.InvoiceOutItemData, 0, len(inv.Items)),
	}
	for idx, it := range inv.Items {
		data.Items = append(data.Items, pdfgen.InvoiceOutItemData{
			Pos:         idx + 1,
			Description: it.Description,
			Qty:         it.Qty,
			UnitPrice:   it.UnitPrice,
			TaxCode:     it.TaxCode,
			Currency:    inv.Currency,
		})
	}

	opts := pdfgen.TemplateOptions{
		HeaderText:   effectiveTemplate.HeaderText,
		FooterText:   effectiveTemplate.FooterText,
		TopFirstMM:   effectiveTemplate.TopFirstMM,
		TopOtherMM:   effectiveTemplate.TopOtherMM,
		PrimaryColor: primaryColor,
		AccentColor:  accentColor,
	}
	imgIDs := map[string]string{}
	if t.LogoDocID != nil {
		imgIDs["logo"] = *t.LogoDocID
	}
	if t.BgFirstDocID != nil {
		imgIDs["bg_first"] = *t.BgFirstDocID
	}
	if t.BgOtherDocID != nil {
		imgIDs["bg_other"] = *t.BgOtherDocID
	}

	pdfBytes, err := pdfgen.RenderInvoiceOutWithAttachments(ctx, mg, mongoDB, data, opts, imgIDs, attachments)
	if err != nil {
		return nil, "", err
	}
	return pdfBytes, number, nil
}

// buildZugferdFile erzeugt die ZUGFeRD-Ausgabe: dieselbe CII-XML wie der
// XRechnung-Endpunkt (E.4.3.3), eingebettet als factur-x.xml in die
// bestehende Rechnungs-PDF. Der Dateiname der Einbettung ist durch
// ZUGFeRD 2.x/Factur-X fest vorgegeben - Extraktionswerkzeuge suchen
// genau danach.
//
// Offengelegte Einschraenkung (ADR 0022): das Ergebnis ist eine PDF mit
// inhaltlich vollstaendiger, eingebetteter CII-XML, aber OHNE formale
// PDF/A-3-Konformitaet (gofpdf bietet kein XMP, kein OutputIntent, kein
// /AFRelationship). Es wird deshalb an keiner Stelle PDF/A behauptet.
func buildZugferdFile(
	ctx context.Context,
	invoiceID uuid.UUID,
	companyID string,
	eInvoiceSvc *accounting.EInvoiceService,
	arSvc *accounting.ARService,
	pdfSvc *settings.PDFService,
	brandingSvc *settings.BrandingService,
	mg *mongo.Client,
	mongoDB string,
) (content []byte, filename string, err error) {
	model, err := eInvoiceSvc.BuildCIIInvoice(ctx, invoiceID, companyID)
	if err != nil {
		return nil, "", err
	}
	xml, err := accounting.BuildCrossIndustryInvoice(model)
	if err != nil {
		return nil, "", err
	}

	pdfBytes, number, err := buildInvoiceOutPDF(ctx, invoiceID, companyID, arSvc, pdfSvc, brandingSvc, mg, mongoDB, []pdfgen.Attachment{{
		Filename:    zugferdAttachmentName,
		Description: "Rechnungsdaten im ZUGFeRD/Factur-X-Format (CII)",
		Content:     xml,
	}})
	if err != nil {
		return nil, "", err
	}

	return pdfBytes, fmt.Sprintf("zugferd_%s.pdf", sanitizeFilename(number)), nil
}
