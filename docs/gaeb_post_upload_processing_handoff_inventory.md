# GAEB: Folgeinventar nach abgesicherten Upload-Endzustaenden

## Ziel

Subtask 3.1.73.1 bestimmt den kleinsten fachlich wertvollen GAEB-Risikobereich
ausserhalb des nun vollstaendig getesteten Client-Upload-Eingangs. Dieser Leaf
aendert keine Runtime- oder Testlogik.

## Abgesicherter Stand

Der Clientvertrag fuer QuotesPage._importGAEB() unterscheidet jetzt:

- fehlende Projekt-ID vor der Dateiauswahl;
- vom Benutzer abgebrochene Dateiauswahl;
- erfolgreichen Upload mit Listen-Reload und Erfolgshinweis;
- strukturierte fachliche Uploadablehnung;
- technischen, nicht als ApiException modellierten Uploadfehler.

Ein erfolgreicher Upload erzeugt serverseitig einen quote_imports-Datensatz im
Status uploaded und speichert die Quelldatei in GridFS.

## Unmittelbare fachliche Luecke

Nach dem Upload gibt es keinen produktiven Handoff von uploaded nach parsed
oder failed:

- CreateGAEBImport speichert Quelle und Importlauf;
- SaveImportParseResult und MarkImportFailed existieren nur als
  serviceinterne Verarbeitungsschnittstellen;
- es gibt keinen Parseradapter, keinen Worker und keinen API-Ausloeser, der
  die gespeicherte GAEB-Quelle liest und den Importlauf ueberfuehrt.

Ein erfolgreich hochgeladener Import kann deshalb im Fachprozess dauerhaft im
Status uploaded stehen. Review, Apply und die vorhandene Angebotsstrecke sind
erst ab parsed beziehungsweise reviewed erreichbar.

## Optionen im Vergleich

### Option A: Minimalen Verarbeitungshandoff planen

Der vorhandene Importlauf wird nach dem Upload durch einen klar abgegrenzten,
synchronen Parseradapter verarbeitet. Der Adapter liest die GridFS-Quelle und
uebergibt entweder normalisierte Positionen an SaveImportParseResult oder
einen strukturierten Fehler an MarkImportFailed.

Bewertung: kleinster Schritt, der den bereits vorhandenen Statusraum wieder
an die erreichbare Review-Strecke anschliesst.

### Option B: Nur uploaded im Client erklaeren

Die UI koennte den wartenden Status sichtbarer beschreiben.

Bewertung: verbessert Transparenz, loest aber nicht die fachliche Stagnation
und kann erst nach einer klaren Verarbeitungssemantik sinnvoll formuliert
werden.

### Option C: GAEB-KI-Mapping direkt anschliessen

Material- oder Preisvorschlaege koennten nach dem Upload erzeugt werden.

Bewertung: setzt verlässlich geparste Positionen, Quellverweise und einen
reproduzierbaren Parservertrag voraus; daher zu frueh.

### Option D: GridFS- und Postgres-Kompensation haerten

Ein fehlgeschlagener Datenbankinsert nach erfolgreichem GridFS-Upload koennte
die Quelldatei bereinigen.

Bewertung: ein relevanter technischer Robustheitspunkt, aber ohne
Verarbeitungshandoff bleibt die zentrale fachliche Strecke weiterhin
unterbrochen. Er folgt sinnvoll nach dem minimalen Prozessvertrag.

## Entscheidung

Der kleinste verbleibende fachliche Risikobereich ist der fehlende
Verarbeitungshandoff von uploaded zu parsed oder failed. Er wird in einem
eigenen, eng zugeschnittenen Block vorbereitet:

1. **3.1.73.2** – Minimalen Parseradapter- und Statusuebergabevertrag
   definieren, ohne ein GAEB-Format zu implementieren.
2. **3.1.73.3** – Kleinste synchrone Verarbeitungsnaht fuer einen
   deterministischen Parseradapter implementieren und testen.
3. **3.1.73.4** – Handoff-Implementierung und Nachweise auditieren.

## Nicht-Ziele

- keine GAEB-X83-, X84-, D83-, P83- oder XML-Parserimplementierung;
- keine KI-, Mapping-, Preis- oder Kalkulationslogik;
- kein Worker, Queueing, Retry oder asynchrone Orchestrierung;
- keine UI- oder Berechtigungsaenderung;
- keine GridFS-Kompensation oder Aufraeumlogik.

## Naechster Leaf

Subtask 3.1.73.2 definiert ausschliesslich das minimale Adapterinterface,
die Lesegraenze zur gespeicherten Quelle, die beiden erlaubten
Statusausgaenge und die gezielten Testfaelle.

## Ergebnis

3.1.73.1 ist abgeschlossen. Der fehlende Uebergang nach erfolgreichem Upload
ist der kleinste fachliche Anschluss, bevor Review, Angebotsuebernahme und
spaetere KI-Verarbeitung wieder erreichbar werden.
