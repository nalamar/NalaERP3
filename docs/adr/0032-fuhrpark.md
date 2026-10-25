# ADR 0032 — Fuhrpark: Fahrzeuge, Termine, Fahrer, Fahrtenbuch

Datum: 2026-10-25
Status: **Entwurf — wartet auf Entscheidungen des Nutzers**
(`docs/open-questions.md`, 2026-10-25). Kein Produktivcode vor der Antwort.
Bezug: `aufgabe.md` §1 Domäne G ("Fahrzeuge, Termine (HU/AU, UVV,
Wartung), Fahrtenbuch, Kosten, Führerscheinkontrolle"); Backlog G.1–G.3;
ADR 0027 (Qualifikationen, Führerscheinkontrolle dorthin verwiesen),
ADR 0028 (Betriebsmittel, Fahrzeuge ausdrücklich ausgenommen), ADR 0031
(Stellen).

## Kontext (Ist-Stand, belegt)

- **Kein Bestand.** `docs/01-gap-analysis.md:102`: "Kein Package, keine
  Migration, keine Client-Seite mit Fahrzeug-/Fuhrpark-Bezug". Einzige
  Spur ist die Rolle `role-fleet` (`017_auth.sql:72`, "Fahrzeuge und
  mobile Assets"); sie hat bisher nur die `*.self`-Rechte der HR-
  Migrationen (086–089), keine eigenen.
- **Betriebsmittel (F.3, ADR 0028)** führen Einzelstücke mit
  Inventarnummer, Ausgabe an Mitarbeiter (unveränderlich, Rückgabe und
  Storno), Prüfungen mit gespeichertem Folgetermin, Sperre bei
  "Mängel"/"nicht bestanden", Fälligkeitsliste mit 30 Tagen Vorwarnzeit.
  `089_assets.sql:22`: "Fahrzeuge gehoeren zum Fuhrpark (Epic G) und
  fehlen bewusst"; die Kategorie `fahrzeug` wird abgelehnt
  (`assets_test.go:40`). Stapler und Hubarbeitsbühnen passen dort unter
  `maschine`.
- **Qualifikationen (F.2, ADR 0027)** führen unveränderliche Nachweise
  mit gespeichertem `gueltig_bis`, Fälligkeit, Nachweisdokumente
  (F.10.1), Anforderungen je Mitarbeiter und über **Stellen** (F.11).
  ADR 0027 hat die Führerscheinkontrolle an Epic G verwiesen.

## Recherche (Sekundärquellen, nicht am Normtext geprüft)

| Thema | Befund | Folge für das Modell |
|---|---|---|
| Hauptuntersuchung (HU, mit AU) | Pflicht des Halters (§29 StVZO, Anlage VIII); fällig im Monat der Plakette; Pkw erste HU nach 36, dann alle 24 Monate; Fahrgastbeförderung 12; schwere Nutzfahrzeuge 12 oder kürzer; bei Anhängern widersprechen sich die Quellen | Folgetermin **als Monat** und **angegeben** speichern, nicht aus einer Fahrzeugklasse berechnen (Anlage VIII ist zu verzweigt); Vorschlag 24 Monate |
| UVV-Prüfung | dienstlich genutzte Fahrzeuge mindestens jährlich durch einen Sachkundigen (DGUV Vorschrift 70 §57); Ergebnis schriftlich bis zur nächsten Prüfung aufbewahren; ob HU/Inspektion sie bei Pkw ersetzt, sagen die Quellen uneinheitlich | eigener Termin, Vorschlag 12 Monate, Nachweisdokument möglich |
| Führerscheinkontrolle | Halterhaftung (§21 StVG); **keine gesetzliche Frequenz, keine höchstrichterliche Rechtsprechung**; Praxis halbjährlich; ein Gesetzentwurf des Bundesrats (BT-Drs. 20/14039, 2024) will einmalige Vorlage ohne Anlass genügen lassen — Stand unbekannt; DSGVO gilt, Altdaten bei erneuter Prüfung löschen | Kontrolle als Feststellung mit Datum, **keine Kopie des Führerscheins**; Frist 6 Monate als änderbarer Vorschlag |
| Fahrtenbuch | steuerlich nur anerkannt in **geschlossener Form**: nachträgliche Änderungen technisch ausgeschlossen oder in der Datei dokumentiert und erkennbar (BFH VI B 37/23 vom 12.01.2024; VI R 64/04); zeitnah zu führen | Einträge unveränderlich, Korrektur nur als sichtbares Storno mit Neueintrag; Erfassungszeitpunkt speichern; lückenloser Kilometerstand |

Quellen: [§29 StVZO, bussgeldkatalog.net](https://www.bussgeldkatalog.net/strassenverkehrszulassungsordnung/29-stvzo/),
[Anlage VIII StVZO](https://www.gesetze-im-internet.de/stvzo_2012/anlage_viii.html),
[Landtag NRW MMD17-17086](https://www.landtag.nrw.de/portal/WWW/dokumentenarchiv/Dokument/MMD17-17086.pdf),
[LapID Merkblatt Fahrzeugprüfung](https://info.lapid.de/hubfs/Informationsmaterial/Merkbl%C3%A4tter/Merkblatt_Fahrzeugpr%C3%BCfung.pdf),
[KomNet 42730](https://www.komnet.nrw.de/_sitetools/dialog/42730),
[KomNet 9761](https://www.komnet.nrw.de/_sitetools/dialog/9761),
[VDR-Position Dienstwagen](https://www.vdr-service.de/fileadmin/der-verband/politische-arbeit/vdr-positionen/2019-10_VDR-Position_Unfallverhuetungsvorschrift_Ausnahmen-fuer-Dienstwagen.pdf),
[Handwerksblatt Führerscheinkontrolle](https://www.handwerksblatt.de/betriebsfuehrung/fuehrerscheinkontrolle-durch-arbeitgeber-eine-ueberpruefung-soll-reichen),
[BT-Drs. 20/14039](https://dserver.bundestag.de/btd/20/140/2014039.pdf),
[BayObLG 203 StRR 420/22](https://www.gesetze-bayern.de/Content/Rtf/Y-300-Z-BECKRS-B-2022-N-46007?all=False),
[Brandi Newsletter 10/2024](https://www.brandi.net/fileadmin/redakteure/Newsletter/pdf/2024/Newsletter_1024.pdf),
[Forvis Mazars: elektronisches Fahrtenbuch](https://www.forvismazars.com/de/en/who-we-are/news/press-media/newsletters/newsletter-steuern/newsletter-steuern-1-2024/anforderungen-an-ein-elektronisches-fahrtenbuch),
[VLH: nachträgliche Eintragungen](https://www.vlh.de/presse/pressemitteilungen/elektronisches-fahrtenbuch-ungueltig-wegen-nachtraeglicher-eintragungen.html),
[ECOVIS: elektronisches Fahrtenbuch](https://ecovis-kso.com/blog/elektronisches-fahrtenbuch-ordnungsgemaess-fuehren/).

## Zu entscheiden

### 1. Was gehört zum Fuhrpark?

**Empfehlung**: zulassungspflichtige **Fahrzeuge und Anhänger** (Pkw,
Transporter, Lkw, Anhänger). **Stapler, Hubarbeitsbühnen und andere
Maschinen ohne Zulassung bleiben Betriebsmittel** (F.3, Kategorie
`maschine`, dort schon mit Prüfintervall und Ausgabe). Sonst gäbe es zwei
Orte für dieselbe Art von Gerät.

### 2. Eigene Entität oder Betriebsmittel?

- **A — Betriebsmittel um Kategorie `fahrzeug` erweitern.** Spart eine
  Tabelle, aber Kennzeichen, Fahrzeug-Identnummer, Erstzulassung,
  zulässige Gesamtmasse, Kilometerstand, HU nach Monat, Fahrtenbuch und
  Fahrerzuordnung passen nicht in `assets`; jede Abfrage der
  Betriebsmittel müsste Fahrzeuge ausnehmen.
- **B — eigene Entität `vehicles` in einem neuen Paket `internal/fleet`**
  (empfohlen), mit denselben Mustern wie F.3: Mandant, kein Löschen
  (Abmelden/Ausmustern mit Datum und Grund), Status abgeleitet statt
  gespeichert, Kostenstelle (E.1) optional.

### 3. Termine (G.2)

Je Fahrzeug Termine der Arten **HU**, **UVV**, **Wartung/Inspektion**,
dazu Fristen wie **Versicherung** und **Leasingende**. Muster wie die
Prüfungen der Betriebsmittel: ein Termin wird **erledigt** (Datum,
Ergebnis, Prüfer/Werkstatt, Notiz, Dokument), unveränderlich, Korrektur
über Storno; der **Folgetermin wird angegeben und gespeichert**
(Vorschlag: HU 24 Monate als Monat, UVV 12 Monate, Wartung je Fahrzeug);
Fälligkeit beim Lesen berechnet, Vorwarnzeit 30 Tage wie F.3,
betriebsweite Liste. Wartung zusätzlich nach Kilometern (Folge-km).

### 4. Fahrerzuordnung

Dienstwagen mit festem Fahrer oder Pool. Zuordnung wie die Ausgabe der
Betriebsmittel: Fahrzeug, Mitarbeiter, ab, bis (offen, solange er es
hat), unveränderlich bis auf Rückgabe und Storno; höchstens eine offene
Zuordnung je Fahrzeug in der Datenbank. Pool-Fahrzeuge haben keine
Zuordnung; ihre Nutzung zeigt das Fahrtenbuch.

**Sperren oder Hinweis** (→ Frage 3): wie bei den Betriebsmitteln
empfohlen — eine UVV/HU mit Ergebnis "Mängel"/"nicht bestanden" sperrt
die Zuordnung, ein **überfälliger Termin** und eine **nicht gültige
Führerscheinkontrolle des Fahrers** sind ein deutlicher Hinweis, keine
Sperre.

### 5. Führerscheinkontrolle (G.3)

**Empfehlung: über die Qualifikationen aus F.2 statt eigener Tabellen.**
Eine Kontrolle ist eine Feststellung "Fahrerlaubnis am TT.MM.JJJJ
eingesehen, Klassen B, BE, C1" — genau ein Nachweis: unveränderlich,
mit gespeichertem `gueltig_bis` (Frist 6 Monate als änderbarer
Vorschlag), Storno, Fälligkeitsliste, Anmahnen, optional Dokument; die
Klassen stehen in der Nachweis-Nr./Notiz. Wer Firmenfahrzeuge fährt,
bekommt die Stelle "Fahrer" (F.11), die die Kontrolle verlangt — damit
steht sie für jeden Fahrer automatisch im Stand. Dazu eine neue Art im
Vorschlagskatalog (`FUEHRERSCHEINKONTROLLE`, Kategorie `beauftragung`
o. ä.) und eine Stelle `FAHRER`. **Kein Speichern einer
Führerscheinkopie** (Datenminimierung); ein hochgeladenes Dokument ist
möglich, aber nicht vorgesehen.

### 6. Fahrtenbuch (G.3)

Optional je Fahrzeug ("Fahrtenbuch führen"). Ein Eintrag: Datum,
Kilometerstand Beginn/Ende, Fahrtart (dienstlich, privat, Arbeitsweg),
Ziel, Zweck, Geschäftspartner bei Dienstfahrten, Fahrer. **Geschlossene
Form**: Einträge unveränderlich, Korrektur nur als Storno mit Begründung
und Neueintrag (beides sichtbar); Erfassungszeitpunkt gespeichert;
**lückenloser Kilometerstand** (Beginn = Ende des vorigen Eintrags, sonst
Ablehnung mit Hinweis). Bewegungsdaten sind personenbezogen: lesen nur
Fuhrpark und der Fahrer selbst.

### 7. Kosten

Kosten je Fahrzeug (Kraftstoff, Reparatur, Versicherung, Steuer,
Leasing) mit Kostenstelle; Anbindung an Eingangsrechnungen (Epic E)
später. Nicht Teil von G.1/G.2.

### 8. Rechte

Neue Permissions `fleet.self` (eigene Zuordnungen und Fahrten; alle
Systemrollen), `fleet.read`, `fleet.write`. **Empfehlung:** `read` und
`write` an `role-fleet` und `role-admin`. Die Führerscheinkontrolle
erfasst, wer `qualifications.write` hat (bisher Personalabteilung) —
siehe Frage 2.

## Folgen bei Entscheidung wie empfohlen (Vorschau, nicht beschlossen)

G.1 Fahrzeugstamm (Migration, Service, Routen, Client) · G.2 Termine und
Fälligkeit · G.3a Fahrerzuordnung · G.3b Führerscheinkontrolle über
Qualifikationen (Vorschlagskatalog-Art + Stelle) · G.3c Fahrtenbuch ·
G.3d Kosten. Jeweils mit eigener Teilung im Backlog.
