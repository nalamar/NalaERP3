# Direkte Quote-Navigation nach GAEB-Apply: Minimales technisches Zielmodell

## Ziel dieses Dokuments

Dieses Dokument schneidet den naechsten kleinen Ausbau nach dem
abgeschlossenen Apply-MVP zu:

- direkte Navigation zur erzeugten Draft-Quote aus `created_quote_id`
- kleine Apply-Transparenz im bestehenden Importdialog

Nicht Teil dieses Schritts:

- Re-Apply oder Ruecknahme
- Preis-/Material-/Steuermapping
- globale Freigabe-/Apply-Konsole

## 1. Ausgangslage

Nach `POST /api/v1/quotes/imports/{id}/apply` liegt bereits alles Nötige vor:

- Importstatus `applied`
- `created_quote_id` auf dem Importlauf
- Refresh von Importdetail und Quotes-Ansicht im Client

Der fehlende Anschluss ist aktuell rein UX-seitig:

- die erzeugte Quote ist bekannt, aber nicht direkt aus dem Dialog oeffenbar

## 2. Kernentscheidung

Der kleinste sinnvolle technische Folgepunkt ist ein reiner Client-Zuschnitt:

- im bestehenden GAEB-Importdetail eine direkte Aktion
  - `Quote oeffnen`
  anbieten, sobald `created_quote_id` vorhanden ist

Backend-Aenderungen sind dafuer nicht erforderlich.

## 3. Minimales UX-Zielbild

Im vorhandenen Importdialog in `client/lib/pages/quotes_page.dart`:

- nach erfolgreichem Apply bleibt der Status/`created_quote_id` sichtbar
- zusaetzlich erscheint eine kleine Sekundaeraktion:
  - `Quote oeffnen`
- Aktion setzt bestehende Quote-Filter so, dass die Zielquote sofort sichtbar ist
  und laedt danach die Quotes neu

Bewusst klein:

- keine neue Seite
- keine globale Navigation aus Importliste
- keine Mehrfachaktionen

## 4. Technischer Zuschnitt (Client)

### 4.1 Eingangsbedingung

Aktion wird nur angeboten, wenn:

- `created_quote_id` nicht leer ist

### 4.2 Erwartetes Verhalten

1. vorhandene Filter in der Quotes-Ansicht auf Zielquote fokussieren
2. `_load()` ausfuehren
3. Importdialog optional offen lassen oder schliessen (MVP-Entscheidung)

Empfehlung fuer den kleinen Schritt:

- Dialog schliessen und in die Quotes-Liste wechseln, um Kontextbruch zu
  minimieren

### 4.3 Fehlertoleranz

Falls die Zielquote trotz `created_quote_id` nicht geladen werden kann:

- bestehende Fehlerdarstellung (`_quoteErrorMessage`) nutzen
- kein zusaetzlicher Sonderpfad notwendig

## 5. Kleine Apply-Transparenz

Neben der Navigationsaktion reicht im MVP eine knappe Transparenz:

- Anzeige der erzeugten Quote-ID (bereits vorhanden)
- optional Anzeige einer knappen Hinweiszeile:
  - `Die Quote wurde erzeugt und kann jetzt geoeffnet werden.`

Keine weitergehenden Summaries in diesem Schritt:

- keine Accepted/Rejected-Zaehler
- keine Delta-/Preview-Darstellung

## 6. Guard Rails und Abgrenzung

Der Schritt bleibt bewusst risikolos:

- nur UI-Weiterleitung
- keine Aenderung an Apply-Guards
- keine Mutation von Import- oder Quotedaten

Damit bleibt die fachliche Integritaet aus `3.1.8.x` unberuehrt.

## 7. Naechster sinnvoller Schritt

Nach diesem Zielmodell ist der naechste kleine Umsetzungsschritt:

- minimale Client-Implementierung der direkten Quote-Navigation aus
  `created_quote_id` im bestehenden GAEB-Importdialog.
