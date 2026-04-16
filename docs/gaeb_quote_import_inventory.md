# GAEB-Import fuer Angebotsvorbereitung: Ist-Stand und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den aktuellen Repo-Stand rund um Uploads,
Importe und Angebotsanlage, um den **kleinsten risikoarmen Einstiegspunkt**
fuer einen spaeteren GAEB-Importpfad festzulegen.

Es geht in diesem Schritt **bewusst noch nicht** um:

- echten GAEB-Parser
- automatische Quote-Erzeugung
- KI-Preisfindung oder KI-Textableitung
- Formatabdeckung fuer alle GAEB-Varianten

## 1. Ausgangslage im Repo

### 1.1 Angebotslogik ist bereits vorhanden

Der Angebotsstack ist bereits substanziell ausgebaut:

- `server/internal/quotes/service.go`
- `client/lib/pages/quotes_page.dart`
- Quote-Status, Positionen, PDF-Ausgabe, Konvertierung in Auftrag/Rechnung

Der Quote-Editor arbeitet aktuell manuell mit:

- `project_id`
- `contact_id`
- `currency`
- freiem `note`
- frei gepflegten Positionen

Das ist wichtig, weil ein spaeterer GAEB-Pfad nicht bei null startet, sondern
in einen bereits funktionierenden Quote-Editor bzw. Quote-Service einspeisen
muss.

### 1.2 Es gibt bereits Upload- und Dokumentmuster

Fuer Dateien existieren im Repo belastbare Muster:

- Kontaktdokumente ueber GridFS
  - `server/internal/contacts/documents.go`
- Materialdokumente ueber GridFS
  - `server/internal/materials/documents.go`
- PDF-Template-Bilder in Settings
  - `client/lib/api.dart`
  - `server/internal/http/v1.go`

Das bedeutet:

- Datei-Uploads sind architektonisch kein neues Thema
- Dokumente koennen bereits getrennt von fachlichen Stammdaten gespeichert
  werden
- ein erster GAEB-Schritt muss keinen neuen Storage-Stack erfinden

### 1.3 Es gibt bereits ein Importmuster mit Review-/Undo-Gedanken

Im Projektbereich existiert mit LogiKal bereits ein Importpfad:

- `server/internal/projects/import_logikal.go`
- `client/lib/pages/projects_page.dart`
- `project_imports` / `project_import_changes`

Wichtige Architekturmerkmale dieses Musters:

- Import ist ein eigenstaendiger Vorgang
- es gibt eine Import-Historie
- Aenderungen werden nicht stillschweigend versteckt
- Undo/Review ist als Denkmuster bereits vorhanden

Der LogiKal-Import ist fachlich zwar etwas anderes als GAEB, zeigt aber,
dass das Repo bereits mit **import-run-orientierten Prozessen** umgehen kann.

## 2. Was aktuell fuer GAEB fehlt

Im Repo gibt es derzeit **keinen belegten GAEB-Pfad**:

- keine GAEB-Endpunkte
- keine GAEB-Dokumenttabelle
- kein Parser fuer `.x83`, `.x84`, `.d83`, `.p83` oder XML-Varianten
- keine Import-Review-Struktur fuer Angebotspositionen
- kein fachliches Zwischenobjekt zwischen Quelldatei und finalem Angebot

Die eigentliche Luecke liegt daher nicht bei "Datei hochladen", sondern bei:

- fachlicher Modellierung importierter LV-Daten
- Review vor Quote-Erzeugung
- klarer Trennung zwischen Quellimport und finaler Angebotserstellung

## 3. Risiken bei einem zu direkten Einstieg

Ein direkter Einstieg mit "GAEB hochladen -> sofort Quote erzeugen" waere zu
frueh und zu riskant.

Gruende:

- GAEB-Daten enthalten haeufig strukturierte LV-Positionen, aber nicht
  automatisch verkaufsfertige Angebotspositionen
- Positionstexte, Mengeneinheiten und Gliederung muessen oft manuell geprueft
  werden
- Preislogik ist im Repo fuer diesen Pfad noch nicht modelliert
- spaetere KI-Unterstuetzung braucht zuerst ein sauberes Zielobjekt fuer
  importierte Rohdaten und Review-Ergebnisse

Deshalb ist ein parsergetriebener Direktpfad aktuell der falsche erste Schritt.

## 4. Kleinster risikoarmer Einstiegspunkt

Der kleinste sinnvolle Einstiegspunkt ist **kein Parser**, sondern ein
kontrollierter **GAEB-Vorbereitungs- und Review-Pfad**.

### 4.1 Empfohlenes erstes Zielobjekt

Die erste fachliche Einheit sollte ein **GAEB-Importlauf fuer die
Angebotsvorbereitung** sein, nicht sofort eine Quote.

Minimaler Zweck:

- Quelldatei hochladen
- Importlauf einem Projekt und optional einem Kontakt zuordnen
- Datei und Metadaten speichern
- Status eines Importlaufs verfolgen
- spaeteren Parser- und Review-Schritten einen stabilen Anker geben

### 4.2 Empfohlene minimale Statusidee

Fuer ein spaeteres MVP genuegt ein kleiner Statusraum:

- `uploaded`
- `parsed`
- `reviewed`
- `applied`
- `failed`

In `3.1.5.1` wird das noch nicht implementiert, aber dieser Statusraum ist
klein genug, um spaeter Parser- und Review-Logik daran aufzubauen.

## 5. Kleinste sinnvolle Dateipfad-Entscheidung

Der erste GAEB-Pfad sollte nicht direkt an `quotes` haengen, sondern an einen
separaten Vorbereitungsbereich.

Begruendung:

- Quotes sind bereits kaufmaennisch wirksame Belege
- importierte Rohdaten sind noch keine freigegebenen Angebotspositionen
- Parserfehler, unvollstaendige LV-Strukturen oder unklare Mengen duerfen nicht
  sofort die Quote-Domaene verunreinigen

Deshalb sollte der erste echte technische Schritt spaeter eher in Richtung
eines neuen Bereichs gehen wie:

- `quote_imports`
- `quote_import_items`

und nicht als direkter Side-Effect in `quotes` / `quote_items`.

## 6. Empfohlene erste Nutzerreise

Die erste Nutzerreise fuer einen spaeteren MVP sollte sehr klein bleiben:

1. GAEB-Datei einer Angebotsvorbereitung zuordnen
2. Quelldatei speichern und Importlauf anlegen
3. Metadaten und Verarbeitungsstatus sichtbar machen
4. erst in einem spaeteren Schritt geparste Positionen reviewen
5. erst danach bewusst in eine Quote uebernehmen

Damit bleibt die Trennung sauber zwischen:

- Quelle
- importierter Rohstruktur
- geprueftem Review-Ergebnis
- finalem Angebot

## 7. Was als naechster technischer Schritt sinnvoll ist

Der naechste sinnvolle Schritt nach dieser Inventur ist **noch nicht**
ein Parser, sondern die minimale technische Vorbereitung eines
GAEB-Importlauf-Modells.

Empfohlener naechster Block:

1. neues fachliches Zielmodell fuer `quote_imports` inventarisieren
2. Upload-/Status-/Metadatenpfad festlegen
3. bewusst nur Quelle und Review-Anker modellieren
4. Parserlogik erst danach vorbereiten

## 8. Bewusst noch nicht Teil dieses Blocks

- Parsing einzelner GAEB-Formate
- Normalisierung von Positionen
- automatische Uebernahme in `quote_items`
- Kalkulations- oder Preisfindungslogik
- KI-Interpretation von LV-Texten
- vollautomatische Quote-Erzeugung aus GAEB

## 9. Entscheidung

Der kleinste risikoarme Einstiegspunkt fuer den GAEB-Pfad ist:

- **nicht** `GAEB -> Quote` in einem Schritt
- **sondern** `GAEB-Datei -> Importlauf mit Review-Anker`

Damit entsteht zuerst eine belastbare Vorstufe fuer:

- spaetere Parser
- spaetere manuelle Review-Flows
- spaetere KI-Unterstuetzung
- spaetere finale Quote-Erzeugung
