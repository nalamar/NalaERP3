package quotes

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

const gaebXMLSubsetParserVersion = "gaeb-xml-subset-v1"

type GAEBXMLSubsetParser struct{}

func (GAEBXMLSubsetParser) ParseGAEB(_ context.Context, source io.Reader, _ string) (GAEBImportParseResult, error) {
	var document struct {
		XMLName xml.Name `xml:"gaeb"`
		Items   []struct {
			PositionNo  string `xml:"position_no,attr"`
			OutlineNo   string `xml:"outline_no,attr"`
			Qty         string `xml:"qty,attr"`
			Unit        string `xml:"unit,attr"`
			Optional    string `xml:"optional,attr"`
			Description string `xml:"description"`
		} `xml:"item"`
	}
	if err := xml.NewDecoder(source).Decode(&document); err != nil {
		return GAEBImportParseResult{}, fmt.Errorf("GAEB-XML konnte nicht gelesen werden: %w", err)
	}
	if document.XMLName.Local != "gaeb" {
		return GAEBImportParseResult{}, errors.New("GAEB-XML benötigt das Root-Element gaeb")
	}
	if len(document.Items) == 0 {
		return GAEBImportParseResult{}, errors.New("GAEB-XML enthält keine Positionen")
	}
	items := make([]QuoteImportItemInput, 0, len(document.Items))
	for index, raw := range document.Items {
		qty, err := strconv.ParseFloat(strings.TrimSpace(raw.Qty), 64)
		if err != nil || math.IsNaN(qty) || math.IsInf(qty, 0) || qty < 0 {
			return GAEBImportParseResult{}, fmt.Errorf("GAEB-Position %d hat eine ungültige Menge", index+1)
		}
		optional := false
		if value := strings.TrimSpace(raw.Optional); value != "" {
			optional, err = strconv.ParseBool(value)
			if err != nil {
				return GAEBImportParseResult{}, fmt.Errorf("GAEB-Position %d hat ein ungültiges optional-Attribut", index+1)
			}
		}
		item := QuoteImportItemInput{PositionNo: strings.TrimSpace(raw.PositionNo), OutlineNo: strings.TrimSpace(raw.OutlineNo), Description: strings.TrimSpace(raw.Description), Qty: qty, Unit: strings.TrimSpace(raw.Unit), IsOptional: optional, ParserHint: "gaeb_xml_subset_item", SortOrder: index + 1}
		if item.PositionNo == "" || item.Unit == "" || item.Description == "" {
			return GAEBImportParseResult{}, fmt.Errorf("GAEB-Position %d enthält ein Pflichtfeld nicht", index+1)
		}
		items = append(items, item)
	}
	return GAEBImportParseResult{ParserVersion: gaebXMLSubsetParserVersion, DetectedFormat: "gaeb_xml_subset", Items: items}, nil
}
