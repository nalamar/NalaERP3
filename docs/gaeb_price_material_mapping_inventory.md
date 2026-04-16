# GAEB-Preis-/Material-Mapping nach vorhandenem Herkunftsanker: Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Ausbau nach dem
abgeschlossenen GAEB-Pfad aus:

- Upload
- Parse-Speicher
- Read-only-Sicht
- Positions-Review
- kontrollierter Quote-Uebernahme
- Apply-Transparenz
- Herkunfts- und Mapping-Anker

Fokus ist bewusst eng:

- den kleinsten sinnvollen Einstiegspunkt fuer Preis-/Material-Mapping
  identifizieren
- ohne sofortige Automatik oder globale Mapping-Konsole

## 1. Ausgangslage nach vorhandenem Herkunftsanker

Der aktuelle Stand deckt bereits ab:

- `accepted import item -> created quote item` ist persistent verknuepft
- die Verknuepfung ist auf Importpositions-Ebene read-only sichtbar
- die Ziel-Quote kann direkt aus Importlauf und Importposition geoeffnet
  werden

Damit ist die Rueckverfolgbarkeit vorhanden. Die naechste Luecke liegt nicht
mehr in Herkunft oder Navigation, sondern in der fachlichen Anreicherung der
erzeugten Quote-Positionen.

## 2. Was in der erzeugten Quote bewusst noch roh bleibt

Die aktuelle Quote-Uebernahme bleibt absichtlich minimal:

- `description` wird uebernommen
- `qty` wird uebernommen
- `unit` wird uebernommen
- `unit_price = 0`
- `tax_code = ''`
- kein Materialbezug

Die erzeugte Draft-Quote ist damit bearbeitbar, aber fachlich noch nicht
nahe an einer spaeteren belastbaren Kalkulations- oder Materiallogik.

## 3. Verbleibende fachliche Luecke

Nach dem Herkunftsanker fehlt aktuell vor allem:

- eine kleine kontrollierte Bruecke von Quote-Position zu spaeterem
  Materialbezug
- eine kleine kontrollierte Bruecke von Quote-Position zu spaeterem
  Preisbezug
- nachvollziehbar, ob fuer eine uebernommene GAEB-Position bereits ein
  manuell bestaetigter Mapping-Entscheid existiert

Die Luecke ist damit nicht mehr Herkunft, sondern erste fachliche
Zuordnung.

## 4. Was jetzt bewusst nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- automatische Materialsuche gegen `materials`
- automatische Preisvorschlaege
- automatische Steuercode-Ableitung
- Bulk-Mapping ueber ganze Importlaeufe
- KI-Vorschlaege fuer passende Materialien oder Preise
- Rueckschreiben neuer Mapping-Daten in GAEB-Rohpositionen

Diese Themen sind fachlich sinnvoll, aber als naechster Schritt zu gross und
zu fehleranfaellig.

## 5. Kleinster sinnvoller Einstiegspunkt

Der kleinste sinnvolle Folgepunkt ist:

- zuerst einen kleinen manuellen Mapping-Anker auf Ebene der bereits
  erzeugten Quote-Position schaffen

Konkret bedeutet das:

- eine uebernommene Quote-Position kann spaeter genau einen optionalen
  Materialbezug und/oder einen kleinen Preisursprungsanker erhalten
- diese Zuordnung wird bewusst manuell bestaetigt und nicht automatisch
  vorgeschlagen

## 6. Warum der Einstieg an der Quote-Position liegen sollte

Nicht die Importposition, sondern die erzeugte Quote-Position ist jetzt der
richtige Einstiegspunkt, weil:

- dort die bearbeitbare Angebotsrealitaet liegt
- dort Preise spaeter ohnehin gepflegt oder bestaetigt werden
- der Herkunftsanker bereits sauber von Importposition zur Quote-Position
  fuehrt

Damit wird keine zweite konkurrierende Mapping-Welt auf Import-Ebene
erzeugt.

## 7. Empfohlenes Minimalziel fuer den naechsten Strang

Der naechste Mapping-Strang sollte zunaechst nicht Automatik bauen, sondern
nur:

- einen kleinen manuellen Zuordnungsanker fuer erzeugte Quote-Positionen
- klar getrennt nach spaeterem Materialbezug und spaeterem Preisbezug
- rueckverfolgbar ueber den bereits vorhandenen Herkunftsanker

Ein sinnvoller Minimalzuschnitt waere:

- optionaler Material-Link an der uebernommenen Quote-Position
- optionaler kleiner Herkunftshinweis, dass Preis noch manuell/offen ist

## 8. Warum dieser Einstieg den besten Signalwert hat

- er nutzt den bereits vorhandenen Herkunftsanker direkt weiter
- er verbessert die fachliche Nutzbarkeit der erzeugten Draft-Quote sofort
- er vermeidet falsche Sicherheit durch fruehe Automatik
- er schafft spaeter eine belastbare Basis fuer Vorschlags- oder
  Automatiklogik

## 9. Entscheidung

Der naechste sinnvolle GAEB-Folgeabschnitt nach dem Herkunftsanker ist nicht
sofort automatische Preis-, Steuer- oder Materiallogik, sondern zuerst ein
kleiner manueller Mapping-Einstieg auf Ebene der erzeugten Quote-Position.

## 10. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer diesen ersten manuellen
  Preis-/Material-Mapping-Einstieg vorbereiten, bewusst noch ohne
  Vorschlags- oder Automatiklogik
