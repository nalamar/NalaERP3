# Epic 3 Commercial Gap Matrix

## Ziel
Diese Matrix priorisiert die verbleibenden Epic-3-Luecken im kommerziellen Workflow auf Basis des bereits vorhandenen Repo-Stands.

## Bewertungslogik
- `Fachwert`: Beitrag zum ERP-Zielbild im Tagesgeschaeft
- `Abhaengigkeit`: Wie stark andere Epic-3-Themen darauf aufbauen
- `Aufwand`: Erwartete technische Groessenordnung
- `Prioritaet`: konkrete Reihenfolge fuer die weitere Arbeit

## Matrix

| Kandidat | Ist-Stand | Luecke | Fachwert | Abhaengigkeit | Aufwand | Prioritaet |
|---|---|---|---|---|---|---|
| Kommerzielle Kontextsicht pro Kontakt/Projekt | Einzelne Belegbeziehungen existieren, aber verteilt ueber Quote-, Auftrag-, Invoice- und Projektseiten | Keine gebuendelte 360-Grad-Sicht auf Angebots-, Auftrags- und Rechnungsbezug je Kontakt/Projekt | hoch | hoch | mittel | 1 |
| Angebotsrevisionen | Angebot kennt Status und Folgebelege, aber kein explizites Revisionsmodell | Keine kontrollierte Mehrfachversion eines Angebots | hoch | hoch | mittel bis hoch | 2 |
| Workflow-Cockpit Quote -> Auftrag -> Rechnung | Teilweise Workflow-Hinweise in Einzelpages | Kein zusammenhaengender uebergreifender Workflow-Status | hoch | mittel | mittel | 3 |
| Kaufmaennische Timeline / History ueber Belege | Activity existiert fuer Kontakte nur fuer Notes/Tasks/Documents | Keine Timeline fuer kommerzielle Folgeereignisse | mittel bis hoch | mittel | mittel | 4 |
| Angebotsvorlagen / strukturierte Textbausteine | PDF-Templates vorhanden | Keine fachliche Angebotsvorstufe fuer Standardtexte/Module | mittel | mittel | mittel | 5 |
| GAEB-Import | Kein belegter Importpfad im Repo | Kein technischer Einstieg fuer Leistungsverzeichnisse | sehr hoch | sehr hoch | hoch | 6 |
| KI-gestuetzte Angebotsableitung aus GAEB | Kein KI-Pfad vorhanden | Kein Zielsystem fuer automatische LV-Interpretation und Preisfindung | sehr hoch | sehr hoch | sehr hoch | 7 |

## Begruendung der Priorisierung

### 1. Kommerzielle Kontextsicht pro Kontakt/Projekt
- Liefert sofort operativen Nutzen fuer Vertrieb und Projektsteuerung.
- Nutzt bereits vorhandene Datenmodelle und Folgebelegbeziehungen.
- Reduziert Such- und Navigationsaufwand ohne zuerst tiefe neue Domänenmodelle einzufuehren.
- Ist gleichzeitig ein guter technischer Vorbereitungsschritt fuer spaetere Revisionen, GAEB-Importe und KI-generierte Angebotsentwuerfe.

### 2. Angebotsrevisionen
- Fachlich wichtig, aber ohne gute Kontextsicht schwer sauber nutzbar.
- Die Revisionen sollten spaeter in derselben kommerziellen Sicht sichtbar werden.

### 3. Workflow-Cockpit
- Ein durchgaengiges Cockpit ist wertvoll, baut aber logisch auf derselben Datenaggregation wie die Kontextsicht auf.

### 4. Timeline / History
- Sehr sinnvoll, aber nicht der erste Hebel.
- Sie profitiert von der gleichen Aggregationslogik wie Kontakt-/Projekt-Kontext.

### 5 bis 7. Vorlagen, GAEB, KI
- Diese Themen sind klar im Zielbild, brauchen aber zuerst stabile kommerzielle Referenzsicht und belastbare Angebotsdomäne.

## Erster echter Epic-3-Umsetzungskandidat

### Empfohlener Kandidat
`Subtask 3.1.1.3: Kommerzielle Kontextsicht pro Kontakt und Projekt aufbauen`

### Zielbild
- Im Kontaktdetail werden zugeordnete Angebote, Auftraege und Rechnungen sichtbar.
- In der Projektsicht werden dieselben kommerziellen Belege konsistent aggregiert.
- Folgebelegbeziehungen bleiben nachvollziehbar:
  - Angebot -> Auftrag
  - Angebot -> Rechnung
  - Auftrag -> Rechnung

### Warum dieser Kandidat zuerst
- Das Repo enthaelt die noetigen IDs und Folgebelegbeziehungen bereits.
- Der Schritt ist fachlich sichtbar, aber technisch noch ueberschaubar.
- Er schafft die geeignete Leseschicht, bevor neue komplexe Schreiblogik wie Angebotsrevisionen kommt.

## Empfohlene Zerlegung fuer den naechsten Schritt

### Micro-Subtask 1
- Ist-Stand fuer Kontakt- und Projektsicht inventarisieren
- Ziel-API fuer kommerzielle Kontextdaten festlegen

### Micro-Subtask 2
- Minimale Backend-Aggregation fuer Kontaktkontext implementieren

### Micro-Subtask 3
- Kontakt-UI um kommerzielle Beleglisten erweitern

### Micro-Subtask 4
- Projektsicht auf dieselbe Kontextlogik anheben

## Ergebnis
Epic 3 sollte mit sichtbarer kommerzieller Kontextsicht beginnen, nicht mit GAEB oder KI direkt. GAEB-/KI-Angebotserzeugung bleibt das strategische Ziel, ist aber erst nach Aufbau einer stabilen kommerziellen Referenz- und Workflow-Sicht sinnvoll priorisierbar.
