# GAEB-Folgeausbau nach sichtbarem Margenanker:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach dem
abgeschlossenen read-only Margenanker.

Der Fokus bleibt bewusst eng:

- entscheiden, ob jetzt ein minimaler Abweichungs-/Freigabeanker,
  Zielmarge-/Zuschlagslogik oder eine engere Positionskalkulation den besten
  Signalwert hat
- den kleinsten fachlich belastbaren Folgeschnitt bestimmen
- Freigabe, Kalkulation und Automatik weiterhin sauber trennen

## 1. Ausgangslage nach sichtbarer Marge

Der aktuelle Stand deckt bereits ab:

- Material- und Preisquellenbezug an der Quote-Position
- priorisierte Preisquellen
- explizite Primaerpreis-Uebernahme
- persistierte Preisentscheidung
- sichtbare Preisentscheidungshistorie
- read-only Margenanker auf Basis der letzten gespeicherten Preisentscheidung

Damit sieht der Nutzer jetzt:

- aktuelle `unit_price`
- gespeicherte Kostenbasis
- absolute Marge
- prozentuale Marge
- einfachen Status `negative_margin`, `zero_margin` oder `positive_margin`

## 2. Verbleibende fachliche Luecke

Nach sichtbarer Marge bleibt eine neue konkrete Luecke:

- eine negative Marge ist sichtbar
- es gibt aber noch keinen fachlichen Hinweis, ob diese Abweichung besondere
  Aufmerksamkeit oder spaetere Freigabe braucht
- es gibt noch keine Zielmarge, keinen Zuschlag und keine Regel, die Preise
  automatisch veraendert

Die naechste Luecke liegt damit in einer ersten Verantwortlichkeits- und
Risikomarkierung, nicht in einer neuen Preisberechnung.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten engen Blocks:

- echte Freigabeentscheidung
- Rollen- oder Berechtigungsmatrix
- Eskalationsworkflow
- Zielmarge als Stammdaten- oder Angebotsregel
- Zuschlagsmodell
- Rabattlogik
- automatische Preisveraenderung
- Aggregation ueber mehrere Positionen
- KI-gestuetzte Kalkulation

Diese Themen brauchen eine erkennbare Abweichungsbasis, sind aber groesser als
der naechste kleinste Schnitt.

## 4. Vergleich der naheliegenden Folgeoptionen

### Option A: minimaler Abweichungs-/Freigabeanker

Vorteile:

- nutzt das jetzt sichtbare Margensignal direkt
- macht kritische Positionen ohne neue Preislogik erkennbar
- kann read-only und positionsnah bleiben
- bereitet spaetere Freigabe-Workflows vor
- braucht noch keine Rollen, keine Persistenz und keinen Schreibpfad

Nachteile:

- darf nicht so tun, als waere bereits ein echter Freigabeprozess vorhanden
- muss bewusst als Hinweis oder Freigabevorbereitung formuliert werden
- darf keine versteckte Zielmargenlogik einfuehren

### Option B: Zielmarge-/Zuschlagslogik

Vorteile:

- waere ein direkter Schritt in Richtung Angebotskalkulation
- kann spaeter Preisvorschlaege und Zuschlaege fachlich fundieren
- passt langfristig zum ERP-Zielbild

Nachteile:

- fuehrt neue Kalkulationsregeln ein
- braucht fachliche Parameter wie Zielmarge, Gemeinkosten oder Zuschlagstypen
- kann schnell zu Schreibpfaden und Preisautomatik fuehren
- ist groesser als ein erster Risikohinweis

### Option C: engere Positionskalkulation

Vorteile:

- koennte Material, Arbeit, Zuschlaege und Gemeinkosten perspektivisch
  zusammenfuehren
- passt zum spaeteren Metallbau-ERP-Zielbild
- schafft Grundlage fuer belastbare Angebotskalkulation

Nachteile:

- ist ein eigener groesserer Kalkulationsstrang
- braucht mehr Datenquellen als die aktuelle Kostenbasis
- wuerde den engen GAEB-Folgepfad zu stark erweitern

## 5. Entscheidung

Der naechste sinnvolle Folgeausbau ist ein kleiner read-only
Abweichungs-/Freigabeanker an genau einer Quote-Position.

Dieser Block soll keine Freigabe erteilen, keinen Workflow starten und keinen
Preis veraendern. Er soll nur sichtbar machen:

- ob die sichtbare Marge unkritisch ist
- ob die Position wegen negativer Marge spaeter freigaberelevant werden kann
- welche Kostenbasis und welche Marge diese Einschaetzung begruenden

## 6. Warum jetzt Freigabeanker und nicht Zielmarge

Eine Zielmarge waere fachlich wertvoll, aber sie braucht zuerst Regeln:

- welcher Zielwert gilt?
- gilt er je Materialgruppe, Projekt, Kunde oder Angebot?
- ist der Zielwert nur Hinweis oder bindende Grenze?
- darf daraus ein neuer Preis vorgeschlagen werden?

Diese Fragen sind groesser als der naechste kleine Schritt. Ein read-only
Abweichungsanker kann dagegen bereits mit dem vorhandenen Margensignal arbeiten
und spaetere Zielmargen vorbereiten.

## 7. Warum jetzt keine engere Positionskalkulation

Die engere Positionskalkulation braucht mehr als eine Kostenbasis:

- Materialkosten
- Arbeitszeit
- Fertigungskosten
- Gemeinkosten
- Zuschlaege
- Rabattlogik

Der aktuelle Stand bildet erst den ersten Kostenanker ab. Der naechste Schritt
soll deshalb nur markieren, ob dieser Kostenanker eine kritische Abweichung
zeigt.

## 8. Empfohlenes Minimalzielbild

Das Minimalziel fuer den naechsten Strang ist:

- read-only Bewertung fuer genau eine Quote-Position
- Grundlage: vorhandener Margenanker
- Status wie `approval_not_required`, `approval_recommended` oder
  `approval_blocked_until_margin_available`
- Grundtext, der negative Marge oder fehlende Marge knapp erklaert
- Anzeige der relevanten Marge und Kostenbasis

Bewusst nicht enthalten:

- Freigabe speichern
- Freigabe anfordern
- Freigabe erteilen oder ablehnen
- Rollenmodell
- Eskalationsregel
- Zielmarge
- Zuschlagsregel
- Preisveraenderung
- Bulk-Freigabe

## 9. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer einen read-only
  Abweichungs-/Freigabeanker an genau einer Quote-Position zuschneiden

Dieses Zielmodell muss entscheiden:

- ob der neue Service direkt auf `MarginAnchorForQuoteItem(...)` aufsetzt
- welche Statuswerte und Gruende der Client anzeigen soll
- wie fehlende Marge oder fehlende Preisentscheidung dargestellt wird
- wo das Signal im bestehenden Quote-Editor positionsnah erscheinen soll
