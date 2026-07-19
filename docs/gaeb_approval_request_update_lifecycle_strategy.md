# GAEB-Freigabeanforderungen: Update-Lebenszyklusregel

## Ziel dieses Dokuments

Dieses Dokument schneidet die kleinste robuste Lebenszyklusregel fuer aktive
positionsbezogene Freigabeanforderungen beim Speichern eines Draft-Angebots
zu.

Ausgangspunkt:

- aktive Freigabeanforderungen haengen an `quote_items.id`
- `Service.Update(...)` ersetzt aktuell alle Positionen per
  `DELETE FROM quote_items WHERE quote_id=$1`
- `quote_item_approval_requests.quote_item_id` nutzt `ON DELETE CASCADE`

Ohne Zusatzregel verschwinden aktive Anforderungen beim normalen
Quote-Speichern.

## 1. Bewertete Optionen

### Option A: Quote-Save bei aktiven Anforderungen blockieren

Regel:

- Wenn fuer eine Quote mindestens eine `requested`-Anforderung existiert,
  darf `Service.Update(...)` die Quote nicht speichern.

Vorteile:

- kleinster Implementierungsschnitt
- keine implizite fachliche Entscheidung
- aktive Anforderungen bleiben stabil referenzierbar
- passt zum aktuellen Statusmodell `requested/cancelled`
- bereitet einen expliziten Storno-Endpunkt sauber vor

Nachteile:

- Nutzer muss spaeter bewusst stornieren, bevor er das Angebot weiter
  bearbeitet
- UI braucht mittelfristig eine verstaendliche Meldung und Storno-Aktion

### Option B: Aktive Anforderungen vor Positionsersetzung automatisch stornieren

Regel:

- `Service.Update(...)` setzt aktive Anforderungen auf `cancelled`, bevor
  Positionen geloescht werden.

Vorteile:

- Quote-Save bleibt ohne zusaetzliche Nutzeraktion moeglich
- vorhandenes Statusmodell enthaelt bereits `cancelled`

Nachteile:

- Storno erfolgt implizit ohne sichtbare Absicht
- `cancelled_by` waere im aktuellen `Update(...)`-Service nicht sauber
  verfuegbar
- Historie bleibt fachlich schwach, weil der eigentliche Grund
  "Positionsersetzung" nicht modelliert ist
- geloeschte `quote_items` wuerden trotz Storno danach weiterhin die
  referenzierte Position entfernen, wenn die FK-Strategie nicht angepasst
  wird

### Option C: Positionen stabil aktualisieren statt delete/insert

Regel:

- `Service.Update(...)` erhaelt bestehende `quote_items.id`, aktualisiert
  passende Positionen und loescht nur entfernte Positionen.

Vorteile:

- fachlich langfristig besser fuer Entscheidungen, Historien und
  positionsbezogene Anker
- reduziert Cascade-Risiken auch fuer andere Kindtabellen

Nachteile:

- groesserer Umbau des Angebots-Update-Pfads
- Eingabe-DTO und Client-Update muessten positionsstabile IDs sauber
  mitsenden und respektieren
- Konfliktrisiko mit bestehenden Tests und GAEB-Apply-Flows hoeher

## 2. Entscheidung fuer den naechsten Implementierungs-Leaf

Der naechste kleine Implementierungs-Leaf soll Option A umsetzen:

`Service.Update(...)` blockiert Draft-Speichern, wenn aktive
`quote_item_approval_requests` mit `status = 'requested'` zur Quote
existieren.

Fachliche Fehlermeldung:

```text
Aktive Freigabeanforderungen muessen vor dem Speichern storniert werden
```

Begruendung:

- Die bestehende Datenintegritaet wird sofort geschuetzt.
- Es entsteht keine automatische Storno-Mutation.
- Der kommende Storno-Endpunkt bekommt einen klaren Zweck:
  aktive Anforderungen bewusst aufheben, damit Bearbeitung wieder moeglich
  ist.
- Der Entscheidungsworkflow wird nicht auf einem instabilen
  Positionslebenszyklus aufgebaut.

## 3. Backend-Schnitt

Pruefung innerhalb von `Service.Update(...)` nach dem Lock der Quote und vor
dem `UPDATE quotes`:

```sql
SELECT COUNT(*)
FROM quote_item_approval_requests
WHERE quote_id = $1
  AND status = 'requested'
```

Wenn Anzahl groesser 0:

- Fehler abbrechen
- keine Quote-Felder aktualisieren
- keine Positionen loeschen
- keine Anforderungen veraendern

Warum vor dem Quote-Header-Update:

- Die Operation bleibt komplett wirkungslos.
- Es gibt keine partiell geaenderten Summen, Notizen oder Gueltigkeiten.

## 4. API-Verhalten

Der bestehende Update-Endpunkt soll den Domain-Fehler als Client-Fehler
zurueckgeben.

Erwartung:

- HTTP `400 Bad Request`
- bestehendes Fehlerformat
- keine neue Route
- keine neue Berechtigung

Falls die zentrale Fehlerklassifizierung den Text noch nicht als
Domain-Fehler kennt, muss sie um die Meldung zu aktiven
Freigabeanforderungen erweitert werden.

## 5. Client-Verhalten fuer diesen Leaf

Kein neuer Client-Flow im ersten Implementierungsschritt.

Der vorhandene Save-Fehlerpfad reicht zunaechst aus, wenn die API eine
verstaendliche Meldung liefert. Die UI zeigt aktive Anforderungen bereits am
Item an.

Spaeterer Client-Folgepfad:

- Storno-Aktion am Zielmargenblock
- danach erneutes Speichern moeglich

## 6. Testschnitt

Integrationstest im bestehenden Quote-Flow:

1. Draft-Quote mit unterzieliger Position und Preisentscheidung erzeugen
2. Freigabeanforderung erzeugen
3. `PUT /api/v1/quotes/{id}` mit gueltigem Payload senden
4. `400 Bad Request` erwarten
5. Quote erneut laden
6. aktive `active_approval_request` weiterhin vorhanden erwarten

Damit wird die kritische Cascade-Loeschkante direkt abgesichert.

## 7. Nicht-Ziele

Nicht Teil des naechsten Implementierungs-Leafs:

- Storno-Endpunkt
- Genehmigen oder Ablehnen
- positionsstabiles Rewrite von `Service.Update(...)`
- neues Statusmodell fuer Entscheidungen
- Queue offener Freigaben
- UI-Dialog fuer Storno

## 8. Folgepfad

Nach der Blockade-Regel ist der naechste fachlich kleine Schreibpfad:

`Subtask 3.1.33.6: Storno aktiver Freigabeanforderungen zuschneiden`

Erst nach Storno sollte der eigentliche Entscheidungsworkflow geschnitten
werden.
