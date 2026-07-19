# GAEB-Freigabeentscheidungen: Nacharbeits-Folgepfad nach Prozesssperre-Inventur

## Ziel dieser Inventur

Nach den serverseitigen Prozesssperren ist offene Nacharbeit nicht mehr nur ein
Hinweis, sondern blockiert kommerzielle Weiterverarbeitung.

Diese Inventur klaert den naechsten sinnvollen Folgepfad:

- Was sieht der Nutzer bereits?
- Wo entsteht nach einer Blockade Reibung?
- Welche Folgeausbauten sind moeglich?
- Welcher kleinste Ausbau hat jetzt den hoechsten Nutzen?

In diesem Leaf wird kein Runtime-Code geaendert.

## 1. Ausgangslage

Bereits vorhanden:

- Positionsbadge fuer letzte Freigabeentscheidung
- positionsnahe Markierung `Nacharbeit erforderlich`
- quote-weite Warnung `Nacharbeit offen: n Positionen`
- serverseitige Sperren fuer Versand, Annahme, Rechnung und Auftrag
- Freigabehistorie pro Position
- erneute Freigabeanforderung nach fachlicher Korrektur

Die fachliche Definition ist konsistent:

```text
offene Nacharbeit = letzte terminale Positionsfreigabe ist rejected
```

Eine spaetere `approved`-Entscheidung hebt die Sperre auf.

## 2. Aktuelle Nutzerstrecke

Wenn Nacharbeit offen ist, sieht der Nutzer im Angebotskopf:

```text
Nacharbeit offen: 1 Position
```

oder:

```text
Nacharbeit offen: n Positionen
```

In der betroffenen Positionskarte steht:

```text
Nacharbeit erforderlich
Preis, Material oder Zielpreis anpassen und erneut Freigabe anfordern.
```

Wenn der Nutzer trotzdem eine blockierte Aktion ausfuehrt, antwortet das
Backend mit:

```text
Angebot enthaelt abgelehnte Freigabeentscheidungen; Nacharbeit vor Versand, Annahme oder Folgebeleg erforderlich
```

## 3. Reibung nach der Prozesssperre

Die Sperre ist fachlich korrekt, aber der Folgepfad ist noch nicht fuehrend
genug:

- Die Kopf-Warnung nennt nur die Anzahl, nicht die betroffenen Positionen.
- Es gibt keinen Sprung von der Warnung zur ersten betroffenen Position.
- Bei vielen Positionen muss der Nutzer manuell suchen.
- Der Backend-Fehler nennt keine konkrete Position.
- Es gibt keine zentrale Nacharbeitsliste.
- Es gibt keine Aufgaben- oder Verantwortlichkeitslogik.

Bewertung:

Die naechste Verbesserung sollte die bestehende Warnung handlungsnaeher machen,
ohne bereits eine Queue oder neue Persistenz einzufuehren.

## 4. Kandidaten fuer den Folgeausbau

### 4.1 Positionsnummern in der Kopf-Warnung

Moeglicher Ausbau:

```text
Nacharbeit offen: 2 Positionen (Pos. 3, 7)
```

Nutzen:

- Nutzer sieht sofort, wo er suchen muss.
- Keine neue Backend-Persistenz.
- Ableitbar aus bereits geladener Quote-Detailantwort.

Risiken:

- Lange Positionslisten brauchen Kappung, z.B. erste 3 plus `+ n weitere`.
- Voraussetzung: Position muss im Client fuer jede Quote-Position stabil
  verfuegbar sein.

### 4.2 Sprung zur ersten betroffenen Position

Moeglicher Ausbau:

```text
Button: Zur ersten Position
```

Nutzen:

- Direkter Remediation-Pfad.
- Besonders hilfreich bei langen Angeboten.

Risiken:

- Die aktuelle Positionsliste muss technisch scroll- oder fokussierbar sein.
- Stabiler Key pro Positionskarte waere noetig.
- UI-Aenderung ist etwas groesser als reine Textanreicherung.

### 4.3 Backend-Fehler mit Positionsliste

Moeglicher Ausbau:

```json
{
  "error": "...",
  "affected_positions": [3, 7]
}
```

Nutzen:

- API-Consumer bekommen konkrete Daten.
- Client koennte Fehlerdialoge fuehrender machen.

Risiken:

- Neuer API-Response-Vertrag.
- Fehlerstruktur im bestehenden Domainfehlerpfad muesste erweitert werden.
- Fuer den naechsten kleinen Schritt zu breit.

### 4.4 Zentrale Nacharbeitsqueue

Moeglicher Ausbau:

- eigene Ansicht fuer offene Nacharbeit
- Filter nach Kunde, Projekt, Bearbeiter, Alter
- Verantwortliche und Faelligkeiten

Nutzen:

- Sinnvoll fuer operative Steuerung.

Risiken:

- Neues Arbeitslistenmodell.
- Vermutlich neue Endpoints und eventuell Persistenz.
- Fuer den direkten Folgepunkt nach Prozesssperre zu gross.

### 4.5 Remediation-Status nach Korrektur

Moeglicher Ausbau:

- sichtbarer Status, ob nach Ablehnung schon Preis/Material/Zielpreis geaendert
  wurde
- Hinweis, ob erneute Freigabe noch fehlt

Nutzen:

- Klare Zwischenstufe zwischen `rejected` und spaeter `approved`.

Risiken:

- Braucht eine belastbare Aenderungserkennung gegen Snapshots.
- Koennte schnell in ein eigenes Nacharbeits-Statusmodell wachsen.

## 5. Bewertung

Der kleinste sinnvolle Folgeausbau ist:

```text
Quote-weite Nacharbeitswarnung um betroffene Positionsnummern erweitern
```

Begruendung:

- knuepft direkt an die bestehende Kopf-Warnung an
- braucht voraussichtlich keine Backend-Aenderung
- verbessert den Weg zur Nacharbeit sofort
- bleibt kleiner als Sprunganker, Queue oder strukturierte API-Fehler
- ist eine gute Vorstufe fuer spaetere Sprungziele

Nicht als naechstes:

- Queue
- Aufgabenmodell
- neuer Backend-Fehlervertrag
- automatische Remediation-Erkennung

## 6. Offene technische Pruefpunkte fuer den Folge-Leaf

Vor Implementierung ist eng zu pruefen:

- Ist `position` in der Quote-Detailantwort fuer jede Position vorhanden?
- Wird `position` im Client-Draft bereits stabil gelesen?
- Soll die Warnung maximal 3 Positionen anzeigen und danach kuerzen?
- Wie lautet das Wording fuer eine Position, mehrere Positionen und gekappte
  Listen?
- Bleibt die Anzeige kompatibel, wenn `position` fehlt?

## 7. Entscheidung

Subtask 3.1.42.1 ist abgeschlossen.

Naechster kleinster sinnvoller Schritt:

```text
Subtask 3.1.42.2: Positionsnummern fuer quote-weite Nacharbeitswarnung technisch zuschneiden
```

Dieser naechste Leaf soll noch nicht automatisch implementieren, sondern zuerst
den vorhandenen Quote-Detailvertrag und die Client-Datenstruktur gegen den
kleinen Positionsnummern-Ausbau pruefen.
