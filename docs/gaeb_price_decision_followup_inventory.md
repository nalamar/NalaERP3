# GAEB-Folgeausbau nach abgeschlossener Preisbewertung:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen kleinen read-only Preisbewertung.

Der Fokus bleibt bewusst eng:

- den kleinsten sinnvollen Folgepunkt nach sichtbarer Preisbewertung bestimmen
- entscheiden, ob jetzt eine kontrollierte Preisentscheidung, eine explizite
  Preisuebernahme aus der primaeren Quelle oder ein anderer minimaler
  kommerzieller Folgeschritt den besten Signalwert hat
- Bewertung, Entscheidung, Kalkulation und Freigabe weiterhin sauber trennen

## 1. Ausgangslage nach abgeschlossener Preisbewertung

Der aktuelle Stand deckt bereits ab:

- manuelle Materialwahl an der Quote-Position
- Materialsuche und explizite Materialuebernahme
- read-only Preisvorschlag und explizite Preisuebernahme aus dem
  Materialdurchschnitt
- read-only Preisquellenanzeige
- read-only Quellen-Priorisierung
- read-only Preisbewertung gegen die primaere priorisierte Quelle

Damit ist der kleine kommerzielle Basispfad jetzt erweitert auf:

- Material setzen
- Preisquellen sehen
- primaere Quelle erkennen
- aktuellen Positionspreis gegen die primaere Quelle bewerten
- Abweichung absolut und prozentual nachvollziehen

## 2. Verbleibende fachliche Luecke

Nach der Preisbewertung bleibt vor allem diese Luecke:

- wenn die Bewertung eine Abweichung zeigt, kann der Nutzer noch nicht
  kontrolliert entscheiden, die primaere Quelle als Positionspreis zu
  uebernehmen
- die bisherige Preisuebernahme arbeitet nur mit dem Materialdurchschnitt,
  nicht mit der inzwischen priorisierten primaeren Quelle
- die read-only Bewertung macht Transparenz sichtbar, beendet aber den kleinen
  manuellen Entscheidungsfluss noch nicht

Die relevante Luecke liegt damit nicht in neuer Kalkulation, sondern in einer
kontrollierten, expliziten Preisentscheidung auf genau einer Position.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- Vollkalkulation mit Marge, Gemeinkosten, Zuschlaegen oder Rabatten
- automatische Preiswahl ohne Nutzeraktion
- Bulk-Preisentscheidung ueber mehrere Positionen
- Freigabe- oder Eskalationsworkflow
- Zielmargen, Deckungsbeitrag oder Angebotsgesamtbewertung
- KI-gestuetzte Preisoptimierung

Diese Themen waeren bereits deutlich groessere Kalkulations- oder
Automatikstufen.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: kontrollierte Preisentscheidung aus primaerer Quelle

Vorteile:

- schliesst direkt an die sichtbare Preisbewertung an
- bleibt positionsnah und explizit
- nutzt die bereits priorisierte primaere Quelle statt eines neuen
  Regelwerks
- fuehrt nur einen kleinen Write-Pfad ein, ohne Kalkulation oder Automatik
  zu starten

Nachteile:

- muss klar als Nutzerentscheidung modelliert werden
- darf nicht wie eine automatische Preisoptimierung wirken

### Option B: weitere read-only Bewertungsdetails

Vorteile:

- bleibt risikoarm und rein lesend

Nachteile:

- die zentrale Transparenz ist bereits vorhanden
- weitere Details haetten aktuell wenig Entscheidungsnutzen
- wuerden den eigentlichen naechsten Handgriff nur verschieben

### Option C: fruehe Margen- oder Zuschlagslogik

Vorteile:

- ist spaeter fachlich relevant fuer echte Angebotskalkulation

Nachteile:

- kommt vor einer kontrollierten Preisentscheidung zu frueh
- wuerde Kostenbasis, Verkaufspreislogik und Freigabe in einem Schritt
  vermischen
- erfordert deutlich mehr Datenmodell- und UI-Entscheidungen

## 5. Kleinster sinnvoller Folgepunkt

Der kleinste sinnvolle Folgepunkt ist:

- eine explizite Preisuebernahme aus der primaeren priorisierten Quelle direkt
  an genau einer bereits gemappten Draft-Quote-Position

Diese Entscheidung soll nur tun:

- aktuelle primaere Quelle serverseitig erneut bestimmen
- deren `unit_price` auf genau eine Quote-Position uebernehmen
- `price_mapping_status` weiter als `manual` markieren
- aktualisierte Quote zurueckgeben

Sie soll bewusst noch nicht:

- einen neuen Preis berechnen
- mehrere Quellen mitteln
- Margen oder Zuschlaege anwenden
- mehrere Positionen gleichzeitig aendern
- eine Freigabe ausloesen

## 6. Warum jetzt die kontrollierte Preisentscheidung den besten Signalwert hat

Die kontrollierte Preisentscheidung ist jetzt der beste naechste Schritt, weil:

- die primaere Quelle bereits sichtbar und priorisiert ist
- die Abweichung zum aktuellen Positionspreis bereits bewertet ist
- der Nutzer den fachlichen Kontext fuer eine explizite Uebernahme hat
- der Schritt den manuellen Entscheidungsfluss schliesst, ohne Automatik
  einzufuehren
- eine spaetere Margen- oder Freigabelogik auf einem klar gesetzten
  Positionspreis aufbauen kann

Damit liefert eine explizite Preisuebernahme aus der primaeren Quelle jetzt
mehr Signal als weitere read-only Details.

## 7. Warum nicht zuerst Margen- oder Zuschlagslogik

Margen- oder Zuschlagslogik waere jetzt zu frueh, weil:

- der Positionspreis noch nicht kontrolliert aus der bewerteten Kostenbasis
  entschieden werden kann
- sonst zwei Ebenen zugleich vermischt wuerden:
  - welche Kostenbasis soll als Preisanker gelten?
  - welche Marge oder welcher Zuschlag soll darauf angewendet werden?
- ein spaeterer kalkulationsnaher Schritt belastbarer ist, wenn die
  kontrollierte Preisentscheidung stabil ist

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- ein enger Write-Pfad fuer genau eine bereits gemappte Draft-Quote-Position
- Grundlage ist ausschliesslich die primaere Quelle aus der bestehenden
  Quellen-Priorisierung
- keine automatische Ausfuehrung, sondern explizite Nutzeraktion
- Rueckgabe der aktualisierten Quote ueber den bestehenden Editorfluss

Wichtig bleibt:

- keine Bulk-Aktion
- keine Margen-, Zuschlags- oder Rabattlogik
- keine Freigabe- oder Eskalationslogik
- keine neuen Tabellen und keine persistierte Entscheidungsakte
- keine globale Preisarbeitsflaeche

## 9. Warum die Quote-Position weiter der richtige Ort ist

Die Quote-Position bleibt auch fuer diesen Folgepunkt der richtige Ort, weil
dort bereits zusammenlaufen:

- gesetztes `material_id`
- aktueller `unit_price`
- sichtbare Preisbewertung
- primaere Kostenbasis
- manuelle Preisverantwortung des Nutzers

Eine separate Entscheidungs- oder Kalkulationsansicht waere fuer diesen ersten
Write-Pfad unnoetig gross.

## 10. Nutzen vor spaeterer Kalkulationslogik

Schon ohne Kalkulationslogik schafft die kontrollierte Preisentscheidung:

- eine nachvollziehbare Uebernahme aus der bewerteten primaeren Quelle
- weniger manuelle Fehleingabe beim Positionspreis
- klare Grundlage fuer spaetere Margen- und Zuschlagslogik
- konsistenten Anschluss an die bisherige explizite Material- und
  Preisuebernahme

## 11. Entscheidung

Der naechste sinnvolle Folgeausbau nach der abgeschlossenen kleinen
Preisbewertung ist eine kontrollierte, explizite Preisuebernahme aus der
primaeren priorisierten Quelle an genau einer gemappten Draft-Quote-Position.

## 12. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer diese explizite Preisuebernahme
  zuschneiden, bewusst noch ohne Bulk, Margen-, Zuschlags-, Rabatt- oder
  Freigabelogik
