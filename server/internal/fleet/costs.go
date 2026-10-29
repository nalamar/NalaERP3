package fleet

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/hr"
)

// Kosten je Fahrzeug (ADR 0032 §7, Backlog G.3d.1): erfassen, stornieren,
// lesen, auswerten. Geschlossene Form - ein Beleg wird nie geaendert und
// nie geloescht; erlaubt ist allein das einmalige Storno. Die Regeln
// stehen in cost_rules.go.

type Cost struct {
	ID                uuid.UUID  `json:"id"`
	VehicleID         uuid.UUID  `json:"vehicle_id"`
	Kennzeichen       string     `json:"kennzeichen"`
	Fahrzeug          string     `json:"fahrzeug"`
	Datum             time.Time  `json:"datum"`
	Kostenart         string     `json:"kostenart"`
	Bezeichnung       string     `json:"bezeichnung"`
	BetragNetto       float64    `json:"betrag_netto"`
	Lieferant         string     `json:"lieferant"`
	Belegnummer       string     `json:"belegnummer"`
	KmStand           *int       `json:"km_stand"`
	Liter             *float64   `json:"liter"`
	KostenstelleID    *string    `json:"kostenstelle_id"`
	Notiz             string     `json:"notiz"`
	Status            string     `json:"status"`
	StornoReason      string     `json:"storno_reason,omitempty"`
	StorniertAt       *time.Time `json:"storniert_at,omitempty"`
	StorniertByUserID *string    `json:"storniert_by_user_id,omitempty"`
	CreatedByUserID   *string    `json:"created_by_user_id"`
	CreatedAt         time.Time  `json:"created_at"`
}

// CostFilter schraenkt Liste und Auswertung ein. Ohne Angaben: alle nicht
// stornierten Kosten des Mandanten.
type CostFilter struct {
	VehicleID        *uuid.UUID
	Kostenart        string
	Von              *time.Time
	Bis              *time.Time
	IncludeStorniert bool // nur fuer die Liste; die Auswertung zaehlt nie stornierte
}

// CostSum ist die Summe einer Kostenart.
type CostSum struct {
	Kostenart   string  `json:"kostenart"`
	Anzahl      int     `json:"anzahl"`
	BetragNetto float64 `json:"betrag_netto"`
}

// CostSummary: Summen je Kostenart (in der Reihenfolge von Kostenarten,
// nur vorkommende) und gesamt.
type CostSummary struct {
	Summen []CostSum `json:"summen"`
	Gesamt float64   `json:"gesamt"`
}

type CostService struct {
	pg *pgxpool.Pool
}

func NewCostService(pg *pgxpool.Pool) *CostService {
	return &CostService{pg: pg}
}

const costColumns = `c.id, c.vehicle_id, v.kennzeichen, v.bezeichnung, c.datum, c.kostenart, c.bezeichnung,
	c.betrag_netto::float8, c.lieferant, c.belegnummer, c.km_stand, c.liter::float8, c.kostenstelle_id, c.notiz,
	c.status, c.storno_reason, c.storniert_at, c.storniert_by_user_id, c.created_by_user_id, c.created_at`

const costFrom = `
	  FROM fleet_costs c
	  JOIN fleet_vehicles v ON v.id = c.vehicle_id`

// costWhere: $1 Mandant, $2 Fahrzeug, $3 Kostenart, $4 von, $5 bis.
const costWhere = `
	 WHERE v.company_id=$1
	   AND ($2::uuid IS NULL OR c.vehicle_id=$2)
	   AND ($3 = '' OR c.kostenart=$3)
	   AND ($4::date IS NULL OR c.datum >= $4)
	   AND ($5::date IS NULL OR c.datum <= $5)`

func scanCost(row pgx.Row) (*Cost, error) {
	var x Cost
	if err := row.Scan(&x.ID, &x.VehicleID, &x.Kennzeichen, &x.Fahrzeug, &x.Datum, &x.Kostenart, &x.Bezeichnung,
		&x.BetragNetto, &x.Lieferant, &x.Belegnummer, &x.KmStand, &x.Liter, &x.KostenstelleID, &x.Notiz,
		&x.Status, &x.StornoReason, &x.StorniertAt, &x.StorniertByUserID, &x.CreatedByUserID, &x.CreatedAt); err != nil {
		return nil, err
	}
	return &x, nil
}

// RecordCost erfasst einen Beleg - auch am abgemeldeten Fahrzeug
// (Schlussrechnung). Ohne Kostenstelle gilt die des Fahrzeugs zum
// Zeitpunkt des Erfassens.
func (s *CostService) RecordCost(ctx context.Context, in CostInput, actorUserID, companyID string) (*Cost, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.VehicleID == uuid.Nil {
		return nil, errors.New("vehicle_id erforderlich")
	}
	in = NormalizeCostInput(in)
	if err := ValidateCostInput(in, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var vehicleKostenstelle *string
	if err := tx.QueryRow(ctx, `SELECT kostenstelle_id FROM fleet_vehicles WHERE id=$1 AND company_id=$2`, in.VehicleID, companyID).
		Scan(&vehicleKostenstelle); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Fahrzeug nicht gefunden")
		}
		return nil, err
	}
	kostenstelle := in.KostenstelleID
	if kostenstelle == nil {
		kostenstelle = vehicleKostenstelle
	} else if err := checkKostenstelle(ctx, tx, kostenstelle, companyID); err != nil {
		return nil, err
	}

	id := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO fleet_costs (id, vehicle_id, datum, kostenart, bezeichnung, betrag_netto, lieferant, belegnummer,
			km_stand, liter, kostenstelle_id, notiz, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		id, in.VehicleID, in.Datum, in.Kostenart, in.Bezeichnung, in.BetragNetto, in.Lieferant, in.Belegnummer,
		in.KmStand, in.Liter, kostenstelle, in.Notiz, nullableUserID(actorUserID)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetCost(ctx, id, companyID)
}

func (s *CostService) GetCost(ctx context.Context, id uuid.UUID, companyID string) (*Cost, error) {
	x, err := scanCost(s.pg.QueryRow(ctx, `SELECT `+costColumns+costFrom+`
		 WHERE c.id=$1 AND v.company_id=$2`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Kosten nicht gefunden")
		}
		return nil, err
	}
	return x, nil
}

// ListCosts liefert Kosten des Mandanten, die juengsten zuerst. Von/Bis
// grenzen das Datum ein (jeweils einschliesslich); stornierte nur auf
// Wunsch.
func (s *CostService) ListCosts(ctx context.Context, companyID string, f CostFilter) ([]Cost, error) {
	rows, err := s.pg.Query(ctx, `SELECT `+costColumns+costFrom+costWhere+`
		   AND (c.status='active' OR $6)
		 ORDER BY c.datum DESC, c.created_at DESC, c.id`,
		companyID, f.VehicleID, f.Kostenart, f.Von, f.Bis, f.IncludeStorniert)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Cost, 0)
	for rows.Next() {
		x, err := scanCost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *x)
	}
	return out, rows.Err()
}

// SummarizeCosts summiert die nicht stornierten Kosten je Kostenart.
func (s *CostService) SummarizeCosts(ctx context.Context, companyID string, f CostFilter) (*CostSummary, error) {
	rows, err := s.pg.Query(ctx, `SELECT c.kostenart, count(*), sum(c.betrag_netto)::float8`+costFrom+costWhere+`
		   AND c.status='active'
		 GROUP BY c.kostenart`,
		companyID, f.VehicleID, f.Kostenart, f.Von, f.Bis)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byArt := map[string]CostSum{}
	for rows.Next() {
		var x CostSum
		if err := rows.Scan(&x.Kostenart, &x.Anzahl, &x.BetragNetto); err != nil {
			return nil, err
		}
		byArt[x.Kostenart] = x
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := &CostSummary{Summen: make([]CostSum, 0, len(byArt))}
	// Gesamt in Cent addieren - float-Summen aus Teilsummen runden sonst.
	var cents int64
	for _, k := range Kostenarten {
		if x, ok := byArt[k]; ok {
			out.Summen = append(out.Summen, x)
			cents += roundCents(x.BetragNetto)
		}
	}
	out.Gesamt = float64(cents) / 100
	return out, nil
}

func roundCents(v float64) int64 { return int64(math.Round(v * 100)) }

// StornoCost storniert einen falsch erfassten Beleg - einmalig, mit
// Begruendung, auch am abgemeldeten Fahrzeug. Der Beleg bleibt sichtbar.
func (s *CostService) StornoCost(ctx context.Context, id uuid.UUID, reason, actorUserID, companyID string) (*Cost, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if id == uuid.Nil {
		return nil, errors.New("Kosten-ID erforderlich")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("Storno-Begründung erforderlich")
	}
	ct, err := s.pg.Exec(ctx, `
		UPDATE fleet_costs c SET status='storniert', storniert_at=now(), storno_reason=$3, storniert_by_user_id=$4
		  FROM fleet_vehicles v
		 WHERE c.id=$1 AND v.id=c.vehicle_id AND v.company_id=$2 AND c.status='active'`,
		id, companyID, reason, nullableUserID(actorUserID))
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() != 1 {
		// Keine Zeile: fremd/unbekannt oder schon storniert.
		if _, err := s.GetCost(ctx, id, companyID); err != nil {
			return nil, err // auch "Kosten nicht gefunden"
		}
		return nil, errors.New("Kosten sind bereits storniert")
	}
	return s.GetCost(ctx, id, companyID)
}
