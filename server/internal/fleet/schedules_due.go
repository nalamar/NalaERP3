package fleet

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Betriebsweite Faelligkeitsliste der Fuhrpark-Termine (ADR 0032 §3,
// Backlog G.2.2.3): die Arbeitsliste des Fuhrparks - was gesperrt,
// ueberfaellig oder innerhalb der Vorwarnzeit faellig ist. Dieselbe
// Auswertung wie ListSchedules (EvaluateSchedule, ApplyKmDue);
// TestScheduleDueListMatchesStanding wacht darueber.

// ScheduleDueEntry ist ein offener Punkt: ein Termin eines Fahrzeugs.
type ScheduleDueEntry struct {
	VehicleID          uuid.UUID        `json:"vehicle_id"`
	Kennzeichen        string           `json:"kennzeichen"`
	VehicleBezeichnung string           `json:"vehicle_bezeichnung"`
	ScheduleID         uuid.UUID        `json:"schedule_id"`
	Art                string           `json:"art"`
	Bezeichnung        string           `json:"bezeichnung"`
	Standing           ScheduleStanding `json:"standing"`
}

// dueRank ordnet nach Dringlichkeit: gesperrt vor ueberfaellig vor bald.
func dueRank(status string) int {
	switch status {
	case TerminstandGesperrt:
		return 0
	case TerminstandUeberfaellig:
		return 1
	default:
		return 2
	}
}

var terminOrder = map[string]int{TerminHU: 0, TerminUVV: 1, TerminWartung: 2, TerminVersicherung: 3, TerminLeasing: 4, TerminSonstige: 5}

// DueList liefert fuer alle NICHT abgemeldeten Fahrzeuge des Mandanten die
// AKTIVEN Termine, die am Tag today gesperrt, ueberfaellig oder innerhalb
// der Vorwarnzeit faellig sind. Termine mit Stand "ok" oder ohne Termin
// erscheinen nicht.
//
// Sortierung: das Dringendste zuerst (gesperrt, ueberfaellig, faellig
// bald), darin was am laengsten ueberfaellig ist bzw. am fruehesten
// faellig wird, dann nach Kennzeichen und Terminart.
func (s *ScheduleService) DueList(ctx context.Context, today time.Time, warnDays int, companyID string) ([]ScheduleDueEntry, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	type row struct {
		sc          Schedule
		kennzeichen string
		vehicleBez  string
	}
	rows, err := s.pg.Query(ctx, `SELECT `+scheduleColumns+`, v.kennzeichen, v.bezeichnung
		  FROM fleet_vehicle_schedules s
		  JOIN fleet_vehicles v ON v.id = s.vehicle_id
		 WHERE v.company_id=$1 AND v.abgemeldet_am IS NULL AND s.aktiv`, companyID)
	if err != nil {
		return nil, err
	}
	items := make([]row, 0)
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.sc.ID, &r.sc.VehicleID, &r.sc.Art, &r.sc.Bezeichnung, &r.sc.Starttermin, &r.sc.StartKm, &r.sc.IntervallMonate,
			&r.sc.IntervallKm, &r.sc.Notiz, &r.sc.Aktiv, &r.sc.CreatedAt, &r.kennzeichen, &r.vehicleBez); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(items))
	for i, r := range items {
		ids[i] = r.sc.ID
	}
	completions, err := loadActiveCompletions(ctx, s.pg, ids)
	if err != nil {
		return nil, err
	}
	vehicleIDs := make([]uuid.UUID, len(items))
	for i, r := range items {
		vehicleIDs[i] = r.sc.VehicleID
	}
	kms, err := loadKmStand(ctx, s.pg, vehicleIDs)
	if err != nil {
		return nil, err
	}

	out := make([]ScheduleDueEntry, 0)
	for _, r := range items {
		st := ApplyKmDue(EvaluateSchedule(r.sc.Art, r.sc.Starttermin, r.sc.StartKm, completions[r.sc.ID], today, warnDays),
			kmOf(kms, r.sc.VehicleID), DefaultScheduleWarnKm)
		if st.Status != TerminstandGesperrt && st.Status != TerminstandUeberfaellig && st.Status != TerminstandFaelligBald {
			continue
		}
		out = append(out, ScheduleDueEntry{
			VehicleID: r.sc.VehicleID, Kennzeichen: r.kennzeichen, VehicleBezeichnung: r.vehicleBez,
			ScheduleID: r.sc.ID, Art: r.sc.Art, Bezeichnung: r.sc.Bezeichnung, Standing: st,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return dueLess(out[i], out[j]) })
	return out, nil
}

// dueLess ist die Reihenfolge der Faelligkeitsliste (siehe DueList).
// Vollstaendig bestimmt - sie haengt nicht von der Reihenfolge ab, in der
// die Datenbank die Termine liefert.
func dueLess(a, b ScheduleDueEntry) bool {
	if ra, rb := dueRank(a.Standing.Status), dueRank(b.Standing.Status); ra != rb {
		return ra < rb
	}
	// Ohne Faelligkeit (gesperrt ohne Folgetermin) ans Ende der Gruppe.
	ta, tb := a.Standing.TageBisFaellig, b.Standing.TageBisFaellig
	if (ta == nil) != (tb == nil) {
		return ta != nil
	}
	if ta != nil && *ta != *tb {
		return *ta < *tb
	}
	if a.Kennzeichen != b.Kennzeichen {
		return a.Kennzeichen < b.Kennzeichen
	}
	if terminOrder[a.Art] != terminOrder[b.Art] {
		return terminOrder[a.Art] < terminOrder[b.Art]
	}
	if lb, lbb := strings.ToLower(a.Bezeichnung), strings.ToLower(b.Bezeichnung); lb != lbb {
		return lb < lbb
	}
	return a.ScheduleID.String() < b.ScheduleID.String()
}
