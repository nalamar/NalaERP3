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
// Bis zur Erweiterung der Importpositionen (I.2.2) gehen Kurz- und
// Langtext zusammen in Description, die Positionsart in ParserHint;
// Bedarfs- und Alternativpositionen gelten als optional.
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
			PositionNo:  p.OZ,
			OutlineNo:   p.Bereich,
			Description: description,
			Qty:         p.Menge,
			Unit:        p.Einheit,
			IsOptional:  p.Art != gaeb.ArtNormal && p.Art != gaeb.ArtGrund,
			ParserHint:  p.Art,
			SortOrder:   i + 1,
		})
	}
	return GAEBImportParseResult{ParserVersion: gaebDAXMLParserVersion, DetectedFormat: lv.Format + "_" + strings.ToLower(lv.Phase), Items: items}, nil
}
