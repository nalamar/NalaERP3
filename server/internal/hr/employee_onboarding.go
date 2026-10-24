package hr

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Anlage eines Mitarbeiters samt Anforderungen der Stelle und vorhandener
// Nachweise in EINEM Aufruf (Backlog F.10.5.1). Vorgabe des Nutzers vom
// 2026-10-13: beim Anlegen festlegen, welche Qualifikationen die Stelle
// braucht und welche schon vorhanden sind. Bisher waren das drei Aufrufe
// (F.7.3.3); schlug einer fehl, war der Mitarbeiter halb erfasst. Jetzt
// gilt: ganz oder gar nicht.

// querier ist, was Pool und Transaktion gemeinsam haben.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// InitialQualification ist ein bei der Anlage vorhandener Nachweis. Die
// Felder entsprechen QualificationInput ohne Mitarbeiter. Ohne JSON-Tags:
// die Daten liest der Handler als JJJJ-MM-TT wie bei POST /qualifications.
type InitialQualification struct {
	TypeID     uuid.UUID
	ErworbenAm time.Time
	GueltigBis *time.Time
	Aussteller string
	NachweisNr string
	Note       string
}

type EmployeeWithQualificationsInput struct {
	Employee EmployeeInput
	// RequiredTypeIDs: Anforderungen der Stelle (neu angelegt, also nur
	// aktive Arten des Mandanten).
	RequiredTypeIDs []uuid.UUID
	// PositionIDs: Stellen des Mitarbeiters (F.11.3.2; nur aktive Stellen
	// des Mandanten). Ihre Arten gelten wie in SetEmployeePositions.
	PositionIDs    []uuid.UUID
	Qualifications []InitialQualification
}

type EmployeeWithQualifications struct {
	Employee       *Employee                  `json:"employee"`
	Requirements   []QualificationRequirement `json:"requirements"`
	Positions      []Position                 `json:"positions"`
	Qualifications []EmployeeQualification    `json:"qualifications"`
}

func (q InitialQualification) input(employeeID uuid.UUID) QualificationInput {
	return QualificationInput{
		EmployeeIDs: []uuid.UUID{employeeID},
		TypeID:      q.TypeID,
		ErworbenAm:  q.ErworbenAm,
		GueltigBis:  q.GueltigBis,
		Aussteller:  q.Aussteller,
		NachweisNr:  q.NachweisNr,
		Note:        q.Note,
	}
}

// CreateEmployeeWithQualifications legt Mitarbeiter, Anforderungen und
// Nachweise in einer Transaktion an. Es gelten dieselben Regeln wie bei
// den Einzelaufrufen (EmployeeService.Create, SetRequirements, Create);
// scheitert eine, wird nichts gespeichert.
func (s *QualificationService) CreateEmployeeWithQualifications(ctx context.Context, in EmployeeWithQualificationsInput, actorUserID, companyID string) (*EmployeeWithQualifications, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	for _, id := range in.RequiredTypeIDs {
		if id == uuid.Nil {
			return nil, errors.New("type_id erforderlich")
		}
	}
	positionIDs, err := normalizePositionIDs(in.PositionIDs)
	if err != nil {
		return nil, err
	}
	// Vorab ohne Datenbank: der Mitarbeiter hat noch keine ID, deshalb
	// steht hier ein Platzhalter (validateQualificationInput prueft nur,
	// dass einer genannt ist).
	for _, q := range in.Qualifications {
		if err := validateQualificationInput(q.input(uuid.New())); err != nil {
			return nil, err
		}
	}

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	emp, err := createEmployee(ctx, tx, in.Employee, companyID)
	if err != nil {
		return nil, err
	}
	if err := setRequirementsTx(ctx, tx, emp.ID, uniqueTypeIDs(in.RequiredTypeIDs), actorUserID, companyID); err != nil {
		return nil, err
	}
	if err := setEmployeePositionsTx(ctx, tx, emp.ID, positionIDs, actorUserID, companyID); err != nil {
		return nil, err
	}
	records := make([]EmployeeQualification, 0, len(in.Qualifications))
	for _, q := range in.Qualifications {
		out, err := createQualificationsTx(ctx, tx, q.input(emp.ID), actorUserID, companyID)
		if err != nil {
			return nil, err
		}
		records = append(records, out...)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	reqs, err := s.ListRequirements(ctx, emp.ID, companyID)
	if err != nil {
		return nil, err
	}
	positions, err := s.ListEmployeePositions(ctx, emp.ID, companyID)
	if err != nil {
		return nil, err
	}
	return &EmployeeWithQualifications{Employee: emp, Requirements: reqs, Positions: positions, Qualifications: records}, nil
}
