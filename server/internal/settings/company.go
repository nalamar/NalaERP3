package settings

import (
    "context"
    "errors"
    "strings"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type CompanyProfile struct {
    ID            string    `json:"id"`
    Name          string    `json:"name"`
    LegalForm     string    `json:"legal_form"`
    BranchName    string    `json:"branch_name"`
    Street        string    `json:"street"`
    PostalCode    string    `json:"postal_code"`
    City          string    `json:"city"`
    Country       string    `json:"country"`
    Email         string    `json:"email"`
    Phone         string    `json:"phone"`
    Website       string    `json:"website"`
    InvoiceEmail  string    `json:"invoice_email"`
    TaxNo         string    `json:"tax_no"`
    VatID         string    `json:"vat_id"`
    BankName      string    `json:"bank_name"`
    AccountHolder string    `json:"account_holder"`
    IBAN          string    `json:"iban"`
    BIC           string    `json:"bic"`
    UpdatedAt     time.Time `json:"updated_at"`
}

type CompanyBranch struct {
    ID        string    `json:"id"`
    CompanyID string    `json:"company_id"`
    Code      string    `json:"code"`
    Name      string    `json:"name"`
    Street    string    `json:"street"`
    PostalCode string   `json:"postal_code"`
    City      string    `json:"city"`
    Country   string    `json:"country"`
    Email     string    `json:"email"`
    Phone     string    `json:"phone"`
    IsDefault bool      `json:"is_default"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type CompanyService struct{ pg *pgxpool.Pool }

func NewCompanyService(pg *pgxpool.Pool) *CompanyService { return &CompanyService{pg: pg} }

func (s *CompanyService) Get(ctx context.Context) (*CompanyProfile, error) {
    var out CompanyProfile
    err := s.pg.QueryRow(ctx, `
        SELECT id, name, legal_form, branch_name, street, postal_code, city, country, email, phone, website,
               invoice_email, tax_no, vat_id, bank_name, account_holder, iban, bic, updated_at
        FROM company_profiles
        WHERE id='default'
    `).Scan(
        &out.ID, &out.Name, &out.LegalForm, &out.BranchName, &out.Street, &out.PostalCode, &out.City, &out.Country,
        &out.Email, &out.Phone, &out.Website, &out.InvoiceEmail, &out.TaxNo, &out.VatID, &out.BankName,
        &out.AccountHolder, &out.IBAN, &out.BIC, &out.UpdatedAt,
    )
    if err != nil {
        return nil, err
    }
    return &out, nil
}

func (s *CompanyService) Upsert(ctx context.Context, in CompanyProfile) error {
    in.Name = trim(in.Name)
    if in.Name == "" {
        return errors.New("Firmenname erforderlich")
    }
    in.LegalForm = trim(in.LegalForm)
    in.BranchName = trim(in.BranchName)
    in.Street = trim(in.Street)
    in.PostalCode = trim(in.PostalCode)
    in.City = trim(in.City)
    in.Country = normalizeCountry(in.Country)
    in.Email = trim(in.Email)
    in.Phone = trim(in.Phone)
    in.Website = trim(in.Website)
    in.InvoiceEmail = trim(in.InvoiceEmail)
    in.TaxNo = trim(in.TaxNo)
    in.VatID = strings.ToUpper(trim(in.VatID))
    in.BankName = trim(in.BankName)
    in.AccountHolder = trim(in.AccountHolder)
    in.IBAN = normalizeCompactUpper(in.IBAN)
    in.BIC = normalizeCompactUpper(in.BIC)

    _, err := s.pg.Exec(ctx, `
        INSERT INTO company_profiles (
            id, name, legal_form, branch_name, street, postal_code, city, country, email, phone, website,
            invoice_email, tax_no, vat_id, bank_name, account_holder, iban, bic, updated_at
        ) VALUES (
            'default', $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
            $11,$12,$13,$14,$15,$16,$17, now()
        )
        ON CONFLICT (id) DO UPDATE SET
            name=EXCLUDED.name,
            legal_form=EXCLUDED.legal_form,
            branch_name=EXCLUDED.branch_name,
            street=EXCLUDED.street,
            postal_code=EXCLUDED.postal_code,
            city=EXCLUDED.city,
            country=EXCLUDED.country,
            email=EXCLUDED.email,
            phone=EXCLUDED.phone,
            website=EXCLUDED.website,
            invoice_email=EXCLUDED.invoice_email,
            tax_no=EXCLUDED.tax_no,
            vat_id=EXCLUDED.vat_id,
            bank_name=EXCLUDED.bank_name,
            account_holder=EXCLUDED.account_holder,
            iban=EXCLUDED.iban,
            bic=EXCLUDED.bic,
            updated_at=now()
    `, in.Name, in.LegalForm, in.BranchName, in.Street, in.PostalCode, in.City, in.Country, in.Email, in.Phone,
        in.Website, in.InvoiceEmail, in.TaxNo, in.VatID, in.BankName, in.AccountHolder, in.IBAN, in.BIC)
    return err
}

func normalizeCountry(s string) string {
    s = strings.ToUpper(trim(s))
    if s == "" {
        return "DE"
    }
    return s
}

func normalizeCompactUpper(s string) string {
    s = strings.ToUpper(trim(s))
    return strings.ReplaceAll(s, " ", "")
}

func (s *CompanyService) ListBranches(ctx context.Context) ([]CompanyBranch, error) {
    rows, err := s.pg.Query(ctx, `
        SELECT id, company_id, code, name, street, postal_code, city, country, email, phone, is_default, created_at, updated_at
        FROM company_branches
        WHERE company_id='default'
        ORDER BY is_default DESC, name ASC, created_at ASC
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    out := make([]CompanyBranch, 0)
    for rows.Next() {
        var it CompanyBranch
        if err := rows.Scan(
            &it.ID, &it.CompanyID, &it.Code, &it.Name, &it.Street, &it.PostalCode, &it.City,
            &it.Country, &it.Email, &it.Phone, &it.IsDefault, &it.CreatedAt, &it.UpdatedAt,
        ); err != nil {
            return nil, err
        }
        out = append(out, it)
    }
    return out, nil
}

func (s *CompanyService) CreateBranch(ctx context.Context, in CompanyBranch) (*CompanyBranch, error) {
    in.Name = trim(in.Name)
    if in.Name == "" {
        return nil, errors.New("Niederlassungsname erforderlich")
    }
    in.Code = trim(in.Code)
    in.Street = trim(in.Street)
    in.PostalCode = trim(in.PostalCode)
    in.City = trim(in.City)
    in.Country = normalizeCountry(in.Country)
    in.Email = trim(in.Email)
    in.Phone = trim(in.Phone)
    if err := s.ensureBranchCodeUnique(ctx, "", in.Code); err != nil {
        return nil, err
    }
    if in.IsDefault {
        if _, err := s.pg.Exec(ctx, `UPDATE company_branches SET is_default=false WHERE company_id='default'`); err != nil {
            return nil, err
        }
    }
    id := strings.ReplaceAll(strings.ToLower(time.Now().UTC().Format("20060102150405.000000000")), ".", "")
    id = "branch-" + id
    _, err := s.pg.Exec(ctx, `
        INSERT INTO company_branches (
            id, company_id, code, name, street, postal_code, city, country, email, phone, is_default, created_at, updated_at
        ) VALUES (
            $1, 'default', $2,$3,$4,$5,$6,$7,$8,$9,$10, now(), now()
        )
    `, id, in.Code, in.Name, in.Street, in.PostalCode, in.City, in.Country, in.Email, in.Phone, in.IsDefault)
    if err != nil {
        return nil, err
    }
    return s.getBranch(ctx, id)
}

func (s *CompanyService) UpdateBranch(ctx context.Context, id string, in CompanyBranch) (*CompanyBranch, error) {
    id = trim(id)
    if id == "" {
        return nil, errors.New("ID erforderlich")
    }
    current, err := s.getBranch(ctx, id)
    if err != nil {
        return nil, err
    }
    code := trim(in.Code)
    name := trim(in.Name)
    if name == "" {
        name = current.Name
    }
    if name == "" {
        return nil, errors.New("Niederlassungsname erforderlich")
    }
    if code == "" {
        code = current.Code
    }
    if err := s.ensureBranchCodeUnique(ctx, id, code); err != nil {
        return nil, err
    }
    isDefault := in.IsDefault
    if isDefault {
        if _, err := s.pg.Exec(ctx, `UPDATE company_branches SET is_default=false WHERE company_id='default' AND id<>$1`, id); err != nil {
            return nil, err
        }
    }
    _, err = s.pg.Exec(ctx, `
        UPDATE company_branches
        SET code=$2, name=$3, street=$4, postal_code=$5, city=$6, country=$7, email=$8, phone=$9, is_default=$10, updated_at=now()
        WHERE id=$1
    `, id, code, name,
        firstNonEmpty(trim(in.Street), current.Street),
        firstNonEmpty(trim(in.PostalCode), current.PostalCode),
        firstNonEmpty(trim(in.City), current.City),
        normalizeCountry(firstNonEmpty(in.Country, current.Country)),
        firstNonEmpty(trim(in.Email), current.Email),
        firstNonEmpty(trim(in.Phone), current.Phone),
        isDefault,
    )
    if err != nil {
        return nil, err
    }
    return s.getBranch(ctx, id)
}

func (s *CompanyService) DeleteBranch(ctx context.Context, id string) error {
    id = trim(id)
    if id == "" {
        return errors.New("ID erforderlich")
    }
    cmd, err := s.pg.Exec(ctx, `DELETE FROM company_branches WHERE id=$1`, id)
    if err != nil {
        return err
    }
    if cmd.RowsAffected() == 0 {
        return errors.New("Niederlassung nicht gefunden")
    }
    return nil
}

func (s *CompanyService) getBranch(ctx context.Context, id string) (*CompanyBranch, error) {
    var it CompanyBranch
    err := s.pg.QueryRow(ctx, `
        SELECT id, company_id, code, name, street, postal_code, city, country, email, phone, is_default, created_at, updated_at
        FROM company_branches
        WHERE id=$1
    `, id).Scan(
        &it.ID, &it.CompanyID, &it.Code, &it.Name, &it.Street, &it.PostalCode, &it.City,
        &it.Country, &it.Email, &it.Phone, &it.IsDefault, &it.CreatedAt, &it.UpdatedAt,
    )
    if err != nil {
        return nil, err
    }
    return &it, nil
}

func (s *CompanyService) ensureBranchCodeUnique(ctx context.Context, excludeID, code string) error {
    code = strings.ToLower(trim(code))
    if code == "" {
        return nil
    }
    var existingID string
    err := s.pg.QueryRow(ctx, `
        SELECT id
        FROM company_branches
        WHERE company_id='default' AND lower(btrim(code))=$1 AND id<>$2
        LIMIT 1
    `, code, excludeID).Scan(&existingID)
    if err == nil && existingID != "" {
        return errors.New("Niederlassungscode bereits vorhanden")
    }
    return nil
}

func firstNonEmpty(v, fallback string) string {
    if trim(v) != "" {
        return trim(v)
    }
    return fallback
}
