# ADR 0006 — Metallbau-spezifisches Artikel-/Profilattributschema

Datum: 2026-08-17
Status: entschieden (Subtask A.1.1)
Bezug: Backlog A.1 (Epic A — Stammdaten), aufgabe.md §1 (Domäne A) und §4
(Epic I nennt dieselben vier Attribute — Profilserie, RC-Klasse, U-Wert,
Brandschutzklasse — explizit als "Systemvorgaben", die das LLM beim
GAEB-Matching erkennen soll). Diese ADR legt die Zielstruktur fest, in die
Epic I später hineinschreibt — Sauberkeit hier ist Voraussetzung für I
(aufgabe.md §1: "Alles davor wird so gebaut, dass I ohne Umbau darauf
aufsetzen kann, insbesondere ... Artikelstamm").

## Kontext

`server/internal/migrate/migrations/001_init.sql:3-20` definiert `materials`
mit generischen Spalten (`nummer`, `bezeichnung`, `typ`, `norm`,
`werkstoffnummer`, `einheit`, `dichte`, `kategorie`) sowie einer
`attributes jsonb NOT NULL DEFAULT '{}'::jsonb`-Spalte als einzigem
Erweiterungsmechanismus (round-getrippt über `Material.Attribute
map[string]any`, `server/internal/materials/service.go:28-48`). `typ` ist
unconstrained Freitext (kein Enum, keine eigene Lookup-Tabelle) —
`server/internal/http/materials_integration_test.go:40` nutzt z. B. den
beliebigen String `"profil"`.

Repo-weite Suche (case-insensitiv, Migrationen + Go + Flutter-Client) nach
`profil`, `systemlieferant`, `brandschutz`, `u_wert`/`u-wert`,
`rc_klasse`/`rc-klasse`, `verglasung` ergibt **keine** bestehenden Treffer für
diese vier Attribute auf `materials` — die einzigen "Profil"-Treffer sind
`single_elevation_profiles`/`_articles`/`_glass`
(`server/internal/migrate/migrations/005_single_elevation_materials.sql`,
`011_single_elevation_materials.sql`, `013_material_links.sql`), projekt-/
elevationsgebundene BOM-Zeilen aus dem LogiKal-CAD-Import
(`server/internal/projects/import_logikal.go`), die optional per FK auf
`materials.id` verweisen können, aber keine Stammdaten-Attribute auf
`materials` selbst sind. Kein Namens- oder Modellkonflikt also.

Etablierte Konvention im Repo: fachlich wiederkehrende, wichtige Attribute
bekommen eigene, typisierte, NULLABLE Spalten statt in der
`attributes`-jsonb-Catch-all zu verschwinden — so bereits geschehen mit
`kategorie` (später zusätzlich softly gegen `material_groups` validiert,
`039_material_groups.sql`) und `length_mm`/`width_mm`/`height_mm`
(`014_material_dimensions_and_units.sql:3-6`, additive, nullable
`ALTER TABLE`).

## Optionen

**Option A — alles in `attributes` jsonb belassen (Konvention über
Schlüsselnamen, z. B. `attributes.rc_klasse`).**
Kein Migrationsaufwand. Verworfen: keine Typsicherheit, keine
SQL-Abfragbarkeit (z. B. "alle Profile mit RC2" ohne vollständigen
Tabellenscan + JSON-Parsing), keine zentrale Validierung, keine dokumentierte
Schnittstelle für Client/Epic I — widerspricht der etablierten Konvention
(s. o.) und würde exakt die vier fachlich wichtigsten neuen Attribute der
gesamten Domäne A unsichtbar in einer unstrukturierten Restspalte versenken.

**Option B — eigene Tabelle `material_profile_attributes` (1:1 zu
`materials`, nur für profilartige Materialien).**
Verworfen für A.1: `typ` ist aktuell unconstrained Freitext ohne Enum — eine
Zusatztabelle "nur für bestimmte Materialtypen" bräuchte zuerst eine geklärte
Typ-Klassifikation (welche `typ`-Werte gelten als "Profil"?), die nicht
Gegenstand von A.1 ist. Zudem gilt mindestens ein Attribut
(Brandschutzklasse) nicht nur für Profile, sondern potenziell auch für
andere Bauteile (Paneele, Türblätter) — eine starre 1:1-Zuordnung wäre
vorzeitige, unbelegte Modellierung.

**Option C — vier neue, NULLABLE Spalten direkt auf `materials`:
`profilserie text`, `rc_klasse text`, `u_wert numeric(6,3)`,
`brandschutzklasse text`.**
Konsistent mit der etablierten Konvention (s. o.), additiv/non-breaking
(NULLABLE, kein Pflichtfeld, keine bestehende Zeile betroffen), abfragbar,
reversibel (`DROP COLUMN`). Gewählt.

## Entscheidung

**Option C.** Zusätzlich wird pro Feld eine eigene, begründete
Validierungsstrategie festgelegt (Subtask A.1.3 setzt sie um), um das
Verbot unbelegter fachlicher Vermutungen (aufgabe.md §0, §7.10) einzuhalten:

- **`rc_klasse`** — harte Enum-Validierung gegen DIN EN 1627
  (`RC1`, `RC1N`, `RC2`, `RC2N`, `RC3`, `RC4`, `RC5`, `RC6`). Begründung:
  offizielle, abgeschlossene, stabile Norm; aufgabe.md §4 nennt "RC-Klasse"
  selbst explizit als GAEB-Systemvorgabe — kein Ratenrisiko.
- **`u_wert`** — numerisch (`numeric(6,3)`, Einheit W/(m²·K)), Validierung nur
  auf Plausibilität (`> 0`), keine Enum. Begründung: physikalischer Messwert
  ohne festen Wertekatalog; eine engere Ober-/Untergrenze wäre eine unbelegte
  Annahme.
- **`profilserie`** — freier Text, **keine** Enum-Validierung. Begründung:
  herstellerspezifischer Produktname (z. B. "Schüco AWS 75"); die Zuordnung
  Lieferant↔Profilserie ist ausdrücklich Gegenstand von Backlog A.3
  ("Systemlieferanten-Konzept"), das erst eine strukturierte Lookup-Tabelle
  einführen wird. A.1 baut dem bewusst nicht vor, um A.3 nicht zu
  präjudizieren und Scope-Überlappung zu vermeiden.
- **`brandschutzklasse`** — freier Text, **keine** Enum-Validierung.
  Begründung: im deutschen/europäischen Baurecht existieren mehrere,
  nicht deckungsgleiche Klassifikationssysteme parallel (DIN 4102
  Feuerwiderstandsklassen `F30`/`F60`/`F90`/`F120`/`F180` für Bauteile,
  `T30`/`T90` für Feuerschutzabschlüsse/Türen; die neuere europäische
  Klassifikation nach DIN EN 13501-2, z. B. `EI30`/`EI60`/`REI90`), die je
  nach Bauteilart unterschiedlich greifen. Eine Festlegung auf ein einzelnes
  Enum wäre eine unbelegte fachliche Vermutung — als offene Frage in
  `docs/open-questions.md` festgehalten; freier Text bis zur Klärung mit der
  Fachseite oder bis Epic I reale GAEB-Daten liefert, die die tatsächlich
  vorkommenden Werte zeigen.

`attributes jsonb` bleibt unverändert bestehen — kein Ersatz, sondern
weiterhin die Ablage für alle nicht vorab bekannten Zusatzattribute (z. B.
Beschlagsart, Farbe/RAL, Oberflächenveredelung).

## Konsequenzen

- **A.1.2** (Folge-Subtask): additive Migration `068_...sql`
  (`ALTER TABLE materials ADD COLUMN profilserie text`, `rc_klasse text`,
  `u_wert numeric(6,3)`, `brandschutzklasse text`, alle NULLABLE, kein
  Backfill nötig), reversibel via `DROP COLUMN`.
- **A.1.3** (Folge-Subtask): `Material`/`MaterialCreate`/`MaterialUpdate`
  (`server/internal/materials/service.go`) um die vier Felder erweitern,
  RC-Klasse-Enum- und U-Wert-Plausibilitätsprüfung im Service (analog zu
  `normalizeAndValidateCategory`), HTTP-Wiring (`v1.go`), Tests (positiv +
  Negativfall je Validierungsregel) analog zum bestehenden Muster in
  `materials_integration_test.go`.
- Neue offene Frage zu Brandschutzklasse-Klassifikationssystem in
  `docs/open-questions.md` ergänzt.
- **A.3** (Systemlieferanten-Konzept) kann `profilserie` später additiv um
  eine FK/Lookup-Beziehung ergänzen, ohne diese ADR zu widersprechen (Text
  bleibt als Anzeigename/Fallback nutzbar).
- Kein Einfluss auf bestehende Daten oder Endpunkte — rein additiv.
