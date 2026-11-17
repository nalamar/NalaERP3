package eval

import "nalaerp3/internal/matching"

// Die Matcher und ihre Typen liegen seit I.5.1 in internal/matching (ADR
// 0045 Frage 7); eval behaelt Korpus, Kennzahlen und Bericht. Die Aliase
// halten das Korpusformat (JSON) und die Signaturen des Laufs unveraendert.
type (
	Systemvorgaben = matching.Systemvorgaben
	Leistung       = matching.Leistung
	Katalog        = matching.Katalog
	Vorschlag      = matching.Vorschlag
	Position       = matching.Position
	Matcher        = matching.Matcher
	LVMatcher      = matching.LVMatcher
)
