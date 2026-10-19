package hr

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Urlaubskonto (ADR 0029, Backlog F.8.3.1).

// LeaveDaysPerWorkday: 20 Tage bei der 5-Tage-Woche (Entscheidung des
// Nutzers), anteilig 4 Tage je woechentlichem Arbeitstag. Das ist zugleich
// der gesetzliche Mindesturlaub (24 Werktage bei 6 Werktagen je Woche,
// §3 BUrlG) - Vorgabe und Untergrenze fallen zusammen.
const LeaveDaysPerWorkday = 4

// DefaultLeaveDays ist die Vorgabe und Untergrenze fuer einen Mitarbeiter
// mit der angegebenen Zahl von Arbeitstagen je Woche.
func DefaultLeaveDays(arbeitstageProWoche int) float64 {
	return float64(LeaveDaysPerWorkday * arbeitstageProWoche)
}

func formatDays(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', -1, 64), ".", ",", 1)
}

// validateHalfDays: Tage nur in halben Schritten, nicht negativ, hoechstens
// ein Jahr.
func validateHalfDays(v float64, field string) error {
	if v < 0 || v > 366 || math.IsNaN(v) || v*2 != math.Trunc(v*2) {
		return fmt.Errorf("Ungültiger Wert für %s: ganze oder halbe Tage von 0 bis 366", field)
	}
	return nil
}

// validateEmployeeLeaveDays prueft einen eingetragenen Jahresanspruch gegen
// die Untergrenze der Arbeitswoche. nil = Vorgabe, immer gueltig.
func validateEmployeeLeaveDays(v *float64, arbeitstage []int) error {
	if v == nil {
		return nil
	}
	if err := validateHalfDays(*v, "urlaubstage_jahr"); err != nil {
		return err
	}
	if minimum := DefaultLeaveDays(len(arbeitstage)); *v < minimum {
		return fmt.Errorf("Ungültiger Urlaubsanspruch: mindestens %s Tage bei %d Arbeitstagen je Woche (gesetzlicher Mindesturlaub)", formatDays(minimum), len(arbeitstage))
	}
	return nil
}

// LeaveAccount ist der Stand des Urlaubskontos eines Jahres.
//
//   - Rest = Anspruch + Uebertrag - Genommen (genehmigter Urlaub).
//   - Verfuegbar = Rest - Verplant (offene Urlaubsantraege).
//
// Festgelegt ist false, solange die Personalabteilung fuer das Jahr nichts
// eingetragen hat; dann gilt der Jahresanspruch des Mitarbeiters.
type LeaveAccount struct {
	EmployeeID       uuid.UUID  `json:"employee_id"`
	Jahr             int        `json:"jahr"`
	AnspruchTage     float64    `json:"anspruch_tage"`
	UebertragTage    float64    `json:"uebertrag_tage"`
	GenommenTage     float64    `json:"genommen_tage"`
	VerplantTage     float64    `json:"verplant_tage"`
	RestTage         float64    `json:"rest_tage"`
	VerfuegbarTage   float64    `json:"verfuegbar_tage"`
	HinweisErteiltAm *time.Time `json:"hinweis_erteilt_am,omitempty"`
	Notiz            string     `json:"notiz"`
	Festgelegt       bool       `json:"festgelegt"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
}

// LeaveAccountInput ersetzt das Konto eines Jahres vollstaendig.
type LeaveAccountInput struct {
	AnspruchTage     float64
	UebertragTage    float64
	HinweisErteiltAm *time.Time
	Notiz            string
}

type LeaveAccountService struct{ pg *pgxpool.Pool }

func NewLeaveAccountService(pg *pgxpool.Pool) *LeaveAccountService {
	return &LeaveAccountService{pg: pg}
}

// Get liefert das Konto eines Jahres.
func (s *LeaveAccountService) Get(ctx context.Context, employeeID uuid.UUID, year int, companyID string) (*LeaveAccount, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if !validCalendarYear(year) {
		return nil, errCalendarYear
	}
	var yearly *float64
	var days []int16
	err := s.pg.QueryRow(ctx, `SELECT urlaubstage_jahr::float8, arbeitstage FROM hr_employees WHERE id=$1 AND company_id=$2`, employeeID, companyID).Scan(&yearly, &days)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("Mitarbeiter nicht gefunden")
	}
	if err != nil {
		return nil, err
	}
	acc := &LeaveAccount{EmployeeID: employeeID, Jahr: year}
	if yearly != nil {
		acc.AnspruchTage = *yearly
	} else {
		acc.AnspruchTage = DefaultLeaveDays(len(days))
	}
	var updated time.Time
	err = s.pg.QueryRow(ctx, `
        SELECT anspruch_tage::float8, uebertrag_tage::float8, hinweis_erteilt_am, notiz, updated_at
          FROM hr_leave_accounts WHERE employee_id=$1 AND jahr=$2
    `, employeeID, year).Scan(&acc.AnspruchTage, &acc.UebertragTage, &acc.HinweisErteiltAm, &acc.Notiz, &updated)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return nil, err
	default:
		acc.Festgelegt = true
		acc.UpdatedAt = &updated
	}
	if acc.GenommenTage, acc.VerplantTage, err = s.usedDays(ctx, employeeID, year, companyID); err != nil {
		return nil, err
	}
	acc.RestTage = acc.AnspruchTage + acc.UebertragTage - acc.GenommenTage
	acc.VerfuegbarTage = acc.RestTage - acc.VerplantTage
	return acc, nil
}

// usedDays summiert genehmigten und offenen Urlaub (Typ vacation) im Jahr.
// Ein Antrag ueber den Jahreswechsel wird nach Arbeitstagen auf die Jahre
// verteilt; seine gespeicherte Tageszahl (z. B. ein halber Tag weniger)
// wird dabei anteilig uebernommen.
func (s *LeaveAccountService) usedDays(ctx context.Context, employeeID uuid.UUID, year int, companyID string) (taken, planned float64, err error) {
	first, last := calendarDay(year, time.January, 1), calendarDay(year, time.December, 31)
	rows, err := s.pg.Query(ctx, `
        SELECT status, start_date, end_date, days::float8 FROM hr_leave_requests
         WHERE employee_id=$1 AND typ='vacation' AND status IN ('approved','pending')
           AND start_date <= $3 AND end_date >= $2
    `, employeeID, first, last)
	if err != nil {
		return 0, 0, err
	}
	type req struct {
		status     string
		start, end time.Time
		days       float64
	}
	var reqs []req
	for rows.Next() {
		var r req
		if err := rows.Scan(&r.status, &r.start, &r.end, &r.days); err != nil {
			rows.Close()
			return 0, 0, err
		}
		reqs = append(reqs, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	cal := NewCalendarService(s.pg)
	for _, r := range reqs {
		share := r.days
		if r.start.Before(first) || r.end.After(last) {
			total, err := cal.EmployeeWorkDays(ctx, employeeID, r.start, r.end, companyID)
			if err != nil {
				return 0, 0, err
			}
			from, to := r.start, r.end
			if from.Before(first) {
				from = first
			}
			if to.After(last) {
				to = last
			}
			inYear, err := cal.EmployeeWorkDays(ctx, employeeID, from, to, companyID)
			if err != nil {
				return 0, 0, err
			}
			share = 0
			if total > 0 {
				share = math.Round(r.days*float64(inYear)/float64(total)*2) / 2
			}
		}
		if r.status == "approved" {
			taken += share
		} else {
			planned += share
		}
	}
	return taken, planned, nil
}

// Set legt das Konto eines Jahres fest (vollstaendiger Ersatz). Der
// Anspruch darf hier unter der Untergrenze liegen - Teilurlaub im Ein-
// oder Austrittsjahr (§5 BUrlG).
func (s *LeaveAccountService) Set(ctx context.Context, employeeID uuid.UUID, year int, in LeaveAccountInput, actorUserID, companyID string) (*LeaveAccount, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if !validCalendarYear(year) {
		return nil, errCalendarYear
	}
	if err := validateHalfDays(in.AnspruchTage, "anspruch_tage"); err != nil {
		return nil, err
	}
	if err := validateHalfDays(in.UebertragTage, "uebertrag_tage"); err != nil {
		return nil, err
	}
	var owned bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hr_employees WHERE id=$1 AND company_id=$2)`, employeeID, companyID).Scan(&owned); err != nil {
		return nil, err
	}
	if !owned {
		return nil, errors.New("Mitarbeiter nicht gefunden")
	}
	var hint any
	if in.HinweisErteiltAm != nil {
		hint = dateOnly(*in.HinweisErteiltAm)
	}
	if _, err := s.pg.Exec(ctx, `
        INSERT INTO hr_leave_accounts (employee_id, jahr, anspruch_tage, uebertrag_tage, hinweis_erteilt_am, notiz, updated_at, updated_by_user_id)
        VALUES ($1,$2,$3,$4,$5,$6,now(),$7)
        ON CONFLICT (employee_id, jahr) DO UPDATE SET
            anspruch_tage=EXCLUDED.anspruch_tage,
            uebertrag_tage=EXCLUDED.uebertrag_tage,
            hinweis_erteilt_am=EXCLUDED.hinweis_erteilt_am,
            notiz=EXCLUDED.notiz,
            updated_at=EXCLUDED.updated_at,
            updated_by_user_id=EXCLUDED.updated_by_user_id
    `, employeeID, year, in.AnspruchTage, in.UebertragTage, hint, strings.TrimSpace(in.Notiz), nullableUserID(actorUserID)); err != nil {
		return nil, err
	}
	return s.Get(ctx, employeeID, year, companyID)
}
