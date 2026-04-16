# GAEB-Kleine Kandidatenauswahl: Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen read-only Materialkandidatenliste.

Der Fokus bleibt bewusst eng:

- kleinsten sinnvollen Einstiegspunkt fuer eine erste Kandidatenauswahl
  festlegen
- nur fuer bereits sichtbare Kandidaten an der Quote-Position
- noch ohne Materialsuche, Rankinglogik oder automatische Uebernahme

## 1. Ausgangslage nach read-only Kandidatenliste

Der aktuelle Stand deckt bereits ab:

- persistenter Herkunftsanker `accepted import item -> created quote item`
- optionales `material_id` an Quote-Positionen
- `price_mapping_status = open/manual`
- read-only `material_candidate_status = none/available`
- kleine read-only Materialkandidatenliste direkt an der Quote-Position

Damit ist jetzt nicht nur Kandidatenpotenzial sichtbar, sondern bereits eine
erste kleine Menge konkreter Kandidaten.

## 2. Verbleibende fachliche Luecke

Nach der read-only Kandidatenliste fehlt vor allem:

- eine kleine Handlung, um einen sichtbaren Kandidaten direkt zu uebernehmen
- ein kontrollierter Uebergang von read-only Vorschau zu manuellem Mapping
- eine enge Aktion, die vorhandene Kandidaten nutzt, ohne in Suche oder
  Matchinglogik auszuufern

Die Luecke ist damit nicht mehr Transparenz, sondern ein erster kleiner
Uebernahmeschritt.

## 3. Was bewusst noch nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- Materialsuche ausserhalb sichtbarer Kandidaten
- Ranking oder bessere Trefferlogik
- automatische Vorbelegung eines besten Kandidaten
- Bulk-Auswahl ueber ganze Quotes
- Preisvorschlaege oder Preisautomatik
- globale Mapping- oder Kandidatenkonsole

Diese Punkte waeren bereits groessere Such-, Matching- oder Automatikstufen.

## 4. Kleinster sinnvoller Einstiegspunkt

Der kleinste sinnvolle Folgepunkt ist:

- eine kleine Auswahl- bzw. Uebernahmeaktion direkt aus der bereits
  sichtbaren read-only Kandidatenliste

Der Einstiegspunkt soll nur beantworten:

- kann ein bereits sichtbarer Kandidat direkt auf `material_id`
  uebernommen werden?
- bleibt dieser Schritt eng auf die einzelne Quote-Position begrenzt?
- bleibt die Verantwortung weiter klar manuell und explizit?

## 5. Warum Auswahl vor Suche oder Ranking kommen sollte

Die kleine Kandidatenauswahl ist der bessere naechste Schritt, weil:

- sie die vorhandene Kandidatenliste operativ nutzbar macht
- sie den bestehenden manuellen Mapping-Pfad direkt erweitert
- sie mehr Signal liefert als weitere read-only Feinschliffe
- sie Suche, Ranking und Automatik weiterhin sauber entkoppelt vorbereiten
  kann

So entsteht der erste echte Nutzwert aus vorhandenen Kandidaten, ohne schon
eine groessere Matchingstufe zu versprechen.

## 6. Empfohlenes Minimalzielbild

Das Minimalzielbild fuer den naechsten Strang ist:

- direkte Auswahl eines bereits sichtbaren Kandidaten an genau einer
  Quote-Position
- Uebernahme von `material_id` aus dem gewaelten Kandidaten
- weiterhin enger manueller Statuspfad ohne Automatik

Wichtig ist:

- es geht nicht um den „besten“ Kandidaten
- es geht nicht um Kandidatensuche
- es geht nur um die kleine kontrollierte Uebernahme eines bereits
  sichtbaren Kandidaten

## 7. Warum die Quote-Position weiter der richtige Ort ist

Die Quote-Position bleibt der richtige Ort, weil dort bereits zusammenlaufen:

- Herkunftsanker
- manueller Mapping-Status
- read-only Kandidatenstatus
- konkrete read-only Kandidatenliste

Eine separate Auswahloberflaeche waere fuer diesen ersten Schritt unnoetig
gross.

## 8. Nutzen schon vor spaeterer Suche oder Automatik

Schon ohne Suche oder Ranking schafft eine kleine Kandidatenauswahl:

- schnelleren Abschluss einfacher Mapping-Faelle
- direkten operativen Nutzen aus bereits sichtbaren Kandidaten
- klareren Uebergang von Transparenz zu erster Interaktion
- bessere Vorbereitung fuer spaetere Such- und Matchingausbauten

## 9. Entscheidung

Der naechste sinnvolle Folgeausbau nach der read-only
Materialkandidatenliste ist nicht sofort Materialsuche, Kandidatenranking
oder automatische Uebernahme, sondern zuerst eine kleine
Kandidatenauswahl-Aktion direkt an der uebernommenen Quote-Position.

## 10. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer die kleine
  Kandidatenauswahl-Aktion zuschneiden, bewusst noch ohne Such-,
  Ranking- oder Automatiklogik
