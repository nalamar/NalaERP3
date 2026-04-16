# GAEB-Importlauf MVP: Abschluss-Audit

## Ziel dieses Dokuments

Dieses Dokument bewertet den aktuell umgesetzten **parserfreien**
GAEB-Importlauf nach Backend- und Client-Anbindung.

Ziel ist eine enge Entscheidung:

- ist der MVP-Block vor Parser-/Review-Logik bereits sauber abgeschlossen
- oder gibt es **genau einen** weiteren kleinen Haertungsschritt mit gutem
  Signal

## 1. Was im MVP bereits steht

Der parserfreie Importlauf ist inzwischen in beiden Schichten vorhanden.

### 1.1 Backend

Vorhanden:

- `quote_imports` als Importlauf-Container
- Dateiablage in GridFS
- Metadaten in Postgres
- kleine Endpunkte
  - `POST /api/v1/quotes/imports/gaeb`
  - `GET /api/v1/quotes/imports`
  - `GET /api/v1/quotes/imports/{id}`
- Integrationstest fuer `Upload -> List -> Detail`

### 1.2 Client

Vorhanden:

- Upload-Call in `client/lib/api.dart`
- Sichtbarkeit innerhalb der Angebotsumgebung
- kompakte Liste projektbezogener Importlaeufe
- read-only Detaildialog fuer einzelne Importlaeufe

## 2. Was bewusst noch nicht dazugehört

Nicht Teil dieses MVP-Blocks:

- echter GAEB-Parser
- Format-Erkennung
- Positionsspeicherung
- Review-UI fuer importierte LV-Daten
- Uebernahme in `quote_items`
- automatische Quote-Erzeugung
- KI-Logik

Diese Punkte sind gross genug, um **nicht** mehr als kleine Haertung zu
gelten.

## 3. Bewertung des aktuellen Zustands

Der aktuelle Stand ist fachlich bereits brauchbar fuer einen ersten
Import-Run-Anker:

- Upload ist projektbezogen verankert
- Quelldatei und Metadaten sind nachvollziehbar gespeichert
- Importlaeufe sind sichtbar
- der Pfad verunreinigt die Quote-Domaene noch nicht

Damit ist der zentrale Architekturentscheid sauber umgesetzt:

- **nicht** `GAEB -> Quote`
- **sondern** `GAEB-Datei -> Importlauf mit Review-Anker`

## 4. Ein verbleibender kleiner Haertungsschritt mit gutem Signal

Es bleibt genau **ein** kleiner Schritt uebrig, der vor Parser-/Review-Logik
noch guten Nutzen hat:

- **fachliche Dateityp-Grenze fuer GAEB-Uploads haerten**

### 4.1 Warum dieser Schritt sinnvoll ist

Aktuell ist der Pfad zwar als GAEB-Upload benannt, technisch wird aber noch
jede beliebige Datei angenommen, solange der Multipart-Upload formal stimmt.

Das erzeugt unnoetige Unschaerfe:

- Nutzer koennen versehentlich PDF, ZIP oder beliebige Office-Dateien in den
  GAEB-Pfad laden
- der Importlauf wirkt dadurch fachlich offener als er ist
- spaetere Parserfehler wuerden unnoetig bereits auf der falschen Ebene
  entstehen

### 4.2 Warum dieser Schritt noch klein genug ist

Der Schritt braucht:

- keine Parserlogik
- keine neue Persistenz
- keine neue UI-Struktur
- keine Review-Modelle

Noetig waere nur:

- kleine serverseitige Validierung auf typische GAEB-Endungen
  - z. B. `.x83`, `.x84`, `.d83`, `.p83`, `.gaeb`, `.xml`
- saubere Fehlermeldung bei nicht erlaubtem Dateityp
- optional spiegelnde Client-Fehlermeldung, aber keine neue UI

## 5. Schritte, die bewusst nicht mehr "klein" sind

Folgende Punkte wurden geprueft und bewusst **nicht** als naechster
Haertungsschritt eingeordnet:

- Download der Quelldatei
  - nuetzlich, aber bereits ein eigener UX-Ausbau
- Kontakt-Auswahl direkt im Uploaddialog
  - sinnvoll, aber kein Harterfordernis fuer den Review-Anker
- globale Importseite ausserhalb der Angebotsumgebung
  - klar groesserer UI-Ausbau
- Pagination / Filter-UI fuer Importlisten
  - ebenfalls groesser als der aktuelle MVP-Block

## 6. Entscheidung

Der parserfreie GAEB-Importlauf ist **noch nicht ganz abgeschlossen**.

Es bleibt genau ein weiterer kleiner Haertungsschritt mit gutem Signal:

- **serverseitige Dateityp-Validierung fuer den GAEB-Uploadpfad**

Danach sollte der Block sauber beendet werden, bevor Parser-, Review- oder
KI-Logik begonnen wird.
