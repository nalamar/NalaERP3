# GAEB-Freigabe: Folgeinventur nach der Approval-Queue-Widgettest-Haertung

## Ziel

Subtask 3.1.54.1 bestimmt den kleinsten fachlich sinnvollen Folgeausbau nach
dem abgeschlossenen und widgetgetesteten Queue-Entscheidungsdialog. Dieser Leaf
aendert keine Laufzeitlogik.

## Ausgangslage

Der Approval-Strang bietet inzwischen:

- persistente Freigabeanforderungen und terminale Entscheidungen
- Anzeigenamen, Entscheidungskommentare und Positionshistorie
- eine globale read-only Freigabeanforderungs-Queue
- eine globale Nacharbeits-Queue
- direkte Navigation zur betroffenen Quote-Position
- Genehmigen und Ablehnen direkt aus der Queue
- kompakten Entscheidungsdialog mit Anfrage-, Preis- und Zielkontext
- einen deterministischen Widgettest fuer Kontext, Nullwerte und
  Kommentarweitergabe

Die Freigabeanforderungs-Queue wird bereits vollstaendig vom bestehenden
Endpoint geladen. In der `QuotesPage`-Karte werden jedoch nur die ersten drei
Eintraege gerendert. Weitere Eintraege erscheinen ausschliesslich als
statischer Text `+n weitere offene Anforderungen` und sind aus dieser Karte
nicht direkt erreichbar.

## Verbleibende Optionen

### A: Clientseitiges Ein- und Ausklappen der bestehenden Karte

Die bereits geladenen Queue-Eintraege werden bei Bedarf in derselben Karte
vollstaendig angezeigt. Ein kleiner Textbutton ersetzt den bisherigen
statischen Hinweis und schaltet zwischen kompakter und erweiterter Ansicht.

Vorteile:

- schliesst die konkrete Erreichbarkeitsluecke
- bleibt client-only
- nutzt unveraendert dasselbe Readmodel und dieselben Aktionen
- keine neue Navigation oder Berechtigung
- kompakte Standardansicht bleibt erhalten

Grenze:

- keine serverseitige Pagination
- fuer sehr grosse Queues spaeter nicht ausreichend

Bewertung: kleinster Folgeausbau mit direktem operativem Nutzen.

### B: Dedizierte Approval-Arbeitsliste

Eine eigene Seite koennte Pagination, Filter, Sortierung und spaeter
Massenaktionen aufnehmen.

Bewertung: fachlich sinnvoll bei nachgewiesenem Mengendruck, aber aktuell eine
neue UI-Flaeche und deutlich groesser als die konkrete Erreichbarkeitsluecke.

### C: Serverseitige Pagination

Der Queue-Endpoint koennte `limit` und `offset` sowie Gesamtanzahl erhalten.

Bewertung: fuer grosse Datenmengen spaeter notwendig. Ohne dedizierte
Arbeitsliste oder messbares Volumen erweitert dies Backend, API und Client zu
frueh.

### D: Workflow-Cockpit, KPI oder SLA

Offene Anzahl, Alter, Zielabweichung und Eskalationsstufen koennten spaeter in
Cockpit und Reporting einfliessen.

Bewertung: benoetigt eigene Entscheidungen zu Verantwortlichkeit,
Priorisierung, Fristen und Aggregation. Kein kleiner Anschlussleaf.

### E: Zweiter Ablehnen-Widgettest

Genehmigen und Ablehnen teilen Dialog, Kontextbau und Kommentartrim-Semantik.
Der abgeschlossene Audit hat einen duplizierten Dialogtest ohne konkretes
zusaetzliches Regressionssignal bewertet.

Bewertung: derzeit nicht erforderlich.

## Entscheidung

Der naechste Ausbau ist eine kleine clientseitige Ein-/Ausklappfunktion fuer
die bestehende Freigabeanforderungs-Karte.

Das Ziel bleibt eng:

- standardmaessig weiterhin maximal drei Eintraege
- bei mehr als drei Eintraegen interaktive Aktion `Alle anzeigen`
- erweiterte Ansicht verwendet die bereits geladenen `_approvalRequestItems`
- Aktion `Weniger anzeigen` fuehrt zur kompakten Ansicht zurueck
- bestehende Genehmigen-, Ablehnen- und Oeffnen-Aktionen bleiben unveraendert

## Warum keine neue Backend-Stufe

Der Client besitzt die vollstaendige Liste bereits. Die aktuelle Luecke ist
nicht fehlende Serverinformation, sondern die nicht interaktive Begrenzung der
Darstellung. Ein Backend-, API- oder Persistenzausbau wuerde fuer diesen ersten
Schritt keinen zusaetzlichen Fachwert liefern.

## Naechster Leaf

Subtask 3.1.54.2 schneidet ausschliesslich das technische Minimalzielmodell
fuer die Ein-/Ausklappfunktion zu:

- lokaler UI-State und Ruecksetzverhalten
- Ableitung der sichtbaren Queue-Eintraege
- Position und Beschriftung der Umschaltaktion
- Verhalten nach Filterwechsel und Queue-Refresh
- kleinste sinnvolle Testabdeckung

## Nicht-Ziele

- noch keine Implementierung
- keine dedizierte Approval-Seite
- keine Pagination, Sortierung oder neuen Filter
- keine Massenentscheidung
- keine Cockpit-, KPI-, SLA- oder Eskalationslogik
- keine Backend-, API-, Datenbank- oder Berechtigungsaenderung
- kein weiterer Entscheidungsdialog-Test

## Ergebnis

3.1.54.1 ist abgeschlossen. Der kleinste Folgeausbau ist die clientseitige
Ein-/Ausklappfunktion der bestehenden Freigabeanforderungs-Karte. Der naechste
Leaf 3.1.54.2 definiert dafuer das technische Minimalzielmodell.
