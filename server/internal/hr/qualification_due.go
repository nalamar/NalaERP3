package hr

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Anmahnen anstehender Erneuerungen (ADR 0027, Backlog F.2.6.1). Vorgabe
// des Nutzers vom 2026-10-13: "Eine anstehende Erneuerung sollte vom
// System angemahnt werden."
//
// Zwei Sichten auf dieselben Regeln: der Stand EINES Mitarbeiters
// (Standing) und die Liste aller offenen Punkte des Betriebs (DueList).
// Beide stuetzen sich auf evaluateEmployeeStanding, damit sie nicht
// auseinanderlaufen; TestQualificationDueListMatchesStanding wacht
// darueber.

// qualificationCatalog sind die AKTIVEN Arten eines Mandanten.
type qualificationCatalog struct {
	types map[uuid.UUID]QualificationType
	// pflicht sind die Arten mit pflicht_fuer_alle.
	pflicht []uuid.UUID
}

func (s *QualificationService) loadActiveCatalog(ctx context.Context, companyID string) (qualificationCatalog, error) {
	c := qualificationCatalog{types: make(map[uuid.UUID]QualificationType), pflicht: make([]uuid.UUID, 0)}
	rows, err := s.pg.Query(ctx, `SELECT `+qualificationTypeColumns+` FROM hr_qualification_types WHERE company_id=$1 AND aktiv`, companyID)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		qt, err := scanQualificationType(rows)
		if err != nil {
			return c, err
		}
		c.types[qt.ID] = *qt
		if qt.PflichtFuerAlle {
			c.pflicht = append(c.pflicht, qt.ID)
		}
	}
	return c, rows.Err()
}

// evaluateEmployeeStanding wendet die Regeln fuer EINEN Mitarbeiter an:
//
//   - erforderlich ist, was ihm als Anforderung zugeordnet ist, und - wenn
//     er aktiv ist - was fuer alle Pflicht ist;
//   - stillgelegte Arten erscheinen nicht, weder ihre Nachweise noch
//     Anforderungen auf sie (sie stehen nicht im Katalog);
//   - stornierte Nachweise zaehlen nicht (EvaluateQualifications).
//
// Ergebnis mit Angaben zur Art, das Dringendste zuerst, darin nach Name.
// Reine Funktion ohne Datenbank.
func (c qualificationCatalog) evaluateEmployeeStanding(active bool, ownRequirements []uuid.UUID, records []EmployeeQualification, today time.Time, warnDays int) []EmployeeQualificationStanding {
	required := make([]uuid.UUID, 0, len(ownRequirements)+len(c.pflicht))
	for _, id := range ownRequirements {
		if _, ok := c.types[id]; ok {
			required = append(required, id)
		}
	}
	if active {
		required = append(required, c.pflicht...)
	}
	relevant := make([]EmployeeQualification, 0, len(records))
	for _, r := range records {
		if _, ok := c.types[r.TypeID]; ok {
			relevant = append(relevant, r)
		}
	}

	standings := EvaluateQualifications(relevant, required, today, warnDays)
	out := make([]EmployeeQualificationStanding, 0, len(standings))
	for _, st := range standings {
		qt := c.types[st.TypeID]
		out = append(out, EmployeeQualificationStanding{QualificationStanding: st, Code: qt.Code, Name: qt.Name, Kategorie: qt.Kategorie})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := qualificationStandRank(out[i].Status), qualificationStandRank(out[j].Status)
		if ri != rj {
			return ri < rj
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// QualificationDueEntry ist ein offener Punkt der betriebsweiten Liste:
// ein Mitarbeiter und eine Art, die fehlt, abgelaufen ist oder ablaeuft.
type QualificationDueEntry struct {
	EmployeeID uuid.UUID `json:"employee_id"`
	PersonalNr *string   `json:"personalnummer,omitempty"`
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	EmployeeQualificationStanding
}

// DueList liefert fuer alle AKTIVEN Mitarbeiter des Mandanten alles, was
// am Tag today fehlt, abgelaufen ist oder innerhalb der Vorwarnzeit
// ablaeuft. Gueltige Eintraege erscheinen nicht - die Liste ist die
// Arbeitsliste der Personalabteilung.
//
// Ausgeschiedene (inaktive) Mitarbeiter erscheinen nicht: bei ihnen ist
// nichts mehr zu erneuern.
//
// Sortierung: das Dringendste zuerst (fehlt, abgelaufen, laeuft ab), darin
// was am laengsten ueberfaellig ist bzw. am fruehesten ablaeuft, dann nach
// Name des Mitarbeiters und der Art.
//
// Die Daten werden in vier Abfragen fuer den ganzen Mandanten geladen,
// nicht je Mitarbeiter.
func (s *QualificationService) DueList(ctx context.Context, today time.Time, warnDays int, companyID string) ([]QualificationDueEntry, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}

	catalog, err := s.loadActiveCatalog(ctx, companyID)
	if err != nil {
		return nil, err
	}

	type person struct {
		id         uuid.UUID
		personalNr *string
		first      string
		last       string
	}
	people := make([]person, 0)
	empRows, err := s.pg.Query(ctx, `SELECT id, personalnummer, first_name, last_name FROM hr_employees WHERE company_id=$1 AND active`, companyID)
	if err != nil {
		return nil, err
	}
	for empRows.Next() {
		var p person
		if err := empRows.Scan(&p.id, &p.personalNr, &p.first, &p.last); err != nil {
			empRows.Close()
			return nil, err
		}
		people = append(people, p)
	}
	empRows.Close()
	if err := empRows.Err(); err != nil {
		return nil, err
	}

	requirements := make(map[uuid.UUID][]uuid.UUID)
	reqRows, err := s.pg.Query(ctx, `
		SELECT r.employee_id, r.type_id
		  FROM hr_employee_qualification_requirements r
		  JOIN hr_employees e ON e.id = r.employee_id
		 WHERE e.company_id=$1 AND e.active
	`, companyID)
	if err != nil {
		return nil, err
	}
	for reqRows.Next() {
		var emp, typ uuid.UUID
		if err := reqRows.Scan(&emp, &typ); err != nil {
			reqRows.Close()
			return nil, err
		}
		requirements[emp] = append(requirements[emp], typ)
	}
	reqRows.Close()
	if err := reqRows.Err(); err != nil {
		return nil, err
	}

	records := make(map[uuid.UUID][]EmployeeQualification)
	recRows, err := s.pg.Query(ctx, `
		SELECT `+qualificationColumns+`
		  FROM hr_employee_qualifications q
		  JOIN hr_employees e ON e.id = q.employee_id
		 WHERE e.company_id=$1 AND e.active AND q.status='active'
	`, companyID)
	if err != nil {
		return nil, err
	}
	for recRows.Next() {
		q, err := scanQualification(recRows)
		if err != nil {
			recRows.Close()
			return nil, err
		}
		records[q.EmployeeID] = append(records[q.EmployeeID], *q)
	}
	recRows.Close()
	if err := recRows.Err(); err != nil {
		return nil, err
	}

	out := make([]QualificationDueEntry, 0)
	for _, p := range people {
		for _, st := range catalog.evaluateEmployeeStanding(true, requirements[p.id], records[p.id], today, warnDays) {
			if st.Status == QualificationStandGueltig {
				continue
			}
			out = append(out, QualificationDueEntry{EmployeeID: p.id, PersonalNr: p.personalNr, FirstName: p.first, LastName: p.last, EmployeeQualificationStanding: st})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ra, rb := qualificationStandRank(a.Status), qualificationStandRank(b.Status); ra != rb {
			return ra < rb
		}
		// "fehlt" hat keine Tage; sonst: am weitesten ueberfaellig bzw.
		// am fruehesten ablaufend zuerst.
		if a.TageBisAblauf != nil && b.TageBisAblauf != nil && *a.TageBisAblauf != *b.TageBisAblauf {
			return *a.TageBisAblauf < *b.TageBisAblauf
		}
		if a.LastName != b.LastName {
			return a.LastName < b.LastName
		}
		if a.FirstName != b.FirstName {
			return a.FirstName < b.FirstName
		}
		if a.EmployeeID != b.EmployeeID {
			return a.EmployeeID.String() < b.EmployeeID.String()
		}
		return a.Name < b.Name
	})
	return out, nil
}
