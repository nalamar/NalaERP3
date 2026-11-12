package production

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Hinweise der Montagetermine und Wochenuebersicht je Team (ADR 0038
// Frage 6 (a), Backlog H.3.6c). Hinweise sind keine Sperren und werden
// nicht gespeichert; stornierte Termine haben keine. Die Abwesenheit im
// Team nennt keine Person (DSGVO).

// assemblyFactsSelect ermittelt je Termin, was die Regeln brauchen:
// Status und Liefertermin des Auftrags, die spaeteste geplante Woche der
// offenen Arbeitsgaenge, ein anderer aktiver Termin desselben Teams im
// Zeitraum, eine geltende Abwesenheit eines aktiven Teammitglieds im
// Zeitraum und ob die juengste HU/UVV des Fahrzeugs es sperrt (Stand der
// erfassten Pruefungen, wie fleet.assignmentBlock).
const assemblyFactsSelect = `SELECT a.id, COALESCE(o.status, ''), o.liefertermin,
	(SELECT max(op.geplante_woche) FROM production_order_operations op
	  WHERE op.order_id = a.order_id
	    AND NOT EXISTS (SELECT 1 FROM production_operation_feedback f WHERE f.operation_id = op.id AND f.zurueckgenommen_at IS NULL)),
	a.team_id IS NOT NULL AND EXISTS (SELECT 1 FROM production_assembly_appointments b
	  WHERE b.team_id = a.team_id AND b.company_id = a.company_id AND b.id <> a.id
	    AND b.storniert_at IS NULL AND b.von <= a.bis AND b.bis >= a.von),
	a.team_id IS NOT NULL AND EXISTS (SELECT 1 FROM hr_absences x JOIN hr_employees e ON e.id = x.employee_id
	  WHERE e.team_id = a.team_id AND e.active AND x.status = 'approved'
	    AND x.start_date <= a.bis AND x.end_date >= a.von),
	a.vehicle_id IS NOT NULL AND EXISTS (SELECT 1 FROM fleet_vehicle_schedules s
	  WHERE s.vehicle_id = a.vehicle_id AND s.aktiv AND s.art IN ('hu', 'uvv')
	    AND (SELECT c.ergebnis FROM fleet_vehicle_completions c WHERE c.schedule_id = s.id AND c.status = 'active'
	          ORDER BY c.erledigt_am DESC, c.created_at DESC LIMIT 1) IN ('maengel', 'nicht_bestanden'))
	  FROM production_assembly_appointments a
	  LEFT JOIN production_orders o ON o.id = a.order_id
	 WHERE a.id = ANY($1) AND a.company_id = $2`

// fillAssemblyHints setzt die Hinweise der nicht stornierten Termine
// (scanAppointment legt sie leer an).
func fillAssemblyHints(ctx context.Context, q querier, list []Appointment, companyID string, today time.Time) error {
	idx := map[uuid.UUID]int{}
	ids := []uuid.UUID{}
	for i := range list {
		if list[i].StorniertAt == nil {
			idx[list[i].ID] = i
			ids = append(ids, list[i].ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.Query(ctx, assemblyFactsSelect, ids, companyID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id                       uuid.UUID
			status                   string
			liefertermin, lastOpen   *time.Time
			doppelt, abwesend, sperr bool
		)
		if err := rows.Scan(&id, &status, &liefertermin, &lastOpen, &doppelt, &abwesend, &sperr); err != nil {
			return err
		}
		x := &list[idx[id]]
		x.Hinweise = AssemblyHints(AssemblyFacts{
			TeamDoppelt:       doppelt,
			TeamAbwesend:      abwesend,
			FahrzeugGesperrt:  sperr,
			VorFertigstellung: AssemblyBeforeCompletion(status, x.Von, lastOpen, today),
			NachLiefertermin:  AssemblyAfterDelivery(x.Bis, liefertermin),
		})
	}
	return rows.Err()
}

// AssemblyTeamRow: aktive Termine eines Teams im Zeitraum; TeamID nil ist
// "ohne Team".
type AssemblyTeamRow struct {
	TeamID   *uuid.UUID    `json:"team_id"`
	TeamName string        `json:"team_name"`
	Termine  []Appointment `json:"termine"`
}

type AssemblyOverview struct {
	// Montag der ersten bis Sonntag der letzten Woche
	Von   time.Time         `json:"von"`
	Bis   time.Time         `json:"bis"`
	Teams []AssemblyTeamRow `json:"teams"`
}

// AssemblyWeeks: alle Teams des Mandanten nach Name mit ihren nicht
// stornierten Terminen, die den Zeitraum beruehren (ganze Wochen,
// hoechstens 26), danach "ohne Team", falls es solche Termine gibt.
func (s *Service) AssemblyWeeks(ctx context.Context, von, bis time.Time, companyID string) (*AssemblyOverview, error) {
	weeks, err := capacityWeeks(von, bis)
	if err != nil {
		return nil, err
	}
	out := &AssemblyOverview{Von: weeks[0], Bis: weeks[len(weeks)-1].AddDate(0, 0, 6), Teams: []AssemblyTeamRow{}}
	rows, err := s.pg.Query(ctx, `SELECT id, name FROM hr_teams WHERE company_id=$1 ORDER BY lower(name), id`, companyID)
	if err != nil {
		return nil, err
	}
	teamIdx := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		teamIdx[id] = len(out.Teams)
		out.Teams = append(out.Teams, AssemblyTeamRow{TeamID: &id, TeamName: name, Termine: []Appointment{}})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	list, err := s.queryAppointments(ctx, `
		 WHERE a.company_id=$1 AND a.storniert_at IS NULL AND a.von <= $3 AND a.bis >= $2
		 ORDER BY a.von, a.created_at, a.id`, companyID, companyID, out.Von, out.Bis)
	if err != nil {
		return nil, err
	}
	ohne := AssemblyTeamRow{Termine: []Appointment{}}
	for _, x := range list {
		if x.TeamID != nil {
			if i, ok := teamIdx[*x.TeamID]; ok {
				out.Teams[i].Termine = append(out.Teams[i].Termine, x)
				continue
			}
		}
		ohne.Termine = append(ohne.Termine, x)
	}
	if len(ohne.Termine) > 0 {
		out.Teams = append(out.Teams, ohne)
	}
	return out, nil
}
