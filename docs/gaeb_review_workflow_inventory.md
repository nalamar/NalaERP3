# GAEB-Review fuer Importpositionen: Ist-Stand und kleinster schreibender Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Ausbauabschnitt nach
dem abgeschlossenen parsernahen Read-only-Block fuer `quote_import_items`.

Es geht hier bewusst noch **nicht** um:

- direkte Quote-Uebernahme
- Preis- oder Material-Mapping
- KI-gestuetzte Anreicherung
- Vollabdeckung komplexer GAEB-Hierarchien

Ziel ist nur, die **Review-Zielstrecke** fuer importierte GAEB-Positionen so
zu schneiden, dass der erste schreibende Schritt klein, fachlich sauber und
technisch reversibel bleibt.

## 1. Ausgangslage nach dem Read-only-Block

Der aktuelle Stand ist jetzt:

- `quote_imports` als Importlauf-Container ist vorhanden
- `quote_import_items` als flacher Rohpositionsspeicher ist vorhanden
- Importlaeufe koennen `uploaded`, `parsed` oder `failed` sein
- Rohpositionen sind ueber Backend-Lesepfade abrufbar
- Rohpositionen sind in der Angebotsumgebung read-only sichtbar

Damit ist die Strecke

- `Datei hochladen -> Importlauf erzeugen -> Rohpositionen speichern -> lesen`

sauber abgeschlossen.

Die eigentliche Luecke verschiebt sich jetzt auf

- **manuelle Review vor jeder spaeteren Quote-Uebernahme**.

## 2. Was aktuell noch fehlt

Im Repo fehlen aktuell weiterhin:

- ein kleiner Schreibpfad fuer Review-Entscheidungen je Importposition
- eine klare fachliche Bedeutung der Item-Review-Stati im Benutzerfluss
- eine explizite Importlauf-Freigabe von `parsed` nach `reviewed`
- Guard Rails, wann ein Importlauf als reviewt gelten darf

Die zentrale fachliche Luecke ist damit nicht mehr Parsing oder Sichtbarkeit,
sondern:

- wie Benutzer einzelne Rohpositionen annehmen oder verwerfen
- und wann ein gesamter Importlauf als ausreichend geprueft gilt

## 3. Relevante vorhandene Muster

### 3.1 Read-only-Stufe ist bereits sauber getrennt

Der aktuelle Stand trennt bereits sauber zwischen:

- Importlauf-Metadaten
- Rohpositionen
- finalen `quote_items`

Diese Trennung ist wichtig und darf auch im Review-Schritt nicht aufgeweicht
werden.

### 3.2 Review muss vor Quote-Uebernahme liegen

Die vorhandene Angebotsdomaene ist bereits ausgereift:

- `quotes`
- `quote_items`
- manueller Quote-Editor

Deshalb waere ein frueher Schritt

- `parsed -> direkt in quote_items uebernehmen`

fachlich zu hart und wuerde Korrekturbedarf direkt in die Zielobjekte
verschieben.

## 4. Kleinster sinnvolle schreibende Einstiegspunkt

Der kleinste sinnvolle naechste Block ist **nicht**

- komplette Review-Seite
- Sammelaktionen fuer ganze Importe
- Quote-Erzeugung aus akzeptierten Positionen

sondern

- **kleiner Item-Review-Pfad auf einzelnen `quote_import_items`**

### 4.1 Empfohlene erste schreibende Aktion

Die erste neue Aktion sollte pro Position nur Folgendes erlauben:

- `review_status` setzen
- optionale `review_note` pflegen

Mehr nicht.

Kein Editieren von:

- `description`
- `qty`
- `unit`
- `position_no`

Damit bleibt der erste Review-Schritt eine **fachliche Freigabe-/Verwerfungs-
entscheidung**, nicht schon ein kompletter Rohdaten-Editor.

## 5. Empfohlenes Minimalzielbild fuer Review-Stati

### 5.1 Item-Ebene

Fuer den ersten schreibenden Review-Block reicht:

- `pending`
- `accepted`
- `rejected`

Semantik:

- `pending`
  - Position wurde noch nicht fachlich bewertet
- `accepted`
  - Position ist grundsaetzlich fuer spaetere Uebernahme geeignet
- `rejected`
  - Position soll nicht in eine spaetere Quote-Uebernahme einfliessen

### 5.2 Importlauf-Ebene

Der Importlauf selbst sollte im naechsten Block noch **nicht** automatisch bei
jedem Item-Schreibvorgang auf `reviewed` springen.

Stattdessen sollte die Importlauf-Freigabe erst in einem spaeteren kleinen
Schritt kommen, wenn klar ist:

- ob alle Items bewertet sein muessen
- oder ob `accepted/rejected` fuer eine Teilmenge reicht

Damit bleibt der erste Review-Schritt bewusst enger.

## 6. Kleinste sinnvolle Nutzerreise

Der erste schreibende Review-Flow sollte klein bleiben:

1. Importlauf ist `parsed`
2. Benutzer oeffnet Rohpositionsliste
3. Benutzer markiert einzelne Positionen als `accepted` oder `rejected`
4. Benutzer kann optional eine kurze `review_note` hinterlegen
5. Alles andere bleibt unveraendert

Wichtig:

- noch keine Sammelfreigabe
- noch keine Quote-Uebernahme
- noch kein Rohdaten-Edit

## 7. Empfohlener kleinster technischer Einstiegspunkt

Der erste schreibende Review-Block sollte voraussichtlich nur einen sehr
kleinen API-Schnitt einfuehren:

- `PATCH /api/v1/quotes/imports/{id}/items/{itemID}/review`

Minimaler Body:

- `review_status`
- `review_note`

Guard Rails:

- nur fuer Importlaeufe im Status `parsed`
- nur fuer erlaubte Review-Stati
- Item muss zum angegebenen Importlauf gehoeren

## 8. Was bewusst noch nicht Teil dieses Blocks sein sollte

Fuer den naechsten Review-Schritt bewusst **nicht** vorziehen:

- Inline-Bearbeitung der Rohpositionsdaten
- Bulk-Actions wie "alle akzeptieren"
- Importlauf auf `reviewed` setzen
- Quote-Uebernahme aus `accepted`-Positionen
- Mapping auf Material-, Leistungs- oder Preisstrukturen
- KI-Hinweise oder automatische Review-Vorschlaege

## 9. Entscheidung

Der naechste sinnvolle Epic-3-Schritt nach dem parsernahen Read-only-Block
ist:

- **nicht** weiterer Read-only-Ausbau
- **nicht** direkte Quote-Uebernahme
- **sondern** die fachliche Vorbereitung eines kleinen schreibenden
  Item-Review-Pfads

Der kleinste saubere Einstiegspunkt ist damit:

- `quote_import_items` read-only lassen, aber
- pro Position genau `review_status` und `review_note` schreibbar machen
- Importlauf-Freigabe und Quote-Uebernahme erst spaeter anschliessen
