# ADR 0046: Modellprofile, Feedback der Kalkulatoren, Scoring und Lernen aus Korrekturen

- Status: **Angenommen** (2026-11-17, "fortsetzen" = Empfehlung, ANNAHME):
  1–7 jeweils (a). Fragen in `docs/open-questions.md`, Abschnitt
  2026-11-16 I.8
- Bezug: Auskunft des Nutzers zu ADR 0044 Frage 6 (2026-11-16), ADR 0043
  (Modellzugriff, Datenschutz, Grenzen), ADR 0044 (Matching), ADR 0045
  (Vorschläge, Prüfliste, Freigabe), `aufgabe.md` §4

## Auskunft des Nutzers (2026-11-16, sinngemäß)

Die Modelle werden nicht vorab im Labor verglichen, sondern im Betrieb durch
die Kalkulatoren: Sind mehrere Modelle hinterlegt, wählen sie, mit welchem sie
ein Angebot erarbeiten, und melden zurück, wie viel sie nacharbeiten mussten
und wo das System falsch lag. Aus diesem Wissen soll das System dazulernen
(eventuell über RAG), und es entsteht ein Scoring, welches Modell besser
arbeitet.

## Bestandsaufnahme (belegt)

- **Ein Modell je Mandant:** `company_llm_settings` hat `company_id` als
  Primärschlüssel (Migration 119): ein Anbieter, ein Endpunkt, ein Modell,
  ein Schlüssel, Zeitlimit, Tageslimit, Schalter `aktiv`. Seite
  „Sprachmodell“ im Client pflegt genau diese Zeile.
- **Tageslimit** zählt je Mandant und Tag (`company_llm_usage`), unabhängig
  vom Modell.
- **ADR 0045** speichert je Vorschlag schon Quelle, Anbieter und Modell und
  je Entscheidung Person und Zeitpunkt (bestätigt, geändert, verworfen).
  Daraus lässt sich die Nacharbeit je Lauf ohne Zusatzaufwand zählen.
- **Schwelle** sollte nach ADR 0045 Frage 4 je Mandant in
  `company_llm_settings` stehen. Mit mehreren Modellen passt sie nicht mehr
  dorthin: ein Score von 0,9 bedeutet bei jedem Modell etwas anderes (ADR 0044
  Frage 4: Schwelle je Anbieter und Modell).
- **Kein Vektorspeicher, keine Embeddings** (ADR 0044 Frage 2 (c) bewusst
  verschoben). Die Vorauswahl der Kandidaten arbeitet mit Wortabgleich.
- **Wichtig zur Erwartung:** Ein Sprachmodell lernt durch Rückmeldungen nicht
  selbst dazu. „Dazulernen“ heißt hier: das System merkt sich bestätigte und
  korrigierte Zuordnungen und gibt ähnliche frühere Entscheidungen beim
  nächsten Lauf mit (RAG im Sinne von „passende Beispiele nachschlagen und
  mitgeben“).
- **Grenze des Vergleichs im Betrieb:** Die Modelle bearbeiten verschiedene
  LVs, verschiedene Kalkulatoren entscheiden. Das Scoring ist deshalb ein
  Hinweis, kein sauberer Versuch; es wird erst ab einer Mindestmenge
  angezeigt.

## Zu entscheiden

1. **Mehrere Modelle je Mandant.**
   (a) neue Tabelle `company_llm_profiles` (Name, Anbieter, Endpunkt, Modell,
   Schlüssel verschlüsselt, Zeitlimit, Schwelle, aktiv, Standardprofil);
   `company_llm_settings` behält den Hauptschalter und das Tageslimit des
   Mandanten; die bestehende Einstellung wird beim Migrieren zum ersten
   Profil „Standard“;
   (b) mehrere Zeilen in `company_llm_settings`.
   Empfehlung: **(a)** — Tageslimit und Hauptschalter bleiben eine Sache des
   Mandanten, alles Modellbezogene wandert ans Profil. Damit wird ADR 0045
   Frage 4 angepasst: **Schwelle je Profil** (Standard 0,9).
2. **Auswahl beim Lauf.**
   (a) beim Start des Laufs wählt der Kalkulator ein Profil oder
   „Wortabgleich ohne Modell“ (Standard: das Standardprofil); das gewählte
   Profil wird am Lauf und an jedem Vorschlag gespeichert (Name, Anbieter,
   Modell als Abschrift, damit das Scoring nach Änderung oder Löschen des
   Profils stimmt);
   (b) fest je Nutzer.
   Empfehlung: **(a)** — „Wortabgleich“ als Vergleichsgröße im Scoring
   zeigt, was ein Modell überhaupt bringt.
3. **Feedback.**
   (a) zwei Stufen: **automatisch** aus den Entscheidungen der Prüfliste
   (bestätigt unverändert, andere Leistung gewählt, verworfen, ohne
   Vorschlag selbst zugeordnet); beim Ändern oder Verwerfen ein **Grund** zur
   Auswahl (falsche Leistung, Systemvorgabe falsch, Einheit/Menge, keine
   passende Leistung im Katalog, Sonstiges) mit optionalem Freitext; bei der
   Freigabe des Laufs eine **Gesamtbewertung** (1–5, optional geschätzte
   Nacharbeit in Minuten, optionaler Freitext);
   (b) nur Freitext am Lauf.
   Empfehlung: **(a)** — der größte Teil kostet keinen Klick; Gründe als
   Auswahl lassen sich zählen, Freitext nicht.
4. **Scoring.**
   (a) Auswertung je Profil (und Modell als Abschrift) über einen Zeitraum:
   Abdeckung (Positionen mit Vorschlag), Übernahme unverändert, **Fehlerquote
   über der Schwelle** (bestätigungsreife Vorschläge, die geändert oder
   verworfen wurden — die Kennzahl für das Ziel ≤ 2 %), Gründe, mittlere
   Gesamtbewertung und Nacharbeit, Aufrufe, Tokens, Dauer; erst ab 50
   entschiedenen Positionen je Profil angezeigt, darunter mit Hinweis
   „zu wenig Daten“; Seite unter Einstellungen mit `settings.manage`;
   (b) wie (a), zusätzlich **Schattenlauf**: ein zweites Profil bearbeitet
   dasselbe LV im Hintergrund, nur zum Vergleich (doppelte Kosten und
   Tageslimit);
   (c) nur Rohdaten-Export.
   Empfehlung: **(a)**; (b) später als Schalter, wenn die Unterschiede im
   Betrieb nicht eindeutig sind.
5. **Lernen aus Korrekturen.**
   (a) „Gedächtnis“ je Mandant aus entschiedenen Zuordnungen (Kurztext,
   Auszug Langtext, Einheit → Leistung oder „kein Ziel“, mit Quelle und
   Zeitpunkt): bei einem neuen Lauf
   - **gleicher Text** (nach Vereinheitlichung) wie eine bestätigte frühere
     Zuordnung → Vorschlag ohne Modell, Quelle `gedaechtnis` (deterministisch
     vor probabilistisch; bleibt ein Vorschlag mit Freigabe),
   - **ähnlicher Text** (Wortabgleich wie die Vorauswahl) → die frühere
     Leistung kommt in die Kandidaten, und bis zu 5 frühere Entscheidungen
     gehen als Beispiele in den Aufruf (auch verworfene als „nicht diese
     Leistung“);
   nur aus demselben Mandant, abgeschaltete Leistungen fallen heraus, eine
   neuere Entscheidung ersetzt eine ältere zum selben Text;
   (b) zusätzlich Vektorsuche (pgvector und Embedding-Dienst) für die
   Ähnlichkeit;
   (c) Feinabstimmung (Fine-Tuning) beim Anbieter.
   Empfehlung: **(a)** — ohne neuen Dienst und ohne dauerhafte Daten beim
   Anbieter; (b) kann die Ähnlichkeitssuche später ersetzen; (c) gibt
   Mandantendaten dauerhaft an den Anbieter und bindet an ihn.
6. **Auswertung nach Personen.**
   (a) Scoring nur je Profil/Modell, **nie je Kalkulator**; die Person an der
   Entscheidung dient nur der Nachvollziehbarkeit am einzelnen Lauf;
   (b) auch je Person.
   Empfehlung: **(a)** — eine Leistungsauswertung von Beschäftigten ist
   mitbestimmungspflichtig (§ 87 Abs. 1 Nr. 6 BetrVG) und nicht Ziel der
   Auskunft.
7. **Messlauf I.4.3 am Korpus.**
   (a) bleibt als **optionaler** Go-Test (nur mit gesetzter
   Umgebungsvariable), ohne Termin: Prüfung der Sicherheitsfälle
   (Anweisungen im Langtext) und erster Wert für die Schwelle eines neuen
   Profils; Hauptweg des Vergleichs ist das Scoring im Betrieb;
   (b) streichen.
   Empfehlung: **(a)**. Schlüssel trägt der Administrator auf der Seite
   „Sprachmodell“ je Profil ein (verschlüsselt gespeichert, wie heute) —
   eine `.env` braucht es dafür nicht.

Unabhängig davon (aus §4, ADR 0043/0044/0045, nicht zur Wahl): Preise nie vom
Modell; Beispiele aus dem Gedächtnis enthalten nur LV- und Katalogfelder,
keine Kunden, Projekte oder Personen; das Protokoll enthält keine Inhalte;
jede Zuordnung — auch aus dem Gedächtnis — bleibt ein Vorschlag bis zur
Freigabe.

## Entscheidung (2026-11-17, ANNAHME)

Alle sieben Fragen mit (a): Modellprofile je Mandant mit Schwelle je Profil
(Hauptschalter und Tageslimit bleiben am Mandanten, bestehende Einstellung
wird Profil „Standard“), Auswahl des Profils oder „Wortabgleich“ beim Start
des Laufs, Feedback aus den Entscheidungen mit Grund und Gesamtbewertung,
Scoring je Profil ab 50 entschiedenen Positionen, Gedächtnis je Mandant
ohne Vektorsuche, keine Auswertung je Person, Messlauf am Korpus optional.

## Vorschlag zur Umsetzung (bei Empfehlung)

Reihenfolge zusammen mit ADR 0045 (I.5), damit die Vorschläge von Anfang an
das Profil tragen:

- **I.5.1** Umzug des Matchers nach `internal/matching` (unverändert).
- **I.8.1** Modellprofile: Migration 122 (`company_llm_profiles`, Übernahme
  der bestehenden Einstellung, Schwelle je Profil), Service und HTTP;
  **I.8.2** Client (Seite „Sprachmodell“ mit Profilen).
- **I.5.2–I.5.5** wie ADR 0045, mit Profil am Lauf und an jedem Vorschlag
  (Migration 123 ff.).
- **I.8.3** Feedback: Gründe beim Ändern/Verwerfen, Gesamtbewertung bei der
  Freigabe (Server und Client).
- **I.8.4** Scoring je Profil (Server und Client).
- **I.8.5** Gedächtnis: gleicher Text als Vorschlag, ähnliche Entscheidungen
  als Kandidaten und Beispiele; Messung am Korpus mit simulierten
  Korrekturen.

## Folgen

- Die Seite „Sprachmodell“ und `llm.SettingsService` werden umgebaut
  (Profile statt einer Zeile); bestehende Einstellungen gehen nicht verloren
  (Übernahme als Profil „Standard“), die bisherigen Endpunkte bleiben bis zum
  Umbau des Clients erhalten.
- Die Prompts werden länger (Beispiele aus dem Gedächtnis); die Grenzen aus
  ADR 0044 (Kürzungen, Bündelgröße) gelten weiter.
- Bis zur Antwort ändert sich kein Produktivcode; I.5.1 hängt nicht davon ab.
