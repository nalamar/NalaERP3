# GAEB Review MVP Audit

## Ziel

Pruefen, ob nach dem ersten schreibenden Review-Pfad fuer einzelne
`quote_import_items` noch genau ein weiterer kleiner Haertungsschritt mit gutem
Signal uebrig ist oder ob der Block vor Importlauf-Freigabe und
Quote-Uebernahme sauber beendet werden sollte.

## Umgesetzter Stand

Der aktuelle GAEB-Review-MVP umfasst bewusst nur den kleinsten moeglichen
Schreibpfad auf Positionsebene:

- Rohpositionen liegen bereits in `quote_import_items`
- Lesepfade fuer Importpositionen sind vorhanden
- ein einzelner Schreibpfad fuer `review_status` und `review_note` ist
  serverseitig angebunden
- die Client-Anbindung lebt direkt im bestehenden Importpositionsdialog
- zulaessige Review-Stati sind eng auf `pending`, `accepted`, `rejected`
  begrenzt
- Review-Schreiben ist nur fuer Importe im Status `parsed` erlaubt

Nicht Teil dieses MVP sind weiterhin:

- Batch-Review ueber mehrere Positionen
- automatischer Wechsel des Importlaufs auf `reviewed`
- Importlauf-Freigabe als eigener Schritt
- Quote-Uebernahme aus akzeptierten Positionen
- Rohdatenbearbeitung an `description`, `qty`, `unit` oder Strukturfeldern
- Preis-, Material- oder KI-Mapping

## Audit

### 1. Scope-Qualitaet

Der aktuelle Zuschnitt ist technisch und fachlich konsistent:

- Review erfolgt auf der kleinsten fachlich sinnvollen Einheit, der
  Importposition
- der Schreibpfad bleibt getrennt von Parser, Freigabe und Quote-Erzeugung
- bestehende Guard Rails verhindern Review auf ungeparsten Importlaeufen
- die Client-Einbindung zeigt den Nutzen des Pfads sofort, ohne neue Screens
  aufzubauen

### 2. Verbleibende kleine Haertungsschritte

Es bleibt innerhalb dieses Review-MVP **kein weiterer kleiner
Haertungsschritt mit gutem Signal** uebrig.

Moegliche naheliegende Folgepunkte waeren zwar:

- Review-Summen oder Zaehler pro Importlauf
- Batch-Aktionen `alle akzeptieren` oder `alle ablehnen`
- Hervorhebung reviewed vs. pending direkt in der Importliste
- automatische Importlauf-Freigabe bei Vollstaendigkeit

Diese Schritte waeren aber bereits eigenstaendige Ausbauphasen und nicht mehr
bloss Haertung des vorhandenen Minimalpfads. Sie fuehren jeweils entweder neue
Aggregationslogik, neue Interaktionsmuster oder bereits den Uebergang zur
Importlauf-Freigabe ein.

### 3. Risiko- und Signalbewertung

Der aktuelle Block hat gutes Signal, weil er:

- den ersten echten Review-Schreibpfad liefert
- ohne neue Komplexitaetsachse auskommt
- vorhandene Parser- und Read-only-Bausteine sinnvoll erweitert

Ein weiterer Mini-Schritt innerhalb desselben Blocks wuerde dagegen nur noch
lokalen Komfort liefern, aber keine neue tragfaehige Fachfaehigkeit.

## Entscheidung

Der Block **`3.1.7` kann nach dem ersten schreibenden Review-Pfad sauber
beendet werden**.

Der naechste sinnvolle Ausbau liegt nicht in weiterem Review-Feinschliff,
sondern in der naechsten fachlichen Stufe:

1. Review-Ergebnisse auf Importlauf-Ebene sichtbar machen
2. einen kleinen Freigabe-/Apply-Schnitt fuer den Importlauf definieren
3. erst danach die kontrollierte Quote-Uebernahme aus akzeptierten Positionen
   angehen

## Empfohlener naechster Schritt

`3.1.8.1: Importlauf-Freigabe und kontrollierte Quote-Uebernahme fachlich
inventarisieren und den kleinsten sicheren Einstiegspunkt nach
Positions-Review festlegen`
