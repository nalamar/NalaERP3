# GAEB: Strategie für die Client-Prozessaktion

Subtask 3.1.77.2 ist abgeschlossen. ApiClient erhält
processGAEBQuoteImport(importId), das POST /quotes/imports/{id}/process ohne
Body aufruft und den QuoteImport zurückgibt.

QuotesPage zeigt für einen Import mit Status uploaded und quotes.write genau
eine Aktion Verarbeiten. Sie öffnet den bestehenden Fortschrittsdialog, ruft
den Client auf, lädt die Importliste neu und zeigt den Ergebnisstatus. Bei
failed bleibt die fachliche Fehlermeldung sichtbar; bei API-Fehler bleibt der
Import unverändert und eine Snackbar erklärt den Fehler.

3.1.77.3 implementiert API-Aufruf, Aktion und zwei Widgettests für parsed und
failed. Autoprocessing, Polling, Retry, Worker und neue Parserformate bleiben
ausgeschlossen.
