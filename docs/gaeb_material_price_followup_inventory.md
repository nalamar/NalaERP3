# GAEB-Preisvorschlag: Folgeinventur nach abgeschlossenem Read-only-Block

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den kleinsten sinnvollen Folgeausbau nach dem
abgeschlossenen Block `kleiner Preisvorschlag`.

Zu entscheiden ist, welcher naechste Schritt den besten Signalwert hat:

- enge explizite Preisuebernahme aus dem sichtbaren Preisanker
- ein anderer minimaler kommerzieller Folgeschritt

## 1. Ausgangslage

Der aktuelle Stand deckt jetzt bereits ab:

- manuelle Materialwahl an der Quote-Position
- read-only Preisanker nach gesetztem `material_id`
- klaren Herkunftshinweis fuer den Preisanker
- keine automatische oder stille Aenderung von `unit_price`

Damit ist die kleine Sichtbarmachung des Preisankers geloest.

Noch nicht geloest ist aber der operative Bruch zwischen:

- sichtbarem Preisanker
- und tatsaechlich gesetztem `unit_price` an der Quote-Position

## 2. Kernbeobachtung

Nach dem abgeschlossenen Read-only-Preisblock ist der naechste relevante
Schmerzpunkt nicht:

- weiterer Anzeige-Feinschliff
- fruehes Ranking mehrerer Preisquellen
- Historienlogik
- Bulk-Kalkulation

Der naechste echte operative Bedarf ist:

- ein bereits sichtbarer Preisanker soll kontrolliert und explizit auf genau
  eine Quote-Position uebernommen werden koennen

## 3. Bewertete Folgeoptionen

### Option A: Enge explizite Preisuebernahme aus dem sichtbaren Preisanker

Signalwert:

- sehr hoch

Warum:

- schliesst direkt den noch offenen manuellen Bruch
- bleibt eng am bestehenden Positions-Workflow
- nutzt den bereits sichtbaren Preisanker unmittelbar
- fuehrt noch keine neue Preislogik ein

### Option B: Weiterer Read-only-Feinschliff am Preisanker

Signalwert:

- niedrig

Warum:

- verbessert den operativen Fluss kaum
- verlaengert nur den read-only Block
- loest nicht das eigentliche Anschlussproblem

### Option C: Fruehes Ranking oder Priorisierung mehrerer Preisquellen

Signalwert:

- niedrig bis mittel, aber zu frueh

Warum:

- wuerde den Scope deutlich vergroessern
- setzt mehrere konkurrierende Preisquellen voraus
- ist vor expliziter Preisuebernahme fachlich nicht der kleinste Schritt

### Option D: Historien- oder Durchschnittslogik ausbauen

Signalwert:

- zu frueh

Warum:

- fuehrt in Preisstrategie statt in den naechsten kleinen Bedienfluss
- ist nicht noetig, um den bereits sichtbaren Preisanker operativ nutzbar zu
  machen

## 4. Entscheidung

Die Entscheidung dieser Folgeinventur ist:

- der naechste minimale Ausbau mit dem besten Signalwert ist eine enge
  explizite Preisuebernahme aus dem bereits sichtbaren Preisanker

Nicht der beste naechste Schritt sind:

- weiterer Read-only-Feinschliff
- fruehes Ranking
- Historienlogik
- andere groessere Kalkulations- oder Preisbloecke

## 5. Warum genau diese Richtung jetzt passt

Diese Richtung passt jetzt am besten, weil sie:

- direkt auf dem vorhandenen Preisanker aufsetzt
- weiterhin genau eine Quote-Position fokussiert
- die manuelle Verantwortung klar erhaelt
- aus Sicht des Nutzers den naechsten echten Handgriff abbildet

Damit bleibt der Pfad fachlich konsistent:

- erst Material setzen
- dann Preisanker sichtbar machen
- dann Preis explizit uebernehmen

## 6. Was bewusst noch nicht mitgezogen werden sollte

Nicht in denselben Folgeblock ziehen:

- mehrere Preisquellen gleichzeitig
- Ranking oder Scores fuer Preise
- Preis-Historien
- automatische Preisuebernahme
- Bulk-Preisuebernahmen
- Margen-, Zuschlags- oder Vollkalkulationslogik

## 7. Naechster sinnvoller Schritt

Nach dieser Folgeinventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer die enge explizite
  Preisuebernahme aus dem sichtbaren Preisanker zuschneiden
