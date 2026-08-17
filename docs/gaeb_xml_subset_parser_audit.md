# GAEB: Audit des XML-Subset-Parsers

Subtask 3.1.75.4 ist ohne Befund abgeschlossen. Der Parser akzeptiert nur das
dokumentierte namespace-freie gaeb-Root mit mindestens einer vollständigen
item-Position. Position, Menge, Einheit, Beschreibung, optionale
Gliederungsnummer und optional-Flag werden deterministisch normalisiert.

Falsches Root-Element, fehlende Pflichtfelder, unendliche oder negative Mengen
und ungültige optional-Werte liefern Fehler und führen über ProcessGAEBImport
kontrolliert nach failed. Parserkennung, Format und Hint entsprechen dem
Strategievertrag. Es wurden keine Binärformate, generisches XML, KI-,
Mapping-, Preis- oder Reviewlogik ergänzt.

go test ./internal/quotes -run TestGAEB(XMLSubsetParser|ProcessGAEBImport)
-count=1 sowie git diff --check sind gruen.
