# ADR 0043: Austauschbarer Zugriff auf ein Sprachmodell (Provider-Interface)

- Status: Angenommen (2026-11-16; Fragen 1–6 vom Nutzer beantwortet, mit
  Anpassungen bei 1 und 6)
- Bezug: Backlog I.3, `aufgabe.md` §4 (LLM nur für Matching und Textarbeit,
  Preise nie vom Modell, Score und Begründung, Freigabe durch einen Menschen,
  Modellzugriff austauschbar, lokales Modell möglich), ADR 0039
  (Evaluationsset, Matcher-Schnittstelle), ADR 0033 (Zugangsdaten je Mandant
  verschlüsselt)

## Bestandsaufnahme

- **Kein Modellzugriff im Code**, keine Abhängigkeit zu einem Anbieter, keine
  Konfiguration dafür.
- **Matcher-Schnittstelle** gibt es im Evaluationsset (`eval.Matcher`:
  `Vorschlagen(Position, *Katalog) Vorschlag` mit Code, Score, Begründung,
  Systemvorgaben). Die Messung prüft Score in [0, 1] und Begründung bei jeder
  Zuordnung; Ausgangswert Wortabgleich 25,5 % Treffer bei 0 % Fehlzuordnung,
  Ziel für I.4: Fehlzuordnung ≤ 2 %, Treffer ≥ 70 %.
- **Zugangsdaten je Mandant:** Vorbild Mailzugang (ADR 0033, Migration 104):
  eine Zeile je Mandant, Geheimnis nur verschlüsselt (AES-256-GCM über
  `secretbox`, Schlüssel aus `SETTINGS_ENCRYPTION_KEY`, Mandant als AAD), die
  API liefert es nie aus; Schalter „aktiv“ nur bei vollständiger Einrichtung.
- **Was an ein Modell ginge:** LV-Texte (Kurz-/Langtext, Menge, Einheit, Art)
  und der Leistungskatalog. Personenbezogene Daten (Kunde, Adresse, Namen)
  stehen nicht in diesen Feldern, wohl aber im Angebot und Projekt drumherum.

## Zu entscheiden

1. **Anbieter.** (a) Eine schmale Schnittstelle mit zwei Anbindungen: eine für
   OpenAI-kompatible Endpunkte (deckt lokale Server wie Ollama, vLLM oder
   LM Studio und viele Cloud-Dienste ab) und eine für die Anthropic-API;
   welcher Dienst genutzt wird, steht in den Einstellungen; (b) nur ein
   Cloud-Anbieter; (c) nur lokal. Empfehlung: **(a)** — erfüllt „lokales
   Modell möglich“ ohne Festlegung auf einen Anbieter. **Auskunft erbeten:**
   Gibt es einen Anbieter oder Vertrag, den der Betrieb nutzen will, oder
   Hardware für ein lokales Modell?
2. **Form der Schnittstelle.** (a) Allgemein: Anfrage mit Systemtext,
   Nachrichten, verlangtem JSON-Schema, Höchstlänge und Temperatur; Antwort
   mit Text und Verbrauch. Die fachliche Logik (Kandidaten vorauswählen,
   Antwort gegen Katalog und Schema prüfen, nie einen Preis übernehmen) bleibt
   im ERP; (b) fachliche Schnittstelle je Aufgabe (`MatchPosition` …) im
   Anbieter. Empfehlung: **(a)** — die Regeln aus §4 stehen einmal im ERP und
   gelten für jeden Anbieter.
3. **Einstellungen und Schlüssel.** (a) Je Mandant in den Einstellungen wie
   der Mailzugang: Anbieter, Endpunkt, Modell, API-Schlüssel nur
   verschlüsselt, Schalter „aktiv“ (Standard aus); (b) global über
   Umgebungsvariablen. Empfehlung: **(a)** — gleiches, schon geprüftes Muster;
   für die Entwicklung zusätzlich Umgebungsvariablen, die nur in der lokalen
   `.env` stehen.
4. **Datenschutz.** (a) An das Modell gehen nur LV-Felder und Katalogdaten
   (feste Liste von Feldern), nie Kunde, Projekt, Adressen oder Namen; KI ist
   je Mandant ausdrücklich einzuschalten; (b) keine Einschränkung.
   Empfehlung: **(a)** — DSGVO; bei Cloud-Anbietern verlassen die Daten das
   Haus.
5. **Protokoll.** (a) Je Aufruf Anbieter, Modell, Dauer, Verbrauch (Tokens),
   Ergebnis (ok/Fehler/ungültige Antwort) und Prüfsumme der Eingabe — keine
   Inhalte im Log; die Begründung landet am Vorschlag, nicht im Log; (b) volle
   Anfragen und Antworten protokollieren. Empfehlung: **(a)**.
6. **Grenzen.** (a) Zeitlimit je Aufruf (60 s), höchstens 2 Wiederholungen bei
   vorübergehenden Fehlern, Höchstzahl Aufrufe je Tag und Mandant
   (einstellbar), Abbruch mit klarer Meldung statt Teilergebnis ohne Hinweis;
   (b) keine Grenzen. Empfehlung: **(a)** — begrenzt Kosten und hängende
   Läufe.

Unabhängig davon: Tests laufen gegen einen Test-Anbieter ohne Netz; ein
optionaler Test gegen einen echten Dienst läuft nur mit gesetzter
Umgebungsvariable und nie in der Standard-Suite.

## Entscheidung

Antwort des Nutzers (2026-11-16):

1. **(a), erweitert um Mistral:** Anbindungen für OpenAI-kompatible Endpunkte,
   Anthropic und Mistral.
2. **(a)** allgemeine Schnittstelle, fachliche Regeln im ERP.
3. **(a)** Einstellungen je Mandant wie der Mailzugang.
4. **(a)** nur LV- und Katalogfelder an das Modell, KI je Mandant einzuschalten.
5. **(a)** Protokoll ohne Inhalte.
6. **(a), aber Zeitlimit 300 s als Standard und je Mandant einstellbar**;
   höchstens 2 Wiederholungen, Tageslimit je Mandant.

Auskunft: Es gibt keinen bevorzugten Anbieter oder Vertrag — im Betrieb soll
sich zeigen, welcher Anbieter sich am besten schlägt. Folge: I.4 macht die
Anbieter am Evaluationsset (ADR 0039) vergleichbar. Hardware für ein lokales
Modell gibt es noch nicht, sie ist eine spätere Überlegung; die
OpenAI-kompatible Anbindung hält den Weg offen.

## Vorschlag zur Umsetzung (bei Empfehlung)

- **Paket** `server/internal/llm`: Schnittstelle `Provider`, Anfrage-/
  Antworttypen, Test-Anbieter, Prüfung der JSON-Antwort gegen das Schema,
  Grenzen und Protokoll als Hülle um jeden Anbieter.
- **I.3.1** Schnittstelle, Test-Anbieter, Hülle (Zeitlimit, Wiederholung,
  Protokoll ohne Inhalte), Feldliste für den Datenschutz.
- **I.3.2** Anbindung OpenAI-kompatibel (lokal und Cloud).
- **I.3.3** Anbindung Anthropic.
- **I.3.4** Anbindung Mistral.
- **I.3.5** Einstellungen je Mandant (Migration, verschlüsselter Schlüssel,
  Zeitlimit mit Standard 300 s, Tageslimit, API und Client).
- Danach **I.4**: Matcher mit Modell, gemessen am Evaluationsset (ADR 0039).

## Folgen

- Ohne Einrichtung bleibt alles wie heute; kein Datenverkehr nach außen.
- Neue Abhängigkeiten nur, wenn eine Anbindung sie braucht; HTTP-Aufrufe
  lassen sich mit der Standardbibliothek umsetzen.
