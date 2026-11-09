package production

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/hr"
)

// Auslastung je Arbeitsplatz und Woche (ADR 0037 §4/§5, Backlog H.2.4):
// verfuegbar = Stunden je Tag x Arbeitstage Mo-Fr ohne gesetzliche
// Feiertage (Bundesland des Mandanten) und Betriebsruhe; geplant = Summe
// der Planzeiten der Arbeitsgaenge nicht fertiger, nicht stornierter
// Auftraege in dieser Woche. Ueberlast ist ein Hinweis, keine Sperre.

const (
	// MaxCapacityWeeks begrenzt den Zeitraum einer Abfrage.
	MaxCapacityWeeks = 26
	HintUeberlast    = "ueberlast"
)

var (
	errCapacityRange   = errors.New("Ungültiger Zeitraum: bis liegt vor von")
	errCapacityTooLong = errors.New("Zeitraum ist zu groß (höchstens 26 Wochen)")
)

type CapacityWeek struct {
	// Montag der Woche
	Woche         time.Time `json:"woche"`
	Arbeitstage   int       `json:"arbeitstage"`
	VerfuegbarMin float64   `json:"verfuegbar_min"`
	GeplantMin    float64   `json:"geplant_min"`
	Hinweise      []string  `json:"hinweise"`
}

type CapacityRow struct {
	WorkCenterID uuid.UUID      `json:"work_center_id"`
	Name         string         `json:"name"`
	StundenJeTag float64        `json:"stunden_je_tag"`
	Aktiv        bool           `json:"aktiv"`
	Wochen       []CapacityWeek `json:"wochen"`
	// Planzeit der Arbeitsgaenge ohne geplante Woche (offene Auftraege).
	UngeplantMin float64 `json:"ungeplant_min"`
}

type Capacity struct {
	Von            time.Time     `json:"von"`
	Bis            time.Time     `json:"bis"`
	Arbeitsplaetze []CapacityRow `json:"arbeitsplaetze"`
}

// capacityWeeks: Montage von der Woche von bis zur Woche von bis.
func capacityWeeks(von, bis time.Time) ([]time.Time, error) {
	start, end := WeekStart(von), WeekStart(bis)
	if end.Before(start) {
		return nil, errCapacityRange
	}
	var out []time.Time
	for w := start; !w.After(end); w = w.AddDate(0, 0, 7) {
		if len(out) == MaxCapacityWeeks {
			return nil, errCapacityTooLong
		}
		out = append(out, w)
	}
	return out, nil
}

// freeDays: gesetzliche Feiertage und Betriebsruhe des Mandanten in den
// Jahren der Wochen.
func freeDays(ctx context.Context, cal *hr.CalendarService, weeks []time.Time, companyID string) (func(time.Time) bool, error) {
	free := map[time.Time]bool{}
	years := map[int]bool{}
	for _, w := range weeks {
		years[w.Year()] = true
		years[w.AddDate(0, 0, 4).Year()] = true
	}
	for y := range years {
		c, err := cal.Calendar(ctx, y, companyID)
		if err != nil {
			return nil, err
		}
		for _, d := range c.Days {
			free[dateOnly(d.Date)] = true
		}
	}
	return func(d time.Time) bool { return free[dateOnly(d)] }, nil
}

// Capacity: alle aktiven Arbeitsplaetze des Mandanten, dazu deaktivierte,
// auf denen noch offene Arbeit geplant ist; je Woche des Zeitraums
// verfuegbar, geplant und Hinweis "ueberlast".
func (s *Service) Capacity(ctx context.Context, von, bis time.Time, companyID string) (*Capacity, error) {
	weeks, err := capacityWeeks(von, bis)
	if err != nil {
		return nil, err
	}
	isFree, err := freeDays(ctx, hr.NewCalendarService(s.pg), weeks, companyID)
	if err != nil {
		return nil, err
	}

	// offene Arbeit: im Zeitraum oder ungeplant
	type load struct {
		byWeek    map[time.Time]float64
		ungeplant float64
	}
	loads := map[uuid.UUID]*load{}
	rows, err := s.pg.Query(ctx, `SELECT op.work_center_id, op.geplante_woche, op.ruestzeit_min::float8, op.minuten_je_stueck::float8, o.stueckzahl
		  FROM production_order_operations op
		  JOIN production_orders o ON o.id = op.order_id
		 WHERE o.company_id=$1 AND o.status NOT IN ('fertig', 'storniert')
		   AND (op.geplante_woche IS NULL OR op.geplante_woche BETWEEN $2 AND $3)`,
		companyID, weeks[0], weeks[len(weeks)-1])
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var wc uuid.UUID
		var woche *time.Time
		var ruest, jeStueck float64
		var stueck int
		if err := rows.Scan(&wc, &woche, &ruest, &jeStueck, &stueck); err != nil {
			rows.Close()
			return nil, err
		}
		l := loads[wc]
		if l == nil {
			l = &load{byWeek: map[time.Time]float64{}}
			loads[wc] = l
		}
		m := PlannedMinutes(ruest, jeStueck, stueck)
		if woche == nil {
			l.ungeplant += m
		} else {
			l.byWeek[dateOnly(*woche)] += m
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	centers, err := s.ListWorkCenters(ctx, companyID, true)
	if err != nil {
		return nil, err
	}
	out := &Capacity{Von: weeks[0], Bis: weeks[len(weeks)-1], Arbeitsplaetze: []CapacityRow{}}
	for _, c := range centers {
		l := loads[c.ID]
		if !c.Aktiv && l == nil {
			continue
		}
		row := CapacityRow{WorkCenterID: c.ID, Name: c.Name, StundenJeTag: c.StundenJeTag, Aktiv: c.Aktiv, Wochen: make([]CapacityWeek, 0, len(weeks))}
		if l != nil {
			row.UngeplantMin = l.ungeplant
		}
		for _, w := range weeks {
			days := WeekWorkDays(w, isFree)
			cw := CapacityWeek{Woche: w, Arbeitstage: days, VerfuegbarMin: AvailableMinutes(c.StundenJeTag, days), Hinweise: []string{}}
			if l != nil {
				cw.GeplantMin = l.byWeek[w]
			}
			if Overloaded(cw.GeplantMin, cw.VerfuegbarMin) {
				cw.Hinweise = append(cw.Hinweise, HintUeberlast)
			}
			row.Wochen = append(row.Wochen, cw)
		}
		out.Arbeitsplaetze = append(out.Arbeitsplaetze, row)
	}
	return out, nil
}
