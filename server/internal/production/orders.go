package production

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/materials"
	"nalaerp3/internal/settings"
)

// Fertigungsauftraege (ADR 0034, Backlog H.1.2): Entwurf anlegen und
// aendern, freigeben (mit Kopie der Position), beginnen, fertigmelden,
// stornieren, lesen. Nie geloescht. Die Regeln stehen in rules.go.

// NumberEntity und NumberPattern: Nummernkreis der Fertigungsauftraege.
const (
	NumberEntity  = "production_order"
	NumberPattern = "FA-{YYYY}-{NNNN}"
)

type Order struct {
	ID                 uuid.UUID  `json:"id"`
	Nummer             string     `json:"nummer"`
	ProjectID          uuid.UUID  `json:"project_id"`
	ProjectNummer      string     `json:"project_nummer"`
	ProjectName        string     `json:"project_name"`
	ElevationID        *uuid.UUID `json:"elevation_id"`
	SingleElevationID  *uuid.UUID `json:"single_elevation_id"`
	Stueckzahl         int        `json:"stueckzahl"`
	Liefertermin       *time.Time `json:"liefertermin"`
	Ausfuehrungsklasse *string    `json:"ausfuehrungsklasse"`
	Notiz              string     `json:"notiz"`
	Status             string     `json:"status"`
	// Kopie bei der Freigabe; im Entwurf die aktuellen Werte der Position.
	PhaseName      string   `json:"phase_name"`
	PositionNummer string   `json:"position_nummer"`
	PositionName   string   `json:"position_name"`
	VarianteName   string   `json:"variante_name"`
	WidthMM        *float64 `json:"width_mm"`
	HeightMM       *float64 `json:"height_mm"`

	FreigegebenAt       *time.Time `json:"freigegeben_at,omitempty"`
	FreigegebenByUserID *string    `json:"freigegeben_by_user_id,omitempty"`
	BegonnenAt          *time.Time `json:"begonnen_at,omitempty"`
	BegonnenByUserID    *string    `json:"begonnen_by_user_id,omitempty"`
	FertigAt            *time.Time `json:"fertig_at,omitempty"`
	FertigByUserID      *string    `json:"fertig_by_user_id,omitempty"`
	StornoReason        string     `json:"storno_reason,omitempty"`
	StorniertAt         *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID   *string    `json:"storniert_by_user_id,omitempty"`
	CreatedByUserID     *string    `json:"created_by_user_id"`
	CreatedAt           time.Time  `json:"created_at"`
	// Lager, aus dem bei der Freigabe reserviert wurde (leer: ohne
	// Reservierung freigegeben).
	ReservierungLagerID *string `json:"reservierung_lager_id"`

	// Hinweise (HintKeinKundenauftrag ...), keine Sperren.
	Hinweise []string `json:"hinweise"`
	// Fertigungsstueckliste - nur in der Einzelansicht und erst ab der
	// Freigabe.
	Items []Item `json:"items,omitempty"`

	inhaltHash string
}

type Item struct {
	Pos           int      `json:"pos"`
	Art           string   `json:"art"`
	SupplierCode  string   `json:"supplier_code"`
	ArticleCode   string   `json:"article_code"`
	Description   string   `json:"description"`
	Configuration string   `json:"configuration"`
	LengthMM      *float64 `json:"length_mm"`
	WidthMM       *float64 `json:"width_mm"`
	HeightMM      *float64 `json:"height_mm"`
	AreaM2        *float64 `json:"area_m2"`
	MengeJeStueck float64  `json:"menge_je_stueck"`
	// Bedarf des Auftrags: menge_je_stueck * stueckzahl.
	MengeGesamt float64 `json:"menge_gesamt"`
	Unit        string  `json:"unit"`
	// Lagermaterial der Projektzeile, bei der Freigabe kopiert.
	MaterialID      *string `json:"material_id"`
	MaterialEinheit string  `json:"material_einheit"`
	// Nur mit Reservierungslager und passender Einheit (ReservationQty),
	// nicht bei stornierten Auftraegen: Bedarf in der Einheit des
	// Materials, davon aktiv reserviert, Rest als Fehlmenge.
	Bedarf     *float64 `json:"bedarf"`
	Reserviert *float64 `json:"reserviert"`
	Fehlmenge  *float64 `json:"fehlmenge"`
}

// OrderFilter: ohne Angaben alle nicht stornierten Auftraege des Mandanten.
type OrderFilter struct {
	ProjectID        *uuid.UUID
	ElevationID      *uuid.UUID
	Status           string
	IncludeStorniert bool
}

// DraftUpdate: im Entwurf aenderbare Angaben. Position und Variante
// bleiben - fuer eine andere Position einen neuen Auftrag anlegen.
type DraftUpdate struct {
	Stueckzahl         int        `json:"stueckzahl"`
	Liefertermin       *time.Time `json:"liefertermin"`
	Ausfuehrungsklasse *string    `json:"ausfuehrungsklasse"`
	Notiz              string     `json:"notiz"`
}

type Service struct {
	pg  *pgxpool.Pool
	num *settings.NumberingService
}

func NewService(pg *pgxpool.Pool) *Service {
	return &Service{pg: pg, num: settings.NewNumberingService(pg)}
}

var ErrNotFound = errors.New("Fertigungsauftrag nicht gefunden")

func nullableUserID(userID string) *string {
	if strings.TrimSpace(userID) == "" {
		return nil
	}
	return &userID
}

const orderColumns = `o.id, o.nummer, o.project_id, p.nummer, p.name, o.elevation_id, o.single_elevation_id, o.stueckzahl,
	o.liefertermin, o.ausfuehrungsklasse, o.notiz, o.status, o.phase_name, o.position_nummer, o.position_name,
	o.variante_name, o.width_mm::float8, o.height_mm::float8, o.inhalt_hash,
	o.freigegeben_at, o.freigegeben_by_user_id, o.begonnen_at, o.begonnen_by_user_id, o.fertig_at, o.fertig_by_user_id,
	o.storno_reason, o.storniert_at, o.storniert_by_user_id, o.created_by_user_id, o.created_at, o.reservierung_lager_id`

const orderFrom = `
	  FROM production_orders o
	  JOIN projects p ON p.id = o.project_id`

func scanOrder(row pgx.Row) (*Order, error) {
	var x Order
	if err := row.Scan(&x.ID, &x.Nummer, &x.ProjectID, &x.ProjectNummer, &x.ProjectName, &x.ElevationID, &x.SingleElevationID,
		&x.Stueckzahl, &x.Liefertermin, &x.Ausfuehrungsklasse, &x.Notiz, &x.Status, &x.PhaseName, &x.PositionNummer,
		&x.PositionName, &x.VarianteName, &x.WidthMM, &x.HeightMM, &x.inhaltHash,
		&x.FreigegebenAt, &x.FreigegebenByUserID, &x.BegonnenAt, &x.BegonnenByUserID, &x.FertigAt, &x.FertigByUserID,
		&x.StornoReason, &x.StorniertAt, &x.StorniertByUserID, &x.CreatedByUserID, &x.CreatedAt, &x.ReservierungLagerID); err != nil {
		return nil, err
	}
	return &x, nil
}

// querier: Pool oder Transaktion.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// checkPosition: Projekt des Mandanten, Position im Projekt, Variante an
// der Position.
func checkPosition(ctx context.Context, q querier, in OrderInput, companyID string) error {
	var ok bool
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM projects WHERE id=$1 AND company_id=$2)`, in.ProjectID, companyID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errors.New("Projekt nicht gefunden")
	}
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project_elevations e JOIN project_phases ph ON ph.id = e.phase_id
		WHERE e.id=$1 AND ph.project_id=$2)`, in.ElevationID, in.ProjectID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errors.New("Position nicht im Projekt")
	}
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project_single_elevations WHERE id=$1 AND elevation_id=$2)`,
		in.SingleElevationID, in.ElevationID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return errors.New("Variante nicht an der Position")
	}
	return nil
}

// nextNumber legt den Nummernkreis des Mandanten an, falls er fehlt -
// sonst vergaebe settings.NumberingService.Next eine "PO-"-Nummer (siehe
// Migration 079).
func (s *Service) nextNumber(ctx context.Context, companyID string) (string, error) {
	if _, err := s.pg.Exec(ctx, `INSERT INTO number_sequences (company_id, entity, pattern, next_value)
		VALUES ($1, $2, $3, 1) ON CONFLICT DO NOTHING`, companyID, NumberEntity, NumberPattern); err != nil {
		return "", err
	}
	return s.num.Next(ctx, NumberEntity, companyID)
}

// Create legt einen Entwurf an. Die Positionsmenge wird hier nicht
// geprueft - ein Ueberschreiten zeigt der Hinweis.
func (s *Service) Create(ctx context.Context, in OrderInput, actorUserID, companyID string) (*Order, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	in = NormalizeOrderInput(in)
	if err := ValidateOrderInput(in); err != nil {
		return nil, err
	}
	if err := checkPosition(ctx, s.pg, in, companyID); err != nil {
		return nil, err
	}
	nummer, err := s.nextNumber(ctx, companyID)
	if err != nil {
		return nil, err
	}
	id := uuid.New()
	if _, err := s.pg.Exec(ctx, `
		INSERT INTO production_orders (id, company_id, nummer, project_id, elevation_id, single_elevation_id, stueckzahl,
			liefertermin, ausfuehrungsklasse, notiz, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		id, companyID, nummer, in.ProjectID, in.ElevationID, in.SingleElevationID, in.Stueckzahl,
		in.Liefertermin, in.Ausfuehrungsklasse, in.Notiz, nullableUserID(actorUserID)); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

// UpdateDraft aendert Stueckzahl, Liefertermin, Ausfuehrungsklasse und
// Notiz - nur im Entwurf. Nach der Freigabe gilt die Kopie; eine
// Aenderung braucht Storno und neuen Auftrag.
func (s *Service) UpdateDraft(ctx context.Context, id uuid.UUID, in DraftUpdate, companyID string) (*Order, error) {
	n := NormalizeOrderInput(OrderInput{Stueckzahl: in.Stueckzahl, Liefertermin: in.Liefertermin,
		Ausfuehrungsklasse: in.Ausfuehrungsklasse, Notiz: in.Notiz})
	if err := ValidateOrderDetails(n.Stueckzahl, n.Ausfuehrungsklasse); err != nil {
		return nil, err
	}
	ct, err := s.pg.Exec(ctx, `UPDATE production_orders SET stueckzahl=$3, liefertermin=$4, ausfuehrungsklasse=$5, notiz=$6
		 WHERE id=$1 AND company_id=$2 AND status=$7`,
		id, companyID, n.Stueckzahl, n.Liefertermin, n.Ausfuehrungsklasse, n.Notiz, StatusEntwurf)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		if _, err := s.Get(ctx, id, companyID); err != nil {
			return nil, err
		}
		return nil, errors.New("Nur ein Entwurf kann geändert werden")
	}
	return s.Get(ctx, id, companyID)
}

// loadSnapshot liest Position, Variante und Materialliste, wie sie jetzt
// im Projekt stehen. ok=false: Position oder Variante gibt es nicht mehr
// (oder die Variante haengt nicht mehr an der Position).
func loadSnapshot(ctx context.Context, q querier, elevationID, variantID uuid.UUID) (Snapshot, bool, error) {
	var s Snapshot
	err := q.QueryRow(ctx, `
		SELECT ph.name, e.nummer, e.name, COALESCE(e.external_guid,''), v.name, COALESCE(v.external_guid,''),
		       e.width_mm::float8, e.height_mm::float8
		  FROM project_single_elevations v
		  JOIN project_elevations e ON e.id = v.elevation_id
		  JOIN project_phases ph ON ph.id = e.phase_id
		 WHERE v.id=$1 AND e.id=$2`, variantID, elevationID).
		Scan(&s.PhaseName, &s.PositionNummer, &s.PositionName, &s.PositionGUID, &s.VarianteName, &s.VarianteGUID, &s.WidthMM, &s.HeightMM)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, false, nil
	}
	if err != nil {
		return s, false, err
	}
	rows, err := q.Query(ctx, `
		SELECT 'profil', COALESCE(supplier_code,''), COALESCE(article_code,''), COALESCE(description,''), '',
		       length_mm::float8, NULL::float8, NULL::float8, NULL::float8, qty::float8, COALESCE(unit,''), COALESCE(material_id,'')
		  FROM single_elevation_profiles WHERE single_elevation_id=$1
		UNION ALL
		SELECT 'artikel', COALESCE(supplier_code,''), COALESCE(article_code,''), COALESCE(description,''), '',
		       NULL, NULL, NULL, NULL, qty::float8, COALESCE(unit,''), COALESCE(material_id,'')
		  FROM single_elevation_articles WHERE single_elevation_id=$1
		UNION ALL
		SELECT 'glas', '', '', COALESCE(description,''), COALESCE(configuration,''),
		       NULL, width_mm::float8, height_mm::float8, area_m2::float8, qty::float8, COALESCE(unit,''), COALESCE(material_id,'')
		  FROM single_elevation_glass WHERE single_elevation_id=$1`, variantID)
	if err != nil {
		return s, false, err
	}
	defer rows.Close()
	s.Items = make([]SnapshotItem, 0)
	for rows.Next() {
		var it SnapshotItem
		if err := rows.Scan(&it.Art, &it.SupplierCode, &it.ArticleCode, &it.Description, &it.Configuration,
			&it.LengthMM, &it.WidthMM, &it.HeightMM, &it.AreaM2, &it.MengeJeStueck, &it.Unit, &it.MaterialID); err != nil {
			return s, false, err
		}
		s.Items = append(s.Items, it)
	}
	return s, true, rows.Err()
}

// Release gibt einen Entwurf frei: Position, Variante und Materialliste
// werden kopiert (in der Reihenfolge des Fingerabdrucks) und danach nie
// mehr geaendert. Mit Lager (optional, H.1.5) wird je Zeile mit
// Lagermaterial und passender Einheit das Verfuegbare reserviert; eine
// Fehlmenge verhindert die Freigabe nicht.
func (s *Service) Release(ctx context.Context, id uuid.UUID, lagerID, actorUserID, companyID string) (*Order, error) {
	lagerID = strings.TrimSpace(lagerID)
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cur, err := lockOrder(ctx, tx, id, companyID)
	if err != nil {
		return nil, err
	}
	if _, err := NextStatus(cur.status, AktionFreigeben); err != nil {
		return nil, err
	}
	if lagerID != "" {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM warehouses WHERE id=$1 AND company_id=$2)`, lagerID, companyID).Scan(&ok); err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("Lager nicht gefunden")
		}
	}
	if cur.elevationID == nil || cur.variantID == nil {
		return nil, errors.New("Position oder Variante ist nicht mehr im Projekt")
	}
	snap, ok, err := loadSnapshot(ctx, tx, *cur.elevationID, *cur.variantID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Position oder Variante ist nicht mehr im Projekt")
	}
	if err := ValidateSnapshot(snap); err != nil {
		return nil, err
	}
	c := canonical(snap)
	einheiten, err := materialUnits(ctx, tx, c.Items, companyID)
	if err != nil {
		return nil, err
	}
	var lager *string
	if lagerID != "" {
		lager = &lagerID
	}
	if _, err := tx.Exec(ctx, `
		UPDATE production_orders SET status=$2, phase_name=$3, position_nummer=$4, position_name=$5, position_guid=$6,
			variante_name=$7, variante_guid=$8, width_mm=$9, height_mm=$10, inhalt_hash=$11,
			freigegeben_at=now(), freigegeben_by_user_id=$12, reservierung_lager_id=$13
		 WHERE id=$1`,
		id, StatusFreigegeben, c.PhaseName, c.PositionNummer, c.PositionName, c.PositionGUID, c.VarianteName, c.VarianteGUID,
		c.WidthMM, c.HeightMM, Fingerprint(snap), nullableUserID(actorUserID), lager); err != nil {
		return nil, err
	}
	for i, it := range c.Items {
		// Nur Material des Mandanten wird kopiert und reserviert.
		var material, reservierung *string
		if einheit, ok := einheiten[it.MaterialID]; ok {
			m := it.MaterialID
			material = &m
			if want, ok := ReservationQty(it.Art, it.Unit, einheit, it.LengthMM, it.MengeJeStueck*float64(cur.stueckzahl)); ok && lager != nil {
				rid, _, err := materials.ReserveAvailableTx(ctx, tx, companyID, m, lagerID, cur.projectID.String(), want, ReservierungGrund, cur.nummer)
				if err != nil {
					return nil, err
				}
				if rid != "" {
					reservierung = &rid
				}
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO production_order_items (id, order_id, pos, art, supplier_code, article_code, description, configuration,
				length_mm, width_mm, height_mm, area_m2, menge_je_stueck, unit, material_id, stock_reservation_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
			uuid.New(), id, i+1, it.Art, it.SupplierCode, it.ArticleCode, it.Description, it.Configuration,
			it.LengthMM, it.WidthMM, it.HeightMM, it.AreaM2, it.MengeJeStueck, it.Unit, material, reservierung); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

// materialUnits: Einheit je Lagermaterial der Zeilen, nur Material des
// Mandanten.
func materialUnits(ctx context.Context, q querier, items []SnapshotItem, companyID string) (map[string]string, error) {
	ids := make([]string, 0)
	for _, it := range items {
		if it.MaterialID != "" {
			ids = append(ids, it.MaterialID)
		}
	}
	out := make(map[string]string)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT id, einheit FROM materials WHERE company_id=$1 AND id = ANY($2)`, companyID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, einheit string
		if err := rows.Scan(&id, &einheit); err != nil {
			return nil, err
		}
		out[id] = einheit
	}
	return out, rows.Err()
}

type lockedOrder struct {
	status                 string
	nummer                 string
	projectID              uuid.UUID
	stueckzahl             int
	elevationID, variantID *uuid.UUID
}

func lockOrder(ctx context.Context, tx pgx.Tx, id uuid.UUID, companyID string) (*lockedOrder, error) {
	var x lockedOrder
	err := tx.QueryRow(ctx, `SELECT status, nummer, project_id, stueckzahl, elevation_id, single_elevation_id FROM production_orders
		WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).Scan(&x.status, &x.nummer, &x.projectID, &x.stueckzahl, &x.elevationID, &x.variantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &x, err
}

// Start: freigegeben -> in Arbeit.
func (s *Service) Start(ctx context.Context, id uuid.UUID, actorUserID, companyID string) (*Order, error) {
	return s.step(ctx, id, AktionBeginnen, `begonnen_at=now(), begonnen_by_user_id=$3`, actorUserID, companyID)
}

// Finish: in Arbeit -> fertig.
func (s *Service) Finish(ctx context.Context, id uuid.UUID, actorUserID, companyID string) (*Order, error) {
	return s.step(ctx, id, AktionFertigmelden, `fertig_at=now(), fertig_by_user_id=$3`, actorUserID, companyID)
}

func (s *Service) step(ctx context.Context, id uuid.UUID, aktion, set, actorUserID, companyID string) (*Order, error) {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cur, err := lockOrder(ctx, tx, id, companyID)
	if err != nil {
		return nil, err
	}
	next, err := NextStatus(cur.status, aktion)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE production_orders SET status=$2, `+set+` WHERE id=$1`,
		id, next, nullableUserID(actorUserID)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

// Cancel storniert mit Begruendung - aus jedem Status vor "fertig". Der
// Auftrag und seine Stueckliste bleiben sichtbar; aktive Reservierungen
// werden freigegeben.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*Order, error) {
	r, err := ValidateStornoReason(reason)
	if err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cur, err := lockOrder(ctx, tx, id, companyID)
	if err != nil {
		return nil, err
	}
	next, err := NextStatus(cur.status, AktionStornieren)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE production_orders SET status=$2, storno_reason=$3, storniert_at=now(), storniert_by_user_id=$4
		 WHERE id=$1`, id, next, r, nullableUserID(actorUserID)); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT stock_reservation_id FROM production_order_items
		WHERE order_id=$1 AND stock_reservation_id IS NOT NULL`, id)
	if err != nil {
		return nil, err
	}
	reservierungen, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	for _, rid := range reservierungen {
		if _, err := materials.ReleaseReservationTx(ctx, tx, companyID, rid); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, id, companyID)
}

// Get liefert einen Auftrag mit Hinweisen und - ab der Freigabe - der
// Fertigungsstueckliste.
func (s *Service) Get(ctx context.Context, id uuid.UUID, companyID string) (*Order, error) {
	x, err := scanOrder(s.pg.QueryRow(ctx, `SELECT `+orderColumns+orderFrom+` WHERE o.id=$1 AND o.company_id=$2`, id, companyID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := s.decorate(ctx, x, companyID); err != nil {
		return nil, err
	}
	if x.FreigegebenAt != nil {
		if x.Items, err = s.items(ctx, x); err != nil {
			return nil, err
		}
	}
	if x.Status == StatusFreigegeben || x.Status == StatusInArbeit {
		for _, it := range x.Items {
			if it.Fehlmenge != nil && *it.Fehlmenge > 0 {
				x.Hinweise = append(x.Hinweise, HintFehlmenge)
				break
			}
		}
	}
	return x, nil
}

// List: Auftraege des Mandanten nach Liefertermin (ohne Termin zuletzt),
// dann Nummer; stornierte nur auf Wunsch oder mit Status-Filter.
func (s *Service) List(ctx context.Context, companyID string, f OrderFilter) ([]Order, error) {
	rows, err := s.pg.Query(ctx, `SELECT `+orderColumns+orderFrom+`
		 WHERE o.company_id=$1
		   AND ($2::uuid IS NULL OR o.project_id=$2)
		   AND ($3::uuid IS NULL OR o.elevation_id=$3)
		   AND ($4 = '' OR o.status=$4)
		   AND (o.status <> 'storniert' OR $5 OR $4 <> '')
		 ORDER BY o.liefertermin NULLS LAST, o.nummer`,
		companyID, f.ProjectID, f.ElevationID, f.Status, f.IncludeStorniert)
	if err != nil {
		return nil, err
	}
	out := make([]Order, 0)
	for rows.Next() {
		x, err := scanOrder(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := s.decorate(ctx, &out[i], companyID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// items liest die Fertigungsstueckliste mit Material und - bei einem
// Reservierungslager - Bedarf, aktiv Reserviertem und Fehlmenge.
func (s *Service) items(ctx context.Context, x *Order) ([]Item, error) {
	rows, err := s.pg.Query(ctx, `SELECT i.pos, i.art, i.supplier_code, i.article_code, i.description, i.configuration,
		i.length_mm::float8, i.width_mm::float8, i.height_mm::float8, i.area_m2::float8, i.menge_je_stueck::float8, i.unit,
		i.material_id, COALESCE(m.einheit,''), COALESCE(r.qty::float8, 0)
		  FROM production_order_items i
		  LEFT JOIN materials m ON m.id = i.material_id
		  LEFT JOIN stock_reservations r ON r.id = i.stock_reservation_id AND r.status = 'aktiv'
		 WHERE i.order_id=$1 ORDER BY i.pos`, x.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Item, 0)
	for rows.Next() {
		var it Item
		var reserviert float64
		if err := rows.Scan(&it.Pos, &it.Art, &it.SupplierCode, &it.ArticleCode, &it.Description, &it.Configuration,
			&it.LengthMM, &it.WidthMM, &it.HeightMM, &it.AreaM2, &it.MengeJeStueck, &it.Unit,
			&it.MaterialID, &it.MaterialEinheit, &reserviert); err != nil {
			return nil, err
		}
		it.MengeGesamt = it.MengeJeStueck * float64(x.Stueckzahl)
		if x.ReservierungLagerID != nil && it.MaterialID != nil && x.Status != StatusStorniert {
			if bedarf, ok := ReservationQty(it.Art, it.Unit, it.MaterialEinheit, it.LengthMM, it.MengeGesamt); ok {
				fehl := Shortfall(bedarf, reserviert)
				it.Bedarf, it.Reserviert, it.Fehlmenge = &bedarf, &reserviert, &fehl
			}
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// decorate setzt die Hinweise und zeigt im Entwurf die aktuellen Werte
// der Position. Stornierte Auftraege bekommen keine Hinweise.
func (s *Service) decorate(ctx context.Context, x *Order, companyID string) error {
	x.Hinweise = make([]string, 0)
	if x.Status == StatusStorniert {
		return nil
	}
	var hasSalesOrder bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sales_orders WHERE project_id=$1 AND company_id=$2 AND status <> 'canceled')`,
		x.ProjectID, companyID).Scan(&hasSalesOrder); err != nil {
		return err
	}
	if !hasSalesOrder {
		x.Hinweise = append(x.Hinweise, HintKeinKundenauftrag)
	}
	if x.ElevationID == nil || x.SingleElevationID == nil {
		x.Hinweise = append(x.Hinweise, HintPositionEntfernt)
		return nil
	}
	snap, ok, err := loadSnapshot(ctx, s.pg, *x.ElevationID, *x.SingleElevationID)
	if err != nil {
		return err
	}
	if !ok {
		x.Hinweise = append(x.Hinweise, HintPositionEntfernt)
		return nil
	}
	if x.Status != StatusFertig {
		var menge float64
		var andere int
		if err := s.pg.QueryRow(ctx, `SELECT e.menge::float8,
			COALESCE((SELECT sum(stueckzahl) FROM production_orders
			           WHERE elevation_id=e.id AND id<>$2 AND status <> 'storniert'), 0)
			  FROM project_elevations e WHERE e.id=$1`, *x.ElevationID, x.ID).Scan(&menge, &andere); err != nil {
			return err
		}
		if QuantityExceeded(menge, andere, x.Stueckzahl) {
			x.Hinweise = append(x.Hinweise, HintMengeUeberschritten)
		}
	}
	if x.FreigegebenAt == nil {
		c := canonical(snap)
		x.PhaseName, x.PositionNummer, x.PositionName, x.VarianteName = c.PhaseName, c.PositionNummer, c.PositionName, c.VarianteName
		x.WidthMM, x.HeightMM = c.WidthMM, c.HeightMM
	} else if Fingerprint(snap) != x.inhaltHash {
		x.Hinweise = append(x.Hinweise, HintPositionGeaendert)
	}
	return nil
}
