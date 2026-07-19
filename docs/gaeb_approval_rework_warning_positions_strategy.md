# GAEB-Freigabeentscheidungen: Positionsnummern in Nacharbeitswarnung-Strategie

## Ziel dieses Zuschnitts

Dieses Dokument schneidet den naechsten kleinen UX-Schritt technisch zu:

```text
Quote-weite Nacharbeitswarnung zeigt betroffene Positionsnummern
```

Der Zuschnitt prueft bewusst, ob der Schritt ohne Backend-Aenderung moeglich
ist.

## 1. Befund Backend

Die Quote-Detailantwort liefert aktuell pro Position:

- `id`
- `description`
- `qty`
- `unit`
- `unit_price`
- `tax_code`
- Mapping- und Freigabefelder
- `latest_approval_decision`

Nicht explizit ausgeliefert:

- `position`

Das Backend liest die Positionen aber stabil sortiert:

```text
ORDER BY qi.position
```

Bewertung:

- Die Positionsreihenfolge der Detailantwort entspricht der Angebotsposition.
- Ein neues JSON-Feld `position` waere fachlich sauber, ist fuer diesen kleinen
  Warnungs-Ausbau aber nicht zwingend.
- Eine Backend-Aenderung wuerde zusaetzliche DTO-, Scan- und Testanpassung
  bedeuten und ist fuer die naechste kleinste UI-Verbesserung nicht noetig.

## 2. Befund Client

Die quote-weite Warnung zaehlt aktuell:

```text
selected['items'][*]['latest_approval_decision']['status'] == rejected
```

Der Editor zeigt Positionen bereits ueber den Listenindex:

```text
Position ${index + 1}
```

Die Detailansicht zeigt die Positionsliste in derselben Reihenfolge aus
`selected['items']`.

Bewertung:

- Der Client kann betroffene Positionsnummern aus dem vorhandenen Array-Index
  ableiten.
- Die Nummern sind kompatibel mit der sichtbaren Reihenfolge.
- Fehlende oder kuenftig explizite `position`-Felder koennen spaeter
  beruecksichtigt werden, ohne diesen Schritt zu blockieren.

## 3. Zielmodell fuer den kleinen Schritt

Neue clientseitige Ableitung:

```text
_quoteRejectedApprovalDecisionPositions(...)
```

Rueckgabe:

- Liste positiver Positionsnummern
- Nummer = `index + 1`
- nur Items mit `latest_approval_decision.status == rejected`

Die bestehende Zaehlfunktion kann danach entweder ersetzt oder intern aus der
Positionsliste abgeleitet werden.

## 4. Wording

Singular:

```text
Nacharbeit offen: 1 Position
Betroffene Position: Pos. 3
Eine Position wurde zuletzt abgelehnt. Position pruefen, Preis/Material/Zielpreis nacharbeiten und erneut Freigabe anfordern.
```

Plural bis drei Positionen:

```text
Nacharbeit offen: 2 Positionen
Betroffene Positionen: Pos. 3, 7
Mehrere Positionen wurden zuletzt abgelehnt. Positionen pruefen, Preis/Material/Zielpreis nacharbeiten und erneut Freigabe anfordern.
```

Plural mit Kappung:

```text
Betroffene Positionen: Pos. 3, 7, 9 + 2 weitere
```

Kappungsregel:

- maximal drei Positionsnummern direkt anzeigen
- Rest als `+ n weitere`

## 5. Scope-Grenzen

Nicht Teil des naechsten Implementierungs-Leaves:

- Backend-Feld `position`
- API-Fehler mit Positionsliste
- Scroll-/Sprunganker
- Fokus oder Hervorhebung der Positionskarte
- Nacharbeitsqueue
- Aufgabenmodell
- Remediation-Status nach Korrektur

## 6. Kompatibilitaet

Die Ableitung bleibt stabil, wenn:

- `items` fehlt oder leer ist
- einzelne Eintraege keine Map sind
- `latest_approval_decision` fehlt
- `status` leer oder unbekannt ist

In diesen Faellen wird die Position nicht als offene Nacharbeit gezaehlt.

## 7. Verifikation fuer Implementierung

Nach Umsetzung reicht fuer diesen engen Client-Schritt:

```text
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Backend-Tests sind nicht erforderlich, solange kein Backend-DTO und kein
API-Vertrag geaendert wird.

## 8. Entscheidung

Subtask 3.1.42.2 ist abgeschlossen.

Der naechste kleinste Implementierungsschritt ist:

```text
Subtask 3.1.42.3: Positionsnummern in quote-weiter Nacharbeitswarnung clientseitig anzeigen
```

Der Schritt soll nur `client/lib/pages/quotes_page.dart` betreffen und die
Positionsnummern aus der vorhandenen `selected['items']`-Reihenfolge ableiten.
