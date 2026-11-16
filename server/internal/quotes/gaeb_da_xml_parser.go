package quotes

import (
	"context"
	"fmt"
	"io"
	"strings"

	"nalaerp3/internal/quotes/gaeb"
)

const gaebDAXMLParserVersion = "gaeb-da-xml-v1"

// GAEBDAXMLParser liest GAEB DA XML (X83) ueber das Paket gaeb (ADR 0040).
// Kurz- und Langtext stehen einzeln und zusammen in Description (fuer den
// heutigen Client und die Uebernahme), die Positionsart auch in
// ParserHint; Bedarfs- und Alternativpositionen gelten als optional.
type GAEBDAXMLParser struct{}

func (GAEBDAXMLParser) ParseGAEB(_ context.Context, source io.Reader, _ string) (GAEBImportParseResult, error) {
	lv, err := gaeb.Parse(source)
	if err != nil {
		return GAEBImportParseResult{}, fmt.Errorf("GAEB-Datei konnte nicht gelesen werden: %w", err)
	}
	items := make([]QuoteImportItemInput, 0, len(lv.Positionen))
	for i, p := range lv.Positionen {
		description := p.Kurztext
		if p.Langtext != "" {
			description += "\n\n" + p.Langtext
		}
		items = append(items, QuoteImportItemInput{
			PositionNo:     p.OZ,
			OutlineNo:      p.Bereich,
			Description:    description,
			Qty:            p.Menge,
			Unit:           p.Einheit,
			IsOptional:     p.Art != gaeb.ArtNormal && p.Art != gaeb.ArtGrund,
			ParserHint:     p.Art,
			SortOrder:      i + 1,
			ShortText:      p.Kurztext,
			LongText:       p.Langtext,
			ItemKind:       p.Art,
			IsLumpSum:      p.Pauschal,
			BasePositionNo: p.Zu,
		})
	}
	groups := make([]QuoteImportGroupInput, 0, len(lv.Bereiche))
	for _, b := range lv.Bereiche {
		groups = append(groups, QuoteImportGroupInput{OutlineNo: b.OZ, ParentOutlineNo: b.Eltern, Kind: b.Stufe, Bezeichnung: b.Bezeichnung})
	}
	return GAEBImportParseResult{ParserVersion: gaebDAXMLParserVersion, DetectedFormat: lv.Format + "_" + strings.ToLower(lv.Phase),
		Items: items, Groups: groups, Vorbemerkungen: lv.Vorbemerkungen}, nil
}
