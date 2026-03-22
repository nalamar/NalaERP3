// Anweisung
Du bist Codex, ein extrem disziplinierter Senior Full-Stack Architect.

Wir wollen die vorliegende Code-Basis nutzen und diese zu einem FullStack Metallbau-ERP ausbauen mit Angebotsverwaltung, Waren- und Lagerverwaltung, Bestellwesen, Finanzwesen, Personalverwaltung inklusive HR-Management (Urlaub, Weiterbildung, Asset Management) Fuhrpark und Produktionssteuerung. Wichtig ist das am Ende Angebote automatisiert über KI aus GAEB Dateiern erzeugt werden sollen!

Regeln (niemals brechen):
- Analysiere zuerst das hochgeladene Git-Repo vollständig.
- Erstelle eine hierarchische Task-Breakdown: Epic → Feature → Task → Subtask (max. 6 Ebenen tief).
- Gib die komplette Hierarchy als Markdown-Tree aus.
- Arbeite immer nur an **einer** aktuellen Subtask.
- Wenn eine Subtask zu groß ist → zerlege sie selbstständig in 3–6 Micro-Subtasks und bearbeite nur die erste davon.
- Nutze den Code Interpreter + Artifacts/Canvas für alle Datei-Änderungen (nie nur Text ausgeben).
- Am Ende jeder Antwort **immer** folgenden State-Block ausgeben:

=== CODEX STATE [START] ===
Aktueller Hierarchy-Pfad: Epic 3 / Feature 2.1 / Task 5 / Subtask 3.2
Erledigt: [Liste aller abgeschlossenen Leaves, nur Nummerierung]
Nächste zu bearbeitende Subtask: [genaue Beschreibung]
Aktueller Git-Stand: [kurze Zusammenfassung der letzten Änderungen]
Zusammenfassung für Fortsetzung (max 1200 Tokens): 
[kompakter Text mit allen wichtigen Entscheidungen, Architektur-Annahmen, offenen Fragen]
=== CODEX STATE [ENDE] ===

Du darfst den State-Block nie verändern oder weglassen.


// Analyse
Ist-Analyse

Die Codebasis ist kein Greenfield, sondern bereits ein früher ERP-Monolith mit Flutter-Web-Client und Go-API. Zentral vorhanden sind:

Architektur und Container-Setup mit Go + Flutter + Postgres + Mongo GridFS + Redis in README.md und docker-compose.yml.
API-Grundstruktur, Routing und Modulverdrahtung in main.go, server.go, router.go und v1.go.
Bereits umgesetzte Fachdomänen: Kontakte/CRM, Materialien, Lagerbewegungen, Bestellungen, Projekte, LogiKal-Import, PDF-Templates, Einheiten, Projektdokumente/Assets in service.go, service.go, service.go, service.go.
Datenbankschema bis Migrationsstand 016 mit belastbarer Basis für Material, Einkauf, Kontakte, Projekte, Varianten, Importlogs und Assets in server/internal/migrate/migrations.
Clientseitig existieren bereits Module für Dashboard, Projekte, Materialwirtschaft, Kontakte, Bestellungen, Einstellungen und API-Client in main.dart, api.dart, dashboard_page.dart, projects_page.dart, materials_page.dart, purchase_orders_page.dart, settings_page.dart.
Fazit: Die beste Strategie ist kein Neuaufbau, sondern kontrollierter Ausbau entlang vorhandener Module. Größte Lücken sind Auth/Rollen, Angebotswesen, Finanzwesen, HR, Fuhrpark, Produktionssteuerung und die spätere KI/GAEB-Pipeline.

Task Breakdown

- Epic 1: Plattform- und Architektur-Fundament fuer ERP-Skalierung
  - Feature 1.1: Bestandsaufnahme und Zielarchitektur
    - Task 1.1.1: Repository-Ist-Analyse und Capability-Mapping
      - Subtask 1.1.1.1: Codebasis nach Frontend, Backend, Datenmodell und Infrastruktur inventarisieren
      - Subtask 1.1.1.2: Bereits vorhandene ERP-Funktionen und White-Spots dokumentieren
      - Subtask 1.1.1.3: Ziel-Domaenen und Abhaengigkeiten fuer Metallbau-ERP priorisieren
    - Task 1.1.2: Zielbild fuer modulare Monolith-Architektur definieren
      - Subtask 1.1.2.1: Bounded Contexts fuer Sales, Procurement, Inventory, Finance, HR, Fleet, Production, AI festlegen
      - Subtask 1.1.2.2: API-, Service- und Datenzugriffskonventionen vereinheitlichen
      - Subtask 1.1.2.3: Ereignis- und Automationspunkte fuer spaetere KI-Workflows definieren
  - Feature 1.2: Sicherheit, Identitaet und Rechte
    - Task 1.2.1: Authentifizierung einfuehren
      - Subtask 1.2.1.1: User- und Session-Datenmodell entwerfen
      - Subtask 1.2.1.2: Login, Refresh, Logout mit Redis-Session-Layer implementieren
      - Subtask 1.2.1.3: Clientseitigen Auth-Flow und Guarding einfuehren
    - Task 1.2.2: Rollen- und Rechtekonzept
      - Subtask 1.2.2.1: Rollenmodell fuer Vertrieb, Einkauf, Buchhaltung, HR, Produktion, Admin definieren
      - Subtask 1.2.2.2: API-Endpunkte mit Berechtigungspruefungen absichern
      - Subtask 1.2.2.3: UI-Freigaben und Mandanten-Sichtbarkeit steuern
  - Feature 1.3: Querschnitt und Betriebsfaehigkeit
    - Task 1.3.1: Observability und Fehlerbehandlung
      - Subtask 1.3.1.1: Einheitliches Logging und Request-Korrelation einfuehren
      - Subtask 1.3.1.2: Domainfehler in strukturierte API-Responses ueberfuehren
      - Subtask 1.3.1.3: Technische Health-Checks fuer Postgres, Mongo und Redis erweitern
    - Task 1.3.2: Test- und Release-Basis
      - Subtask 1.3.2.1: Service-Tests fuer Kernmodule aufbauen
      - Subtask 1.3.2.2: API-Integrationstests mit Compose-Testumgebung einfuehren
      - Subtask 1.3.2.3: CI-Pipeline fuer Lint, Test und Build definieren

- Epic 2: Stammdaten, CRM und Organisationsgrundlagen
  - Feature 2.1: Kontakt- und Geschaeftspartnerverwaltung
    - Task 2.1.1: Kontaktmodell haerten
      - Subtask 2.1.1.1: Kunden-, Lieferanten- und Partnerstatus fachlich schaerfen
      - Subtask 2.1.1.2: Zahlungsbedingungen, Debitor/Kreditor-Referenzen und Steuermerkmale erweitern
      - Subtask 2.1.1.3: Dublettenpruefung und Suchindizes verbessern
    - Task 2.1.2: Kontaktinteraktionen
      - Subtask 2.1.2.1: Notizen, Aufgaben und Historie pro Kontakt einfuehren
      - Subtask 2.1.2.2: Ansprechpartner-Rollen und Kommunikationskanaele erweitern
      - Subtask 2.1.2.3: Dokumentenanhaenge an Kontakte anbinden
  - Feature 2.2: Organisations- und Firmenstammdaten
    - Task 2.2.1: Unternehmensparameter zentralisieren
      - Subtask 2.2.1.1: Firmenprofile, Niederlassungen und Bankdaten modellieren
      - Subtask 2.2.1.2: Steuer-, Waehrungs- und Lokalisierungsparameter verwalten
      - Subtask 2.2.1.3: Dokumentlayouts und Branding systemweit vereinheitlichen
    - Task 2.2.2: Nummernkreise und Referenzdaten
      - Subtask 2.2.2.1: Nummernkreisstrategie fuer alle Belege definieren
      - Subtask 2.2.2.2: Einheiten, Materialgruppen und Statuskataloge administrierbar machen
      - Subtask 2.2.2.3: Seed- und Migrationsstrategie fuer Referenzdaten festlegen

- Epic 3: Vertrieb, Angebotswesen und Auftragsuebergabe
  - Feature 3.1: Angebotsverwaltung
    - Task 3.1.1: Angebotskopf und Positionen
      - Subtask 3.1.1.1: Angebotsdatenmodell fuer Kopf, Positionen, Versionen und Status entwerfen
      - Subtask 3.1.1.2: Angebots-CRUD und Versionslogik implementieren
      - Subtask 3.1.1.3: Angebotsmaske im Client mit Positionsbearbeitung aufbauen
    - Task 3.1.2: Kalkulation und Preisbildung
      - Subtask 3.1.2.1: Kalkulationsschema fuer Material, Lohn, Fremdleistung und Zuschlaege definieren
      - Subtask 3.1.2.2: Positions- und Summenkalkulation implementieren
      - Subtask 3.1.2.3: Nachlass, Deckungsbeitrag und Freigaben abbilden
    - Task 3.1.3: Angebotsdokumente
      - Subtask 3.1.3.1: PDF-Ausgabe fuer Angebote mit Template-Engine erstellen
      - Subtask 3.1.3.2: Versand- und Freigabehistorie erfassen
      - Subtask 3.1.3.3: Angebotsannahme in Auftrag/Projekt ueberfuehren
  - Feature 3.2: Auftragsuebergabe in Projekte und Beschaffung
    - Task 3.2.1: Sales-to-Execution-Flow
      - Subtask 3.2.1.1: Uebernahme von Angebotsposten in Projektstruktur definieren
      - Subtask 3.2.1.2: Beschaffungs- und Produktionsbedarf aus Angebot ableiten
      - Subtask 3.2.1.3: Statusfluss von Lead ueber Angebot zu Auftrag etablieren
  - Feature 3.3: KI-gestuetzte Angebotserzeugung aus GAEB
    - Task 3.3.1: GAEB-Import-Pipeline
      - Subtask 3.3.1.1: GAEB-Dateiformate und Parserstrategie evaluieren
      - Subtask 3.3.1.2: Importiertes Leistungsverzeichnis in internes Angebotsmodell mappen
      - Subtask 3.3.1.3: Validierungs- und Fehlerprotokoll fuer GAEB-Import aufbauen
    - Task 3.3.2: KI-Assistenz fuer Positionserzeugung
      - Subtask 3.3.2.1: Prompt- und Extraktionsschema fuer LV-Positionen definieren
      - Subtask 3.3.2.2: Material-, Leistungs- und Kalkulationsvorschlaege generieren
      - Subtask 3.3.2.3: Benutzerpruefung, Korrektur und Lernschleife einbauen
    - Task 3.3.3: Angebotsautomatisierung
      - Subtask 3.3.3.1: Import->KI->Kalkulation->Angebotsentwurf Workflow orchestrieren
      - Subtask 3.3.3.2: Vertrauensniveau, Quellenhinweise und Review-Gates darstellen
      - Subtask 3.3.3.3: Benchmarks gegen manuelle Kalkulation einfuehren

- Epic 4: Materialwirtschaft, Lager und Einkauf
  - Feature 4.1: Materialstamm und technische Artikel
    - Task 4.1.1: Materialmodell erweitern
      - Subtask 4.1.1.1: Materialattribute fuer Metallbauprofile, Bleche, Glas, Beschlaege und Normteile standardisieren
      - Subtask 4.1.1.2: Lieferantenartikel, Alternativen und Preislisten anbinden
      - Subtask 4.1.1.3: Dokumente, Zertifikate und technische Datenblaetter strukturieren
  - Feature 4.2: Lager- und Bestandsfuehrung
    - Task 4.2.1: Lagerlogik professionalisieren
      - Subtask 4.2.1.1: Lagerplaetze, Chargen, Reservierungen und Inventurprozesse erweitern
      - Subtask 4.2.1.2: Mindestbestaende, Meldebestand und Dispositionsregeln implementieren
      - Subtask 4.2.1.3: Buchungsarten fuer Einlagerung, Umlagerung, Entnahme, Korrektur absichern
    - Task 4.2.2: Materialfluss an Projekt und Produktion koppeln
      - Subtask 4.2.2.1: Projektbezogene Reservierungen einfuehren
      - Subtask 4.2.2.2: Materialverbrauch aus Fertigung rueckmelden
      - Subtask 4.2.2.3: Lagerkennzahlen und Bewegungsjournal visualisieren
  - Feature 4.3: Einkauf und Bestellwesen
    - Task 4.3.1: Bestellungen ausbauen
      - Subtask 4.3.1.1: Bestellkopf, Positionen und Wareneingang fachlich erweitern
      - Subtask 4.3.1.2: Liefertermine, Teilmengen, Mahnungen und Statusautomatismen einfuehren
      - Subtask 4.3.1.3: Bestell-PDF und E-Mail/Export-Prozesse erweitern
    - Task 4.3.2: Beschaffungsdisposition
      - Subtask 4.3.2.1: Bedarfsermittlung aus Angebot, Projekt und Mindestbestand aufbauen
      - Subtask 4.3.2.2: Bestellvorschlaege generieren
      - Subtask 4.3.2.3: Lieferantenvergleich und Freigabeprozess einbauen

- Epic 5: Projektabwicklung und Produktionsnahe Prozesse
  - Feature 5.1: Projektstruktur und technische Positionen
    - Task 5.1.1: Projektmodell erweitern
      - Subtask 5.1.1.1: Projektphasen, Positionen und Varianten mit kaufmaennischen Feldern anreichern
      - Subtask 5.1.1.2: Projektstatus, Meilensteine und Verantwortlichkeiten definieren
      - Subtask 5.1.1.3: Dokumente, Assets und Importhistorie konsolidieren
    - Task 5.1.2: LogiKal-Integration absichern
      - Subtask 5.1.2.1: Re-Import-Regeln, Undo und Datenkonsistenz haerten
      - Subtask 5.1.2.2: Bild-/Asset-Konvertierung robuster gestalten
      - Subtask 5.1.2.3: Material-Mapping aus LogiKal gegen Stammdaten verbessern
  - Feature 5.2: Produktionssteuerung
    - Task 5.2.1: Arbeitsvorbereitung
      - Subtask 5.2.1.1: Fertigungsauftraege aus Projektpositionen ableiten
      - Subtask 5.2.1.2: Arbeitsplaene, Arbeitsgaenge und Ressourcen modellieren
      - Subtask 5.2.1.3: Stuecklisten und Verbrauchslisten versionieren
    - Task 5.2.2: Shopfloor und Rueckmeldungen
      - Subtask 5.2.2.1: Start/Stopp, Mengen- und Ausschussrueckmeldungen erfassen
      - Subtask 5.2.2.2: Maschinen- und Arbeitsplatzbelegung visualisieren
      - Subtask 5.2.2.3: Produktionsfortschritt in Projektstatus zurueckspiegeln

- Epic 6: Finanzwesen und kaufmaennische Abwicklung
  - Feature 6.1: Debitoren und Kreditoren
    - Task 6.1.1: Rechnungen und Gutschriften
      - Subtask 6.1.1.1: Ausgangsrechnungen aus Angebot/Auftrag/Projekt modellieren
      - Subtask 6.1.1.2: Eingangsrechnungen gegen Bestellungen und Wareneingaenge pruefen
      - Subtask 6.1.1.3: Steuer-, Faelligkeits- und Belegstatuslogik implementieren
    - Task 6.1.2: Zahlungsmanagement
      - Subtask 6.1.2.1: Offene Posten und Mahnwesen aufbauen
      - Subtask 6.1.2.2: Zahlungsausgleich und Teilzahlungen erfassen
      - Subtask 6.1.2.3: Kreditoren-/Debitorenreporting bereitstellen
  - Feature 6.2: Kostenrechnung und Controlling
    - Task 6.2.1: Projekt- und Kostenstellencontrolling
      - Subtask 6.2.1.1: Kostenstellen, Kostentraeger und Buchungslogik modellieren
      - Subtask 6.2.1.2: Soll-Ist-Vergleiche fuer Projekte und Produktion implementieren
      - Subtask 6.2.1.3: Deckungsbeitrag und Liquiditaetsvorschau bereitstellen

- Epic 7: Personal, HR und Asset Management
  - Feature 7.1: Mitarbeiterstamm und Organisation
    - Task 7.1.1: Mitarbeiterdaten
      - Subtask 7.1.1.1: Mitarbeiterstamm, Vertraege, Rollen und Qualifikationen modellieren
      - Subtask 7.1.1.2: Organisationszuordnung zu Teams, Standorten und Kostenstellen abbilden
      - Subtask 7.1.1.3: Zugriffsrechte mit HR-Daten verknuepfen
  - Feature 7.2: HR-Prozesse
    - Task 7.2.1: Urlaub und Abwesenheiten
      - Subtask 7.2.1.1: Urlaubskonten, Antraege und Freigabeworkflow umsetzen
      - Subtask 7.2.1.2: Krankmeldungen und sonstige Abwesenheiten erfassen
      - Subtask 7.2.1.3: Teamkalender und Kapazitaetswirkung abbilden
    - Task 7.2.2: Weiterbildung und Qualifikationen
      - Subtask 7.2.2.1: Schulungen, Zertifikate und Faelligkeiten verwalten
      - Subtask 7.2.2.2: Einsatzfaehigkeit fuer Maschinen, Fahrzeuge und Arbeiten pruefen
      - Subtask 7.2.2.3: Erinnerungen und Nachweisablage bereitstellen
  - Feature 7.3: HR-Asset-Management
    - Task 7.3.1: Mitarbeiterbezogene Assets
      - Subtask 7.3.1.1: Ausgabe und Ruecknahme von Werkzeugen, IT und PSA modellieren
      - Subtask 7.3.1.2: Seriennummern, Zustand und Verantwortlichkeit verfolgen
      - Subtask 7.3.1.3: Verknuepfung zu Lager, Einkauf und Kostenstellen herstellen

- Epic 8: Fuhrpark und mobile Betriebsmittel
  - Feature 8.1: Fahrzeugverwaltung
    - Task 8.1.1: Fahrzeugstamm
      - Subtask 8.1.1.1: Fahrzeuge, Anhaenger und Maschinen als Assets modellieren
      - Subtask 8.1.1.2: Wartung, Prueftermine, Versicherung und Dokumente verwalten
      - Subtask 8.1.1.3: Fahrerzuordnung und Verfuegbarkeit darstellen
    - Task 8.1.2: Fuhrparkprozesse
      - Subtask 8.1.2.1: Fahrtenbuch und Einsatzplanung einfuehren
      - Subtask 8.1.2.2: Kosten, Verbraeuche und Schadensfaelle erfassen
      - Subtask 8.1.2.3: Schnittstelle zu Projekten und HR schaffen

- Epic 9: Reporting, Automation und Entscheidungsunterstuetzung
  - Feature 9.1: Operatives Reporting
    - Task 9.1.1: Management-Dashboard
      - Subtask 9.1.1.1: KPI-Modell fuer Vertrieb, Einkauf, Lager, Projekte, Finanzen und HR definieren
      - Subtask 9.1.1.2: Dashboard-Widgets und Drilldowns im Client entwickeln
      - Subtask 9.1.1.3: Filter nach Zeitraum, Standort und Verantwortlichem bereitstellen
  - Feature 9.2: Workflow-Automation
    - Task 9.2.1: Regelbasierte Automationen
      - Subtask 9.2.1.1: Ereignisgesteuerte Benachrichtigungen fuer Freigaben und Faelligkeiten einbauen
      - Subtask 9.2.1.2: Aufgaben- und Reminder-Engine fuer operatives Follow-up entwickeln
      - Subtask 9.2.1.3: Eskalationen und Audit-Trails absichern


// State

Die Auftragsliste hat jetzt einen echten Teilfaktura-Fokus. In sales_orders_page.dart gibt es einen Schnellfilter Teilfaktura, der die Liste auf Aufträge mit Restbetrag eingrenzt. Außerdem zeigen die Listeneinträge jetzt nicht nur Status und Brutto, sondern auch den Faktura-Fortschritt sowie Rechnungen / Restbetrag, statt den Nutzer dafür erst in die Detailansicht zu zwingen.

Die Regression dafür steckt in sales_order_context_pages_test.dart: Der neue Test prüft, dass nach Aktivieren des Chips nur noch teilfakturierte Aufträge sichtbar bleiben. Verifikation bleibt weiter durch das lokale Tooling begrenzt: dart format client/lib/pages/sales_orders_page.dart client/test/sales_order_context_pages_test.dart lief erneut nach 20s in einen Timeout, und flutter test test/sales_order_context_pages_test.dart lief nach 120s in einen Timeout.

=== CODEX STATE [START] ===
Aktueller Hierarchy-Pfad: Epic 2 / Feature 2.2 / Task 2.2.1 / Subtask 2.2.1.3 / Micro-Subtask 7 / Micro-Subtask 8

Erledigt:

Subtask 2.2.1.3 / Micro-Subtask 7 / Micro-Subtask 8: Teilfaktura-/Mehrfachbeleg-UX in der Auftragsliste verdichten, inklusive Schnellfilter und Restbetragsanzeige
Nächste zu bearbeitende Subtask:

Subtask 2.2.1.3 / Micro-Subtask 7 / Micro-Subtask 9: Flutter-Tooling/Teststabilisierung oder weitere Feinschliffe für Folgebeleg-Filter und Statusdarstellung in angrenzenden Listen
Aktueller Git-Stand:

sales_orders_page.dart hat jetzt Teilfaktura-Schnellfilter, Fortschrittslabel und Restbetrags-Hinweis in der Liste
sales_order_context_pages_test.dart prüft den neuen Listenfilter
Frühere Backend-Felder related_invoice_count und remaining_gross_amount werden jetzt auch in der Listen-UX genutzt
codex.md wurde nicht geändert
Zusammenfassung für Fortsetzung:

Teilfaktura ist jetzt auch in der Auftragsliste operativ nutzbar. Die größte offene Lücke bleibt weiterhin nicht fachlich, sondern technisch: lokale Flutter-/Dart-Kommandos hängen weiterhin bei Formatierung und Widget-Testlauf.
=== CODEX STATE [ENDE] ===

