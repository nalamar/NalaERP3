# GAEB-Folgeausbau nach Entscheidungskommentaren:
# Inventur und kleinster Einstiegspunkt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert den naechsten sinnvollen Folgeausbau nach der
abgeschlossenen Kommentar-UI fuer Genehmigen und Ablehnen.

Der Fokus bleibt bewusst eng:

- zentrale Freigabe-Queue, Nutzeranzeigenamen und Entscheidungsbadge
  fachlich vergleichen
- den kleinsten naechsten Ausbau mit gutem Signal bestimmen
- keine neue Laufzeitlogik vor dem Zuschnitt implementieren

## 1. Ausgangslage

Der aktuelle Stand deckt ab:

- Freigabeanforderungen je Quote-Position
- Storno aktiver Anforderungen
- Genehmigen und Ablehnen aktiver Anforderungen
- optionale Entscheidungskommentare aus dem Quote-Editor
- Historie je Quote-Position mit Status, Grund, Snapshots,
  Entscheidungsmetadaten und Kommentar
- Berechtigungstrennung:
  - `quotes.write` fuer Anforderung/Storno und normale Quote-Bearbeitung
  - `quotes.approve` fuer Genehmigen/Ablehnen
  - `quotes.read` fuer Historienlesen

Damit sind Entscheidungen fachlich begruendbar und nachtraeglich sichtbar.

Verbleibende Luecken:

- Nutzer werden in Historie nur als technische IDs angezeigt.
- Offene Anforderungen sind weiterhin nur in der Quote selbst auffindbar.
- Terminale Entscheidungen sind erst nach explizitem Historienabruf sichtbar.

## 2. Option A: Zentrale Freigabe-Queue

Eine Freigabe-Queue wuerde offene Anforderungen systemweit sichtbar machen.

Moegliches Ziel:

- `GET /api/v1/quotes/approval-requests`
- nur aktive `requested`-Anforderungen
- Join auf Quote, Quote-Position, Projekt, Kontakt und Nutzer
- Filter nach Projekt/Kontakt/Alter/Grund
- UI als Dashboard- oder eigene Liste fuer Freigebende

Vorteile:

- hoher operativer Nutzen
- Freigebende finden offene Aufgaben ohne einzelne Quotes zu oeffnen
- Grundlage fuer echte Freigabearbeit im ERP

Nachteile:

- neuer Backend-Endpoint
- neues Listen-Readmodel
- neue UI-Flaeche
- Berechtigungszuschnitt muss bewusst entschieden werden
- profitiert stark von lesbaren Nutzeranzeigenamen

Bewertung:

Fachlich wichtig, aber groesser als der kleinste naechste Schritt.

## 3. Option B: Nutzeranzeigenamen

Nutzeranzeigenamen wuerden technische IDs in der Historie und spaeteren Queue
lesbarer machen.

Moegliches Ziel:

- bestehendes `QuoteItemApprovalRequest`-Readmodel um optionale
  Anzeigename-Felder erweitern
- Werte aus `users.display_name`, fallback auf `email`, fallback auf ID
- zunaechst nur im bestehenden Historien-Endpoint
- Client-Historie zeigt bevorzugt Anzeigenamen

Moegliche Felder:

```text
requested_by_name
cancelled_by_name
decided_by_name
```

Vorteile:

- sehr kleiner Backend- und Client-Scope
- nutzt vorhandene `users.display_name`
- verbessert die gerade geschaffene Historie direkt
- bereitet die spaetere Queue vor
- keine neue Mutation und keine neue Permission erforderlich

Nachteile:

- offene Anforderungen werden dadurch noch nicht global auffindbar
- terminale Entscheidungen bleiben ohne Badge erst nach Historienabruf sichtbar
- historisierte Anzeigenamen werden nicht snapshotartig gespeichert, sondern
  aus dem aktuellen User-Stamm gelesen

Bewertung:

Kleinster naechster Schritt mit gutem Signal.

## 4. Option C: Entscheidungsbadge

Ein Entscheidungsbadge wuerde die letzte terminale Entscheidung direkt an der
Quote-Position sichtbar machen.

Moegliches Ziel:

- letztes terminales Approval je Position als read-only Feld
- Anzeige im Quote-Editor, z. B. `Genehmigt`, `Abgelehnt`, `Storniert`
- Historie bleibt Detailansicht

Vorteile:

- terminale Entscheidungen sind schneller sichtbar
- bleibt positionsnah
- vermeidet zunaechst eine globale Queue

Nachteile:

- braucht neues Readmodel fuer letzte terminale Anforderung
- Quote-Detailantwort wird groesser
- Abgrenzung zu `active_approval_request` muss sehr klar bleiben
- Nutzeranzeigenamen waeren auch hier hilfreich

Bewertung:

Sinnvoller Folgepunkt, aber fachlich riskanter als Anzeigenamen, weil die
Quote-Detailantwort um eine zweite Approval-Sicht erweitert wuerde.

## 5. Entscheidung

Der kleinste sinnvolle Folgeausbau ist zuerst Nutzeranzeigenamen fuer die
Freigabehistorie.

Begruendung:

- Die Historie ist bereits vorhanden und wird genutzt.
- Entscheidungskommentare sind jetzt erfassbar; nun fehlt die lesbare Person.
- `users.display_name` ist bereits im Auth/User-Modell vorhanden.
- Der Ausbau bleibt read-only.
- Der Schritt verbessert spaeter sowohl Queue als auch Badge.
- Queue und Badge koennen danach auf einem besser lesbaren Approval-Readmodel
  aufsetzen.

## 6. Minimalziel fuer den naechsten Leaf

Der naechste Leaf sollte nur das technische Minimalzielmodell fuer
Nutzeranzeigenamen in der Freigabehistorie zuschneiden.

Zu entscheiden:

- welche Response-Felder verwendet werden
- ob Anzeige auf `display_name`, `email`, `id` fallbackt
- ob nur `ListApprovalRequestsForQuoteItem(...)` erweitert wird oder auch
  `active_approval_request`
- ob Client-Draft Felder wie `requestedByName`, `cancelledByName`,
  `decidedByName` bekommt
- wie die UI weiterhin auf technische IDs fallbackt

Nicht im naechsten Leaf:

- Implementierung
- zentrale Queue
- Entscheidungsbadge
- Migration
- User-Snapshot an Approval-Requests
- neue Berechtigungen

## 7. Reihenfolge danach

Empfohlene Reihenfolge:

1. Nutzeranzeigenamen technisch zuschneiden
2. Backend-Historien-Readmodel um Anzeigenamen erweitern
3. Client-Historie auf Anzeigenamen umstellen
4. Audit des Anzeigenamen-Blocks
5. Danach neu entscheiden zwischen:
   - zentrale Freigabe-Queue
   - Entscheidungsbadge je Quote-Position

Die Queue sollte erst starten, wenn ihr Readmodel fachlich lesbar genug ist.
Technische User-IDs in einer Queue waeren fuer Freigebende zu schwach.
