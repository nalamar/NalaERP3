package contacts

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SupplierProfileSeries (Systemlieferanten-Bindung) — Backlog A.3, siehe
// docs/adr/0008-systemlieferanten-konzept.md.
type SupplierProfileSeries struct {
	ID          string    `json:"id"`
	ContactID   string    `json:"contact_id"`
	Profilserie string    `json:"profilserie"`
	Notiz       string    `json:"notiz"`
	CreatedAt   time.Time `json:"created_at"`
}

type SupplierProfileSeriesCreate struct {
	Profilserie string `json:"profilserie"`
	Notiz       string `json:"notiz"`
}

// ensureSupplierRole prueft, dass der Kontakt contactID zum Mandanten
// companyID gehoert UND eine Lieferantenrolle hat (supplier|both) - nur
// solche Kontakte duerfen Profilserien gebunden bekommen (ADR 0008). Nutzt
// s.Get als Existenz- UND Ownership-Pruefung, analog zu CreateAddress.
// Vergleich case-insensitiv, da Contact.Rolle beim Anlegen NICHT auf
// Kleinschreibung normalisiert wird (nur isIn() prueft case-insensitiv,
// der gespeicherte Wert kann z. B. "Supplier" lauten) - bestehendes,
// unabhaengiges Verhalten von contacts.Create/Update, hier nur beruecksichtigt.
func (s *Service) ensureSupplierRole(ctx context.Context, contactID, companyID string) error {
	c, err := s.Get(ctx, contactID, companyID)
	if err != nil {
		return err
	}
	rolle := strings.ToLower(strings.TrimSpace(c.Rolle))
	if rolle != "supplier" && rolle != "both" {
		return errors.New("Ungültige Lieferantenrolle: Kontakt hat weder rolle=supplier noch rolle=both")
	}
	return nil
}

func (s *Service) CreateSupplierProfileSeries(ctx context.Context, contactID string, in SupplierProfileSeriesCreate, companyID string) (*SupplierProfileSeries, error) {
	profilserie := strings.TrimSpace(in.Profilserie)
	if profilserie == "" {
		return nil, errors.New("profilserie erforderlich")
	}
	if err := s.ensureSupplierRole(ctx, contactID, companyID); err != nil {
		return nil, err
	}
	var duplicate bool
	if err := s.pg.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM supplier_profile_series WHERE contact_id=$1 AND profilserie=$2)
    `, contactID, profilserie).Scan(&duplicate); err != nil {
		return nil, err
	}
	if duplicate {
		return nil, errors.New("Für diesen Lieferanten existiert bereits eine Bindung an diese Profilserie")
	}
	id := uuid.NewString()
	var row SupplierProfileSeries
	err := s.pg.QueryRow(ctx, `
        INSERT INTO supplier_profile_series (id, contact_id, profilserie, notiz)
        VALUES ($1,$2,$3,$4)
        RETURNING id, contact_id, profilserie, notiz, created_at
    `, id, contactID, profilserie, strings.TrimSpace(in.Notiz)).Scan(
		&row.ID, &row.ContactID, &row.Profilserie, &row.Notiz, &row.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) ListSupplierProfileSeries(ctx context.Context, contactID string, companyID string) ([]SupplierProfileSeries, error) {
	if _, err := s.Get(ctx, contactID, companyID); err != nil {
		return nil, err
	}
	rows, err := s.pg.Query(ctx, `
        SELECT id, contact_id, profilserie, notiz, created_at
        FROM supplier_profile_series WHERE contact_id=$1
        ORDER BY profilserie ASC
    `, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SupplierProfileSeries, 0)
	for rows.Next() {
		var row SupplierProfileSeries
		if err := rows.Scan(&row.ID, &row.ContactID, &row.Profilserie, &row.Notiz, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *Service) DeleteSupplierProfileSeries(ctx context.Context, contactID, seriesID string, companyID string) error {
	if _, err := s.Get(ctx, contactID, companyID); err != nil {
		return err
	}
	_, err := s.pg.Exec(ctx, `DELETE FROM supplier_profile_series WHERE contact_id=$1 AND id=$2`, contactID, seriesID)
	return err
}

// SupplierForProfileSeries ist ein Treffer aus ListSuppliersByProfileSeries.
type SupplierForProfileSeries struct {
	ID          string `json:"id"`
	ContactID   string `json:"contact_id"`
	ContactName string `json:"contact_name"`
	Profilserie string `json:"profilserie"`
	Notiz       string `json:"notiz"`
}

// ListSuppliersByProfileSeries ist der Reverse-Lookup: welche Lieferanten
// fuehren die angegebene Profilserie (die eigentliche Motivation hinter
// "Bindung" - nicht nur "welche Serien fuehrt Lieferant Y", siehe ADR 0008
// Konsequenzen). Exakter, getrimmter Textvergleich (kein ILIKE) - konsistent
// mit dem UNIQUE(contact_id, profilserie)-Constraint, der ebenfalls
// case-sensitiv ist.
func (s *Service) ListSuppliersByProfileSeries(ctx context.Context, profilserie string, companyID string) ([]SupplierForProfileSeries, error) {
	profilserie = strings.TrimSpace(profilserie)
	if profilserie == "" {
		return nil, errors.New("profilserie erforderlich")
	}
	rows, err := s.pg.Query(ctx, `
        SELECT sps.id, sps.contact_id, c.name, sps.profilserie, sps.notiz
        FROM supplier_profile_series sps
        JOIN contacts c ON c.id = sps.contact_id
        WHERE sps.profilserie = $1 AND c.company_id = $2
        ORDER BY c.name ASC
    `, profilserie, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SupplierForProfileSeries, 0)
	for rows.Next() {
		var row SupplierForProfileSeries
		if err := rows.Scan(&row.ID, &row.ContactID, &row.ContactName, &row.Profilserie, &row.Notiz); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}
