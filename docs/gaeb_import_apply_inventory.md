# Importlauf-Freigabe und kontrollierte Quote-Uebernahme: Ist-Stand und kleinster sicherer Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Ausbauabschnitt nach
dem abgeschlossenen Review-MVP fuer `quote_import_items`.

Es geht hier bewusst noch **nicht** um:

- automatische Preisermittlung
- Material- oder Leistungsmapping
- KI-gestuetzte Positionsanreicherung
- Bulk-Konvertierung mehrerer Importlaeufe
- Uebernahme in bestehende, bereits bearbeitete Angebote

Ziel ist nur, den kleinsten **sicheren** Einstiegspunkt nach dem
Positions-Review zu bestimmen:

- wann ein Importlauf fachlich zur Uebernahme bereit ist
- welches Zielobjekt zuerst beschrieben werden darf
- welche Guard Rails vor der ersten Quote-Uebernahme noetig sind

## 1. Ausgangslage nach dem Review-MVP

Der aktuelle Stand ist jetzt:

- `quote_imports` als Importlauf-Container ist vorhanden
- Quelldatei und Metadatenpfad fuer GAEB-Uploads sind vorhanden
- `quote_import_items` als Rohpositionsspeicher ist vorhanden
- Parsernahe Rohpositionen sind read-only sichtbar
- einzelne Importpositionen koennen mit
  - `review_status`
  - `review_note`
  bewertet werden

Damit ist die Strecke

- `Datei hochladen -> Importlauf erzeugen -> Positionen speichern -> lesen -> Positionen reviewen`

sauber aufgebaut.

Die fachliche Luecke verschiebt sich jetzt auf:

- **Importlauf-Freigabe**
- und die **kontrollierte Uebernahme akzeptierter Positionen in eine Quote**

## 2. Was aktuell noch fehlt

Im Repo fehlen aktuell weiterhin:

- eine explizite fachliche Bedeutung von `reviewed` auf Importlauf-Ebene
- ein Guard-Rail-Schnitt, wann ein Importlauf ueberhaupt apply-faehig ist
- eine erste kontrollierte Uebernahme von `accepted`-Positionen in eine Quote
- eine klare Entscheidung, ob zuerst
  - neue Quotes erzeugt
  - oder bestehende Quotes veraendert
  werden duerfen

Die wichtigste offene Frage ist damit nicht mehr:

- wie einzelne Positionen bewertet werden

sondern:

- **wie aus bewerteten Importpositionen ein sicherer Angebotsentwurf entsteht**

## 3. Relevante vorhandene Muster

### 3.1 Die Angebotsdomaene ist bereits eigenstaendig

Das bestehende Quote-Modell in `server/internal/quotes/service.go` ist bereits
stabil und produktiv nutzbar:

- `quotes`
- `quote_items`
- `QuoteInput`
- `Create(...)`

`Create(...)` baut ein neues Angebot aus:

- `project_id`
- `contact_id`
- `currency`
- `note`
- flachen `items`

Das ist wichtig, weil es fuer den ersten Apply-Schritt ein klares, bereits
vorhandenes Zielobjekt gibt:

- **ein neuer Angebotsentwurf**

### 3.2 Review und Zielobjekt sind heute sauber getrennt

Aktuell existiert eine saubere Trennung zwischen:

- `quote_import_items` als Quelle
- `quote_items` als Zielobjekt eines echten Angebots

Diese Trennung ist wertvoll und sollte im ersten Apply-Schritt bewusst
erhalten bleiben.

Ein frueher Schritt

- `accepted`-Positionen direkt in ein bestehendes Angebot hineinschreiben

waere fachlich deutlich riskanter als:

- aus einem freigegebenen Importlauf **ein neues Angebot im Status `draft`**
  zu erzeugen

## 4. Was der kleinste sichere Einstiegspunkt **nicht** ist

Der naechste kleine Block sollte bewusst **nicht** sein:

- bestehende Quotes erweitern oder ueberschreiben
- teilweises Mischen importierter und bereits manuell bearbeiteter Quote-Items
- automatisches Erzeugen finaler Preise aus parsernahen Rohdaten
- Quote-Uebernahme trotz noch offener `pending`-Positionen
- direkter Wechsel eines Importlaufs auf `applied`, ohne Zielquote sauber zu
  persistieren

Diese Varianten waeren fachlich zu hart, weil sie:

- bestehende Zielobjekte mutieren
- die Rueckverfolgbarkeit verschlechtern
- Konflikte mit manuell bearbeiteten Angebotsentwuerfen erzeugen

## 5. Kleinster sichere Einstiegspunkt nach dem Review

Der kleinste sichere naechste Schritt ist:

- **einen vollstaendig reviewed Importlauf kontrolliert in genau eine neue
  Quote im Status `draft` zu uebernehmen**

Dabei sollen nur Positionen mit

- `review_status = accepted`

in die neue Quote einfliessen.

Positionen mit

- `pending`
- `rejected`

duerfen nicht uebernommen werden.

## 6. Empfohlenes Minimalzielbild fuer die Importlauf-Freigabe

### 6.1 Importlauf-Ebene

Vor der ersten Quote-Uebernahme braucht es eine fachliche Freigabelogik auf
Importlauf-Ebene.

Der erste sichere Zuschnitt sollte noch **keine** freie Freigabe trotz offener
Positionen erlauben.

Empfohlene Minimalregel:

- ein Importlauf ist erst dann apply-faehig, wenn **keine**
  `quote_import_items.review_status = 'pending'` mehr vorhanden sind

Das ist streng, aber fuer den ersten Apply-Schritt fachlich gut vertretbar:

- keine halbfertigen Importlaeufe
- kein implizites Ignorieren unbewerteter Positionen

### 6.2 Statusidee

Der bisher vorbereitete Statusraum enthaelt bereits:

- `uploaded`
- `parsed`
- `reviewed`
- `applied`
- `failed`

Der naechste sichere Ausbau sollte daraus erstmals aktiv nutzen:

- `parsed -> reviewed`
- spaeter bei erfolgreicher Quote-Erzeugung `reviewed -> applied`

## 7. Empfohlenes Minimalzielbild fuer die Quote-Uebernahme

Die erste Quote-Uebernahme sollte bewusst klein bleiben:

- Ziel ist immer **eine neue Quote**
- Status der Zielquote ist immer `draft`
- Quelle sind nur `accepted`-Importpositionen
- `project_id` des Importlaufs wird uebernommen
- `contact_id` des Importlaufs wird uebernommen, falls vorhanden
- `currency` bleibt zunaechst auf bestehendem Quote-Default-Pfad
- `note` kann zunaechst leer bleiben oder einen kleinen Herkunftshinweis tragen

### 7.1 Erste sichere Feldabbildung

Der erste Apply-Schritt sollte parsernahe Rohdaten nur so weit abbilden, wie
das bestehende Quote-Modell es ohne neue Preis- oder Mappinglogik erlaubt:

- `description` -> `quote_items.description`
- `qty` -> `quote_items.qty`
- `unit` -> `quote_items.unit`

Fuer den ersten sicheren Wurf sollte gelten:

- `unit_price = 0`
- `tax_code = ''`

Damit entsteht **kein** fachlich falscher Preisautomatismus.

Die eigentliche kaufmaennische Bearbeitung bleibt danach bewusst im normalen
Quote-Editor.

## 8. Kleinste sinnvolle Nutzerreise

Die erste sichere Nutzerreise nach dem Review sollte klein bleiben:

1. Importlauf ist `parsed`
2. alle Importpositionen werden auf `accepted` oder `rejected` gesetzt
3. Importlauf kann als `reviewed` gelten
4. Benutzer startet eine kontrollierte Uebernahme
5. System erzeugt eine **neue Quote im Status `draft`**
6. nur `accepted`-Positionen werden uebernommen
7. Importlauf referenziert die erzeugte Quote

Wichtig:

- noch kein Editieren der Rohdaten waehrend der Uebernahme
- noch keine Uebernahme in bestehende Quotes
- noch keine automatische Preisfindung

## 9. Empfohlene Guard Rails fuer den ersten Apply-Schnitt

Der erste Apply-Pfad sollte mindestens diese Regeln haben:

- Importlauf muss existieren
- Importlauf muss im Status `reviewed` sein
- es muss mindestens eine `accepted`-Position geben
- es darf keine `pending`-Position mehr geben
- `created_quote_id` darf noch nicht gesetzt sein
- dieselbe Importquelle darf nicht mehrfach angewendet werden

Zusatznutzen:

- der Schritt bleibt idempotenznah
- Rueckverfolgung `Importlauf -> erzeugte Quote` bleibt erhalten

## 10. Was bewusst noch nicht Teil dieses Blocks sein sollte

Auch nach dieser Inventur sollte der naechste Apply-Block bewusst **nicht**
vorziehen:

- Uebernahme in bestehende Quotes
- Batch-Freigabe mehrerer Importlaeufe
- automatische Preis- oder Materialzuordnung
- teilweises Re-Apply nach spaeteren Review-Aenderungen
- Aufsplitten nach Leistungsbereichen oder Hierarchieebenen
- KI-gestuetzte Zusammenfassung oder Formulierung der Quote

## 11. Entscheidung

Der naechste sinnvolle Epic-3-Schritt nach dem Positions-Review ist:

- **nicht** weiterer Review-Feinschliff
- **nicht** direkte Uebernahme in bestehende Angebote
- **sondern** zuerst die fachliche Vorbereitung eines kleinen,
  kontrollierten Apply-Pfads

Der kleinste sichere Einstiegspunkt ist damit:

- Importlauf nur dann apply-faehig machen, wenn alle Positionen bewertet sind
- daraus genau **eine neue Quote im Status `draft`** erzeugen
- nur `accepted`-Positionen uebernehmen
- Preise und kaufmaennische Veredelung weiterhin im bestehenden Quote-Editor
  nachziehen

## 12. Naechster sinnvoller Schritt

Nach diesem Dokument ist der naechste kleine und saubere Schritt:

- das minimale technische Zielmodell fuer
  - Importlauf-Freigabe auf `reviewed`
  - und kontrollierte Quote-Uebernahme nach `applied`
  vorzubereiten,
  ohne schon die eigentliche Apply-Logik breit auszubauen
