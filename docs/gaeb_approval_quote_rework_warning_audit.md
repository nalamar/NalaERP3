# GAEB-Freigabeentscheidungen: Quote-weite Nacharbeitswarnung-Audit

## Ziel dieses Audits

Dieses Dokument prueft den umgesetzten kleinsten Aggregationsschritt nach der
positionsnahen Nacharbeitsmarkierung:

```text
Quote-weite Warnung fuer zuletzt abgelehnte Positionsentscheidungen
```

Geprueft werden:

- Client-Ableitung aus der Quote-Detailantwort
- Anzeigeort im Angebotsdetail
- Singular-/Plural-Wording
- bewusst ausgeschlossene Sperr- und Workflowlogik
- Abschlussverifikation

## 1. Ergebnis

Die quote-weite Nacharbeitswarnung ist fuer den aktuellen MVP-Scope
abgeschlossen.

Erfuellt:

- Der Client zaehlt Positionen mit
  `latest_approval_decision.status == rejected`.
- Die Warnung erscheint im Angebotsdetail nach den Kopf-Chips.
- Die Warnung ist nur sichtbar, wenn mindestens eine Position betroffen ist.
- Singular und Plural werden getrennt formuliert.
- Die Warnung bleibt read-only.
- Es gibt keine Backend-Aenderung.
- Es gibt keine neue Persistenz, Queue, Permission oder Prozesssperre.
- Status- und Folgebelegaktionen bleiben technisch unveraendert.

Nicht umgesetzt und bewusst ausserhalb dieses Blocks:

- Backend-Aggregat `approval_summary`
- Backend-Sperre fuer Statuswechsel
- Sperre fuer Annahme, Rechnung oder Auftrag
- Positionsnummernliste oder Sprunganker
- zentrale Freigabe- oder Nacharbeitsqueue
- Aufgabenmodell mit Verantwortlichen und Faelligkeiten

## 2. Client-Befund

Die Ableitung sitzt in:

```text
_quoteRejectedApprovalDecisionCount(...)
```

Die Funktion liest ausschliesslich die bereits geladene Quote-Detailantwort:

```text
selected['items'][*]['latest_approval_decision']['status']
```

Bewertung:

- Der Client nutzt das vorhandene Backend-Readmodel.
- Antworten ohne `items` oder ohne `latest_approval_decision` bleiben
  kompatibel.
- Es entsteht kein weiterer HTTP-Roundtrip.
- Die Warnung ist konsistent mit der positionsnahen Markierung.

## 3. UI-Befund

Die Warnung erscheint im Angebotsdetail nach den Kopf-Chips und vor Hinweis,
Quote-Date und Gueltigkeit.

Angezeigte Texte:

```text
Nacharbeit offen: 1 Position
Eine Position wurde zuletzt abgelehnt. Position pruefen, Preis/Material/Zielpreis nacharbeiten und erneut Freigabe anfordern.
```

oder:

```text
Nacharbeit offen: n Positionen
Mehrere Positionen wurden zuletzt abgelehnt. Positionen pruefen, Preis/Material/Zielpreis nacharbeiten und erneut Freigabe anfordern.
```

Bewertung:

- Die Warnung ist im Angebotskopf sichtbar, bevor Nutzer zur Positionsliste
  scrollen.
- Die Warnung steht nahe an Status, Kunde, Projekt und Folgebelegen.
- Sie erzeugt keine neue Aktion und blockiert keine bestehende Aktion.
- Detailinformationen bleiben weiterhin in Positionskarte, Badge und Historie.

## 4. Fachlicher Befund

Die Warnung bedeutet:

- Mindestens eine Position braucht aus Sicht der letzten Freigabeentscheidung
  Nacharbeit.
- Das Angebot sollte vor weiterer kommerzieller Verarbeitung geprueft werden.

Die Warnung bedeutet nicht:

- Das Angebot ist technisch gesperrt.
- Alle Positionen sind betroffen.
- Eine Aufgabe wurde erzeugt.
- Eine erneute Freigabe wurde automatisch angefordert.

Bewertung:

Der Block erhoeht Sichtbarkeit ohne neue Prozesssemantik. Das ist fuer diesen
kleinen Schritt korrekt.

## 5. Scope-Grenzen

### 5.1 Keine Backend-Sperre

Statuswechsel und Folgebelegaktionen bleiben unveraendert.

Bewertung:

Eine echte Sperre muss serverseitig validiert werden und braucht eine eigene
Erledigungsregel. UI-only-Sperren waeren nicht belastbar.

### 5.2 Keine Liste betroffener Positionen

Die Warnung zeigt nur die Anzahl.

Bewertung:

Fuer den ersten Aggregationsschritt reicht das. Eine Positionsliste mit
Sprungziel waere ein eigener UX-Folgeblock.

### 5.3 Keine Queue

Die Warnung ist keine zentrale Arbeitsliste.

Bewertung:

Eine Queue braucht Verantwortliche, Filter, Statusmodell und eigenen Endpoint.

## 6. Verifikation

Ausgefuehrter Abschlussbefehl:

```text
flutter analyze lib/pages/quotes_page.dart lib/api.dart
```

Ergebnis:

- Flutter Analyzer ohne Issues.

Backend-Tests sind fuer diesen Leaf nicht erforderlich, weil keine
Backend-Dateien oder API-Vertraege geaendert wurden.

## 7. Entscheidung

Der Warnungsblock ist nach erfolgreicher Client-Verifikation abgeschlossen.

Naechster sinnvoller Folgepfad:

```text
Subtask 3.1.41.1: Backend-Prozesssperre fuer abgelehnte Positionsentscheidungen fachlich zuschneiden
```

Begruendung:

- Position und Angebotskopf machen Nacharbeit jetzt sichtbar.
- Der naechste fachliche Schritt ist die Frage, ob und welche Aktionen
  serverseitig blockiert werden sollen.
- Eine Sperre sollte vor Implementierung separat zugeschnitten werden, weil sie
  Statuswechsel, Annahme und Folgebelege betreffen kann.
