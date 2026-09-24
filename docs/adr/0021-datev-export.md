# ADR 0021 — DATEV-Export (EXTF, Format-Version 13, Buchungsstapel)

Datum: 2026-09-04
Status: entschieden (Subtask E.3.1)
Bezug: Backlog E.3 (Epic E — Finanzwesen, dritte Task, nach E.1/E.2).

## Kontext

`aufgabe.md` §2 fordert fix eine "Buchhaltungsschnittstelle: DATEV-konformer
Export (`EXTF / DATEV-Format-Version 13`)". `docs/open-questions.md` hatte
dazu vermerkt: "Keine Angabe, ob ein konkretes Buchhaltungssystem als
DATEV-Exportziel vorgegeben ist — wird bei Erreichen von Epic E erneut
geprüft." **Diese Frage ist jetzt geklärt**: EXTF ("Extern-Format") ist
laut DATEV-Definition genau dafür da, Daten aus NICHT-DATEV-Software in
JEDE DATEV-kompatible Buchhaltungssoftware zu importieren — ein konkretes
Zielsystem ist damit keine Voraussetzung für die Formatwahl, sondern der
ganze Sinn des Formats. Kein Blocker mehr.

**Datenbasis im Repo**: `journal_entries`/`journal_lines` (017/018, erweitert
um `company_id` in 058 und `kostenstelle_id` in E.1/081) sind die einzige
Buchführungs-Datenquelle — jede Buchung, die über `accounting.JournalService`
läuft (u. a. `sales`/`bank`-Reconciliation, künftig `accounting.APService`),
landet dort. `accounts` (017) ist ein SKR04-Auszug (Kommentar im Migrations-
Header: "Seed SKR04-Auszug"). Es gibt AKTUELL KEINE Buchungsperioden-/
Exportlauf-Verwaltung — der Export ist rein lesend über einen frei wählbaren
Datumsbereich.

**Fehlende Recherchegrundlage im Repo**: Es existiert keine lokale Kopie der
offiziellen DATEV-Datensatzbeschreibung. Da eine falsch geratene Feldreihen-
folge/-belegung eine für die Buchhaltungssoftware unbrauchbare oder – schlimmer
– STILL FALSCH interpretierte Datei erzeugen würde (GoBD-relevantes Risiko),
wurde die exakte Feldspezifikation für diese ADR per `WebSearch`/`WebFetch`
recherchiert und gegen zwei unabhängige, sich deckende Quellen verifiziert:
1. Eine reale Beispieldatei `EXTF_Buchungsstapel.csv` (Kopf- und Spaltenzeile
   mit echten Feldwerten) aus dem aktiv gepflegten Open-Source-Ruby-Gem
   [`ledermann/datev`](https://github.com/ledermann/datev) (production-genutzt
   für exakt diesen Zweck, MIT-lizenziert).
2. Der vollständige Feld-für-Feld-Quellcode desselben Gems
   (`lib/datev/base/header.rb`: 31 Header-Felder; `lib/datev/base/booking.rb`:
   125 Buchungssatz-Felder inkl. Typ/Länge/Pflichtfeld-Kennzeichen/Kommentare
   aus der DATEV-Doku; `lib/datev/export.rb`: Trennzeichen/Encoding/
   Zeilenende; `lib/datev/field/*.rb`: exakte Ausgabeformatierung je Feldtyp).

Beide Quellen stimmen in Struktur, Feldnamen und Beispielwerten exakt überein
— hohe Verlässlichkeit trotz fehlender Offline-Primärquelle. Diese Recherche
ersetzt keine offizielle DATEV-Zertifizierung (die ohnehin nur DATEV selbst
vergibt und ein eigenes Prüfverfahren mit Testdaten voraussetzt, das in dieser
Umgebung nicht durchführbar ist) — Ziel ist ein strukturell und inhaltlich
korrektes, in jede DATEV-kompatible Software importierbares EXTF-Format-13-
Buchungsstapel-Dokument, nicht ein zertifiziertes DATEV-Partner-Modul.

## Optionen

**Formatkategorie**: DATEV EXTF kennt mehrere Datenkategorien (21 =
Buchungsstapel, 46 = Kontenbeschriftungen/Kontenrahmen, 16 = Debitoren/
Kreditoren-Stammdaten, u. a.). Nur **21 "Buchungsstapel"** deckt den in
aufgabe.md geforderten Anwendungsfall (Buchführungsdaten exportieren) und
passt zur vorhandenen Datenbasis (`journal_entries`/`journal_lines`).
Kontenrahmen-/Stammdaten-Export ist nicht Teil dieser Task — unsere Accounts
folgen bereits SKR04, ein Abgleich mit dem Kontenrahmen des Zielsystems ist
Aufgabe des Anwenders/Steuerberaters beim Import, nicht unseres Exports.

**Struktur (durch Recherche belegt, nicht verhandelbar)**: Datei = eine
Kopfzeile (31 Felder) + eine Spaltennamen-Zeile (fest vorgegebene Liste,
125 Spalten für Buchungssätze) + N Datenzeilen. Semikolon-getrennt,
Windows-1252-kodiert, Zeilenende `\r\n`, Dezimaltrennzeichen Komma (kein
Tausendertrennzeichen), Textfelder in doppelte Anführungszeichen gefasst
(enthaltene `"` verdoppelt), numerische/Datums-Felder unquotiert, leere
Felder bleiben komplett leer (auch bei Textfeldern keine leeren `""`).
Gewählt: 1:1-Umsetzung dieser recherchierten Struktur, keine Vereinfachung
(z. B. kein UTF-8 statt Windows-1252 — DATEV-Importer verlangen Windows-1252
und ein UTF-8-Umlaut würde sonst als Mojibake importiert).

**Feldabdeckung der 125 Buchungssatz-Spalten**: Alle befüllen (unrealistisch,
viele Felder wie "Abrechnungsreferent", "BVV-Position", 20×"Zusatzinformation"-
Paare haben keine Entsprechung in unserem Datenmodell) vs. nur die für unseren
Datenbestand sinnvollen/vorhandenen Felder befüllen, Rest korrekt leer lassen?
Gewählt: **Minimal-aber-vollständig-valide** — das ist außerdem exakt das
Muster, das die recherchierte Referenzimplementierung selbst in ihrem
Beispiel verwendet (von 125 Spalten sind dort nur 9 befüllt). Ein leeres
Feld ist laut Spezifikation ausdrücklich zulässig (keine Pflichtfelder
außer den in `booking.rb` explizit als `required: true` markierten:
Umsatz, Soll/Haben-Kennzeichen, Konto, Gegenkonto, Belegdatum).

**Befüllte Buchungssatz-Felder und ihre Quelle**:
| DATEV-Feld | Quelle |
|---|---|
| Umsatz (ohne Soll/Haben-Kz) | `abs(journal_lines.debit - journal_lines.credit)` (in unserem Modell ist je Zeile nur eine Seite ≠0, siehe unten) |
| Soll/Haben-Kennzeichen | `"S"` wenn `debit>0`, sonst `"H"` |
| Konto | `journal_lines.account_code`, linksbündig mit `0` auf `Sachkontenlänge` aufgefüllt |
| Gegenkonto (ohne BU-Schlüssel) | siehe eigener Abschnitt unten (Mehrzeiler-Problem) |
| Belegdatum | `journal_entries.entry_date`, Format TTMM (laut Spezifikation OHNE Jahr — das Jahr wird beim Import aus dem bebuchbaren Zeitraum abgeleitet) |
| Belegfeld 1 | `journal_entries.source_id`, falls gesetzt (z. B. Rechnungsnummer), sonst leer |
| Buchungstext | `journal_lines.memo`, falls gesetzt, sonst Fallback `journal_entries.description` |
| KOST1 – Kostenstelle | `cost_centers.code` der verknüpften `journal_lines.kostenstelle_id`, falls gesetzt (NICHT die interne UUID — DATEV erwartet die im Buchhaltungssystem sichtbare Kostenstellen-Nummer) |

Alle übrigen 117 Spalten bleiben leer (u. a. `WKZ Umsatz` — Buchungen sind in
diesem System durchgängig EUR, siehe `journal_entries.currency`-Default;
Fremdwährung ist außerhalb des aktuellen Scopes und würde `Kurs`/`WKZ Umsatz`
zusätzlich erfordern; `BU-Schlüssel` — DATEVs BU-Schlüssel-Vokabular für
Steuerautomatik ist ein eigenes, umfangreiches Regelwerk, das keine 1:1-
Entsprechung zu unseren `tax_codes` hat, Steuerbuchungen laufen in unserem
System bereits als eigene Buchungszeilen auf Steuerkonten (`1571`/`1576`/
`1771`/`1776`) statt über einen BU-Schlüssel — Zuordnung wäre Rätselraten,
bleibt leer).

**Gegenkonto bei Mehrzeilern (zentrale Design-Entscheidung)**: DATEVs
Buchungsstapel-Zeile ist strukturell für Zwei-Konten-Buchungen ausgelegt
(genau ein Konto, genau ein Gegenkonto pro Zeile). `JournalService.create`
erlaubt aber beliebig viele Zeilen pro Buchung (nur Summe Soll = Summe
Haben wird geprüft, siehe `journal.go:69`) — z. B. um Kostenstellen-Splits
abzubilden (ADR 0019: "eine einzelne Buchung kann mehrere Kostenstellen
gleichzeitig betreffen"). Drei Fälle:
- **1 Soll-Zeile : 1 Haben-Zeile** (Normalfall, deckt praktisch alle
  bisherigen Buchungsquellen ab — `bank`-Abgleich, künftig `APService`):
  trivial, Gegenkonto der einen Zeile = Konto der anderen.
- **1 Zeile auf einer Seite : N Zeilen auf der anderen** (z. B. eine
  Sammelrechnung auf ein Bankkonto gebucht, aber auf N Kostenstellen/
  Aufwandskonten aufgeteilt): jede der N Zeilen bekommt als Gegenkonto das
  eine Konto der Gegenseite — das ist die von DATEV selbst vorgesehene und
  unterstützte "Splittbuchung mit Sammelkonto"-Darstellung, technisch
  unproblematisch.
- **N Zeilen : M Zeilen (beide Seiten >1)**: KEIN eindeutiges Gegenkonto pro
  Zeile ableitbar, ohne ein Sammel-/Verrechnungskonto zu erfinden, das es in
  unserem Kontenrahmen nicht gibt. Entscheidung: **solche Buchungen werden
  beim Export mit einer expliziten Fehlermeldung übersprungen/abgelehnt**
  (welche Buchung, `journal_entries.id`/`entry_date` in der Fehlermeldung),
  NICHT stillschweigend mit einem geratenen Gegenkonto exportiert — ein
  falsches Gegenkonto in einer Buchhaltungsschnittstelle ist ein GoBD-
  relevanter Fehler, kein kosmetisches Problem. Betrifft nach aktuellem
  Codestand voraussichtlich keine einzige bestehende Buchungsquelle (alle
  bisherigen Aufrufer von `JournalService.Create`/`CreateTx` erzeugen 1:1-
  oder 1:N-Buchungen), ist aber durch die offene `JournalLineInput[]`-API
  theoretisch möglich und muss daher sauber behandelt statt ignoriert werden.

**Neue Konfigurationsfelder (Berater-/Mandantennummer etc.)**: DATEVs
Kopfzeile verlangt Werte, die in unserem Datenmodell aktuell nirgends
existieren: Beraternummer (`Berater`, Pflicht, ≥1001), Mandantennummer
(`Mandant`, Pflicht), Sachkontenlänge (`Sachkontenlänge`, Pflicht — unsere
Konten sind 4-stellig), Kontenrahmen (`SKR`, unsere Accounts sind SKR04),
Wirtschaftsjahresbeginn (`WJ-Beginn`, Pflicht). Diese sind mandantenweite,
mit dem Steuerberater abzustimmende Stammdaten — passen inhaltlich zu
`company_profiles` (dort liegen bereits `tax_no`/`vat_id`/Bankdaten, also
ebenfalls "mit Behörden/Dritten abzustimmende Stammdaten"), nicht zu einer
neuen eigenen Tabelle. Gewählt: additive Spalten auf `company_profiles`
(`datev_berater_nr`, `datev_mandant_nr` — beide nullable, Export schlägt mit
klarer Fehlermeldung fehl, solange nicht gepflegt; `datev_skr` NOT NULL
DEFAULT '04'; `datev_sachkontenlaenge` NOT NULL DEFAULT 4;
`datev_fiscal_year_start_month` NOT NULL DEFAULT 1 [Kalenderjahr, mit
Abstand häufigster deutscher Fall] — vereinfachend nur der Monat, der Tag
wird als der 1. angenommen; ein abweichender Wirtschaftsjahresbeginn
mitten im Monat ist in der Praxis extrem selten und wird hier bewusst NICHT
abgebildet, siehe `docs/open-questions.md`).

**Festschreibung**: Kopf- UND Zeilen-Feld `Festschreibung` wird IMMER `0`
(keine Festschreibung) gesetzt/leer gelassen. Unser Export soll keine
Festschreibungs-Entscheidung der Ziel-Buchhaltungssoftware erzwingen — das
ist eine bewusste Entscheidung des Anwenders/Steuerberaters beim Import,
unabhängig von unserem eigenen GoBD-Festschreibungsbegriff (Epic 0.3, der
sich auf UNSERE Belege bezieht, nicht auf DATEVs Buchungsstapel-Import).

**Herkunfts-Kennzeichen** (Kopf-Feld 8, 2-stellig): offiziell nur für bei
DATEV registrierte Softwarehäuser reserviert (wird beim Import ohnehin durch
`"SV"` ersetzt, siehe Feldkommentar in `header.rb`). Da NalaERP3 keine
registrierte Kennung hat, wird der auch in der Referenzimplementierung als
Platzhalter verwendete Wert `"RE"` übernommen — folgenlos, da überschrieben.

**Dateiname**: `EXTF_Buchungsstapel_<von:yyyyMMdd>_<bis:yyyyMMdd>.csv` —
DATEV schreibt keinen Dateinamen zwingend vor, empfiehlt aber ein
`EXTF_`-Präfix zur Wiedererkennung; Zeitraum im Namen für Nachvollziehbarkeit
ohne die Datei öffnen zu müssen.

**Endpunkt & Permission**: `GET /accounting/datev-export?von=YYYY-MM-DD&bis=YYYY-MM-DD`,
liefert `Content-Type: text/csv`, `Content-Disposition: attachment;
filename="..."`, Body Windows-1252-kodiert. Neue Permission `datev.export`
(analog zur E.1-Entscheidung für `cost_centers.*`/E.1-Vorgänger `bank.read`/
`bank.write`: keine bestehende Permission passt — Buchungsstapel-Export ist
eine eigene, sensible Fähigkeit [voller Ledger-Export], keine Wiederverwendung
von generischem `accounts`/`bank`-Lesezugriff), zugewiesen an `role-finance`
und `role-admin`.

## Entscheidung

- **Formatkategorie 21 "Buchungsstapel", Format-Version 13** (fix laut
  aufgabe.md), 31-Feld-Kopfzeile + 125-Spalten-Kopfzeile + Datenzeilen,
  Semikolon-getrennt, Windows-1252, `\r\n`, Komma als Dezimaltrennzeichen —
  Feldspezifikation recherchiert und gegen zwei unabhängige Quellen verifiziert
  (siehe Kontext).
- Nur die für unser Datenmodell sinnvollen Buchungssatz-Felder werden befüllt
  (Umsatz, Soll/Haben-Kennzeichen, Konto, Gegenkonto, Belegdatum, Belegfeld 1,
  Buchungstext, KOST1) — Rest bleibt spezifikationskonform leer.
- **Gegenkonto**: 1:1- und 1:N/N:1-Buchungen werden korrekt aufgelöst; echte
  N:M-Buchungen (mehrere Zeilen auf BEIDEN Seiten) werden mit expliziter
  Fehlermeldung vom Export ausgeschlossen statt geraten.
- **Neue additive Spalten auf `company_profiles`**: `datev_berater_nr`,
  `datev_mandant_nr` (beide nullable, Pflicht-Prüfung erst beim Export),
  `datev_skr` (Default `'04'`), `datev_sachkontenlaenge` (Default `4`),
  `datev_fiscal_year_start_month` (Default `1`).
- **Festschreibung immer `0`/leer**, **Herkunfts-Kennzeichen `"RE"`**
  (folgenlos, siehe oben).
- **`GET /accounting/datev-export?von=&bis=`**, neue Permission
  `datev.export` (Rollen `role-finance`/`role-admin`).
- Keine Schreiblogik, keine Änderung an `journal_entries`/`journal_lines`/
  `accounts`/`cost_centers` — rein lesender Export.

## Konsequenzen

- **E.3.2** (Folge-Subtask): additive Migration auf `company_profiles`
  (fünf neue Spalten, alle mit Default oder nullable, reversibel) + neue
  Permission `datev.export`.
- **E.3.3** (Folge-Subtask): Anwendungscode — CSV-Builder (Header-/Spalten-/
  Datenzeilen-Generierung inkl. Windows-1252-Encoding), DB-Abfrage
  (`journal_entries`/`journal_lines`/`accounts`/`cost_centers` im Zeitraum,
  Gegenkonto-Auflösung inkl. N:M-Ablehnung), HTTP-Wiring, Tests. Aufgrund der
  Größe (125-Spalten-Struct, Encoding-Logik, DB-Query, HTTP-Wiring, mehrere
  Testfälle inkl. N:M-Negativfall) wird bei Erreichen von E.3.3 voraussichtlich
  in Micro-Subtasks zerlegt (analog D.3.3/E.2.3).
- **Bekannte, bewusst nicht abgedeckte Fälle** (dokumentiert, kein Blocker):
  Fremdwährungsbuchungen (`WKZ Umsatz`/`Kurs` bleiben leer, alle Buchungen
  aktuell EUR), BU-Schlüssel-basierte Steuerautomatik (Steuerbuchungen laufen
  bereits als eigene Zeilen auf Steuerkonten), Wirtschaftsjahresbeginn an
  einem anderen Tag als dem 1. eines Monats, echte DATEV-Zertifizierung
  (nicht durchführbar in dieser Umgebung, siehe Kontext).
- Neue offene Frage vermerkt in `docs/open-questions.md`: exakter
  Wirtschaftsjahresbeginn-Tag (aktuell nur Monat konfigurierbar, Tag fix 1.).
- Kein Einfluss auf bestehende `journal_entries`/`journal_lines`/`accounts`/
  `cost_centers`/`company_profiles`-Daten oder deren Lesepfade (alle neuen
  Spalten sind additiv, mit Default oder nullable).
