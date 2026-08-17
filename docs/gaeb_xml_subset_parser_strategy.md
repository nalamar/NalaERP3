# GAEB: Strategie fuer den XML-Subset-Parser

## Ziel

Subtask 3.1.75.2 definiert einen einzigen, bewusst nicht generischen XML-
Subset als ersten konkreten GAEBImportParser.

## Akzeptiertes Dokument

Nur Dokumente mit diesem Namespace-freien Minimalformat werden akzeptiert:

    <gaeb>
      <item position_no="01.001" qty="2" unit="Stk">
        <description>Aluminiumfenster</description>
      </item>
    </gaeb>

Regeln:

- Root-Element muss gaeb sein.
- Mindestens ein item ist erforderlich.
- position_no ist nicht leer.
- qty ist eine endliche Zahl >= 0.
- unit ist nicht leer.
- description enthält nach Trim mindestens ein Zeichen.
- item-Reihenfolge wird als SortOrder 1..n beibehalten.
- optionale Attribute outline_no und optional werden als OutlineNo und
  IsOptional übernommen; optional akzeptiert nur true oder false.

## Ergebnisvertrag

Der Adapter liefert:

- ParserVersion: gaeb-xml-subset-v1;
- DetectedFormat: gaeb_xml_subset;
- QuoteImportItemInput mit den beschriebenen Rohfeldern;
- ParserHint: gaeb_xml_subset_item.

Jeder XML-, Struktur- oder Feldfehler liefert einen beschreibenden error.
ProcessGAEBImport führt den Import damit kontrolliert nach failed; der Parser
schreibt selbst keinen Status und kennt keine Infrastruktur.

## Nicht-Ziele

- keine Namespaces, LV-Hierarchie, Zuschläge, Alternativgruppen oder Mengen-
  berechnungen;
- keine X83-, X84-, D83-, P83- oder generische XML-Unterstützung;
- keine Preis-, Mapping-, Review- oder KI-Logik.

## Verifikation für 3.1.75.3

- Parserunit-Test für zwei gültige Items inklusive optionaler Attribute.
- Parserunit-Tests für falsches Root-Element und fehlendes Pflichtfeld.
- Service-Ende-zu-Ende-Test: XML-Quelle über ProcessGAEBImport führt nach
  parsed und persistiert die normalisierten Positionen.

## Ergebnis

3.1.75.2 ist abgeschlossen. Der erste konkrete Adapter bleibt als
gaeb-xml-subset-v1 eindeutig versioniert und strikt begrenzt.
