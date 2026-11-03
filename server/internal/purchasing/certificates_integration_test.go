package purchasing

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"nalaerp3/internal/testutil"
)

func wantErr(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil || err.Error() != msg {
		t.Fatalf("Fehler %v, erwartet %q", err, msg)
	}
}

// Zertifikate am Lieferanten im Service (Backlog H.4.2a).
func TestSupplierCertificates(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	s := seedEvidence(t, env)
	other := seedEvidence(t, env)
	svc := NewCertificateService(env.PG, env.Mongo, env.Cfg.MongoDB)
	svc.now = func() time.Time { return time.Date(2031, 11, 3, 15, 0, 0, 0, time.UTC) } // nicht heute: die Uhr des Service zaehlt

	c, err := svc.Create(ctx, " "+s.supplier+" ", CertificateInput{Art: " EN1090 ", Nummer: " 1090-CPR-123 ", Stelle: " Prüfstelle ", ExcBis: sp("exc2"), GueltigBis: d("2031-12-31")}, "", s.company)
	if err != nil {
		t.Fatal(err)
	}
	if c.Art != "en1090" || c.ExcBis == nil || *c.ExcBis != "EXC2" || c.Nummer != "1090-CPR-123" || c.Stelle != "Prüfstelle" || c.SupplierID != s.supplier {
		t.Fatalf("angelegt: %+v", c)
	}
	_, err = svc.Create(ctx, s.supplier, CertificateInput{Art: "en1090"}, "", s.company)
	wantErr(t, err, "Ausführungsklasse erforderlich")
	_, err = svc.Create(ctx, s.supplier, CertificateInput{Art: "iso3834"}, "", other.company)
	wantErr(t, err, "Lieferant nicht gefunden")
	_, err = svc.Create(ctx, "kein-kontakt", CertificateInput{Art: "iso3834"}, "", s.company)
	wantErr(t, err, "Lieferant nicht gefunden")
	customer := s.supplier + "-kunde"
	if _, err := env.PG.Exec(ctx, `INSERT INTO contacts (id, typ, rolle, name, company_id) VALUES ($1,'org','customer','Kunde',$2)`, customer, s.company); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Create(ctx, customer, CertificateInput{Art: "iso3834"}, "", s.company)
	wantErr(t, err, "Lieferant nicht gefunden")

	// ISO 3834 ohne Ende; ein zweites EN 1090 endet in 30 Tagen (3.12.),
	// das erste (bis 31.12.) gilt darueber hinaus.
	iso, err := svc.Create(ctx, s.supplier, CertificateInput{Art: "iso3834"}, "", s.company)
	if err != nil {
		t.Fatal(err)
	}
	soon, err := svc.Create(ctx, s.supplier, CertificateInput{Art: "en1090", ExcBis: sp("EXC3"), GueltigBis: d("2031-12-03")}, "", s.company)
	if err != nil {
		t.Fatal(err)
	}
	list, err := svc.List(ctx, s.supplier, false, s.company)
	if err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{}
	for _, x := range list {
		ids = append(ids, x.ID)
	}
	if !slices.Equal(ids, []uuid.UUID{iso.ID, c.ID, soon.ID}) {
		t.Fatalf("Reihenfolge %v", ids)
	}
	// soon endet am 3.12., c gilt am 4.12. weiter: kein Hinweis fuer soon;
	// c endet am 31.12. ohne Nachfolger: Hinweis.
	if !slices.Equal(list[1].Hinweise, []string{HintZertifikatLaeuftAb}) || len(list[2].Hinweise) != 0 || len(list[0].Hinweise) != 0 {
		t.Fatalf("Hinweise %v / %v / %v", list[0].Hinweise, list[1].Hinweise, list[2].Hinweise)
	}
	if l, err := svc.List(ctx, s.supplier, false, other.company); err == nil || len(l) != 0 {
		t.Fatalf("fremder Mandant: %v, %v", l, err)
	}

	// Storno: Grund Pflicht, einmalig, Angaben bleiben; danach rechnet der
	// Hinweis ohne c - soon laeuft dann ohne Nachfolger ab.
	_, err = svc.Storno(ctx, c.ID, " ", "", s.company)
	wantErr(t, err, "Storno-Begründung erforderlich")
	_, err = svc.Storno(ctx, c.ID, "falsch", "", other.company)
	wantErr(t, err, "Zertifikat nicht gefunden")
	st, err := svc.Storno(ctx, c.ID, " falscher Lieferant ", "", s.company)
	if err != nil {
		t.Fatal(err)
	}
	if st.StorniertAt == nil || st.StornoReason != "falscher Lieferant" || st.Nummer != "1090-CPR-123" || st.GueltigBis == nil {
		t.Fatalf("storniert: %+v", st)
	}
	_, err = svc.Storno(ctx, c.ID, "nochmal", "", s.company)
	wantErr(t, err, "Zertifikat ist bereits storniert")
	list, _ = svc.List(ctx, s.supplier, false, s.company)
	if len(list) != 2 || list[1].ID != soon.ID || !slices.Equal(list[1].Hinweise, []string{HintZertifikatLaeuftAb}) {
		t.Fatalf("nach Storno: %+v", list)
	}
	list, _ = svc.List(ctx, s.supplier, true, s.company)
	if len(list) != 3 || list[1].ID != c.ID || len(list[1].Hinweise) != 0 {
		t.Fatalf("mit stornierten: %+v", list)
	}
	_, err = svc.Get(ctx, soon.ID, other.company)
	wantErr(t, err, "Zertifikat nicht gefunden")
}

// Dateien zum Zertifikat (Backlog H.4.2a).
func TestSupplierCertificateDocuments(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	ctx := context.Background()
	s := seedEvidence(t, env)
	other := seedEvidence(t, env)
	svc := NewCertificateService(env.PG, env.Mongo, env.Cfg.MongoDB)
	c, err := svc.Create(ctx, s.supplier, CertificateInput{Art: "iso3834"}, "", s.company)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := svc.Create(ctx, s.supplier, CertificateInput{Art: "sonstiges"}, "", s.company)
	if err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.4\nNachweis\n")
	doc, err := svc.UploadDocument(ctx, c.ID, bytes.NewReader(pdf), `C:\Scans\zert.pdf`, "", s.company)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Filename != "zert.pdf" || doc.ContentType != "application/pdf" || doc.Length != int64(len(pdf)) {
		t.Fatalf("hochgeladen: %+v", doc)
	}
	_, err = svc.UploadDocument(ctx, c.ID, strings.NewReader("nur Text"), "x.txt", "", s.company)
	if !errors.Is(err, ErrEvidenceDocumentType) {
		t.Fatalf("Text: %v", err)
	}
	_, err = svc.UploadDocument(ctx, c.ID, strings.NewReader(""), "x.pdf", "", s.company)
	wantErr(t, err, "Datei erforderlich")
	big := append([]byte("%PDF-1.4\n"), make([]byte, MaxEvidenceDocumentBytes)...)
	_, err = svc.UploadDocument(ctx, c.ID, bytes.NewReader(big), "gross.pdf", "", s.company)
	if !errors.Is(err, ErrEvidenceTooLarge) {
		t.Fatalf("zu gross: %v", err)
	}
	_, err = svc.UploadDocument(ctx, c.ID, bytes.NewReader(pdf), "x.pdf", "", other.company)
	wantErr(t, err, "Zertifikat nicht gefunden")

	docs, err := svc.ListDocuments(ctx, c.ID, s.company)
	if err != nil || len(docs) != 1 || docs[0].ID != doc.ID {
		t.Fatalf("Liste: %v, %v", docs, err)
	}
	if d2, err := svc.ListDocuments(ctx, c2.ID, s.company); err != nil || len(d2) != 0 {
		t.Fatalf("anderes Zertifikat: %v, %v", d2, err)
	}
	rc, meta, err := svc.OpenDocument(ctx, c.ID, doc.ID, s.company)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, pdf) || meta.Filename != "zert.pdf" {
		t.Fatalf("Download: %q, %+v", got, meta)
	}
	_, _, err = svc.OpenDocument(ctx, c2.ID, doc.ID, s.company)
	wantErr(t, err, "Dokument nicht gefunden")
	_, _, err = svc.OpenDocument(ctx, c.ID, doc.ID, other.company)
	wantErr(t, err, "Zertifikat nicht gefunden")

	if _, err := svc.Storno(ctx, c.ID, "ersetzt", "", s.company); err != nil {
		t.Fatal(err)
	}
	_, err = svc.UploadDocument(ctx, c.ID, bytes.NewReader(pdf), "x.pdf", "", s.company)
	wantErr(t, err, "Zertifikat ist storniert")
	// Dateien eines stornierten Zertifikats bleiben lesbar.
	if docs, err := svc.ListDocuments(ctx, c.ID, s.company); err != nil || len(docs) != 1 {
		t.Fatalf("nach Storno: %v, %v", docs, err)
	}

	noMongo := NewCertificateService(env.PG, nil, "")
	_, err = noMongo.UploadDocument(ctx, c2.ID, bytes.NewReader(pdf), "x.pdf", "", s.company)
	wantErr(t, err, "MongoDB nicht konfiguriert")
}
