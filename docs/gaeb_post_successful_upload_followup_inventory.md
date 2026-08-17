# GAEB: Folgeinventar nach abgesichertem erfolgreichem Upload

## Ziel

Subtask 3.1.70.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
dem belegten projektgebundenen GAEB-Upload. Dieser Leaf aendert keine Runtime-
oder Testlogik.

## Ausgangslage

Der Client-Harness belegt nun:

- Abbruch ohne Projekt-ID vor dem Dateipicker
- lokale und deterministische Dateiauswahl im Test
- Weitergabe von Accept-Filter, Dateiname, Bytes und Content-Type
- getrimmte Projektbindung
- erfolgreichen Upload, Importlisten-Reload und Erfolgshinweis
- die nachgelagerte Review-, Apply- und Navigationskette in ihren zentralen
  Erfolgs- und Fehlerausgaengen

Die Picker-Testnaht und das normalisierte Fake-Uploadpayload koennen fuer den
noch offenen Fehlerausgang wiederverwendet werden.

## Unmittelbare offene Luecke

`QuotesPage._importGAEB()` besitzt bereits einen Catch-Pfad:

```text
Datei ausgewaehlt
  -> Fortschrittsdialog geoeffnet
  -> uploadGAEBQuoteImport wirft ApiException
  -> Fortschrittsdialog geschlossen
  -> strukturierte Fehlermeldung angezeigt
  -> kein Importlisten-Reload und kein Erfolgshinweis
```

Dieser Zustand ist noch nicht widgetgetestet. Ohne Absicherung koennte ein
fehlgeschlagener Upload als Erfolg erscheinen, den Dialog offen lassen oder
eine unnoetige Aktualisierung ausloesen.

## Folgeoptionen im Vergleich

### Option A: strukturierten Uploadfehler widgettesten

Der bestehende Fake erhaelt eine dateinamenspezifische Fehlerkonfiguration.
Nach injizierter X83-Datei wirft der Upload eine 422-`ApiException`. Der Test
belegt Versuchspayload, ausbleibenden Reload, geschlossenen Fortschrittsdialog,
sichtbare API-Meldung und fehlenden Erfolgshinweis.

Bewertung: kleinster eigenstaendiger Folgeausbau mit direktem Schutz vor einem
falschen Importzustand.

### Option B: abgebrochene Dateiauswahl widgettesten

Der Picker koennte `null` zurueckgeben. Erwartet waeren kein Upload, kein
Dialog und kein Feedback.

Bewertung: technisch kleiner, aber fachlich schwaches Signal. Der Pfad mutiert
nichts und ist ein einfacher frueher Return.

### Option C: Fortschrittsdialog waehrend eines angehaltenen Uploads testen

Ein `Completer` koennte den Upload pausieren, um den Zwischenzustand sichtbar
zu pruefen.

Bewertung: asynchron deutlich fragiler. Erfolg und Fehler belegen bereits den
Dialogabschluss; der Zwischenzustand liefert weniger Fachnutzen als der noch
offene Fehlervertrag.

### Option D: Dateiinhalt oder Dateigroesse clientseitig validieren

Vor dem Upload koennten leere oder zu grosse Dateien abgewiesen werden.

Bewertung: benoetigt neue Fachregeln und Abstimmung mit Server- und Proxygrenzen.
Das ist kein kleiner Testrest.

### Option E: Mapping oder KI-Anreicherung ausbauen

Material-, Preis- oder KI-Vorschlaege koennten auf importierten Positionen
vertieft werden.

Bewertung: strategisch wichtig, aber ein eigener groesserer Block. Zuerst wird
der jetzt testbare Uploadfehler ohne neue Produktlogik geschlossen.

## Entscheidung

Der kleinste fachlich wertvolle Folgeausbau ist ein isolierter Widgettest fuer
einen strukturiert abgewiesenen GAEB-Upload.

Der Zielumfang bleibt eng:

- vorhandener lokaler Picker-Callback
- `quotes.read` und `quotes.write`
- getrimmte Projekt-ID und feste X83-Datei
- dateinamenspezifische 422-`ApiException` im Fake
- Uploadversuch wird weiterhin vollstaendig aufgezeichnet
- Importlisten-Zaehler bleibt nach dem initialen Abruf bei eins
- Fortschrittsdialog und `AlertDialog` sind nach dem Fehler geschlossen
- API-Meldung ist sichtbar
- Erfolgshinweis und zweiter Listenabruf bleiben aus

## Zerlegung

Task 3.1.70 wird in drei weitere Leaves zerlegt:

1. **3.1.70.2** – Minimalstrategie fuer Fake-Fehlervertrag und Widgettest
   definieren.
2. **3.1.70.3** – Fake-Fehlerkonfiguration und genau einen Uploadfehler-
   Widgettest implementieren und verifizieren.
3. **3.1.70.4** – Implementierung und Nachweise auditieren.

## Nicht-Ziele

- noch keine Implementierung
- kein Picker-Abbruch-Test
- kein angehaltener Fortschrittsdialog-Test
- keine Runtime-, Backend-, API-, DB- oder Permission-Aenderung
- keine Dateiinhalt- oder Groessenregel
- keine Mapping-, Kalkulations- oder KI-Logik

## Naechster Leaf

Subtask 3.1.70.2 definiert ausschliesslich:

- dateinamenspezifische Fake-Fehlerkonfiguration
- Reihenfolge von Versuchserfassung und Fehlerwurf
- kleinstes Datei-, Projekt- und Fehlerpayload
- Reload-, Dialog-, Fehler- und Erfolg-Negativassertions
- gezielte Verifikationskommandos

## Ergebnis

3.1.70.1 ist abgeschlossen. Der kleinste fachlich wertvolle Folgeausbau ist
der Widgettest eines strukturiert abgewiesenen GAEB-Uploads unter
Wiederverwendung der bestehenden Picker-Testnaht.
