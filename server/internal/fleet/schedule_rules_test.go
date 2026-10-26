package fleet

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEndOfMonthAndNormalizeDue(t *testing.T) {
	for in, want := range map[time.Time]time.Time{
		day(2026, 3, 1):   day(2026, 3, 31),
		day(2026, 2, 10):  day(2026, 2, 28),
		day(2028, 2, 10):  day(2028, 2, 29),
		day(2026, 12, 31): day(2026, 12, 31),
	} {
		if got := EndOfMonth(in); !got.Equal(want) {
			t.Errorf("EndOfMonth(%v) = %v, erwartet %v", in, got, want)
		}
	}
	withTime := time.Date(2026, 5, 7, 15, 30, 0, 0, time.UTC)
	if got := NormalizeDue(TerminHU, withTime); !got.Equal(day(2026, 5, 31)) {
		t.Fatalf("HU: %v", got)
	}
	if got := NormalizeDue(TerminUVV, withTime); !got.Equal(day(2026, 5, 7)) {
		t.Fatalf("UVV: %v", got)
	}
}

func TestNextDue(t *testing.T) {
	if got := NextDue(TerminHU, day(2026, 3, 10), intPtr(24)); got == nil || !got.Equal(day(2028, 3, 31)) {
		t.Fatalf("HU: %v", got)
	}
	if got := NextDue(TerminUVV, day(2026, 3, 10), intPtr(12)); got == nil || !got.Equal(day(2027, 3, 10)) {
		t.Fatalf("UVV: %v", got)
	}
	// Monatsende wie bei den Qualifikationen.
	if got := NextDue(TerminWartung, day(2026, 1, 31), intPtr(1)); got == nil || !got.Equal(day(2026, 2, 28)) {
		t.Fatalf("Wartung Monatsende: %v", got)
	}
	if got := NextDue(TerminLeasing, day(2026, 1, 31), nil); got != nil {
		t.Fatalf("ohne Intervall: %v", got)
	}
}

func TestDefaultIntervalMonthsAndLabels(t *testing.T) {
	for art, want := range map[string]*int{TerminHU: intPtr(24), TerminUVV: intPtr(12), TerminVersicherung: intPtr(12), TerminWartung: nil, TerminLeasing: nil, TerminSonstige: nil} {
		got := DefaultIntervalMonths(art)
		if (got == nil) != (want == nil) || (got != nil && *got != *want) {
			t.Errorf("%s: %v, erwartet %v", art, got, want)
		}
	}
	if TerminLabel(TerminHU, "") != "Hauptuntersuchung" || TerminLabel(TerminUVV, "") != "UVV-Prüfung" || TerminLabel(TerminSonstige, "Tacho") != "Tacho" {
		t.Fatal("Bezeichnungen")
	}
}

func TestValidateScheduleInput(t *testing.T) {
	if err := ValidateScheduleInput(TerminHU, "", nil, intPtr(24), nil); err != nil {
		t.Fatalf("gueltig: %v", err)
	}
	if err := ValidateScheduleInput(TerminSonstige, "Tachograph", nil, nil, nil); err != nil {
		t.Fatalf("sonstige: %v", err)
	}
	if err := ValidateScheduleInput(TerminWartung, "", intPtr(30000), nil, intPtr(30000)); err != nil {
		t.Fatalf("Wartung nach km: %v", err)
	}
	for _, tc := range []struct {
		name                  string
		art, bez              string
		startKm, monate, ivKm *int
		want                  string
	}{
		{"Art unbekannt", "tuev", "", nil, nil, nil, "Ungültige Terminart"},
		{"sonstige ohne Bezeichnung", TerminSonstige, "", nil, nil, nil, "Bezeichnung erforderlich"},
		{"HU mit Bezeichnung", TerminHU, "x", nil, nil, nil, "Bezeichnung nur bei sonstigen Terminen"},
		{"Start-km 0", TerminWartung, "", intPtr(0), nil, nil, "Kilometerstand muss größer als 0 sein"},
		{"Intervall 0", TerminUVV, "", nil, intPtr(0), nil, "Intervall muss größer als 0 sein"},
		{"Intervall km 0", TerminWartung, "", nil, nil, intPtr(0), "Intervall muss größer als 0 sein"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateScheduleInput(tc.art, tc.bez, tc.startKm, tc.monate, tc.ivKm); err == nil || err.Error() != tc.want {
				t.Fatalf("erwartet %q, bekommen %v", tc.want, err)
			}
		})
	}
}

func TestValidateCompletion(t *testing.T) {
	today := day(2026, 10, 26)
	ez := datePtr(2020, 6, 1)
	if err := ValidateCompletion(TerminHU, today, ErgebnisBestanden, datePtr(2028, 10, 31), intPtr(1000), nil, ez, today); err != nil {
		t.Fatalf("HU gueltig: %v", err)
	}
	if err := ValidateCompletion(TerminHU, day(2026, 10, 1), ErgebnisNichtBestanden, datePtr(2026, 11, 30), nil, nil, ez, today); err != nil {
		t.Fatalf("nicht bestanden mit Nachpruefungsfrist: %v", err)
	}
	if err := ValidateCompletion(TerminWartung, day(2020, 6, 1), ErgebnisErledigt, nil, intPtr(0), intPtr(30000), ez, today); err != nil {
		t.Fatalf("Wartung am Tag der Erstzulassung, km 0: %v", err)
	}
	for _, tc := range []struct {
		name     string
		art      string
		am       time.Time
		ergebnis string
		next     *time.Time
		km, nkm  *int
		want     string
	}{
		{"ohne Datum", TerminUVV, time.Time{}, ErgebnisBestanden, nil, nil, nil, "Datum erforderlich"},
		{"morgen", TerminUVV, day(2026, 10, 27), ErgebnisBestanden, nil, nil, nil, "Datum darf nicht in der Zukunft liegen"},
		{"vor Erstzulassung", TerminUVV, day(2020, 5, 31), ErgebnisBestanden, nil, nil, nil, "Datum darf nicht vor der Erstzulassung liegen"},
		{"HU erledigt", TerminHU, today, ErgebnisErledigt, nil, nil, nil, "Ungültiges Prüfergebnis"},
		{"UVV leer", TerminUVV, today, "", nil, nil, nil, "Ungültiges Prüfergebnis"},
		{"Wartung bestanden", TerminWartung, today, ErgebnisBestanden, nil, nil, nil, "Ungültiges Ergebnis (erwartet: erledigt)"},
		{"Versicherung Maengel", TerminVersicherung, today, ErgebnisMaengel, nil, nil, nil, "Ungültiges Ergebnis (erwartet: erledigt)"},
		{"Folgetermin am selben Tag", TerminUVV, today, ErgebnisBestanden, &today, nil, nil, "Folgetermin muss nach dem Datum liegen"},
		{"km negativ", TerminWartung, today, ErgebnisErledigt, nil, intPtr(-1), nil, "Kilometerstand darf nicht negativ sein"},
		{"Folge-km 0", TerminWartung, today, ErgebnisErledigt, nil, nil, intPtr(0), "Kilometerstand muss größer als 0 sein"},
		{"Folge-km gleich km", TerminWartung, today, ErgebnisErledigt, nil, intPtr(30000), intPtr(30000), "Folgetermin nach Kilometern muss über dem Kilometerstand liegen"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateCompletion(tc.art, tc.am, tc.ergebnis, tc.next, tc.km, tc.nkm, ez, today); err == nil || err.Error() != tc.want {
				t.Fatalf("erwartet %q, bekommen %v", tc.want, err)
			}
		})
	}
}

func completion(am time.Time, ergebnis string, next *time.Time, created time.Time) Completion {
	return Completion{ID: uuid.New(), ErledigtAm: am, Ergebnis: ergebnis, NaechsterTermin: next, Status: CompletionActive, CreatedAt: created}
}

func TestEvaluateSchedule(t *testing.T) {
	today := day(2026, 10, 26)
	t0 := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)

	// Ohne Erledigung: Starttermin.
	st := EvaluateSchedule(TerminHU, datePtr(2027, 5, 31), nil, nil, today, 30)
	if st.Status != TerminstandOK || st.FaelligAm == nil || !st.FaelligAm.Equal(day(2027, 5, 31)) || st.LetzteErledigungID != nil {
		t.Fatalf("Starttermin: %+v", st)
	}
	if st := EvaluateSchedule(TerminHU, nil, nil, nil, today, 30); st.Status != TerminstandOhneTermin || st.TageBisFaellig != nil {
		t.Fatalf("ohne Termin: %+v", st)
	}
	// Grenzen der Vorwarnzeit: heute = faellig bald (0 Tage), 30 Tage =
	// faellig bald, 31 = ok, gestern = ueberfaellig.
	for start, want := range map[time.Time]string{
		today:                   TerminstandFaelligBald,
		today.AddDate(0, 0, 30): TerminstandFaelligBald,
		today.AddDate(0, 0, 31): TerminstandOK,
		today.AddDate(0, 0, -1): TerminstandUeberfaellig,
	} {
		s := start
		if got := EvaluateSchedule(TerminUVV, &s, nil, nil, today, 30); got.Status != want {
			t.Errorf("faellig %v: %s, erwartet %s", s, got.Status, want)
		}
	}
	if got := EvaluateSchedule(TerminUVV, datePtr(2026, 10, 30), nil, nil, today, -5); got.Status != TerminstandOK || *got.TageBisFaellig != 4 {
		t.Fatalf("negative Vorwarnzeit wie 0: %+v", got)
	}
	// Heute faellig bei negativer Vorwarnzeit: wie 0 - faellig bald, nicht ok.
	if got := EvaluateSchedule(TerminUVV, &today, nil, nil, today, -5); got.Status != TerminstandFaelligBald {
		t.Fatalf("heute faellig, Vorwarnzeit -5: %+v", got)
	}

	// Die Erledigung loest den Starttermin ab; die juengste zaehlt.
	old := completion(day(2024, 3, 1), ErgebnisBestanden, datePtr(2026, 3, 31), t0)
	recent := completion(day(2026, 3, 10), ErgebnisBestanden, datePtr(2028, 3, 31), t0)
	st = EvaluateSchedule(TerminHU, datePtr(2026, 3, 31), nil, []Completion{recent, old}, today, 30)
	if st.Status != TerminstandOK || !st.FaelligAm.Equal(day(2028, 3, 31)) || *st.LetzteErledigungID != recent.ID || st.LetztesErgebnis != ErgebnisBestanden {
		t.Fatalf("juengste Erledigung: %+v", st)
	}
	// Erledigung ohne Folgetermin: ohne Termin, auch wenn ein Starttermin
	// bestand.
	if st := EvaluateSchedule(TerminLeasing, datePtr(2026, 12, 31), nil, []Completion{completion(day(2026, 10, 1), ErgebnisErledigt, nil, t0)}, today, 30); st.Status != TerminstandOhneTermin {
		t.Fatalf("ohne Folgetermin: %+v", st)
	}

	// Stornierte zaehlt nicht.
	failed := completion(day(2026, 10, 20), ErgebnisNichtBestanden, datePtr(2026, 11, 30), t0)
	failed.Status = CompletionStorniert
	if st := EvaluateSchedule(TerminHU, nil, nil, []Completion{recent, failed}, today, 30); st.Status != TerminstandOK || *st.LetzteErledigungID != recent.ID {
		t.Fatalf("Storno: %+v", st)
	}

	// Sperre: juengste HU/UVV mit Maengeln oder nicht bestanden - die
	// Faelligkeit (Nachpruefung) steht trotzdem dabei.
	for _, ergebnis := range []string{ErgebnisMaengel, ErgebnisNichtBestanden} {
		bad := completion(day(2026, 10, 20), ergebnis, datePtr(2026, 11, 30), t0)
		st := EvaluateSchedule(TerminUVV, nil, nil, []Completion{recent, bad}, today, 30)
		if st.Status != TerminstandGesperrt || st.FaelligAm == nil || !st.FaelligAm.Equal(day(2026, 11, 30)) {
			t.Fatalf("%s: %+v", ergebnis, st)
		}
	}
	// Eine neuere bestandene hebt die Sperre auf.
	bad := completion(day(2026, 10, 1), ErgebnisMaengel, nil, t0)
	fixed := completion(day(2026, 10, 20), ErgebnisBestanden, datePtr(2027, 10, 20), t0)
	if st := EvaluateSchedule(TerminUVV, nil, nil, []Completion{bad, fixed}, today, 30); st.Status != TerminstandOK {
		t.Fatalf("nach Reparatur: %+v", st)
	}
	// Gleiches Datum: die zuletzt erfasste zaehlt.
	sameA := completion(day(2026, 10, 20), ErgebnisMaengel, nil, t0)
	sameB := completion(day(2026, 10, 20), ErgebnisBestanden, datePtr(2027, 10, 20), t0.Add(time.Hour))
	if st := EvaluateSchedule(TerminUVV, nil, nil, []Completion{sameB, sameA}, today, 30); st.Status != TerminstandOK || *st.LetzteErledigungID != sameB.ID {
		t.Fatalf("gleiches Datum: %+v", st)
	}
	// Bei Terminen ohne Pruefergebnis gibt es keine Sperre (auch wenn das
	// Ergebnis faelschlich "maengel" waere).
	if st := EvaluateSchedule(TerminWartung, nil, nil, []Completion{completion(day(2026, 10, 1), ErgebnisMaengel, datePtr(2027, 10, 1), t0)}, today, 30); st.Status != TerminstandOK {
		t.Fatalf("Wartung ohne Sperre: %+v", st)
	}
	// km-Ziel: aus der Erledigung, sonst Start-km.
	if st := EvaluateSchedule(TerminWartung, nil, intPtr(30000), nil, today, 30); st.FaelligKm == nil || *st.FaelligKm != 30000 {
		t.Fatalf("Start-km: %+v", st)
	}
	withKm := completion(day(2026, 10, 1), ErgebnisErledigt, nil, t0)
	withKm.NaechsterKm = intPtr(60000)
	if st := EvaluateSchedule(TerminWartung, nil, intPtr(30000), []Completion{withKm}, today, 30); st.FaelligKm == nil || *st.FaelligKm != 60000 {
		t.Fatalf("Folge-km: %+v", st)
	}
}
