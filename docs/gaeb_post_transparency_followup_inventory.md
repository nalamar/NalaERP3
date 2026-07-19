# GAEB-Folgeausbau nach Preisentscheidungs-Transparenz:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen read-only Preisentscheidungs-Transparenz.

Der Fokus bleibt bewusst eng:

- den kleinsten kommerziellen Folgepunkt nach Bewertung, Primaerpreis-
  Uebernahme und Transparenz bestimmen
- entscheiden, ob jetzt ein kalkulationsnaher Margen-/Zuschlagsanker, eine
  kleine Preisentscheidungs-Persistenz oder ein minimaler
  Abweichungs-/Freigabeanker den besten Signalwert hat
- Kalkulation, Entscheidungshistorie, Freigabe und Automatik weiter sauber
  trennen

## 1. Ausgangslage nach abgeschlossener Transparenz

Der aktuelle Stand deckt bereits ab:

- GAEB-Positionen koennen kontrolliert in Draft-Quotes uebernommen werden
- Quote-Positionen koennen mit Material verbunden werden
- Preisquellen sind sichtbar
- Preisquellen werden priorisiert
- der aktuelle Positionspreis kann gegen die primaere Quelle bewertet werden
- die primaere Quelle kann explizit als Positionspreis uebernommen werden
- die Position kann read-only zeigen, ob der aktuelle Preis der aktuellen
  primaeren Quelle entspricht oder davon abweicht

Damit ist der kleine operative Preisfluss jetzt geschlossen:

- Material setzen
- Kostenanker sehen
- Kostenanker priorisieren
- Preis bewerten
- Primaerpreis uebernehmen
- Uebereinstimmung oder Abweichung sichtbar machen

## 2. Verbleibende fachliche Luecke

Nach diesem Stand bleibt eine neue Luecke:

- die aktuelle Uebereinstimmung wird live aus vorhandenen Daten berechnet
- die eigentliche Preisentscheidung wird nicht als Ereignis oder Snapshot
  festgehalten
- es gibt keinen stabilen historischen Bezug auf die Quelle, die zum Zeitpunkt
  der Uebernahme galt

Das ist fuer spaetere Kalkulation relevant, weil Margen, Zuschlaege und
Freigaben fachlich auf einer nachvollziehbaren Kostenbasis aufsetzen sollten.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten engen Blocks:

- Vollkalkulation eines Angebots
- Zielmarge oder Deckungsbeitrag ueber alle Positionen
- Zuschlagsmatrix, Gemeinkosten oder Rabattstaffeln
- Freigabe-Workflow mit Rollen und Eskalation
- Bulk-Bewertung oder Bulk-Preisentscheidung
- automatische Preisoptimierung
- KI-gestuetzte Preisstrategie

Diese Themen brauchen eine stabilere Entscheidungsgrundlage.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: kalkulationsnaher Margen-/Zuschlagsanker

Vorteile:

- ist fachlich der naechste sichtbare Schritt Richtung Angebotskalkulation
- bringt das ERP-Zielbild naeher an Verkaufspreislogik und Deckungsbeitrag
- kann spaeter direkt aus GAEB-Positionen Angebotswerte ableiten

Nachteile:

- wuerde auf einer nur live berechneten Kostenbasis starten
- die Quelle zum Zeitpunkt der Preisentscheidung waere nicht stabil
  dokumentiert
- unklar bleibt, ob eine Marge auf Einkaufspreis, aktueller Primaerquelle oder
  bereits gesetztem Positionspreis gerechnet wird
- fuehrt schnell zu neuen Feldern und Regeln, bevor die Entscheidung selbst
  auditierbar ist

### Option B: kleine Preisentscheidungs-Persistenz

Vorteile:

- baut direkt auf der expliziten Primaerpreis-Uebernahme auf
- haelt den Entscheidungszeitpunkt und den verwendeten Quellensnapshot fest
- macht spaetere Margen-, Zuschlags- und Freigabelogik belastbarer
- bleibt positionsnah und kann auf genau eine Entscheidung pro Aktion
  begrenzt werden
- erfordert noch keine Kalkulationsregeln

Nachteile:

- fuehrt erstmals neue Persistenz fuer diesen Preisentscheidungsstrang ein
- muss sehr eng bleiben, damit keine vollwertige Historien- oder
  Freigabeakte entsteht

### Option C: minimaler Abweichungs-/Freigabeanker

Vorteile:

- koennte Preise unter Kostenbasis oder starke Abweichungen markieren
- passt spaeter zu Verantwortlichkeiten im ERP

Nachteile:

- Freigabe braucht eine nachvollziehbare Entscheidung und Kostenbasis
- ohne persistierten Snapshot ist die Begruendung spaeter instabil
- fuehrt schnell Workflow-Zustaende ein, bevor die Entscheidungsdaten
  belastbar sind

## 5. Kleinster sinnvoller Folgepunkt

Der kleinste sinnvolle Folgepunkt ist:

- eine enge Preisentscheidungs-Persistenz fuer die explizite Uebernahme der
  primaeren Quelle an genau einer Quote-Position

Dieser Schritt soll nur festhalten:

- Quote-Position
- Materialbezug
- verwendete primaere Quelle zum Entscheidungszeitpunkt
- uebernommener Preis
- Waehrung
- Quellreferenz und Quelldatum, soweit vorhanden
- Entscheidungsart wie `primary_source_applied`
- Zeitpunkt der Entscheidung

Er soll bewusst noch nicht:

- Margen oder Zuschlaege berechnen
- Rabatte anwenden
- Freigaben ausloesen
- Entscheidungskommentare oder Rollenmodell erzwingen
- Bulk-Entscheidungen speichern
- Preise automatisch setzen

## 6. Warum jetzt nicht sofort Marge oder Zuschlag

Ein Margen-/Zuschlagsanker waere jetzt fachlich reizvoll, aber noch zu frueh.

Grund:

- die Preisbasis ist sichtbar und bewertbar, aber noch nicht als
  Entscheidungssnapshot stabil
- spaetere Kalkulation muss erklaeren koennen, welche Quelle als Kostenbasis
  galt
- ohne Persistenz wuerden spaetere Aenderungen an Preisquellen die
  historische Nachvollziehbarkeit schwaechen

Ein kleiner Persistenzanker schafft damit mehr strukturellen Wert als eine
erste Margenformel.

## 7. Warum jetzt nicht sofort Freigabe

Ein Freigabeanker braucht mindestens:

- belastbare Entscheidungsdaten
- nachvollziehbare Kostenbasis
- klare Abweichungsregel
- Verantwortlichkeitsmodell

Davon ist aktuell nur die read-only Abweichung vorhanden. Eine Freigabe waere
deshalb fachlich groesser als der naechste sinnvolle Schritt.

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- ein kleiner persistierter Preisentscheidungs-Snapshot genau dann, wenn die
  primaere Quelle explizit uebernommen wird
- keine neue Benutzerentscheidung im UI
- keine neue Arbeitsflaeche
- keine Kalkulationsregel
- keine Freigabe

Der bestehende Pfad `ApplyPrimaryPriceSourceForQuoteItem(...)` ist der
richtige Anknuepfungspunkt, weil dort die explizite Entscheidung bereits
stattfindet und die primaere Quelle serverseitig bestimmt wird.

## 9. Entscheidung

Der naechste sinnvolle Folgeausbau nach der abgeschlossenen
Preisentscheidungs-Transparenz ist nicht sofort Marge, Zuschlag oder Freigabe,
sondern eine kleine Preisentscheidungs-Persistenz fuer die explizite
Primaerpreis-Uebernahme.

## 10. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer die Preisentscheidungs-Persistenz
  zuschneiden, bewusst noch ohne Margen-, Zuschlags-, Rabatt-, Freigabe-,
  Bulk- oder Automatiklogik

