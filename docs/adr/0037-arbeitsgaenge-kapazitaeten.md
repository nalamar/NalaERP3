# ADR 0037 — Produktionssteuerung: Arbeitsgänge und Kapazitäten

Datum: 2026-11-09
Status: **Angenommen** (2026-11-09, ANNAHME) — "fortsetzen" nach den
sechs Fragen mit Empfehlung (`docs/open-questions.md`, Abschnitt
2026-11-09 H.2): alle **(a)**. Die Auskunftsfragen (Arbeitszeiten in
Logikal, Logikal MES) gelten als unbekannt: Import der Zeiten später,
Rückmeldung je Arbeitsgang wird in H.3 entschieden.
Bezug: Backlog H.2 "Stücklisten/Arbeitsgänge/Kapazitäten"; ADR 0034
(Fertigungsaufträge, Abschnitt 3: "Arbeitsgänge, Kapazitäten (H.2) und
Rückmeldungen/BDE (H.3) kommen später und hängen am Auftrag"); ADR 0029
(Arbeitstage, Feiertage, Betriebsruhe); ADR 0024 (Zeiterfassung).

## Kontext (Ist-Stand, belegt)

- **Stückliste gibt es schon.** Seit H.1 kopiert die Freigabe die
  Materialliste der Logikal-Variante in den Auftrag
  (`production_order_items`: Profil mit Länge, Artikel, Glas mit Maßen,
  ohne Preise; Migration 107), mit Materialbezug und Reservierung
  (Migration 108). Eine eigene, von Logikal unabhängige oder mehrstufige
  Stückliste gibt es nicht.
- **Keine Arbeitsgänge, keine Arbeitsplätze, keine Planzeiten.** Suche im
  Server nach Arbeitsgang/Routing/Arbeitsplatz/Kapazität: keine Treffer
  außer der Betriebsmittel-Kategorie `maschine` (`assets`, Migration 089).
- **Der Logikal-Import liest keine Zeiten** (`projects/import_logikal.go`:
  Projects, Phases, ElevationGroups, Elevations, SingleElevations,
  Insertions, Profiles, Articles, Glas). Logikal selbst kennt
  Arbeitszeiten in der Elementeingabe und hat einen Export/Import für
  Arbeitszeiten; ob sie in der SQLite-Übergabe an das ERP stehen, ist
  nicht belegt (siehe Recherche).
- **Lohnstunden nur in der Angebotskalkulation** (`quote_item_calculations`,
  Migration 073: `lohn_stunden`, `lohn_stundensatz`, Zuschlag). Angebots-
  positionen haben keinen Bezug zu Projektpositionen (`quotes.project_id`
  nur am Kopf) — die Stunden lassen sich keinem Fertigungsauftrag
  zuordnen.
- **Personal**: Arbeitstage je Mitarbeiter (`hr_employees.arbeitstage`,
  Migration 091), gesetzliche Feiertage je Bundesland im Code, Betriebsruhe
  (`hr_company_days`, Migration 090), Abwesenheiten und Urlaub
  (Migration 092–094), Teams (`hr_teams`), Stellen (Migration 097/098),
  Zeiterfassung mit Projektbezug (`hr_time_entries.project_id`, ohne
  Bezug zum Fertigungsauftrag). **Keine Sollstunden je Tag** am
  Mitarbeiter.
- **Fertigungsauftrag** (H.1): Liefertermin, Stückzahl, Status
  entwurf → freigegeben → in_arbeit → fertig / storniert; die Kopie ab
  Freigabe ist unveränderlich.

## Recherche (Sekundärquellen)

| Thema | Befund | Folge |
|---|---|---|
| Logikal → ERP | Übergabe von Auftrags- und Materialbedarfsdaten per SQLite (bzw. mdb); Arbeitszeiten werden in Logikal je Element gepflegt, es gibt "Arbeitszeiten exportieren und importieren" — Inhalt der Seite nicht geprüft | Planzeiten aus Logikal nur nach Prüfung einer echten Exportdatei; bis dahin von Hand |
| Logikal MES | Orgadata/Forterro bietet ein eigenes MES mit Status der Arbeitsgänge und Störmeldungen | Wer Logikal MES nutzt, braucht im ERP keine Rückmeldung je Arbeitsgang (H.3) — Frage an den Nutzer |

Quellen: [Orgadata: Logikal-Projektdaten im ERP](https://help.orgadata.com/us/?p=7220),
[Orgadata: ERP-Export per mdb](https://help.orgadata.com/en/documentation/interfaces/erp/erp-export-via-mdb/),
[Orgadata: Arbeitszeiten in der Elementeingabe](https://help.orgadata.com/dokumentation/kalkulation-und-arbeitszeiten/arbeitszeiten/arbeitszeiten-in-der-elementeingabe/),
[Logikal MES](https://orgadata.com/global/en/solutions/logikal-mes.html).

## Zu entscheiden

### 1. Umfang von H.2

(a) **Arbeitsgänge je Auftrag mit Planzeit und grober Auslastung** je
Arbeitsplatz und Woche (geplant gegen verfügbar, Überlast als Hinweis).
(b) Arbeitsgänge nur als Abhakliste ohne Zeiten und Auslastung.
(c) Feinplanung je Maschine und Stunde mit automatischer Terminierung.
**Empfehlung (a)** — beantwortet "schaffen wir das bis zum Liefertermin?"
mit wenig Pflegeaufwand; (c) ist ein eigenes Produkt (Logikal MES,
Leitstand) und passt nicht zu einem Metallbaubetrieb ohne Maschinendaten
im ERP (ADR 0034: Maschinendaten nicht nachbauen).

### 2. Stücklisten

(a) **Keine eigene Stückliste**: die Fertigungsstückliste aus H.1 gilt;
"Stücklisten" in H.2 ist damit erledigt. (b) Eigene, mehrstufige
Stücklisten als Stammdaten (Baugruppen, Eigenfertigung ohne Logikal).
**Empfehlung (a)** — alle Aufträge entstehen aus Logikal-Positionen;
(b) erst, wenn es Fertigung ohne Logikal gibt.

### 3. Herkunft der Planzeiten

(a) **Arbeitsgangvorlagen je Mandant** (z. B. Zuschnitt, Bearbeitung,
Zusammenbau, Beschlag, Verglasung, Endkontrolle) mit Arbeitsplatz und
Minuten je Stück; beim Anlegen des Auftrags vorgeschlagen, am Auftrag
änderbar; Planzeit = Rüstzeit + Minuten je Stück × Stückzahl.
(b) Aus Logikal importieren. (c) Aus der Angebotskalkulation.
**Empfehlung (a)**, (b) später, wenn eine Exportdatei mit Zeiten
vorliegt; (c) geht nicht (kein Bezug Angebotsposition → Projektposition).
Frage an den Nutzer: pflegt der Betrieb Arbeitszeiten in Logikal?

### 4. Kapazität

(a) **Arbeitsplätze als Stammdaten je Mandant** (Name, Stunden je
Arbeitstag, aktiv); verfügbar je Woche = Stunden je Tag × Werktage Mo–Fr
abzüglich Feiertage (Bundesland des Mandanten) und Betriebsruhe.
(b) Kapazität aus den Mitarbeitern (Arbeitstage, Abwesenheiten) — dafür
fehlen Sollstunden je Tag und eine Zuordnung Mitarbeiter → Arbeitsplatz.
**Empfehlung (a)**; Mitarbeiterbezug und Abwesenheiten erst, wenn ein
Bedarf da ist (H.3 oder später).

### 5. Planung

(a) **Je Arbeitsgang eine geplante Woche**, von Hand gesetzt, vorbelegt
rückwärts vom Liefertermin (letzter Arbeitsgang in der Woche vor dem
Liefertermin, davor je Arbeitsgang eine Woche früher); Auslastung je
Arbeitsplatz und Woche; Überlast und "Woche nach Liefertermin" als
Hinweis, keine Sperre. (b) Je Arbeitsgang ein Tag. (c) Nur der
Liefertermin, keine Planung je Arbeitsgang.
**Empfehlung (a)** — Wochenraster genügt für die Grobplanung, Tage
täuschen Genauigkeit vor.

### 6. Änderbarkeit

(a) **Arbeitsgänge sind Planung, nicht Teil der eingefrorenen Kopie**:
änderbar (hinzufügen, Zeit, Woche, Arbeitsplatz) bis der Auftrag fertig
oder storniert ist, ohne Storno des Auftrags; Änderungen im
`entity_change_log`. Arbeitsgänge stornierter und fertiger Aufträge
zählen nicht zur Auslastung. (b) Mit der Freigabe einfrieren.
**Empfehlung (a)**.

## Folgen (beschlossen)

- Migration 112: `production_work_centers` (Arbeitsplätze),
  `production_operation_templates` (Vorlagen), `production_order_operations`
  (Arbeitsgänge am Auftrag mit Reihenfolge, Arbeitsplatz, Rüstzeit,
  Minuten je Stück, geplanter Woche als Montag der Woche).
  Rechte: vorhandene `production.read/write`; Stammdaten mit
  `production.write`.
- Teilung: H.2.1 Migration und Regeln (Planzeit, Vorbelegung der Woche,
  verfügbare Stunden je Woche), H.2.2 Service und Routen Stammdaten,
  H.2.3 Arbeitsgänge am Auftrag, H.2.4 Auslastung je Arbeitsplatz und
  Woche, H.2.5 Client.
- Rückmeldung je Arbeitsgang (Ist-Zeit, erledigt) bleibt H.3 (BDE) —
  sofern nicht Logikal MES das übernimmt.
