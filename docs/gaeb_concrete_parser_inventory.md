# GAEB: Folgeinventar nach produktivem Process-Endpunkt

## Ziel

Subtask 3.1.75.1 bestimmt den kleinsten verbleibenden fachlichen
GAEB-Risikobereich nach dem auditierten Process-Endpunkt. Dieser Leaf aendert
keine Runtime- oder Testlogik.

## Ausgangslage

Upload, expliziter Verarbeitungsausloeser, Statusuebergabe und Reviewstrecke
sind technisch verbunden. Der Process-Endpunkt kann jedoch nur arbeiten, wenn
ein GAEBImportParser bereitgestellt wird. Der Repository-Stand enthält bewusst
noch keinen solchen Adapter; ohne ihn liefert die API transparent 500 und
belässt den Import auf uploaded.

Damit ist nicht mehr der Transport, sondern die erste belastbare Umwandlung
von einer gespeicherten Quelle in QuoteImportItemInput der Engpass.

## Optionen

### Option A: Minimaler XML-Subset-Parser

Ein enger Parser akzeptiert ausschließlich XML-basierte, explizit
dokumentierte Test-LV-Positionen und erzeugt nur Position, Beschreibung,
Menge und Einheit. Nicht erkannte Strukturen führen kontrolliert nach failed.

Bewertung: kleinster realer Parseradapter; schafft einen Ende-zu-Ende-Pfad
ohne eine unzuverlässige Behauptung vollständiger GAEB-Unterstützung.

### Option B: X83-Binärformat zuerst

Ein vollständiger X83-Parser würde den häufigen Praxisfall bedienen.

Bewertung: fachlich wertvoll, aber erheblich größer: Containerstruktur,
Versionen, Validierung und Referenzdateien benötigen einen eigenen Block.

### Option C: KI-Textanalyse als Parser ersetzen

Ein Modell könnte beliebige Dateiinhalte interpretieren.

Bewertung: ohne deterministische Grundextraktion und Quellenbelege nicht
revisionssicher; kein Einstiegspunkt.

## Entscheidung

Der nächste kleine Ausbaustrang ist ein streng begrenzter XML-Subset-Adapter.
Er wird nur für ein explizites Testformat aktiviert, besitzt eine feste
Parserkennung und verwirft unbekannte oder unvollständige Inhalte. X83, X84,
D83, P83, generisches XML und KI bleiben ausdrücklich unerledigt.

## Zerlegung

1. **3.1.75.2** – XML-Subset, Positionsmapping und Fehlervertrag definieren.
2. **3.1.75.3** – Deterministischen XML-Subset-Parser und Service-Ende-zu-Ende-Test implementieren.
3. **3.1.75.4** – Parserimplementierung und Nachweise auditieren.

## Ergebnis

3.1.75.1 ist abgeschlossen. Ein strikt begrenzter XML-Subset-Parser ist der
kleinste reale Anschluss an den nun vorhandenen Prozessvertrag.
