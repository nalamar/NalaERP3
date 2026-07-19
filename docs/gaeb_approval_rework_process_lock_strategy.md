# GAEB-Freigabeentscheidungen: Backend-Prozesssperre bei offener Nacharbeit

## Ziel dieses Zuschnitts

Dieses Dokument legt fest, welche serverseitigen Aktionen bei zuletzt
abgelehnten Positionsfreigaben gesperrt werden sollen.

Der Zuschnitt ist bewusst fachlich und technisch eng:

- Definition "offene Nacharbeit"
- betroffene Quote- und Folgebelegaktionen
- erlaubte Remediation-Pfade
- empfohlene Backend-Guards
- spaetere Tests

In diesem Leaf wird noch kein Runtime-Code geaendert.

## 1. Fachliche Definition

Offene Nacharbeit liegt vor, wenn mindestens eine Position eines Angebots als
letzte terminale Freigabeentscheidung den Status `rejected` hat.

Terminale Freigabeentscheidungen sind:

- `approved`
- `rejected`

Nicht terminal und fuer diese Nacharbeitsdefinition nicht zaehlend:

- `requested`
- `cancelled`

Damit ist die Sperre bewusst an das vorhandene Readmodel
`latest_approval_decision` angelehnt: Eine spaetere genehmigte Entscheidung hebt
eine fruehere Ablehnung auf. Eine spaetere Ablehnung setzt Nacharbeit wieder
offen.

## 2. Zu sperrende Aktionen

### 2.1 Statuswechsel

Der Quote-Statuswechsel soll gesperrt werden fuer:

- `sent`
- `accepted`

Begruendung:

- `sent` bedeutet externe kaufmaennische Weitergabe.
- `accepted` bedeutet kommerzielle Annahme.
- Beide Zustaende duerfen nicht erreicht werden, solange mindestens eine
  Position fachlich zuletzt abgelehnt wurde.

Der bestehende `Accept`-Flow ruft intern `UpdateStatus(..., "accepted")` auf und
wird dadurch automatisch mitgesperrt, sobald der Guard in `UpdateStatus` sitzt.

### 2.2 Rechnung aus Angebot

`ConvertToInvoice` soll gesperrt werden.

Begruendung:

- Die Umwandlung erzeugt einen Folgebeleg.
- Der Flow setzt das Angebot anschliessend faktisch auf `accepted`.
- Eine Rechnung waere bei offener Positionsnacharbeit kaufmaennisch zu spaet.

Der Guard muss innerhalb der Transaktion ausgefuehrt werden, nachdem das Angebot
gesperrt und bevor Positionen in Rechnungspositionen uebernommen werden.

### 2.3 Auftrag aus Angebot

`sales.Service.CreateFromQuote` soll gesperrt werden.

Begruendung:

- Die Umwandlung in einen Auftrag liegt technisch im Sales-Service, nicht im
  Quote-Service.
- Der Endpoint `/quotes/{id}/convert-to-sales-order` delegiert direkt in diesen
  Service.
- Ein bereits angenommener Quote-Status allein darf nicht reichen, falls danach
  noch eine abgelehnte Positionsentscheidung sichtbar ist.

Der Guard gehoert deshalb auch in die Sales-Service-Transaktion.

## 3. Erlaubte Aktionen

Folgende Aktionen bleiben erlaubt:

- Statuswechsel nach `draft`
- Statuswechsel nach `rejected`
- Bearbeitung von Entwuerfen ueber `Update`
- Revision ueber `Revise`, sofern die bestehenden Status- und Folgebelegregeln
  sie erlauben
- neue Freigabeanforderung nach fachlicher Korrektur
- Genehmigung einer spaeteren Freigabeanforderung

Begruendung:

Die Sperre soll kommerzielle Weiterverarbeitung verhindern, aber Nacharbeit
nicht blockieren. Remediation muss moeglich bleiben.

## 4. Empfohlene Fehlermeldung

Ein einheitlicher Domainfehler reicht fuer alle gesperrten Aktionen:

```text
Angebot enthaelt abgelehnte Freigabeentscheidungen; Nacharbeit vor Versand, Annahme oder Folgebeleg erforderlich
```

Bewertung:

- Der Text nennt die Ursache.
- Der Text nennt die blockierten Zielarten.
- Der Text vermeidet UI-spezifische Begriffe.

## 5. Technischer Guard

Empfohlene Hilfsabfrage:

```sql
SELECT EXISTS (
  SELECT 1
  FROM quote_items qi
  JOIN LATERAL (
    SELECT qar.status
    FROM quote_item_approval_requests qar
    WHERE qar.quote_item_id = qi.id
      AND qar.status IN ('approved', 'rejected')
    ORDER BY qar.decided_at DESC NULLS LAST, qar.updated_at DESC
    LIMIT 1
  ) latest ON true
  WHERE qi.quote_id = $1
    AND latest.status = 'rejected'
)
```

Der Guard sollte als kleine Backend-Hilfe umgesetzt werden, damit Statuswechsel,
Rechnungsumwandlung und Auftragsumwandlung dieselbe Definition verwenden.

Wichtig:

- Keine Migration.
- Keine neue Permission.
- Kein neuer API-Response-Typ.
- Keine Client-only-Sperre als Ersatz fuer Backend-Validierung.

## 6. Umsetzung in Micro-Subtasks

Die Prozesssperre wird in kleine Leaves geteilt:

- Subtask 3.1.41.2: Backend-Guard fuer Quote-Statuswechsel bei offener
  Nacharbeit implementieren
- Subtask 3.1.41.3: Backend-Guard fuer Rechnungserzeugung aus Angebot
  implementieren
- Subtask 3.1.41.4: Backend-Guard fuer Auftragserzeugung aus Angebot
  implementieren
- Subtask 3.1.41.5: HTTP-/Service-Tests fuer gesperrte und erlaubte Pfade
  ergaenzen

## 7. Teststrategie fuer Folge-Leaves

Spaetere Tests sollen mindestens abdecken:

- Statuswechsel nach `sent` scheitert bei zuletzt abgelehnter Position.
- Statuswechsel nach `accepted` scheitert bei zuletzt abgelehnter Position.
- `Accept` scheitert indirekt ueber denselben Guard.
- `ConvertToInvoice` scheitert bei offener Nacharbeit.
- `CreateFromQuote` scheitert bei offener Nacharbeit.
- Statuswechsel nach `draft` oder `rejected` bleibt moeglich.
- Eine spaetere `approved`-Entscheidung hebt die Sperre auf.

## 8. Entscheidung

Die Backend-Prozesssperre wird eingefuehrt, aber nur fuer kommerziell bindende
Weiterverarbeitung:

- Versand
- Annahme
- Rechnung
- Auftrag

Nacharbeit, Revision und erneute Freigabe bleiben offen. Der naechste kleinste
Implementierungsschritt ist der Guard fuer `UpdateStatus`, weil er zugleich den
direkten Statuswechsel und den `Accept`-Flow absichert.
