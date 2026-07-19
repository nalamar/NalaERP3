# GAEB Approval Rework Jump Highlight Final Audit

## Scope

- Leaf: `3.1.44.2`
- Ziel: Hervorhebung nach Sprung zur Nacharbeitsposition final pruefen und den naechsten kleinen GAEB-Folgepfad zuschneiden.
- Bezug:
  - `docs/gaeb_approval_rework_jump_target_final_audit.md`
  - `docs/gaeb_approval_rework_jump_highlight_audit.md`

## Abschlusspruefung

Der UI-Pfad ist fuer den aktuellen MVP-Zuschnitt geschlossen:

- Die Nacharbeitswarnung springt zur ersten betroffenen Position.
- Die Zielposition wird unmittelbar vor dem Scroll hervorgehoben.
- Die Hervorhebung ist bewusst kurzlebig und wird nach drei Sekunden entfernt.
- `_quoteItemHighlightToken` verhindert konkurrierende Auto-Reset-Effekte bei schnellem mehrfachen Klick.
- Beim Wechsel des Angebotsdetails wird die Hervorhebung geloescht.
- Die Umsetzung bleibt rein clientseitig und nutzt keine neuen API-, Server- oder Datenbankvertraege.

## Nutzungsbewertung

Der Ablauf ist jetzt ausreichend gefuehrt:

1. Nutzer sieht quote-weite Nacharbeitswarnung.
2. Nutzer sieht betroffene Positionsnummern.
3. Nutzer kann zur ersten Position springen.
4. Zielposition ist visuell als Kontextanker erkennbar.

Damit ist der erste Nacharbeits-Navigationspfad bedienbar, ohne bereits eine
vollstaendige Nacharbeits-Queue oder Mehrpositionsnavigation einzufuehren.

## Bewusst offene Punkte

Diese Punkte bleiben ausserhalb von Task `3.1.44`:

- keine Navigation zur naechsten oder vorherigen Nacharbeitsposition
- keine persistente Hervorhebung
- kein automatischer Editor-Fokus
- keine direkte Nacharbeitsaktion aus der Angebotsdetail-Zeile
- keine API-Fehlerdetails mit Positionsliste

## Naechster kleiner GAEB-Folgepfad

Nach Navigation und Hervorhebung ist die naechste kleine Luecke nicht mehr der
Sprung selbst, sondern die Nachvollziehbarkeit der Ablehnung im Angebotsdetail.

Aktueller Befund:

- Der Editor zeigt Freigabehistorie und positionsnahe Nacharbeitsdetails.
- Die Angebotsdetail-Positionsliste zeigt nur Beschreibung, Menge, Preis,
  Steuer und Zeilensumme.
- Die quote-weite Warnung nennt Positionsnummern, aber die Zielzeile selbst
  zeigt keinen Ablehnungsgrund oder Kommentar.

Kleinster sinnvoller Anschluss:

```text
Subtask 3.1.45.1: Ablehnungsgrund der Nacharbeitsposition im Angebotsdetail anzeigen
```

Vorgeschlagener Zuschnitt:

- client-only in `client/lib/pages/quotes_page.dart`
- `latest_approval_decision` der Positionszeile auswerten
- fuer `status == rejected` unter der Positionsbeschreibung einen kompakten
  Hinweis anzeigen
- Inhalt: `reason_text`, fallback `reason_code`, optional `decision_comment`
- keine neue API
- keine neue Persistenz
- keine Historienliste im Angebotsdetail
- keine Bearbeitungsaktion

## Verifikation dieses Leafs

Keine Runtime-Aenderung in diesem Leaf. Bestehende Verifikation aus `3.1.44.1`
bleibt gueltig:

- `dart format lib/pages/quotes_page.dart`
- `flutter analyze lib/pages/quotes_page.dart lib/api.dart`

Beide Pruefungen waren erfolgreich.
