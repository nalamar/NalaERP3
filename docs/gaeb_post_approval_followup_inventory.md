# GAEB-Folgeausbau nach abgeschlossenem Approval-Hint:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach dem
abgeschlossenen read-only Approval-Hint.

Der Fokus bleibt bewusst eng:

- entscheiden, ob jetzt Zielmarge-/Zuschlagslogik, echter Freigabe-Workflow
  oder engere Positionskalkulation den besten Signalwert hat
- den kleinsten fachlich belastbaren Folgeschnitt bestimmen
- Kalkulation, Freigabe und Automatik weiter sauber trennen

## 1. Ausgangslage nach abgeschlossenem Approval-Hint

Der aktuelle Stand deckt bereits ab:

- Herkunftsanker zwischen GAEB-Importposition und erzeugter Quote-Position
- manuelle Materialzuordnung und Materialsuche
- Preisvorschlag, Preisquellen und Quellen-Priorisierung
- explizite Primaerpreis-Uebernahme
- persistierte Preisentscheidung
- read-only Preisentscheidungs-Historie
- read-only Margenanker auf Basis der letzten gespeicherten Preisentscheidung
- read-only Approval-Hint mit drei Statuspfaden:
  - `approval_not_required`
  - `approval_recommended`
  - `approval_blocked_until_margin_available`

Damit ist jetzt sichtbar:

- welcher aktuelle Positionspreis gilt
- welche gespeicherte Kostenbasis verwendet wird
- welche absolute und prozentuale Marge daraus entsteht
- ob eine negative Marge eine spaetere Freigabe nahelegt

## 2. Verbleibende fachliche Luecke

Nach dem Approval-Hint ist die naechste Luecke nicht mehr die reine
Risikomarkierung. Die Luecke liegt jetzt in der fachlichen Bewertungsregel:

- negative Marge ist eindeutig kritisch
- Nullmarge oder positive Marge sind aber noch nicht fachlich bewertet
- es gibt keine Zielmarge
- es gibt keinen Zielaufschlag
- es gibt keine Schwelle, ab der eine Position trotz positiver Marge als
  pruefpflichtig gilt
- es gibt keine Regel, welcher Verkaufspreis aus Kostenbasis und Zielwert
  folgen wuerde

Ein echter Freigabeprozess waere ohne diese Regel nur ein Workflow um ein zu
duennes Signal.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten engen Blocks:

- Freigabe anfordern, erteilen oder ablehnen
- persistenter Freigabestatus
- Rollen- oder Berechtigungsmatrix
- Eskalation, Wiedervorlage oder Kommentarakte
- automatische Preisveraenderung
- Angebotsweite Freigabepruefung
- Vollkalkulation mit Arbeitszeit, Fremdleistung und Gemeinkosten
- KI-gestuetzte Kalkulations- oder Freigabeentscheidung

Diese Themen bleiben fachlich wichtig, sind aber groesser als der naechste
kleine Schnitt.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: Zielmarge-/Zuschlagsanker

Vorteile:

- baut direkt auf Kostenbasis und aktuellem Positionspreis auf
- erklaert, ob positive Marge wirklich ausreichend ist
- liefert die fehlende Bewertungsregel fuer spaetere Freigabe
- kann zunaechst read-only und positionsnah bleiben
- braucht noch keinen Freigabe-Schreibpfad
- kann mit einem bewusst einfachen Default-Zielwert starten

Nachteile:

- fuehrt erstmals eine fachliche Zielregel ein
- muss klar als Bewertung oder Vorschlag formuliert werden
- darf keine automatische Preissetzung verstecken

### Option B: echter Freigabe-Workflow

Vorteile:

- ist langfristig fuer negative oder zu niedrige Marge relevant
- schafft Verantwortlichkeit und Nachvollziehbarkeit
- passt zum ERP-Zielbild

Nachteile:

- braucht Rollen, Status, Historie und Verantwortliche
- braucht fachliche Schwellwerte, sonst ist unklar, wann Freigabe noetig ist
- waere ein neuer Write-Pfad mit hoeherem fachlichem Risiko
- wuerde den gerade abgeschlossenen read-only Hinweis zu schnell in Workflow
  erweitern

### Option C: engere Positionskalkulation

Vorteile:

- passt langfristig zum Metallbau-ERP-Zielbild
- kann Material, Arbeit, Fremdleistung und Gemeinkosten zusammenfuehren
- ist Grundlage fuer belastbare Angebotskalkulation

Nachteile:

- braucht mehrere neue Datenquellen und Modelle
- ist ein eigener groesserer Kalkulationsstrang
- wuerde vor einer einfachen Zielwertregel zu viel Scope aufnehmen
- ist fuer den naechsten kleinen GAEB-Folgepunkt zu breit

## 5. Entscheidung

Der naechste sinnvolle Folgeausbau ist ein kleiner read-only
Zielmargen-/Zuschlagsanker an genau einer Quote-Position.

Dieser Block soll noch keinen Preis veraendern und keine Freigabe starten. Er
soll nur sichtbar machen:

- welche Zielmarge oder welcher Zielaufschlag fuer diese erste Stufe gilt
- welcher Zielverkaufspreis daraus auf Basis der gespeicherten Kostenbasis
  folgen wuerde
- wie weit der aktuelle Positionspreis vom Zielpreis entfernt ist
- ob die Position nur negativ, unter Ziel, auf Ziel oder ueber Ziel liegt

## 6. Warum jetzt Zielmarge und nicht Freigabe

Der Approval-Hint kann negative Marge markieren. Fuer echte Freigabe reicht das
noch nicht, weil der Prozess fachlich wissen muss:

- reicht eine kleine positive Marge?
- ist Nullmarge bereits freigabepflichtig?
- gilt pro Position, Angebot, Kunde oder Materialgruppe derselbe Zielwert?
- welche Abweichung soll toleriert werden?

Ein read-only Zielmargenanker beantwortet zuerst diese Bewertungsfrage. Danach
kann ein Freigabe-Workflow deutlich sauberer zugeschnitten werden.

## 7. Warum jetzt keine engere Positionskalkulation

Die engere Positionskalkulation braucht mehr als die vorhandene Kostenbasis:

- Materialmengen und Verschnitt
- Arbeitszeit
- Maschinenzeit
- Fremdleistungen
- Gemeinkosten
- Zuschlags- und Rabattregeln

Der aktuelle GAEB-Pfad besitzt bereits eine belastbare Positionskostenbasis aus
der gespeicherten Preisentscheidung. Der kleinste naechste Schritt ist deshalb
nicht die volle Kalkulationsstruktur, sondern eine einfache Zielwertbewertung
auf dieser Kostenbasis.

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- read-only Bewertung fuer genau eine Quote-Position
- Grundlage: vorhandener Margenanker und damit gespeicherte Kostenbasis
- einfacher Default-Zielwert als technischer Startpunkt
- Ausgabe von Zielmarge, Zielpreis, aktueller Abweichung und Status
- Anzeige direkt im bestehenden Quote-Editor

Moegliche Statuswerte:

- `below_cost`: aktueller Preis liegt unter Kostenbasis
- `below_target`: aktueller Preis liegt ueber Kostenbasis, aber unter Ziel
- `on_target`: aktueller Preis erreicht den Zielwert
- `above_target`: aktueller Preis liegt ueber Ziel
- `target_blocked_until_margin_available`: keine gespeicherte Kostenbasis

Bewusst nicht enthalten:

- Zielwerte je Kunde, Projekt, Materialgruppe oder Benutzer
- Speichern von Zielmargen
- Preis automatisch anwenden
- Freigabe anfordern
- Rollen oder Eskalation
- Angebotsweite Aggregation
- KI-Automatik

## 9. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein technisches Minimalzielmodell fuer den read-only
  Zielmargen-/Zuschlagsanker zuschneiden

Dieses Zielmodell muss entscheiden:

- ob der neue Service direkt auf `MarginAnchorForQuoteItem(...)` aufsetzt
- welcher Default-Zielwert fuer die erste Stufe verwendet wird
- welche Felder und Statuswerte an den Client gehen
- wie fehlende Kostenbasis dargestellt wird
- wo der Block im bestehenden Quote-Editor positionsnah erscheint
