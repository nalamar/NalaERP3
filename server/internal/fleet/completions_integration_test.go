package fleet

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Erledigungen gegen echtes Postgres (Backlog G.2.2.2).

func TestCompletionProposalsAndNormalization(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewScheduleService(env.PG)
	company := seedCompany(t, env)
	v := seedVehicle(t, env, company, "M-C 1")
	actor := testutil.SeedAuthUser(t, env, "compl-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "fleet")

	hu, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminHU}, "", company)
	uvv, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminUVV}, "", company)
	wartung, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminWartung, IntervallKm: intPtr(30000)}, "", company)
	leasing, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminLeasing}, "", company)

	// HU: Vorschlag aus 24 Monaten, zum Monatsende.
	c, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: hu.ID, ErledigtAm: day(2026, 3, 10), Ergebnis: " bestanden ", KmStand: intPtr(42000), Pruefer: " TÜV Süd ", Notiz: " ok "}, actor, company)
	if err != nil {
		t.Fatalf("HU: %v", err)
	}
	if !c.ErledigtAm.Equal(day(2026, 3, 10)) || c.NaechsterTermin == nil || !c.NaechsterTermin.Equal(day(2028, 3, 31)) || c.Ergebnis != ErgebnisBestanden ||
		c.Pruefer != "TÜV Süd" || c.Notiz != "ok" || c.KmStand == nil || *c.KmStand != 42000 || c.NaechsterKm != nil || c.Status != CompletionActive ||
		c.CreatedByUserID == nil || *c.CreatedByUserID != actor {
		t.Fatalf("HU: %+v", c)
	}
	// Angegebener HU-Folgetermin: zum Monatsende.
	c2, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: hu.ID, ErledigtAm: day(2026, 4, 1), Ergebnis: ErgebnisBestanden, NaechsterTermin: datePtr(2027, 6, 2)}, "", company)
	if err != nil || !c2.NaechsterTermin.Equal(day(2027, 6, 30)) {
		t.Fatalf("HU angegeben: %+v %v", c2, err)
	}
	// UVV: angegebener Termin bleibt der Tag.
	c3, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: uvv.ID, ErledigtAm: day(2026, 4, 1), Ergebnis: ErgebnisBestanden, NaechsterTermin: datePtr(2027, 3, 15)}, "", company)
	if err != nil || !c3.NaechsterTermin.Equal(day(2027, 3, 15)) {
		t.Fatalf("UVV angegeben: %+v %v", c3, err)
	}
	// Wartung: Folge-km aus km-Stand + Intervall-km; angegebene bleiben.
	c4, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: wartung.ID, ErledigtAm: day(2026, 4, 1), Ergebnis: ErgebnisErledigt, KmStand: intPtr(31000)}, "", company)
	if err != nil || c4.NaechsterKm == nil || *c4.NaechsterKm != 61000 || c4.NaechsterTermin != nil {
		t.Fatalf("Wartung: %+v %v", c4, err)
	}
	c5, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: wartung.ID, ErledigtAm: day(2026, 5, 1), Ergebnis: ErgebnisErledigt, KmStand: intPtr(35000), NaechsterKm: intPtr(50000)}, "", company)
	if err != nil || *c5.NaechsterKm != 50000 {
		t.Fatalf("Wartung angegeben: %+v %v", c5, err)
	}
	// Ohne km-Stand kein Folge-km-Vorschlag; Leasing ohne Intervall ohne Folgetermin.
	c6, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: wartung.ID, ErledigtAm: day(2026, 5, 2), Ergebnis: ErgebnisErledigt}, "", company)
	if err != nil || c6.NaechsterKm != nil {
		t.Fatalf("Wartung ohne km: %+v %v", c6, err)
	}
	c7, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: leasing.ID, ErledigtAm: day(2026, 5, 2), Ergebnis: ErgebnisErledigt}, "", company)
	if err != nil || c7.NaechsterTermin != nil {
		t.Fatalf("Leasing: %+v %v", c7, err)
	}
	// Nach Maengeln oder nicht bestanden kein Vorschlag aus dem Intervall
	// (es steht eine Nachpruefung an); eine angegebene Frist bleibt.
	for _, ergebnis := range []string{ErgebnisMaengel, ErgebnisNichtBestanden} {
		bad, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: uvv.ID, ErledigtAm: day(2026, 5, 1), Ergebnis: ergebnis}, "", company)
		if err != nil || bad.NaechsterTermin != nil {
			t.Fatalf("%s ohne Vorschlag: %+v %v", ergebnis, bad, err)
		}
	}
	nach, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: hu.ID, ErledigtAm: day(2026, 5, 1), Ergebnis: ErgebnisNichtBestanden, NaechsterTermin: datePtr(2026, 6, 1)}, "", company)
	if err != nil || nach.NaechsterTermin == nil || !nach.NaechsterTermin.Equal(day(2026, 6, 30)) {
		t.Fatalf("Nachpruefungsfrist: %+v %v", nach, err)
	}
	// Ein spaeter geaendertes Intervall aendert die gespeicherten nicht.
	if _, err := svc.UpdateSchedule(ctx, hu.ID, ScheduleUpdate{IntervallMonate: intPtr(12), Aktiv: true}, company); err != nil {
		t.Fatalf("Intervall aendern: %v", err)
	}
	list, err := svc.ListCompletions(ctx, hu.ID, false, company)
	if err != nil || len(list) != 3 || list[0].ID != nach.ID || list[1].ID != c2.ID || !list[2].NaechsterTermin.Equal(day(2028, 3, 31)) {
		t.Fatalf("gespeicherter Folgetermin: %+v %v", list, err)
	}
}

func TestCompletionErrors(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewScheduleService(env.PG)
	vsvc := NewVehicleService(env.PG)
	company := seedCompany(t, env)
	foreignCompany := seedCompany(t, env)
	v, err := vsvc.Create(ctx, VehicleInput{Kennzeichen: "M-F 1", Art: ArtLkw, Erstzulassung: datePtr(2020, 6, 1)}, company, "")
	if err != nil {
		t.Fatalf("Fahrzeug: %v", err)
	}
	hu, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminHU}, "", company)
	wartung, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminWartung}, "", company)
	still, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminVersicherung}, "", company)
	if _, err := svc.UpdateSchedule(ctx, still.ID, ScheduleUpdate{Aktiv: false}, company); err != nil {
		t.Fatalf("stilllegen: %v", err)
	}
	gone := seedVehicle(t, env, company, "M-F 2")
	goneHU, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: gone.ID, Art: TerminHU}, "", company)
	if _, err := vsvc.Deregister(ctx, gone.ID, today(), "verkauft", company); err != nil {
		t.Fatalf("Abmelden: %v", err)
	}
	foreignV := seedVehicle(t, env, foreignCompany, "M-F 1")
	foreignHU, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: foreignV.ID, Art: TerminHU}, "", foreignCompany)
	tomorrow := today().AddDate(0, 0, 1)

	for _, tc := range []struct {
		name    string
		in      CompletionInput
		company string
		want    string
	}{
		{"ohne Mandant", CompletionInput{ScheduleID: hu.ID, ErledigtAm: day(2026, 1, 1), Ergebnis: ErgebnisBestanden}, "", "Mandant erforderlich"},
		{"ohne Termin", CompletionInput{ErledigtAm: day(2026, 1, 1), Ergebnis: ErgebnisBestanden}, company, "schedule_id erforderlich"},
		{"Termin unbekannt", CompletionInput{ScheduleID: uuid.New(), ErledigtAm: day(2026, 1, 1), Ergebnis: ErgebnisBestanden}, company, "Termin nicht gefunden"},
		{"fremder Termin", CompletionInput{ScheduleID: foreignHU.ID, ErledigtAm: day(2026, 1, 1), Ergebnis: ErgebnisBestanden}, company, "Termin nicht gefunden"},
		{"stillgelegt", CompletionInput{ScheduleID: still.ID, ErledigtAm: day(2026, 1, 1), Ergebnis: ErgebnisErledigt}, company, "Termin ist stillgelegt"},
		{"abgemeldet", CompletionInput{ScheduleID: goneHU.ID, ErledigtAm: day(2026, 1, 1), Ergebnis: ErgebnisBestanden}, company, "Fahrzeug ist abgemeldet"},
		{"ohne Datum", CompletionInput{ScheduleID: hu.ID, Ergebnis: ErgebnisBestanden}, company, "Datum erforderlich"},
		{"morgen", CompletionInput{ScheduleID: hu.ID, ErledigtAm: tomorrow, Ergebnis: ErgebnisBestanden}, company, "Datum darf nicht in der Zukunft liegen"},
		{"vor Erstzulassung", CompletionInput{ScheduleID: hu.ID, ErledigtAm: day(2020, 5, 31), Ergebnis: ErgebnisBestanden}, company, "Datum darf nicht vor der Erstzulassung liegen"},
		{"HU erledigt", CompletionInput{ScheduleID: hu.ID, ErledigtAm: day(2026, 1, 1), Ergebnis: ErgebnisErledigt}, company, "Ungültiges Prüfergebnis"},
		{"Wartung bestanden", CompletionInput{ScheduleID: wartung.ID, ErledigtAm: day(2026, 1, 1), Ergebnis: ErgebnisBestanden}, company, "Ungültiges Ergebnis (erwartet: erledigt)"},
		{"HU-Folgetermin im selben Monat", CompletionInput{ScheduleID: hu.ID, ErledigtAm: day(2026, 1, 31), Ergebnis: ErgebnisBestanden, NaechsterTermin: datePtr(2026, 1, 5)}, company, "Folgetermin muss nach dem Datum liegen"},
		{"Folge-km unter km", CompletionInput{ScheduleID: wartung.ID, ErledigtAm: day(2026, 1, 1), Ergebnis: ErgebnisErledigt, KmStand: intPtr(5000), NaechsterKm: intPtr(4000)}, company, "Folgetermin nach Kilometern muss über dem Kilometerstand liegen"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.RecordCompletion(ctx, tc.in, "", tc.company); err == nil || err.Error() != tc.want {
				t.Fatalf("erwartet %q, bekommen %v", tc.want, err)
			}
		})
	}
	var n int
	if err := env.PG.QueryRow(ctx, `SELECT count(*) FROM fleet_vehicle_completions WHERE schedule_id = ANY($1)`, []uuid.UUID{hu.ID, wartung.ID, still.ID, goneHU.ID, foreignHU.ID}).Scan(&n); err != nil || n != 0 {
		t.Fatalf("nach Fehlern %d Erledigungen (%v)", n, err)
	}
}

func TestCompletionStornoAndStanding(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	svc := NewScheduleService(env.PG)
	company := seedCompany(t, env)
	foreignCompany := seedCompany(t, env)
	v := seedVehicle(t, env, company, "M-ST 1")
	actor := testutil.SeedAuthUser(t, env, "storno-"+uuid.NewString()[:8]+"@example.test", "pw-123456789", "fleet")
	now := today()
	hu, _ := svc.CreateSchedule(ctx, ScheduleInput{VehicleID: v.ID, Art: TerminHU}, "", company)
	standing := func() string {
		l, err := svc.ListSchedules(ctx, v.ID, false, now, DefaultScheduleWarnDays, company)
		if err != nil || len(l) != 1 {
			t.Fatalf("Stand: %v %v", l, err)
		}
		return l[0].Standing.Status
	}

	good, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: hu.ID, ErledigtAm: now.AddDate(0, 0, -10), Ergebnis: ErgebnisBestanden}, "", company)
	if err != nil || standing() != TerminstandOK {
		t.Fatalf("bestanden: %v / %s", err, standing())
	}
	bad, err := svc.RecordCompletion(ctx, CompletionInput{ScheduleID: hu.ID, ErledigtAm: now, Ergebnis: ErgebnisMaengel, NaechsterTermin: ptrTime(now.AddDate(0, 1, 0))}, "", company)
	if err != nil || standing() != TerminstandGesperrt {
		t.Fatalf("Maengel sperren: %v / %s", err, standing())
	}

	for name, tc := range map[string]struct {
		id      uuid.UUID
		reason  string
		company string
		want    string
	}{
		"ohne Grund":      {bad.ID, " ", company, "Storno-Begründung erforderlich"},
		"ohne ID":         {uuid.Nil, "x", company, "Erledigungs-ID erforderlich"},
		"unbekannt":       {uuid.New(), "x", company, "Erledigung nicht gefunden"},
		"fremder Mandant": {bad.ID, "x", foreignCompany, "Erledigung nicht gefunden"},
		"ohne Mandant":    {bad.ID, "x", "", "Mandant erforderlich"},
	} {
		if _, err := svc.StornoCompletion(ctx, tc.id, tc.reason, "", tc.company); err == nil || err.Error() != tc.want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if standing() != TerminstandGesperrt {
		t.Fatal("abgelehnter Storno hat gewirkt")
	}
	st, err := svc.StornoCompletion(ctx, bad.ID, " falsches Fahrzeug ", actor, company)
	if err != nil || st.Status != CompletionStorniert || st.StornoReason != "falsches Fahrzeug" || st.StorniertAt == nil || st.StorniertByUserID == nil || *st.StorniertByUserID != actor {
		t.Fatalf("Storno: %+v %v", st, err)
	}
	if standing() != TerminstandOK {
		t.Fatalf("nach Storno wieder die vorherige: %s", standing())
	}
	if _, err := svc.StornoCompletion(ctx, bad.ID, "nochmal", "", company); err == nil || err.Error() != "Erledigung ist bereits storniert" {
		t.Fatalf("zweiter Storno: %v", err)
	}
	if l, _ := svc.ListCompletions(ctx, hu.ID, false, company); len(l) != 1 || l[0].ID != good.ID {
		t.Fatalf("aktive Erledigungen: %+v", l)
	}
	if l, _ := svc.ListCompletions(ctx, hu.ID, true, company); len(l) != 2 || l[0].ID != bad.ID {
		t.Fatalf("mit stornierten, juengste zuerst: %+v", l)
	}
	if _, err := svc.ListCompletions(ctx, hu.ID, true, foreignCompany); err == nil || err.Error() != "Termin nicht gefunden" {
		t.Fatalf("fremde Liste: %v", err)
	}

	// Am abgemeldeten Fahrzeug: keine neue Erledigung, Storno bleibt moeglich.
	if _, err := NewVehicleService(env.PG).Deregister(ctx, v.ID, now, "verkauft", company); err != nil {
		t.Fatalf("Abmelden: %v", err)
	}
	if _, err := svc.StornoCompletion(ctx, good.ID, "Berichtigung", "", company); err != nil {
		t.Fatalf("Storno nach Abmeldung: %v", err)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
