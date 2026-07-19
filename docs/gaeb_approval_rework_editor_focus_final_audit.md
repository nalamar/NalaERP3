# GAEB Approval Rework Editor Focus Final Audit

## Scope

- Leaf: `3.1.46.4`
- Ziel: Editor-Fokus fuer Nacharbeitsposition abschliessend pruefen und den naechsten kleinen GAEB-Folgepfad zuschneiden.
- Bezug:
  - `docs/gaeb_approval_rework_editor_context_inventory.md`
  - `docs/gaeb_approval_rework_editor_focus_strategy.md`
  - `docs/gaeb_approval_rework_editor_focus_audit.md`

## Abschlusspruefung

Der Fokuspfad ist fuer den aktuellen MVP-Zuschnitt geschlossen:

- Die Detailansicht zeigt offene Nacharbeit quote-weit und positionsnah.
- Abgelehnte Entwurfspositionen zeigen im Angebotsdetail eine `Bearbeiten`-Aktion.
- Die Aktion uebergibt Item-ID und Positionsnummer an den bestehenden Editor-Dialog.
- Der Editor sucht das Ziel bevorzugt per Item-ID und nutzt die Positionsnummer als Fallback.
- Der Editor scrollt nach dem ersten Rendern per `Scrollable.ensureVisible(...)` zur Zielposition.
- Die Zielposition bleibt im Editor dezent hervorgehoben.
- Die vorhandenen Nacharbeits- und Freigabeinformationen in `_QuoteItemRow` bleiben unveraendert sichtbar.
- Die Umsetzung bleibt client-only und veraendert keine API-, Server- oder Datenbankvertraege.

## Nutzungsbewertung

Die gefuehrte Nacharbeitsstrecke ist jetzt durchgaengig:

1. Angebot zeigt offene Nacharbeit.
2. Warnung zeigt betroffene Positionsnummern.
3. Nutzer kann zur ersten Nacharbeitsposition springen.
4. Zielposition wird hervorgehoben.
5. Position zeigt Ablehnungsgrund und optionalen Kommentar.
6. Nutzer kann die betroffene Position direkt im Editor oeffnen.
7. Editor scrollt zur Zielposition und hebt sie hervor.

Damit ist die Navigation von der Warnung bis zur konkreten Bearbeitungsstelle
ausreichend geschlossen.

## Bewusst offene Punkte

Diese Punkte bleiben ausserhalb von Task `3.1.46`:

- keine automatische Preis-, Material- oder Zielpreisaenderung
- keine automatische erneute Freigabeanforderung
- keine Nacharbeitsqueue
- keine Mehrpositionsnavigation im Editor
- keine serverseitige Nacharbeitsstatus-Entitaet
- keine API-Fehlerdetails mit Positionsliste

## Naechster kleiner GAEB-Folgepfad

Nach der gefuehrten Navigation ist die naechste fachliche Luecke der Abschluss
der Nacharbeit:

- Offene Nacharbeit wird weiterhin durch die letzte terminale Entscheidung
  `latest_approval_decision.status == rejected` bestimmt.
- Eine Positionsaenderung allein hebt die Prozesssperre nicht auf.
- Die Sperre loest sich erst, wenn eine spaetere Freigabeentscheidung genehmigt
  ist.
- Nutzer brauchen deshalb eine klare Strecke von `Position nacharbeiten` zu
  `erneut Freigabe anfordern`.

Kleinster sinnvoller Anschluss:

```text
Subtask 3.1.47.1: Erneute Freigabe nach Nacharbeit fachlich inventarisieren
```

Vorgeschlagener Zuschnitt:

- bestehende Freigabeanforderung und Zielmargen-Anker pruefen
- klaeren, wann eine nachbearbeitete Position erneut freigabefaehig ist
- pruefen, ob der Editor bereits genug sichtbare Aktion bietet
- keine neue Persistenz im ersten Schritt
- keine automatische Entscheidung
- keine Queue

## Verifikation dieses Leafs

Keine Runtime-Aenderung in diesem Leaf. Bestehende Verifikation aus `3.1.46.3`
bleibt gueltig:

- `dart format lib/pages/quotes_page.dart`
- `flutter analyze lib/pages/quotes_page.dart lib/api.dart`

Beide Pruefungen waren erfolgreich.
