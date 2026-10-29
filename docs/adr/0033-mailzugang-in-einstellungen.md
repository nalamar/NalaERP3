# ADR 0033 — Mail-Zugang in den Einstellungen, Passwort verschlüsselt

Datum: 2026-10-29
Status: entschieden (Nutzer am 2026-10-29: "die Mail Zugangsdaten sollten in
den Einstellungen eintragbar sein")
Bezug: Backlog F.14 (E-Mail-Versand der Hinweise aus F.10.4), ADR 0030
(keine Rohdaten in Logs), `AGENTS.md` (keine Secrets im Repo).

## Kontext

F.14 war so geplant, dass der SMTP-Zugang nur als Umgebungsvariablen
(`SMTP_HOST`, `SMTP_PASSWORD`, …) gesetzt wird. Der Nutzer möchte ihn
stattdessen in der Anwendung pflegen: Mail-Server, Port, Verschlüsselung,
Benutzer, Passwort und Absender ändern sich, ohne dass jemand an den Server
muss — und jeder Mandant kann ein eigenes Konto haben.

Damit liegt ein Passwort in der Datenbank. Ein Datenbank-Dump (Sicherung,
Weitergabe zur Fehlersuche) enthielte es im Klartext, und eine API, die die
Einstellung zurückliefert, könnte es ausliefern.

## Entscheidung

1. **Tabelle `company_mail_settings`, eine Zeile je Mandant** (Migration
   104): aktiv, Server, Port, Verschlüsselung, Benutzer, Passwort
   (verschlüsselt), Absenderadresse, Absendername, zuletzt geändert.
2. **Das Passwort wird nur verschlüsselt gespeichert** — AES-256-GCM,
   Schlüssel aus der Umgebungsvariablen `SETTINGS_ENCRYPTION_KEY` (32 Byte,
   Base64). Der Schlüssel liegt also NICHT in der Datenbank: ein Dump allein
   verrät das Passwort nicht. Format `0x01 || Nonce (12) || Chiffrat`; das
   Versionsbyte erlaubt später einen Schlüsselwechsel. Die Mandanten-ID ist
   zusätzliche authentifizierte Angabe (AAD): ein Chiffrat, das in die Zeile
   eines anderen Mandanten kopiert wird, lässt sich nicht entschlüsseln.
3. **Ohne gültigen Schlüssel** lässt sich kein Passwort speichern
   (fachliche Meldung); ein gesetzter, aber ungültiger Schlüssel verhindert
   den Start des Servers (`ValidateForStartup`). Geht der Schlüssel
   verloren, wird das Passwort neu eingetragen — sonst geht nichts verloren.
4. **Die API liefert das Passwort nie aus**, auch nicht verschlüsselt — nur
   `password_set`. Ein leeres Passwort beim Speichern lässt das bisherige
   stehen; Löschen ist ein eigener Schalter.
5. **Verschlüsselung der Verbindung Pflicht**: `ssl` (implizites TLS,
   üblich Port 465) oder `starttls` (üblich Port 587). Unverschlüsselt ist
   nicht vorgesehen — das Passwort ginge im Klartext über das Netz.
6. **Logs und Versandprotokoll** enthalten weder Passwort noch Mailinhalt;
   Fehler des Mail-Servers werden nur zusammengefasst ausgegeben (ADR 0030).
7. Pflegen dürfen die Einstellung nur Benutzer mit `settings.manage`, wie
   die übrigen Mandanten-Einstellungen.

## Folgen

- Die frühere Regel "SMTP-Zugang nur als Umgebungsvariablen" ist hiermit
  ersetzt. Als Umgebungsvariable bleibt allein der Schlüssel.
- Für den Betrieb muss `SETTINGS_ENCRYPTION_KEY` gesetzt und gesichert
  werden (`openssl rand -base64 32`), getrennt von der Datenbanksicherung.
- Zugangsdaten für die Entwicklung stehen nur in der lokalen, nicht
  eingecheckten `.env`, nie im Repo, in Fixtures oder Tests.
