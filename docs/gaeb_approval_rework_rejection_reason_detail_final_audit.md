# GAEB Approval Rework Rejection Reason Detail Final Audit

## Scope

- Leaf: `3.1.45.2`
- Ziel: Ablehnungsgrund im Angebotsdetail abschliessend pruefen und den naechsten kleinen GAEB-Folgepfad zuschneiden.
- Bezug:
  - `docs/gaeb_approval_rework_rejection_reason_detail_audit.md`
  - `docs/gaeb_approval_rework_jump_highlight_final_audit.md`

## Abschlusspruefung

Der Detail-Hinweis ist fuer den aktuellen MVP-Zuschnitt geschlossen:

- Die Angebotsdetail-Positionszeile liest `latest_approval_decision` aus dem bestehenden Quote-Item-JSON.
- Nur `status == rejected` erzeugt eine sichtbare Nacharbeitszeile.
- `reason_text` wird bevorzugt angezeigt.
- Bekannte `reason_code`s werden lokal lesbar gemappt.
- Unbekannte `reason_code`s bleiben sichtbar statt still verworfen zu werden.
- `decision_comment` wird optional an den Hinweis angehaengt.
- Positionen ohne `latest_approval_decision` oder ohne Ablehnung bleiben unveraendert.
- Die Umsetzung bleibt client-only und veraendert keine API-, Server- oder Datenbankvertraege.

## Nutzungsbewertung

Die Nacharbeitsfuehrung im Angebotsdetail ist jetzt ausreichend geschlossen:

1. Kopf-Warnung nennt offene Nacharbeit.
2. Betroffene Positionsnummern sind sichtbar.
3. Nutzer kann zur ersten Position springen.
4. Zielposition wird kurz hervorgehoben.
5. Die Positionszeile zeigt den Ablehnungsgrund und optionalen Kommentar.

Damit ist die fachliche Ursache der Nacharbeit im Angebotsdetail sichtbar, ohne
eine volle Freigabehistorie oder eine zentrale Nacharbeitsqueue einzufuehren.

## Bewusst offene Punkte

Diese Punkte bleiben ausserhalb von Task `3.1.45`:

- keine vollstaendige Freigabehistorie im Angebotsdetail
- keine direkte Bearbeitungsaktion in der Positionszeile
- kein Fokus auf eine konkrete Editor-Position
- keine Mehrpositions-Navigation
- keine serverseitige Nacharbeitsliste

## Naechster kleiner GAEB-Folgepfad

Nach Sichtbarkeit und Grundanzeige ist die naechste kleine Luecke der Uebergang
von der Detailansicht in die Nacharbeit. Der Nutzer sieht jetzt, welche Position
warum abgelehnt wurde, muss die Position im Bearbeitungsdialog aber selbst
wiederfinden.

Kleinster sinnvoller Anschluss:

```text
Subtask 3.1.46.1: Nacharbeitsposition aus Angebotsdetail im Editor-Kontext vorselektieren
```

Vorgeschlagener Zuschnitt:

- client-only in `client/lib/pages/quotes_page.dart`
- beim Klick auf eine abgelehnte Detailposition oder einen kleinen Button
  `Bearbeiten` die bestehende Angebotsbearbeitung oeffnen
- Positionsnummer oder Item-ID an den Editor-Dialog uebergeben
- im Editor die betroffene Position initial sichtbar machen oder markieren
- keine Backend-/API-Aenderung
- keine automatische Preis- oder Materialaenderung
- keine Queue und kein Batch-Workflow

Vor einer Implementierung sollte der naechste Leaf zuerst inventarisieren, wie
`_QuoteEditorDialog` aktuell Positionen rendert und ob dort bereits Keys oder
Scroll-Controller existieren.

## Verifikation dieses Leafs

Keine Runtime-Aenderung in diesem Leaf. Bestehende Verifikation aus `3.1.45.1`
bleibt gueltig:

- `dart format lib/pages/quotes_page.dart`
- `flutter analyze lib/pages/quotes_page.dart lib/api.dart`

Beide Pruefungen waren erfolgreich.
