# GAEB-Parser und Review fuer Importpositionen: Ist-Stand und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Ausbauabschnitt nach
dem nun abgeschlossenen parserfreien GAEB-Importlauf.

Es geht hier bewusst noch **nicht** um:

- Implementierung eines echten GAEB-Parsers
- Formatabdeckung fuer einzelne GAEB-Varianten
- automatische Quote-Erzeugung
- KI-Preislogik oder KI-Textableitung

Ziel ist nur, das **Parser- und Review-Zielbild** fuer importierte Positionen
so zu schneiden, dass der naechste Block klein, fachlich sauber und technisch
reversibel bleibt.

## 1. Ausgangslage nach Abschluss des parserfreien Importlaufs

Der aktuelle Stand ist jetzt:

- `quote_imports` als Importlauf-Container ist vorhanden
- Quelldateien werden in GridFS gespeichert
- Projektbezug ist erzwungen, Kontaktbezug optional
- Status und Metadaten sind sichtbar
- der Uploadpfad ist auf zulaessige GAEB-Dateitypen begrenzt

Damit ist der Einstiegspunkt `GAEB-Datei -> Importlauf` sauber abgeschlossen.

Die eigentliche Luecke verschiebt sich jetzt von **Datei + Metadaten** auf
**geparste Positionen + Review vor Quote-Uebernahme**.

## 2. Was fuer die naechste Stufe noch fehlt

Im Repo fehlen aktuell weiterhin:

- ein Zielobjekt fuer geparste LV-Positionen
- eine persistente Review-Struktur je Importlauf
- Parserfehler auf Positions- oder Dateiebene
- ein Statusuebergang von `uploaded` nach `parsed` oder `failed`
- eine bewusste Freigabestufe vor Uebernahme in `quote_items`

Die zentrale fachliche Luecke ist damit nicht "Datei lesen", sondern:

- wie importierte Rohpositionen gespeichert werden
- wie Benutzer sie pruefen und korrigieren
- und erst danach in die Angebotsdomaene uebernehmen

## 3. Relevante Muster im vorhandenen Repo

### 3.1 Quote-Zieldomaene ist bereits klar

Die finale kaufmaennische Zielstruktur ist im Repo bereits vorhanden:

- `quotes`
- `quote_items`
- manueller Quote-Editor in `client/lib/pages/quotes_page.dart`

Wichtige Erkenntnis:

- geparste GAEB-Positionen duerfen nicht direkt `quote_items` werden
- vorher braucht es eine Roh- und Review-Stufe

### 3.2 Importlaufmuster aus LogiKal bleibt nuetzlich

Mit `project_imports` und `project_import_changes` existiert bereits ein
Import-Run-Muster mit Aenderungsprotokoll.

Dieses Muster zeigt:

- Importe sollten als eigenstaendige Laeufe modelliert bleiben
- Aenderungen sollen nachvollziehbar und nicht still sein
- spaetere Review- oder Apply-Schritte sollten explizit bleiben

Fuer GAEB ist das fachlich anders, aber der Grundgedanke bleibt passend:

- Rohdaten getrennt halten
- Review explizit machen
- Uebernahme bewusst ausloesen

## 4. Kleinster sinnvolle Einstiegspunkt fuer Parser + Review

Der kleinste sinnvolle naechste Block ist **nicht**

- `GAEB parsen -> sofort Quote erzeugen`

sondern

- `GAEB parsen -> Importpositionen speichern -> Review-Anker schaffen`

### 4.1 Empfohlenes neues Aggregat

Als naechste technische Einheit sollte ein neues Objekt eingefuehrt werden:

- `quote_import_items`

Dieses Objekt repraesentiert **geparste Rohpositionen**, noch keine finalen
Angebotspositionen.

## 5. Empfohlenes Minimalzielbild fuer `quote_import_items`

Die erste Review-Stufe sollte bewusst flach bleiben und keine komplette
GAEB-Hierarchie modellieren.

Empfohlenes Minimalfeldset:

- `id UUID PRIMARY KEY`
- `import_id UUID NOT NULL REFERENCES quote_imports(id) ON DELETE CASCADE`
- `position_no TEXT NOT NULL`
- `outline_no TEXT NOT NULL DEFAULT ''`
- `description TEXT NOT NULL`
- `qty NUMERIC(15,3) NOT NULL DEFAULT 0`
- `unit TEXT NOT NULL DEFAULT ''`
- `is_optional BOOLEAN NOT NULL DEFAULT false`
- `parser_hint TEXT NOT NULL DEFAULT ''`
- `review_status TEXT NOT NULL DEFAULT 'pending'`
- `review_note TEXT NOT NULL DEFAULT ''`
- `sort_order INT NOT NULL DEFAULT 0`

### 5.1 Warum dieses Modell klein genug ist

Es verzichtet bewusst auf:

- tiefe LV-Baumstruktur
- Nachunternehmer-/Preisfelder
- automatische Material- oder Artikelzuordnung
- Diff- oder Versionierungslogik

Damit bleibt es ein **Review-Container fuer importierte Positionen** und nicht
schon eine komplette GAEB-Engine.

## 6. Empfohlene minimale Status- und Review-Idee

### 6.1 Importlauf-Status

Der bereits vorbereitete Statusraum kann jetzt sinnvoll aktiviert werden:

- `uploaded`
- `parsed`
- `reviewed`
- `applied`
- `failed`

### 6.2 Item-Review-Status

Fuer Positionen reicht zunaechst ein kleiner Review-Raum:

- `pending`
- `accepted`
- `rejected`

Optional spaeter:

- `edited`

Fuer den kleinsten Einstieg ist das aber noch nicht noetig.

## 7. Kleinste sinnvolle Nutzerreise fuer die naechste Stufe

Der naechste fachliche Flow sollte klein bleiben:

1. bestehender Importlauf in `uploaded`
2. Parser liest Datei und erzeugt `quote_import_items`
3. Importlauf wechselt auf `parsed` oder `failed`
4. Benutzer sieht nur eine einfache Rohpositionsliste
5. Benutzer markiert Positionen als uebernehmbar oder nicht
6. Quote-Erzeugung kommt erst in einem spaeteren Block

Wichtig:

- `parsed` ist noch nicht `reviewed`
- `reviewed` ist noch nicht `applied`

## 8. Was bewusst noch nicht Teil dieses Blocks sein sollte

Fuer den naechsten Block bewusst **nicht** vorziehen:

- automatische Quote-Erzeugung aus akzeptierten Positionen
- Preisvorschlaege
- Zuordnung zu Materialstammdaten
- KI-basierte Beschreibungskorrekturen
- Vollabdeckung aller GAEB-Unterformate
- komplexe LV-Baum- oder Losstruktur im UI

## 9. Entscheidung

Der naechste sinnvolle Epic-3-Schritt nach dem parserfreien Importlauf ist:

- **nicht** direkt Parser + Quote-Erzeugung
- **sondern** die fachliche Vorbereitung eines kleinen Parser- und
  Review-Zielbilds rund um `quote_import_items`

Der kleinste saubere Einstiegspunkt ist damit:

- `quote_imports` behalten
- `quote_import_items` als Rohpositions-Container einfuehren
- Importlauf auf `parsed`/`failed` bringen
- Review erst als einfache Rohpositionssicht vorbereiten
