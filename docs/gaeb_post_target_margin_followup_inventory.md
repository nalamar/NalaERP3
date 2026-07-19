# GAEB-Folgeausbau nach abgeschlossenem Zielmargenanker:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach dem
abgeschlossenen read-only Zielmargen-/Zuschlagsanker.

Der Fokus bleibt bewusst eng:

- entscheiden, ob Zielwert-Konfiguration, kontrollierte Zielpreis-Uebernahme
  oder echter Freigabe-Workflow den besten naechsten Signalwert hat
- den kleinsten fachlich belastbaren Folgeschnitt bestimmen
- Zielwert, Preisentscheidung, Freigabe und Automatik weiterhin sauber trennen

## 1. Ausgangslage nach Zielmargenanker

Der aktuelle Stand deckt bereits ab:

- gespeicherte Preisentscheidung als Kostenbasis
- read-only Margenanker
- read-only Approval-Hint
- read-only Zielmargenanker an genau einer Quote-Position
- technischer Default `target_margin_percent = 20.00`
- Zielpreis aus Kostenbasis und Zielwert
- Zielabweichung absolut und prozentual
- Statuswerte wie `below_target`, `on_target` und
  `target_blocked_until_margin_available`
- positionsnahe Anzeige im bestehenden Quote-Editor

Damit sieht der Nutzer jetzt:

- aktuelle Kostenbasis
- aktuellen Verkaufspreis
- aktuelle Marge
- Zielwert
- Zielpreis
- Abweichung zum Zielpreis

## 2. Verbleibende fachliche Luecke

Die zentrale Luecke ist jetzt nicht mehr die Berechnung eines Zielpreises,
sondern die Herkunft des Zielwerts.

Aktuell ist der Zielwert:

- technisch konstant
- nicht konfigurierbar
- nicht als fachliche Regel nachvollziehbar
- nicht je Mandant, Angebot, Kunde, Projekt oder Materialgruppe ableitbar

Solange der Zielwert nur ein technischer Default ist, waeren Preisuebernahme
oder Freigabe-Workflow fachlich zu frueh.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten engen Blocks:

- Zielpreis automatisch setzen
- Zielpreis manuell uebernehmen
- Freigabe anfordern
- Freigabe erteilen oder ablehnen
- Rollen- oder Berechtigungsmatrix
- Eskalation oder Kommentierung
- Zielwert je Kunde, Projekt oder Materialgruppe
- Angebotsweite Zielmargenpruefung
- Rabatt-, Nachlass- oder Gemeinkostenlogik
- KI-gestuetzte Zielwertfindung

Diese Themen bleiben fachlich wichtig, brauchen aber zuerst eine belastbare
Quelle fuer den Zielwert.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: Zielwert-Konfiguration und Persistenz

Vorteile:

- ersetzt den technischen Default durch eine fachlich sichtbare Grundlage
- macht spaetere Preisuebernahme und Freigabe belastbarer
- kann klein starten, etwa als globaler Angebots-Zielmargenwert
- bleibt zunaechst ohne Preisveraenderung und ohne Workflow
- reduziert das Risiko, dass ein harter Default als echte Regel missverstanden
  wird

Nachteile:

- fuehrt eine erste persistente Kalkulationsregel ein
- braucht eine klare Scope-Grenze, damit keine Regelmatrix entsteht
- muss bewusst ohne Preisautomatik bleiben

### Option B: kontrollierte Zielpreis-Uebernahme

Vorteile:

- schliesst direkt an den sichtbaren Zielpreis an
- koennte eine klare Nutzeraktion anbieten
- waere operativ schnell nuetzlich

Nachteile:

- wuerde einen Verkaufspreis aus einem technischen Default ableiten
- erzeugt einen neuen Write-Pfad
- braucht eine Entscheidung, ob und wie ein Zielpreis-Snapshot persistiert wird
- kann spaeter schwer korrigierbar sein, wenn Zielwerte noch nicht fachlich
  konfiguriert sind

### Option C: echter Freigabe-Workflow

Vorteile:

- passt langfristig zu niedriger Marge und Zielabweichungen
- schafft Verantwortlichkeit und Historie
- ist ein wichtiger ERP-Baustein

Nachteile:

- braucht Rollen, Status, Historie und Verantwortliche
- braucht stabile Schwellwerte, sonst ist unklar, wann Freigabe noetig ist
- waere ein groesserer Workflow-Strang
- sollte nicht auf einem technischen Default statt einer Zielwertregel
  aufbauen

## 5. Entscheidung

Der naechste sinnvolle Folgeausbau ist eine kleine Zielwert-Konfiguration und
Persistenz fuer den Zielmargenanker.

Der erste Schnitt soll bewusst nur eine Frage beantworten:

- Welcher Zielmargenwert gilt als fachliche Grundlage fuer die erste
  Zielpreisbewertung?

Er soll noch nicht:

- Preise veraendern
- Zielpreise uebernehmen
- Freigaben starten
- Rollen einfuehren
- kundenspezifische oder materialgruppenspezifische Regeln modellieren

## 6. Warum jetzt Zielwert-Konfiguration und nicht Zielpreis-Uebernahme

Eine Zielpreis-Uebernahme waere operativ attraktiv, aber sie wuerde aktuell
einen Preis aus einem technischen Default erzeugen.

Vor einer solchen Schreibaktion sollte klar sein:

- woher der Zielwert kommt
- wer ihn fachlich verantwortet
- ob er sichtbar konfiguriert ist
- ob der Zielwert nur Hinweis oder verbindliche Regel ist

Die kleine Zielwert-Konfiguration klaert zuerst diese Grundlage.

## 7. Warum jetzt kein Freigabe-Workflow

Ein Freigabe-Workflow braucht mindestens:

- ein pruefbares Zielsignal
- eine stabile Zielwertquelle
- einen Statusraum
- Verantwortliche oder Rollen
- Historie oder Kommentare

Aktuell ist nur das Zielsignal vorhanden. Die Zielwertquelle ist noch ein
technischer Default. Deshalb waere ein Freigabe-Workflow jetzt zu gross und
fachlich zu frueh.

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- genau ein persistenter, globaler Zielmargenwert fuer Angebotspositionen
- Nutzung durch `TargetMarginAnchorForQuoteItem(...)`
- read-only Ausgabe des verwendeten Zielwerts bleibt erhalten
- keine Preisveraenderung
- keine Freigabe
- keine Regelmatrix

Moeglicher technischer Schnitt:

- Settings- oder Kalkulationsparameter `quote_target_margin_percent`
- Default weiterhin `20.00`, wenn kein Wert konfiguriert ist
- Zielmargenanker liest den konfigurierten Wert statt einer hart codierten
  Konstante
- kleine Settings- oder API-Schreibflaeche erst nach technischem Zielmodell
  bewerten

## 9. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein technisches Minimalzielmodell fuer die Zielwert-Konfiguration und
  Persistenz zuschneiden

Dieses Zielmodell muss entscheiden:

- ob der Zielwert in der bestehenden Settings-Domaene oder einer eigenen
  Kalkulationsparameter-Struktur liegt
- welcher Default gilt, wenn noch kein Zielwert gespeichert ist
- welche Validierung fuer Prozentwerte gilt
- ob die erste Stufe nur Backend/Settings oder auch eine minimale Client-Fläche
  enthaelt
- wie der Zielmargenanker den konfigurierten Wert ohne Preisautomatik nutzt
