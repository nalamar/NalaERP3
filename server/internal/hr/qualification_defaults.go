package hr

// Vorschlagskatalog fuer Qualifikationen, Unterweisungen und Beauftragungen
// im Metallbau (ADR 0027). Ein Mandant uebernimmt ihn mit
// QualificationTypeService.AdoptDefaults in seinen eigenen Katalog und kann
// danach jeden Eintrag aendern, stilllegen oder ergaenzen.
//
// Die Fristen sind VORSCHLAGSWERTE. Sie stammen aus einer Recherche ueber
// Sekundaerquellen (siehe ADR 0027, "Quellen der Recherche"); wo der
// Hinweis "nicht eigens recherchiert" sagt, ist nicht einmal das der Fall.
// Der Betrieb soll sie mit seiner Fachkraft fuer Arbeitssicherheit
// abgleichen.
//
// Regeln fuer diese Liste (durch Tests abgesichert):
//   - Jede Art hat hoechstens eine Frist. Ein Thema mit mehreren Laufzeiten
//     besteht aus mehreren Arten: beim Stapler die Ausbildung (laeuft nicht
//     ab), die jaehrliche Unterweisung und die schriftliche Beauftragung.
//   - Jede Unterweisung hat eine Frist. Eine wiederkehrende Unterweisung
//     ohne Frist wuerde nie angemahnt.
//   - Nur die allgemeine Arbeitsschutzunterweisung ist Pflicht fuer alle.
//   - Keine Gesundheitsdaten: arbeitsmedizinische Vorsorge und
//     Eignungsuntersuchungen gehoeren nicht in diesen Katalog.

const (
	hinweisJahresregel   = "Wiederholung mindestens jährlich (DGUV Vorschrift 1 §4)."
	hinweisNichtGeprueft = "Nicht eigens recherchiert; Frist nach der allgemeinen Jahresregel für Unterweisungen. Bitte prüfen."
)

func months(n int) *int { return &n }

// DefaultQualificationTypes liefert den Vorschlagskatalog. Jeder Aufruf
// liefert eine eigene Kopie, damit ein Aufrufer die Vorlage nicht
// veraendern kann.
func DefaultQualificationTypes() []QualificationTypeInput {
	const (
		q = QualificationKategorieQualifikation
		u = QualificationKategorieUnterweisung
		b = QualificationKategorieBeauftragung
	)
	return []QualificationTypeInput{
		// Vom Nutzer genannt (2026-10-13).
		{Code: "ARBEITSSCHUTZ", Name: "Arbeitsschutz – allgemeine Sicherheitsunterweisung", Kategorie: u, GueltigkeitMonate: months(12), PflichtFuerAlle: true,
			Hinweis: "Vor Aufnahme der Tätigkeit und danach mindestens jährlich; zu dokumentieren (ArbSchG §12, DGUV Vorschrift 1 §4)."},
		{Code: "ERSTHELFER", Name: "Erste Hilfe – Ersthelfer (Aus-/Fortbildung)", Kategorie: q, GueltigkeitMonate: months(24),
			Hinweis: "Fortbildung in der Regel alle zwei Jahre (DGUV Vorschrift 1 §26)."},

		{Code: "STAPLER_SCHEIN", Name: "Stapler – Ausbildung (Staplerschein)", Kategorie: q,
			Hinweis: "Ausbildung nach DGUV Grundsatz 308-001; läuft nicht ab. Zusätzlich nötig: Beauftragung und jährliche Unterweisung."},
		{Code: "STAPLER_UNTERWEISUNG", Name: "Stapler – jährliche Unterweisung", Kategorie: u, GueltigkeitMonate: months(12), Hinweis: hinweisJahresregel},
		{Code: "STAPLER_BEAUFTRAGUNG", Name: "Stapler – schriftliche Beauftragung", Kategorie: b,
			Hinweis: "Schriftliche Beauftragung durch den Unternehmer (DGUV Vorschrift 68 §7); gilt bis Widerruf."},

		{Code: "HUBARBEITSBUEHNE_SCHEIN", Name: "Hubarbeitsbühne/Scherenbühne – Ausbildung (Bedienerausweis)", Kategorie: q,
			Hinweis: "Ausbildung nach DGUV Grundsatz 308-008; läuft nicht ab. Zusätzlich nötig: Beauftragung und jährliche Unterweisung."},
		{Code: "HUBARBEITSBUEHNE_UNTERWEISUNG", Name: "Hubarbeitsbühne/Scherenbühne – jährliche Unterweisung", Kategorie: u, GueltigkeitMonate: months(12), Hinweis: hinweisJahresregel},
		{Code: "HUBARBEITSBUEHNE_BEAUFTRAGUNG", Name: "Hubarbeitsbühne/Scherenbühne – schriftliche Beauftragung", Kategorie: b,
			Hinweis: "Schriftliche Beauftragung mit Angabe der Bauarten (DGUV Grundsatz 308-008); gilt bis Widerruf."},

		// Einen "Leiterschein" fuer Benutzer gibt es nicht: fuer sie ist es
		// die Unterweisung. Die formale Qualifikation ist die befaehigte
		// Person, die Leitern PRUEFT.
		{Code: "LEITERN_UNTERWEISUNG", Name: "Leitern und Tritte – Unterweisung der Benutzer", Kategorie: u, GueltigkeitMonate: months(12), Hinweis: hinweisJahresregel},
		{Code: "LEITERN_PRUEFER", Name: "Leitern und Tritte – befähigte Person zur Prüfung", Kategorie: q,
			Hinweis: "Für die Person, die Leitern und Tritte prüft, nicht für jeden Benutzer (BetrSichV, TRBS 1203, DGUV Information 208-016)."},

		// Weitere, recherchiert.
		{Code: "SCHWEISSERPRUEFUNG", Name: "Schweißerprüfung (DIN EN ISO 9606-1)", Kategorie: q, GueltigkeitMonate: months(36),
			Hinweis: "Drei Jahre gültig. Zusätzlich Bestätigung alle sechs Monate durch Arbeitgeber/Schweißaufsicht – diese Zwischenbestätigung wird hier NICHT überwacht."},
		{Code: "BRANDSCHUTZHELFER", Name: "Brandschutzhelfer", Kategorie: q, GueltigkeitMonate: months(36),
			Hinweis: "Wiederholung alle drei bis fünf Jahre bei normaler Brandgefährdung (ASR A2.2, DGUV Information 205-023); hier die kürzere Frist."},

		// Weitere, im Metallbau ueblich, NICHT eigens recherchiert.
		{Code: "KRANFUEHRER", Name: "Kran – Ausbildung Kranführer", Kategorie: q,
			Hinweis: "Nicht eigens recherchiert. Bitte prüfen, ob zusätzlich Beauftragung und jährliche Unterweisung als eigene Arten nötig sind."},
		{Code: "ANSCHLAEGER", Name: "Anschlagen von Lasten – Unterweisung", Kategorie: u, GueltigkeitMonate: months(12), Hinweis: hinweisNichtGeprueft},
		{Code: "PSA_ABSTURZ", Name: "Persönliche Schutzausrüstung gegen Absturz – Unterweisung", Kategorie: u, GueltigkeitMonate: months(12), Hinweis: hinweisNichtGeprueft},
		{Code: "GEFAHRSTOFFE", Name: "Gefahrstoffe – Unterweisung", Kategorie: u, GueltigkeitMonate: months(12), Hinweis: hinweisNichtGeprueft},
		{Code: "ELEKTRO_UNTERWIESEN", Name: "Elektrotechnisch unterwiesene Person – Unterweisung", Kategorie: u, GueltigkeitMonate: months(12), Hinweis: hinweisNichtGeprueft},
		{Code: "LADUNGSSICHERUNG", Name: "Ladungssicherung – Unterweisung", Kategorie: u, GueltigkeitMonate: months(12), Hinweis: hinweisNichtGeprueft},

		// Fuhrpark (ADR 0032 §5, Entscheidung des Nutzers vom 2026-10-25):
		// die Kontrolle der Fahrerlaubnis als Nachweis - die Feststellung
		// "eingesehen am ..., Klassen ...", KEINE Kopie des Fuehrerscheins.
		// Kategorie qualifikation: eine Beauftragung gilt bis Widerruf, eine
		// Unterweisung ist es nicht; die Frist ist ein Vorschlag.
		{Code: "FUEHRERSCHEINKONTROLLE", Name: "Führerscheinkontrolle (Einsicht in die Fahrerlaubnis)", Kategorie: q, GueltigkeitMonate: months(6),
			Hinweis: "Halterpflicht (§21 StVG). Eine gesetzliche Frequenz gibt es nicht, halbjährlich ist Praxis. Nur Datum und Klassen festhalten (z. B. in der Nachweis-Nr.), keine Kopie des Führerscheins."},
	}
}
