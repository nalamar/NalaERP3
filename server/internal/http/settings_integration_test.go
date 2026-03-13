package apihttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nalaerp3/internal/testutil"
)

func TestCompanyProfileAndBranchesSettingsFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-settings@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-settings@example.com", "Secret123!")

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/company/", nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initial company profile, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	updateBody := []byte(`{
		"name":"Nala Metallbau GmbH",
		"legal_form":"GmbH",
		"branch_name":"Zentrale",
		"street":"Werkstraße 10",
		"postal_code":"12345",
		"city":"Berlin",
		"country":"at",
		"email":"info@nala.example",
		"phone":"+49 30 123456",
		"website":"https://nala.example",
		"invoice_email":"rechnung@nala.example",
		"tax_no":"12/345/67890",
		"vat_id":"de123456789",
		"bank_name":"Musterbank",
		"account_holder":"Nala Metallbau GmbH",
		"iban":"de12 3456 7890 1234 5678 90",
		"bic":"testdeff"
	}`)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/company/", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for company update, got %d with body %s", updateRec.Code, updateRec.Body.String())
	}

	getAfterReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/company/", nil)
	getAfterReq.Header.Set("Authorization", "Bearer "+accessToken)
	getAfterRec := httptest.NewRecorder()
	handler.ServeHTTP(getAfterRec, getAfterReq)
	if getAfterRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for updated company profile, got %d with body %s", getAfterRec.Code, getAfterRec.Body.String())
	}

	var profile struct {
		Name      string `json:"name"`
		Country   string `json:"country"`
		VatID     string `json:"vat_id"`
		IBAN      string `json:"iban"`
		BIC       string `json:"bic"`
		BankName  string `json:"bank_name"`
		Branch    string `json:"branch_name"`
		City      string `json:"city"`
		TaxNo     string `json:"tax_no"`
		Email     string `json:"email"`
		InvoiceEM string `json:"invoice_email"`
	}
	if err := json.Unmarshal(getAfterRec.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode company profile: %v", err)
	}
	if profile.Name != "Nala Metallbau GmbH" {
		t.Fatalf("expected company name, got %q", profile.Name)
	}
	if profile.Country != "AT" {
		t.Fatalf("expected normalized country AT, got %q", profile.Country)
	}
	if profile.VatID != "DE123456789" {
		t.Fatalf("expected normalized vat id, got %q", profile.VatID)
	}
	if profile.IBAN != "DE12345678901234567890" {
		t.Fatalf("expected compact iban, got %q", profile.IBAN)
	}
	if profile.BIC != "TESTDEFF" {
		t.Fatalf("expected uppercase bic, got %q", profile.BIC)
	}

	createBranchBody := []byte(`{
		"code":"HQ",
		"name":"Hauptsitz",
		"street":"Werkstraße 10",
		"postal_code":"12345",
		"city":"Berlin",
		"country":"de",
		"email":"berlin@nala.example",
		"phone":"+49 30 123456",
		"is_default":true
	}`)
	createBranchReq := httptest.NewRequest(http.MethodPost, "/api/v1/settings/company/branches", bytes.NewReader(createBranchBody))
	createBranchReq.Header.Set("Authorization", "Bearer "+accessToken)
	createBranchReq.Header.Set("Content-Type", "application/json")
	createBranchRec := httptest.NewRecorder()
	handler.ServeHTTP(createBranchRec, createBranchReq)
	if createBranchRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for branch create, got %d with body %s", createBranchRec.Code, createBranchRec.Body.String())
	}

	var createdBranch struct {
		ID        string `json:"id"`
		Code      string `json:"code"`
		Name      string `json:"name"`
		Country   string `json:"country"`
		IsDefault bool   `json:"is_default"`
	}
	if err := json.Unmarshal(createBranchRec.Body.Bytes(), &createdBranch); err != nil {
		t.Fatalf("decode branch create response: %v", err)
	}
	if createdBranch.ID == "" {
		t.Fatal("expected branch id")
	}
	if createdBranch.Country != "DE" {
		t.Fatalf("expected normalized branch country DE, got %q", createdBranch.Country)
	}
	if !createdBranch.IsDefault {
		t.Fatal("expected default branch")
	}

	listBranchReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/company/branches", nil)
	listBranchReq.Header.Set("Authorization", "Bearer "+accessToken)
	listBranchRec := httptest.NewRecorder()
	handler.ServeHTTP(listBranchRec, listBranchReq)
	if listBranchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for branch list, got %d with body %s", listBranchRec.Code, listBranchRec.Body.String())
	}

	var branches []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		City      string `json:"city"`
		IsDefault bool   `json:"is_default"`
	}
	if err := json.Unmarshal(listBranchRec.Body.Bytes(), &branches); err != nil {
		t.Fatalf("decode branch list: %v", err)
	}
	if len(branches) == 0 {
		t.Fatal("expected at least one branch")
	}

	updateBranchBody := []byte(`{
		"code":"HQ-BER",
		"name":"Hauptsitz Berlin",
		"city":"Potsdam",
		"country":"ch",
		"is_default":true
	}`)
	updateBranchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/company/branches/"+createdBranch.ID, bytes.NewReader(updateBranchBody))
	updateBranchReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateBranchReq.Header.Set("Content-Type", "application/json")
	updateBranchRec := httptest.NewRecorder()
	handler.ServeHTTP(updateBranchRec, updateBranchReq)
	if updateBranchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for branch update, got %d with body %s", updateBranchRec.Code, updateBranchRec.Body.String())
	}

	var updatedBranch struct {
		ID        string `json:"id"`
		Code      string `json:"code"`
		Name      string `json:"name"`
		City      string `json:"city"`
		Country   string `json:"country"`
		IsDefault bool   `json:"is_default"`
	}
	if err := json.Unmarshal(updateBranchRec.Body.Bytes(), &updatedBranch); err != nil {
		t.Fatalf("decode branch update response: %v", err)
	}
	if updatedBranch.Code != "HQ-BER" || updatedBranch.Name != "Hauptsitz Berlin" {
		t.Fatalf("unexpected updated branch %#v", updatedBranch)
	}
	if updatedBranch.City != "Potsdam" || updatedBranch.Country != "CH" {
		t.Fatalf("expected updated city/country, got %#v", updatedBranch)
	}

	deleteBranchReq := httptest.NewRequest(http.MethodDelete, "/api/v1/settings/company/branches/"+createdBranch.ID, nil)
	deleteBranchReq.Header.Set("Authorization", "Bearer "+accessToken)
	deleteBranchRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteBranchRec, deleteBranchReq)
	if deleteBranchRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for branch delete, got %d with body %s", deleteBranchRec.Code, deleteBranchRec.Body.String())
	}

	listAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/company/branches", nil)
	listAfterDeleteReq.Header.Set("Authorization", "Bearer "+accessToken)
	listAfterDeleteRec := httptest.NewRecorder()
	handler.ServeHTTP(listAfterDeleteRec, listAfterDeleteReq)
	if listAfterDeleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for branch list after delete, got %d with body %s", listAfterDeleteRec.Code, listAfterDeleteRec.Body.String())
	}
	if err := json.Unmarshal(listAfterDeleteRec.Body.Bytes(), &branches); err != nil {
		t.Fatalf("decode branch list after delete: %v", err)
	}
	if len(branches) != 0 {
		t.Fatalf("expected no branches after delete, got %#v", branches)
	}
}

func TestCompanyLocalizationSettingsFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-settings-localization@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-settings-localization@example.com", "Secret123!")

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/company/localization", nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initial localization settings, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var initial struct {
		DefaultCurrency string  `json:"default_currency"`
		TaxCountry      string  `json:"tax_country"`
		StandardVATRate float64 `json:"standard_vat_rate"`
		Locale          string  `json:"locale"`
		Timezone        string  `json:"timezone"`
		DateFormat      string  `json:"date_format"`
		NumberFormat    string  `json:"number_format"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode initial localization: %v", err)
	}
	if initial.DefaultCurrency != "EUR" {
		t.Fatalf("expected default currency EUR, got %q", initial.DefaultCurrency)
	}
	if initial.TaxCountry != "DE" {
		t.Fatalf("expected default tax country DE, got %q", initial.TaxCountry)
	}

	updateBody := []byte(`{
		"default_currency":"chf",
		"tax_country":"at",
		"standard_vat_rate":20.0,
		"locale":"de-AT",
		"timezone":"Europe/Vienna",
		"date_format":"yyyy-MM-dd",
		"number_format":"de-AT"
	}`)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/company/localization", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for localization update, got %d with body %s", updateRec.Code, updateRec.Body.String())
	}

	getAfterReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/company/localization", nil)
	getAfterReq.Header.Set("Authorization", "Bearer "+accessToken)
	getAfterRec := httptest.NewRecorder()
	handler.ServeHTTP(getAfterRec, getAfterReq)
	if getAfterRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for updated localization settings, got %d with body %s", getAfterRec.Code, getAfterRec.Body.String())
	}

	var updated struct {
		DefaultCurrency string  `json:"default_currency"`
		TaxCountry      string  `json:"tax_country"`
		StandardVATRate float64 `json:"standard_vat_rate"`
		Locale          string  `json:"locale"`
		Timezone        string  `json:"timezone"`
		DateFormat      string  `json:"date_format"`
		NumberFormat    string  `json:"number_format"`
	}
	if err := json.Unmarshal(getAfterRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated localization: %v", err)
	}
	if updated.DefaultCurrency != "CHF" {
		t.Fatalf("expected normalized currency CHF, got %q", updated.DefaultCurrency)
	}
	if updated.TaxCountry != "AT" {
		t.Fatalf("expected normalized tax country AT, got %q", updated.TaxCountry)
	}
	if updated.StandardVATRate != 20.0 {
		t.Fatalf("expected vat rate 20.0, got %v", updated.StandardVATRate)
	}
	if updated.Locale != "de-AT" {
		t.Fatalf("expected locale de-AT, got %q", updated.Locale)
	}
	if updated.Timezone != "Europe/Vienna" {
		t.Fatalf("expected timezone Europe/Vienna, got %q", updated.Timezone)
	}
	if updated.DateFormat != "yyyy-MM-dd" {
		t.Fatalf("expected date format yyyy-MM-dd, got %q", updated.DateFormat)
	}
	if updated.NumberFormat != "de-AT" {
		t.Fatalf("expected number format de-AT, got %q", updated.NumberFormat)
	}
}

func TestCompanyBrandingSettingsFlow(t *testing.T) {
	env := testutil.SetupIntegrationEnv(t)
	testutil.SeedAuthUser(t, env, "integration-settings-branding@example.com", "Secret123!", "admin")

	handler := NewRouterWithDeps(env.PG, env.Mongo, env.Redis, env.Cfg)
	accessToken := loginIntegrationUser(t, handler, "integration-settings-branding@example.com", "Secret123!")

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/company/branding", nil)
	getReq.Header.Set("Authorization", "Bearer "+accessToken)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for initial branding settings, got %d with body %s", getRec.Code, getRec.Body.String())
	}

	var initial struct {
		DisplayName        string `json:"display_name"`
		PrimaryColor       string `json:"primary_color"`
		AccentColor        string `json:"accent_color"`
		DocumentHeaderText string `json:"document_header_text"`
		DocumentFooterText string `json:"document_footer_text"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode initial branding settings: %v", err)
	}
	if initial.PrimaryColor != "#1F4B99" {
		t.Fatalf("expected default primary color #1F4B99, got %q", initial.PrimaryColor)
	}
	if initial.AccentColor != "#6B7280" {
		t.Fatalf("expected default accent color #6B7280, got %q", initial.AccentColor)
	}

	updateBody := []byte(`{
		"display_name":"NALA Metallbau",
		"claim":"Praezision in Metall",
		"primary_color":"1f4b99",
		"accent_color":"#d97706",
		"document_header_text":"Standard-Kopf fuer alle Dokumente",
		"document_footer_text":"Standard-Fusstext fuer alle Dokumente"
	}`)
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/settings/company/branding", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for branding update, got %d with body %s", updateRec.Code, updateRec.Body.String())
	}

	getAfterReq := httptest.NewRequest(http.MethodGet, "/api/v1/settings/company/branding", nil)
	getAfterReq.Header.Set("Authorization", "Bearer "+accessToken)
	getAfterRec := httptest.NewRecorder()
	handler.ServeHTTP(getAfterRec, getAfterReq)
	if getAfterRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for updated branding settings, got %d with body %s", getAfterRec.Code, getAfterRec.Body.String())
	}

	var updated struct {
		DisplayName        string `json:"display_name"`
		Claim              string `json:"claim"`
		PrimaryColor       string `json:"primary_color"`
		AccentColor        string `json:"accent_color"`
		DocumentHeaderText string `json:"document_header_text"`
		DocumentFooterText string `json:"document_footer_text"`
	}
	if err := json.Unmarshal(getAfterRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated branding settings: %v", err)
	}
	if updated.DisplayName != "NALA Metallbau" {
		t.Fatalf("expected display name, got %q", updated.DisplayName)
	}
	if updated.Claim != "Praezision in Metall" {
		t.Fatalf("expected claim, got %q", updated.Claim)
	}
	if updated.PrimaryColor != "#1F4B99" {
		t.Fatalf("expected normalized primary color #1F4B99, got %q", updated.PrimaryColor)
	}
	if updated.AccentColor != "#D97706" {
		t.Fatalf("expected normalized accent color #D97706, got %q", updated.AccentColor)
	}
	if updated.DocumentHeaderText != "Standard-Kopf fuer alle Dokumente" {
		t.Fatalf("expected header text, got %q", updated.DocumentHeaderText)
	}
	if updated.DocumentFooterText != "Standard-Fusstext fuer alle Dokumente" {
		t.Fatalf("expected footer text, got %q", updated.DocumentFooterText)
	}
}
