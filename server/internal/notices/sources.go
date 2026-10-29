package notices

import (
	"context"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/assets"
	"nalaerp3/internal/fleet"
	"nalaerp3/internal/hr"
)

// Quellen der Hinweise. Sie nutzen die Faelligkeitslisten und den Stand
// der Bereiche - dieselbe Auswertung wie die Listen, die der Hinweis
// oeffnet. Die kleinen Schnittstellen erlauben Tests ohne Datenbank.

type qualificationDueLister interface {
	DueList(ctx context.Context, today time.Time, warnDays int, companyID string) ([]hr.QualificationDueEntry, error)
}

type qualificationStanding interface {
	Standing(ctx context.Context, employeeID uuid.UUID, today time.Time, warnDays int, companyID string) ([]hr.EmployeeQualificationStanding, error)
}

type assetDueLister interface {
	DueList(ctx context.Context, today time.Time, warnDays int, companyID string) ([]assets.AssetDueEntry, error)
}

type scheduleDueLister interface {
	DueList(ctx context.Context, today time.Time, warnDays int, companyID string) ([]fleet.ScheduleDueEntry, error)
}

var qualificationCritical = map[string]bool{hr.QualificationStandFehlt: true, hr.QualificationStandAbgelaufen: true}

// QualificationsDue: was im Betrieb fehlt, abgelaufen ist oder ablaeuft -
// fuer die Personalabteilung (qualifications.read).
type QualificationsDue struct{ Svc qualificationDueLister }

func (q QualificationsDue) Notices(ctx context.Context, r Recipient, today time.Time) ([]Notice, error) {
	if !r.Can("qualifications.read") {
		return nil, nil
	}
	entries, err := q.Svc.DueList(ctx, today, hr.DefaultQualificationWarnDays, r.CompanyID)
	if err != nil {
		return nil, err
	}
	statuses := make([]string, 0, len(entries))
	for _, e := range entries {
		statuses = append(statuses, e.Status)
	}
	return countNotice(KindQualificationsDue, "Fällige Qualifikationen", statuses, qualificationCritical), nil
}

// OwnQualifications: die eigenen Faelligkeiten eines Mitarbeiters - fuer
// jeden, der mit einem Mitarbeiter verknuepft ist und seine eigenen
// Qualifikationen sehen darf.
type OwnQualifications struct{ Svc qualificationStanding }

func (q OwnQualifications) Notices(ctx context.Context, r Recipient, today time.Time) ([]Notice, error) {
	if r.EmployeeID == nil || !(r.Can("qualifications.self") || r.Can("qualifications.read")) {
		return nil, nil
	}
	standing, err := q.Svc.Standing(ctx, *r.EmployeeID, today, hr.DefaultQualificationWarnDays, r.CompanyID)
	if err != nil {
		return nil, err
	}
	statuses := make([]string, 0)
	for _, st := range standing {
		if st.Status != hr.QualificationStandGueltig {
			statuses = append(statuses, st.Status)
		}
	}
	return countNotice(KindOwnQualifications, "Meine Qualifikationen", statuses, qualificationCritical), nil
}

// AssetInspectionsDue: Pruefungen der Betriebsmittel (assets.read).
type AssetInspectionsDue struct{ Svc assetDueLister }

func (a AssetInspectionsDue) Notices(ctx context.Context, r Recipient, today time.Time) ([]Notice, error) {
	if !r.Can("assets.read") {
		return nil, nil
	}
	entries, err := a.Svc.DueList(ctx, today, assets.DefaultInspectionWarnDays, r.CompanyID)
	if err != nil {
		return nil, err
	}
	statuses := make([]string, 0, len(entries))
	for _, e := range entries {
		statuses = append(statuses, e.Status)
	}
	return countNotice(KindAssetInspectionDue, "Fällige Prüfungen der Betriebsmittel", statuses,
		map[string]bool{assets.PruefstandGesperrt: true, assets.PruefstandUeberfaellig: true, assets.PruefstandNieGeprueft: true}), nil
}

// FleetSchedulesDue: Termine der Fahrzeuge (fleet.read).
type FleetSchedulesDue struct{ Svc scheduleDueLister }

func (f FleetSchedulesDue) Notices(ctx context.Context, r Recipient, today time.Time) ([]Notice, error) {
	if !r.Can("fleet.read") {
		return nil, nil
	}
	entries, err := f.Svc.DueList(ctx, today, fleet.DefaultScheduleWarnDays, r.CompanyID)
	if err != nil {
		return nil, err
	}
	statuses := make([]string, 0, len(entries))
	for _, e := range entries {
		statuses = append(statuses, e.Standing.Status)
	}
	return countNotice(KindFleetSchedulesDue, "Fällige Fahrzeugtermine", statuses,
		map[string]bool{fleet.TerminstandGesperrt: true, fleet.TerminstandUeberfaellig: true}), nil
}

// qualificationSource: hr.QualificationService erfuellt beides.
type qualificationSource interface {
	qualificationDueLister
	qualificationStanding
}

// NewDefaultService baut den Service mit allen Quellen - an EINER Stelle,
// damit GET /notices und der E-Mail-Versand (F.14.4) dieselben Hinweise
// liefern.
func NewDefaultService(q qualificationSource, a assetDueLister, f scheduleDueLister) *Service {
	return NewService(
		QualificationsDue{Svc: q},
		OwnQualifications{Svc: q},
		AssetInspectionsDue{Svc: a},
		FleetSchedulesDue{Svc: f},
	)
}
