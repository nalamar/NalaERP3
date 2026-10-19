package hr

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Teams anlegen und pflegen (ADR 0029, Backlog F.8.5). Teams haben vorerst
// KEINE Wirkung auf Rechte - Urlaub genehmigen nur Personalabteilung und
// Geschaeftsfuehrung (Entscheidung des Nutzers vom 2026-10-19). Der
// Teamleiter wird schon gepflegt, damit LeaveDecisionRight/mayDecide ihn
// spaeter nutzen kann; heute dient das Team der Abwesenheitsuebersicht.

// TeamInput ersetzt Name und Teamleiter eines Teams vollstaendig.
type TeamInput struct {
	Name string     `json:"name"`
	Lead *uuid.UUID `json:"lead_employee_id"`
}

// validateTeam prueft Name (Pflicht, je Mandant ohne Ruecksicht auf
// Gross-/Kleinschreibung eindeutig) und Teamleiter (Mitarbeiter des
// Mandanten). except: das Team selbst bei einer Aenderung.
func (s *EmployeeService) validateTeam(ctx context.Context, in *TeamInput, except *uuid.UUID, companyID string) error {
	if strings.TrimSpace(companyID) == "" {
		return errors.New("Mandant erforderlich")
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return errors.New("Teamname erforderlich")
	}
	var taken bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_teams WHERE company_id=$1 AND lower(name)=lower($2) AND ($3::uuid IS NULL OR id<>$3))`,
		companyID, in.Name, except).Scan(&taken); err != nil {
		return err
	}
	if taken {
		return errors.New("Team mit diesem Namen existiert bereits")
	}
	if in.Lead != nil {
		var owned bool
		if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_employees WHERE id=$1 AND company_id=$2)`, *in.Lead, companyID).Scan(&owned); err != nil {
			return err
		}
		if !owned {
			return errors.New("Mitarbeiter nicht gefunden")
		}
	}
	return nil
}

// GetTeam liefert ein Team des Mandanten mit Mitgliederzahl.
func (s *EmployeeService) GetTeam(ctx context.Context, id uuid.UUID, companyID string) (*Team, error) {
	var t Team
	err := s.pg.QueryRow(ctx, `SELECT t.id, t.name, t.lead_employee_id, (SELECT count(*) FROM hr_employees e WHERE e.team_id=t.id)
		FROM hr_teams t WHERE t.id=$1 AND t.company_id=$2`, id, companyID).Scan(&t.ID, &t.Name, &t.Lead, &t.Mitglieder)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("Team nicht gefunden")
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateTeam legt ein Team an.
func (s *EmployeeService) CreateTeam(ctx context.Context, in TeamInput, companyID string) (*Team, error) {
	if err := s.validateTeam(ctx, &in, nil, companyID); err != nil {
		return nil, err
	}
	id := uuid.New()
	if _, err := s.pg.Exec(ctx, `INSERT INTO hr_teams (id, name, lead_employee_id, company_id) VALUES ($1,$2,$3,$4)`, id, in.Name, in.Lead, companyID); err != nil {
		return nil, err
	}
	return s.GetTeam(ctx, id, companyID)
}

// UpdateTeam ersetzt Name und Teamleiter; Teamleiter nil entfernt ihn.
func (s *EmployeeService) UpdateTeam(ctx context.Context, id uuid.UUID, in TeamInput, companyID string) (*Team, error) {
	if err := s.validateTeam(ctx, &in, &id, companyID); err != nil {
		return nil, err
	}
	if _, err := s.pg.Exec(ctx, `UPDATE hr_teams SET name=$2, lead_employee_id=$3 WHERE id=$1 AND company_id=$4`, id, in.Name, in.Lead, companyID); err != nil {
		return nil, err
	}
	// Ein unbekanntes oder fremdes Team meldet GetTeam als nicht gefunden.
	return s.GetTeam(ctx, id, companyID)
}

// DeleteTeam loescht ein Team ohne Mitglieder. Mit Mitgliedern muessen
// diese zuerst einem anderen oder keinem Team zugeordnet werden - sonst
// verloeren sie still ihre Zuordnung.
func (s *EmployeeService) DeleteTeam(ctx context.Context, id uuid.UUID, companyID string) error {
	if _, err := s.GetTeam(ctx, id, companyID); err != nil {
		return err
	}
	// Die Pruefung auf Mitglieder steht im DELETE selbst - so schliesst sie
	// auch ein gleichzeitig zugeordnetes Mitglied aus.
	ct, err := s.pg.Exec(ctx, `DELETE FROM hr_teams t WHERE t.id=$1 AND t.company_id=$2 AND NOT EXISTS (SELECT 1 FROM hr_employees e WHERE e.team_id=t.id)`, id, companyID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return errors.New("Team hat noch Mitglieder - zuerst umsetzen")
	}
	return nil
}
