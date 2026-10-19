package hr

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

// Teams anlegen und pflegen (Backlog F.8.5).

func TestTeamsLifecycle(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	company := seedWorkdaysCompany(t, env)
	other := seedWorkdaysCompany(t, env)
	svc := NewEmployeeService(env.PG)
	lead, err := svc.Create(ctx, EmployeeInput{FirstName: "Lea", LastName: "Der"}, company)
	if err != nil {
		t.Fatal(err)
	}
	foreignLead, err := svc.Create(ctx, EmployeeInput{FirstName: "Fr", LastName: "Emd"}, other)
	if err != nil {
		t.Fatal(err)
	}

	team, err := svc.CreateTeam(ctx, TeamInput{Name: "  Montage  ", Lead: &lead.ID}, company)
	if err != nil || team.Name != "Montage" || team.Lead == nil || *team.Lead != lead.ID || team.Mitglieder != 0 {
		t.Fatalf("anlegen: %+v, %v", team, err)
	}
	_, err = svc.CreateTeam(ctx, TeamInput{Name: "montage"}, company)
	wantEmployeeError(t, err, "Team mit diesem Namen existiert bereits")
	_, err = svc.CreateTeam(ctx, TeamInput{Name: "  "}, company)
	wantEmployeeError(t, err, "Teamname erforderlich")
	_, err = svc.CreateTeam(ctx, TeamInput{Name: "Lager", Lead: &foreignLead.ID}, company)
	wantEmployeeError(t, err, "Mitarbeiter nicht gefunden")
	// Derselbe Name im anderen Mandanten ist frei.
	if _, err := svc.CreateTeam(ctx, TeamInput{Name: "Montage"}, other); err != nil {
		t.Fatalf("anderer Mandant: %v", err)
	}

	buero, err := svc.CreateTeam(ctx, TeamInput{Name: "Büro"}, company)
	if err != nil {
		t.Fatal(err)
	}
	// Umbenennen: der eigene Name (anders geschrieben) ist erlaubt, ein
	// fremder nicht; Teamleiter entfernen.
	upd, err := svc.UpdateTeam(ctx, team.ID, TeamInput{Name: "MONTAGE"}, company)
	if err != nil || upd.Name != "MONTAGE" || upd.Lead != nil {
		t.Fatalf("aendern: %+v, %v", upd, err)
	}
	_, err = svc.UpdateTeam(ctx, team.ID, TeamInput{Name: "büro"}, company)
	wantEmployeeError(t, err, "Team mit diesem Namen existiert bereits")
	_, err = svc.UpdateTeam(ctx, team.ID, TeamInput{Name: "X"}, other)
	wantEmployeeError(t, err, "Team nicht gefunden")
	_, err = svc.UpdateTeam(ctx, uuid.New(), TeamInput{Name: "X"}, company)
	wantEmployeeError(t, err, "Team nicht gefunden")
	if got, _ := svc.GetTeam(ctx, team.ID, company); got.Name != "MONTAGE" {
		t.Fatalf("abgelehnte Aenderung hat gespeichert: %+v", got)
	}

	// Mitglieder zaehlen; mit Mitgliedern nicht loeschbar.
	member, err := svc.Create(ctx, EmployeeInput{FirstName: "Mit", LastName: "Glied", TeamID: &team.ID}, company)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := svc.GetTeam(ctx, team.ID, company); err != nil || got.Mitglieder != 1 {
		t.Fatalf("Mitglieder im Einzelabruf: %+v, %v", got, err)
	}
	teams, err := svc.ListTeams(ctx, company)
	if err != nil || len(teams) != 2 || teams[0].Name != "Büro" || teams[1].Mitglieder != 1 || teams[0].Mitglieder != 0 {
		t.Fatalf("Liste: %+v, %v", teams, err)
	}
	wantEmployeeError(t, svc.DeleteTeam(ctx, team.ID, company), "Team hat noch Mitglieder - zuerst umsetzen")
	wantEmployeeError(t, svc.DeleteTeam(ctx, buero.ID, other), "Team nicht gefunden")
	if err := svc.Update(ctx, member.ID, map[string]any{"team_id": buero.ID.String()}, company); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTeam(ctx, team.ID, company); err != nil {
		t.Fatalf("loeschen: %v", err)
	}
	wantEmployeeError(t, svc.DeleteTeam(ctx, team.ID, company), "Team nicht gefunden")
	if teams, _ := svc.ListTeams(ctx, company); len(teams) != 1 || teams[0].ID != buero.ID || teams[0].Mitglieder != 1 {
		t.Fatalf("nach Loeschen: %+v", teams)
	}
}
