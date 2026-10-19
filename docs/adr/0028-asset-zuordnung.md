# ADR 0028 — Asset-Zuordnung (Werkzeuge, IT, PSA)

Datum: 2026-10-15
Status: entschieden (vorgeschlagen in F.3.1; die vier offenen Punkte hat
der Nutzer am 2026-10-15 beantwortet, siehe "Entscheidungen des Nutzers")
Bezug: Backlog F.3 (Epic F — Personal & HR). Nach F.2 gemäß
Backlog-Reihenfolge (Entscheidung des Nutzers vom 2026-10-13).

## Kontext

`aufgabe.md` §1 nennt für Domäne F "Asset-Zuordnung"; der Backlog-Titel
präzisiert "Werkzeuge/IT/PSA". Die Vorgängerplanung in `codex.md`
(Feature 7.3) beschreibt Ausgabe und Rücknahme von Werkzeugen, IT und
PSA, Seriennummern, Zustand, Verantwortlichkeit und den Bezug zu Lager,
Einkauf und Kostenstellen.

**Bestand (belegt):** im Repo gibt es nichts dazu.
`docs/01-gap-analysis.md:96`: "kein Schema-Bezug gefunden (auch keine
generische Asset-Tabelle für Werkzeuge/IT/PSA)". Die Suche nach
`werkzeug`, `inventarnummer`, `seriennummer`, `betriebsmittel`,
`arbeitsmittel`, `psa`, `equipment` über Server und Client findet nur
andere Bedeutungen. **`project_assets` (`016_project_assets.sql`) sind
Projekt-DATEIEN** (GridFS-Verweise auf Zeichnungen und Bilder), keine
Betriebsmittel — der Name ist eine Verwechslungsfalle. Das Lager
(`materials`, `warehouses`, `stock_movements`, Chargen in `batches`)
führt Mengen von Material, keine einzeln identifizierbaren Gegenstände.

Vorhandene Bausteine: Mitarbeiterstamm über HTTP (F.4), Kostenstellen
(E.1), die Rechteprüfung "eigene mit self, fremde mit breitem Recht",
das Muster "unveränderlich, Korrektur über Storno" (F.1, F.2) und
"Fälligkeit beim Lesen berechnen, anmahnen" (F.2).

## Recherche: Prüfpflichten an Arbeitsmitteln

Per Websuche über **Sekundärquellen** geprüft (Quellen am Ende). Für
ein Metallbau-ERP relevant, weil Werkzeug, Leiter und Auffanggurt genau
die Gegenstände sind, die ausgegeben werden:

| Arbeitsmittel | Befund | Typischer Wert |
|---|---|---|
| ortsveränderliche elektrische Betriebsmittel (Bohrmaschine, Flex, Kabeltrommel) | DGUV Vorschrift 3 Anhang 2 nennt Richtwerte; Fristen legt der Arbeitgeber nach Gefährdungsbeurteilung fest (BetrSichV) | 6 Monate, auf Baustellen 3 Monate |
| Leitern und Tritte | wiederkehrende Prüfung durch befähigte Person (§14 Abs. 2 BetrSichV), Richtwert jährlich (DGUV Information 208-016); Sichtkontrolle vor jeder Benutzung | 12 Monate |
| PSA gegen Absturz (Auffanggurt, Verbindungsmittel) | mindestens jährlich durch Sachkundigen (DGUV Regel 112-198, Grundsatz 312-906); ohne gültigen Prüfnachweis keine Verwendung; nach einem Sturz sofort aussondern | 12 Monate |

Daraus folgt: wer ausgibt, muss wissen, ob der Gegenstand **gerade
geprüft** ist. Eine Asset-Verwaltung ohne Prüffristen würde ausgerechnet
die Frage offenlassen, die im Schadensfall gestellt wird. Die
Qualifikation des Prüfers (z. B. "befähigte Person Leitern") gibt es
seit F.2 im Katalog — ADR 0027 hat die Prüfung der Arbeitsmittel selbst
ausdrücklich an F.3 verwiesen.

## Optionen und Entscheidungen (vorbehaltlich der offenen Punkte)

### 1. Was ist ein Asset?

**A — Mengen über das Lager** (wie Material). Verworfen für den Kern:
"Bohrmaschine Nr. 17 hat Herr X" und "Bohrmaschine Nr. 17 ist bis
März geprüft" sind Aussagen über ein **einzelnes Stück**; das Lager
kennt nur Mengen.

**B — einzeln identifizierbare Gegenstände mit Inventarnummer.**
Vorgeschlagen. Ein Asset ist ein Stück mit Inventarnummer (eindeutig je
Mandant), Bezeichnung, Kategorie, Seriennummer, Hersteller, Modell,
Anschaffungsdatum und -preis, optionaler Kostenstelle (E.1), Standort
(Freitext), Prüfintervall und Status.

**Verbrauchs-PSA** (Handschuhe, Schutzbrillen, Gehörschutz,
Arbeitsschuhe in Größen) passt nicht in B: sie wird nicht
zurückgegeben und hat keine Inventarnummer. Vorschlag: nicht Teil von
F.3 (→ offene Frage 2).

### 2. Kategorie

Feste Liste: `werkzeug`, `elektrowerkzeug`, `messmittel`, `leiter`,
`psa`, `it`, `maschine`, `sonstiges`. Sie dient der Gruppierung und dem
Vorschlag eines Prüfintervalls, nicht der Fachlogik. Anders als bei den
Qualifikationen (ADR 0027) **kein mandanteneigener Katalog**: die
Kategorie ist grob, und "erweiterbar" ist hier über `sonstiges` plus
Bezeichnung abgedeckt. Fahrzeuge gehören zum Fuhrpark (Epic G) und sind
ausgenommen.

### 3. Ausgabe und Rücknahme

Eine **Ausgabe** hält fest: Asset, Mitarbeiter, ausgegeben am,
zurückgegeben am (leer, solange er es hat), Zustand bei Rückgabe,
Notiz, wer ausgegeben und wer zurückgenommen hat.

Muster wie die Zeiteinträge (ADR 0024): eine Ausgabe wird nicht
geändert und nicht gelöscht; erlaubt sind genau zwei einmalige
Übergänge — Rückgabe und Storno (mit Begründung). Die Historie ("wer
hatte die Leiter am Unfalltag?") bleibt vollständig. Die Datenbank
garantiert **höchstens eine offene Ausgabe je Asset** (partieller
eindeutiger Index), nicht nur der Code.

Ausgegeben wird an **Mitarbeiter**. Ausgabe an ein Projekt, eine
Baustelle oder ein Fahrzeug ist nicht Teil von F.3 (Epics G/H).

### 4. Prüfungen (→ offene Frage 1)

Eine **Prüfung** hält fest: Asset, geprüft am, Ergebnis (`bestanden`,
`maengel`, `nicht_bestanden`), nächste Prüfung fällig am (angegeben
oder aus dem Prüfintervall des Assets berechnet und **gespeichert**,
wie `gueltig_bis` in ADR 0027), Prüfer (Name; optional ein
Mitarbeiter), Notiz. Unveränderlich, Korrektur über Storno.

Fälligkeit wird beim Lesen berechnet, nach dem Muster der
Qualifikationen: `geprueft`, `faellig_bald`, `ueberfaellig`, `nie_geprueft`
(für Assets mit Prüfintervall ohne Prüfung) und **`gesperrt`** für ein
Asset, dessen letzte Prüfung `nicht_bestanden` ergab. Eine
betriebsweite Liste mahnt an, wie in F.2.

**Ausgabe eines gesperrten oder überfälligen Assets:** Vorschlag —
ablehnen bei `gesperrt`, Hinweis (nicht ablehnen) bei `ueberfaellig`.
Begründung: ein durchgefallener Auffanggurt darf nicht verwendet
werden; eine um zwei Tage überfällige Prüfung soll den Betrieb nicht
lahmlegen, aber sichtbar sein. (→ offene Frage 1)

**Ergänzung 2026-10-19 (F.3.7):** auch das Ergebnis `maengel` sperrt
(Entscheidung des Nutzers: "Mängel sollen Ausgabe sperren") — bis zur
nächsten bestandenen Prüfung, unabhängig von einem Folgetermin. Die
Ablehnung nennt den Grund ("… letzte Prüfung mit Mängeln"). Die
Vorwarnzeit von 30 Tagen hat der Nutzer bestätigt.

### 5. Ausmusterung

Ein Asset wird nie gelöscht (Ausgaben und Prüfungen verweisen darauf),
sondern **ausgemustert** (Datum, Grund). Ein ausgemustertes Asset kann
nicht ausgegeben werden; eine offene Ausgabe muss vorher zurückgegeben
sein.

### 6. Rechte (→ offene Frage 3)

Neue Permissions nach dem bekannten Muster:

- `assets.self` — die eigenen ausgegebenen Assets sehen; alle
  Systemrollen.
- `assets.read` — alle Assets, Ausgaben, Prüfungen, Fälligkeitsliste.
- `assets.write` — Assets pflegen, ausgeben, zurücknehmen, Prüfungen
  erfassen, stornieren.

~~Vorschlag für `read`/`write`: `role-hr`, `role-inventory` und
`role-admin`.~~ **Entschieden: `read`/`write` nur an `role-hr`** (und
`role-admin` wie in allen HR-Migrationen; er hat ohnehin
`admin.superuser`). Das Lager kann später aufgenommen werden; das ist
eine Zuordnung in `role_permissions`. Eine Oberfläche, um Rechte von
Rollen zu ändern, gibt es bisher nicht (nur Rollen je Benutzer), also
geht das über eine kleine Migration.

### 7. Ort im Code

Neues Paket `internal/assets` statt `hr`: Werkzeug und Leiter sind
nicht nur Personalthema; Fuhrpark (G) und Produktion (H) werden darauf
aufsetzen. Der Bezug zum Mitarbeiter läuft über `hr_employees` in SQL
und über die vorhandene Auflösung Benutzer→Mitarbeiter.

## Offen — Freigabe des Nutzers nötig vor F.3.2 (erledigt, siehe unten)

1. **Prüfungen in F.3?** Vorschlag: ja, als eigene Subtasks nach
   Stamm und Ausgabe, mit Anmahnen wie bei den Qualifikationen; dazu
   "gesperrt" bei nicht bestandener Prüfung (Ausgabe abgelehnt) und
   Hinweis bei überfälliger Prüfung (Ausgabe erlaubt).
2. **Verbrauchs-PSA** (Handschuhe, Brillen, Schuhe — ohne Rückgabe):
   Vorschlag: nicht in F.3; F.3 führt nur Einzelstücke mit
   Inventarnummer (dazu zählt PSA wie der Auffanggurt). Verbrauchs-PSA
   später über das Lager mit Bezug zum Empfänger (neue
   Backlog-Position).
3. **Wer gibt aus?** Vorschlag: Personalabteilung, Lager und
   Administrator (`role-hr`, `role-inventory`, `role-admin`).
4. **Empfangsbestätigung:** Vorschlag analog F.2 — die ausgebende Stelle
   erfasst, der Mitarbeiter bestätigt im System nicht; eine
   unterschriebene Ausgabeliste bleibt Papier bzw. später ein
   hochgeladenes Dokument.

## Entscheidungen des Nutzers (2026-10-15)

1. Prüfungen: "Ja, soll Teil von F.3 sein" — wie vorgeschlagen,
   einschließlich Sperre bei `nicht_bestanden` und Hinweis bei
   überfälliger Prüfung.
2. Verbrauchs-PSA: "Vorschlag umsetzen" — nicht in F.3; eigener
   Backlog-Eintrag F.12 (Lager mit Bezug zum Empfänger).
3. Wer gibt aus: "Erstmal nur Personalabteilung, später eventuell dem
   Lager das Recht aber einräumen können." — siehe Abschnitt 6.
4. Empfangsbestätigung: "Nein, keine Bestätigung vorerst."

**Abweichung von Abschnitt 1 bei der Umsetzung (F.3.2):** der Status
eines Assets wird **nicht gespeichert**, sondern abgeleitet —
"ausgegeben" aus einer offenen Ausgabe, "ausgemustert" aus
`ausgemustert_am`, "gesperrt"/"überfällig" aus den Prüfungen. Eine
gespeicherte Kopie könnte auseinanderlaufen. "In Reparatur" war nicht
gefordert und entfällt; bei Bedarf als eigene Erweiterung.

## Konsequenzen

- **F.3.2** Migration `089_assets.sql`: `assets`, `asset_assignments`,
  `asset_inspections`, drei Permissions.
- **F.3.3** Asset-Stamm: Service (anlegen, ändern, ausmustern, lesen).
- **F.3.4** Ausgabe und Rücknahme: Service.
- **F.3.5** Prüfungen und Fälligkeit: Fachregeln ohne DB, Service,
  betriebsweite Liste.
- **F.3.6** HTTP-Anbindung (voraussichtlich zu teilen).

## Quellen der Recherche

- DGUV Vorschrift 3, Prüffristen: <https://forum-verlag.com/fachwissen/elektrosicherheit-und-elektrotechnik/prueffristen-nach-dguv-v3/>,
  <https://industrie-fachwissen.de/werkstatt/dguv-v3-pruefung-ortsveraenderliche-betriebsmittel-fristen-richtwerte-kosten>,
  <https://www.komnet.nrw.de/_sitetools/dialog/17791>
- PSA gegen Absturz: <https://www.absturzsicherung.de/en/fall-arrest-manual/dguv/dguv-rule-112-198.html>,
  <https://vorschriften.bgn-branchenwissen.de/daten/dguv/208_032/5.htm>,
  <https://www.bgbau.de/fileadmin/Medien-Objekte/Medien/Bausteine/228_Betriebsanweisungen_Arbeitsmittel_Auffangsystem/ba_auffangsystemabsturz.doc>
- Leitern: <https://vorschriften.bgn-branchenwissen.de/daten/tr/trbs2121_2/5.htm>,
  <https://cdn.vbg.de/media/d0c98190242d49a384a1fe7c27afad8b/dld:attachment/fi_pruefempf_table_puv.pdf>,
  <https://www.komnet.nrw.de/_sitetools/dialog/12373>
