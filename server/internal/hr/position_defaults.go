package hr

// Vorschlagskatalog typischer Stellen im Metallbau (ADR 0031, Backlog
// F.11.4). Ein Mandant uebernimmt ihn mit PositionService.AdoptDefaults
// und kann danach jede Stelle aendern, stilllegen oder ergaenzen.
//
// NICHT RECHERCHIERT: welche Arten eine Stelle verlangt, ist aus dem
// Qualifikations-Vorschlagskatalog (ADR 0027) abgeleitet, nicht aus einer
// Rechtsquelle. Der Betrieb soll die Zuordnung mit seiner Fachkraft fuer
// Arbeitssicherheit abgleichen; jeder Eintrag sagt das im Hinweis.
//
// Regeln fuer diese Liste (durch Tests abgesichert):
//   - Jede Art steht im Qualifikations-Vorschlagskatalog (Abgleich ueber
//     den Code).
//   - Keine Art, die ohnehin fuer alle Pflicht ist (Arbeitsschutz).
//   - Jede Stelle verlangt mindestens eine Art.

const hinweisStelleNichtRecherchiert = "Vorschlag, nicht recherchiert: die Zuordnung ist aus dem Qualifikationskatalog abgeleitet. Bitte mit der Fachkraft für Arbeitssicherheit abgleichen."

// PositionDefault ist eine vorgeschlagene Stelle; TypeCodes sind Codes aus
// DefaultQualificationTypes.
type PositionDefault struct {
	Code      string
	Name      string
	Hinweis   string
	TypeCodes []string
}

// DefaultPositions liefert den Vorschlagskatalog, je Aufruf eine eigene
// Kopie.
func DefaultPositions() []PositionDefault {
	return []PositionDefault{
		{Code: "MONTEUR", Name: "Monteur", Hinweis: hinweisStelleNichtRecherchiert, TypeCodes: []string{
			"PSA_ABSTURZ", "LEITERN_UNTERWEISUNG",
			"HUBARBEITSBUEHNE_SCHEIN", "HUBARBEITSBUEHNE_UNTERWEISUNG", "HUBARBEITSBUEHNE_BEAUFTRAGUNG",
		}},
		{Code: "SCHWEISSER", Name: "Schweißer", Hinweis: hinweisStelleNichtRecherchiert, TypeCodes: []string{
			"SCHWEISSERPRUEFUNG", "BRANDSCHUTZHELFER", "GEFAHRSTOFFE",
		}},
		{Code: "LAGERIST", Name: "Lagerist", Hinweis: hinweisStelleNichtRecherchiert, TypeCodes: []string{
			"STAPLER_SCHEIN", "STAPLER_UNTERWEISUNG", "STAPLER_BEAUFTRAGUNG", "LADUNGSSICHERUNG",
		}},
		{Code: "STAPLERFAHRER", Name: "Staplerfahrer", Hinweis: hinweisStelleNichtRecherchiert, TypeCodes: []string{
			"STAPLER_SCHEIN", "STAPLER_UNTERWEISUNG", "STAPLER_BEAUFTRAGUNG",
		}},
		{Code: "KRANFUEHRER", Name: "Kranführer", Hinweis: hinweisStelleNichtRecherchiert, TypeCodes: []string{
			"KRANFUEHRER", "ANSCHLAEGER",
		}},
	}
}
