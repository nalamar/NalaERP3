# ADR 0029 — Abwesenheiten, Feiertage, Urlaubskonto, Teams

Datum: 2026-10-19
Status: entschieden (vorgeschlagen in F.8.1; die fünf offenen Punkte hat
der Nutzer am 2026-10-19 beantwortet, siehe "Entscheidungen des Nutzers")
Bezug: Backlog F.8 (Epic F — Personal & HR); `aufgabe.md` §1, Domäne F:
"Urlaub/Abwesenheit". Baut auf ADR 0025 (Urlaubsanträge über HTTP) auf.

## Kontext — Bestand (belegt)

- **Urlaubsanträge** (`hr_leave_requests`, `internal/hr/service.go`,
  Routen `internal/http/leave_requests.go`): stellen, listen, einmalig
  entscheiden; Rechte `leave_requests.self/.read/.decide`; Typen
  `vacation | sick | unpaid | other`; Überschneidungsprüfung gegen offene
  und genehmigte Anträge. **Die Tageszahl zählt Kalendertage** (ein
  Antrag Fr–Mo kostet 4 Tage statt 2). **Kein Zurückziehen**, kein Storno
  eines genehmigten Antrags.
- **`hr_absences`** und **`hr_holidays`** (seit `020_hr_base.sql`):
  in keinem Go-Code verwendet. `hr_holidays` hat **keine `company_id`**,
  nur `location TEXT DEFAULT 'DE'`, und ist leer.
- **Urlaubsanspruch:** nirgends abgebildet — weder am Mitarbeiter noch je
  Jahr. Es gibt also kein Urlaubskonto.
- **Bundesland:** weder am Mandanten (`company_profiles`) noch am
  Mitarbeiter; `hr_employees.location` ist Freitext.
- **Teams** (`hr_teams`: Name, `lead_employee_id`, `company_id` seit 059):
  nur lesbar (`GET /employees/teams`), nicht anlegbar.
- **Client:** keine Oberfläche für Urlaub — weder Antrag noch
  Genehmigung (Befund bei F.8.1; die API-Methoden fehlen in `api.dart`).

## Recherche (Sekundärquellen, Quellen am Ende)

- **Mindesturlaub** (§3 BUrlG): 24 Werktage bei 6-Tage-Woche, also
  **20 Arbeitstage bei 5-Tage-Woche**. Mehr ist Vertrag/Tarif.
- **Übertragung** (§7 Abs. 3): Urlaub ist im Kalenderjahr zu nehmen;
  übertragen nur aus dringenden Gründen, dann **bis 31.03.** des
  Folgejahres.
- **Verfall nur nach Hinweis** (BAG 19.02.2019, 9 AZR 541/15): der
  Arbeitgeber muss rechtzeitig und konkret auf den Resturlaub und den
  drohenden Verfall hinweisen und das **im Zweifel beweisen**. Ohne
  Hinweis kein Verfall und keine Verjährung (BAG 20.12.2022,
  9 AZR 266/20). Bei Langzeiterkrankung Verfall spätestens 15 Monate nach
  Ende des Urlaubsjahres (Rechtsprechung mit Ausnahmen).
- **Teilurlaub** (§5): 1/12 je vollem Monat bei Ein-/Austritt im Jahr;
  voller Anspruch erst nach 6 Monaten (§4).
- **Krank im Urlaub** (§9): durch Attest nachgewiesene Krankheitstage
  werden nicht auf den Urlaub angerechnet.
- **Feiertage:** Ländersache. 9 Tage gelten in allen Ländern (Neujahr,
  Karfreitag, Ostermontag, 1. Mai, Christi Himmelfahrt, Pfingstmontag,
  3. Oktober, 1. und 2. Weihnachtstag); dazu je Land 1–4 weitere
  (Heilige Drei Könige, Frauentag Berlin, Fronleichnam, Mariä
  Himmelfahrt, Weltkindertag Thüringen, Reformationstag, Allerheiligen,
  Buß- und Bettag Sachsen). **Sonderfälle:** Mariä Himmelfahrt in Bayern
  nur in überwiegend katholischen Gemeinden; Augsburger Friedensfest
  (08.08.) nur in Augsburg.

**Folgerung:** Das System darf Resturlaub **nicht still verfallen
lassen** — ob er verfällt, hängt an einem Hinweis, den das System nicht
kennt. Es soll den Resturlaub sichtbar machen und den Hinweis
nachweisbar machen.

## Vorschlag

### 1. Arbeitstage statt Kalendertage
Ein Antrag zählt **Arbeitstage**: Tage des Zeitraums, die laut
Arbeitstage-Muster des Mitarbeiters Arbeitstage sind (Standard Mo–Fr),
**abzüglich gesetzlicher Feiertage**. Eine kleinere Angabe bleibt
zulässig (halbe Tage). Bestehende Anträge werden nicht umgerechnet
(Bestand, bisher kein produktiver Client).

### 2. Feiertage berechnen statt pflegen (→ Frage 1)
Gesetzliche Feiertage werden **im Code berechnet** (Osterformel + feste
Daten je Land) — keine Tabelle, die jemand jährlich füllen muss.
Bundesland als **Einstellung am Mandanten** (Migration), optional am
Mitarbeiter überschreibbar (Monteure an anderem Standort).
`hr_holidays` bekommt `company_id` und dient nur noch **betrieblichen
Zusatztagen** (Betriebsruhe, 24./31.12.), gepflegt von der
Personalabteilung. Sonderfälle Mariä Himmelfahrt (Bayern) und Augsburger
Friedensfest als Schalter am Mandanten.

### 3. Urlaubskonto je Jahr (→ Frage 2)
Neue Tabelle `hr_leave_entitlements` (Mitarbeiter, Jahr, Anspruch,
Übertrag aus Vorjahr, Notiz). Am Mitarbeiter ein **Jahresanspruch** als
Vorgabe, aus der das Konto eines neuen Jahres angelegt wird; im
Eintritts-/Austrittsjahr schlägt das System den Teilurlaub nach §5 vor
(nur Vorschlag, die Personalabteilung bestätigt). **Rest = Anspruch +
Übertrag − genehmigte Urlaubstage − offene Urlaubstage (als "verplant"
getrennt ausgewiesen).** Nur Typ `vacation` zählt aufs Konto.

### 4. Kein automatischer Verfall (→ Frage 3)
Der Übertrag ins Folgejahr wird **von der Personalabteilung gesetzt**,
nicht automatisch berechnet und nicht automatisch gestrichen. Dazu eine
Liste "Resturlaub" (alle Mitarbeiter mit offenem Rest) und je
Mitarbeiter/Jahr ein Feld **"Hinweis auf Resturlaub und Verfall erteilt
am"** als Nachweis im Sinne des BAG.

### 5. Abwesenheiten (→ Frage 4)
`hr_absences` wird die eine Quelle für "wer fehlt wann":
- **Genehmigter Antrag** → Abwesenheit in derselben Transaktion
  (`source_request_id`).
- **Krankheit** wird von der Personalabteilung **direkt als Abwesenheit**
  erfasst, ohne Antrag und Genehmigung — eine Krankmeldung wird nicht
  "genehmigt". **Keine Diagnose, kein Grund** (Gesundheitsdaten,
  Art. 9 DSGVO); optional "Attest liegt vor" ja/nein.
- **Krank im Urlaub (§9):** Erfassen einer Krankheit in einem genehmigten
  Urlaub mit Attest schreibt die betroffenen Urlaubstage aufs Konto
  zurück (Urlaubsabwesenheit für diese Tage storniert).
- Unveränderlich wie Zeiterfassung: Korrektur über **Storno mit
  Begründung**.

### 6. Zurückziehen und Storno
- **Offener Antrag:** der Antragsteller zieht ihn selbst zurück
  (neuer Status `withdrawn`).
- **Genehmigter Antrag:** nur mit `leave_requests.decide` stornierbar,
  mit Pflicht-Begründung; die Abwesenheit wird mitstorniert, die Tage
  gehen aufs Konto zurück. Bereits vergangene Tage: zulässig, aber mit
  Begründung (Fehlbuchung).

### 7. Teams (→ Frage 5)
Teams anlegen, umbenennen, Teamleiter setzen mit `employees.write`.
Vorerst **ohne fachliche Wirkung auf Rechte** — Teams dienen der
Abwesenheitsübersicht ("wer aus meinem Team fehlt nächste Woche").

### 8. Abwesenheitsübersicht
`GET /absences?from&to[&team_id]`: mit `leave_requests.read` alle mit
Art; mit `leave_requests.self` die eigenen. **Kollegen sehen sich
gegenseitig nicht** (vorerst; siehe Frage 5).

### 9. Rechte
Bestehende Rechte reichen: `leave_requests.self/.read/.decide`.
Konto und Krankmeldung pflegt, wer `leave_requests.decide` hat; die
Mandanten-Einstellung Bundesland gehört zu `settings.manage`.

## Offen — Freigabe des Nutzers nötig vor F.8.2

1. **Feiertage:** berechnen + Bundesland am Mandanten, am Mitarbeiter
   überschreibbar, betriebliche Zusatztage gepflegt? *Empfehlung: ja.*
2. **Urlaubskonto:** Jahreskonto mit Jahresanspruch am Mitarbeiter als
   Vorgabe und Teilurlaub als Vorschlag? Welche Vorgabe bei neuen
   Mitarbeitern — gesetzlich 20 oder betrieblich üblich (z. B. 30)?
   *Empfehlung: ja; Vorgabe als Mandanten-Einstellung, Startwert 30.*
3. **Verfall:** kein automatischer Verfall, Übertrag setzt die
   Personalabteilung, Nachweis "Hinweis erteilt am"?
   *Empfehlung: ja — automatischer Verfall ist ohne nachweisbaren
   Hinweis rechtlich falsch.*
4. **Krankheit:** direkt als Abwesenheit durch die Personalabteilung,
   ohne Diagnose, mit "Attest ja/nein" und Rückbuchung nach §9?
   Darf der Mitarbeiter sich selbst krankmelden (ohne Genehmigung, nur
   Meldung)? *Empfehlung: ja; Selbstmeldung erlaubt, Personalabteilung
   wird darüber sichtbar informiert (Liste "neue Krankmeldungen").*
5. **Teams und Sichtbarkeit:** Darf der Teamleiter Anträge seines Teams
   genehmigen, und sehen Kollegen, wer im Team abwesend ist (ohne Art)?
   *Empfehlung: beides vorerst nein — erst Grundfunktion, dann als
   eigener Schritt; Teamleiter-Genehmigung braucht eine Stellvertreter-
   Regel.*

## Entscheidungen des Nutzers (2026-10-19)

1. **Feiertage:** "Ja genau so" — berechnet, Bundesland am Mandanten,
   am Mitarbeiter überschreibbar, betriebliche Zusatztage pflegbar.
2. **Urlaubskonto:** "Genau so. Jeder Mitarbeiter bekommt mindestens 20
   Tage Urlaub, wenn anderes ausgehandelt wird, hinterlegt die
   Personalabteilung das im Mitarbeiter." → **Vorgabe 20 Tage** (statt
   der vorgeschlagenen Mandanten-Einstellung mit 30); abweichender
   Anspruch am Mitarbeiter. *Annahme (zu bestätigen bei F.8.3):* die
   20 gelten für die 5-Tage-Woche; bei weniger Arbeitstagen je Woche
   anteilig (gesetzlich 4 × Arbeitstage je Woche, z. B. 12 bei drei
   Tagen). Ein Anspruch unter dieser Untergrenze wird abgelehnt.
   Vorgelegt am 2026-10-19, Antwort "fortsetzen" — als Zustimmung
   gewertet und so umgesetzt (F.8.3.1), nicht ausdrücklich bestätigt.
3. **Verfall:** "Genau so" — kein automatischer Verfall, Übertrag durch
   die Personalabteilung, Nachweisfeld "Hinweis erteilt am".
4. **Krankheit:** "So umsetzen" — direkt als Abwesenheit ohne Diagnose,
   Attest ja/nein, Rückbuchung nach §9, Selbstmeldung erlaubt, Liste
   "neue Krankmeldungen" für die Personalabteilung.
5. **Genehmigung und Teams:** "Nein, Urlaub genehmigt nur
   Personalabteilung oder Geschäftsführer im Moment, eine Erweiterung
   auf Teamleiter aber bitte schon mit vorbereiten." → Genehmigen bleibt
   bei `leave_requests.decide` (heute Rollen `role-hr` und `role-admin`
   laut 087 — das deckt Personalabteilung und Geschäftsführung ab).
   **Vorbereitung Teamleiter (F.8.4):** die Frage "darf dieser Login
   diesen Antrag entscheiden?" wird an **einer** Stelle im Service
   beantwortet (statt nur über `requirePermission`), die den
   Antrag kennt; der Teamleiter (`hr_teams.lead_employee_id`) wird dort
   später mit einem eigenen Recht (etwa `leave_requests.decide_team`)
   ergänzt. Das Recht wird jetzt **nicht** angelegt. Kollegen sehen
   Abwesenheiten anderer vorerst nicht.

## Vorgeschlagene Aufteilung (nach Freigabe)

- **F.8.2** Feiertage und Arbeitstage, geteilt in **F.8.2.1**
  Feiertagsberechnung und Arbeitstage-Zählung (rein, ohne DB) und
  **F.8.2.2** Bundesland am Mandanten/Mitarbeiter, betriebliche
  Zusatztage, Zählung in neuen Anträgen.
- **F.8.3** Urlaubskonto (Tabelle, Jahresanspruch, Teilurlaub-Vorschlag,
  Rest, Liste Resturlaub, Hinweis-Nachweis) mit Routen.
- **F.8.4** Abwesenheiten: aus Genehmigung, Krankmeldung, §9-Rückbuchung,
  Zurückziehen/Storno von Anträgen, Übersicht.
- **F.8.5** Teams anlegen/ändern.
- **F.8.6** Client: Urlaub beantragen und zurückziehen, Konto anzeigen,
  Genehmigen, Krankmeldung, Abwesenheitsübersicht, Teams (weiter
  geteilt).

## Abweichung bei der Umsetzung (F.8.2.2, 2026-10-19)

Die betrieblichen Zusatztage stehen nicht in `hr_holidays` (mit neuer
`company_id`), sondern in der eigenen Tabelle `hr_company_days`
(Migration 090). Grund: ADR 0002 hat `hr_holidays` ausdrücklich als
mandantenfreien Feiertagskalender je Land festgelegt; mit
`company_id` hätte dieselbe Tabelle zwei Bedeutungen. `hr_holidays`
bleibt ungenutzt. Das Bundesland steht in `hr_calendar_settings`, nicht
in `company_profiles` — die Firmenstammdaten werden vollständig per
Upsert ersetzt und gehören zu Rechnung und Briefkopf.

## Konsequenzen

- Migrationen ab 090 (Bundesland, Konto, `hr_holidays.company_id`,
  Antragsstatus `withdrawn`, Abwesenheits-Storno). Backlog 0.49 nimmt
  dann die nächste freie Nummer.
- Bestehende Anträge behalten ihre Kalendertag-Zählung.
- Kein Verfall durch das System; die Verantwortung für den Hinweis bleibt
  beim Arbeitgeber, das System liefert Liste und Nachweisfeld.

## Quellen der Recherche

- LTO: Urlaub, Hinweise, Verfall, Verjährung —
  https://www.lto.de/karriere/im-job/stories/detail/urlaub-arbeitsrecht-hinweise-verfall-maerz-verjaehrung
- factorial: Urlaubsübertragung — https://factorialhr.de/blog/urlaubsuebertragung/
- WEKA: Resturlaub, Verfall oder Übertragung —
  https://www.weka.de/betriebsrat-personalrat/checkliste-voraussetzungen-fuer-urlaubsverfall-dl/
- absentify: Feiertage Deutschland nach Ländern —
  https://absentify.com/de/feiertage/deutschland
- jurawelt: Feiertage 2026 nach Bundesländern —
  https://jurawelt.com/feiertage-2026-im-kalender-deutschland-nach-bundeslaendern-sortiert/

Die Quellen sind Ratgeber und Kanzleiinformationen, keine Urteilstexte.
Die Feiertagsliste je Land wird in F.8.2 gegen die Landesgesetze bzw.
eine zweite Quelle je Land geprüft und als Test festgeschrieben.
