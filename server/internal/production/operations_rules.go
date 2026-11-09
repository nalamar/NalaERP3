package production

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Fachregeln der Arbeitsgaenge und Kapazitaeten ohne Datenbank (ADR 0037,
// Backlog H.2.1). Tabellen in Migration 112.

// MaxMinuten begrenzt Ruestzeit und Minuten je Stueck gegen Tippfehler
// (10000 Minuten sind rund eine Woche Arbeit an einem Stueck).
const MaxMinuten = 10000

type WorkCenterInput struct {
	Name         string  `json:"name"`
	StundenJeTag float64 `json:"stunden_je_tag"`
}

func NormalizeWorkCenterInput(in WorkCenterInput) WorkCenterInput {
	in.Name = strings.TrimSpace(in.Name)
	return in
}

func ValidateWorkCenterInput(in WorkCenterInput) error {
	if in.Name == "" {
		return errors.New("Name des Arbeitsplatzes erforderlich")
	}
	if in.StundenJeTag <= 0 || in.StundenJeTag > 24 {
		return errors.New("Stunden je Tag müssen größer als 0 und höchstens 24 sein")
	}
	return nil
}

type TemplateInput struct {
	Name            string    `json:"name"`
	WorkCenterID    uuid.UUID `json:"work_center_id"`
	Reihenfolge     int       `json:"reihenfolge"`
	RuestzeitMin    float64   `json:"ruestzeit_min"`
	MinutenJeStueck float64   `json:"minuten_je_stueck"`
}

func NormalizeTemplateInput(in TemplateInput) TemplateInput {
	in.Name = strings.TrimSpace(in.Name)
	return in
}

func ValidateTemplateInput(in TemplateInput) error {
	if in.Name == "" {
		return errors.New("Name des Arbeitsgangs erforderlich")
	}
	if in.WorkCenterID == uuid.Nil {
		return errors.New("Arbeitsplatz erforderlich")
	}
	if in.Reihenfolge <= 0 {
		return errors.New("Reihenfolge muss größer als 0 sein")
	}
	return validateTimes(in.RuestzeitMin, in.MinutenJeStueck)
}

type OperationInput struct {
	Name            string    `json:"name"`
	WorkCenterID    uuid.UUID `json:"work_center_id"`
	RuestzeitMin    float64   `json:"ruestzeit_min"`
	MinutenJeStueck float64   `json:"minuten_je_stueck"`
	// Ein beliebiger Tag der Woche; gespeichert wird ihr Montag.
	GeplanteWoche *time.Time `json:"geplante_woche"`
}

// NormalizeOperationInput entfernt Leerraum und setzt die geplante Woche
// auf ihren Montag.
func NormalizeOperationInput(in OperationInput) OperationInput {
	in.Name = strings.TrimSpace(in.Name)
	if in.GeplanteWoche != nil {
		w := WeekStart(*in.GeplanteWoche)
		in.GeplanteWoche = &w
	}
	return in
}

func ValidateOperationInput(in OperationInput) error {
	if in.Name == "" {
		return errors.New("Name des Arbeitsgangs erforderlich")
	}
	if in.WorkCenterID == uuid.Nil {
		return errors.New("Arbeitsplatz erforderlich")
	}
	return validateTimes(in.RuestzeitMin, in.MinutenJeStueck)
}

func validateTimes(ruest, jeStueck float64) error {
	if ruest < 0 || jeStueck < 0 {
		return errors.New("Zeiten dürfen nicht negativ sein")
	}
	if ruest > MaxMinuten || jeStueck > MaxMinuten {
		return errors.New("Zeit ist zu groß")
	}
	return nil
}

// PlannedMinutes: Planzeit eines Arbeitsgangs fuer den ganzen Auftrag.
func PlannedMinutes(ruestzeitMin, minutenJeStueck float64, stueckzahl int) float64 {
	return ruestzeitMin + minutenJeStueck*float64(stueckzahl)
}

// WeekStart liefert den Montag der ISO-Woche (nur Datum, UTC).
func WeekStart(t time.Time) time.Time {
	d := dateOnly(t)
	back := (int(d.Weekday()) + 6) % 7 // Montag 0 ... Sonntag 6
	return d.AddDate(0, 0, -back)
}

// DefaultWeeks schlaegt fuer n Arbeitsgaenge (in ihrer Reihenfolge) je
// eine Woche vor: der letzte in der Woche vor der Woche des Liefertermins,
// jeder fruehere eine Woche davor (ADR 0037 §5). Ohne Liefertermin bleibt
// alles ungeplant (nil).
func DefaultWeeks(liefertermin *time.Time, n int) []*time.Time {
	out := make([]*time.Time, n)
	if liefertermin == nil {
		return out
	}
	last := WeekStart(*liefertermin).AddDate(0, 0, -7)
	for i := 0; i < n; i++ {
		w := last.AddDate(0, 0, -7*(n-1-i))
		out[i] = &w
	}
	return out
}

// AfterDelivery: die geplante Woche beginnt erst nach dem Liefertermin
// (Hinweis, keine Sperre). Die Woche des Liefertermins selbst zaehlt
// nicht.
func AfterDelivery(geplanteWoche, liefertermin *time.Time) bool {
	if geplanteWoche == nil || liefertermin == nil {
		return false
	}
	return WeekStart(*geplanteWoche).After(dateOnly(*liefertermin))
}

// WeekWorkDays zaehlt die Arbeitstage Mo-Fr der Woche ab montag, ohne die
// Tage, fuer die isFree true liefert (Feiertage, Betriebsruhe).
func WeekWorkDays(montag time.Time, isFree func(time.Time) bool) int {
	start := WeekStart(montag)
	n := 0
	for i := 0; i < 5; i++ {
		d := start.AddDate(0, 0, i)
		if isFree == nil || !isFree(d) {
			n++
		}
	}
	return n
}

// AvailableMinutes: Kapazitaet eines Arbeitsplatzes in einer Woche.
func AvailableMinutes(stundenJeTag float64, arbeitstage int) float64 {
	return stundenJeTag * 60 * float64(arbeitstage)
}

// Overloaded: geplante Minuten liegen ueber der Kapazitaet; Rundungs-
// rauschen zaehlt nicht.
func Overloaded(geplantMin, verfuegbarMin float64) bool {
	return geplantMin > verfuegbarMin+1e-6
}
