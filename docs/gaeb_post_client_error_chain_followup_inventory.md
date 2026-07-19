# GAEB: Folgeinventar nach abgeschlossener Client-Fehlerkettenabsicherung

## Ziel

Subtask 3.1.68.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
der Absicherung von Positionsreview, Importfreigabe, Apply und Navigation in
Erfolgs- und Fehlerfaellen. Dieser Leaf aendert keine Runtime- oder Testlogik.

## Abgedeckter Prozessstand

Der vorhandene Client-Harness belegt inzwischen:

- Vorschau und Detailansicht geladener GAEB-Importe
- read-only Positionsdetail und Permission-Grenze fuer `Review setzen`
- erfolgreichen und abgewiesenen Einzelpositionsreview
- erfolgreiche und abgewiesene Importlauf-Freigabe
- erfolgreiche und abgewiesene Erzeugung einer Draft-Quote
- erfolgreiche und fehlgeschlagene Navigation zur erzeugten Quote

Die Backend-Integration deckt zusaetzlich Upload, Parsing, Reviewzustand,
Transformation und Verknuepfung der akzeptierten Positionen ab. Ein weiterer
Test innerhalb dieser bereits geschlossenen Kette waere weitgehend redundant.

## Noch ungetesteter vorgelagerter Einstieg

Vor der abgesicherten Kette liegt der UI-Einstieg `GAEB-Import`. Die Runtime
erzwingt dort bereits eine fachlich wichtige Projektbindung:

```text
GAEB-Import anklicken
  -> Projekt-ID aus dem Filter lesen und trimmen
  -> bei leerer Projekt-ID Hinweis anzeigen
  -> vor Dateiauswahl und Upload abbrechen
```

Diese Schutzgrenze ist im Widget-Harness noch nicht belegt. Sie verhindert,
dass ein Leistungsverzeichnis ohne Projektkontext in den Importprozess gelangt.

## Folgeoptionen im Vergleich

### Option A: Projektpflicht am GAEB-Importeinstieg widgettesten

Mit `quotes.write`, aber ohne Projektfilter, wird `GAEB-Import` angeklickt. Der
Test erwartet den vorhandenen Hinweis und keinen Fortschrittsdialog. Da die
Validierung vor `browser.pickFile(...)` liegt, sind weder Browser-Fake noch
Upload-API-Erweiterung erforderlich.

Bewertung: kleinster isolierter Folgeausbau mit direktem fachlichem
Zuordnungsschutz.

### Option B: erfolgreichen Upload widgettesten

Dateiauswahl, Dateiname, Bytes, Content-Type, Projekt-ID, Fortschrittsdialog,
Upload-Aufruf, Import-Reload und Erfolgssnackbar koennten gemeinsam getestet
werden.

Bewertung: fachlich wertvoll, aber groesser. Der direkte Browserdateipicker ist
aktuell nicht injizierbar und benoetigt zuerst eine Testnaht oder einen
abstrahierten Pickervertrag.

### Option C: Uploadfehler widgettesten

Ein strukturierter API-Fehler koennte nach ausgewaehlter Datei auf
Dialogabschluss und Fehlermeldung geprueft werden.

Bewertung: setzt dieselbe noch fehlende Picker-Testnaht wie Option B voraus und
ist kein kleiner Anschlussleaf.

### Option D: weitere Transformations- und Mappingregeln ausbauen

Materialzuordnung, Preisquellen, Steuer- oder Kalkulationsregeln koennten
vertieft werden.

Bewertung: hoher fachlicher Wert, aber ein eigener Backend- und
Domänenmodellblock. Der deterministische Basistransfer ist bereits belegt.

### Option E: KI-gestuetzte Angebotsanreicherung beginnen

GAEB-Positionen koennten Text-, Material- oder Preisvorschlaege erhalten.

Bewertung: strategisches Kernziel, aber kein kleiner Restpunkt. Modellvertrag,
Quellenbelege, Konfidenz, Auditierbarkeit und menschliche Freigabe muessen als
eigener Block entworfen werden.

## Entscheidung

Der kleinste fachlich wertvolle Folgeausbau ist ein isolierter Widgettest fuer
die Projektpflicht am bestehenden `GAEB-Import`-Einstieg.

Der Zielumfang bleibt eng:

- `QuotesPage` mit `quotes.read` und `quotes.write`
- kein initialer Projektfilter beziehungsweise leere Projekt-ID
- sichtbare Aktion `GAEB-Import`
- Klick auf die Aktion
- sichtbarer Hinweis
  `Für den GAEB-Import bitte zuerst eine Projekt-ID im Filter setzen.`
- kein Fortschrittsdialog `GAEB-Import wird hochgeladen...`
- keine Picker-, Upload-, Backend- oder Runtime-Erweiterung

Der Test soll nicht versuchen, einen Browserdateidialog zu beobachten. Der
fruehe Return vor `browser.pickFile(...)` ist die stabile fachliche Grenze.

## Technische Minimalgrenze

Der vorhandene `_FakeApiClient` benoetigt fuer diesen Test keine neue Methode:

- `hasPermission(...)` kann `quotes.write` bereits freigeben
- eine leere `quoteImportList` reicht fuer den Seitenaufbau
- ohne Projekt-ID endet `_importGAEB()` vor jedem API- oder Browserzugriff

Der Test kann direkt neben den bestehenden GAEB-Vorschautests angelegt werden.
Stabile Finder sind der `FilledButton` mit `GAEB-Import`, der vollstaendige
Hinweistext und das Ausbleiben des Fortschrittstextes.

## Nicht-Ziele

- noch keine Implementierung
- keine Picker-Abstraktion
- kein erfolgreicher oder fehlgeschlagener Upload
- keine Dateityp- oder Groessenvalidierung
- keine Runtime-, API-, Backend-, DB- oder Permission-Aenderung
- keine Transformations-, Mapping-, Kalkulations- oder KI-Logik

## Naechster Leaf

Subtask 3.1.68.2 definiert die Minimalstrategie fuer genau diesen Widgettest:

- kleinster Seitenaufbau ohne Projektfilter
- erforderliche Berechtigungen
- stabile Aktion- und Hinweisfinder
- Negativassertion fuer den Fortschrittsdialog
- gezielter Testname und Verifikationskommandos

## Ergebnis

3.1.68.1 ist abgeschlossen. Der kleinste fachlich wertvolle Folgeausbau sichert
die bereits vorhandene Projektpflicht ab, bevor ein GAEB-Dateipicker oder
Upload gestartet wird.
