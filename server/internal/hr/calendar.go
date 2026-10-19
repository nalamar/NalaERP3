package hr

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Feiertagskalender je Mandant (ADR 0029, Backlog F.8.2.2): welches
// Bundesland gilt, die bayerischen Schalter und die betrieblichen
// Zusatztage. Die gesetzlichen Feiertage selbst berechnet holidays.go.

// CalendarSettings ist die Einstellung eines Mandanten. Bundesland ""
// heisst "noch nicht eingestellt": dann gelten nur die neun bundesweiten
// Feiertage.
type CalendarSettings struct {
	Bundesland             string     `json:"bundesland"`
	MariaeHimmelfahrt      bool       `json:"mariae_himmelfahrt"`
	AugsburgerFriedensfest bool       `json:"augsburger_friedensfest"`
	UpdatedAt              *time.Time `json:"updated_at,omitempty"`
}

// CompanyDay ist ein betrieblicher freier Tag (Betriebsruhe, 24.12.).
type CompanyDay struct {
	ID        uuid.UUID `json:"id"`
	Datum     time.Time `json:"datum"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Arten eines Kalendertags.
const (
	CalendarDayGesetzlich  = "gesetzlich"
	CalendarDayBetrieblich = "betrieblich"
)

// CalendarDay ist ein freier Tag im Kalender eines Mandanten.
type CalendarDay struct {
	Date time.Time `json:"date"`
	Name string    `json:"name"`
	Art  string    `json:"art"`
}

// CompanyCalendar ist der Kalender eines Jahres: das angewandte
// Bundesland ("" = nur bundesweite Feiertage) und alle freien Tage.
type CompanyCalendar struct {
	Year       int           `json:"year"`
	Bundesland string        `json:"bundesland"`
	Days       []CalendarDay `json:"days"`
}

// Jahre ausserhalb dieses Bereichs sind ein Eingabefehler, kein Kalender.
const (
	minCalendarYear = 1900
	maxCalendarYear = 2200
)

var errCalendarYear = errors.New("Ungültiges Jahr")

type CalendarService struct{ pg *pgxpool.Pool }

func NewCalendarService(pg *pgxpool.Pool) *CalendarService { return &CalendarService{pg: pg} }

// GetSettings liefert die Einstellung; ohne gespeicherte Zeile die leere.
func (s *CalendarService) GetSettings(ctx context.Context, companyID string) (*CalendarSettings, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	var out CalendarSettings
	var land *string
	var updated time.Time
	err := s.pg.QueryRow(ctx, `
        SELECT bundesland, mariae_himmelfahrt, augsburger_friedensfest, updated_at
          FROM hr_calendar_settings WHERE company_id=$1
    `, companyID).Scan(&land, &out.MariaeHimmelfahrt, &out.AugsburgerFriedensfest, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return &CalendarSettings{}, nil
	}
	if err != nil {
		return nil, err
	}
	if land != nil {
		out.Bundesland = *land
	}
	out.UpdatedAt = &updated
	return &out, nil
}

// SetSettings ersetzt die Einstellung vollstaendig. Bundesland "" setzt
// sie zurueck auf "nicht eingestellt"; Kleinschreibung wird angenommen.
// Die Schalter werden ausserhalb Bayerns gespeichert, wirken dort aber
// nicht (PublicHolidays).
func (s *CalendarService) SetSettings(ctx context.Context, in CalendarSettings, actorUserID, companyID string) (*CalendarSettings, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	land := strings.ToUpper(strings.TrimSpace(in.Bundesland))
	var landArg any
	if land != "" {
		if _, ok := Bundeslaender[land]; !ok {
			return nil, ErrUnknownBundesland
		}
		landArg = land
	}
	if _, err := s.pg.Exec(ctx, `
        INSERT INTO hr_calendar_settings (company_id, bundesland, mariae_himmelfahrt, augsburger_friedensfest, updated_at, updated_by_user_id)
        VALUES ($1,$2,$3,$4,now(),$5)
        ON CONFLICT (company_id) DO UPDATE SET
            bundesland=EXCLUDED.bundesland,
            mariae_himmelfahrt=EXCLUDED.mariae_himmelfahrt,
            augsburger_friedensfest=EXCLUDED.augsburger_friedensfest,
            updated_at=EXCLUDED.updated_at,
            updated_by_user_id=EXCLUDED.updated_by_user_id
    `, companyID, landArg, in.MariaeHimmelfahrt, in.AugsburgerFriedensfest, nullableUserID(actorUserID)); err != nil {
		return nil, err
	}
	return s.GetSettings(ctx, companyID)
}

func validCalendarYear(year int) bool { return year >= minCalendarYear && year <= maxCalendarYear }

// ListCompanyDays liefert die betrieblichen Zusatztage eines Jahres, nach
// Datum sortiert.
func (s *CalendarService) ListCompanyDays(ctx context.Context, year int, companyID string) ([]CompanyDay, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if !validCalendarYear(year) {
		return nil, errCalendarYear
	}
	rows, err := s.pg.Query(ctx, `
        SELECT id, datum, name, created_at FROM hr_company_days
         WHERE company_id=$1 AND datum >= $2 AND datum <= $3
         ORDER BY datum
    `, companyID, calendarDay(year, time.January, 1), calendarDay(year, time.December, 31))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CompanyDay{}
	for rows.Next() {
		var d CompanyDay
		if err := rows.Scan(&d.ID, &d.Datum, &d.Name, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CreateCompanyDay traegt einen betrieblichen freien Tag ein. Ein Datum
// gibt es je Mandant nur einmal.
func (s *CalendarService) CreateCompanyDay(ctx context.Context, datum time.Time, name, actorUserID, companyID string) (*CompanyDay, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if datum.IsZero() {
		return nil, errors.New("Datum erforderlich")
	}
	if !validCalendarYear(datum.Year()) {
		return nil, errCalendarYear
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("Bezeichnung erforderlich")
	}
	d := CompanyDay{ID: uuid.New(), Datum: dateOnly(datum), Name: name}
	err := s.pg.QueryRow(ctx, `
        INSERT INTO hr_company_days (id, company_id, datum, name, created_by_user_id)
        VALUES ($1,$2,$3,$4,$5) RETURNING created_at
    `, d.ID, companyID, d.Datum, d.Name, nullableUserID(actorUserID)).Scan(&d.CreatedAt)
	if isUniqueViolation(err, "idx_hr_company_days_company_datum") {
		return nil, errors.New("Betrieblicher freier Tag für dieses Datum ist bereits erfasst")
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// DeleteCompanyDay entfernt einen betrieblichen freien Tag - eine
// Planungsangabe, kein Nachweis, deshalb ohne Storno.
func (s *CalendarService) DeleteCompanyDay(ctx context.Context, id uuid.UUID, companyID string) error {
	if strings.TrimSpace(companyID) == "" {
		return errors.New("Mandant erforderlich")
	}
	tag, err := s.pg.Exec(ctx, `DELETE FROM hr_company_days WHERE id=$1 AND company_id=$2`, id, companyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("Betrieblicher freier Tag nicht gefunden")
	}
	return nil
}

// Calendar liefert alle freien Tage eines Jahres fuer den Mandanten:
// gesetzliche nach eingestelltem Bundesland (sonst nur die bundesweiten)
// und betriebliche, nach Datum sortiert. Faellt ein betrieblicher Tag auf
// einen Feiertag, erscheinen beide.
func (s *CalendarService) Calendar(ctx context.Context, year int, companyID string) (*CompanyCalendar, error) {
	if !validCalendarYear(year) {
		return nil, errCalendarYear
	}
	settings, err := s.GetSettings(ctx, companyID)
	if err != nil {
		return nil, err
	}
	var public []Holiday
	if settings.Bundesland == "" {
		public = NationwideHolidays(year)
	} else {
		public, err = PublicHolidays(year, settings.Bundesland, HolidayOptions{
			MariaeHimmelfahrt:      settings.MariaeHimmelfahrt,
			AugsburgerFriedensfest: settings.AugsburgerFriedensfest,
		})
		if err != nil {
			return nil, err
		}
	}
	companyDays, err := s.ListCompanyDays(ctx, year, companyID)
	if err != nil {
		return nil, err
	}
	out := &CompanyCalendar{Year: year, Bundesland: settings.Bundesland, Days: make([]CalendarDay, 0, len(public)+len(companyDays))}
	for _, h := range public {
		out.Days = append(out.Days, CalendarDay{Date: h.Date, Name: h.Name, Art: CalendarDayGesetzlich})
	}
	for _, d := range companyDays {
		out.Days = append(out.Days, CalendarDay{Date: dateOnly(d.Datum), Name: d.Name, Art: CalendarDayBetrieblich})
	}
	sort.SliceStable(out.Days, func(i, j int) bool { return out.Days[i].Date.Before(out.Days[j].Date) })
	return out, nil
}
