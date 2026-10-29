# ADR 0032 — Fuhrpark: Fahrzeuge, Termine, Fahrer, Fahrtenbuch

Datum: 2026-10-25
Status: **entschieden** — alle fünf Fragen vom Nutzer am 2026-10-25
ausdrücklich wie empfohlen beantwortet (siehe "Entscheidungen des
Nutzers").
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

## Entscheidungen des Nutzers (2026-10-25)

1. Umfang: "Nur Fahrzeuge und Anhänger" — Maschinen ohne Zulassung
   bleiben Betriebsmittel (F.3).
2. Führerscheinkontrolle über die Qualifikationen, Frist 6 Monate als
   Vorschlag, Stelle "Fahrer", keine Kopie, Erfassung durch die
   Personalabteilung: "Ja genau so".
3. Sperre bei UVV/HU "Mängel"/"nicht bestanden", sonst Hinweis: "Genau
   so".
4. Fahrtenbuch optional je Fahrzeug, geschlossene Form, Fahrer erfassen
   selbst, lesen nur Fuhrpark und Fahrer: "Ja".
5. `fleet.read`/`fleet.write` an Fuhrpark und Administrator, nicht an die
   Personalabteilung: "Genau so".

## Umsetzung (Teilung im Backlog)

G.1.2 Migration und Service Fahrzeugstamm · G.1.3 HTTP-Routen · G.1.4
Client · G.2 Termine (HU, UVV, Wartung, Fristen) und Fälligkeit · G.3a
Fahrerzuordnung · G.3b Führerscheinkontrolle über Qualifikationen · G.3c
Fahrtenbuch · G.3d Kosten.

Präzisierungen G.1.2: Fahrzeugarten `pkw`, `transporter`, `lkw`,
`anhaenger`. Kennzeichen in Großbuchstaben mit einfachen Leerzeichen
gespeichert, eindeutig je Mandant **unter den nicht abgemeldeten**
Fahrzeugen (ein Kennzeichen kann nach Abmeldung neu vergeben werden).
Fahrzeug-Identnummer optional, 17 Zeichen nach ISO 3779 (A-Z ohne I, O,
Q; 0-9), eindeutig je Mandant. Abmelden mit Datum und Grund, endgültig;
gelöscht wird nichts.

Präzisierung G.2 (2026-10-26, technische Ausgestaltung von §3, keine
neue fachliche Entscheidung): wer ein Fahrzeug neu erfasst, kennt meist
nur den nächsten Termin von der Plakette, nicht die letzte HU. Deshalb
zwei Dinge statt nur "Erledigungen":

- **Termin** (Planung, änderbar): je Fahrzeug und Art (HU, UVV, Wartung,
  Versicherung, Leasingende, Sonstiges mit Bezeichnung) mit
  **Starttermin**, Start-km, Intervall in Monaten bzw. km und Notiz;
  stilllegbar.
- **Erledigung** (Beleg, unveränderlich, Storno): Datum, Ergebnis (HU/UVV:
  bestanden, Mängel, nicht bestanden; sonst "erledigt"), km-Stand,
  Prüfstelle/Werkstatt, **gespeicherter Folgetermin** (Vorschlag aus dem
  Intervall) und Folge-km.
- Fällig ist der Folgetermin der jüngsten nicht stornierten Erledigung,
  sonst der Starttermin. **HU immer zum Monatsende** (Plakettenmonat).
  Sperre bei jüngster HU/UVV mit Mängeln oder nicht bestanden bis zur
  nächsten bestandenen; der Folgetermin (Nachprüfung) bleibt sichtbar.
  Fälligkeit nach km kommt erst mit dem Fahrtenbuch (G.3c), weil vorher
  kein aktueller Kilometerstand bekannt ist.
- Vorschläge für das Intervall: HU 24, UVV 12, Versicherung 12 Monate.

Präzisierung G.3b (2026-10-26, technische Ausgestaltung von §5): die Art
`FUEHRERSCHEINKONTROLLE` hat die Kategorie **`qualifikation`**, nicht
`beauftragung` — eine Beauftragung gilt im Katalog bis Widerruf (ohne
Frist), eine Unterweisung ist die Kontrolle nicht. Frist 6 Monate als
Vorschlag; der Hinweis der Art sagt "nur Datum und Klassen, keine
Kopie". Die Stelle `FAHRER` verlangt nur diese Art.

Nachtrag G.2.2.3 (2026-10-26): nach einer HU oder UVV mit **Mängeln oder
nicht bestanden** schlägt der Service **keinen** Folgetermin aus dem
Intervall vor — dann steht eine Nachprüfung an, nicht der reguläre
Termin in 24 Monaten. Die Frist der Nachprüfung wird angegeben (bei der
HU zum Monatsende) oder fehlt. Gefunden beim Test der Fälligkeitsliste.

Präzisierung G.3c (2026-10-28, technische Ausgestaltung von §6, keine
neue fachliche Entscheidung):

- **Lückenlos** heißt: eine neue Fahrt beginnt genau am Ende ihrer
  Vorgängerin (der nicht stornierten Fahrt mit dem höchsten Ende bis zu
  ihrem Beginn); Kilometerbereiche überlappen nie; das Datum passt zur
  Reihenfolge. Damit ein stornierter Eintrag **ersetzt** werden kann, darf
  eine Fahrt in eine Lücke fallen (am Ende der Vorgängerin beginnend, ohne
  die Nachfolgerin zu überlappen); eine verbleibende Lücke wird angezeigt,
  nicht verschwiegen. Vor der ersten Fahrt darf nur nachgetragen werden,
  wenn die Kette verbunden bleibt.
- Dienstfahrt: **Reiseziel und Reisezweck Pflicht**, der aufgesuchte
  Geschäftspartner soweit vorhanden (nicht jede Fahrt hat einen, z. B. zur
  eigenen Baustelle) — ANNAHME. Privatfahrt: nur Kilometer; Arbeitsweg:
  ohne Pflichtangaben.
- Erfassungszeitpunkt wird gespeichert und nicht geändert (Nachweis
  "zeitnah"); eine Frist wird nicht erzwungen.
- Das Fahrtenbuch führt nur, wer am Fahrzeug "Fahrtenbuch führen" gesetzt
  hat; Fahrten nach der Abmeldung oder vor der Erstzulassung werden
  abgelehnt.

Präzisierung G.3d (2026-10-29, technische Ausgestaltung von §7, keine
neue fachliche Entscheidung; ANNAHMEN, vom Nutzer mit "Beende G3"
beauftragt):

- Ein Kosteneintrag ist ein **Beleg in geschlossener Form** wie
  Fahrtenbuch und Erledigungen: nie geändert, nie gelöscht, Fehler nur
  über ein Storno mit Begründung.
- Kostenarten: Kraftstoff, Wartung, Reparatur, Versicherung, Steuer,
  Leasing, Sonstiges (dann mit Bezeichnung). Wartung ist ergänzt, weil
  sie neben der Reparatur der häufigste Werkstattbeleg ist.
- Erfasst wird der **Nettobetrag** (> 0, Cent genau); die Vorsteuer
  gehört in die Buchhaltung. Gutschriften werden als Storno des Belegs
  abgebildet, nicht als negativer Betrag.
- Optional: Lieferant, Belegnummer, km-Stand, Liter (nur Kraftstoff).
- Die **Kostenstelle wird beim Erfassen festgehalten** (ohne Angabe die
  des Fahrzeugs); eine spätere Änderung am Fahrzeug verschiebt keine
  gebuchten Kosten.
- Datum: Pflicht, nicht in der Zukunft; vor der Erstzulassung (Leasing,
  Versicherung) und nach der Abmeldung (Schlussrechnung) zulässig.
- Lesen mit `fleet.read`, erfassen und stornieren mit `fleet.write`
  (Kosten sind keine personenbezogenen Bewegungsdaten).
- Auswertung: Summen je Kostenart und gesamt, nach Fahrzeug, Zeitraum
  und Kostenart filterbar; stornierte zählen nie.
- Die Anbindung an Eingangsrechnungen (Epic E) bleibt offen.
