package assets

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nalaerp3/internal/hr"
)

// Pruefungen von Betriebsmitteln (ADR 0028, Backlog F.3.5.1): erfassen,
// stornieren, lesen. Unveraenderlich wie die Nachweise der
// Qualifikationen - eine Pruefung hat Beweisfunktion gegenueber
// Berufsgenossenschaft und Aufsicht. Fehler werden storniert und neu
// erfasst; eine Wiederholung ist eine NEUE Pruefung.

type InspectionInput struct {
	AssetID    uuid.UUID `json:"asset_id"`
	GeprueftAm time.Time `json:"geprueft_am"`
	Ergebnis   string    `json:"ergebnis"`
	// NaechstePruefungAm: nil = aus dem Pruefintervall des Assets
	// berechnen (bei nicht bestandener Pruefung: keine).
	NaechstePruefungAm *time.Time `json:"naechste_pruefung_am"`
	PrueferName        string     `json:"pruefer_name"`
	PrueferEmployeeID  *uuid.UUID `json:"pruefer_employee_id"`
	Notiz              string     `json:"notiz"`
}

const inspectionColumns = `i.id, i.asset_id, i.geprueft_am, i.ergebnis, i.naechste_pruefung_am, i.pruefer_name,
	i.pruefer_employee_id, i.notiz, i.status, i.storno_reason, i.storniert_at, i.storniert_by_user_id,
	i.created_by_user_id, i.created_at`

func scanInspection(row pgx.Row) (*Inspection, error) {
	var i Inspection
	if err := row.Scan(&i.ID, &i.AssetID, &i.GeprueftAm, &i.Ergebnis, &i.NaechstePruefungAm, &i.PrueferName,
		&i.PrueferEmployeeID, &i.Notiz, &i.Status, &i.StornoReason, &i.StorniertAt, &i.StorniertByUserID,
		&i.CreatedByUserID, &i.CreatedAt); err != nil {
		return nil, err
	}
	return &i, nil
}

// RecordInspection erfasst eine Pruefung.
//
// Die naechste Pruefung wird hier FESTGELEGT und gespeichert (angegeben
// oder aus dem Pruefintervall berechnet). Aendert der Betrieb spaeter das
// Intervall, gilt eine erfolgte Pruefung nicht rueckwirkend anders.
//
// Die Asset-Zeile wird gesperrt wie bei Ausgabe und Ausmusterung: eine
// gleichzeitige Ausgabe sieht die Pruefung entweder ganz oder gar nicht.
func (s *Service) RecordInspection(ctx context.Context, in InspectionInput, actorUserID, companyID string) (*Inspection, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.AssetID == uuid.Nil {
		return nil, errors.New("asset_id erforderlich")
	}
	in.Ergebnis = strings.TrimSpace(in.Ergebnis)
	in.PrueferName = strings.TrimSpace(in.PrueferName)
	in.Notiz = strings.TrimSpace(in.Notiz)

	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ausgemustertAm, anschaffungsdatum *time.Time
	var intervall *int
	if err := tx.QueryRow(ctx, `SELECT ausgemustert_am, anschaffungsdatum, pruefintervall_monate FROM assets WHERE id=$1 AND company_id=$2 FOR UPDATE`, in.AssetID, companyID).
		Scan(&ausgemustertAm, &anschaffungsdatum, &intervall); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Asset nicht gefunden")
		}
		return nil, err
	}
	if ausgemustertAm != nil {
		return nil, errors.New("Asset ist ausgemustert")
	}
	if err := ValidateInspection(in.GeprueftAm, in.Ergebnis, in.NaechstePruefungAm, in.PrueferName, anschaffungsdatum, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	if in.PrueferEmployeeID != nil {
		var owned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_employees WHERE id=$1 AND company_id=$2)`, *in.PrueferEmployeeID, companyID).Scan(&owned); err != nil {
			return nil, err
		}
		if !owned {
			return nil, errors.New("Mitarbeiter nicht gefunden")
		}
	}

	geprueftAm := dateOnly(in.GeprueftAm)
	naechste := in.NaechstePruefungAm
	if naechste != nil {
		d := dateOnly(*naechste)
		naechste = &d
	} else if in.Ergebnis != ErgebnisNichtBestanden {
		naechste = NextInspectionDue(geprueftAm, intervall)
	}

	i, err := scanInspection(tx.QueryRow(ctx, `
		INSERT INTO asset_inspections AS i (id, asset_id, geprueft_am, ergebnis, naechste_pruefung_am, pruefer_name, pruefer_employee_id, notiz, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING `+inspectionColumns,
		uuid.New(), in.AssetID, geprueftAm, in.Ergebnis, naechste, in.PrueferName, in.PrueferEmployeeID, in.Notiz, nullableUserID(actorUserID)))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return i, nil
}

// StornoInspection storniert eine falsch erfasste Pruefung. Einmalig, mit
// Begruendung. Ist sie die juengste, wird danach wieder die vorherige
// massgeblich - auch eine Sperre durch "nicht bestanden" faellt so weg.
func (s *Service) StornoInspection(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*Inspection, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Prüfungs-ID erforderlich")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("Storno-Begründung erforderlich")
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	if err := tx.QueryRow(ctx, `
		SELECT i.status
		  FROM asset_inspections i
		  JOIN assets a ON a.id = i.asset_id
		 WHERE i.id=$1 AND a.company_id=$2
		   FOR UPDATE OF i
	`, id, companyID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Prüfung nicht gefunden")
		}
		return nil, err
	}
	if status == InspectionStorniert {
		return nil, errors.New("Prüfung ist bereits storniert")
	}
	ct, err := tx.Exec(ctx, `
		UPDATE asset_inspections SET status='storniert', storniert_at=now(), storno_reason=$2, storniert_by_user_id=$3
		 WHERE id=$1 AND status='active'
	`, id, reason, nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		return nil, errors.New("Prüfung ist bereits storniert")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetInspection(ctx, id, companyID)
}

func (s *Service) GetInspection(ctx context.Context, id uuid.UUID, companyID string) (*Inspection, error) {
	i, err := scanInspection(s.pg.QueryRow(ctx, `SELECT `+inspectionColumns+`
		  FROM asset_inspections i
		  JOIN assets a ON a.id = i.asset_id
		 WHERE i.id=$1 AND a.company_id=$2`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Prüfung nicht gefunden")
		}
		return nil, err
	}
	return i, nil
}

// ListInspections liefert die Pruefungen eines Assets, die juengste
// zuerst (dieselbe Reihenfolge, nach der EvaluateInspections die
// massgebliche waehlt). Stornierte nur auf Wunsch.
func (s *Service) ListInspections(ctx context.Context, assetID uuid.UUID, includeStorniert bool, companyID string) ([]Inspection, error) {
	var owned bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM assets WHERE id=$1 AND company_id=$2)`, assetID, companyID).Scan(&owned); err != nil {
		return nil, err
	}
	if !owned {
		return nil, errors.New("Asset nicht gefunden")
	}
	rows, err := s.pg.Query(ctx, `SELECT `+inspectionColumns+`
		  FROM asset_inspections i
		 WHERE i.asset_id=$1 AND (i.status='active' OR $2)
		 ORDER BY i.geprueft_am DESC, i.created_at DESC, i.id`, assetID, includeStorniert)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Inspection, 0)
	for rows.Next() {
		i, err := scanInspection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *i)
	}
	return out, rows.Err()
}
