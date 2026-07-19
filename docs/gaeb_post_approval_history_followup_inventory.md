# GAEB-Folgeausbau nach Freigabehistorie:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen positionsbezogenen Freigabehistorie.

Der Fokus bleibt bewusst eng:

- Kommentar-UI, Freigabe-Queue, Nutzeranzeigenamen und Entscheidungsbadge
  fachlich vergleichen
- den kleinsten naechsten Ausbau mit gutem Signal bestimmen
- keine neue Mutation oder Lesesicht vor dem Zuschnitt implementieren

## 1. Ausgangslage

Der aktuelle Stand deckt bereits ab:

- Zielmargenanker und Approval-Hint je Quote-Position
- Freigabeanforderung als persistente Entitaet
- stabile aktive Anforderung trotz Quote-Update-Guard
- Storno aktiver Anforderungen
- Genehmigen und Ablehnen aktiver Anforderungen
- Entscheidungsmetadaten mit `decided_by`, `decided_at` und
  `decision_comment`
- Approved-Snapshots nur bei Genehmigung
- separater read-only Historien-Endpunkt je Quote-Position
- Inline-Historie im bestehenden Quote-Editor

Die Historie ist damit fachlich nachvollziehbar, aber noch nicht komfortabel:

- Genehmigen/Ablehnen in der UI senden aktuell keinen Kommentar.
- Die Historie zeigt technische User-IDs.
- Offene Anforderungen sind nur in der jeweiligen Quote sichtbar.
- Terminale Entscheidungen sind erst nach explizitem Historienabruf sichtbar.

## 2. Verbleibende fachliche Luecken

### 2.1 Kommentar-UI

Backend und Client-API unterstuetzen bereits optionale Kommentare fuer:

- Genehmigen
- Ablehnen

Die Quote-Editor-UI ruft diese API-Methoden aber ohne Kommentar auf. Dadurch
bleibt `decision_comment` meistens leer, obwohl die Historie dieses Feld
anzeigen kann.

Fachliche Wirkung:

- Ablehnungen sind schwerer nachzuvollziehen.
- Genehmigungen unter Zielmarge enthalten keine Begruendung.
- Die bereits implementierte Historie bleibt unterinformiert.

### 2.2 Freigabe-Queue

Eine zentrale Queue wuerde offene Anforderungen ueber alle Quotes hinweg
sichtbar machen.

Fachliche Wirkung:

- Freigebende muessen nicht jede Quote einzeln oeffnen.
- Offene Anforderungen koennen priorisiert werden.
- Es entsteht ein echter Arbeitsvorrat.

Notwendiger Scope:

- neues Backend-Readmodel ueber `quote_item_approval_requests`
- Join auf Quote, Position, Projekt/Kontakt und eventuell Nutzer
- neuer Endpoint, vermutlich unter `/api/v1/quotes/approval-requests`
- neue Client-Seite oder Dashboard-Block
- klare Berechtigung, vermutlich `quotes.approve` oder ein separates
  `quotes.approval_queue.read`

### 2.3 Nutzeranzeigenamen

Die Historie zeigt derzeit technische IDs aus:

- `requested_by`
- `cancelled_by`
- `decided_by`

Fachliche Wirkung:

- Auditierbarkeit ist vorhanden.
- Lesbarkeit fuer Fachanwender ist begrenzt.

Notwendiger Scope:

- Join auf `users`
- neue optionale Response-Felder wie `requested_by_name`,
  `cancelled_by_name`, `decided_by_name`
- Fallback auf technische ID
- Anpassung in Historie und spaeter Queue

### 2.4 Entscheidungsbadge

Ein positionsnahes Badge koennte den letzten terminalen Entscheid direkt an
der Position anzeigen, ohne die Historie zu laden.

Fachliche Wirkung:

- Nutzer sehen nach Reload schneller, dass eine Position genehmigt, abgelehnt
  oder storniert wurde.
- Der explizite Historienabruf bleibt fuer Details erhalten.

Notwendiger Scope:

- kleines Readmodel fuer letzte Anforderung je Quote-Position
- Einbettung in Quote-Detail oder separater leichter Endpoint
- UI-Anzeige im Quote-Editor

Risiko:

- Die Quote-Detailantwort wuerde weiter wachsen.
- Der Unterschied zwischen aktiver Anforderung und letzter terminaler
  Anforderung muss sehr klar bleiben.

## 3. Optionenvergleich

### Option A: Kommentar-UI fuer Genehmigen/Ablehnen

Vorteile:

- nutzt vorhandene Backend- und Client-API
- keine Migration erforderlich
- keine neue Berechtigung erforderlich
- verbessert die gerade geschaffene Historie direkt
- kleiner UI-Scope im bestehenden Quote-Editor

Nachteile:

- hilft nicht beim Finden offener Anforderungen
- verbessert nicht die Anzeige technischer User-IDs
- bleibt positionsnah und nicht global

### Option B: Zentrale Freigabe-Queue

Vorteile:

- hoher operativer Nutzen fuer Freigebende
- macht offene Anforderungen systemweit sichtbar
- ist langfristig ERP-typisch

Nachteile:

- groesserer Backend- und UI-Scope
- neue Listen-, Filter- und Navigationsfragen
- Berechtigungsmodell muss genauer geschnitten werden
- sollte von guten Kommentaren und Nutzeranzeigenamen profitieren

### Option C: Nutzeranzeigenamen

Vorteile:

- verbessert Historie und spaetere Queue
- nutzt vorhandene `users.display_name`
- fachlich leicht verstaendlich

Nachteile:

- braucht Backend-Response-Erweiterungen
- loest keine fehlenden Entscheidungsbegruendungen
- hat weniger Signal als Kommentar-UI, solange Kommentare leer bleiben

### Option D: Entscheidungsbadge

Vorteile:

- macht terminale Entscheidungen schneller sichtbar
- bleibt positionsnah
- kann die Historie sinnvoll ergaenzen

Nachteile:

- braucht neues Readmodel oder Erweiterung der Quote-Detailantwort
- Gefahr einer zweiten, teilweise redundanten Historienanzeige
- ohne Kommentare bleibt die Aussage bei Ablehnungen duenn

## 4. Entscheidung

Der kleinste sinnvolle Folgeausbau ist zuerst eine Kommentar-UI fuer
Genehmigen und Ablehnen im bestehenden Quote-Editor.

Begruendung:

- Die Datenbank, Service-Methoden, HTTP-Endpunkte und Client-API koennen
  Kommentare bereits verarbeiten.
- Die Historie zeigt `decision_comment` bereits an.
- Der aktuelle UI-Pfad laesst das wichtigste Kontextfeld leer.
- Der Scope bleibt rein frontendnah und positionsbezogen.
- Queue, Anzeigenamen und Badge koennen danach auf aussagekraeftigeren
  Historieneintraegen aufbauen.

## 5. Minimalziel fuer den naechsten Leaf

Der naechste Leaf sollte nur das technische UI-Zielmodell fuer
Entscheidungskommentare zuschneiden.

Zu entscheiden:

- ein gemeinsamer kleiner Dialog fuer `Genehmigen` und `Ablehnen`
- optionaler Kommentar, kein Pflichtfeld im ersten Schritt
- clientseitiges Limit maximal 500 Zeichen passend zum Backend
- vorhandene API-Methoden mit `comment` weiterverwenden
- Ladezustand und Fehlerbehandlung im bestehenden Muster belassen
- nach Erfolg aktive Anforderung entfernen und Historie invalidieren wie
  bisher

Nicht im naechsten Leaf:

- Implementierung des Dialogs
- Pflichtkommentar bei Ablehnung
- Kommentar fuer Storno oder Request
- Backend-Aenderungen
- neue Migration
- Freigabe-Queue
- Nutzeranzeigenamen
- Entscheidungsbadge

## 6. Reihenfolge danach

Empfohlene Reihenfolge nach der Kommentar-UI:

1. Kommentar-UI technisch zuschneiden
2. Kommentar-UI implementieren
3. Audit des Kommentar-UI-Blocks
4. Danach neu entscheiden zwischen:
   - zentrale Freigabe-Queue
   - Nutzeranzeigenamen fuer Historie/Queue
   - Entscheidungsbadge an Quote-Positionen

Die Queue sollte erst starten, wenn die einzelnen Entscheidungen genuegend
Kontext tragen. Sonst entsteht ein Arbeitsvorrat, dessen Entscheidungen zwar
sichtbar, aber fachlich schlecht begruendet sind.
