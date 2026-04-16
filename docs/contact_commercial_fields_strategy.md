# Kommerzielle Kontaktfelder-Strategie

## Ausgangslage

Die Contacts-Domaene besitzt bereits erste kaufmaennische Felder:

- `payment_terms`
- `debtor_no`
- `creditor_no`
- `tax_country`
- `tax_exempt`
- bestehend seit frueher: `vat_id`, `tax_no`, `waehrung`

Diese Felder sind bereits in API und Client sichtbar, aber fachlich noch zu flach:

- `payment_terms` ist Freitext statt belastbarer Referenz.
- `debtor_no` und `creditor_no` sind nicht an die Geschaeftsbeziehung gekoppelt.
- `tax_country` und `tax_exempt` reichen fuer grenzueberschreitende Faelle nur als Minimalmodell.
- Es gibt noch keine saubere Trennung zwischen Stammdaten, buchhalterischer Ableitung und Belegpraxis.

## Zielbild

Die kommerziellen Kontaktfelder sollen kuenftig drei Aufgaben sauber abdecken:

1. Belegvorschlaege fuer Vertrieb und Einkauf
2. Vorbereitung fuer Debitoren-/Kreditorenlogik im Finanzwesen
3. steuerliche Ableitungen fuer Angebot, Auftrag, Bestellung und Rechnung

## 1. Zahlungsbedingungen

### Ziel

`payment_terms` soll mittelfristig nicht nur Freitext sein, sondern auf einen administrierbaren Katalog verweisen.

Empfohlenes Zielmodell:

- `payment_term_code`
- optional weitergefuehrtes Anzeige-/Snapshot-Feld auf Belegen

Ein Zahlungsbedingungskatalog soll mindestens tragen:

- `code`
- `name`
- `description`
- `days_due`
- `discount_days`
- `discount_percent`
- `is_active`
- `sort_order`

### Uebergang

- Bestehendes `payment_terms` bleibt vorerst kompatibel.
- Neue Belege duerfen den Text weiterhin uebernehmen.
- Spaeter wird `payment_terms` von Freitext auf Katalogreferenz plus Snapshot umgestellt.

## 2. Debitor- und Kreditor-Referenzen

### Ziel

`debtor_no` und `creditor_no` bleiben wichtige Aussenreferenzen, muessen aber fachlich an die Geschaeftsbeziehung gekoppelt werden.

Regeln:

- `debtor_no` ist nur fuer Kontakte mit Kundenbezug fachlich sinnvoll.
- `creditor_no` ist nur fuer Kontakte mit Lieferantenbezug fachlich sinnvoll.
- Ein Kontakt darf beide Nummern haben, wenn er beide Beziehungen besitzt.
- Nummern bleiben mandantenweit eindeutig.

### Bedeutung

- Diese Felder sind keine internen Primary Keys.
- Sie sind Referenzen fuer Buchhaltung, Import/Export, FiBu-Schnittstellen und Belegdruck.
- Sie muessen historisch stabil sein und duerfen nicht automatisch recycelt werden.

### Uebergang

- Bestehende Felder bleiben erhalten.
- In spaeteren Validierungen wird die Beziehungskopplung hinzugefuegt.
- Solange `rolle` noch flach ist, darf die API die Werte tolerieren, aber kuenftig fachliche Hinweise oder UI-Sperren erhalten.

## 3. Steuermerkmale

### Vorhandene Basis

Aktuell vorhanden:

- `vat_id`
- `tax_no`
- `tax_country`
- `tax_exempt`

### Ziel

Die Steuerlogik soll ausbaubar werden, ohne die Contacts-Domaene sofort mit voller Buchungslogik zu ueberladen.

Empfohlene fachliche Erweiterungsrichtung:

- `tax_country`
- `tax_exempt`
- `vat_id`
- spaeter zusaetzlich ein explizites Steuerprofil oder Steuerregime

Moegliche spaetere Zielwerte fuer ein Steuerprofil:

- `domestic_standard`
- `domestic_exempt`
- `intra_eu_reverse_charge`
- `export_third_country`
- `domestic_private`

### Fachregeln

- `tax_country` bleibt Pflichtfeld mit ISO-Landcode.
- `tax_exempt` allein ist nicht ausreichend fuer alle Faelle, bleibt aber als Uebergangsflag sinnvoll.
- `vat_id` ist besonders relevant fuer Firmenkunden im EU-Kontext.
- `tax_no` bleibt nationales Zusatzmerkmal, nicht universelle Primäridentitaet.

## 4. Belegableitung

Kontaktdaten sollen Default-Werte fuer Belege liefern, aber Belege muessen ihren eigenen Snapshot behalten.

Deshalb gilt:

- Kontaktstammdaten liefern Vorschlaege.
- Angebote, Bestellungen, Auftraege und Rechnungen speichern eigene uebernommene Werte.
- Spaetere Aenderungen am Kontakt duerfen alte Belege nicht stillschweigend veraendern.

Das betrifft insbesondere:

- Zahlungsbedingungen
- Debitor-/Kreditorreferenzen
- Steuerland
- Steuerbefreiung bzw. kuenftiges Steuerprofil

## 5. Validierungsstrategie

### Kurzfristig

- Bestehende Felder bleiben kompatibel.
- Es soll nur dort validiert werden, wo bereits heute klare Regeln bestehen:
  - `tax_country` als normalisierter ISO-Code
  - Dublettenpruefung fuer `debtor_no` und `creditor_no`
  - Leerstring-Toleranz fuer optionale Felder

### Mittelfristig

- `debtor_no` nur mit Kundenbezug
- `creditor_no` nur mit Lieferantenbezug
- `payment_terms` ueber Katalog statt Freitext
- Steuervalidierung anhand Kontaktart und Steuerprofil

## 6. Migrationsstrategie

### Phase 1: Fachmodell schaerfen

- Dokumentation und Regeln festlegen
- bestehende Felder stabil weiter nutzen

### Phase 2: Zahlungsbedingungen strukturieren

- eigener Referenzdatenkatalog fuer Zahlungsbedingungen
- Contact-Feld kontrolliert auf Referenz plus Snapshot-Pfad umstellen

### Phase 3: Beziehungssensitive Validierung

- Debitor/Kreditor gegen geschaerftes Beziehungsmodell validieren
- UI nur noch passende Felder je Beziehung aktiv anbieten

### Phase 4: Steuerprofil erweitern

- einfaches `tax_exempt` zu expliziterem Steuerprofil ausbauen
- Beleglogik und Rechnungsstellung darauf aufsetzen

## Entscheidung fuer den naechsten technischen Schritt

Der naechste Implementierungsschritt sollte noch kein Vollumbau aller Contact-Felder sein. Am meisten Signal hat zuerst die Strukturierung der Zahlungsbedingungen:

1. Ist-Zustand in Contacts kompatibel lassen.
2. Zielmodell fuer einen Zahlungsbedingungen-Katalog festlegen.
3. Danach Katalog und Settings-Verwaltung einführen.
4. Debitor/Kreditor- und Steuerprofil-Validierung erst auf Basis des geschaerften Kontaktbeziehungsmodells nachziehen.
