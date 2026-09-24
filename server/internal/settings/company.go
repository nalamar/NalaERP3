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

	// DATEV-Export (ADR 0021, Backlog E.3). DatevBeraterNr/DatevMandantNr
	// bleiben nullable - der Export prueft explizit auf deren Vorhandensein
	// statt einen Dummy-Wert zu exportieren.
	DatevBeraterNr            *int   `json:"datev_berater_nr"`
	DatevMandantNr            *int   `json:"datev_mandant_nr"`
	DatevSKR                  string `json:"datev_skr"`
	DatevSachkontenlaenge     int    `json:"datev_sachkontenlaenge"`
	DatevFiscalYearStartMonth int    `json:"datev_fiscal_year_start_month"`
}

type CompanyBranch struct {
	ID         string    `json:"id"`
	CompanyID  string    `json:"company_id"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	Street     string    `json:"street"`
	PostalCode string    `json:"postal_code"`
	City       string    `json:"city"`
	Country    string    `json:"country"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	IsDefault  bool      `json:"is_default"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CompanyService struct{ pg *pgxpool.Pool }

func NewCompanyService(pg *pgxpool.Pool) *CompanyService { return &CompanyService{pg: pg} }

func (s *CompanyService) Get(ctx context.Context, companyID string) (*CompanyProfile, error) {
	var out CompanyProfile
	err := s.pg.QueryRow(ctx, `
        SELECT id, name, legal_form, branch_name, street, postal_code, city, country, email, phone, website,
               invoice_email, tax_no, vat_id, bank_name, account_holder, iban, bic, updated_at,
               datev_berater_nr, datev_mandant_nr, datev_skr, datev_sachkontenlaenge, datev_fiscal_year_start_month
        FROM company_profiles
        WHERE id=$1
    `, companyID).Scan(
		&out.ID, &out.Name, &out.LegalForm, &out.BranchName, &out.Street, &out.PostalCode, &out.City, &out.Country,
		&out.Email, &out.Phone, &out.Website, &out.InvoiceEmail, &out.TaxNo, &out.VatID, &out.BankName,
		&out.AccountHolder, &out.IBAN, &out.BIC, &out.UpdatedAt,
		&out.DatevBeraterNr, &out.DatevMandantNr, &out.DatevSKR, &out.DatevSachkontenlaenge, &out.DatevFiscalYearStartMonth,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *CompanyService) Upsert(ctx context.Context, in CompanyProfile, companyID string) error {
	if strings.TrimSpace(companyID) == "" {
		return errors.New("Mandant erforderlich")
	}
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
            $1, $2,$3,$4,$5,$6,$7,$8,$9,$10,$11,
            $12,$13,$14,$15,$16,$17,$18, now()
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
    `, companyID, in.Name, in.LegalForm, in.BranchName, in.Street, in.PostalCode, in.City, in.Country, in.Email, in.Phone,
		in.Website, in.InvoiceEmail, in.TaxNo, in.VatID, in.BankName, in.AccountHolder, in.IBAN, in.BIC)
	return err
}

// DatevSettingsInput sind die fuenf DATEV-Exportkonfigurationsfelder
// (ADR 0021, Backlog E.3). Eigene, enge Update-Funktion statt Erweiterung
// des generischen Upsert-Vollersatzes (analog zur SetKostenstelle-
// Entscheidung aus E.2.3.1): ein Client, der die restlichen 18
// CompanyProfile-Felder nicht kennt, wuerde bei einem Vollersatz sonst
// versehentlich Berater-/Mandantennummer o.ae. zuruecksetzen.
type DatevSettingsInput struct {
	BeraterNr            *int   `json:"datev_berater_nr"`
	MandantNr            *int   `json:"datev_mandant_nr"`
	SKR                  string `json:"datev_skr"`
	Sachkontenlaenge     int    `json:"datev_sachkontenlaenge"`
	FiscalYearStartMonth int    `json:"datev_fiscal_year_start_month"`
}

func (s *CompanyService) UpdateDatevSettings(ctx context.Context, in DatevSettingsInput, companyID string) (*CompanyProfile, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	if in.BeraterNr != nil && *in.BeraterNr < 1001 {
		return nil, errors.New("DATEV-Beraternummer muss mindestens 1001 sein")
	}
	if in.MandantNr != nil && *in.MandantNr <= 0 {
		return nil, errors.New("DATEV-Mandantennummer muss größer als 0 sein")
	}
	skr := trim(in.SKR)
	if skr == "" {
		skr = "04"
	}
	sachkontenlaenge := in.Sachkontenlaenge
	if sachkontenlaenge <= 0 {
		sachkontenlaenge = 4
	}
	fiscalMonth := in.FiscalYearStartMonth
	if fiscalMonth < 1 || fiscalMonth > 12 {
		fiscalMonth = 1
	}

	_, err := s.pg.Exec(ctx, `
        UPDATE company_profiles
        SET datev_berater_nr=$2, datev_mandant_nr=$3, datev_skr=$4, datev_sachkontenlaenge=$5, datev_fiscal_year_start_month=$6, updated_at=now()
        WHERE id=$1
    `, companyID, in.BeraterNr, in.MandantNr, skr, sachkontenlaenge, fiscalMonth)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, companyID)
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

func (s *CompanyService) ListBranches(ctx context.Context, companyID string) ([]CompanyBranch, error) {
	rows, err := s.pg.Query(ctx, `
        SELECT id, company_id, code, name, street, postal_code, city, country, email, phone, is_default, created_at, updated_at
        FROM company_branches
        WHERE company_id=$1
        ORDER BY is_default DESC, name ASC, created_at ASC
    `, companyID)
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

func (s *CompanyService) CreateBranch(ctx context.Context, in CompanyBranch, companyID string) (*CompanyBranch, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
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
	if err := s.ensureBranchCodeUnique(ctx, "", in.Code, companyID); err != nil {
		return nil, err
	}
	if in.IsDefault {
		if _, err := s.pg.Exec(ctx, `UPDATE company_branches SET is_default=false WHERE company_id=$1`, companyID); err != nil {
			return nil, err
		}
	}
	id := strings.ReplaceAll(strings.ToLower(time.Now().UTC().Format("20060102150405.000000000")), ".", "")
	id = "branch-" + id
	_, err := s.pg.Exec(ctx, `
        INSERT INTO company_branches (
            id, company_id, code, name, street, postal_code, city, country, email, phone, is_default, created_at, updated_at
        ) VALUES (
            $1, $2, $3,$4,$5,$6,$7,$8,$9,$10,$11, now(), now()
        )
    `, id, companyID, in.Code, in.Name, in.Street, in.PostalCode, in.City, in.Country, in.Email, in.Phone, in.IsDefault)
	if err != nil {
		return nil, err
	}
	return s.getBranch(ctx, id, companyID)
}

func (s *CompanyService) UpdateBranch(ctx context.Context, id string, in CompanyBranch, companyID string) (*CompanyBranch, error) {
	id = trim(id)
	if id == "" {
		return nil, errors.New("ID erforderlich")
	}
	current, err := s.getBranch(ctx, id, companyID)
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
	if err := s.ensureBranchCodeUnique(ctx, id, code, companyID); err != nil {
		return nil, err
	}
	isDefault := in.IsDefault
	if isDefault {
		if _, err := s.pg.Exec(ctx, `UPDATE company_branches SET is_default=false WHERE company_id=$1 AND id<>$2`, companyID, id); err != nil {
			return nil, err
		}
	}
	_, err = s.pg.Exec(ctx, `
        UPDATE company_branches
        SET code=$3, name=$4, street=$5, postal_code=$6, city=$7, country=$8, email=$9, phone=$10, is_default=$11, updated_at=now()
        WHERE id=$1 AND company_id=$2
    `, id, companyID, code, name,
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
	return s.getBranch(ctx, id, companyID)
}

func (s *CompanyService) DeleteBranch(ctx context.Context, id string, companyID string) error {
	id = trim(id)
	if id == "" {
		return errors.New("ID erforderlich")
	}
	cmd, err := s.pg.Exec(ctx, `DELETE FROM company_branches WHERE id=$1 AND company_id=$2`, id, companyID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("Niederlassung nicht gefunden")
	}
	return nil
}

func (s *CompanyService) getBranch(ctx context.Context, id string, companyID string) (*CompanyBranch, error) {
	var it CompanyBranch
	err := s.pg.QueryRow(ctx, `
        SELECT id, company_id, code, name, street, postal_code, city, country, email, phone, is_default, created_at, updated_at
        FROM company_branches
        WHERE id=$1 AND company_id=$2
    `, id, companyID).Scan(
		&it.ID, &it.CompanyID, &it.Code, &it.Name, &it.Street, &it.PostalCode, &it.City,
		&it.Country, &it.Email, &it.Phone, &it.IsDefault, &it.CreatedAt, &it.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &it, nil
}

func (s *CompanyService) ensureBranchCodeUnique(ctx context.Context, excludeID, code string, companyID string) error {
	code = strings.ToLower(trim(code))
	if code == "" {
		return nil
	}
	var existingID string
	err := s.pg.QueryRow(ctx, `
        SELECT id
        FROM company_branches
        WHERE company_id=$1 AND lower(btrim(code))=$2 AND id<>$3
        LIMIT 1
    `, companyID, code, excludeID).Scan(&existingID)
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
