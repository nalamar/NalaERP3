# GAEB-Freigabeentscheidungen: Folgeaktionen nach terminaler Entscheidung

## Ziel dieses Dokuments

Dieses Dokument schneidet den naechsten fachlichen Folgeblock nach dem
abgeschlossenen positionsnahen Entscheidungsbadge zu.

Ausgangspunkt:

- Freigabeanforderungen koennen positionsbezogen angefordert, storniert,
  genehmigt und abgelehnt werden.
- Entscheidungen enthalten Kommentar, Entscheider und genehmigte Snapshots.
- Die Freigabehistorie ist abrufbar.
- Die letzte terminale Entscheidung ist im Quote-Editor als Badge sichtbar.
- Es gibt noch keine operative Folgeaktion nach `approved` oder `rejected`.

Der naechste Schritt soll zuerst klären, welche Folgeaktionen fachlich klein,
wertvoll und technisch anschlussfaehig sind.

## 1. Aktueller Stand

Der bestehende Freigabestrang beantwortet inzwischen:

- Warum wurde eine Freigabe angefordert?
- Wer hat entschieden?
- Wann wurde entschieden?
- Was wurde kommentiert?
- Wurde genehmigt oder abgelehnt?
- Welche genehmigten Snapshots gelten im Genehmigungsfall?

Was noch nicht beantwortet wird:

- Was soll nach einer Ablehnung operativ passieren?
- Was soll nach einer Genehmigung freigegeben oder entsperrt werden?
- Wann ist eine Position fuer Angebotsabgabe ausreichend bearbeitet?
- Muss ein Angebot mit abgelehnten Positionen blockiert werden?
- Muss eine genehmigte Position automatisch als erledigt markiert werden?
- Braucht Vertrieb eine Wiedervorlage oder Aufgabenliste?

## 2. Fachliche Leitplanken

### 2.1 Genehmigung

`approved` bedeutet im aktuellen Scope:

- Die konkrete Freigabeanforderung wurde fachlich akzeptiert.
- Die im Request gespeicherten Preis- und Zielmargensnapshots wurden
  genehmigt.
- Die Quote-Position darf im Kontext dieser Entscheidung weiterbearbeitet
  werden.

`approved` bedeutet noch nicht:

- Das gesamte Angebot ist freigegeben.
- Der Quote-Status darf automatisch wechseln.
- Ein Auftrag oder Folgebeleg darf automatisch entstehen.
- Die Position ist gegen spaetere Preisveraenderungen gesperrt.
- Alle kaufmaennischen Risiken des Angebots sind geprueft.

### 2.2 Ablehnung

`rejected` bedeutet im aktuellen Scope:

- Die konkrete Freigabeanforderung wurde nicht akzeptiert.
- Der Kommentar ist die fachliche Rueckmeldung fuer Nacharbeit.
- Die Position braucht eine operative Reaktion, bevor sie wieder belastbar ist.

`rejected` bedeutet noch nicht:

- Die Position wird automatisch geloescht.
- Der Preis wird automatisch zurueckgesetzt.
- Das gesamte Angebot ist abgelehnt.
- Eine neue Freigabeanforderung ist verboten.

## 3. Naheliegende Folgeoptionen

### Option A: Wiedervorlage fuer abgelehnte Positionen

Inhalt:

- Abgelehnte Positionen werden als nacharbeitsbeduerftig sichtbar.
- Der Kommentar der Ablehnung dient als Rueckmeldung.
- Vertrieb oder Kalkulation kann Preis, Material oder Zielpreis anpassen.
- Danach kann erneut eine Freigabe angefordert werden.

Vorteile:

- kleinster fachlicher Nutzen nach `rejected`
- passt direkt zum vorhandenen Badge und Kommentar
- keine neue Queue zwingend noetig
- kann zunaechst read-only als Positionsstatus im Editor starten

Nachteile:

- erzeugt noch keine echte Aufgabe mit Faelligkeit oder Verantwortlichem
- kann in groesseren Angeboten ohne Sammelsicht uebersehen werden

Bewertung:

Sehr guter erster Folgepunkt, weil er die abgelehnte Entscheidung in eine klare
Nacharbeitslogik uebersetzt.

### Option B: Prozesssperre bei abgelehnter Position

Inhalt:

- Angebote mit mindestens einer zuletzt abgelehnten Position werden beim
  Statuswechsel oder bei Folgebelegaktionen blockiert.
- Die Sperre endet, wenn eine neue Freigabe angefordert und genehmigt wurde
  oder die Position fachlich korrigiert wird.

Vorteile:

- verhindert versehentliche Angebotsabgabe trotz Ablehnung
- hoher kaufmaennischer Schutz
- passt zu ERP-Prozesssicherheit

Nachteile:

- braucht genaue Definition, welche Aktionen gesperrt werden
- kann Nutzer blockieren, wenn Korrektur- und Re-Request-Logik noch nicht
  sauber genug ist
- erfordert Backend-Regeln auf Quote-Ebene

Bewertung:

Wichtig, aber als erster Implementierungsleaf zu breit, solange die
Nacharbeitslogik noch nicht modelliert ist.

### Option C: Genehmigte Position als erledigt markieren

Inhalt:

- Eine genehmigte terminale Entscheidung wird als erledigter Freigabepunkt
  betrachtet.
- Das Badge zeigt den Abschluss.
- Keine weitere Aktion entsteht, solange keine neue risikorelevante
  Preisveraenderung erfolgt.

Vorteile:

- entspricht dem aktuellen Badge-Verhalten
- keine neue Persistenz noetig
- vermeidet kuenstliche Arbeitsschritte nach Genehmigung

Nachteile:

- erkennt noch nicht, ob spaetere Preis- oder Materialaenderungen die
  Genehmigung fachlich entwerten
- kein angebotsweiter Freigabestatus

Bewertung:

Fuer den MVP korrekt: `approved` ist ein Abschluss der konkreten Anforderung,
nicht der Start eines neuen Prozesses.

### Option D: Zentrale Freigabe-Queue

Inhalt:

- Offene Freigabeanforderungen und abgelehnte Nacharbeit werden zentral
  gelistet.
- Filter nach Verantwortlichkeit, Status, Kunde, Angebot und Datum.

Vorteile:

- bessere operative Steuerung
- sinnvoll fuer mehrere Freigebende
- Grundlage fuer Eskalation und SLA

Nachteile:

- braucht Listenendpoint und neues UI
- braucht Verantwortlichkeitsmodell
- groesser als ein einzelner positionsnaher Folgeleaf

Bewertung:

Langfristig sinnvoll, aber nicht der kleinste naechste Schritt.

### Option E: Automatische Angebotsfreigabe nach Positionsgenehmigung

Inhalt:

- Wenn alle kritischen Positionen genehmigt sind, kann das Angebot automatisch
  oder halbautomatisch in einen freigegebenen Status wechseln.

Vorteile:

- verbindet Positionsfreigabe mit Angebotsprozess
- hoher Prozessnutzen in spaeterem Vertriebsausbau

Nachteile:

- braucht angebotsweite Kritikalitaetsregeln
- braucht Statusmodell fuer Angebotsfreigabe
- kann nicht nur aus einer einzelnen Positionsentscheidung abgeleitet werden

Bewertung:

Zu gross fuer den naechsten Schritt.

## 4. Entscheidung fuer den naechsten kleinen Folgepfad

Der kleinste sinnvolle naechste Folgepfad ist:

```text
Positionsnahe Nacharbeitsmarkierung fuer die letzte abgelehnte Entscheidung
```

Kernidee:

- Wenn `latest_approval_decision.status == rejected`, gilt die Position als
  nacharbeitsbeduerftig.
- Die bestehende Badge-Anzeige liefert bereits Kommentar und Entscheider.
- Der naechste Implementierungsblock soll daraus zunaechst eine klarere
  positionsnahe UI-Aussage machen.
- Noch keine globale Queue.
- Noch keine Quote-weite Prozesssperre.
- Noch keine automatische Preisveraenderung.

Warum Ablehnung zuerst:

- Eine Genehmigung ist im aktuellen Scope bereits ein Abschluss.
- Eine Ablehnung erzeugt echten operativen Bedarf.
- Der kleinste Nutzen entsteht, wenn Nutzer sofort sehen: diese Position muss
  nachbearbeitet werden.

## 5. Minimalzielbild fuer den naechsten Implementierungsblock

### 5.1 Client

Bei `latestApprovalDecision.status == rejected` soll die Positionskarte
deutlicher anzeigen:

- `Nacharbeit erforderlich`
- Ablehnungskommentar, falls vorhanden
- optionaler Hinweis: `Preis, Material oder Zielpreis anpassen und erneut
  Freigabe anfordern`

Dabei bleibt:

- das bestehende Entscheidungsbadge erhalten
- die Historie manuell ladbar
- eine neue Freigabeanforderung ueber bestehende Zielmargen-/Approval-Aktion
  moeglich, sobald die fachlichen Voraussetzungen erfuellt sind

### 5.2 Backend

Fuer den ersten Implementierungsblock ist kein neues Backend-Feld zwingend
notwendig.

Begruendung:

- `latest_approval_decision.status` reicht fuer die erste UI-Markierung.
- Kommentar und Grund sind bereits vorhanden.
- Eine echte Prozesssperre oder Queue waere ein eigener Backend-Block.

### 5.3 Tests

Client-Verifikation:

```text
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Backend-Verifikation nur bei Backend-Aenderungen:

```text
go test ./internal/quotes ./internal/http
```

## 6. Bewusst ausgeschlossene Punkte

Nicht Teil des naechsten Implementierungsleafs:

- neue Tabelle fuer Aufgaben oder Wiedervorlagen
- Verantwortliche oder Faelligkeiten
- automatische Sperre beim Quote-Statuswechsel
- automatische Angebotsfreigabe
- zentrale Freigabe-Queue
- neue Permission
- neue Approval-Statuswerte
- automatische Preis- oder Materialkorrektur

## 7. Spaetere Folgepunkte

Nach der positionsnahen Nacharbeitsmarkierung koennen sinnvoll folgen:

1. Quote-weite Warnung, wenn mindestens eine Position zuletzt abgelehnt wurde.
2. Backend-Prozesssperre fuer Statuswechsel oder Folgebelege bei offener
   Nacharbeit.
3. Zentrale Queue fuer offene Freigaben und abgelehnte Nacharbeit.
4. Verantwortliche, Faelligkeiten und Eskalationen.
5. Angebotsweite Freigaberegel auf Basis aller kritischen Positionen.

## 8. Naechster Implementierungs-Leaf

```text
Subtask 3.1.39.2: Positionsnahe Nacharbeitsmarkierung fuer abgelehnte Freigabeentscheidungen im Quote-Editor anzeigen
```

Umfang:

- bestehendes `latestApprovalDecision` im Client auswerten
- bei `rejected` kompakten Nacharbeits-Hinweis in `_QuoteItemRow` anzeigen
- keine Backend-Aenderung
- keine neue Persistenz
- keine neue Queue

Nicht enthalten:

- Quote-weite Sperre
- Aufgabenmodell
- automatische Wiedervorlage
- Angebotsstatus-Automatik
