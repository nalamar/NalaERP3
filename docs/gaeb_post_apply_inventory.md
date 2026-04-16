# Zielstrecke nach erzeugter GAEB-Draft-Quote: Inventur und kleinster naechster Schritt

## Ziel dieses Dokuments

Dieses Dokument inventarisiert die Strecke nach dem abgeschlossenen
Freigabe-/Apply-MVP (`parsed -> reviewed -> applied`) und legt fest, welcher
naechste Ausbau fachlich den kleinsten sinnvollen Schritt bildet.

Fokus:

- was nach `created_quote_id` noch fehlt
- welche Folgeoptionen sinnvoll sind
- welcher Einstiegspunkt den besten Signal-/Risikowert hat

## 1. Ausgangslage nach dem Apply-MVP

Der aktuelle Stand ist:

- Importlauf kann auf `reviewed` freigegeben werden
- Importlauf kann kontrolliert auf `applied` gehen
- dabei wird genau eine neue Quote im Status `draft` erzeugt
- `created_quote_id` ist als Rueckverknuepfung vorhanden
- der bestehende GAEB-Importdialog zeigt Freigabe/Apply bereits an

Damit ist die technische Mindestkette bis zur Draft-Quote geschlossen.

## 2. Verbleibende Luecke nach erfolgreichem Apply

Die groesste verbleibende Luecke liegt nicht in der Erzeugung, sondern in der
Anschlussfuehrung:

- Nutzer sieht, dass eine Quote erzeugt wurde
- aber der Uebergang in den eigentlichen Angebotsarbeitsfluss ist noch schwach

Genauer fehlen aktuell:

- direkte Navigation in die erzeugte Quote
- klare Apply-Transparenz auf Importlauf-Ebene (z. B. kompakte Anzeige, was
  uebernommen wurde und was nicht)
- spaeter ein strukturierter Mapping-Pfad (Preis/Material/Steuer)

## 3. Folgeoptionen im Vergleich

### Option A: Quote-Navigation direkt nach Apply

Nutzen:

- schliesst den Arbeitsfluss `Import -> Quote-Bearbeitung` sofort
- wenig fachliches Risiko
- kleinster UX-Gewinn mit direkter Wirkung

Aufwand:

- klein; bestehende Quote-Filter/Seite sind bereits vorhanden

### Option B: Apply-Transparenz ausbauen (Preview/Delta/Zaehler)

Nutzen:

- bessere Nachvollziehbarkeit der Uebernahme

Aufwand:

- mittel; benoetigt zusaetzliche Aggregation und UI-Flaeche
- risikoarm, aber groesser als Navigation

### Option C: Fruehes Mapping (Preis/Material/Steuer)

Nutzen:

- hoher spaeterer Business-Wert

Aufwand/Risiko:

- hoch; fachlich komplex und fehleranfaellig
- kein kleiner naechster Schritt

## 4. Entscheidung

Der kleinste sinnvolle naechste Schritt nach dem Apply-MVP ist:

- **Quote-Navigation** als direkte Anschlussfuehrung nach erfolgreichem Apply

Begruendung:

- minimaler Eingriff
- unmittelbarer operativer Nutzen
- keine neue fachliche Komplexitaet
- gute Basis fuer spaetere Apply-Transparenz und Mapping-Ausbau

## 5. Bewusste Abgrenzung

Nicht Teil des naechsten kleinen Schritts:

- komplexe Apply-Preview/Delta-Logik
- Re-Apply oder Ruecknahme
- Preis-/Material-/Steuermapping
- Uebernahme in bestehende Quotes
- globale Freigabe-/Apply-Konsole

## 6. Naechster sinnvoller Schritt

Nach dieser Inventur ist der naechste kleine Folgepunkt:

- minimales technisches Zielmodell fuer die direkte Navigation zur erzeugten
  Draft-Quote (`created_quote_id`) festlegen und auf einen kleinen
  Dialog-/Seitenuebergang begrenzen.
