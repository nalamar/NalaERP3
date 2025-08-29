package contacts

import (
    "context"
    "errors"
    "fmt"
    "strings"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

type Service struct { pg *pgxpool.Pool }
func NewService(pg *pgxpool.Pool) *Service { return &Service{pg: pg} }

// Enumerationen (fest)
func Roles() []string { return []string{"customer","supplier","both","other"} }
func Types() []string { return []string{"org","person"} }
func AddressKinds() []string { return []string{"billing","shipping","other"} }

func isIn(v string, list []string) bool {
    v = strings.ToLower(strings.TrimSpace(v))
    for _, x := range list { if v == x { return true } }
    return false
}

// Core types
type Contact struct {
    ID        string    `json:"id"`
    Typ       string    `json:"typ"`        // org | person
    Rolle     string    `json:"rolle"`      // customer | supplier | both | other
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Telefon   string    `json:"telefon"`
    UStID     string    `json:"ust_id"`
    SteuerNr  string    `json:"steuernummer"`
    Waehrung  string    `json:"waehrung"`
    Aktiv     bool      `json:"aktiv"`
    Angelegt  time.Time `json:"angelegt_am"`
}

type ContactCreate struct {
    Typ      string `json:"typ"`
    Rolle    string `json:"rolle"`
    Name     string `json:"name"`
    Email    string `json:"email"`
    Telefon  string `json:"telefon"`
    UStID    string `json:"ust_id"`
    SteuerNr string `json:"steuernummer"`
    Waehrung string `json:"waehrung"`
}

type ContactUpdate struct {
    Typ      *string `json:"typ"`
    Rolle    *string `json:"rolle"`
    Name     *string `json:"name"`
    Email    *string `json:"email"`
    Telefon  *string `json:"telefon"`
    UStID    *string `json:"ust_id"`
    SteuerNr *string `json:"steuernummer"`
    Waehrung *string `json:"waehrung"`
    Aktiv    *bool   `json:"aktiv"`
}

type ContactFilter struct {
    Q      string
    Rolle  string
    Typ    string
    Limit  int
    Offset int
}

func (s *Service) Create(ctx context.Context, in ContactCreate) (*Contact, error) {
    if strings.TrimSpace(in.Name) == "" { return nil, errors.New("Name erforderlich") }
    if in.Typ == "" { in.Typ = "org" }
    if !isIn(in.Typ, Types()) { return nil, errors.New("Ungültiger Typ") }
    if in.Rolle == "" { in.Rolle = "other" }
    if !isIn(in.Rolle, Roles()) { return nil, errors.New("Ungültige Rolle") }
    if in.Waehrung == "" { in.Waehrung = "EUR" }
    id := uuid.NewString()
    var c Contact
    err := s.pg.QueryRow(ctx, `
        INSERT INTO contacts (id, typ, rolle, name, email, phone, vat_id, tax_no, waehrung)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
        RETURNING id, typ, rolle, name, COALESCE(email,''), COALESCE(phone,''), COALESCE(vat_id,''), COALESCE(tax_no,''), waehrung, aktiv, angelegt_am
    `, id, in.Typ, in.Rolle, in.Name, in.Email, in.Telefon, in.UStID, in.SteuerNr, in.Waehrung).Scan(
        &c.ID, &c.Typ, &c.Rolle, &c.Name, &c.Email, &c.Telefon, &c.UStID, &c.SteuerNr, &c.Waehrung, &c.Aktiv, &c.Angelegt,
    )
    if err != nil { return nil, err }
    return &c, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Contact, error) {
    var c Contact
    err := s.pg.QueryRow(ctx, `SELECT id, typ, rolle, name, COALESCE(email,''), COALESCE(phone,''), COALESCE(vat_id,''), COALESCE(tax_no,''), waehrung, aktiv, angelegt_am FROM contacts WHERE id=$1`, id).Scan(
        &c.ID, &c.Typ, &c.Rolle, &c.Name, &c.Email, &c.Telefon, &c.UStID, &c.SteuerNr, &c.Waehrung, &c.Aktiv, &c.Angelegt,
    )
    if err != nil { return nil, err }
    return &c, nil
}

func (s *Service) Update(ctx context.Context, id string, u ContactUpdate) (*Contact, error) {
    // build SET dynamically
    sets := make([]string, 0)
    args := make([]any, 0)
    idx := 1
    add := func(field string, v any) { sets = append(sets, fmt.Sprintf("%s=$%d", field, idx)); args = append(args, v); idx++ }
    if u.Typ != nil {
        if !isIn(*u.Typ, Types()) { return nil, errors.New("Ungültiger Typ") }
        add("typ", *u.Typ)
    }
    if u.Rolle != nil {
        if !isIn(*u.Rolle, Roles()) { return nil, errors.New("Ungültige Rolle") }
        add("rolle", *u.Rolle)
    }
    if u.Name != nil { add("name", *u.Name) }
    if u.Email != nil { add("email", *u.Email) }
    if u.Telefon != nil { add("phone", *u.Telefon) }
    if u.UStID != nil { add("vat_id", *u.UStID) }
    if u.SteuerNr != nil { add("tax_no", *u.SteuerNr) }
    if u.Waehrung != nil { add("waehrung", *u.Waehrung) }
    if u.Aktiv != nil { add("aktiv", *u.Aktiv) }
    if len(sets) == 0 { return s.Get(ctx, id) }
    args = append(args, id)
    q := fmt.Sprintf("UPDATE contacts SET %s WHERE id=$%d", strings.Join(sets, ", "), idx)
    if _, err := s.pg.Exec(ctx, q, args...); err != nil { return nil, err }
    return s.Get(ctx, id)
}

func (s *Service) DeleteSoft(ctx context.Context, id string) error {
    _, err := s.pg.Exec(ctx, `UPDATE contacts SET aktiv=false WHERE id=$1`, id)
    return err
}

func (s *Service) List(ctx context.Context, f ContactFilter) ([]Contact, error) {
    lim := f.Limit; if lim <= 0 || lim > 200 { lim = 50 }
    off := f.Offset
    sb := strings.Builder{}
    sb.WriteString(`SELECT id, typ, rolle, name, COALESCE(email,''), COALESCE(phone,''), COALESCE(vat_id,''), COALESCE(tax_no,''), waehrung, aktiv, angelegt_am FROM contacts`)
    var conds []string
    var args []any
    idx := 1
    if strings.TrimSpace(f.Q) != "" {
        conds = append(conds, fmt.Sprintf("(name ILIKE $%d OR email ILIKE $%d OR phone ILIKE $%d OR vat_id ILIKE $%d)", idx, idx+1, idx+2, idx+3))
        q := "%" + f.Q + "%"; args = append(args, q, q, q, q); idx += 4
    }
    if strings.TrimSpace(f.Rolle) != "" { conds = append(conds, fmt.Sprintf("rolle=$%d", idx)); args = append(args, f.Rolle); idx++ }
    if strings.TrimSpace(f.Typ) != "" { conds = append(conds, fmt.Sprintf("typ=$%d", idx)); args = append(args, f.Typ); idx++ }
    if len(conds) > 0 { sb.WriteString(" WHERE "); sb.WriteString(strings.Join(conds, " AND ")) }
    sb.WriteString(" ORDER BY name ASC")
    sb.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", lim, off))
    rows, err := s.pg.Query(ctx, sb.String(), args...)
    if err != nil { return nil, err }
    defer rows.Close()
    out := make([]Contact, 0, lim)
    for rows.Next() {
        var c Contact
        if err := rows.Scan(&c.ID, &c.Typ, &c.Rolle, &c.Name, &c.Email, &c.Telefon, &c.UStID, &c.SteuerNr, &c.Waehrung, &c.Aktiv, &c.Angelegt); err != nil { return nil, err }
        out = append(out, c)
    }
    return out, nil
}

// Addresses
type Address struct {
    ID         string `json:"id"`
    ContactID  string `json:"contact_id"`
    Art        string `json:"art"` // billing | shipping | other
    Zeile1     string `json:"zeile1"`
    Zeile2     string `json:"zeile2"`
    PLZ        string `json:"plz"`
    Ort        string `json:"ort"`
    Land       string `json:"land"`
    Primary    bool   `json:"is_primary"`
}

type AddressCreate struct {
    Art    string `json:"art"`
    Zeile1 string `json:"zeile1"`
    Zeile2 string `json:"zeile2"`
    PLZ    string `json:"plz"`
    Ort    string `json:"ort"`
    Land   string `json:"land"`
    Primary bool  `json:"is_primary"`
}

type AddressUpdate struct {
    Art    *string `json:"art"`
    Zeile1 *string `json:"zeile1"`
    Zeile2 *string `json:"zeile2"`
    PLZ    *string `json:"plz"`
    Ort    *string `json:"ort"`
    Land   *string `json:"land"`
    Primary *bool  `json:"is_primary"`
}

func (s *Service) CreateAddress(ctx context.Context, contactID string, in AddressCreate) (*Address, error) {
    if strings.TrimSpace(in.Zeile1) == "" { return nil, errors.New("Zeile1 erforderlich") }
    if in.Art == "" { in.Art = "other" }
    if !isIn(in.Art, AddressKinds()) { return nil, errors.New("Ungültige Adressart") }
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
    if err != nil { return nil, err }
    if a.Primary {
        // optional: could ensure only one primary by resetting others
        _, _ = s.pg.Exec(ctx, `UPDATE contact_addresses SET is_primary=false WHERE contact_id=$1 AND id<>$2 AND is_primary=true`, contactID, a.ID)
    }
    return &a, nil
}

func (s *Service) ListAddresses(ctx context.Context, contactID string) ([]Address, error) {
    rows, err := s.pg.Query(ctx, `SELECT id, contact_id, art, zeile1, zeile2, plz, ort, land, is_primary FROM contact_addresses WHERE contact_id=$1 ORDER BY is_primary DESC, art ASC, id ASC`, contactID)
    if err != nil { return nil, err }
    defer rows.Close()
    out := make([]Address, 0)
    for rows.Next() {
        var a Address
        if err := rows.Scan(&a.ID, &a.ContactID, &a.Art, &a.Zeile1, &a.Zeile2, &a.PLZ, &a.Ort, &a.Land, &a.Primary); err != nil { return nil, err }
        out = append(out, a)
    }
    return out, nil
}

func (s *Service) UpdateAddress(ctx context.Context, contactID, addressID string, u AddressUpdate) (*Address, error) {
    sets := make([]string, 0)
    args := make([]any, 0)
    idx := 1
    add := func(field string, v any) { sets = append(sets, fmt.Sprintf("%s=$%d", field, idx)); args = append(args, v); idx++ }
    if u.Art != nil {
        if !isIn(*u.Art, AddressKinds()) { return nil, errors.New("Ungültige Adressart") }
        add("art", *u.Art)
    }
    if u.Zeile1 != nil { if strings.TrimSpace(*u.Zeile1) == "" { return nil, errors.New("Zeile1 erforderlich") } ; add("zeile1", *u.Zeile1) }
    if u.Zeile2 != nil { add("zeile2", *u.Zeile2) }
    if u.PLZ != nil { add("plz", *u.PLZ) }
    if u.Ort != nil { add("ort", *u.Ort) }
    if u.Land != nil { add("land", *u.Land) }
    if u.Primary != nil { add("is_primary", *u.Primary) }
    if len(sets) == 0 { return s.getAddress(ctx, addressID) }
    args = append(args, contactID, addressID)
    q := fmt.Sprintf("UPDATE contact_addresses SET %s WHERE contact_id=$%d AND id=$%d", strings.Join(sets, ", "), idx, idx+1)
    if _, err := s.pg.Exec(ctx, q, args...); err != nil { return nil, err }
    if u.Primary != nil && *u.Primary {
        _, _ = s.pg.Exec(ctx, `UPDATE contact_addresses SET is_primary=false WHERE contact_id=$1 AND id<>$2 AND is_primary=true`, contactID, addressID)
    }
    return s.getAddress(ctx, addressID)
}

func (s *Service) getAddress(ctx context.Context, addressID string) (*Address, error) {
    var a Address
    if err := s.pg.QueryRow(ctx, `SELECT id, contact_id, art, zeile1, zeile2, plz, ort, land, is_primary FROM contact_addresses WHERE id=$1`, addressID).Scan(
        &a.ID, &a.ContactID, &a.Art, &a.Zeile1, &a.Zeile2, &a.PLZ, &a.Ort, &a.Land, &a.Primary,
    ); err != nil { return nil, err }
    return &a, nil
}

func (s *Service) DeleteAddress(ctx context.Context, contactID, addressID string) error {
    _, err := s.pg.Exec(ctx, `DELETE FROM contact_addresses WHERE contact_id=$1 AND id=$2`, contactID, addressID)
    return err
}

// Persons
type Person struct {
    ID        string `json:"id"`
    ContactID string `json:"contact_id"`
    Anrede    string `json:"anrede"`
    Vorname   string `json:"vorname"`
    Nachname  string `json:"nachname"`
    Position  string `json:"position"`
    Email     string `json:"email"`
    Telefon   string `json:"telefon"`
    Mobil     string `json:"mobil"`
    Primary   bool   `json:"is_primary"`
}

type PersonCreate struct {
    Anrede   string `json:"anrede"`
    Vorname  string `json:"vorname"`
    Nachname string `json:"nachname"`
    Position string `json:"position"`
    Email    string `json:"email"`
    Telefon  string `json:"telefon"`
    Mobil    string `json:"mobil"`
    Primary  bool   `json:"is_primary"`
}

type PersonUpdate struct {
    Anrede   *string `json:"anrede"`
    Vorname  *string `json:"vorname"`
    Nachname *string `json:"nachname"`
    Position *string `json:"position"`
    Email    *string `json:"email"`
    Telefon  *string `json:"telefon"`
    Mobil    *string `json:"mobil"`
    Primary  *bool   `json:"is_primary"`
}

func (s *Service) CreatePerson(ctx context.Context, contactID string, in PersonCreate) (*Person, error) {
    if strings.TrimSpace(in.Nachname) == "" && strings.TrimSpace(in.Vorname) == "" { return nil, errors.New("Name erforderlich") }
    id := uuid.NewString()
    var p Person
    err := s.pg.QueryRow(ctx, `
        INSERT INTO contact_persons (id, contact_id, anrede, vorname, nachname, position, email, phone, mobile, is_primary)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
        RETURNING id, contact_id, anrede, vorname, nachname, position, email, phone, mobile, is_primary
    `, id, contactID, in.Anrede, in.Vorname, in.Nachname, in.Position, in.Email, in.Telefon, in.Mobil, in.Primary).Scan(
        &p.ID, &p.ContactID, &p.Anrede, &p.Vorname, &p.Nachname, &p.Position, &p.Email, &p.Telefon, &p.Mobil, &p.Primary,
    )
    if err != nil { return nil, err }
    if p.Primary { _, _ = s.pg.Exec(ctx, `UPDATE contact_persons SET is_primary=false WHERE contact_id=$1 AND id<>$2 AND is_primary=true`, contactID, p.ID) }
    return &p, nil
}

func (s *Service) ListPersons(ctx context.Context, contactID string) ([]Person, error) {
    rows, err := s.pg.Query(ctx, `SELECT id, contact_id, anrede, vorname, nachname, position, email, phone, mobile, is_primary FROM contact_persons WHERE contact_id=$1 ORDER BY is_primary DESC, nachname ASC, vorname ASC`, contactID)
    if err != nil { return nil, err }
    defer rows.Close()
    out := make([]Person, 0)
    for rows.Next() {
        var p Person
        if err := rows.Scan(&p.ID, &p.ContactID, &p.Anrede, &p.Vorname, &p.Nachname, &p.Position, &p.Email, &p.Telefon, &p.Mobil, &p.Primary); err != nil { return nil, err }
        out = append(out, p)
    }
    return out, nil
}

func (s *Service) UpdatePerson(ctx context.Context, contactID, personID string, u PersonUpdate) (*Person, error) {
    sets := make([]string, 0)
    args := make([]any, 0)
    idx := 1
    add := func(field string, v any) { sets = append(sets, fmt.Sprintf("%s=$%d", field, idx)); args = append(args, v); idx++ }
    if u.Anrede != nil { add("anrede", *u.Anrede) }
    if u.Vorname != nil { add("vorname", *u.Vorname) }
    if u.Nachname != nil { add("nachname", *u.Nachname) }
    if u.Position != nil { add("position", *u.Position) }
    if u.Email != nil { add("email", *u.Email) }
    if u.Telefon != nil { add("phone", *u.Telefon) }
    if u.Mobil != nil { add("mobile", *u.Mobil) }
    if u.Primary != nil { add("is_primary", *u.Primary) }
    if len(sets) == 0 { return s.getPerson(ctx, personID) }
    args = append(args, contactID, personID)
    q := fmt.Sprintf("UPDATE contact_persons SET %s WHERE contact_id=$%d AND id=$%d", strings.Join(sets, ", "), idx, idx+1)
    if _, err := s.pg.Exec(ctx, q, args...); err != nil { return nil, err }
    if u.Primary != nil && *u.Primary {
        _, _ = s.pg.Exec(ctx, `UPDATE contact_persons SET is_primary=false WHERE contact_id=$1 AND id<>$2 AND is_primary=true`, contactID, personID)
    }
    return s.getPerson(ctx, personID)
}

func (s *Service) getPerson(ctx context.Context, personID string) (*Person, error) {
    var p Person
    if err := s.pg.QueryRow(ctx, `SELECT id, contact_id, anrede, vorname, nachname, position, email, phone, mobile, is_primary FROM contact_persons WHERE id=$1`, personID).Scan(
        &p.ID, &p.ContactID, &p.Anrede, &p.Vorname, &p.Nachname, &p.Position, &p.Email, &p.Telefon, &p.Mobil, &p.Primary,
    ); err != nil { return nil, err }
    return &p, nil
}

func (s *Service) DeletePerson(ctx context.Context, contactID, personID string) error {
    _, err := s.pg.Exec(ctx, `DELETE FROM contact_persons WHERE contact_id=$1 AND id=$2`, contactID, personID)
    return err
}

// Facets
func (s *Service) ListRoles(ctx context.Context) ([]string, error) { return Roles(), nil }
