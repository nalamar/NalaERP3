package pdfgen

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"strings"

	gofpdf "github.com/jung-kurt/gofpdf"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

// ImageSources enthält vorab geladene Bilder als Byte-Slices.
type ImageSources struct {
	Logo    []byte
	BgFirst []byte
	BgOther []byte
}

// TemplateOptions beschreibt die Template-bezogenen Einstellungen.
type TemplateOptions struct {
	HeaderText   string
	FooterText   string
	TopFirstMM   float64
	TopOtherMM   float64
	PrimaryColor string
	AccentColor  string
}

type rgbColor struct {
	R int
	G int
	B int
}

// PurchaseOrderData ist die für den Druck relevante Sicht auf eine Bestellung.
type PurchaseOrderData struct {
	Number    string
	OrderDate string // formatiert
	Currency  string
	Status    string
	Note      string
	// Supplier kann bei Bedarf erweitert werden (Name/Adresse)
	Items []PurchaseOrderItemData
}

type PurchaseOrderItemData struct {
	Pos         int
	Description string
	Qty         float64
	UOM         string
	UnitPrice   float64
	Currency    string
}

type InvoiceOutData struct {
	Number      string
	InvoiceDate string
	DueDate     string
	Currency    string
	Status      string
	ContactName string
	ContactID   string
	NetAmount   float64
	TaxAmount   float64
	GrossAmount float64
	PaidAmount  float64
	Items       []InvoiceOutItemData
}

type InvoiceOutItemData struct {
	Pos         int
	Description string
	Qty         float64
	UnitPrice   float64
	TaxCode     string
	Currency    string
}

type QuoteData struct {
	Number        string
	ProjectName   string
	ProjectStatus string
	CreatedDate   string
	ValidUntil    string
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	Currency      string
	Note          string
	NetAmount     float64
	TaxAmount     float64
	GrossAmount   float64
	Items         []QuoteItemData
}

type QuoteItemData struct {
	Pos             int
	PhaseLabel      string
	Description     string
	Qty             float64
	Unit            string
	UnitPrice       float64
	TaxCode         string
	LineTotal       float64
	DimensionsLabel string
	SurfaceLabel    string
}

// RenderPurchaseOrder rendert eine Bestellung als PDF unter Verwendung der Template-Optionen und Images.
// mg/dbName werden nur genutzt, wenn imageDocIDs gesetzt sind und images nicht vorab geladen wurden.
func RenderPurchaseOrder(ctx context.Context, mg *mongo.Client, dbName string, po PurchaseOrderData, tmpl TemplateOptions, imageDocIDs map[string]string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(fmt.Sprintf("Bestellung %s", po.Number), false)
	pdf.SetAuthor("NalaERP3", false)
	tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	primary, accent := templateColors(tmpl)

	// Bilder ggf. aus GridFS laden
	var imgs ImageSources
	var err error
	if imageDocIDs != nil {
		if id := strings.TrimSpace(imageDocIDs["logo"]); id != "" {
			imgs.Logo, _ = loadFromGridFS(ctx, mg, dbName, id)
		}
		if id := strings.TrimSpace(imageDocIDs["bg_first"]); id != "" {
			imgs.BgFirst, _ = loadFromGridFS(ctx, mg, dbName, id)
		}
		if id := strings.TrimSpace(imageDocIDs["bg_other"]); id != "" {
			imgs.BgOther, _ = loadFromGridFS(ctx, mg, dbName, id)
		}
	}

	// Header: Hintergrund/Logo/Kopftext + Top-Margin je Seite setzen
	pdf.SetHeaderFuncMode(func() {
		// Seitengröße
		w, h := pdf.GetPageSize()
		page := pdf.PageNo()

		// Hintergrund wählen
		if page == 1 {
			if len(imgs.BgFirst) > 0 {
				registerAndDrawImage(pdf, "bg_first", bytes.NewReader(imgs.BgFirst), 0, 0, w, h)
			} else if len(imgs.Logo) > 0 {
				// Logo oben links ca. 30mm breit, proportionale Höhe
				registerAndDrawImage(pdf, "logo", bytes.NewReader(imgs.Logo), 10, 10, 30, 0)
			}
			// Kopftext (falls kein Vollbild-Hintergrund)
			if len(imgs.BgFirst) == 0 && strings.TrimSpace(tmpl.HeaderText) != "" {
				pdf.SetFont("Helvetica", "", 9)
				pdf.SetTextColor(primary.R, primary.G, primary.B)
				pdf.SetXY(10, 10)
				pdf.MultiCell(w-20, 4.5, tr(tmpl.HeaderText), "", "R", false)
				pdf.SetTextColor(0, 0, 0)
			}
			// Start-Y für Inhalt
			pdf.SetY(tmpl.TopFirstMM)
		} else {
			if len(imgs.BgOther) > 0 {
				registerAndDrawImage(pdf, "bg_other", bytes.NewReader(imgs.BgOther), 0, 0, w, h)
			} else if len(imgs.Logo) > 0 {
				registerAndDrawImage(pdf, "logo", bytes.NewReader(imgs.Logo), 10, 10, 20, 0)
			}
			if len(imgs.BgOther) == 0 && strings.TrimSpace(tmpl.HeaderText) != "" {
				pdf.SetFont("Helvetica", "", 9)
				pdf.SetTextColor(primary.R, primary.G, primary.B)
				pdf.SetXY(10, 10)
				pdf.MultiCell(w-20, 4.5, tr(tmpl.HeaderText), "", "R", false)
				pdf.SetTextColor(0, 0, 0)
			}
			pdf.SetY(tmpl.TopOtherMM)
		}
	}, true)

	// Footer: Fußtext
	pdf.SetFooterFunc(func() {
		if strings.TrimSpace(tmpl.FooterText) == "" {
			return
		}
		_, h := pdf.GetPageSize()
		pdf.SetY(h - 15)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(accent.R, accent.G, accent.B)
		pdf.CellFormat(0, 5, tr(tmpl.FooterText), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})

	// Erste Seite anlegen (HeaderFunc zeichnet Hintergründe/Logo und setzt Y)
	pdf.AddPage()

	// Titel / Kopfdaten Bestellung
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(primary.R, primary.G, primary.B)
	pdf.CellFormat(0, 7, tr(fmt.Sprintf("Bestellung %s", po.Number)), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Datum: %s    Status: %s    Währung: %s", po.OrderDate, po.Status, po.Currency)), "", 1, "L", false, 0, "")
	if strings.TrimSpace(po.Note) != "" {
		pdf.SetFont("Helvetica", "", 9)
		pdf.MultiCell(0, 5, tr(po.Note), "", "L", false)
	}
	drawSectionRule(pdf, accent)

	// Abstand vor Tabelle
	pdf.Ln(3)

	// Items-Tabelle
	renderItemsTable(pdf, tr, po.Items, primary, accent)

	// Ausgeben
	var buf bytes.Buffer
	if err = pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func RenderInvoiceOut(ctx context.Context, mg *mongo.Client, dbName string, inv InvoiceOutData, tmpl TemplateOptions, imageDocIDs map[string]string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(fmt.Sprintf("Rechnung %s", inv.Number), false)
	pdf.SetAuthor("NalaERP3", false)
	tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	primary, accent := templateColors(tmpl)

	var imgs ImageSources
	var err error
	if imageDocIDs != nil {
		if id := strings.TrimSpace(imageDocIDs["logo"]); id != "" {
			imgs.Logo, _ = loadFromGridFS(ctx, mg, dbName, id)
		}
		if id := strings.TrimSpace(imageDocIDs["bg_first"]); id != "" {
			imgs.BgFirst, _ = loadFromGridFS(ctx, mg, dbName, id)
		}
		if id := strings.TrimSpace(imageDocIDs["bg_other"]); id != "" {
			imgs.BgOther, _ = loadFromGridFS(ctx, mg, dbName, id)
		}
	}

	pdf.SetHeaderFuncMode(func() {
		w, h := pdf.GetPageSize()
		page := pdf.PageNo()
		if page == 1 {
			if len(imgs.BgFirst) > 0 {
				registerAndDrawImage(pdf, "bg_first", bytes.NewReader(imgs.BgFirst), 0, 0, w, h)
			} else if len(imgs.Logo) > 0 {
				registerAndDrawImage(pdf, "logo", bytes.NewReader(imgs.Logo), 10, 10, 30, 0)
			}
			if len(imgs.BgFirst) == 0 && strings.TrimSpace(tmpl.HeaderText) != "" {
				pdf.SetFont("Helvetica", "", 9)
				pdf.SetTextColor(primary.R, primary.G, primary.B)
				pdf.SetXY(10, 10)
				pdf.MultiCell(w-20, 4.5, tr(tmpl.HeaderText), "", "R", false)
				pdf.SetTextColor(0, 0, 0)
			}
			pdf.SetY(tmpl.TopFirstMM)
		} else {
			if len(imgs.BgOther) > 0 {
				registerAndDrawImage(pdf, "bg_other", bytes.NewReader(imgs.BgOther), 0, 0, w, h)
			} else if len(imgs.Logo) > 0 {
				registerAndDrawImage(pdf, "logo", bytes.NewReader(imgs.Logo), 10, 10, 20, 0)
			}
			if len(imgs.BgOther) == 0 && strings.TrimSpace(tmpl.HeaderText) != "" {
				pdf.SetFont("Helvetica", "", 9)
				pdf.SetTextColor(primary.R, primary.G, primary.B)
				pdf.SetXY(10, 10)
				pdf.MultiCell(w-20, 4.5, tr(tmpl.HeaderText), "", "R", false)
				pdf.SetTextColor(0, 0, 0)
			}
			pdf.SetY(tmpl.TopOtherMM)
		}
	}, true)

	pdf.SetFooterFunc(func() {
		if strings.TrimSpace(tmpl.FooterText) == "" {
			return
		}
		_, h := pdf.GetPageSize()
		pdf.SetY(h - 15)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(accent.R, accent.G, accent.B)
		pdf.CellFormat(0, 5, tr(tmpl.FooterText), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})

	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(primary.R, primary.G, primary.B)
	pdf.CellFormat(0, 7, tr(fmt.Sprintf("Rechnung %s", inv.Number)), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Rechnungsdatum: %s    Status: %s    Währung: %s", inv.InvoiceDate, inv.Status, inv.Currency)), "", 1, "L", false, 0, "")
	if strings.TrimSpace(inv.DueDate) != "" {
		pdf.CellFormat(0, 6, tr("Fällig: "+inv.DueDate), "", 1, "L", false, 0, "")
	}
	if strings.TrimSpace(inv.ContactName) != "" || strings.TrimSpace(inv.ContactID) != "" {
		recipient := strings.TrimSpace(inv.ContactName)
		if recipient == "" {
			recipient = inv.ContactID
		}
		pdf.CellFormat(0, 6, tr("Kunde: "+recipient), "", 1, "L", false, 0, "")
	}

	pdf.Ln(3)
	drawSectionRule(pdf, accent)
	pdf.Ln(3)
	renderInvoiceItemsTable(pdf, tr, inv.Items, primary, accent)

	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(primary.R, primary.G, primary.B)
	pdf.CellFormat(0, 6, tr("Summen"), "", 1, "R", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(0, 5, tr("Netto: "+money(inv.NetAmount, inv.Currency)), "", 1, "R", false, 0, "")
	pdf.CellFormat(0, 5, tr("Steuer: "+money(inv.TaxAmount, inv.Currency)), "", 1, "R", false, 0, "")
	pdf.CellFormat(0, 5, tr("Brutto: "+money(inv.GrossAmount, inv.Currency)), "", 1, "R", false, 0, "")
	pdf.CellFormat(0, 5, tr("Bezahlt: "+money(inv.PaidAmount, inv.Currency)), "", 1, "R", false, 0, "")
	pdf.SetTextColor(accent.R, accent.G, accent.B)
	pdf.CellFormat(0, 5, tr("Offen: "+money(inv.GrossAmount-inv.PaidAmount, inv.Currency)), "", 1, "R", false, 0, "")
	pdf.SetTextColor(0, 0, 0)

	var buf bytes.Buffer
	if err = pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func RenderQuote(ctx context.Context, mg *mongo.Client, dbName string, quote QuoteData, tmpl TemplateOptions, imageDocIDs map[string]string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(fmt.Sprintf("Angebot %s", quote.Number), false)
	pdf.SetAuthor("NalaERP3", false)
	tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")
	primary, accent := templateColors(tmpl)

	var imgs ImageSources
	var err error
	if imageDocIDs != nil {
		if id := strings.TrimSpace(imageDocIDs["logo"]); id != "" {
			imgs.Logo, _ = loadFromGridFS(ctx, mg, dbName, id)
		}
		if id := strings.TrimSpace(imageDocIDs["bg_first"]); id != "" {
			imgs.BgFirst, _ = loadFromGridFS(ctx, mg, dbName, id)
		}
		if id := strings.TrimSpace(imageDocIDs["bg_other"]); id != "" {
			imgs.BgOther, _ = loadFromGridFS(ctx, mg, dbName, id)
		}
	}

	pdf.SetHeaderFuncMode(func() {
		w, h := pdf.GetPageSize()
		page := pdf.PageNo()
		if page == 1 {
			if len(imgs.BgFirst) > 0 {
				registerAndDrawImage(pdf, "bg_first", bytes.NewReader(imgs.BgFirst), 0, 0, w, h)
			} else if len(imgs.Logo) > 0 {
				registerAndDrawImage(pdf, "logo", bytes.NewReader(imgs.Logo), 10, 10, 30, 0)
			}
			if len(imgs.BgFirst) == 0 && strings.TrimSpace(tmpl.HeaderText) != "" {
				pdf.SetFont("Helvetica", "", 9)
				pdf.SetTextColor(primary.R, primary.G, primary.B)
				pdf.SetXY(10, 10)
				pdf.MultiCell(w-20, 4.5, tr(tmpl.HeaderText), "", "R", false)
				pdf.SetTextColor(0, 0, 0)
			}
			pdf.SetY(tmpl.TopFirstMM)
		} else {
			if len(imgs.BgOther) > 0 {
				registerAndDrawImage(pdf, "bg_other", bytes.NewReader(imgs.BgOther), 0, 0, w, h)
			} else if len(imgs.Logo) > 0 {
				registerAndDrawImage(pdf, "logo", bytes.NewReader(imgs.Logo), 10, 10, 20, 0)
			}
			if len(imgs.BgOther) == 0 && strings.TrimSpace(tmpl.HeaderText) != "" {
				pdf.SetFont("Helvetica", "", 9)
				pdf.SetTextColor(primary.R, primary.G, primary.B)
				pdf.SetXY(10, 10)
				pdf.MultiCell(w-20, 4.5, tr(tmpl.HeaderText), "", "R", false)
				pdf.SetTextColor(0, 0, 0)
			}
			pdf.SetY(tmpl.TopOtherMM)
		}
	}, true)

	pdf.SetFooterFunc(func() {
		if strings.TrimSpace(tmpl.FooterText) == "" {
			return
		}
		_, h := pdf.GetPageSize()
		pdf.SetY(h - 15)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(accent.R, accent.G, accent.B)
		pdf.CellFormat(0, 5, tr(tmpl.FooterText), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})

	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(primary.R, primary.G, primary.B)
	pdf.CellFormat(0, 7, tr(fmt.Sprintf("Angebot %s", quote.Number)), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(0, 6, tr(fmt.Sprintf("Projekt: %s    Status: %s", quote.ProjectName, quote.ProjectStatus)), "", 1, "L", false, 0, "")
	if strings.TrimSpace(quote.CreatedDate) != "" {
		pdf.CellFormat(0, 6, tr("Angelegt am: "+quote.CreatedDate), "", 1, "L", false, 0, "")
	}
	if strings.TrimSpace(quote.ValidUntil) != "" {
		pdf.CellFormat(0, 6, tr("Gueltig bis: "+quote.ValidUntil), "", 1, "L", false, 0, "")
	}
	if strings.TrimSpace(quote.CustomerName) != "" {
		pdf.CellFormat(0, 6, tr("Kunde: "+quote.CustomerName), "", 1, "L", false, 0, "")
	}
	if strings.TrimSpace(quote.CustomerEmail) != "" || strings.TrimSpace(quote.CustomerPhone) != "" {
		parts := make([]string, 0, 2)
		if strings.TrimSpace(quote.CustomerEmail) != "" {
			parts = append(parts, "E-Mail: "+quote.CustomerEmail)
		}
		if strings.TrimSpace(quote.CustomerPhone) != "" {
			parts = append(parts, "Telefon: "+quote.CustomerPhone)
		}
		pdf.CellFormat(0, 6, tr(strings.Join(parts, "    ")), "", 1, "L", false, 0, "")
	}

	pdf.Ln(3)
	drawSectionRule(pdf, accent)
	pdf.Ln(3)
	renderQuoteItemsTable(pdf, tr, quote.Items, primary, accent)
	if strings.TrimSpace(quote.Note) != "" {
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "", 9)
		pdf.MultiCell(0, 5, tr("Hinweis: "+quote.Note), "", "L", false)
	}
	if strings.TrimSpace(quote.Currency) != "" {
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetTextColor(primary.R, primary.G, primary.B)
		pdf.CellFormat(0, 6, tr("Summen"), "", 1, "R", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(0, 0, 0)
		pdf.CellFormat(0, 5, tr("Netto: "+money(quote.NetAmount, quote.Currency)), "", 1, "R", false, 0, "")
		pdf.CellFormat(0, 5, tr("Steuer: "+money(quote.TaxAmount, quote.Currency)), "", 1, "R", false, 0, "")
		pdf.SetTextColor(accent.R, accent.G, accent.B)
		pdf.CellFormat(0, 5, tr("Brutto: "+money(quote.GrossAmount, quote.Currency)), "", 1, "R", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	var buf bytes.Buffer
	if err = pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderItemsTable(pdf *gofpdf.Fpdf, tr func(string) string, items []PurchaseOrderItemData, primary, accent rgbColor) {
	// Spaltenbreiten
	colPos := []float64{12, 90, 20, 20, 25, 23}
	// Header
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(primary.R, primary.G, primary.B)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetDrawColor(accent.R, accent.G, accent.B)
	headers := []string{"Pos", "Bezeichnung", "Menge", "Einheit", "Einzelpreis", "Gesamt"}
	for i, h := range headers {
		pdf.CellFormat(colPos[i], 7, tr(h), "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(0, 0, 0)
	for _, it := range items {
		total := it.Qty * it.UnitPrice
		// Pos
		pdf.CellFormat(colPos[0], 6, tr(fmt.Sprintf("%d", it.Pos)), "1", 0, "R", false, 0, "")
		// Bezeichnung (MultiCell innerhalb eines Zeilen-Layouts)
		x, y := pdf.GetXY()
		pdf.MultiCell(colPos[1], 5, tr(it.Description), "1", "L", false)
		// ermittelte Höhe der Zelle
		h := pdf.GetY() - y
		// Menge, Einheit, Preis, Gesamt neben die MultiCell setzen
		pdf.SetXY(x+colPos[1], y)
		pdf.CellFormat(colPos[2], h, tr(trimFloat(it.Qty)), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colPos[3], h, tr(it.UOM), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colPos[4], h, tr(money(it.UnitPrice, it.Currency)), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colPos[5], h, tr(money(total, it.Currency)), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}
}

func renderInvoiceItemsTable(pdf *gofpdf.Fpdf, tr func(string) string, items []InvoiceOutItemData, primary, accent rgbColor) {
	colPos := []float64{12, 88, 20, 28, 20, 22}
	headers := []string{"Pos", "Bezeichnung", "Menge", "Einzelpreis", "Steuer", "Gesamt"}
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(primary.R, primary.G, primary.B)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetDrawColor(accent.R, accent.G, accent.B)
	for i, h := range headers {
		pdf.CellFormat(colPos[i], 7, tr(h), "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(0, 0, 0)
	for _, it := range items {
		total := it.Qty * it.UnitPrice
		x, y := pdf.GetXY()
		pdf.CellFormat(colPos[0], 6, tr(fmt.Sprintf("%d", it.Pos)), "1", 0, "R", false, 0, "")
		x, y = pdf.GetXY()
		pdf.MultiCell(colPos[1], 5, tr(it.Description), "1", "L", false)
		h := pdf.GetY() - y
		pdf.SetXY(x+colPos[1], y)
		pdf.CellFormat(colPos[2], h, tr(trimFloat(it.Qty)), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colPos[3], h, tr(money(it.UnitPrice, it.Currency)), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colPos[4], h, tr(it.TaxCode), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colPos[5], h, tr(money(total, it.Currency)), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}
}

func renderQuoteItemsTable(pdf *gofpdf.Fpdf, tr func(string) string, items []QuoteItemData, primary, accent rgbColor) {
	colPos := []float64{9, 18, 51, 14, 16, 22, 14, 18, 23, 25}
	headers := []string{"Pos", "Los", "Position", "Menge", "Einheit", "Abmessung", "Steuer", "Preis", "Gesamt", "Serie/Oberfl."}
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(primary.R, primary.G, primary.B)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetDrawColor(accent.R, accent.G, accent.B)
	for i, h := range headers {
		pdf.CellFormat(colPos[i], 7, tr(h), "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(0, 0, 0)
	for _, it := range items {
		x, y := pdf.GetXY()
		pdf.CellFormat(colPos[0], 6, tr(fmt.Sprintf("%d", it.Pos)), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colPos[1], 6, tr(it.PhaseLabel), "1", 0, "L", false, 0, "")
		x, y = pdf.GetXY()
		pdf.MultiCell(colPos[2], 5, tr(it.Description), "1", "L", false)
		h := pdf.GetY() - y
		pdf.SetXY(x+colPos[2], y)
		pdf.CellFormat(colPos[3], h, tr(trimFloat(it.Qty)), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colPos[4], h, tr(it.Unit), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colPos[5], h, tr(it.DimensionsLabel), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colPos[6], h, tr(it.TaxCode), "1", 0, "C", false, 0, "")
		priceText := ""
		totalText := ""
		if it.UnitPrice != 0 || it.LineTotal != 0 {
			priceText = money(it.UnitPrice, "")
			totalText = money(it.LineTotal, "")
		}
		pdf.CellFormat(colPos[7], h, tr(priceText), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colPos[8], h, tr(totalText), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colPos[9], h, tr(it.SurfaceLabel), "1", 0, "L", false, 0, "")
		pdf.Ln(-1)
	}
}

func drawSectionRule(pdf *gofpdf.Fpdf, accent rgbColor) {
	pdf.SetDrawColor(accent.R, accent.G, accent.B)
	x, y := pdf.GetXY()
	pdf.Line(x, y, 200, y)
	pdf.Ln(1.5)
}

func dimensionsLabel(widthMM, heightMM *float64) string {
	if widthMM == nil && heightMM == nil {
		return ""
	}
	if widthMM != nil && heightMM != nil {
		return fmt.Sprintf("%s x %s mm", trimFloat(*widthMM), trimFloat(*heightMM))
	}
	if widthMM != nil {
		return fmt.Sprintf("B %s mm", trimFloat(*widthMM))
	}
	return fmt.Sprintf("H %s mm", trimFloat(*heightMM))
}

func trimFloat(v float64) string {
	// 3 Nachkommastellen max., ohne überflüssige Nullen
	s := fmt.Sprintf("%.3f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		s = "0"
	}
	return s
}

func money(v float64, cur string) string {
	// 2 Nachkommastellen und Tausenderpunkte
	s := fmt.Sprintf("%.2f", v)
	parts := strings.Split(s, ".")
	ip := parts[0]
	dp := ""
	if len(parts) > 1 {
		dp = parts[1]
	}
	out := ""
	for i, c := range reverse(ip) {
		if i != 0 && i%3 == 0 {
			out = "." + out
		}
		out = string(c) + out
	}
	if dp != "" {
		out = out + "," + dp
	}
	if strings.TrimSpace(cur) != "" {
		sym := currencySymbol(strings.TrimSpace(strings.ToUpper(cur)))
		out = out + " " + sym
	}
	return out
}

func currencySymbol(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "EUR":
		return "€"
	case "USD":
		return "$"
	case "GBP":
		return "£"
	case "JPY":
		return "¥"
	case "CHF":
		return "CHF"
	default:
		return code
	}
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func loadFromGridFS(ctx context.Context, mg *mongo.Client, dbName, hexID string) ([]byte, error) {
	if mg == nil || dbName == "" {
		return nil, nil
	}
	oid, err := primitive.ObjectIDFromHex(hexID)
	if err != nil {
		return nil, err
	}
	bucket, err := gridfs.NewBucket(mg.Database(dbName))
	if err != nil {
		return nil, err
	}
	ds, err := bucket.OpenDownloadStream(oid)
	if err != nil {
		return nil, err
	}
	defer ds.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, ds); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func registerAndDrawImage(pdf *gofpdf.Fpdf, name string, r io.Reader, x, y, w, h float64) {
	// Register reader; ignore error if already registered
	opt := gofpdf.ImageOptions{ImageType: "", ReadDpi: true}
	_ = pdf.RegisterImageOptionsReader(name, opt, r)
	// Falls sowohl w als auch h 0 sind, Standardbreite setzen
	if (w == 0 && h == 0) || math.IsNaN(w) || math.IsNaN(h) {
		w = 20
		h = 0 // proportional
	}
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	pdf.ImageOptions(name, x, y, w, h, false, opt, 0, "")
}

func templateColors(tmpl TemplateOptions) (rgbColor, rgbColor) {
	return parseHexColor(tmpl.PrimaryColor, rgbColor{R: 31, G: 75, B: 153}), parseHexColor(tmpl.AccentColor, rgbColor{R: 107, G: 114, B: 128})
}

func parseHexColor(hex string, fallback rgbColor) rgbColor {
	hex = strings.TrimSpace(strings.TrimPrefix(hex, "#"))
	if len(hex) != 6 {
		return fallback
	}
	for _, r := range hex {
		if !strings.ContainsRune("0123456789ABCDEFabcdef", r) {
			return fallback
		}
	}
	var color rgbColor
	if _, err := fmt.Sscanf(strings.ToUpper(hex), "%02X%02X%02X", &color.R, &color.G, &color.B); err != nil {
		return fallback
	}
	return color
}
