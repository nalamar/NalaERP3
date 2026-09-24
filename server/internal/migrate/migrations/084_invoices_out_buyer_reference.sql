-- E-Rechnung Ausgang (ADR 0022, Backlog E.4.2). Eine additive Spalte auf
-- invoices_out fuer die EN-16931-Kaeuferreferenz BT-10 ("BuyerReference"),
-- die im bestehenden Datenmodell nirgends existiert: bei B2G-Rechnungen die
-- Leitweg-ID des oeffentlichen Auftraggebers, bei B2B die vom Kunden
-- geforderte Bestell-/Kundenreferenz.
--
-- Bewusst auf invoices_out und NICHT auf contacts: eine Leitweg-ID bzw.
-- Kundenreferenz gehoert je Rechnung/Vorgang zum Beleg, sie ist keine
-- stabile Stammdaten-Eigenschaft des Kontakts (ADR 0022, Abschnitt
-- "BuyerReference-Quelle").
--
-- Bewusst nullable und OHNE Default: die Pflicht-Pruefung erfolgt erst beim
-- tatsaechlichen E-Rechnungs-Export (E.4.3) mit klarer Fehlermeldung, nicht
-- bei der Rechnungsanlage. Ein NOT NULL hier wuerde alle bestehenden
-- Rechnungs-Erstellungspfade brechen und eine rueckwirkende Datenmigration
-- fuer Bestandsrechnungen erfordern; ein Dummy-Default wuerde eine
-- ungueltige E-Rechnung mit erfundener Referenz erzeugen statt den
-- fehlenden Wert sichtbar zu machen. Gleiches Muster wie
-- company_profiles.datev_berater_nr/datev_mandant_nr in
-- 083_datev_export.sql.
--
-- Bewusst KEIN CHECK-Constraint (anders als z.B. invoice_type in
-- 074_invoices_out_invoice_type.sql): der Wertebereich ist keine feste
-- Werteliste, sondern eine extern vorgegebene Freitext-Kennung
-- (Leitweg-IDs und Kundenreferenzen haben kein projektweit erzwingbares
-- Format). Eine Laengen-/Formatpruefung fuer den Export gehoert in den
-- Anwendungscode (E.4.3).
--
-- Bewusst KEIN Index: die Spalte wird ausschliesslich zusammen mit der
-- bereits ueber den Primaerschluessel geladenen Rechnung gelesen
-- (GET /invoices-out/{id}/xrechnung bzw. /zugferd), nie als Filter- oder
-- Sortierkriterium.
--
-- Keine Permission noetig: beide geplanten Export-Endpunkte nutzen laut
-- ADR 0022 die bereits bestehende Permission invoices_out.read.

ALTER TABLE invoices_out
    ADD COLUMN IF NOT EXISTS buyer_reference text;

COMMENT ON COLUMN invoices_out.buyer_reference IS
    'EN 16931 BT-10 BuyerReference (Leitweg-ID bei B2G, Kundenreferenz bei B2B). Nullable; Pflichtpruefung erst beim E-Rechnungs-Export, siehe ADR 0022.';

-- DOWN (manuell auszufuehren; der Migrationsrunner in server/internal/migrate
-- kennt keine automatischen Rollbacks, siehe docs/backlog.md 0.13):
--   ALTER TABLE invoices_out DROP COLUMN IF EXISTS buyer_reference;
-- DATENVERLUSTRISIKO: Sobald an Rechnungen eine Leitweg-ID/Kundenreferenz
-- gepflegt wurde, geht diese bei einem Downgrade unwiderruflich verloren;
-- betroffene Rechnungen koennen danach nicht mehr als E-Rechnung exportiert
-- werden, bis der Wert erneut erfasst ist. Die Rechnungen selbst, ihre
-- Betraege, Buchungen und ihr Storno-/Zahlungsstatus bleiben unberuehrt -
-- die Spalte wird von keiner anderen Tabelle referenziert und von keinem
-- Constraint verwendet.
