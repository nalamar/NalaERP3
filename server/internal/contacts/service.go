package contacts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
)

type Service struct {
	pg      *pgxpool.Pool
	mg      *mongo.Client
	mongoDB string
}

func NewService(pg *pgxpool.Pool) *Service { return &Service{pg: pg} }

func (s *Service) WithMongo(mg *mongo.Client, mongoDB string) *Service {
	s.mg = mg
	s.mongoDB = mongoDB
	return s
}

// Enumerationen (fest)
func Roles() []string        { return []string{"customer", "supplier", "partner", "both", "other"} }
func Types() []string        { return []string{"org", "person"} }
func Statuses() []string     { return []string{"lead", "active", "inactive", "blocked"} }
func AddressKinds() []string { return []string{"billing", "shipping", "other"} }
func TaskStatuses() []string { return []string{"open", "in_progress", "done", "canceled"} }
func PersonRoles() []string {
	return []string{"management", "purchasing", "sales", "accounting", "project", "technical", "other"}
}
func CommunicationChannels() []string {
	return []string{"email", "phone", "mobile", "whatsapp", "teams", "other"}
}

func isIn(v string, list []string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, x := range list {
		if v == x {
			return true
		}
	}
	return false
}

func normalizeOptional(value string) string {
	return strings.TrimSpace(value)
}

func normalizeLowerOptional(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}

func normalizeStatusAndActive(status *string, aktiv *bool) (string, bool) {
	currentStatus := "active"
	currentAktiv := true
	if status != nil && strings.TrimSpace(*status) != "" {
		currentStatus = strings.ToLower(strings.TrimSpace(*status))
	}
	if aktiv != nil {
		currentAktiv = *aktiv
	} else {
		currentAktiv = currentStatus != "inactive" && currentStatus != "blocked"
	}

	if !currentAktiv {
		if currentStatus == "active" {
			currentStatus = "inactive"
		}
		if currentStatus == "lead" {
			currentStatus = "inactive"
		}
	}

	if currentAktiv && (currentStatus == "inactive" || currentStatus == "blocked") {
		currentAktiv = false
	}

	return currentStatus, currentAktiv
}

func (s *Service) ensureNoDuplicate(ctx context.Context, excludeID, name, email, vatID, taxNo, debtorNo, creditorNo string) error {
	if s.pg == nil {
		return nil
	}

	normalizedVAT := normalizeLowerOptional(vatID)
	if normalizedVAT != "" {
		var existingID string
		err := s.pg.QueryRow(ctx, `
            SELECT id
            FROM contacts
            WHERE lower(btrim(vat_id)) = $1
              AND ($2 = '' OR id <> $2)
            LIMIT 1
        `, normalizedVAT, excludeID).Scan(&existingID)
		if err == nil {
			return errors.New("Kontakt mit gleicher USt-IdNr. bereits vorhanden")
		}
	}

	normalizedTaxNo := normalizeLowerOptional(taxNo)
	if normalizedTaxNo != "" {
		var existingID string
		err := s.pg.QueryRow(ctx, `
			SELECT id
			FROM contacts
			WHERE lower(btrim(tax_no)) = $1
			  AND ($2 = '' OR id <> $2)
			LIMIT 1
		`, normalizedTaxNo, excludeID).Scan(&existingID)
		if err == nil {
			return errors.New("Kontakt mit gleicher Steuernummer bereits vorhanden")
		}
	}

	normalizedName := normalizeLowerOptional(name)
	normalizedEmail := normalizeLowerOptional(email)
	if normalizedName != "" && normalizedEmail != "" {
		var existingID string
		err := s.pg.QueryRow(ctx, `
            SELECT id
            FROM contacts
            WHERE lower(btrim(name)) = $1
              AND lower(btrim(email)) = $2
              AND ($3 = '' OR id <> $3)
            LIMIT 1
        `, normalizedName, normalizedEmail, excludeID).Scan(&existingID)
		if err == nil {
			return errors.New("Kontakt mit gleichem Namen und gleicher E-Mail bereits vorhanden")
		}
	}

	normalizedDebtorNo := normalizeLowerOptional(debtorNo)
	if normalizedDebtorNo != "" {
		var existingID string
		err := s.pg.QueryRow(ctx, `
            SELECT id
            FROM contacts
            WHERE lower(btrim(debtor_no)) = $1
              AND ($2 = '' OR id <> $2)
            LIMIT 1
        `, normalizedDebtorNo, excludeID).Scan(&existingID)
		if err == nil {
			return errors.New("Kontakt mit gleicher Debitor-Nr. bereits vorhanden")
		}
	}

	normalizedCreditorNo := normalizeLowerOptional(creditorNo)
	if normalizedCreditorNo != "" {
		var existingID string
		err := s.pg.QueryRow(ctx, `
            SELECT id
            FROM contacts
            WHERE lower(btrim(creditor_no)) = $1
              AND ($2 = '' OR id <> $2)
            LIMIT 1
        `, normalizedCreditorNo, excludeID).Scan(&existingID)
		if err == nil {
			return errors.New("Kontakt mit gleicher Kreditor-Nr. bereits vorhanden")
		}
	}

	return nil
}

// Core types
type Contact struct {
	ID                  string    `json:"id"`
	Typ                 string    `json:"typ"`    // org | person
	Rolle               string    `json:"rolle"`  // customer | supplier | partner | both | other
	Status              string    `json:"status"` // lead | active | inactive | blocked
	Name                string    `json:"name"`
	Email               string    `json:"email"`
	Telefon             string    `json:"telefon"`
	UStID               string    `json:"ust_id"`
	SteuerNr            string    `json:"steuernummer"`
	Waehrung            string    `json:"waehrung"`
	Zahlungsbedingungen string    `json:"zahlungsbedingungen"`
	DebitorNr           string    `json:"debitor_nr"`
	KreditorNr          string    `json:"kreditor_nr"`
	SteuerLand          string    `json:"steuer_land"`
	Steuerbefreit       bool      `json:"steuerbefreit"`
	Aktiv               bool      `json:"aktiv"`
	Angelegt            time.Time `json:"angelegt_am"`
}

type ContactCreate struct {
	Typ                 string `json:"typ"`
	Rolle               string `json:"rolle"`
	Status              string `json:"status"`
	Name                string `json:"name"`
	Email               string `json:"email"`
	Telefon             string `json:"telefon"`
	UStID               string `json:"ust_id"`
	SteuerNr            string `json:"steuernummer"`
	Waehrung            string `json:"waehrung"`
	Zahlungsbedingungen string `json:"zahlungsbedingungen"`
	DebitorNr           string `json:"debitor_nr"`
	KreditorNr          string `json:"kreditor_nr"`
	SteuerLand          string `json:"steuer_land"`
	Steuerbefreit       bool   `json:"steuerbefreit"`
}

type ContactUpdate struct {
	Typ                 *string `json:"typ"`
	Rolle               *string `json:"rolle"`
	Status              *string `json:"status"`
	Name                *string `json:"name"`
	Email               *string `json:"email"`
	Telefon             *string `json:"telefon"`
	UStID               *string `json:"ust_id"`
	SteuerNr            *string `json:"steuernummer"`
	Waehrung            *string `json:"waehrung"`
	Zahlungsbedingungen *string `json:"zahlungsbedingungen"`
	DebitorNr           *string `json:"debitor_nr"`
	KreditorNr          *string `json:"kreditor_nr"`
	SteuerLand          *string `json:"steuer_land"`
	Steuerbefreit       *bool   `json:"steuerbefreit"`
	Aktiv               *bool   `json:"aktiv"`
}

type ContactFilter struct {
	Q      string
	Rolle  string
	Status string
	Typ    string
	Limit  int
	Offset int
}

func (s *Service) Create(ctx context.Context, in ContactCreate) (*Contact, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("Name erforderlich")
	}
	if in.Typ == "" {
		in.Typ = "org"
	}
	if !isIn(in.Typ, Types()) {
		return nil, errors.New("Ungültiger Typ")
	}
	if in.Rolle == "" {
		in.Rolle = "other"
	}
	if !isIn(in.Rolle, Roles()) {
		return nil, errors.New("Ungültige Rolle")
	}
	status, aktiv := normalizeStatusAndActive(&in.Status, nil)
	if !isIn(status, Statuses()) {
		return nil, errors.New("Ungültiger Status")
	}
	in.Name = normalizeOptional(in.Name)
	in.Email = normalizeOptional(in.Email)
	in.Telefon = normalizeOptional(in.Telefon)
	in.UStID = normalizeOptional(in.UStID)
	in.SteuerNr = normalizeOptional(in.SteuerNr)
	in.Zahlungsbedingungen = normalizeOptional(in.Zahlungsbedingungen)
	in.DebitorNr = normalizeOptional(in.DebitorNr)
	in.KreditorNr = normalizeOptional(in.KreditorNr)
	if in.Waehrung == "" {
		in.Waehrung = "EUR"
	}
	if strings.TrimSpace(in.SteuerLand) == "" {
		in.SteuerLand = "DE"
	}
	in.SteuerLand = strings.ToUpper(strings.TrimSpace(in.SteuerLand))
	if err := s.ensureNoDuplicate(ctx, "", in.Name, in.Email, in.UStID, in.SteuerNr, in.DebitorNr, in.KreditorNr); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	var c Contact
	err := s.pg.QueryRow(ctx, `
        INSERT INTO contacts (id, typ, rolle, status, name, email, phone, vat_id, tax_no, waehrung, payment_terms, debtor_no, creditor_no, tax_country, tax_exempt, aktiv)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
        RETURNING id, typ, rolle, status, name, COALESCE(email,''), COALESCE(phone,''), COALESCE(vat_id,''), COALESCE(tax_no,''), waehrung, COALESCE(payment_terms,''), COALESCE(debtor_no,''), COALESCE(creditor_no,''), COALESCE(tax_country,'DE'), tax_exempt, aktiv, angelegt_am
    `, id, in.Typ, in.Rolle, status, in.Name, in.Email, in.Telefon, in.UStID, in.SteuerNr, in.Waehrung, in.Zahlungsbedingungen, in.DebitorNr, in.KreditorNr, in.SteuerLand, in.Steuerbefreit, aktiv).Scan(
		&c.ID, &c.Typ, &c.Rolle, &c.Status, &c.Name, &c.Email, &c.Telefon, &c.UStID, &c.SteuerNr, &c.Waehrung, &c.Zahlungsbedingungen, &c.DebitorNr, &c.KreditorNr, &c.SteuerLand, &c.Steuerbefreit, &c.Aktiv, &c.Angelegt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Contact, error) {
	var c Contact
	err := s.pg.QueryRow(ctx, `SELECT id, typ, rolle, status, name, COALESCE(email,''), COALESCE(phone,''), COALESCE(vat_id,''), COALESCE(tax_no,''), waehrung, COALESCE(payment_terms,''), COALESCE(debtor_no,''), COALESCE(creditor_no,''), COALESCE(tax_country,'DE'), tax_exempt, aktiv, angelegt_am FROM contacts WHERE id=$1`, id).Scan(
		&c.ID, &c.Typ, &c.Rolle, &c.Status, &c.Name, &c.Email, &c.Telefon, &c.UStID, &c.SteuerNr, &c.Waehrung, &c.Zahlungsbedingungen, &c.DebitorNr, &c.KreditorNr, &c.SteuerLand, &c.Steuerbefreit, &c.Aktiv, &c.Angelegt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) Update(ctx context.Context, id string, u ContactUpdate) (*Contact, error) {
	if u.Typ != nil {
		if !isIn(*u.Typ, Types()) {
			return nil, errors.New("Ungültiger Typ")
		}
	}
	if u.Rolle != nil {
		if !isIn(*u.Rolle, Roles()) {
			return nil, errors.New("Ungültige Rolle")
		}
	}
	if u.Status != nil {
		normalized, _ := normalizeStatusAndActive(u.Status, u.Aktiv)
		if !isIn(normalized, Statuses()) {
			return nil, errors.New("Ungültiger Status")
		}
	}

	current, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// build SET dynamically
	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(field string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", field, idx))
		args = append(args, v)
		idx++
	}
	effectiveName := current.Name
	effectiveEmail := current.Email
	effectiveVAT := current.UStID
	effectiveTaxNo := current.SteuerNr
	effectiveDebtorNo := current.DebitorNr
	effectiveCreditorNo := current.KreditorNr
	if u.Typ != nil {
		add("typ", *u.Typ)
	}
	if u.Rolle != nil {
		add("rolle", *u.Rolle)
	}
	statusValue := u.Status
	aktivValue := u.Aktiv
	if statusValue != nil {
		normalized, normalizedAktiv := normalizeStatusAndActive(statusValue, aktivValue)
		add("status", normalized)
		add("aktiv", normalizedAktiv)
		aktivValue = nil
	}
	if u.Name != nil {
		effectiveName = normalizeOptional(*u.Name)
		add("name", effectiveName)
	}
	if u.Email != nil {
		effectiveEmail = normalizeOptional(*u.Email)
		add("email", effectiveEmail)
	}
	if u.Telefon != nil {
		add("phone", *u.Telefon)
	}
	if u.UStID != nil {
		effectiveVAT = normalizeOptional(*u.UStID)
		add("vat_id", effectiveVAT)
	}
	if u.SteuerNr != nil {
		effectiveTaxNo = normalizeOptional(*u.SteuerNr)
		add("tax_no", effectiveTaxNo)
	}
	if u.Waehrung != nil {
		add("waehrung", *u.Waehrung)
	}
	if u.Zahlungsbedingungen != nil {
		add("payment_terms", *u.Zahlungsbedingungen)
	}
	if u.DebitorNr != nil {
		effectiveDebtorNo = normalizeOptional(*u.DebitorNr)
		add("debtor_no", effectiveDebtorNo)
	}
	if u.KreditorNr != nil {
		effectiveCreditorNo = normalizeOptional(*u.KreditorNr)
		add("creditor_no", effectiveCreditorNo)
	}
	if u.SteuerLand != nil {
		add("tax_country", strings.ToUpper(strings.TrimSpace(*u.SteuerLand)))
	}
	if u.Steuerbefreit != nil {
		add("tax_exempt", *u.Steuerbefreit)
	}
	if aktivValue != nil {
		normalizedStatus, normalizedAktiv := normalizeStatusAndActive(nil, aktivValue)
		add("aktiv", normalizedAktiv)
		add("status", normalizedStatus)
	}
	if err := s.ensureNoDuplicate(ctx, id, effectiveName, effectiveEmail, effectiveVAT, effectiveTaxNo, effectiveDebtorNo, effectiveCreditorNo); err != nil {
		return nil, err
	}
	if len(sets) == 0 {
		return s.Get(ctx, id)
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE contacts SET %s WHERE id=$%d", strings.Join(sets, ", "), idx)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) DeleteSoft(ctx context.Context, id string) error {
	_, err := s.pg.Exec(ctx, `UPDATE contacts SET aktiv=false, status='inactive' WHERE id=$1`, id)
	return err
}

func (s *Service) List(ctx context.Context, f ContactFilter) ([]Contact, error) {
	lim := f.Limit
	if lim <= 0 || lim > 200 {
		lim = 50
	}
	off := f.Offset
	sb := strings.Builder{}
	sb.WriteString(`SELECT id, typ, rolle, status, name, COALESCE(email,''), COALESCE(phone,''), COALESCE(vat_id,''), COALESCE(tax_no,''), waehrung, COALESCE(payment_terms,''), COALESCE(debtor_no,''), COALESCE(creditor_no,''), COALESCE(tax_country,'DE'), tax_exempt, aktiv, angelegt_am FROM contacts`)
	var conds []string
	var args []any
	idx := 1
	if strings.TrimSpace(f.Q) != "" {
		conds = append(conds, fmt.Sprintf("(name ILIKE $%d OR email ILIKE $%d OR phone ILIKE $%d OR vat_id ILIKE $%d OR tax_no ILIKE $%d OR debtor_no ILIKE $%d OR creditor_no ILIKE $%d)", idx, idx+1, idx+2, idx+3, idx+4, idx+5, idx+6))
		q := "%" + f.Q + "%"
		args = append(args, q, q, q, q, q, q, q)
		idx += 7
	}
	if strings.TrimSpace(f.Rolle) != "" {
		conds = append(conds, fmt.Sprintf("rolle=$%d", idx))
		args = append(args, f.Rolle)
		idx++
	}
	if strings.TrimSpace(f.Status) != "" {
		conds = append(conds, fmt.Sprintf("status=$%d", idx))
		args = append(args, f.Status)
		idx++
	}
	if strings.TrimSpace(f.Typ) != "" {
		conds = append(conds, fmt.Sprintf("typ=$%d", idx))
		args = append(args, f.Typ)
		idx++
	}
	if len(conds) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conds, " AND "))
	}
	sb.WriteString(" ORDER BY name ASC")
	sb.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", lim, off))
	rows, err := s.pg.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Contact, 0, lim)
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.Typ, &c.Rolle, &c.Status, &c.Name, &c.Email, &c.Telefon, &c.UStID, &c.SteuerNr, &c.Waehrung, &c.Zahlungsbedingungen, &c.DebitorNr, &c.KreditorNr, &c.SteuerLand, &c.Steuerbefreit, &c.Aktiv, &c.Angelegt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Addresses
type Address struct {
	ID        string `json:"id"`
	ContactID string `json:"contact_id"`
	Art       string `json:"art"` // billing | shipping | other
	Zeile1    string `json:"zeile1"`
	Zeile2    string `json:"zeile2"`
	PLZ       string `json:"plz"`
	Ort       string `json:"ort"`
	Land      string `json:"land"`
	Primary   bool   `json:"is_primary"`
}

type AddressCreate struct {
	Art     string `json:"art"`
	Zeile1  string `json:"zeile1"`
	Zeile2  string `json:"zeile2"`
	PLZ     string `json:"plz"`
	Ort     string `json:"ort"`
	Land    string `json:"land"`
	Primary bool   `json:"is_primary"`
}

type AddressUpdate struct {
	Art     *string `json:"art"`
	Zeile1  *string `json:"zeile1"`
	Zeile2  *string `json:"zeile2"`
	PLZ     *string `json:"plz"`
	Ort     *string `json:"ort"`
	Land    *string `json:"land"`
	Primary *bool   `json:"is_primary"`
}

func (s *Service) CreateAddress(ctx context.Context, contactID string, in AddressCreate) (*Address, error) {
	if strings.TrimSpace(in.Zeile1) == "" {
		return nil, errors.New("Zeile1 erforderlich")
	}
	if in.Art == "" {
		in.Art = "other"
	}
	if !isIn(in.Art, AddressKinds()) {
		return nil, errors.New("Ungültige Adressart")
	}
	id := uuid.NewString()
	var a Address
	if _, err := s.pg.Exec(ctx, `SELECT 1 FROM contacts WHERE id=$1`, contactID); err != nil {
		// pgx Exec returns commandTag even if not found; we rely on FK for existence
	}
	err := s.pg.QueryRow(ctx, `
        INSERT INTO contact_addresses (id, contact_id, art, zeile1, zeile2, plz, ort, land, is_primary)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
        RETURNING id, contact_id, art, zeile1, zeile2, plz, ort, land, is_primary
    `, id, contactID, in.Art, in.Zeile1, in.Zeile2, in.PLZ, in.Ort, in.Land, in.Primary).Scan(
		&a.ID, &a.ContactID, &a.Art, &a.Zeile1, &a.Zeile2, &a.PLZ, &a.Ort, &a.Land, &a.Primary,
	)
	if err != nil {
		return nil, err
	}
	if a.Primary {
		// optional: could ensure only one primary by resetting others
		_, _ = s.pg.Exec(ctx, `UPDATE contact_addresses SET is_primary=false WHERE contact_id=$1 AND id<>$2 AND is_primary=true`, contactID, a.ID)
	}
	return &a, nil
}

func (s *Service) ListAddresses(ctx context.Context, contactID string) ([]Address, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, contact_id, art, zeile1, zeile2, plz, ort, land, is_primary FROM contact_addresses WHERE contact_id=$1 ORDER BY is_primary DESC, art ASC, id ASC`, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Address, 0)
	for rows.Next() {
		var a Address
		if err := rows.Scan(&a.ID, &a.ContactID, &a.Art, &a.Zeile1, &a.Zeile2, &a.PLZ, &a.Ort, &a.Land, &a.Primary); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *Service) UpdateAddress(ctx context.Context, contactID, addressID string, u AddressUpdate) (*Address, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(field string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", field, idx))
		args = append(args, v)
		idx++
	}
	if u.Art != nil {
		if !isIn(*u.Art, AddressKinds()) {
			return nil, errors.New("Ungültige Adressart")
		}
		add("art", *u.Art)
	}
	if u.Zeile1 != nil {
		if strings.TrimSpace(*u.Zeile1) == "" {
			return nil, errors.New("Zeile1 erforderlich")
		}
		add("zeile1", *u.Zeile1)
	}
	if u.Zeile2 != nil {
		add("zeile2", *u.Zeile2)
	}
	if u.PLZ != nil {
		add("plz", *u.PLZ)
	}
	if u.Ort != nil {
		add("ort", *u.Ort)
	}
	if u.Land != nil {
		add("land", *u.Land)
	}
	if u.Primary != nil {
		add("is_primary", *u.Primary)
	}
	if len(sets) == 0 {
		return s.getAddress(ctx, addressID)
	}
	args = append(args, contactID, addressID)
	q := fmt.Sprintf("UPDATE contact_addresses SET %s WHERE contact_id=$%d AND id=$%d", strings.Join(sets, ", "), idx, idx+1)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	if u.Primary != nil && *u.Primary {
		_, _ = s.pg.Exec(ctx, `UPDATE contact_addresses SET is_primary=false WHERE contact_id=$1 AND id<>$2 AND is_primary=true`, contactID, addressID)
	}
	return s.getAddress(ctx, addressID)
}

func (s *Service) getAddress(ctx context.Context, addressID string) (*Address, error) {
	var a Address
	if err := s.pg.QueryRow(ctx, `SELECT id, contact_id, art, zeile1, zeile2, plz, ort, land, is_primary FROM contact_addresses WHERE id=$1`, addressID).Scan(
		&a.ID, &a.ContactID, &a.Art, &a.Zeile1, &a.Zeile2, &a.PLZ, &a.Ort, &a.Land, &a.Primary,
	); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Service) DeleteAddress(ctx context.Context, contactID, addressID string) error {
	_, err := s.pg.Exec(ctx, `DELETE FROM contact_addresses WHERE contact_id=$1 AND id=$2`, contactID, addressID)
	return err
}

// Persons
type Person struct {
	ID               string `json:"id"`
	ContactID        string `json:"contact_id"`
	Anrede           string `json:"anrede"`
	Vorname          string `json:"vorname"`
	Nachname         string `json:"nachname"`
	Position         string `json:"position"`
	Rolle            string `json:"rolle"`
	BevorzugterKanal string `json:"bevorzugter_kanal"`
	Email            string `json:"email"`
	Telefon          string `json:"telefon"`
	Mobil            string `json:"mobil"`
	Primary          bool   `json:"is_primary"`
}

type PersonCreate struct {
	Anrede           string `json:"anrede"`
	Vorname          string `json:"vorname"`
	Nachname         string `json:"nachname"`
	Position         string `json:"position"`
	Rolle            string `json:"rolle"`
	BevorzugterKanal string `json:"bevorzugter_kanal"`
	Email            string `json:"email"`
	Telefon          string `json:"telefon"`
	Mobil            string `json:"mobil"`
	Primary          bool   `json:"is_primary"`
}

type PersonUpdate struct {
	Anrede           *string `json:"anrede"`
	Vorname          *string `json:"vorname"`
	Nachname         *string `json:"nachname"`
	Position         *string `json:"position"`
	Rolle            *string `json:"rolle"`
	BevorzugterKanal *string `json:"bevorzugter_kanal"`
	Email            *string `json:"email"`
	Telefon          *string `json:"telefon"`
	Mobil            *string `json:"mobil"`
	Primary          *bool   `json:"is_primary"`
}

func (s *Service) CreatePerson(ctx context.Context, contactID string, in PersonCreate) (*Person, error) {
	if strings.TrimSpace(in.Nachname) == "" && strings.TrimSpace(in.Vorname) == "" {
		return nil, errors.New("Name erforderlich")
	}
	in.Rolle = strings.TrimSpace(strings.ToLower(in.Rolle))
	if in.Rolle == "" {
		in.Rolle = "other"
	}
	if !isIn(in.Rolle, PersonRoles()) {
		return nil, errors.New("Ungültige Ansprechpartnerrolle")
	}
	in.BevorzugterKanal = strings.TrimSpace(strings.ToLower(in.BevorzugterKanal))
	if in.BevorzugterKanal != "" && !isIn(in.BevorzugterKanal, CommunicationChannels()) {
		return nil, errors.New("Ungültiger Kommunikationskanal")
	}
	id := uuid.NewString()
	var p Person
	err := s.pg.QueryRow(ctx, `
        INSERT INTO contact_persons (id, contact_id, anrede, vorname, nachname, position, rolle, bevorzugter_kanal, email, phone, mobile, is_primary)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
        RETURNING id, contact_id, anrede, vorname, nachname, position, rolle, bevorzugter_kanal, email, phone, mobile, is_primary
    `, id, contactID, in.Anrede, in.Vorname, in.Nachname, in.Position, in.Rolle, in.BevorzugterKanal, in.Email, in.Telefon, in.Mobil, in.Primary).Scan(
		&p.ID, &p.ContactID, &p.Anrede, &p.Vorname, &p.Nachname, &p.Position, &p.Rolle, &p.BevorzugterKanal, &p.Email, &p.Telefon, &p.Mobil, &p.Primary,
	)
	if err != nil {
		return nil, err
	}
	if p.Primary {
		_, _ = s.pg.Exec(ctx, `UPDATE contact_persons SET is_primary=false WHERE contact_id=$1 AND id<>$2 AND is_primary=true`, contactID, p.ID)
	}
	return &p, nil
}

func (s *Service) ListPersons(ctx context.Context, contactID string) ([]Person, error) {
	rows, err := s.pg.Query(ctx, `SELECT id, contact_id, anrede, vorname, nachname, position, rolle, bevorzugter_kanal, email, phone, mobile, is_primary FROM contact_persons WHERE contact_id=$1 ORDER BY is_primary DESC, nachname ASC, vorname ASC`, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Person, 0)
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.ContactID, &p.Anrede, &p.Vorname, &p.Nachname, &p.Position, &p.Rolle, &p.BevorzugterKanal, &p.Email, &p.Telefon, &p.Mobil, &p.Primary); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Service) UpdatePerson(ctx context.Context, contactID, personID string, u PersonUpdate) (*Person, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(field string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", field, idx))
		args = append(args, v)
		idx++
	}
	if u.Anrede != nil {
		add("anrede", *u.Anrede)
	}
	if u.Vorname != nil {
		add("vorname", *u.Vorname)
	}
	if u.Nachname != nil {
		add("nachname", *u.Nachname)
	}
	if u.Position != nil {
		add("position", *u.Position)
	}
	if u.Rolle != nil {
		role := strings.TrimSpace(strings.ToLower(*u.Rolle))
		if role == "" {
			role = "other"
		}
		if !isIn(role, PersonRoles()) {
			return nil, errors.New("Ungültige Ansprechpartnerrolle")
		}
		add("rolle", role)
	}
	if u.BevorzugterKanal != nil {
		channel := strings.TrimSpace(strings.ToLower(*u.BevorzugterKanal))
		if channel != "" && !isIn(channel, CommunicationChannels()) {
			return nil, errors.New("Ungültiger Kommunikationskanal")
		}
		add("bevorzugter_kanal", channel)
	}
	if u.Email != nil {
		add("email", *u.Email)
	}
	if u.Telefon != nil {
		add("phone", *u.Telefon)
	}
	if u.Mobil != nil {
		add("mobile", *u.Mobil)
	}
	if u.Primary != nil {
		add("is_primary", *u.Primary)
	}
	if len(sets) == 0 {
		return s.getPerson(ctx, personID)
	}
	args = append(args, contactID, personID)
	q := fmt.Sprintf("UPDATE contact_persons SET %s WHERE contact_id=$%d AND id=$%d", strings.Join(sets, ", "), idx, idx+1)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	if u.Primary != nil && *u.Primary {
		_, _ = s.pg.Exec(ctx, `UPDATE contact_persons SET is_primary=false WHERE contact_id=$1 AND id<>$2 AND is_primary=true`, contactID, personID)
	}
	return s.getPerson(ctx, personID)
}

func (s *Service) getPerson(ctx context.Context, personID string) (*Person, error) {
	var p Person
	if err := s.pg.QueryRow(ctx, `SELECT id, contact_id, anrede, vorname, nachname, position, rolle, bevorzugter_kanal, email, phone, mobile, is_primary FROM contact_persons WHERE id=$1`, personID).Scan(
		&p.ID, &p.ContactID, &p.Anrede, &p.Vorname, &p.Nachname, &p.Position, &p.Rolle, &p.BevorzugterKanal, &p.Email, &p.Telefon, &p.Mobil, &p.Primary,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) DeletePerson(ctx context.Context, contactID, personID string) error {
	_, err := s.pg.Exec(ctx, `DELETE FROM contact_persons WHERE contact_id=$1 AND id=$2`, contactID, personID)
	return err
}

// Notes
type Note struct {
	ID             string    `json:"id"`
	ContactID      string    `json:"contact_id"`
	Titel          string    `json:"titel"`
	Inhalt         string    `json:"inhalt"`
	ErstelltAm     time.Time `json:"erstellt_am"`
	AktualisiertAm time.Time `json:"aktualisiert_am"`
}

type NoteCreate struct {
	Titel  string `json:"titel"`
	Inhalt string `json:"inhalt"`
}

type NoteUpdate struct {
	Titel  *string `json:"titel"`
	Inhalt *string `json:"inhalt"`
}

func (s *Service) CreateNote(ctx context.Context, contactID string, in NoteCreate) (*Note, error) {
	if strings.TrimSpace(in.Titel) == "" && strings.TrimSpace(in.Inhalt) == "" {
		return nil, errors.New("Notizinhalt erforderlich")
	}
	id := uuid.NewString()
	var n Note
	err := s.pg.QueryRow(ctx, `
        INSERT INTO contact_notes (id, contact_id, titel, inhalt)
        VALUES ($1,$2,$3,$4)
        RETURNING id, contact_id, titel, inhalt, erstellt_am, aktualisiert_am
    `, id, contactID, strings.TrimSpace(in.Titel), strings.TrimSpace(in.Inhalt)).Scan(
		&n.ID, &n.ContactID, &n.Titel, &n.Inhalt, &n.ErstelltAm, &n.AktualisiertAm,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *Service) ListNotes(ctx context.Context, contactID string) ([]Note, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT id, contact_id, titel, inhalt, erstellt_am, aktualisiert_am
        FROM contact_notes
        WHERE contact_id=$1
        ORDER BY aktualisiert_am DESC, erstellt_am DESC, id DESC
    `, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Note, 0)
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.ContactID, &n.Titel, &n.Inhalt, &n.ErstelltAm, &n.AktualisiertAm); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *Service) UpdateNote(ctx context.Context, contactID, noteID string, u NoteUpdate) (*Note, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(field string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", field, idx))
		args = append(args, v)
		idx++
	}
	if u.Titel != nil {
		add("titel", strings.TrimSpace(*u.Titel))
	}
	if u.Inhalt != nil {
		add("inhalt", strings.TrimSpace(*u.Inhalt))
	}
	if len(sets) == 0 {
		return s.getNote(ctx, noteID)
	}
	sets = append(sets, "aktualisiert_am=now()")
	args = append(args, contactID, noteID)
	q := fmt.Sprintf("UPDATE contact_notes SET %s WHERE contact_id=$%d AND id=$%d", strings.Join(sets, ", "), idx, idx+1)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.getNote(ctx, noteID)
}

func (s *Service) getNote(ctx context.Context, noteID string) (*Note, error) {
	var n Note
	err := s.pg.QueryRow(ctx, `
        SELECT id, contact_id, titel, inhalt, erstellt_am, aktualisiert_am
        FROM contact_notes
        WHERE id=$1
    `, noteID).Scan(&n.ID, &n.ContactID, &n.Titel, &n.Inhalt, &n.ErstelltAm, &n.AktualisiertAm)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *Service) DeleteNote(ctx context.Context, contactID, noteID string) error {
	_, err := s.pg.Exec(ctx, `DELETE FROM contact_notes WHERE contact_id=$1 AND id=$2`, contactID, noteID)
	return err
}

// Tasks
type Task struct {
	ID             string     `json:"id"`
	ContactID      string     `json:"contact_id"`
	Titel          string     `json:"titel"`
	Beschreibung   string     `json:"beschreibung"`
	Status         string     `json:"status"`
	FaelligAm      *time.Time `json:"faellig_am"`
	ErledigtAm     *time.Time `json:"erledigt_am"`
	ErstelltAm     time.Time  `json:"erstellt_am"`
	AktualisiertAm time.Time  `json:"aktualisiert_am"`
}

type TaskCreate struct {
	Titel        string  `json:"titel"`
	Beschreibung string  `json:"beschreibung"`
	Status       string  `json:"status"`
	FaelligAm    *string `json:"faellig_am"`
}

type TaskUpdate struct {
	Titel        *string `json:"titel"`
	Beschreibung *string `json:"beschreibung"`
	Status       *string `json:"status"`
	FaelligAm    *string `json:"faellig_am"`
}

func parseOptionalTimestamp(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, errors.New("Ungültiges Fälligkeitsdatum")
	}
	return &parsed, nil
}

func taskCompletionTimestamp(status string) *time.Time {
	if status != "done" {
		return nil
	}
	now := time.Now().UTC()
	return &now
}

func (s *Service) CreateTask(ctx context.Context, contactID string, in TaskCreate) (*Task, error) {
	titel := strings.TrimSpace(in.Titel)
	beschreibung := strings.TrimSpace(in.Beschreibung)
	if titel == "" {
		return nil, errors.New("Aufgabentitel erforderlich")
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "open"
	}
	status = strings.ToLower(status)
	if !isIn(status, TaskStatuses()) {
		return nil, errors.New("Ungültiger Aufgabenstatus")
	}
	faelligAm, err := parseOptionalTimestamp(in.FaelligAm)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	var t Task
	erledigtAm := taskCompletionTimestamp(status)
	err = s.pg.QueryRow(ctx, `
        INSERT INTO contact_tasks (id, contact_id, titel, beschreibung, status, faellig_am, erledigt_am)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        RETURNING id, contact_id, titel, beschreibung, status, faellig_am, erledigt_am, erstellt_am, aktualisiert_am
    `, id, contactID, titel, beschreibung, status, faelligAm, erledigtAm).Scan(
		&t.ID, &t.ContactID, &t.Titel, &t.Beschreibung, &t.Status, &t.FaelligAm, &t.ErledigtAm, &t.ErstelltAm, &t.AktualisiertAm,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) ListTasks(ctx context.Context, contactID string) ([]Task, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT id, contact_id, titel, beschreibung, status, faellig_am, erledigt_am, erstellt_am, aktualisiert_am
        FROM contact_tasks
        WHERE contact_id=$1
        ORDER BY
            CASE status
                WHEN 'open' THEN 0
                WHEN 'in_progress' THEN 1
                WHEN 'done' THEN 2
                WHEN 'canceled' THEN 3
                ELSE 9
            END,
            faellig_am NULLS LAST,
            aktualisiert_am DESC,
            id DESC
    `, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Task, 0)
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ContactID, &t.Titel, &t.Beschreibung, &t.Status, &t.FaelligAm, &t.ErledigtAm, &t.ErstelltAm, &t.AktualisiertAm); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (s *Service) UpdateTask(ctx context.Context, contactID, taskID string, u TaskUpdate) (*Task, error) {
	current, err := s.getTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(field string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", field, idx))
		args = append(args, v)
		idx++
	}
	nextStatus := current.Status
	if u.Titel != nil {
		titel := strings.TrimSpace(*u.Titel)
		if titel == "" {
			return nil, errors.New("Aufgabentitel erforderlich")
		}
		add("titel", titel)
	}
	if u.Beschreibung != nil {
		add("beschreibung", strings.TrimSpace(*u.Beschreibung))
	}
	if u.Status != nil {
		status := strings.ToLower(strings.TrimSpace(*u.Status))
		if !isIn(status, TaskStatuses()) {
			return nil, errors.New("Ungültiger Aufgabenstatus")
		}
		nextStatus = status
		add("status", status)
	}
	if u.FaelligAm != nil {
		faelligAm, err := parseOptionalTimestamp(u.FaelligAm)
		if err != nil {
			return nil, err
		}
		add("faellig_am", faelligAm)
	}
	if len(sets) == 0 {
		return s.getTask(ctx, taskID)
	}
	add("erledigt_am", taskCompletionTimestamp(nextStatus))
	sets = append(sets, "aktualisiert_am=now()")
	args = append(args, contactID, taskID)
	q := fmt.Sprintf("UPDATE contact_tasks SET %s WHERE contact_id=$%d AND id=$%d", strings.Join(sets, ", "), idx, idx+1)
	if _, err := s.pg.Exec(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.getTask(ctx, taskID)
}

func (s *Service) getTask(ctx context.Context, taskID string) (*Task, error) {
	var t Task
	err := s.pg.QueryRow(ctx, `
        SELECT id, contact_id, titel, beschreibung, status, faellig_am, erledigt_am, erstellt_am, aktualisiert_am
        FROM contact_tasks
        WHERE id=$1
    `, taskID).Scan(&t.ID, &t.ContactID, &t.Titel, &t.Beschreibung, &t.Status, &t.FaelligAm, &t.ErledigtAm, &t.ErstelltAm, &t.AktualisiertAm)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) DeleteTask(ctx context.Context, contactID, taskID string) error {
	_, err := s.pg.Exec(ctx, `DELETE FROM contact_tasks WHERE contact_id=$1 AND id=$2`, contactID, taskID)
	return err
}

// Facets
func (s *Service) ListRoles(ctx context.Context) ([]string, error)    { return Roles(), nil }
func (s *Service) ListStatuses(ctx context.Context) ([]string, error) { return Statuses(), nil }
