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
    Logo     []byte
    BgFirst  []byte
    BgOther  []byte
}

// TemplateOptions beschreibt die Template-bezogenen Einstellungen.
type TemplateOptions struct {
    HeaderText string
    FooterText string
    TopFirstMM float64
    TopOtherMM float64
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
    Pos        int
    Description string
    Qty        float64
    UOM        string
    UnitPrice  float64
    Currency   string
}

// RenderPurchaseOrder rendert eine Bestellung als PDF unter Verwendung der Template-Optionen und Images.
// mg/dbName werden nur genutzt, wenn imageDocIDs gesetzt sind und images nicht vorab geladen wurden.
func RenderPurchaseOrder(ctx context.Context, mg *mongo.Client, dbName string, po PurchaseOrderData, tmpl TemplateOptions, imageDocIDs map[string]string) ([]byte, error) {
    pdf := gofpdf.New("P", "mm", "A4", "")
    pdf.SetTitle(fmt.Sprintf("Bestellung %s", po.Number), false)
    pdf.SetAuthor("NalaERP3", false)
    tr := pdf.UnicodeTranslatorFromDescriptor("cp1252")

    // Bilder ggf. aus GridFS laden
    var imgs ImageSources
    var err error
    if imageDocIDs != nil {
        if id := strings.TrimSpace(imageDocIDs["logo"]); id != "" { imgs.Logo, _ = loadFromGridFS(ctx, mg, dbName, id) }
        if id := strings.TrimSpace(imageDocIDs["bg_first"]); id != "" { imgs.BgFirst, _ = loadFromGridFS(ctx, mg, dbName, id) }
        if id := strings.TrimSpace(imageDocIDs["bg_other"]); id != "" { imgs.BgOther, _ = loadFromGridFS(ctx, mg, dbName, id) }
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
                pdf.SetXY(10, 10)
                pdf.MultiCell(w-20, 4.5, tr(tmpl.HeaderText), "", "R", false)
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
                pdf.SetXY(10, 10)
                pdf.MultiCell(w-20, 4.5, tr(tmpl.HeaderText), "", "R", false)
            }
            pdf.SetY(tmpl.TopOtherMM)
        }
    }, true)

    // Footer: Fußtext
    pdf.SetFooterFunc(func() {
        if strings.TrimSpace(tmpl.FooterText) == "" { return }
        _, h := pdf.GetPageSize()
        pdf.SetY(h - 15)
        pdf.SetFont("Helvetica", "", 8)
        pdf.CellFormat(0, 5, tr(tmpl.FooterText), "", 0, "C", false, 0, "")
    })

    // Erste Seite anlegen (HeaderFunc zeichnet Hintergründe/Logo und setzt Y)
    pdf.AddPage()

    // Titel / Kopfdaten Bestellung
    pdf.SetFont("Helvetica", "B", 14)
    pdf.CellFormat(0, 7, tr(fmt.Sprintf("Bestellung %s", po.Number)), "", 1, "L", false, 0, "")
    pdf.SetFont("Helvetica", "", 10)
    pdf.CellFormat(0, 6, tr(fmt.Sprintf("Datum: %s    Status: %s    Währung: %s", po.OrderDate, po.Status, po.Currency)), "", 1, "L", false, 0, "")
    if strings.TrimSpace(po.Note) != "" {
        pdf.SetFont("Helvetica", "", 9)
        pdf.MultiCell(0, 5, tr(po.Note), "", "L", false)
    }

    // Abstand vor Tabelle
    pdf.Ln(3)

    // Items-Tabelle
    renderItemsTable(pdf, tr, po.Items)

    // Ausgeben
    var buf bytes.Buffer
    if err = pdf.Output(&buf); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}

func renderItemsTable(pdf *gofpdf.Fpdf, tr func(string) string, items []PurchaseOrderItemData) {
    // Spaltenbreiten
    colPos := []float64{12, 90, 20, 20, 25, 23}
    // Header
    pdf.SetFont("Helvetica", "B", 10)
    headers := []string{"Pos", "Bezeichnung", "Menge", "Einheit", "Einzelpreis", "Gesamt"}
    for i, h := range headers {
        pdf.CellFormat(colPos[i], 7, tr(h), "1", 0, "C", false, 0, "")
    }
    pdf.Ln(-1)

    pdf.SetFont("Helvetica", "", 9)
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

func trimFloat(v float64) string {
    // 3 Nachkommastellen max., ohne überflüssige Nullen
    s := fmt.Sprintf("%.3f", v)
    s = strings.TrimRight(s, "0")
    s = strings.TrimRight(s, ".")
    if s == "" { s = "0" }
    return s
}

func money(v float64, cur string) string {
    // 2 Nachkommastellen und Tausenderpunkte
    s := fmt.Sprintf("%.2f", v)
    parts := strings.Split(s, ".")
    ip := parts[0]
    dp := ""
    if len(parts) > 1 { dp = parts[1] }
    out := ""
    for i, c := range reverse(ip) {
        if i != 0 && i%3 == 0 { out = "." + out }
        out = string(c) + out
    }
    if dp != "" { out = out + "," + dp }
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
    if mg == nil || dbName == "" { return nil, nil }
    oid, err := primitive.ObjectIDFromHex(hexID)
    if err != nil { return nil, err }
    bucket, err := gridfs.NewBucket(mg.Database(dbName))
    if err != nil { return nil, err }
    ds, err := bucket.OpenDownloadStream(oid)
    if err != nil { return nil, err }
    defer ds.Close()
    var buf bytes.Buffer
    if _, err := io.Copy(&buf, ds); err != nil { return nil, err }
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
    if w < 0 { w = 0 }
    if h < 0 { h = 0 }
    pdf.ImageOptions(name, x, y, w, h, false, opt, 0, "")
}
