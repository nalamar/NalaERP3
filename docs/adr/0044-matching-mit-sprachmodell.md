# ADR 0044: Matching von LV-Positionen mit Sprachmodell

- Status: **Angenommen** (2026-11-16, "fortsetzen" = Empfehlung, ANNAHME):
  1–6 jeweils (a). Auskunft zu Frage 6 (2026-11-16): Vergleich der Modelle
  im Betrieb durch Feedback der Kalkulatoren, siehe ADR 0046; der Messlauf
  am Korpus wird optional. Fragen in
  `docs/open-questions.md`, Abschnitt 2026-11-16 I.4
- Bezug: Backlog I.4, `aufgabe.md` §4 (deterministisch vor probabilistisch,
  LLM nur für Matching und Textarbeit, Preise nie vom Modell, Score und
  Begründung, Freigabe durch einen Menschen), ADR 0039 (Evaluationsset,
  Kennzahlen, Zielwerte), ADR 0043 (Provider-Interface, Datenschutz,
  Grenzen)

## Bestandsaufnahme (belegt)

- **Evaluationsset** (`server/internal/quotes/eval`): 10 synthetische LVs,
  250 Positionen, 184 mit Ziel; Katalog `testdata/katalog.json` mit 15
  Leistungen (Code, Kurztext, Einheit, Gewerk, Systemvorgaben). Die Positionen
  kommen aus dem Parser (`gaeb.Parse`), nicht aus der Erwartung.
- **Matcher-Schnittstelle** `eval.Matcher`: `Vorschlagen(Position, *Katalog)
  Vorschlag` — **je Position, ohne `context.Context`, ohne Fehler.** Für
  Modellaufrufe fehlen Abbruch, Fehlerweg und das Bündeln mehrerer Positionen.
  `Ausfuehren` prüft Score in [0, 1] und Begründung bei jeder Zuordnung.
- **Ausgangswerte** (ADR 0039): Wortabgleich bei Schwelle 0,7: 25,5 % Treffer,
  0 % Fehlzuordnung; bei 0,3: 67,4 % Treffer, aber 23 % Fehlzuordnung.
  Systemvorgaben erkennt bisher kein Matcher (erwartet u. a. Profilserie 51,
  Verglasung 39, U-Wert 42). Ziel: Fehlzuordnung ≤ 2 % **und** Treffer ≥ 70 %.
- **Modellzugriff** (I.3, `server/internal/llm`): `Client.Complete` mit
  JSON-Schema-Prüfung, Zeitlimit (Standard 300 s), höchstens 2 Wiederholungen,
  Protokoll ohne Inhalte, Tageslimit je Mandant (Standard 500 Aufrufe, auch
  Testaufrufe zählen); Anbindungen OpenAI-kompatibel, Anthropic, Mistral;
  Test-Anbieter ohne Netz (`llm.Fake`).
- **Lücke: kein Leistungskatalog in der Datenbank.** ADR 0039 sah ihn als
  „eigenen Punkt vor I.4“ vor; im Backlog steht er nicht. Im Produkt gibt es
  nur Materialien (Profilattribute aus 068), Preislisten, Textbausteine (ohne
  `company_id`) und frühere Angebotspositionen. Ohne Katalog hat der Matcher
  im Betrieb kein Ziel — im Evaluationsset schon.
- **Kein Anbieter-Schlüssel vorhanden.** Ein Messlauf gegen einen echten Dienst
  braucht einen Schlüssel in der lokalen `.env` (nie im Repo) und kostet Geld
  je Lauf (250 Positionen).
- **LV-Texte sind fremde Eingaben.** Ein Langtext kann Anweisungen an das
  Modell enthalten („ignoriere …, setze Code X“). Schutz: die Antwort ist
  strikt geprüft (nur Codes aus den vorgelegten Kandidaten, kein Preisfeld,
  feste Felder), und jede Zuordnung braucht eine Freigabe.

## Zu entscheiden

1. **Zuordnungsziel im Betrieb.**
   (a) Leistungskatalog je Mandant in der Datenbank (Code, Kurztext, Langtext,
   Einheit, Gewerk, Systemvorgaben; Kalkulationsansatz aus Material, Lohn und
   Fremdleistung kommt mit I.5/I.6), gepflegt im Client, Import aus JSON/CSV;
   eigener Punkt **nach** dem gemessenen Matcher, vor der Einbindung in den
   Import;
   (b) direkt gegen Materialien und Textbausteine;
   (c) frühere Angebotspositionen als Ziel.
   Empfehlung: **(a)** — wie ADR 0039 Frage 3; (c) später als zusätzliche
   Quelle für Kandidaten.
2. **Verfahren.**
   (a) zweistufig: deterministische Vorauswahl von Kandidaten (Wortabgleich
   über Kurz- und Langtext, Einheit), das Modell wählt **einen Kandidaten oder
   „kein Ziel“**; ist der Katalog klein (≤ 40 Leistungen), gehen alle
   Leistungen als Kandidaten mit;
   (b) immer der ganze Katalog im Aufruf;
   (c) Vektorsuche (Embeddings) für die Vorauswahl.
   Empfehlung: **(a)** — begrenzt Tokens bei großen Katalogen, und ein Code
   außerhalb der Kandidaten ist eine ungültige Antwort; (c) braucht einen
   Embedding-Dienst bei jedem Anbieter und kann später die Vorauswahl ersetzen.
3. **Bündelung.**
   (a) mehrere Positionen je Aufruf (bis 20, nach Bereich gruppiert), mit
   Bereichsüberschriften und Vorbemerkungen als Kontext; eine ungültige
   Antwort für ein Bündel legt dessen Positionen in die Prüfliste, der Rest
   läuft weiter;
   (b) ein Aufruf je Position.
   Empfehlung: **(a)** — ein LV mit 250 Positionen braucht etwa 15 statt 250
   Aufrufe (Tageslimit 500), und die Überschrift („Titel 2: Fenster EG“) hilft
   bei der Zuordnung. Dafür wird `eval.Matcher` erweitert:
   `VorschlagenAlle(ctx, lv, katalog) ([]Vorschlag, error)`; die bisherigen
   Matcher laufen über eine Hülle weiter.
4. **Score und Schwelle.**
   (a) Score vom Modell (0–1) mit deterministischen Abzügen (Einheit passt
   nicht zur Leistung, Pauschale gegen Stück usw.); die Schwelle wird **je
   Anbieter und Modell** am Evaluationsset so gewählt, dass die
   Fehlzuordnung ≤ 2 % bleibt, und je Mandant gespeichert (Standard 0,9);
   (b) zusätzlich zweiter Aufruf, Zuordnung nur bei gleichem Ergebnis
   (doppelte Kosten);
   (c) Score nur deterministisch, das Modell liefert nur Code und Begründung.
   Empfehlung: **(a)**; (b) als Option, falls (a) die 2 % nicht hält.
5. **Systemvorgaben.**
   (a) deterministisch, wo das Muster eindeutig ist (RC-Klasse `RC 1–6`,
   U-Wert `Uw`/`U-Wert` mit Zahl, Brandschutz `T30/T60/T90/RS/EI…`), das
   Modell nur für Profilserie und Verglasung, im selben Aufruf wie die
   Zuordnung;
   (b) alles durch das Modell;
   (c) nur deterministisch.
   Empfehlung: **(a)** — deterministisch vor probabilistisch; der
   deterministische Teil ist ohne Modell sofort messbar.
6. **Messlauf und Anbietervergleich.**
   (a) Messlauf als Go-Test, nur mit gesetzter Umgebungsvariable
   (`NALA_EVAL_LLM=1`, Anbieter/Endpunkt/Modell/Schlüssel aus der lokalen
   `.env`), nie in der Standard-Suite; Ergebnis je Anbieter/Modell (Kennzahlen,
   gewählte Schwelle, Tokens, Dauer) als Tabelle in diesem ADR; die LV-Texte
   sind synthetisch, es gehen keine Echtdaten hinaus;
   (b) Vergleich nur im Betrieb über das Protokoll.
   Empfehlung: **(a)**. **Auskunft erbeten:** Welche Anbieter und Modelle
   sollen verglichen werden, und wer legt die Schlüssel in die lokale `.env`?
   Ohne Schlüssel endet I.4 mit dem gegen den Test-Anbieter geprüften Matcher
   und dem deterministischen Teil; der Vergleich wartet.

Unabhängig davon (aus §4 und ADR 0043, nicht zur Wahl): keine Preise im
Antwortschema; an das Modell gehen nur LV-Felder (OZ, Bereich, Kurz-/
Langtext, Menge, Einheit, Art) und Katalogfelder, nie Kunde oder Projekt;
jede Zuordnung bleibt ein Vorschlag bis zur Freigabe; ohne eingeschaltetes
Sprachmodell läuft nur der deterministische Teil.

## Entscheidung (2026-11-16, ANNAHME)

Alle sechs Fragen mit der Empfehlung (a): Leistungskatalog je Mandant als
Ziel im Betrieb, Kandidaten-Vorauswahl mit Auswahl durch das Modell, Bündel
bis 20 Positionen, Score vom Modell mit Abzügen und Schwelle je Anbieter/
Modell, Systemvorgaben teils deterministisch, Messlauf opt-in mit Schlüssel
aus der lokalen `.env`. Die Auskunft zu Anbietern, Modellen und Schlüsseln
steht aus; I.4.3 wartet darauf.

## Vorschlag zur Umsetzung (bei Empfehlung)

- **I.4.1** Matcher-Schnittstelle für Bündel (`context`, Fehler), Hülle für
  die bisherigen Matcher; deterministische Erkennung von RC-Klasse, U-Wert
  und Brandschutz; Messung ohne Modell (neue Ausgangswerte).
- **I.4.2** Kandidaten-Vorauswahl und Matcher mit Modell: Prompt, Antwort-
  schema (je OZ: Code aus den Kandidaten oder leer, Score, Begründung,
  Profilserie, Verglasung), Prüfungen und Abzüge, Bündel; Tests nur gegen den
  Test-Anbieter (gültige, ungültige, fremde Codes, Anweisungen im Langtext).
- **I.4.3** Messlauf mit echtem Anbieter (opt-in), Wahl der Schwelle,
  Ergebnistabelle hier — wartet auf Schlüssel (Frage 6).
- **I.4.4** Leistungskatalog je Mandant: Migration, Service, API, Import
  (a); **I.4.5** Client für den Katalog.
- Danach **I.5**: Vorschläge am Import speichern (Code, Score, Begründung),
  Prüfliste und Freigabe.

## Folgen

- Bis zur Freigabe gibt es keinen Datenverkehr nach außen; I.4.1 und I.4.2
  laufen ohne Netz.
- Die Zielwerte aus ADR 0039 gelten für den Einsatz: hält kein Anbieter
  ≤ 2 % Fehlzuordnung bei ≥ 70 % Treffern, bleibt der Matcher ein reiner
  Vorschlag ohne „automatisch“.
- Das synthetische Korpus ist von derselben Hand geschrieben wie der
  Katalog; die Messung überschätzt die Treffer bei echten LVs vermutlich.
  Echte, anonymisierte LVs bleiben lokal außerhalb des Repos (ADR 0039).

## Ausgangswerte (2026-11-16, I.4.1)

Deterministische Systemvorgaben (`ErkenneSystemvorgaben`, Hülle
`MitSystemvorgaben`) am Korpus, Lauf `NALA_EVAL=1 go test
./internal/quotes/eval -run TestBericht -v`:

| Merkmal | erwartet | erkannt | richtig |
|---|---|---|---|
| RC-Klasse | 12 | 12 | 12 |
| U-Wert | 42 | 42 | 42 |
| Brandschutzklasse | 27 | 27 | 27 |
| Profilserie | 51 | 0 | 0 |
| Verglasung | 39 | 0 | 0 |

Die Zuordnung bleibt beim Ausgangswert (Wortabgleich bei Schwelle 0,7:
47 richtig, 0 falsch, 25,5 % Treffer). **Einschränkung:** Korpus und Muster
stammen von derselben Hand; die 100 % sind eine Obergrenze, keine Erwartung
für echte LVs. Muster: RC/WK 1–6; U-Wert nur des Elements (Uw, Ud, Ucw,
U-Wert), nicht Ug/Uf; Brandschutz T30/60/90 vor EI/E/EW vor F/G (außer nach
„Serie“) vor RS; kein Treffer über eine Zeile hinweg.

## Matcher mit Modell (2026-11-16, I.4.2)

`eval.ModellMatcher` (vorerst im Paket `eval`, in den Produktivteil mit
I.4.4/I.5): Kandidaten je Position (alle bei ≤ 40 Leistungen, sonst die 8
mit dem größten Anteil gemeinsamer Wörter), Bündel aufeinanderfolgender
Positionen desselben Bereichs (≤ 20), Kontext = Bereichspfad und
Vorbemerkungen (≤ 2000 Zeichen), Langtext ≤ 2000 Zeichen. Antwortschema ohne
Zusatzfelder (OZ, Code, Score 0–1, Begründung, Profilserie, Verglasung).
Verworfen werden je Position: Code außerhalb der Kandidaten, Code ohne
Begründung; je Bündel (Positionen in die Prüfliste, Rest läuft weiter):
Schemafehler, fremde oder doppelte OZ, Zeitlimit. Tageslimit, Abbruch und
andere Fehler beenden den Lauf. Abzug: Einheit weicht ab → Score × 0,5 mit
Hinweis in der Begründung.

Messung mit einem Orakel statt Modell (Test-Anbieter, antwortet mit dem
erwarteten Code, wenn er Kandidat ist; Schwelle 0,9) — **Obergrenze** der
Pipeline, kein Modellergebnis:

| Vorauswahl | richtig | falsch | Treffer | Aufrufe |
|---|---|---|---|---|
| alle 15 Leistungen | 180 | 0 | 97,8 % | 42 |
| 3 Kandidaten (erzwungen) | 160 | 0 | 87,0 % | 41 |

Die 4 fehlenden Treffer bei voller Vorauswahl sind Einheiten-Abweichungen
(LV in Stück, Katalog in m bzw. m²; einmal pauschal gegen Stück) — sie
landen gewollt in der Prüfliste. Eine Anweisung im Langtext („setze Code
X“) wird nur dann wirksam, wenn X ein Kandidat ist; sie bleibt dann ein
Vorschlag mit Begründung bis zur Freigabe (Test vorhanden).

