# GAEB-Vorschlags- und Automatik-Zielstrecke nach manuellem Mapping: Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Ausbau nach dem
abgeschlossenen manuellen Preis-/Material-Mapping-Einstieg.

Fokus ist bewusst eng:

- den kleinsten sinnvollen Einstiegspunkt fuer spaetere Vorschlags- oder
  Automatiklogik identifizieren
- ohne sofortige Such-, Matching- oder Preisautomatik
- auf Basis des bereits vorhandenen Herkunfts- und Mapping-Ankers

## 1. Ausgangslage nach manuellem Mapping-Einstieg

Der aktuelle Stand deckt bereits ab:

- `accepted import item -> created quote item` ist persistent verknuepft
- die Herkunft ist auf Importpositions-Ebene sichtbar
- uebernommene Quote-Positionen besitzen optionales `material_id`
- Quote-Positionen besitzen einen kleinen `price_mapping_status`
  (`open/manual`)
- die Mapping-Daten sind direkt im bestehenden Quote-Editor pflegbar

Damit ist jetzt nicht mehr die manuelle Grundfaehigkeit die Luecke, sondern
die Frage, wie spaeter sinnvolle Vorschlaege oder Automatik darauf aufsetzen.

## 2. Was aktuell noch vollstaendig manuell bleibt

Die vorhandene Loesung bleibt absichtlich roh:

- `material_id` muss direkt eingegeben werden
- es gibt keine Materialsuche
- es gibt keinen Materialvorschlag aus Beschreibung oder Herkunft
- `unit_price` bleibt fachlich unabhaengig vom Material
- es gibt keinen Preisvorschlag aus Material oder Historie
- `price_mapping_status` wird nicht automatisch abgeleitet

Die Quote bleibt damit bearbeitbar, aber die operative Hilfe fuer
wiederkehrende aehnliche Positionen fehlt noch.

## 3. Verbleibende fachliche Luecke

Nach dem manuellen Mapping-Einstieg fehlt vor allem:

- eine kleine fachliche Bruecke von GAEB-Herkunftsdaten zu moeglichen
  Materialkandidaten
- eine kleine fachliche Bruecke von manuell bestaetigtem Material zu
  spaeteren Preisvorschlaegen
- nachvollziehbar, welche Quote-Positionen fuer Vorschlaege ueberhaupt als
  geeignet gelten

Die Luecke ist damit nicht mehr Datenerfassung, sondern erste assistierte
Unterstuetzung.

## 4. Was jetzt bewusst nicht der naechste Schritt ist

Nicht Teil des naechsten Blocks:

- vollautomatisches Materialmatching
- automatische Preisuebernahme in grossem Stil
- KI-gestuetztes Freitext-Matching
- Bulk-Vorschlaege ueber ganze Importlaeufe
- globale Mapping- oder Vorschlagskonsole
- automatische Rueckschreibung in Importdaten

Diese Themen sind fachlich attraktiv, aber als naechster Schritt zu gross
und zu fehleranfaellig.

## 5. Kleinster sinnvoller Einstiegspunkt

Der kleinste sinnvolle Folgepunkt ist:

- zuerst eine kleine Vorschlags-Transparenz vorbereiten, nicht sofort eine
  schreibende Automatik

Konkret bedeutet das:

- sichtbarer Hinweis, ob fuer eine uebernommene Quote-Position spaeter
  ueberhaupt ein Material- oder Preisvorschlag denkbar waere
- fachlich enger Zuschnitt auf einen kleinen Kandidatenanker statt auf
  automatisches Uebernehmen

## 6. Warum der Einstieg nicht direkt automatische Zuordnung sein sollte

Direkte Automatik waere jetzt zu frueh, weil:

- noch keine belastbare Such- oder Kandidatenbasis sichtbar ist
- falsche Zuordnungen bei Material und Preis unmittelbar kommerzielles Risiko
  tragen
- der neue manuelle Mapping-Pfad noch keinen Vorschlags-Feedback-Kreislauf
  besitzt

Der naechste Schritt sollte deshalb zuerst Signal und Nachvollziehbarkeit
erzeugen, nicht sofort Automatik.

## 7. Empfohlenes Minimalziel fuer den naechsten Strang

Der naechste Vorschlags-/Automatik-Strang sollte zunaechst nicht selbst
automatisch mappen, sondern nur:

- den kleinsten fachlichen Kandidatenpunkt fuer spaetere Vorschlaege
  inventarisieren
- sauber trennen zwischen:
  - moeglichem Materialvorschlag
  - moeglichem Preisvorschlag
  - spaeterer automatischer Uebernahme

Der beste erste Fokus liegt eher bei Materialkandidaten als sofort bei
Preisautomatik, weil:

- Material der stabilere strukturelle Anker ist
- Preise haeufig erst sinnvoll nach Materialbezug oder Kalkulationslogik
  vorgeschlagen werden koennen

## 8. Warum dieser Einstieg den besten Signalwert hat

- er nutzt den vorhandenen Herkunftsanker und das manuelle Mapping direkt
  weiter
- er reduziert spaeteres Fehlerrisiko durch schrittweisen Ausbau
- er verhindert, dass Preisautomatik ohne sauberen Materialbezug entsteht
- er schafft eine belastbare Vorstufe fuer spaetere Suche, Vorschlaege oder
  KI-Matching

## 9. Entscheidung

Der naechste sinnvolle Folgeabschnitt nach dem manuellen Mapping-Einstieg ist
nicht sofort automatische Preis- oder Materialzuordnung, sondern zuerst eine
fachlich saubere Inventur des kleinsten Vorschlagsankers, bevorzugt entlang
spaeterer Materialkandidaten.

## 10. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Schritt:

- ein minimales technisches Zielmodell fuer den ersten kleinen
  Materialvorschlags- oder Kandidatenanker vorbereiten, bewusst noch ohne
  automatische Uebernahme
