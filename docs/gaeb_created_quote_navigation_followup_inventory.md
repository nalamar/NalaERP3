# GAEB: Nachfolgeinventar nach getesteter Erzeugte-Quote-Navigation

## Ziel

Subtask 3.1.64.1 bestimmt den kleinsten fachlich wertvollen Folgeausbau nach
dem abgeschlossenen Erfolgs-Widgettest fuer die Navigation vom angewendeten
GAEB-Import zur erzeugten Draft-Quote. Dieser Leaf aendert keine Runtime- oder
Testlogik.

## Ausgangslage

Die bestehende GAEB-Strecke ist fuer den Erfolgsfall geschlossen:

```text
Import pruefen
  -> Importlauf freigeben
  -> Draft-Quote erzeugen
  -> created_quote_id anzeigen
  -> Quote oeffnen
  -> ausgewaehltes Quote-Detail anzeigen
```

Der Widgettest `QuotesPage GAEB created quote navigation opens selected draft`
belegt insbesondere:

- Weitergabe der eindeutigen `created_quote_id`
- Schliessen des Importdialogs
- Laden und Reload der Zielquote
- sichtbaren Status-, Kunden- und Projektkontext
- Erfolgsfeedback per Snackbar

Die Runtime besitzt daneben bereits einen `catch`-Pfad mit der fachlichen
Fallback-Meldung `Erzeugte Quote konnte nicht geöffnet werden`. Dieser Pfad ist
im Flutter-Harness noch nicht gezielt abgesichert.

## Verbleibende Risiken und Ausbauoptionen

### Option A: Fehlerpfad einer nicht ladbaren erzeugten Quote widgettesten

Ein `applied` Import behaelt eine nicht leere `created_quote_id`, waehrend
`getQuote` deterministisch mit einer `ApiException` fehlschlaegt. Der Test
prueft die angefragte ID, den geschlossenen Importdialog, eine unveraenderte
Angebotsauswahl und das sichtbare Fehlerfeedback.

Nutzen:

- sichert einen bereits vorhandenen realistischen Inkonsistenzfall ab
- verhindert einen stillen oder irrefuehrenden Kontextwechsel
- bleibt client- und read-only
- benoetigt keine Runtime-, Backend-, API-, DB- oder Permission-Aenderung
- ist als einzelner Widgettest klar vom Erfolgsfall getrennt

Bewertung: kleinster Folgeausbau mit neuem Robustheitssignal.

### Option B: Fehlerpfad der Draft-Quote-Erzeugung widgettesten

Ein Fehler von `applyQuoteImport` koennte im bestehenden Dialog dargestellt
werden.

Bewertung: ebenfalls sinnvoll, betrifft aber die vorgelagerte Apply-Mutation
und nicht die jetzt abgeschlossene Navigationsgrenze. Er ist deshalb nicht der
unmittelbarste Folgepunkt.

### Option C: erzeugte Quote fachlich gegen Importpositionen vergleichen

Mengen, Beschreibungen, Preise oder Materialanker koennten zwischen
Importpositionen und Quote-Positionen verglichen werden.

Bewertung: hoher fachlicher Wert, aber ein eigener Mapping- und
Transformationsblock mit groesserem Payload- und Regelumfang.

### Option D: KI-gestuetzte Anreicherung starten

Die Draft-Quote koennte automatisch Text-, Material- oder Preisvorschlaege
erhalten.

Bewertung: strategisch wichtig, aber noch kein kleiner Anschluss. Erforderlich
waeren mindestens Quellenbelege, Konfidenz, Auditierbarkeit, Freigabegrenzen
und ein klarer Modellvertrag.

## Entscheidung

Der kleinste fachlich wertvolle Folgeausbau ist ein separater read-only
Widgettest fuer den Fehler beim Oeffnen einer erzeugten Quote.

Der Zielumfang bleibt eng:

- genau ein `applied` Import mit eindeutiger `created_quote_id`
- Berechtigung `quotes.read`
- deterministischer `getQuote`-Fehler nur fuer diesen Test
- Nachweis der angefragten Quote-ID
- Importdialog wird durch die bestehende Aktion geschlossen
- keine falsche Zielquote wird ausgewaehlt
- API-Fehlermeldung beziehungsweise vorhandener Fallback ist sichtbar
- keine Apply-Mutation, Quote-Bearbeitung oder weitere Konvertierung

## Technische Minimalgrenze

Der bestehende `_FakeApiClient` zeichnet bereits `requestedQuoteIds` auf. Fuer
den Folgeleaf reicht voraussichtlich genau eine optionale, standardmaessig
inaktive Fehlerkonfiguration fuer `getQuote`. Bestehende Tests muessen dadurch
unveraendert bleiben.

Der Test gehoert weiterhin nach
`client/test/sales_order_context_pages_test.dart` und nutzt die vorhandene
`QuotesPage`. Ein neuer Backend- oder API-Vertrag ist nicht erforderlich.

## Nicht-Ziele

- noch keine Implementierung
- keine Runtime-Aenderung
- kein Apply-Fehlerfall
- kein Berechtigungsnegativtest
- keine Positions-, Preis-, Material- oder Kalkulationspruefung
- kein Mapping- oder KI-Ausbau
- keine Backend-, API-, Datenbank- oder Permission-Aenderung

## Naechster Leaf

Subtask 3.1.64.2 definiert die Minimalstrategie fuer:

- kleinste optionale Fehlerkonfiguration in `_FakeApiClient.getQuote`
- stabilen Import- und bestehenden Auswahl-Payload
- erwartete Quote-ID-Aufzeichnung
- stabile Finder fuer Dialogschluss, unveraenderte Auswahl und Fehlerfeedback
- klare Abgrenzung gegen Apply-, Berechtigungs- und Mappingtests

## Ergebnis

3.1.64.1 ist abgeschlossen. Der kleinste fachlich wertvolle Folgeausbau ist
ein isolierter Widgettest fuer eine nicht ladbare, ueber `created_quote_id`
referenzierte Draft-Quote. Die Runtime stellt den Fehlerpfad bereits bereit;
3.1.64.2 schneidet ausschliesslich dessen Testvertrag zu.
