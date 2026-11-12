package production

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Auswahl fuer Montagetermine (Backlog H.3.6d): Teams nur mit Name und
// Fahrzeuge nur mit Kennzeichen und Bezeichnung - keine Personendaten -,
// lesbar mit production.read statt employees.read und fleet.read.

type AssemblyTeam struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type AssemblyVehicle struct {
	ID          uuid.UUID `json:"id"`
	Kennzeichen string    `json:"kennzeichen"`
	Bezeichnung string    `json:"bezeichnung"`
	// Abmeldung in der Zukunft: bis dahin waehlbar.
	AbgemeldetAm *time.Time `json:"abgemeldet_am"`
}

type AssemblyChoices struct {
	Teams     []AssemblyTeam    `json:"teams"`
	Fahrzeuge []AssemblyVehicle `json:"fahrzeuge"`
}

// AssemblyChoices: Teams des Mandanten nach Name; Fahrzeuge, die heute
// nicht abgemeldet sind, nach Kennzeichen.
func (s *Service) AssemblyChoices(ctx context.Context, companyID string) (*AssemblyChoices, error) {
	out := &AssemblyChoices{Teams: []AssemblyTeam{}, Fahrzeuge: []AssemblyVehicle{}}
	rows, err := s.pg.Query(ctx, `SELECT id, name FROM hr_teams WHERE company_id=$1 ORDER BY lower(name), id`, companyID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x AssemblyTeam
		if err := rows.Scan(&x.ID, &x.Name); err != nil {
			rows.Close()
			return nil, err
		}
		out.Teams = append(out.Teams, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = s.pg.Query(ctx, `SELECT id, kennzeichen, bezeichnung, abgemeldet_am FROM fleet_vehicles
		WHERE company_id=$1 AND (abgemeldet_am IS NULL OR abgemeldet_am > $2)
		ORDER BY kennzeichen, id`, companyID, dateOnly(time.Now()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var x AssemblyVehicle
		if err := rows.Scan(&x.ID, &x.Kennzeichen, &x.Bezeichnung, &x.AbgemeldetAm); err != nil {
			return nil, err
		}
		out.Fahrzeuge = append(out.Fahrzeuge, x)
	}
	return out, rows.Err()
}
