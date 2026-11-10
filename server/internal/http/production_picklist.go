package apihttp

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"nalaerp3/internal/pdfgen"
	"nalaerp3/internal/production"
	"nalaerp3/internal/settings"
)

// Kommissionierliste als PDF (ADR 0038 Frage 4 (a), Backlog H.3.3b),
// eingehaengt unter /production/orders, Lesen mit production.read.

var productionStatusLabels = map[string]string{
	production.StatusFreigegeben: "freigegeben",
	production.StatusInArbeit:    "in Arbeit",
}

// pickListPDFData bereitet die Kommissionierliste fuer pdfgen auf.
func pickListPDFData(l *production.PickList, now time.Time) pdfgen.PickListData {
	out := pdfgen.PickListData{
		Number:     l.Nummer,
		Project:    strings.TrimSpace(l.ProjectNummer + " " + l.ProjectName),
		Position:   strings.TrimSpace(l.PositionNummer + " " + l.PositionName),
		Stueckzahl: l.Stueckzahl,
		Status:     productionStatusLabels[l.Status],
		Lager:      l.Lager,
		PrintedAt:  now.Format("02.01.2006 15:04"),
		Lines:      make([]pdfgen.PickListLineData, 0, len(l.Lines)),
	}
	if out.Status == "" {
		out.Status = l.Status
	}
	if l.VarianteName != "" {
		out.Position += " (" + l.VarianteName + ")"
	}
	if l.Liefertermin != nil {
		out.Liefertermin = l.Liefertermin.Format("02.01.2006")
	}
	for _, x := range l.Lines {
		var material []string
		if s := strings.TrimSpace(x.ArticleCode + " " + x.Description); s != "" {
			material = append(material, s)
		}
		if s := strings.TrimSpace(x.MaterialNummer + " " + x.MaterialBezeichnung); s != "" {
			material = append(material, "Lager: "+s)
		}
		menge := strings.TrimSpace(production.FormatQty(x.Menge) + " " + x.Einheit)
		if x.Gesamt {
			menge += " (gesamt)"
		}
		out.Lines = append(out.Lines, pdfgen.PickListLineData{Pos: x.Pos, Material: strings.Join(material, "\n"),
			Abmessung: x.Abmessung, Menge: menge, Plaetze: x.Plaetze})
	}
	return out
}

func mountProductionPickListRoute(r chi.Router, svc *production.Service, brandingSvc *settings.BrandingService) {
	r.With(requirePermission(permProductionRead)).Get("/{id}/kommissionierliste.pdf", func(w http.ResponseWriter, req *http.Request) {
		id, ok := parseUUIDParam(w, req, "id", "Ungültige Auftrags-ID")
		if !ok {
			return
		}
		companyID, _ := companyIDFromContext(req.Context())
		l, err := svc.PickList(req.Context(), id, companyID)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		primary, accent := "", ""
		if b, berr := brandingSvc.Get(req.Context()); berr == nil {
			primary, accent = b.PrimaryColor, b.AccentColor
		}
		pdfBytes, err := pdfgen.RenderPickList(pickListPDFData(l, time.Now()), primary, accent)
		if err != nil {
			writeHTTPError(w, req, http.StatusInternalServerError, err.Error(), err)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=\"Kommissionierliste_"+sanitizeFilename(l.Nummer)+".pdf\"")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
		_, _ = w.Write(pdfBytes)
	})
}
