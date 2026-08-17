package quotes

import (
	"context"
	"strings"
	"testing"
)

func TestGAEBXMLSubsetParser(t *testing.T) {
	result, err := (GAEBXMLSubsetParser{}).ParseGAEB(context.Background(), strings.NewReader(`<gaeb><item position_no="01.001" qty="2" unit="Stk"><description>Fenster</description></item><item position_no="01.002" qty="1.5" unit="Std" optional="true"><description>Montage</description></item></gaeb>`), "test.xml")
	if err != nil || result.ParserVersion != gaebXMLSubsetParserVersion || len(result.Items) != 2 || !result.Items[1].IsOptional {
		t.Fatalf("unexpected result: %+v err=%v", result, err)
	}
	if _, err := (GAEBXMLSubsetParser{}).ParseGAEB(context.Background(), strings.NewReader(`<invalid/>`), "test.xml"); err == nil {
		t.Fatal("expected invalid root error")
	}
	if _, err := (GAEBXMLSubsetParser{}).ParseGAEB(context.Background(), strings.NewReader(`<gaeb><item qty="1" unit="Stk"><description>Fenster</description></item></gaeb>`), "test.xml"); err == nil {
		t.Fatal("expected required field error")
	}
}
