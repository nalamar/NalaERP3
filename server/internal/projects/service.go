package projects

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"nalaerp3/internal/settings"
)

type Service struct{ pg *pgxpool.Pool }

func NewService(pg *pgxpool.Pool) *Service { return &Service{pg: pg} }

// Project represents a top-level project.
type Project struct {
	ID       string    `json:"id"`
	Nummer   string    `json:"nummer"`
	Name     string    `json:"name"`
	KundeID  string    `json:"kunde_id"`
	Status   string    `json:"status"`
	Angelegt time.Time `json:"angelegt_am"`
}

type QuoteSnapshot struct {
	Project       Project     `json:"project"`
	CustomerName  string      `json:"customer_name"`
	CustomerEmail string      `json:"customer_email"`
	CustomerPhone string      `json:"customer_phone"`
	Positionen    []QuoteLine `json:"positionen"`
}

type QuoteLine struct {
	PhaseNummer     string   `json:"phase_nummer"`
	PhaseName       string   `json:"phase_name"`
	PositionsNummer string   `json:"positions_nummer"`
	PositionsName   string   `json:"positions_name"`
	Beschreibung    string   `json:"beschreibung"`
	Menge           float64  `json:"menge"`
	WidthMM         *float64 `json:"width_mm"`
	HeightMM        *float64 `json:"height_mm"`
	Serie           string   `json:"serie"`
	Oberflaeche     string   `json:"oberflaeche"`
}

type ProjectCreate struct {
	Nummer  string `json:"nummer"`
	Name    string `json:"name"`
	KundeID string `json:"kunde_id"`
	Status  string `json:"status"`
}

type ProjectFilter struct {
	Q      string
	Status string
	Limit  int
	Offset int
}

func (s *Service) UpdateStatus(ctx context.Context, id, status string) (*Project, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("Projekt-ID erforderlich")
	}
	if strings.TrimSpace(status) == "" {
		return nil, errors.New("Status erforderlich")
	}
	if _, err := s.pg.Exec(ctx, `UPDATE projects SET status=$2 WHERE id=$1`, id, strings.TrimSpace(status)); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, in ProjectCreate) (*Project, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("Name erforderlich")
	}
	if strings.TrimSpace(in.Status) == "" {
		in.Status = "neu"
	}
	if strings.TrimSpace(in.Nummer) == "" {
		// use configured numbering for projects
		numSvc := settings.NewNumberingService(s.pg)
		if n, err := numSvc.Next(ctx, "project"); err == nil {
			in.Nummer = n
		} else {
			in.Nummer = uuid.NewString()
		}
	}
	id := uuid.NewString()
	var p Project
	err := s.pg.QueryRow(ctx, `
        INSERT INTO projects (id, nummer, name, kunde_id, status)
        VALUES ($1,$2,$3,$4,$5)
        RETURNING id, nummer, name, COALESCE(kunde_id,''), status, angelegt_am
    `, id, in.Nummer, in.Name, nullIfEmpty(in.KundeID), in.Status).Scan(&p.ID, &p.Nummer, &p.Name, &p.KundeID, &p.Status, &p.Angelegt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Project, error) {
	var p Project
	err := s.pg.QueryRow(ctx, `SELECT id, nummer, name, COALESCE(kunde_id,''), status, angelegt_am FROM projects WHERE id=$1`, id).Scan(
		&p.ID, &p.Nummer, &p.Name, &p.KundeID, &p.Status, &p.Angelegt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) BuildQuoteSnapshot(ctx context.Context, id string) (*QuoteSnapshot, error) {
	var snap QuoteSnapshot
	err := s.pg.QueryRow(ctx, `
        SELECT p.id, p.nummer, p.name, COALESCE(p.kunde_id,''), p.status, p.angelegt_am,
               COALESCE(c.name,''), COALESCE(c.email,''), COALESCE(c.telefon,'')
        FROM projects p
        LEFT JOIN contacts c ON c.id = p.kunde_id
        WHERE p.id = $1
    `, id).Scan(
		&snap.Project.ID,
		&snap.Project.Nummer,
		&snap.Project.Name,
		&snap.Project.KundeID,
		&snap.Project.Status,
		&snap.Project.Angelegt,
		&snap.CustomerName,
		&snap.CustomerEmail,
		&snap.CustomerPhone,
	)
	if err != nil {
		return nil, err
	}

	rows, err := s.pg.Query(ctx, `
        SELECT ph.nummer,
               COALESCE(ph.name,''),
               el.nummer,
               el.name,
               COALESCE(el.beschreibung,''),
               el.menge,
               el.width_mm,
               el.height_mm,
               COALESCE(el.serie,''),
               COALESCE(el.oberflaeche,'')
        FROM project_phases ph
        JOIN project_elevations el ON el.phase_id = ph.id
        WHERE ph.project_id = $1
        ORDER BY ph.sort_order ASC, ph.nummer ASC, el.nummer ASC
    `, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	snap.Positionen = make([]QuoteLine, 0)
	for rows.Next() {
		var line QuoteLine
		if err := rows.Scan(
			&line.PhaseNummer,
			&line.PhaseName,
			&line.PositionsNummer,
			&line.PositionsName,
			&line.Beschreibung,
			&line.Menge,
			&line.WidthMM,
			&line.HeightMM,
			&line.Serie,
			&line.Oberflaeche,
		); err != nil {
			return nil, err
		}
		snap.Positionen = append(snap.Positionen, line)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (s *Service) List(ctx context.Context, f ProjectFilter) ([]Project, error) {
	lim := f.Limit
	if lim <= 0 || lim > 200 {
		lim = 50
	}
	off := f.Offset
	sb := strings.Builder{}
	sb.WriteString(`SELECT id, nummer, name, COALESCE(kunde_id,''), status, angelegt_am FROM projects`)
	var conds []string
	var args []any
	idx := 1
	if strings.TrimSpace(f.Q) != "" {
		conds = append(conds, fmt.Sprintf("(nummer ILIKE $%d OR name ILIKE $%d)", idx, idx+1))
		q := "%" + f.Q + "%"
		args = append(args, q, q)
		idx += 2
	}
	if strings.TrimSpace(f.Status) != "" {
		conds = append(conds, fmt.Sprintf("status=$%d", idx))
		args = append(args, f.Status)
		idx++
	}
	if len(conds) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conds, " AND "))
	}
	sb.WriteString(" ORDER BY angelegt_am DESC")
	sb.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", lim, off))
	rows, err := s.pg.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Project, 0, lim)
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Nummer, &p.Name, &p.KundeID, &p.Status, &p.Angelegt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

// Import Log listing
type ImportRun struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"project_id"`
	Source            string    `json:"source"`
	ImportedAt        time.Time `json:"imported_at"`
	CreatedPhases     int       `json:"created_phases"`
	UpdatedPhases     int       `json:"updated_phases"`
	CreatedElevations int       `json:"created_elevations"`
	UpdatedElevations int       `json:"updated_elevations"`
	CreatedVariants   int       `json:"created_variants"`
	UpdatedVariants   int       `json:"updated_variants"`
	DeletedVariants   int       `json:"deleted_variants"`
	MaterialsReplaced int       `json:"materials_replaced_variants"`
}
type ImportChange struct {
	ID          string         `json:"id"`
	ImportID    string         `json:"import_id"`
	Kind        string         `json:"kind"`
	Action      string         `json:"action"`
	InternalID  string         `json:"internal_id"`
	ExternalRef string         `json:"external_ref"`
	Message     string         `json:"message"`
	Before      map[string]any `json:"before_data"`
	After       map[string]any `json:"after_data"`
	CreatedAt   time.Time      `json:"created_at"`
}

func (s *Service) ListImports(ctx context.Context, projectID string) ([]ImportRun, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, project_id, COALESCE(source,''), imported_at, created_phases, updated_phases, created_elevations, updated_elevations, created_variants, updated_variants, deleted_variants, materials_replaced_variants FROM project_imports WHERE project_id=$1 ORDER BY imported_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ImportRun, 0)
	for rows.Next() {
		var r ImportRun
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Source, &r.ImportedAt, &r.CreatedPhases, &r.UpdatedPhases, &r.CreatedElevations, &r.UpdatedElevations, &r.CreatedVariants, &r.UpdatedVariants, &r.DeletedVariants, &r.MaterialsReplaced); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Service) ListImportChanges(ctx context.Context, importID string) ([]ImportChange, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, import_id, kind, action, COALESCE(internal_id::text,''), COALESCE(external_ref,''), COALESCE(message,''), COALESCE(before_data,'{}'::jsonb), COALESCE(after_data,'{}'::jsonb), created_at FROM project_import_changes WHERE import_id=$1 ORDER BY created_at ASC, id ASC`, importID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ImportChange, 0)
	for rows.Next() {
		var c ImportChange
		var b, a []byte
		if err := rows.Scan(&c.ID, &c.ImportID, &c.Kind, &c.Action, &c.InternalID, &c.ExternalRef, &c.Message, &b, &a, &c.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(b, &c.Before)
		_ = json.Unmarshal(a, &c.After)
		out = append(out, c)
	}
	return out, nil
}

func (s *Service) ListImportChangesFiltered(ctx context.Context, importID, kind, action string) ([]ImportChange, error) {
	q := `SELECT id, import_id, kind, action, COALESCE(internal_id::text,''), COALESCE(external_ref,''), COALESCE(message,''), COALESCE(before_data,'{}'::jsonb), COALESCE(after_data,'{}'::jsonb), created_at FROM project_import_changes WHERE import_id=$1`
	args := []any{importID}
	if strings.TrimSpace(kind) != "" {
		q += " AND kind=$2"
		args = append(args, kind)
	}
	if strings.TrimSpace(action) != "" {
		if len(args) == 1 {
			q += " AND action=$2"
			args = append(args, action)
		} else {
			q += " AND action=$3"
			args = append(args, action)
		}
	}
	q += " ORDER BY created_at ASC, id ASC"
	rows, err := s.pg.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ImportChange, 0)
	for rows.Next() {
		var c ImportChange
		var b, a []byte
		if err := rows.Scan(&c.ID, &c.ImportID, &c.Kind, &c.Action, &c.InternalID, &c.ExternalRef, &c.Message, &b, &a, &c.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(b, &c.Before)
		_ = json.Unmarshal(a, &c.After)
		out = append(out, c)
	}
	return out, nil
}

// UndoImport reverts a given import run based on recorded before_data/after_data.
func (s *Service) UndoImport(ctx context.Context, importID string) error {
	// Load changes in reverse order
	rows, err := s.pg.Query(ctx, `SELECT kind, action, COALESCE(internal_id::text,''), COALESCE(before_data,'{}'::jsonb) FROM project_import_changes WHERE import_id=$1 ORDER BY created_at DESC, id DESC`, importID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, action, internalID string
		var braw []byte
		if err := rows.Scan(&kind, &action, &internalID, &braw); err != nil {
			return err
		}
		var before map[string]any
		_ = json.Unmarshal(braw, &before)
		switch kind + "/" + action {
		case "materials/replaced":
			// restore materials for variant internalID from before
			if internalID == "" || before == nil {
				continue
			}
			_, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_profiles WHERE single_elevation_id=$1`, internalID)
			_, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_articles WHERE single_elevation_id=$1`, internalID)
			_, _ = s.pg.Exec(ctx, `DELETE FROM single_elevation_glass WHERE single_elevation_id=$1`, internalID)
			// profiles
			if arr, ok := before["profiles"].([]any); ok {
				for _, it := range arr {
					m := it.(map[string]any)
					_, _ = s.pg.Exec(ctx, `INSERT INTO single_elevation_profiles (id, single_elevation_id, supplier_code, article_code, description, length_mm, qty, unit) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
						uuid.NewString(), internalID, m["supplier_code"], m["article_code"], m["description"], m["length_mm"], m["qty"], m["unit"])
				}
			}
			// articles
			if arr, ok := before["articles"].([]any); ok {
				for _, it := range arr {
					m := it.(map[string]any)
					_, _ = s.pg.Exec(ctx, `INSERT INTO single_elevation_articles (id, single_elevation_id, supplier_code, article_code, description, qty, unit) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
						uuid.NewString(), internalID, m["supplier_code"], m["article_code"], m["description"], m["qty"], m["unit"])
				}
			}
			// glass
			if arr, ok := before["glass"].([]any); ok {
				for _, it := range arr {
					m := it.(map[string]any)
					_, _ = s.pg.Exec(ctx, `INSERT INTO single_elevation_glass (id, single_elevation_id, configuration, description, width_mm, height_mm, area_m2, qty, unit) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
						uuid.NewString(), internalID, m["configuration"], m["description"], m["width_mm"], m["height_mm"], m["area_m2"], m["qty"], m["unit"])
				}
			}
		case "variant/created":
			// delete created variant
			if internalID == "" {
				continue
			}
			_, _ = s.pg.Exec(ctx, `DELETE FROM project_single_elevations WHERE id=$1`, internalID)
		case "variant/updated":
			// restore previous values
			if internalID == "" || before == nil {
				continue
			}
			_, _ = s.pg.Exec(ctx, `UPDATE project_single_elevations SET name=$1, beschreibung=$2, menge=$3, selected=$4, external_guid=$5 WHERE id=$6`,
				before["name"], before["beschreibung"], before["menge"], before["selected"], before["external_guid"], internalID)
		case "variant/deleted":
			// recreate deleted variant with same id
			if internalID == "" || before == nil {
				continue
			}
			_, _ = s.pg.Exec(ctx, `INSERT INTO project_single_elevations (id, elevation_id, name, beschreibung, menge, selected, external_guid) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				internalID, before["elevation_id"], before["name"], before["beschreibung"], before["menge"], before["selected"], before["external_guid"])
		case "elevation/created":
			if internalID == "" {
				continue
			}
			_, _ = s.pg.Exec(ctx, `DELETE FROM project_elevations WHERE id=$1`, internalID)
		case "elevation/updated":
			if internalID == "" || before == nil {
				continue
			}
			_, _ = s.pg.Exec(ctx, `UPDATE project_elevations SET nummer=$1, name=$2, beschreibung=$3, menge=$4, width_mm=$5, height_mm=$6, external_guid=$7 WHERE id=$8`,
				before["nummer"], before["name"], before["beschreibung"], before["menge"], before["width_mm"], before["height_mm"], before["external_guid"], internalID)
		case "phase/created":
			if internalID == "" {
				continue
			}
			_, _ = s.pg.Exec(ctx, `DELETE FROM project_phases WHERE id=$1`, internalID)
		case "phase/updated":
			if internalID == "" || before == nil {
				continue
			}
			_, _ = s.pg.Exec(ctx, `UPDATE project_phases SET nummer=$1, name=$2, beschreibung=$3, sort_order=$4 WHERE id=$5`, before["nummer"], before["name"], before["beschreibung"], before["sort_order"], internalID)
		}
	}
	return nil
}

// ExportImportChangesCSV renders CSV for a given import run.
func (s *Service) ExportImportChangesCSV(ctx context.Context, importID string) (string, error) {
	changes, err := s.ListImportChanges(ctx, importID)
	if err != nil {
		return "", err
	}
	b := &strings.Builder{}
	w := csv.NewWriter(b)
	_ = w.Write([]string{"time", "kind", "action", "internal_id", "external_ref", "message"})
	for _, c := range changes {
		_ = w.Write([]string{c.CreatedAt.Format(time.RFC3339), c.Kind, c.Action, c.InternalID, c.ExternalRef, c.Message})
	}
	w.Flush()
	return b.String(), nil
}

// ----- Phases (Lose)
type Phase struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	Nummer       string    `json:"nummer"`
	Name         string    `json:"name"`
	Beschreibung string    `json:"beschreibung"`
	SortOrder    int       `json:"sort_order"`
	Angelegt     time.Time `json:"angelegt_am"`
}
type PhaseCreate struct {
	Nummer       string `json:"nummer"`
	Name         string `json:"name"`
	Beschreibung string `json:"beschreibung"`
	SortOrder    int    `json:"sort_order"`
}
type PhaseUpdate struct {
	Nummer       *string `json:"nummer"`
	Name         *string `json:"name"`
	Beschreibung *string `json:"beschreibung"`
	SortOrder    *int    `json:"sort_order"`
}

func (s *Service) CreatePhase(ctx context.Context, projectID string, in PhaseCreate) (*Phase, error) {
	if strings.TrimSpace(in.Nummer) == "" {
		in.Nummer = "1"
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("Name erforderlich")
	}
	id := uuid.NewString()
	var p Phase
	err := s.pg.QueryRow(ctx, `
        INSERT INTO project_phases (id, project_id, nummer, name, beschreibung, sort_order)
        VALUES ($1,$2,$3,$4,$5,$6)
        RETURNING id, project_id, nummer, name, COALESCE(beschreibung,''), sort_order, angelegt_am
    `, id, projectID, in.Nummer, in.Name, nullIfEmpty(in.Beschreibung), in.SortOrder).Scan(
		&p.ID, &p.ProjectID, &p.Nummer, &p.Name, &p.Beschreibung, &p.SortOrder, &p.Angelegt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
func (s *Service) ListPhases(ctx context.Context, projectID string) ([]Phase, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, project_id, nummer, name, COALESCE(beschreibung,''), sort_order, angelegt_am FROM project_phases WHERE project_id=$1 ORDER BY sort_order ASC, nummer ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Phase, 0)
	for rows.Next() {
		var p Phase
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.Nummer, &p.Name, &p.Beschreibung, &p.SortOrder, &p.Angelegt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
func (s *Service) GetPhase(ctx context.Context, id string) (*Phase, error) {
	var p Phase
	if err := s.pg.QueryRow(ctx, `SELECT id, project_id, nummer, name, COALESCE(beschreibung,''), sort_order, angelegt_am FROM project_phases WHERE id=$1`, id).Scan(
		&p.ID, &p.ProjectID, &p.Nummer, &p.Name, &p.Beschreibung, &p.SortOrder, &p.Angelegt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}
func (s *Service) UpdatePhase(ctx context.Context, id string, u PhaseUpdate) (*Phase, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(col string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", col, idx))
		args = append(args, v)
		idx++
	}
	if u.Nummer != nil {
		if strings.TrimSpace(*u.Nummer) == "" {
			return nil, errors.New("Nummer erforderlich")
		}
		add("nummer", *u.Nummer)
	}
	if u.Name != nil {
		if strings.TrimSpace(*u.Name) == "" {
			return nil, errors.New("Name erforderlich")
		}
		add("name", *u.Name)
	}
	if u.Beschreibung != nil {
		add("beschreibung", nullIfEmpty(*u.Beschreibung))
	}
	if u.SortOrder != nil {
		add("sort_order", *u.SortOrder)
	}
	if len(sets) == 0 {
		return s.GetPhase(ctx, id)
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE project_phases SET %s WHERE id=$%d", strings.Join(sets, ", "), idx)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.GetPhase(ctx, id)
}
func (s *Service) DeletePhase(ctx context.Context, id string) error {
	if _, err := s.pg.Exec(ctx, `DELETE FROM project_phases WHERE id=$1`, id); err != nil {
		return err
	}
	return nil
}

// ----- Elevations (Kalkulationspositionen)
type Elevation struct {
	ID           string    `json:"id"`
	PhaseID      string    `json:"phase_id"`
	Nummer       string    `json:"nummer"`
	Name         string    `json:"name"`
	Beschreibung string    `json:"beschreibung"`
	Menge        float64   `json:"menge"`
	WidthMM      *float64  `json:"width_mm"`
	HeightMM     *float64  `json:"height_mm"`
	ExternalGUID string    `json:"external_guid"`
	Serie        string    `json:"serie"`
	Oberflaeche  string    `json:"oberflaeche"`
	Picture1Rel  string    `json:"picture1_relpath"`
	Angelegt     time.Time `json:"angelegt_am"`
}
type ElevationCreate struct {
	Nummer       string   `json:"nummer"`
	Name         string   `json:"name"`
	Beschreibung string   `json:"beschreibung"`
	Menge        float64  `json:"menge"`
	WidthMM      *float64 `json:"width_mm"`
	HeightMM     *float64 `json:"height_mm"`
	ExternalGUID string   `json:"external_guid"`
}
type ElevationUpdate struct {
	Nummer       *string  `json:"nummer"`
	Name         *string  `json:"name"`
	Beschreibung *string  `json:"beschreibung"`
	Menge        *float64 `json:"menge"`
	WidthMM      *float64 `json:"width_mm"`
	HeightMM     *float64 `json:"height_mm"`
	ExternalGUID *string  `json:"external_guid"`
	Serie        *string  `json:"serie"`
	Oberflaeche  *string  `json:"oberflaeche"`
}

func (s *Service) CreateElevation(ctx context.Context, phaseID string, in ElevationCreate) (*Elevation, error) {
	if strings.TrimSpace(in.Nummer) == "" {
		in.Nummer = "1"
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("Name erforderlich")
	}
	if in.Menge == 0 {
		in.Menge = 1
	}
	id := uuid.NewString()
	var e Elevation
	err := s.pg.QueryRow(ctx, `
        INSERT INTO project_elevations (id, phase_id, nummer, name, beschreibung, menge, width_mm, height_mm, external_guid)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
        RETURNING id, phase_id, nummer, name, COALESCE(beschreibung,''), menge, width_mm, height_mm, COALESCE(external_guid,''), angelegt_am
    `, id, phaseID, in.Nummer, in.Name, nullIfEmpty(in.Beschreibung), in.Menge, in.WidthMM, in.HeightMM, nullIfEmpty(in.ExternalGUID)).Scan(
		&e.ID, &e.PhaseID, &e.Nummer, &e.Name, &e.Beschreibung, &e.Menge, &e.WidthMM, &e.HeightMM, &e.ExternalGUID, &e.Angelegt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
func (s *Service) ListElevations(ctx context.Context, phaseID string) ([]Elevation, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, phase_id, nummer, name, COALESCE(beschreibung,''), menge, width_mm, height_mm, COALESCE(external_guid,''), COALESCE(serie,''), COALESCE(oberflaeche,''), COALESCE(picture1_relpath,''), angelegt_am FROM project_elevations WHERE phase_id=$1 ORDER BY nummer ASC`, phaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Elevation, 0)
	for rows.Next() {
		var e Elevation
		if err := rows.Scan(&e.ID, &e.PhaseID, &e.Nummer, &e.Name, &e.Beschreibung, &e.Menge, &e.WidthMM, &e.HeightMM, &e.ExternalGUID, &e.Serie, &e.Oberflaeche, &e.Picture1Rel, &e.Angelegt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}
func (s *Service) GetElevation(ctx context.Context, id string) (*Elevation, error) {
	var e Elevation
	if err := s.pg.QueryRow(ctx, `SELECT id, phase_id, nummer, name, COALESCE(beschreibung,''), menge, width_mm, height_mm, COALESCE(external_guid,''), COALESCE(serie,''), COALESCE(oberflaeche,''), COALESCE(picture1_relpath,''), angelegt_am FROM project_elevations WHERE id=$1`, id).Scan(
		&e.ID, &e.PhaseID, &e.Nummer, &e.Name, &e.Beschreibung, &e.Menge, &e.WidthMM, &e.HeightMM, &e.ExternalGUID, &e.Serie, &e.Oberflaeche, &e.Picture1Rel, &e.Angelegt,
	); err != nil {
		return nil, err
	}
	return &e, nil
}
func (s *Service) UpdateElevation(ctx context.Context, id string, u ElevationUpdate) (*Elevation, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(col string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", col, idx))
		args = append(args, v)
		idx++
	}
	if u.Nummer != nil {
		if strings.TrimSpace(*u.Nummer) == "" {
			return nil, errors.New("Nummer erforderlich")
		}
		add("nummer", *u.Nummer)
	}
	if u.Name != nil {
		if strings.TrimSpace(*u.Name) == "" {
			return nil, errors.New("Name erforderlich")
		}
		add("name", *u.Name)
	}
	if u.Beschreibung != nil {
		add("beschreibung", nullIfEmpty(*u.Beschreibung))
	}
	if u.Menge != nil {
		add("menge", *u.Menge)
	}
	if u.WidthMM != nil {
		add("width_mm", *u.WidthMM)
	}
	if u.HeightMM != nil {
		add("height_mm", *u.HeightMM)
	}
	if u.ExternalGUID != nil {
		add("external_guid", nullIfEmpty(*u.ExternalGUID))
	}
	if u.Serie != nil {
		add("serie", nullIfEmpty(*u.Serie))
	}
	if u.Oberflaeche != nil {
		add("oberflaeche", nullIfEmpty(*u.Oberflaeche))
	}
	if len(sets) == 0 {
		return s.GetElevation(ctx, id)
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE project_elevations SET %s WHERE id=$%d", strings.Join(sets, ", "), idx)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.GetElevation(ctx, id)
}
func (s *Service) DeleteElevation(ctx context.Context, id string) error {
	if _, err := s.pg.Exec(ctx, `DELETE FROM project_elevations WHERE id=$1`, id); err != nil {
		return err
	}
	return nil
}

// ----- Single Elevations (Ausführungsvarianten)
type SingleElevation struct {
	ID           string    `json:"id"`
	ElevationID  string    `json:"elevation_id"`
	Name         string    `json:"name"`
	Beschreibung string    `json:"beschreibung"`
	Menge        float64   `json:"menge"`
	Selected     bool      `json:"selected"`
	ExternalGUID string    `json:"external_guid"`
	Angelegt     time.Time `json:"angelegt_am"`
}
type SingleElevationCreate struct {
	Name         string  `json:"name"`
	Beschreibung string  `json:"beschreibung"`
	Menge        float64 `json:"menge"`
	Selected     bool    `json:"selected"`
	ExternalGUID string  `json:"external_guid"`
}
type SingleElevationUpdate struct {
	Name         *string  `json:"name"`
	Beschreibung *string  `json:"beschreibung"`
	Menge        *float64 `json:"menge"`
	Selected     *bool    `json:"selected"`
	ExternalGUID *string  `json:"external_guid"`
}

func (s *Service) CreateSingleElevation(ctx context.Context, elevationID string, in SingleElevationCreate) (*SingleElevation, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("Name erforderlich")
	}
	if in.Menge == 0 {
		in.Menge = 1
	}
	id := uuid.NewString()
	var se SingleElevation
	err := s.pg.QueryRow(ctx, `
        INSERT INTO project_single_elevations (id, elevation_id, name, beschreibung, menge, selected, external_guid)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        RETURNING id, elevation_id, name, COALESCE(beschreibung,''), menge, selected, COALESCE(external_guid,''), angelegt_am
    `, id, elevationID, in.Name, nullIfEmpty(in.Beschreibung), in.Menge, in.Selected, nullIfEmpty(in.ExternalGUID)).Scan(
		&se.ID, &se.ElevationID, &se.Name, &se.Beschreibung, &se.Menge, &se.Selected, &se.ExternalGUID, &se.Angelegt,
	)
	if err != nil {
		return nil, err
	}
	if se.Selected {
		// Sicherstellen: nur eine Variante ausgewählt
		_, _ = s.pg.Exec(ctx, `UPDATE project_single_elevations SET selected=false WHERE elevation_id=$1 AND id<>$2 AND selected=true`, elevationID, se.ID)
	}
	return &se, nil
}
func (s *Service) ListSingleElevations(ctx context.Context, elevationID string) ([]SingleElevation, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, elevation_id, name, COALESCE(beschreibung,''), menge, selected, COALESCE(external_guid,''), angelegt_am FROM project_single_elevations WHERE elevation_id=$1 ORDER BY name ASC`, elevationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SingleElevation, 0)
	for rows.Next() {
		var se SingleElevation
		if err := rows.Scan(&se.ID, &se.ElevationID, &se.Name, &se.Beschreibung, &se.Menge, &se.Selected, &se.ExternalGUID, &se.Angelegt); err != nil {
			return nil, err
		}
		out = append(out, se)
	}
	return out, nil
}
func (s *Service) GetSingleElevation(ctx context.Context, id string) (*SingleElevation, error) {
	var se SingleElevation
	if err := s.pg.QueryRow(ctx, `SELECT id, elevation_id, name, COALESCE(beschreibung,''), menge, selected, COALESCE(external_guid,''), angelegt_am FROM project_single_elevations WHERE id=$1`, id).Scan(
		&se.ID, &se.ElevationID, &se.Name, &se.Beschreibung, &se.Menge, &se.Selected, &se.ExternalGUID, &se.Angelegt,
	); err != nil {
		return nil, err
	}
	return &se, nil
}
func (s *Service) UpdateSingleElevation(ctx context.Context, id string, u SingleElevationUpdate) (*SingleElevation, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(col string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", col, idx))
		args = append(args, v)
		idx++
	}
	if u.Name != nil {
		if strings.TrimSpace(*u.Name) == "" {
			return nil, errors.New("Name erforderlich")
		}
		add("name", *u.Name)
	}
	if u.Beschreibung != nil {
		add("beschreibung", nullIfEmpty(*u.Beschreibung))
	}
	if u.Menge != nil {
		add("menge", *u.Menge)
	}
	if u.Selected != nil {
		add("selected", *u.Selected)
	}
	if u.ExternalGUID != nil {
		add("external_guid", nullIfEmpty(*u.ExternalGUID))
	}
	if len(sets) == 0 {
		return s.GetSingleElevation(ctx, id)
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE project_single_elevations SET %s WHERE id=$%d", strings.Join(sets, ", "), idx)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	// Optional: Exklusivität von Selected erzwingen – hier weggelassen, da keine Selected-Policy
	return s.GetSingleElevation(ctx, id)
}
func (s *Service) DeleteSingleElevation(ctx context.Context, id string) error {
	if _, err := s.pg.Exec(ctx, `DELETE FROM project_single_elevations WHERE id=$1`, id); err != nil {
		return err
	}
	return nil
}

// ----- Material Lists per Single Elevation
type SingleProfile struct {
	ID              string   `json:"id"`
	SingleElevation string   `json:"single_elevation_id"`
	SupplierCode    string   `json:"supplier_code"`
	ArticleCode     string   `json:"article_code"`
	Description     string   `json:"description"`
	LengthMM        *float64 `json:"length_mm"`
	Qty             float64  `json:"qty"`
	Unit            string   `json:"unit"`
	MaterialID      string   `json:"material_id"`
	MaterialNummer  string   `json:"material_nummer"`
	MaterialBez     string   `json:"material_bezeichnung"`
}

type SingleArticle struct {
	ID              string  `json:"id"`
	SingleElevation string  `json:"single_elevation_id"`
	SupplierCode    string  `json:"supplier_code"`
	ArticleCode     string  `json:"article_code"`
	Description     string  `json:"description"`
	Qty             float64 `json:"qty"`
	Unit            string  `json:"unit"`
	MaterialID      string  `json:"material_id"`
	MaterialNummer  string  `json:"material_nummer"`
	MaterialBez     string  `json:"material_bezeichnung"`
}

type SingleGlass struct {
	ID              string   `json:"id"`
	SingleElevation string   `json:"single_elevation_id"`
	Configuration   string   `json:"configuration"`
	Description     string   `json:"description"`
	WidthMM         *float64 `json:"width_mm"`
	HeightMM        *float64 `json:"height_mm"`
	AreaM2          *float64 `json:"area_m2"`
	Qty             float64  `json:"qty"`
	Unit            string   `json:"unit"`
	MaterialID      string   `json:"material_id"`
	MaterialNummer  string   `json:"material_nummer"`
	MaterialBez     string   `json:"material_bezeichnung"`
}

func (s *Service) ListProfilesBySingle(ctx context.Context, singleID string) ([]SingleProfile, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT p.id, p.single_elevation_id,
               COALESCE(p.supplier_code,''), COALESCE(p.article_code,''), COALESCE(p.description,''),
               p.length_mm, COALESCE(p.qty,0), COALESCE(p.unit,''),
               COALESCE(p.material_id,''), COALESCE(m.nummer,''), COALESCE(m.bezeichnung,'')
          FROM single_elevation_profiles p
          LEFT JOIN materials m ON m.id = p.material_id
         WHERE p.single_elevation_id=$1
         ORDER BY p.id ASC`, singleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SingleProfile, 0)
	for rows.Next() {
		var it SingleProfile
		if err := rows.Scan(&it.ID, &it.SingleElevation, &it.SupplierCode, &it.ArticleCode, &it.Description, &it.LengthMM, &it.Qty, &it.Unit, &it.MaterialID, &it.MaterialNummer, &it.MaterialBez); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

func (s *Service) ListArticlesBySingle(ctx context.Context, singleID string) ([]SingleArticle, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT a.id, a.single_elevation_id,
               COALESCE(a.supplier_code,''), COALESCE(a.article_code,''), COALESCE(a.description,''),
               COALESCE(a.qty,0), COALESCE(a.unit,''),
               COALESCE(a.material_id,''), COALESCE(m.nummer,''), COALESCE(m.bezeichnung,'')
          FROM single_elevation_articles a
          LEFT JOIN materials m ON m.id = a.material_id
         WHERE a.single_elevation_id=$1
         ORDER BY a.id ASC`, singleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SingleArticle, 0)
	for rows.Next() {
		var it SingleArticle
		if err := rows.Scan(&it.ID, &it.SingleElevation, &it.SupplierCode, &it.ArticleCode, &it.Description, &it.Qty, &it.Unit, &it.MaterialID, &it.MaterialNummer, &it.MaterialBez); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

func (s *Service) ListGlassBySingle(ctx context.Context, singleID string) ([]SingleGlass, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT g.id, g.single_elevation_id,
               COALESCE(g.configuration,''), COALESCE(g.description,''),
               g.width_mm, g.height_mm, g.area_m2,
               COALESCE(g.qty,0), COALESCE(g.unit,''),
               COALESCE(g.material_id,''), COALESCE(m.nummer,''), COALESCE(m.bezeichnung,'')
          FROM single_elevation_glass g
          LEFT JOIN materials m ON m.id = g.material_id
         WHERE g.single_elevation_id=$1
         ORDER BY g.id ASC`, singleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SingleGlass, 0)
	for rows.Next() {
		var it SingleGlass
		if err := rows.Scan(&it.ID, &it.SingleElevation, &it.Configuration, &it.Description, &it.WidthMM, &it.HeightMM, &it.AreaM2, &it.Qty, &it.Unit, &it.MaterialID, &it.MaterialNummer, &it.MaterialBez); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

// Link zwischen importiertem Varianten-Item und Stammmaterial setzen
func (s *Service) LinkVariantMaterial(ctx context.Context, kind, itemID, materialID string) error {
	if strings.TrimSpace(itemID) == "" || strings.TrimSpace(kind) == "" {
		return errors.New("Ungültige Parameter")
	}
	isUnlink := strings.TrimSpace(materialID) == ""
	if !isUnlink {
		var dummy string
		if err := s.pg.QueryRow(ctx, `SELECT id FROM materials WHERE id=$1`, materialID).Scan(&dummy); err != nil {
			return errors.New("Material nicht gefunden")
		}
	}
	var q string
	switch strings.ToLower(kind) {
	case "profiles":
		q = `UPDATE single_elevation_profiles SET material_id=$1 WHERE id=$2`
	case "articles":
		q = `UPDATE single_elevation_articles SET material_id=$1 WHERE id=$2`
	case "glass":
		q = `UPDATE single_elevation_glass SET material_id=$1 WHERE id=$2`
	default:
		return errors.New("Ungültiger Typ")
	}
	var arg any
	if isUnlink {
		arg = nil
	} else {
		arg = materialID
	}
	if _, err := s.pg.Exec(ctx, q, arg, itemID); err != nil {
		return err
	}
	return nil
}

// ----- Project Assets Mapping (relativer Pfad -> GridFS-ID)
func (s *Service) UpsertProjectAsset(ctx context.Context, projectID, relPath, gridfsID, filename, contentType string, length int64) error {
	_, err := s.pg.Exec(ctx, `
        INSERT INTO project_assets (id, project_id, rel_path, gridfs_id, filename, content_type, length)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        ON CONFLICT (project_id, rel_path) DO UPDATE SET gridfs_id=EXCLUDED.gridfs_id, filename=EXCLUDED.filename, content_type=EXCLUDED.content_type, length=EXCLUDED.length, uploaded_at=now()
    `, uuid.NewString(), projectID, relPath, gridfsID, filename, contentType, length)
	return err
}
func (s *Service) GetProjectAsset(ctx context.Context, projectID, relPath string) (gridfsID, filename, contentType string, length int64, err error) {
	err = s.pg.QueryRow(ctx, `SELECT gridfs_id, filename, content_type, COALESCE(length,0) FROM project_assets WHERE project_id=$1 AND rel_path=$2`, projectID, relPath).Scan(&gridfsID, &filename, &contentType, &length)
	return
}

// ----- Project assets listing -----
type ProjectAsset struct {
	RelPath     string    `json:"rel_path"`
	ContentType string    `json:"content_type"`
	Length      int64     `json:"length"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

func (s *Service) ListProjectAssets(ctx context.Context, projectID string) ([]ProjectAsset, error) {
	rows, err := s.pg.Query(ctx, `SELECT rel_path, content_type, COALESCE(length,0), uploaded_at FROM project_assets WHERE project_id=$1 ORDER BY rel_path ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProjectAsset, 0)
	for rows.Next() {
		var a ProjectAsset
		if err := rows.Scan(&a.RelPath, &a.ContentType, &a.Length, &a.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}
