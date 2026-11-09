# ADR 0038 — Produktionssteuerung: Entnahme, Rückmeldung, Montageplanung

Datum: 2026-11-09
Status: **Angenommen** (2026-11-09, Antwort des Nutzers): 1 (a), 2 (a),
3 (a), 4 (a), **5 (a), (b) vorbereiten**, **6 (a), (b) vorbereiten**,
**7 (a), (b) als mögliches Feature vermerken** — Umsetzung im Abschnitt
"Entscheidung" unten. Die Auskunftsfragen (Logikal MES, Betriebsrat)
sind unbeantwortet; mit 5 (a) ist das ERP der Ort der Rückmeldung.
Fragen in `docs/open-questions.md`, Abschnitt 2026-11-09 H.3.
Bezug: Backlog H.3 "BDE, Kommissionierung, Montageplanung"; `aufgabe.md`
§1 Domäne H (dort zusätzlich "Zuschnitt/Optimierung"); ADR 0034
(Fertigungsaufträge, Reservierung; Präzisierung EN 1090: keine
Chargenpflicht bei der Entnahme); ADR 0037 (Arbeitsgänge, Kapazitäten;
Rückmeldung je Arbeitsgang ausdrücklich H.3); ADR 0024 (Zeiterfassung);
ADR 0032 (Fuhrpark).

## Kontext (Ist-Stand, belegt)

- **Reservierung ja, Entnahme nein.** Die Freigabe reserviert Lagermaterial
  je Stücklistenzeile (`stock_reservations`, Status `aktiv`/`freigegeben`,
  Migration 075/108; Profilstücke auf Stangen und Glas werden nicht
  reserviert, ADR 0034). Storno gibt die Reservierungen frei
  (`production/orders.go`, `Cancel`). **"Fertig" ändert nur den Status**
  (`Finish` → `step`): die Reservierungen bleiben `aktiv`, es gibt keinen
  Lagerabgang. Folge heute: nach jedem fertigen Auftrag ist der Bestand zu
  hoch und das Material bleibt für andere Aufträge gesperrt — bis jemand
  von Hand bucht und die Reservierung freigibt.
- **Lagerbewegungen** gibt es (`stock_movements`: Menge mit Vorzeichen,
  Lager, Platz, Charge, `movement_type`, `reason`, `reference`;
  `materials.CreateMovementTx` bucht in einer offenen Transaktion, seit
  H.4.z1). Reststücke von Profilen: anlegen und verbrauchen mit genutzter
  Länge (Migration 077, `ConsumeOffcut`). Eine Buchung "Entnahme für
  Fertigungsauftrag" mit Bezug zur Stücklistenzeile gibt es nicht.
- **Arbeitsgänge** (H.2): Planung mit Woche und Planzeit, **ohne Status
  und ohne Ist-Zeit**. Die Auslastung zählt alle Arbeitsgänge offener
  Aufträge, auch längst erledigte.
- **Zeiterfassung** (`hr_time_entries`, Migration 086): je Mitarbeiter und
  Tag, mit Projektbezug, **ohne Bezug zu Fertigungsauftrag oder
  Arbeitsgang**.
- **Montage**: nichts. Projekte haben weder Baustellenadresse noch
  Montagetermine (`projects`: Nummer, Name, Kunde, Status, Kostenstelle).
  Es gibt Teams (`hr_teams`), Abwesenheiten, Fahrzeuge mit Sperre nach
  HU/UVV (ADR 0032) und den Betriebskalender (ADR 0029).
- **Zuschnitt/Optimierung** steht in `aufgabe.md`, aber in keinem
  Backlog-Punkt. Logikal optimiert den Stangenzuschnitt selbst; das ERP
  kennt Reststücke (077).

## Recherche (Sekundärquellen)

| Thema | Befund | Folge |
|---|---|---|
| Logikal MES | Bringt die in Logikal vorbereiteten Aufträge an die Arbeitsplätze, zeigt den Status aller Arbeitsschritte und Störmeldungen, Soll-Ist-Vergleich der Zeiten, WPK-Prüfungen | Wer Logikal MES nutzt, meldet Arbeitsgänge dort zurück; das ERP bräuchte dann höchstens "Auftrag fertig" — Frage an den Nutzer (offen seit H.2) |
| Metallbau-ERP allgemein | Werkstattfertigung (planbar) und Montage (flexibel) werden getrennt geplant; Plantafeln sind üblich, die Stückliste steuert Einkauf und Werkstatt | Montageplanung eigenständig neben der Werkstattplanung aus H.2, nicht als weiterer Arbeitsgang |
| Rückmeldung allgemein | Klassisch: Menge und Zeit am Ende des Arbeitsgangs, oft von Hand und verzögert; MES: in Echtzeit am Terminal | Für einen kleinen Betrieb ohne Terminal genügt "erledigt" mit optionaler Ist-Zeit von Hand |

Quellen: [Logikal MES (Orgadata/Forterro)](https://orgadata.com/global/en/solutions/logikal-mes.html),
[Logikal MES, Hilfe](https://help.orgadata.com/en/?p=8003),
[OMR: ERP im Metallbau](https://omr.com/de/reviews/contenthub/erp-metallbau),
[Symestic: BDE](https://www.symestic.com/de-de/bde).

Rechtlicher Rahmen (ohne Quelle, Allgemeinwissen, kein Rechtsrat): Ist-Zeiten
**je Person und Arbeitsgang** sind zur Leistungs- und Verhaltenskontrolle
geeignet — mitbestimmungspflichtig, wo ein Betriebsrat besteht (§ 87 Abs. 1
Nr. 6 BetrVG), und nach DSGVO nur mit klarem Zweck und Löschfrist. Ist-Zeiten
**je Arbeitsgang ohne Person** berühren das nicht.

## Zu entscheiden

### 1. Reihenfolge und Zuschnitt von H.3

(a) **Drei Teile nacheinander**: zuerst Kommissionierung/Entnahme (schließt
die Lücke beim Bestand), dann Rückmeldung je Arbeitsgang, dann
Montageplanung; jeder Teil mit eigener Migration und eigenem Client.
(b) Montageplanung zuerst. (c) Nur Entnahme; Rückmeldung und Montage als
eigene Epics später.
**Empfehlung (a)** — die Entnahme behebt einen echten Fehler im Bestand,
die anderen beiden bauen auf Daten auf, die es schon gibt.

### 2. Entnahme

(a) **Entnahme gegen die Stückliste**: je Zeile mit Lagermaterial eine
Menge entnehmen (vorbelegt mit der offenen Menge), aus dem Lager der
Reservierung oder einem anderen; bucht einen Lagerabgang mit Bezug zur
Zeile und Auftragsnummer als Referenz und verringert bzw. verbraucht die
Reservierung. Teilentnahmen und Mehrentnahme erlaubt (Hinweis), Fehler
über Storno der Entnahme (Gegenbuchung), keine Chargenpflicht (ADR 0034).
Zusätzlich Material ohne Stücklistenzeile entnehmen (Kleinteile).
(b) Retrograd: bei "fertig" automatisch alles laut Stückliste abbuchen.
(c) Keine Lagerbuchung, nur eine Liste zum Abhaken.
**Empfehlung (a)** — bildet ab, was tatsächlich das Lager verlässt;
(b) bucht bei Verschnitt und Ersatzmaterial falsch, (c) lässt den Bestand
falsch.

### 3. Was "fertig" mit offenen Reservierungen tut

(a) **Freigeben, mit Hinweis** "n Zeilen nicht oder nur teilweise
entnommen" vor dem Fertigmelden (keine Sperre). (b) Den Rest retrograd
abbuchen. (c) Stehen lassen (Ist-Stand).
**Empfehlung (a)** — ein fertiger Auftrag braucht kein reserviertes
Material mehr; was wirklich verbraucht wurde, steht in den Entnahmen.

### 4. Kommissionierliste

(a) **PDF je Auftrag** (Lager, Platz, Material, Menge bzw. Profil mit
Länge, Glas mit Maßen; ohne Preise) mit dem vorhandenen PDF-Baustein, dazu
die Entnahme am Bildschirm. (b) Nur Bildschirm.
**Empfehlung (a)** — in der Werkstatt wird auf Papier kommissioniert; der
PDF-Baustein ist vorhanden.

### 5. Rückmeldung je Arbeitsgang

(a) **"Erledigt" je Arbeitsgang** (Zeitpunkt, wer gemeldet hat) mit
**optionaler Ist-Zeit in Minuten ohne Personenbezug**; zurücknehmen mit
Protokoll; erledigte Arbeitsgänge zählen nicht mehr zur Auslastung;
Soll/Ist je Auftrag. (b) Zeiterfassung erweitern: Zeiteinträge mit Bezug
zu Auftrag und Arbeitsgang, Ist-Zeit = Summe (Stempeln je Person).
(c) Keine Rückmeldung im ERP (Logikal MES).
**Empfehlung (a)** — macht die Auslastung richtig und zeigt Soll/Ist,
ohne Leistungskontrolle je Person (BetrVG, DSGVO); (b) erst, wenn
Nachkalkulation je Person gewollt und mit dem Betriebsrat geklärt ist;
(c), falls Logikal MES genutzt wird (Zusatzfrage).

### 6. Montageplanung

(a) **Montagetermin je Fertigungsauftrag**: Zeitraum (von–bis,
Arbeitstage), Team (`hr_teams`), optional Fahrzeug, Baustelle als
Freitext, Notiz; Wochenübersicht der Termine je Team; Hinweise ohne
Sperre: Team doppelt belegt, Abwesenheit im Team, Fahrzeug gesperrt,
Montage vor "fertig" oder nach dem Liefertermin. (b) Termin am Projekt
statt am Auftrag. (c) Nur ein Datum ohne Team und Hinweise.
**Empfehlung (a)** — der Auftrag ist die Einheit, die gefertigt und
eingebaut wird, und hat schon Liefertermin und Status; mehrere Termine je
Auftrag erlaubt (Montage in Abschnitten). (b), falls Montage
auftragsübergreifend geplant wird.

### 7. Zuschnitt/Optimierung

(a) **Bleibt in Logikal**; im ERP nur Reststücke (vorhanden) und deren
Verbrauch bei der Entnahme. (b) Eigene Stangenoptimierung im ERP.
**Empfehlung (a)** — Logikal optimiert bereits; eine zweite Optimierung
liefert abweichende Ergebnisse.

### Zusatzfragen (Auskunft)

- **Nutzt der Betrieb Logikal MES** oder plant es? Dann Frage 5 (c).
- **Gibt es einen Betriebsrat?** Nur für Frage 5 (b) erheblich.

## Entscheidung (2026-11-09)

- **Entnahme** wie 2 (a) und 3 (a), Kommissionierliste wie 4 (a).
  Reststücke: Verbrauch über die vorhandene Funktion (`ConsumeOffcut`)
  mit der Auftragsnummer als Referenz, keine eigene Spalte.
- **Rückmeldung 5 (a), (b) vorbereitet**: Rückmeldungen werden als
  eigene Datensätze je Arbeitsgang gespeichert (nicht als Spalten am
  Arbeitsgang): erledigt, optionale Ist-Minuten, wer gemeldet hat
  (Benutzer, nicht Mitarbeiter), Rücknahme mit Protokoll. Für (b)
  kommt später **nur** ein nullbarer Bezug der Zeiterfassung zum
  Arbeitsgang hinzu (`hr_time_entries` → Arbeitsgang); die Ist-Zeit
  ist dann wahlweise die Summe der Zeiteinträge. Bis BetrVG und DSGVO
  geklärt sind (Zweck, Löschfrist, ggf. Betriebsvereinbarung), keine
  Ist-Zeit mit Personenbezug — (b) wird ein eigener Backlog-Punkt mit
  dieser Klärung als Voraussetzung.
- **Montage 6 (a), (b) vorbereitet**: der Montagetermin hängt in der
  Datenbank am **Projekt** (Pflicht) mit **optionalem**
  Fertigungsauftrag; in (a) verlangt der Service den Auftrag und
  übernimmt dessen Projekt. Für (b) entfällt nur diese Pflicht im
  Service (Termine für ein Projekt oder mehrere Aufträge eines
  Projekts), ohne Migration.
- **Zuschnitt 7 (a)**; eigene Stangenoptimierung im ERP als mögliches
  Feature im Backlog vermerkt (H.6, nicht geplant).

## Folgen

- Migration 113: Entnahmen (`production_order_withdrawals`: Auftrag,
  Zeile oder Material, Lager, Menge, Lagerbewegung, Storno) und
  Reservierungsstatus `verbraucht`; Datenverlustrisiko je Migration.
- Teilung: H.3.1 Migration und Regeln Entnahme, H.3.2 Service und Routen
  (inkl. "fertig" gibt frei), H.3.3 Kommissionierliste (PDF), H.3.4 Client
  Entnahme; H.3.5 Rückmeldung je Arbeitsgang (Migration 114, Service,
  Routen, Client, Auslastung ohne erledigte); H.3.6 Montageplanung
  (Migration 115, Service, Routen, Hinweise, Client). Jeder Teil wird
  vor Beginn weiter geteilt, wenn er mehr als 8 Dateien oder 400 Zeilen
  braucht.
- Rechte: vorhandene `production.read/write`; Montageplanung ebenfalls
  (kein eigenes Recht, solange niemand es verlangt).
