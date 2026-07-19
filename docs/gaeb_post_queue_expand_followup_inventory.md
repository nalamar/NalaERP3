# GAEB: Folgeinventur nach den erweiterbaren Approval-Arbeitskarten

## Ziel

Subtask 3.1.56.1 bestimmt den kleinsten fachlich sinnvollen Folgeausbau nach
den nun voll erreichbaren und widgetgetesteten Freigabeanforderungs- und
Nacharbeits-Queues. Dieser Leaf aendert keine Laufzeitlogik.

## Ausgangslage

Die beiden Approval-Arbeitskarten koennen alle bereits geladenen Eintraege
unabhaengig ein- und ausklappen. Direkt darunter liegt die projektbezogene
GAEB-Importkarte mit einem verwandten, aber nicht identischen Vertrag:

- `_loadQuoteImports()` fordert maximal sechs Importlaeufe an (`limit: 6`).
- `_quoteImports` enthaelt damit eine begrenzte Vorschau, nicht zwingend die
  gesamte Importhistorie.
- `visibleImports` rendert fest nur die ersten drei Eintraege.
- weitere bereits geladene Eintraege erscheinen lediglich als statischer Text
  `+n weitere`.
- diese Eintraege koennen aus der Karte nicht direkt geoeffnet werden.

Damit besteht eine kleine Erreichbarkeitsluecke innerhalb der bereits geladenen
Importvorschau.

## Optionen

### A: Ein- und Ausklappen der geladenen Importvorschau

Die Karte zeigt standardmaessig drei und bei Bedarf alle bis zu sechs bereits
geladenen `_quoteImports`.

Vorteile:

- schliesst die konkrete Luecke ohne neuen Serververtrag
- rein clientseitig
- bestehende `Details`-Aktion bleibt unveraendert
- nutzt das bereits bewaehrte Anzeigezustandsmuster
- die maximale Darstellung bleibt durch `limit: 6` klein

Bewertung: kleinster Folgeausbau mit direktem GAEB-Nutzen.

### B: Vollstaendige Importhistorie mit Pagination

Eine eigene Liste oder ein Dialog koennte alle Importlaeufe mit `limit` und
`offset` laden.

Bewertung: fachlich sinnvoll bei umfangreicher Historie, aber ein neuer
Navigations-, Lade- und Testvertrag. Nicht notwendig, um die bereits geladenen
Eintraege erreichbar zu machen.

### C: Gemeinsame generische Kartenkomponente

Approval, Rework und Importvorschau koennten abstrahiert werden.

Bewertung: Die Karten unterscheiden sich bei Ladebedingung, Farben, Kontext
und Aktionen. Ein Refactoring liefert fuer den aktuellen Fachschritt keinen
zusaetzlichen Nutzen.

### D: GAEB-Importsuche oder Statusfilter

Importlaeufe koennten nach Dateiname oder Status gefiltert werden.

Bewertung: Fuer eine auf sechs Eintraege begrenzte Vorschau zu frueh. Dies
gehoert zu einer spaeteren vollstaendigen Importhistorie.

### E: Approval-Cockpit, KPI oder SLA

Die nun erreichbaren Approval-Arbeitsbereiche koennten aggregiert werden.

Bewertung: benoetigt weiterhin eigene Regeln fuer Verantwortlichkeit,
Priorisierung und Fristen und ist groesser als die sichtbare GAEB-Luecke.

## Entscheidung

Der naechste Ausbau ist eine clientseitige Ein-/Ausklappfunktion fuer die
bereits geladene GAEB-Importvorschau.

Das Ziel wird sprachlich und technisch klar begrenzt:

- standardmaessig drei Importlaeufe
- `Alle anzeigen` zeigt alle aktuell in `_quoteImports` geladenen Eintraege
- aufgrund des bestehenden `limit: 6` sind dies maximal sechs
- `Weniger anzeigen` stellt die Drei-Eintraege-Vorschau wieder her
- `Details` und Importladen bleiben unveraendert
- die Funktion behauptet nicht, die vollstaendige Serverhistorie zu zeigen

Der Importzustand bleibt unabhaengig von Approval-Request- und Rework-Karte.

## Naechster Leaf

Subtask 3.1.56.2 schneidet das technische Minimalzielmodell zu:

- eigener lokaler Import-Expand-State
- Ableitung von `visibleImports`
- Ruecksetz- und Refreshverhalten
- Umgang mit fehlendem Projektfilter
- stabile Umschaltlabels
- Fake-API- und Widgettestgrenze fuer vier Importlaeufe

## Nicht-Ziele

- noch keine Implementierung
- keine vollstaendige Importhistorie
- keine Pagination oder Aenderung von `limit: 6`
- keine Importsuche, Filterung oder Sortierung
- keine generische Kartenkomponente
- keine Aenderung der `Details`-Aktion
- keine Approval-, Cockpit-, KPI-, SLA- oder KI-Erweiterung
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Ergebnis

3.1.56.1 ist abgeschlossen. Der kleinste Folgeausbau macht alle bereits
geladenen Eintraege der begrenzten GAEB-Importvorschau erreichbar. Der naechste
Leaf 3.1.56.2 definiert dafuer das technische Minimalzielmodell.
