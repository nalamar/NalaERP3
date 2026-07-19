# GAEB-Folgeausbau nach Zielmargen-Konfiguration:
# Zielpreis-Uebernahme vs. Freigabe-Hinweis

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten fachlichen Folgepfad nach der
abgeschlossenen Zielmargen-Konfiguration.

Der Fokus bleibt bewusst eng:

- entscheiden, ob als naechstes eine kontrollierte Zielpreis-Uebernahme oder
  ein Freigabe-Schreibpfad den besten Signalwert hat
- den kleinsten sicheren naechsten Leaf bestimmen
- Zielwert, Zielpreis, Freigabe und Automatik weiterhin sauber trennen

## 1. Ausgangslage

Der aktuelle Stand deckt bereits ab:

- gespeicherte Preisentscheidung als Kostenbasis
- read-only Margenanker
- read-only Approval-Hint
- read-only Zielmargenanker
- persistenter globaler Zielmargenwert in `quote_calculation_settings`
- Settings-API `GET/PUT /api/v1/settings/quote-calculation`
- minimale Flutter-Settings-UI fuer den globalen Zielmargenwert
- explizite Preisuebernahme aus der primaeren Preisquelle

Damit ist die Zielwertquelle nicht mehr nur ein technischer Default. Der
Zielmargenanker kann einen fachlich sichtbaren globalen Zielwert verwenden und
positionsnah Zielpreis sowie Zielabweichung anzeigen.

## 2. Verbleibende Fachliche Luecke

Nach der Zielmargen-Konfiguration bleibt diese Luecke:

- Der Zielpreis ist sichtbar, aber noch nicht kontrolliert uebernehmbar.
- Der Approval-Hint empfiehlt Aufmerksamkeit, startet aber keinen Workflow.
- Es gibt noch keinen persistenten Freigabezustand, keine Kommentare und keine
  Rollenentscheidung fuer Angebotsfreigaben.

Die naechste Entscheidung ist deshalb:

- zuerst eine kleine, explizite Zielpreis-Uebernahme pro Position
- oder direkt ein echter Freigabe-Schreibpfad

## 3. Option A: Kontrollierte Zielpreis-Uebernahme

Eine kontrollierte Zielpreis-Uebernahme wuerde:

- genau eine Draft-Quote-Position adressieren
- den Zielpreis serverseitig erneut aus Kostenbasis und Zielmargen-Settings
  berechnen
- `quote_items.unit_price` auf diesen Zielpreis setzen
- `price_mapping_status = 'manual'` setzen oder halten
- die aktualisierte Quote zurueckgeben

Vorteile:

- schliesst direkt an den sichtbaren Zielmargenanker an
- bleibt positionsnah und explizit
- nutzt bereits vorhandene Kostenbasis und Zielwertquelle
- braucht keine neue Freigabe-Persistenz
- bleibt kleiner als Workflow, Rollenmatrix oder Angebotsfreigabe

Risiken:

- veraendert erstmals Verkaufspreise aus dem Zielmargenanker
- muss klar als Nutzeraktion modelliert werden
- darf keine Automatik oder Bulk-Uebernahme einfuehren
- muss verhindern, dass fehlende Kostenbasis oder nicht-draft Quotes mutiert
  werden

## 4. Option B: Echter Freigabe-Schreibpfad

Ein echter Freigabe-Schreibpfad wuerde mindestens brauchen:

- persistente Freigabe-Entitaet oder Statusfelder
- Statusraum wie `requested`, `approved`, `rejected`
- Benutzer- oder Rollenbezug
- Zeitstempel und optional Kommentare
- Regeln, wann eine Freigabe erforderlich ist
- UI fuer Anfordern, Entscheiden und Historie

Vorteile:

- bildet spaeter reale Verantwortlichkeiten ab
- passt langfristig zu negativer Marge oder Zielabweichung
- schafft Auditierbarkeit

Risiken:

- ist deutlich groesser als der aktuelle Positionspfad
- vermischt Schwellwertlogik, Rollen, Historie und Workflow
- braucht zuerst Entscheidungen zu Berechtigungen und Statusmodell
- waere als naechster Leaf zu gross, wenn nur eine Subtask bearbeitet werden
  soll

## 5. Entscheidung

Der kleinste sichere naechste Folgepfad ist:

- technische Strategie fuer eine explizite Zielpreis-Uebernahme pro
  Quote-Position

Noch nicht direkt implementieren:

- Zielpreis-Uebernahme-Endpunkt
- Freigabe anfordern
- Freigabe erteilen oder ablehnen
- Freigabehistorie
- Rollenmatrix
- Bulk-Aktion
- Automatik

Begruendung:

- Der Freigabe-Schreibpfad ist fachlich wichtig, aber fuer den naechsten Leaf
  zu breit.
- Die Zielpreis-Uebernahme ist kleiner, weil Kostenbasis, Zielwert und
  Zielpreis bereits existieren.
- Vor dem Write-Pfad braucht es einen technischen Zuschnitt, damit keine
  Preisautomatik oder verdeckte Kalkulationsregel entsteht.

## 6. Minimalziel Fuer Den Naechsten Leaf

Der naechste Leaf sollte nur das technische Zielmodell fuer die
Zielpreis-Uebernahme zuschneiden.

Zu entscheiden sind:

- Service-Methode, zum Beispiel
  `ApplyTargetUnitPriceForQuoteItem(ctx, quoteID, itemID)`
- API-Pfad, zum Beispiel
  `POST /api/v1/quotes/{id}/items/{itemID}/apply-target-price`
- Guard Rails:
  - Quote existiert
  - Quote ist aktuelle Version
  - Quote ist Draft
  - Position gehoert zur Quote
  - gespeicherte Preisentscheidung als Kostenbasis existiert
  - Zielmargen-Settings sind lesbar oder Default-Fallback ist eindeutig
- Preisberechnung:
  - `target_unit_price = cost_basis_unit_price * (1 + target_margin_percent / 100)`
  - Rundung auf die im System uebliche Preisgenauigkeit
- Seiteneffekte:
  - nur `unit_price` und Mappingstatus der Position aendern
  - Quote-Summen konsistent neu berechnen
  - keine neue Freigabe-Persistenz
- Antwort:
  - aktualisierte Quote

## 7. Nicht-Ziele Des Naechsten Leaf

Nicht Teil des naechsten Leaf:

- Implementierung der Zielpreis-Uebernahme
- Client-Button
- Zielpreis-Historie
- Freigabe-Workflow
- Kommentare
- Rollen
- angebotsweite Zielmargenpruefung
- Zielwerte je Kunde, Projekt oder Materialgruppe
- automatische Preisaktualisierung
- Bulk-Uebernahme
- KI-gestuetzte Zielpreisfindung

## 8. Abhaengigkeiten Und Vorbedingungen

Vor einem spaeteren Implementierungs-Leaf sollten die auditieren Kanten aus
`docs/gaeb_target_margin_config_audit.md` bewusst bewertet werden:

- Soll `0.00` als explizite Zielmarge erlaubt sein?
- Soll Settings-GET selbstheilend sein, wenn `id='default'` fehlt?
- Soll lokale Client-Validierung `0..1000` vor einem Zielpreis-Write-Pfad
  geschaerft werden?
- Bleiben read-only Bewertungsanker bei `quotes.write` oder wechseln sie auf
  `quotes.read`?

Diese Punkte blockieren das Zielmodell nicht, sollten aber vor einer
produktiven Preisuebernahme nicht vergessen werden.

## 9. Naechster Schritt

Der naechste Leaf ist:

- `3.1.31.2`: Technisches Minimalzielmodell fuer die kontrollierte
  Zielpreis-Uebernahme pro Quote-Position zuschneiden

Dieser Leaf bleibt dokumentierend und entscheidet erst danach ueber eine
moegliche Backend-Implementierung.
