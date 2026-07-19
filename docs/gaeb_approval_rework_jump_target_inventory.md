# GAEB-Freigabeentscheidungen: Sprungziel zur ersten Nacharbeitsposition-Inventur

## Ziel dieser Inventur

Nach der Positionsnummern-Anzeige in der quote-weiten Nacharbeitswarnung ist der
naechste moegliche UX-Schritt Navigation:

```text
Von der Warnung direkt zur ersten betroffenen Nacharbeitsposition springen
```

Diese Inventur klaert, ob dieser Ausbau fachlich sinnvoll ist und welcher
kleinste naechste Schritt technisch vertretbar waere.

In diesem Leaf wird kein Runtime-Code geaendert.

## 1. Ausgangslage

Bereits vorhanden:

- Kopf-Warnung bei offener Nacharbeit
- Anzeige betroffener Positionsnummern in der Kopf-Warnung
- Positionsnahe Markierung `Nacharbeit erforderlich`
- serverseitige Sperren fuer Versand, Annahme, Rechnung und Auftrag

Die Warnung ist jetzt informativer, aber noch nicht navigierend.

## 2. Aktuelle UI-Struktur

Die Quote-Detailansicht enthaelt:

- einen scrollbaren Detailbereich
- die Kopf-Warnung oberhalb der Positionsliste
- eine Positionsliste aus `selected['items']`
- Positionsreihenfolge aus der geladenen Detailantwort

Technischer Befund:

- Der Detailbereich nutzt `SingleChildScrollView`.
- Die Positionsliste ist ein eingebettetes `ListView.separated`.
- Die Positionsliste hat `NeverScrollableScrollPhysics`.
- Es gibt aktuell keine `GlobalKey`-Zuordnung pro Positionskarte.
- Es gibt aktuell keinen Scroll-Controller fuer einen gezielten Sprung.

Bewertung:

Ein Sprungziel ist moeglich, braucht aber eine kleine UI-Mechanik. Reines
Wording reicht nicht mehr.

## 3. Fachlicher Nutzen

Ein Sprungziel ist sinnvoll, weil:

- die Warnung bereits konkrete Positionen nennt
- bei langen Angeboten manuelles Suchen weiterhin Reibung erzeugt
- blockierte Prozessaktionen schnelle Remediation verlangen
- der Nutzer direkt zur ersten betroffenen Position gefuehrt werden kann

Der erste sinnvolle Navigationsschritt ist:

```text
Zur ersten betroffenen Position
```

Nicht direkt noetig:

- Sprung zu jeder einzelnen betroffenen Position
- Sidebar oder Minimap
- zentrale Nacharbeitsliste
- Persistenz von Aufgaben

## 4. UX-Zielbild

Wenn offene Nacharbeit existiert, zeigt die Kopf-Warnung zusaetzlich eine kleine
Aktion:

```text
Zur ersten Position
```

Verhalten:

- Klick scrollt zur ersten Position mit `latest_approval_decision.status ==
  rejected`.
- Die Position bleibt dieselbe, die bereits als niedrigste Positionsnummer in
  der Warnung angezeigt wird.
- Wenn keine Position gefunden wird, wird keine Aktion angezeigt.

Optional fuer spaeter:

- kurzzeitige Hervorhebung der Zielposition
- Sprung zu naechster betroffener Position
- Ruecksprung zur Warnung

## 5. Technische Kandidaten

### 5.1 GlobalKey pro betroffener Position

Moeglicher Ansatz:

- fuer die Detail-Positionsliste pro Index einen `GlobalKey` erzeugen
- Zielposition per `Scrollable.ensureVisible(...)` sichtbar machen

Vorteile:

- passt zur vorhandenen `SingleChildScrollView`
- kein neuer Backend-Vertrag
- kein neues Datenmodell

Risiken:

- Keys muessen pro Build stabil genug verwaltet werden
- Detailansicht darf nicht unnoetig komplex werden

### 5.2 ScrollController mit geschaetztem Offset

Moeglicher Ansatz:

- ScrollController am Detailbereich
- Offset anhand Position/Zeilenhoehe schaetzen

Bewertung:

Nicht empfehlenswert. Die ListTiles koennen in der Hoehe variieren; geschaetzte
Offsets waeren fragil.

### 5.3 Positionsliste in eigenen Widget-Block auslagern

Moeglicher Ansatz:

- eigener Widget fuer Warnung und Positionsliste
- dort Keys, Scroll und Hervorhebung kapseln

Bewertung:

Sauberer, aber groesser als der naechste kleinste Schritt. Eher sinnvoll, wenn
spaeter auch Hervorhebung oder mehrere Sprungziele umgesetzt werden.

## 6. Scope-Grenzen fuer den naechsten Schritt

Nicht Teil des ersten Sprungziel-Implementierungsblocks:

- Backend-Feld `position`
- API-Fehlerdetails
- Nacharbeitsqueue
- Aufgabenmodell
- dauerhafte Hervorhebung
- Sprung zu allen betroffenen Positionen
- Umbau der gesamten Quote-Detailansicht

## 7. Entscheidung

Subtask 3.1.43.1 ist abgeschlossen.

Der naechste kleinste Folgepunkt ist:

```text
Subtask 3.1.43.2: Sprungziel zur ersten Nacharbeitsposition technisch zuschneiden
```

Dieser Folge-Leaf soll pruefen und festlegen:

- wo der `GlobalKey`-Map-Zustand in der Detailansicht leben soll
- wie die erste betroffene Position aus der bestehenden Positionsliste
  bestimmt wird
- wo die Aktion `Zur ersten Position` in der Warnung sitzt
- ob zunaechst nur `Scrollable.ensureVisible(...)` reicht
- welche Verifikation fuer diesen client-only Schritt noetig ist
