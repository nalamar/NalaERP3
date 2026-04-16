# Kontaktrollen- und Statusstrategie

## Ausgangslage

Die aktuelle Contacts-Domaene nutzt zwei flache Kataloge:

- `rolle`: `customer`, `supplier`, `partner`, `both`, `other`
- `status`: `lead`, `active`, `inactive`, `blocked`

Das reicht fuer erste CRM-Faelle, ist fuer ein ERP aber fachlich zu grob:

- `both` mischt zwei eigenstaendige Geschaeftsbeziehungen in einen Sonderwert.
- `lead` ist vertriebsnah und passt nicht sauber auf Lieferanten oder allgemeine Partner.
- `active` und `inactive` vermischen Lifecycle, Vertriebsreife und Sperrlogik.
- `aktiv` und `status` tragen aktuell teilweise dieselbe Bedeutung doppelt.

## Zielbild

Kuenftig werden Kontaktklassifikation und Lifecycle strikt getrennt.

### 1. Geschaeftsbeziehung

Ein Kontakt kann mehrere Beziehungsarten parallel haben:

- `customer`
- `supplier`
- `partner`

`other` und `both` sollen perspektivisch entfallen.

Fachregel:

- `both` wird nicht mehr als Zielzustand fortgefuehrt.
- Ein Datensatz kann gleichzeitig Kunde und Lieferant sein.
- `partner` bleibt eine eigene Beziehung und ist zusaetzlich kombinierbar.

Empfohlene technische Zielstruktur:

- Entweder separate Join-Tabelle `contact_relationships`
- oder kurzfristig ein normalisierter Mehrfachkatalog auf Contact-Ebene

Fuer den naechsten Ausbauschritt ist eine Join-Tabelle fachlich sauberer, weil sie spaeter zusätzliche Attribute pro Beziehung tragen kann.

### 2. Lifecycle-Status

Der gemeinsame Kontaktstatus beschreibt nur noch die generelle Nutzbarkeit des Geschaeftspartners:

- `prospect`
- `active`
- `inactive`
- `blocked`

Bedeutung:

- `prospect`: noch nicht operativ freigegebener Interessent, primaer fuer Vertrieb
- `active`: normal nutzbarer Geschaeftspartner
- `inactive`: derzeit nicht aktiv genutzt, aber grundsaetzlich historisch gueltig
- `blocked`: gesperrt, operativ nicht einsetzbar

Fachregel:

- `prospect` darf nur bei Kontakten mit Kundenbezug vorkommen.
- Lieferanten ohne Kundenbezug starten fachlich direkt mit `active` oder `inactive`, nicht mit `prospect`.
- `blocked` bleibt harter Sperrstatus fuer Auswahl, Belegerstellung und operative Prozesse.

### 3. Operative Freigabe

Das Feld `aktiv` soll mittelfristig kein eigenstaendiges Fachkonzept mehr sein.

Zielregel:

- `active` => operativ freigegeben
- `inactive` und `blocked` => nicht operativ freigegeben
- `prospect` => sichtbar, aber nur in vertriebsnahen Prozessen nutzbar

Kurzfristig kann `aktiv` als Kompatibilitaetsfeld weiterlaufen, langfristig soll die Freigabe aus dem Status ableitbar sein.

## Zielmatrix

| Beziehung | Erlaubte Zielstatus | Hinweis |
| --- | --- | --- |
| customer | `prospect`, `active`, `inactive`, `blocked` | Vertriebs- und Bestandskunde |
| supplier | `active`, `inactive`, `blocked` | Kein `prospect` als Standardmodell |
| partner | `active`, `inactive`, `blocked` | Vertriebspartner, Nachunternehmer, Kooperation |
| customer + supplier | `active`, `inactive`, `blocked`, optional `prospect` nur mit echtem Kundenbezug | `both` wird dadurch ersetzt |
| nur historische Altfaelle | bestehende Werte bleiben migrierbar | keine harten Brueche im Bestand |

## Migrationsstrategie

### Phase 1: Fachliche Schaerfung ohne Schema-Bruch

- Bestehende API bleibt lauffaehig.
- `both` wird als Legacy-Wert toleriert, aber nicht mehr als bevorzugter Zielwert dokumentiert.
- `lead` wird fachlich als Vorlaeufer von `prospect` behandelt.
- Neue UI-Texte sollen bereits Richtung `Interessent`/`prospect` und kombinierbare Beziehungen zeigen.

### Phase 2: Beziehungskonzept entkoppeln

- Einfuehrung einer neuen Struktur fuer Mehrfachbeziehungen.
- Migration:
  - `customer` => nur Kundenbeziehung
  - `supplier` => nur Lieferantenbeziehung
  - `partner` => nur Partnerbeziehung
  - `both` => Kunden- und Lieferantenbeziehung
  - `other` => bleibt zunaechst ohne feste Beziehung oder wird in separates Klassifizierungsfeld ueberfuehrt

### Phase 3: Statusmodell normalisieren

- `lead` wird nach `prospect` ueberfuehrt.
- Validierung stellt sicher, dass `prospect` nur mit Kundenbezug verwendet wird.
- `aktiv` wird zum abgeleiteten oder rein technischen Kompatibilitaetsfeld.

## Auswirkungen auf Folgefeatures

- Angebote, Sales-Pipeline und Debitorenlogik muessen auf Kundenbeziehung statt auf `rolle == customer` vertrauen.
- Beschaffung, Bestellungen und Kreditorenlogik muessen auf Lieferantenbeziehung statt auf `rolle == supplier` vertrauen.
- Partnerfunktionen bleiben eigenstaendig und sollten nicht mehr ueber `other` abgefangen werden.
- Dublettenregeln, Nummernkreisvergabe und spaetere Statuskataloge koennen auf dem geschaerften Modell sauberer aufbauen.

## Entscheidung fuer den naechsten technischen Schritt

Der naechste Implementierungsschritt soll noch kein Vollumbau der Contacts-Domaene sein. Zuerst sollte ein kontrollierter Uebergangspfad fuer Rollen eingefuehrt werden:

1. Bestehende Legacy-Werte weiter lesen.
2. `both` in UI und API nicht mehr als bevorzugten Neuwert anbieten.
3. Zielmodell fuer kombinierbare Beziehungen technisch vorbereiten.
4. Erst danach Schema- und API-Umbau fuer Mehrfachbeziehungen starten.
