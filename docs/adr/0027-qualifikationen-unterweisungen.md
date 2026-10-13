# ADR 0027 — Weiterbildung, Qualifikationen, Unterweisungen

Datum: 2026-10-13
Status: entschieden (Subtask F.2.1); am 2026-10-13 durch zwei Antworten des Nutzers ergänzt (siehe "Ergänzung")
Bezug: Backlog F.2 (Epic F — Personal & HR).

## Kontext

`aufgabe.md` §1 nennt für Domäne F "Weiterbildung/Qualifikationen/
Unterweisungen". Im Repo gibt es dazu nichts: die Suche nach `qualif`,
`schulung`, `training`, `unterweis`, `zertifik`, `weiterbild`,
`befähig` über `server/internal` (Go und SQL) und `client/lib` liefert
nur Treffer in der E-Rechnung (`instruction`/`certificate` als
Fachwörter dort); `docs/01-gap-analysis.md:95` führt den Punkt als
"offen, kein Schema-Bezug".

**Vorgabe des Nutzers (2026-10-13):** branchenspezifisch fallen ihm
Arbeitsschutz, Erste Hilfe, Staplerschein, Scherenbühnenschein und
Leiterschein ein; weitere dürfen recherchiert und vorgeschlagen werden;
**die Liste muss in jedem Fall erweiterbar sein.**

Vorhandene Bausteine, auf die F.2 aufsetzt: Mitarbeiterstamm und
Benutzer-Verknüpfung über HTTP (F.4), die Rechteprüfung "eigene Daten
mit self, fremde mit breitem Recht" (`resolveOwnOrPermittedEmployee`),
das Muster "unveränderlich, Korrektur über Storno" und "Auswertung beim
Lesen berechnet, nicht gespeichert" (F.1).

## Recherche: was im Metallbau üblich ist

In dieser Session per Websuche geprüft. **Die Quellen sind überwiegend
Sekundärquellen** (Wissensdatenbank KomNet, Unfallkassen,
Berufsgenossenschaften, Bildungsträger), nicht die Originaltexte in
aktueller Fassung. Die Fristen sind deshalb Vorschlagswerte, die der
Betrieb je Eintrag ändern kann — und vor produktiver Nutzung mit der
Fachkraft für Arbeitssicherheit abgleichen sollte.

| Thema | Befund | Vorschlag Frist |
|---|---|---|
| Allgemeine Unterweisung Arbeitsschutz | ArbSchG §12, DGUV Vorschrift 1 §4: vor Aufnahme der Tätigkeit, danach bei Bedarf, mindestens einmal jährlich; ist zu dokumentieren | 12 Monate |
| Ersthelfer | DGUV Vorschrift 1 §26: Fortbildung "in der Regel" alle zwei Jahre, Nachweise sind vorzulegen; Anzahl: ab mehr als 20 Anwesenden 10 % in sonstigen Betrieben | 24 Monate |
| Flurförderzeuge (Stapler) | DGUV Vorschrift 68 §7: mindestens 18, geeignet, ausgebildet, Befähigung nachgewiesen, **schriftlich beauftragt**; Ausbildung nach DGUV Grundsatz 308-001; die Ausbildung selbst läuft nicht ab, die Unterweisung ist jährlich | Schein unbefristet, Unterweisung 12 Monate |
| Hubarbeitsbühnen (Scherenbühne) | DGUV Grundsatz 308-008 (Ausbildung, schriftliche Beauftragung mit Bauarten); jährliche Unterweisung nach DGUV Vorschrift 1 §4 | Schein unbefristet, Unterweisung 12 Monate |
| Leitern und Tritte | **Für Benutzer gibt es keinen "Schein", sondern die Unterweisung.** Eine formale Qualifikation ist die "befähigte Person zur Prüfung von Leitern und Tritten" (BetrSichV, TRBS 1203, DGUV Information 208-016) — das ist die Person, die die Leitern prüft, nicht jeder, der sie benutzt | Unterweisung 12 Monate; Prüfer unbefristet |
| Schweißerprüfung | DIN EN ISO 9606-1: Zertifikat drei Jahre gültig, **zusätzlich Bestätigung alle sechs Monate** durch Arbeitgeber/Schweißaufsicht, sonst erlischt es vorher; Verlängerung um zwei Jahre unter Bedingungen (Quelle: Unterlage zur ÖNORM-Fassung; die Fassung 2017-12 kennt weitere Varianten) | 36 Monate |
| Brandschutzhelfer | ASR A2.2 / DGUV Information 205-023: Richtwert 5 % der Beschäftigten bei normaler Brandgefährdung, Wiederholung alle drei bis fünf Jahre | 36 Monate |

Weitere im Metallbau übliche Themen, **in dieser Session nicht
recherchiert** und deshalb ohne Aussage zur Rechtsgrundlage
vorgeschlagen: Kranführer (Hallen-/Brückenkran), Anschlagen von Lasten,
persönliche Schutzausrüstung gegen Absturz, Unterweisung Gefahrstoffe,
elektrotechnisch unterwiesene Person, Ladungssicherung.

## Optionen und Entscheidungen

### 1. Fester Katalog im Code oder Stammdaten?

**A — feste Liste (Enum) im Code.** Verworfen: widerspricht der
Vorgabe "erweiterbar" unmittelbar; jeder neue Schein wäre ein Release.

**B — Katalog als Stammdaten je Mandant.** Gewählt. Jeder Mandant
pflegt seine eigenen Arten (anlegen, umbenennen, Frist ändern,
deaktivieren).

Damit ein Mandant nicht bei null anfängt, gibt es einen
**Vorschlagskatalog**: eine im Code gepflegte Liste (die Tabelle oben
plus die sechs weiteren Themen), die ein Mandant per Aufruf in seinen
Katalog übernimmt. Bewusst als ausdrückliche Aktion und nicht als
Migration, die jedem Mandanten Daten unterschiebt: was übernommen ist,
gehört dem Mandanten und kann von ihm geändert werden; ein erneuter
Aufruf ergänzt nur, was fehlt, und überschreibt nichts.

### 2. Eine Art oder mehrere je Thema?

Beim Stapler stecken drei verschiedene Dinge in "Staplerschein": die
einmalige Ausbildung (läuft nicht ab), die jährliche Unterweisung
(läuft ab) und die schriftliche Beauftragung durch den Unternehmer
(gilt bis Widerruf). Ein Eintrag mit einer Frist kann das nicht
abbilden — entweder liefe der Schein nach einem Jahr "ab", oder die
überfällige Unterweisung bliebe unsichtbar.

Entscheidung: jede Art hat **eine Kategorie und höchstens eine
Frist**; ein Thema kann aus mehreren Arten bestehen. Kategorien:

- `qualifikation` — Ausbildung, Schein, Prüfung (z. B. Staplerschein,
  Schweißerprüfung).
- `unterweisung` — wiederkehrende Unterweisung (z. B. Arbeitsschutz
  jährlich, Stapler jährlich).
- `beauftragung` — schriftliche Beauftragung oder Bestellung durch den
  Unternehmer (z. B. Beauftragung Staplerfahrer, Bestellung Ersthelfer).

Der Vorschlagskatalog enthält für Stapler und Hubarbeitsbühne je drei
Arten, für Leitern zwei (Unterweisung der Benutzer; befähigte Person
zur Prüfung).

### 3. Was ist ein Nachweis, und wie lange gilt er?

Ein **Nachweis** ist die Tatsache, dass ein Mitarbeiter an einem Tag
eine Art erworben bzw. erhalten hat: Mitarbeiter, Art, `erworben_am`,
`gueltig_bis`, dazu Aussteller, Nachweisnummer, Notiz.

`gueltig_bis` wird beim Erfassen festgelegt: entweder ausdrücklich
angegeben (das Zertifikat nennt sein eigenes Datum) oder aus
`erworben_am` plus der Frist der Art berechnet; ohne Frist bleibt es
leer (unbefristet). Der Wert wird **gespeichert**, nicht bei jedem
Lesen neu aus der Art berechnet — ändert der Betrieb später die Frist
einer Art, sollen bereits ausgestellte Nachweise nicht rückwirkend
anders gelten.

Eine Wiederholung ist ein **neuer Nachweis**; der alte bleibt stehen.
So ist die Historie ("wann wurde wer unterwiesen") ohne Zusatzlogik
vorhanden. Maßgeblich für den aktuellen Stand ist je Mitarbeiter und
Art der jüngste aktive Nachweis.

### 4. Unveränderlich?

Unterweisungsnachweise haben Beweisfunktion: nach einem Arbeitsunfall
ist die Frage, ob und wann unterwiesen wurde. Ein Nachweis, der sich
nachträglich ändern oder löschen lässt, beweist nichts.

Entscheidung wie bei den Zeiteinträgen (ADR 0024): ein Nachweis wird
nicht geändert und nicht gelöscht. Ein Fehler wird durch **Storno**
(Pflichtbegründung, wer, wann) und Neuerfassung behoben. Einen eigenen
Korrektur-Aufruf wie bei den Zeiten braucht es nicht — es gibt hier
keine Überschneidungsregeln, die Storno und Ersatz in einer Transaktion
erzwingen würden.

Fremdschlüssel auf Mitarbeiter und Art mit `ON DELETE RESTRICT`; Arten
werden nicht gelöscht, sondern deaktiviert.

### 5. Fällig, abgelaufen, fehlt

Der Status wird **beim Lesen berechnet** und nicht gespeichert (wie die
ArbZG-Hinweise in F.1.3):

| Status | Bedeutung |
|---|---|
| `gueltig` | jüngster aktiver Nachweis gilt noch länger als die Vorwarnzeit |
| `laeuft_ab` | gilt noch, aber `gueltig_bis` liegt innerhalb der Vorwarnzeit |
| `abgelaufen` | `gueltig_bis` liegt in der Vergangenheit |
| `fehlt` | Art ist Pflicht für alle, und es gibt keinen aktiven Nachweis |

`gueltig_bis` ist der letzte Tag der Gültigkeit (einschließlich).
Vorwarnzeit: **Annahme 60 Tage**, je Abfrage änderbar.

"Fehlt" setzt voraus, dass das System weiß, wer was haben muss. Eine
vollständige Anforderungsmatrix (welche Tätigkeit verlangt welchen
Schein) gehört zur Einsatzplanung (Epic H) und wird hier nicht gebaut.
F.2 kennt nur den einen Fall, der ohne Tätigkeitsprofile entscheidbar
ist: eine Art kann als **Pflicht für alle aktiven Mitarbeiter**
gekennzeichnet werden (`pflicht_fuer_alle`) — das trifft die
allgemeine Arbeitsschutzunterweisung. Für alle anderen Arten zeigt das
System Ablauf und Fälligkeit dessen, was erfasst ist, behauptet aber
nicht, jemandem fehle ein Staplerschein.

### 6. Unterweisung für eine Gruppe

Unterwiesen wird in der Praxis die ganze Werkstatt auf einmal. Ein
Aufruf je Person wäre eine Zumutung und eine Fehlerquelle.
Entscheidung: ein Nachweis lässt sich in einem Aufruf für mehrere
Mitarbeiter erfassen — in **einer Transaktion**, ganz oder gar nicht.
Es entsteht je Mitarbeiter ein eigener Nachweis (kein gemeinsamer
"Termin"-Datensatz): storniert wird je Person, und die Auswertung
bleibt eine Abfrage je Mitarbeiter und Art.

### 7. Rechte

Neu, nach dem Muster der Zeiterfassung:

- `qualifications.self` — eigene Nachweise und deren Status lesen; an
  alle Systemrollen.
- `qualifications.read` — Nachweise und Auswertung aller Mitarbeiter.
- `qualifications.write` — Nachweise erfassen und stornieren, Katalog
  pflegen. Beide an `role-hr` und `role-admin`.

Den Katalog (Namen der Arten) darf lesen, wer eines der drei Rechte
hat. Mitarbeiter erfassen ihre Nachweise nicht selbst: ein Nachweis
ist eine Aussage des Betriebs, nicht des Mitarbeiters.

## Ergänzung nach Antwort des Nutzers (2026-10-13)

Auf die zwei offenen Fragen hat der Nutzer geantwortet. Beide
Antworten gehen in das Modell ein, bevor Code entsteht.

**1. Wer erfasst, und was soll das System tun?** "Die
Personalabteilung trägt ein, welche Qualifikationen erreicht sind und
wann sie erreicht wurden. Eine anstehende Erneuerung sollte vom System
angemahnt werden."

Damit ist die Annahme bestätigt: kein Selbsteintrag, keine Bestätigung
durch den Mitarbeiter. "Anmahnen" wird so umgesetzt: der Status
`laeuft_ab` (innerhalb der Vorwarnzeit) und `abgelaufen` je Nachweis,
dazu eine Liste aller anstehenden und überfälligen Erneuerungen des
Betriebs für die Personalabteilung (F.2.6) und der eigene Stand für
jeden Mitarbeiter. **Eine aktive Benachrichtigung (E-Mail o. ä.) ist
damit nicht gebaut**: das Repo hat keine Infrastruktur zum Versenden
von Nachrichten. Ob "anmahnen" eine solche Zustellung verlangt, ist
als offene Frage vermerkt (→ Backlog F.10).

**2. Wer braucht was?** "Beim Anlegen eines neuen Mitarbeiters soll die
Personalabteilung festlegen können, welche Qualifikationen für die
Stelle nötig sind und ob sie schon vorhanden sind."

Das ändert Entscheidung 5: es gibt **Anforderungen je Mitarbeiter**
(neue Tabelle `hr_employee_qualification_requirements`: Mitarbeiter,
Art). `fehlt` gilt damit für jede Art, die dem Mitarbeiter als
Anforderung zugeordnet ist **oder** für alle Pflicht ist
(`pflicht_fuer_alle` bleibt, damit die Arbeitsschutzunterweisung nicht
jedem einzeln zugeordnet werden muss), und für die kein gültiger
Nachweis vorliegt.

Eine Anforderung ist Planung, kein Beleg: sie ist änderbar und darf
wieder entfernt werden, wenn sich die Stelle ändert. Deshalb gilt für
sie — anders als für Nachweise — keine Unveränderbarkeit.

"Beim Anlegen" wird über die API in drei Schritten abgebildet, die
die Oberfläche in einem Dialog zusammenfasst: Mitarbeiter anlegen
(`POST /employees`), Anforderungen setzen, vorhandene Nachweise
erfassen. Die Mitarbeiteranlage selbst bleibt unverändert und kennt
keine Qualifikationen; so bleibt der Mitarbeiterstamm ohne Kenntnis
des Katalogs benutzbar. **Bekannte Folge**: die drei Schritte sind
nicht eine Transaktion; scheitert der zweite, existiert der
Mitarbeiter ohne Anforderungen. Das ist erkennbar und nachholbar,
aber nicht atomar.

Eine Anforderungsmatrix je **Stelle oder Tätigkeit** (Vorlagen wie
"Monteur braucht A, B, C", die man einem neuen Mitarbeiter auf einmal
zuweist) gibt es weiterhin nicht — Stellen sind im Mitarbeiterstamm
Freitext (`role`). Das bleibt bei Epic H bzw. Backlog F.10.

Nachtrag F.2.4.1 (2026-10-13): beim Umsetzen der Regeln präzisiert.
(1) Maßgeblich für den Stand ist je Art der Nachweis, der **am
längsten gilt**, nicht der zuletzt erworbene (Entscheidung 3 sagte
"der jüngste aktive Nachweis"). Im Normalfall ist das derselbe; sie
unterscheiden sich, wenn nach einer lang gültigen Ausbildung eine
kürzer laufende Auffrischung erfasst wird — dann soll der Mitarbeiter
nicht als abgelaufen gelten, solange die Ausbildung gilt.
(2) Bei der Fristberechnung gilt am Monatsende der letzte Tag des
Zielmonats (31.01. plus ein Monat = 28.02.).
(3) Der Tag gleichen Datums nach Ablauf der Frist zählt noch als
gültig. (4) Ein Erwerbsdatum in der Zukunft wird abgelehnt.

## Annahmen (gekennzeichnet, änderbar)

- Fristen im Vorschlagskatalog wie in der Tabelle; sie sind Vorgaben
  beim Übernehmen und je Mandant änderbar.
- Vorwarnzeit 60 Tage.
- Fristen in ganzen Monaten.
- Ein Nachweis wird von der Personalabteilung erfasst; der Mitarbeiter
  bestätigt ihn im System **nicht** (vom Nutzer am 2026-10-13 bestätigt).

## Nicht Teil von F.2

- **Arbeitsmedizinische Vorsorge und Eignungsuntersuchungen** (z. B.
  für Fahr- und Steuertätigkeiten). Das sind Gesundheitsdaten nach
  Art. 9 DSGVO mit eigenen Anforderungen; sie gehören nicht in einen
  Katalog neben den Staplerschein. Im Vorschlagskatalog bewusst nicht
  enthalten.
- **Führerscheinkontrolle** → Epic G.
- **Einsatzfähigkeit prüfen** ("darf diese Person diese Maschine
  bedienen?") und Anforderungen je Tätigkeit → Epic H. F.2 legt das
  Datenmodell so an, dass H darauf lesen kann.
- **Sechsmonatige Bestätigung der Schweißerprüfung.** Abgebildet wird
  das Zertifikat mit seinem Ablaufdatum; die Zwischenbestätigungen
  nicht (→ Backlog F.10).
- **Nachweisdokumente** (Scan des Zertifikats, unterschriebene
  Teilnehmerliste). GridFS und ein Muster dafür sind vorhanden
  (`contacts/documents.go`); F.2 erfasst zunächst Aussteller und
  Nachweisnummer (→ Backlog F.10).
- **Prüfung der Arbeitsmittel selbst** (die Leiter, der Stapler) — das
  ist Asset-Verwaltung (F.3) bzw. Fuhrpark.
- Schulungsplanung, Kosten, Anmeldung zu Kursen.
- Client-Oberfläche.

## Konsequenzen

Zerlegung von F.2:

- **F.2.2** Migration `088_hr_qualifications.sql`: Katalog, Nachweise,
  Anforderungen je Mitarbeiter, drei Permissions samt Rollenzuordnung.
- **F.2.3** Katalog: Service (anlegen, ändern, deaktivieren, lesen),
  Vorschlagskatalog samt Übernahme, Tests.
- **F.2.4** Nachweise und Anforderungen: Service (Nachweise erfassen
  einzeln und für eine Gruppe, stornieren, lesen; Anforderungen setzen
  und lesen), Statusauswertung als reine Funktion, Tests.
- **F.2.5** HTTP-Anbindung Katalog und Nachweise mit Rechteprüfung.
- **F.2.6** HTTP-Auswertung: Stand je Mitarbeiter, Liste der fälligen
  und fehlenden Nachweise des Betriebs.

Neue Backlog-Position **F.10**: Nachweisdokumente; Zwischenbestätigung
Schweißerprüfung; Bestätigung durch den Mitarbeiter (je nach Antwort
auf die offene Frage).

## Quellen der Recherche

- DGUV Vorschrift 1 (Fassung November 2013), §4 und §26:
  <https://www.ukbw.de/fileadmin/user_upload/Mediathek/UVV/DGUV_Vorschrift_1.pdf>,
  <https://www.ukbb.de/fileadmin/user_upload/cms/download_user/Praevention/Erste_Hilfe/DGUV_Vorschrift_1.pdf>
- KomNet zu Unterweisungsabständen und Ersthelfer-Fortbildung:
  <https://www.komnet.nrw.de/_sitetools/dialog/4523>,
  <https://www.komnet.nrw.de/_sitetools/dialog/42991>
- Flurförderzeuge: <https://www.komnet.nrw.de/_sitetools/dialog/14247>,
  <https://www.komnet.nrw.de/KomNet/files/32140/requ/308-001_-_bgg925_-_Ausbildung_Beauftragung_Staplerfahrer.pdf>,
  <https://www.sicherheit.uni-freiburg.de/Dokumente/stapler>
- Hubarbeitsbühnen: <https://vorschriften.bgn-branchenwissen.de/daten/dguv/308_008/5.htm>,
  <https://www.komnet.nrw.de/_sitetools/dialog/13153>,
  <https://www.bgbau.de/fileadmin/Medien-Objekte/Medien/Bausteine/b_212/b_212.pdf>
- Leitern und Tritte: <https://www.lvq.de/fileadmin/user_upload/A06_LVQ_BP_Leitern_2024.pdf>,
  <https://www.mibrag.de/wg-inhalte/uploads/Leitern-und-Tritte.pdf>
- Schweißerprüfung: <https://www.wifi.at/uploads/PE2020_A1010_SWST_DE_Rezertifizierung_20160728.pdf>,
  <https://www.dinmedia.de/de/norm/din-en-iso-9606-1/191783939>
- Brandschutzhelfer: <https://www.komnet.nrw.de/KomNet/files/32818/requ/205-023_-_bgi5182_-_Brandschutzhelfer.pdf>,
  <https://www.weka.de/brandschutz/die-neue-asr-a2-2-massnahmen-gegen-braende/>
