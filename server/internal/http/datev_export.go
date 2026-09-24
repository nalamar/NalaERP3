package apihttp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"nalaerp3/internal/accounting"
	"nalaerp3/internal/settings"
)

// buildDatevExportFile bündelt die für E.3.3.3 nötigen Schritte: Firmenprofil
// (Berater-/Mandantennummer usw.) laden, Wirtschaftsjahresbeginn auflösen,
// Buchungen laden (inkl. Gegenkonto-Auflösung, E.3.3.2), CSV-Datei bauen
// (E.3.3.1). Reine Orchestrierung, keine eigene Geschäftslogik - dient dem
// GET /accounting/datev-export-Handler in v1.go.
func buildDatevExportFile(
	ctx context.Context,
	companyID string,
	von, bis time.Time,
	companySvc *settings.CompanyService,
	exportSvc *accounting.DatevExportService,
) (content []byte, filename string, err error) {
	profile, err := companySvc.Get(ctx, companyID)
	if err != nil {
		return nil, "", err
	}
	if profile.DatevBeraterNr == nil || profile.DatevMandantNr == nil {
		return nil, "", errors.New("DATEV-Beraternummer und -Mandantennummer erforderlich, bitte zuerst in den Firmenprofil-Einstellungen hinterlegen")
	}

	rows, err := exportSvc.LoadBookingRows(ctx, companyID, von, bis)
	if err != nil {
		return nil, "", err
	}

	header := accounting.DatevHeaderInput{
		Berater:          *profile.DatevBeraterNr,
		Mandant:          *profile.DatevMandantNr,
		FiscalYearStart:  resolveDatevFiscalYearStart(von, profile.DatevFiscalYearStartMonth),
		SKR:              profile.DatevSKR,
		Sachkontenlaenge: profile.DatevSachkontenlaenge,
		DatumVon:         von,
		DatumBis:         bis,
		Bezeichnung:      fmt.Sprintf("Buchungsstapel %s-%s", von.Format("02.01.2006"), bis.Format("02.01.2006")),
		ExportiertVon:    profile.Name,
		ErzeugtAm:        time.Now(),
	}

	content, err = accounting.BuildDatevBuchungsstapel(header, rows)
	if err != nil {
		return nil, "", err
	}

	filename = fmt.Sprintf("EXTF_Buchungsstapel_%s_%s.csv", von.Format("20060102"), bis.Format("20060102"))
	return content, filename, nil
}

// resolveDatevFiscalYearStart löst den in company_profiles nur als Monat
// konfigurierten Wirtschaftsjahresbeginn (ADR 0021 - Tag fix der 1.) auf das
// konkrete Datum des zum Exportzeitraum gehörenden Wirtschaftsjahres auf:
// das jüngste Vorkommen dieses Monats/1. auf oder vor `von`.
func resolveDatevFiscalYearStart(von time.Time, startMonth int) time.Time {
	if startMonth < 1 || startMonth > 12 {
		startMonth = 1
	}
	year := von.Year()
	if int(von.Month()) < startMonth {
		year--
	}
	return time.Date(year, time.Month(startMonth), 1, 0, 0, 0, 0, time.UTC)
}
