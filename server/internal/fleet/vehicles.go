package fleet

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nalaerp3/internal/hr"
)

// Fahrzeugstamm des Fuhrparks (ADR 0032, Backlog G.1.2): zulassungs-
// pflichtige Fahrzeuge und Anhaenger je Mandant. Maschinen ohne Zulassung
// (Stapler, Hubarbeitsbuehnen) sind Betriebsmittel (internal/assets).
//
// Ein Fahrzeug wird nie geloescht, sondern abgemeldet. Einen gespeicherten
// Status gibt es nicht (Migration 099).

const (
	ArtPkw         = "pkw"
	ArtTransporter = "transporter"
	ArtLkw         = "lkw"
	ArtAnhaenger   = "anhaenger"
)

var arten = map[string]bool{ArtPkw: true, ArtTransporter: true, ArtLkw: true, ArtAnhaenger: true}

// Ein Kennzeichen besteht aus Unterscheidungszeichen, Buchstaben, Ziffern
// und ggf. E/H; geschrieben mit Bindestrich und Leerzeichen ("M-AB 1234",
// "M AB 1234 E"). Bewusst nur der Zeichensatz und die Laenge - die Regeln
// der Zulassungsstellen nachzubauen waere fehleranfaellig.
var kennzeichenPattern = regexp.MustCompile(`^[A-ZÄÖÜ0-9 -]{2,15}$`)

// FIN nach ISO 3779: 17 Zeichen, Ziffern und Grossbuchstaben ohne I, O, Q.
var finPattern = regexp.MustCompile(`^[A-HJ-NPR-Z0-9]{17}$`)

// Namen aus Migration 099 (Uebersetzung statt Vorabpruefung, wie assets).
const (
	kennzeichenIndex = "idx_fleet_vehicles_company_kennzeichen_aktiv"
	finConstraint    = "fleet_vehicles_company_fin_key"
)

func isUniqueViolation(err error, name string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == name
}

type Vehicle struct {
	ID                      uuid.UUID  `json:"id"`
	Kennzeichen             string     `json:"kennzeichen"`
	Art                     string     `json:"art"`
	Bezeichnung             string     `json:"bezeichnung"`
	Hersteller              string     `json:"hersteller"`
	Modell                  string     `json:"modell"`
	FIN                     *string    `json:"fin"`
	Erstzulassung           *time.Time `json:"erstzulassung"`
	ZulaessigeGesamtmasseKg *int       `json:"zulaessige_gesamtmasse_kg"`
	Standort                string     `json:"standort"`
	KostenstelleID          *string    `json:"kostenstelle_id"`
	// Fahrtenbuch: fuer dieses Fahrzeug wird ein Fahrtenbuch gefuehrt
	// (G.3c; optional je Fahrzeug, Entscheidung des Nutzers 2026-10-25).
	Fahrtenbuch     bool       `json:"fahrtenbuch"`
	Notiz           string     `json:"notiz"`
	AbgemeldetAm    *time.Time `json:"abgemeldet_am"`
	AbgemeldetGrund string     `json:"abgemeldet_grund"`
	CreatedAt       time.Time  `json:"created_at"`
}

// VehicleInput sind die Stammdaten - beim Anlegen und beim Aendern.
// Aendern ist ein VOLLSTAENDIGER Ersatz (wie assets.AssetInput); die
// Abmeldung hat einen eigenen Aufruf.
type VehicleInput struct {
	Kennzeichen             string     `json:"kennzeichen"`
	Art                     string     `json:"art"`
	Bezeichnung             string     `json:"bezeichnung"`
	Hersteller              string     `json:"hersteller"`
	Modell                  string     `json:"modell"`
	FIN                     *string    `json:"fin"`
	Erstzulassung           *time.Time `json:"erstzulassung"`
	ZulaessigeGesamtmasseKg *int       `json:"zulaessige_gesamtmasse_kg"`
	Standort                string     `json:"standort"`
	KostenstelleID          *string    `json:"kostenstelle_id"`
	Fahrtenbuch             bool       `json:"fahrtenbuch"`
	Notiz                   string     `json:"notiz"`
}

// ListFilter schraenkt die Liste ein. Leere Art = alle.
type ListFilter struct {
	Art                string
	IncludeAbgemeldete bool
}

type VehicleService struct{ pg *pgxpool.Pool }

func NewVehicleService(pg *pgxpool.Pool) *VehicleService { return &VehicleService{pg: pg} }

const vehicleColumns = `id, kennzeichen, art, bezeichnung, hersteller, modell, fin, erstzulassung, zulaessige_gesamtmasse_kg,
	standort, kostenstelle_id, fahrtenbuch, notiz, abgemeldet_am, abgemeldet_grund, created_at`

func scanVehicle(row pgx.Row) (*Vehicle, error) {
	var v Vehicle
	if err := row.Scan(&v.ID, &v.Kennzeichen, &v.Art, &v.Bezeichnung, &v.Hersteller, &v.Modell, &v.FIN, &v.Erstzulassung, &v.ZulaessigeGesamtmasseKg,
		&v.Standort, &v.KostenstelleID, &v.Fahrtenbuch, &v.Notiz, &v.AbgemeldetAm, &v.AbgemeldetGrund, &v.CreatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// NormalizeKennzeichen: Grossbuchstaben, ohne Rand-Leerzeichen, innen
// einfache Leerzeichen, Leerzeichen um den Bindestrich entfernt
// ("m - ab  1234" -> "M-AB 1234").
func NormalizeKennzeichen(k string) string {
	k = strings.ToUpper(strings.Join(strings.Fields(k), " "))
	k = strings.ReplaceAll(k, " -", "-")
	return strings.ReplaceAll(k, "- ", "-")
}

func normalizeVehicleInput(in VehicleInput) VehicleInput {
	in.Kennzeichen = NormalizeKennzeichen(in.Kennzeichen)
	in.Art = strings.TrimSpace(in.Art)
	in.Bezeichnung = strings.TrimSpace(in.Bezeichnung)
	in.Hersteller = strings.TrimSpace(in.Hersteller)
	in.Modell = strings.TrimSpace(in.Modell)
	in.Standort = strings.TrimSpace(in.Standort)
	if in.FIN != nil {
		f := strings.ToUpper(strings.TrimSpace(*in.FIN))
		if f == "" {
			in.FIN = nil
		} else {
			in.FIN = &f
		}
	}
	if in.KostenstelleID != nil {
		k := strings.TrimSpace(*in.KostenstelleID)
		if k == "" {
			in.KostenstelleID = nil
		} else {
			in.KostenstelleID = &k
		}
	}
	if in.Erstzulassung != nil {
		d := dateOnly(*in.Erstzulassung)
		in.Erstzulassung = &d
	}
	return in
}

// ValidateVehicleInput prueft eine normalisierte Eingabe ohne Datenbank.
func ValidateVehicleInput(in VehicleInput, today time.Time) error {
	if in.Kennzeichen == "" {
		return errors.New("Kennzeichen erforderlich")
	}
	if !kennzeichenPattern.MatchString(in.Kennzeichen) {
		return errors.New("Ungültiges Kennzeichen (Buchstaben, Ziffern, Bindestrich und Leerzeichen, höchstens 15 Zeichen)")
	}
	if !arten[in.Art] {
		return errors.New("Ungültige Fahrzeugart")
	}
	if in.FIN != nil && !finPattern.MatchString(*in.FIN) {
		return errors.New("Ungültige FIN (17 Zeichen, Ziffern und Buchstaben ohne I, O, Q)")
	}
	if in.Erstzulassung != nil && in.Erstzulassung.After(dateOnly(today)) {
		return errors.New("Erstzulassung darf nicht in der Zukunft liegen")
	}
	if in.ZulaessigeGesamtmasseKg != nil && *in.ZulaessigeGesamtmasseKg <= 0 {
		return errors.New("Zulässige Gesamtmasse muss größer als 0 sein")
	}
	return nil
}

// checkKostenstelle: eine angegebene Kostenstelle muss zum Mandanten
// gehoeren (wie assets.checkKostenstelle).
func checkKostenstelle(ctx context.Context, tx pgx.Tx, kostenstelleID *string, companyID string) error {
	if kostenstelleID == nil {
		return nil
	}
	var owned bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cost_centers WHERE id=$1 AND company_id=$2)`, *kostenstelleID, companyID).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return errors.New("Kostenstelle nicht gefunden")
	}
	return nil
}

func vehicleUniqueError(err error) error {
	switch {
	case isUniqueViolation(err, kennzeichenIndex):
		return errors.New("Fahrzeug mit diesem Kennzeichen existiert bereits")
	case isUniqueViolation(err, finConstraint):
		return errors.New("Fahrzeug mit dieser FIN existiert bereits")
	}
	return err
}

func (s *VehicleService) Create(ctx context.Context, in VehicleInput, companyID, actorUserID string) (*Vehicle, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	in = normalizeVehicleInput(in)
	if err := ValidateVehicleInput(in, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := checkKostenstelle(ctx, tx, in.KostenstelleID, companyID); err != nil {
		return nil, err
	}
	v, err := scanVehicle(tx.QueryRow(ctx, `
		INSERT INTO fleet_vehicles (id, company_id, kennzeichen, art, bezeichnung, hersteller, modell, fin, erstzulassung,
			zulaessige_gesamtmasse_kg, standort, kostenstelle_id, fahrtenbuch, notiz, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NULLIF($15,''))
		RETURNING `+vehicleColumns,
		uuid.New(), companyID, in.Kennzeichen, in.Art, in.Bezeichnung, in.Hersteller, in.Modell, in.FIN, in.Erstzulassung,
		in.ZulaessigeGesamtmasseKg, in.Standort, in.KostenstelleID, in.Fahrtenbuch, in.Notiz, actorUserID))
	if err != nil {
		return nil, vehicleUniqueError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *VehicleService) Get(ctx context.Context, id uuid.UUID, companyID string) (*Vehicle, error) {
	v, err := scanVehicle(s.pg.QueryRow(ctx, `SELECT `+vehicleColumns+` FROM fleet_vehicles WHERE id=$1 AND company_id=$2`, id, companyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Fahrzeug nicht gefunden")
		}
		return nil, err
	}
	return v, nil
}

// List liefert die Fahrzeuge des Mandanten nach Kennzeichen; abgemeldete
// nur auf Wunsch.
func (s *VehicleService) List(ctx context.Context, companyID string, f ListFilter) ([]Vehicle, error) {
	art := strings.TrimSpace(f.Art)
	if art != "" && !arten[art] {
		return nil, errors.New("Ungültige Fahrzeugart")
	}
	rows, err := s.pg.Query(ctx, `SELECT `+vehicleColumns+` FROM fleet_vehicles
		WHERE company_id=$1 AND ($2 = '' OR art = $2) AND (abgemeldet_am IS NULL OR $3)
		ORDER BY kennzeichen, id`, companyID, art, f.IncludeAbgemeldete)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Vehicle, 0)
	for rows.Next() {
		v, err := scanVehicle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// Update ersetzt die Stammdaten. Auch ein abgemeldetes Fahrzeug darf
// berichtigt werden; die Abmeldung bleibt davon unberuehrt. Ein neues
// Kennzeichen (Umkennzeichnung) ist erlaubt, solange es kein anderes
// nicht abgemeldetes Fahrzeug traegt.
func (s *VehicleService) Update(ctx context.Context, id uuid.UUID, in VehicleInput, companyID string) (*Vehicle, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	in = normalizeVehicleInput(in)
	if err := ValidateVehicleInput(in, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := checkKostenstelle(ctx, tx, in.KostenstelleID, companyID); err != nil {
		return nil, err
	}
	// Erstzulassung nach einer schon erfassten Abmeldung waere ein
	// Widerspruch.
	var abgemeldetAm *time.Time
	if err := tx.QueryRow(ctx, `SELECT abgemeldet_am FROM fleet_vehicles WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).Scan(&abgemeldetAm); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Fahrzeug nicht gefunden")
		}
		return nil, err
	}
	if abgemeldetAm != nil && in.Erstzulassung != nil && in.Erstzulassung.After(*abgemeldetAm) {
		return nil, errors.New("Erstzulassung darf nicht nach der Abmeldung liegen")
	}
	v, err := scanVehicle(tx.QueryRow(ctx, `
		UPDATE fleet_vehicles
		   SET kennzeichen=$3, art=$4, bezeichnung=$5, hersteller=$6, modell=$7, fin=$8, erstzulassung=$9,
		       zulaessige_gesamtmasse_kg=$10, standort=$11, kostenstelle_id=$12, fahrtenbuch=$13, notiz=$14
		 WHERE id=$1 AND company_id=$2
		RETURNING `+vehicleColumns,
		id, companyID, in.Kennzeichen, in.Art, in.Bezeichnung, in.Hersteller, in.Modell, in.FIN, in.Erstzulassung,
		in.ZulaessigeGesamtmasseKg, in.Standort, in.KostenstelleID, in.Fahrtenbuch, in.Notiz))
	if err != nil {
		return nil, vehicleUniqueError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return v, nil
}

// ValidateDeregistration prueft eine Abmeldung ohne Datenbank.
func ValidateDeregistration(datum time.Time, grund string, erstzulassung *time.Time, today time.Time) error {
	if datum.IsZero() {
		return errors.New("Abmeldedatum erforderlich")
	}
	if strings.TrimSpace(grund) == "" {
		return errors.New("Grund der Abmeldung erforderlich")
	}
	// Abgemeldet wird, was abgemeldet IST - kein Vormerken.
	if dateOnly(datum).After(dateOnly(today)) {
		return errors.New("Abmeldedatum darf nicht in der Zukunft liegen")
	}
	if erstzulassung != nil && dateOnly(datum).Before(dateOnly(*erstzulassung)) {
		return errors.New("Abmeldedatum darf nicht vor der Erstzulassung liegen")
	}
	return nil
}

// Deregister meldet ein Fahrzeug ab. Endgueltig: ein zweiter Aufruf wird
// abgelehnt. Ein irrtuemlich abgemeldetes Fahrzeug wird neu angelegt - das
// Kennzeichen ist nach der Abmeldung wieder frei.
func (s *VehicleService) Deregister(ctx context.Context, id uuid.UUID, datum time.Time, grund, companyID string) (*Vehicle, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	grund = strings.TrimSpace(grund)
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var abgemeldetAm, erstzulassung *time.Time
	if err := tx.QueryRow(ctx, `SELECT abgemeldet_am, erstzulassung FROM fleet_vehicles WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).
		Scan(&abgemeldetAm, &erstzulassung); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Fahrzeug nicht gefunden")
		}
		return nil, err
	}
	if abgemeldetAm != nil {
		return nil, errors.New("Fahrzeug ist bereits abgemeldet")
	}
	if err := ValidateDeregistration(datum, grund, erstzulassung, hr.WorkDateOf(time.Now())); err != nil {
		return nil, err
	}
	// Fahrerzuordnung (G.3a): erst beenden, dann abmelden - wie beim
	// Ausmustern von Betriebsmitteln. Die Fahrzeugzeile ist gesperrt; Assign
	// nimmt dieselbe Sperre.
	var offen, spaeter bool
	if err := tx.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM fleet_vehicle_assignments WHERE vehicle_id=$1 AND status='active' AND ende IS NULL),
		EXISTS(SELECT 1 FROM fleet_vehicle_assignments WHERE vehicle_id=$1 AND status='active' AND ende > $2)`, id, dateOnly(datum)).Scan(&offen, &spaeter); err != nil {
		return nil, err
	}
	if offen {
		return nil, errors.New("Fahrzeug ist noch zugeordnet - erst die Zuordnung beenden")
	}
	if spaeter {
		return nil, errors.New("Abmeldedatum darf nicht vor dem Ende einer Zuordnung liegen")
	}
	v, err := scanVehicle(tx.QueryRow(ctx, `UPDATE fleet_vehicles SET abgemeldet_am=$3, abgemeldet_grund=$4 WHERE id=$1 AND company_id=$2 RETURNING `+vehicleColumns,
		id, companyID, dateOnly(datum), grund))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return v, nil
}
