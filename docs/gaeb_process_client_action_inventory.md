# GAEB: Folgeinventar nach produktiver XML-Verarbeitung

Subtask 3.1.77.1 ist abgeschlossen. Upload und Process-Endpunkt sind nun
produktiv vorhanden, doch QuotesPage kann einen uploaded-Import nicht
ausdrücklich verarbeiten. Der Nutzer erhält nach dem Upload nur die
Importliste; Review und Apply bleiben ohne sichtbaren Übergang unerreichbar.

Der kleinste Folgeausbau ist eine positionsnahe Aktion Verarbeiten nur für
Importe im Status uploaded. Sie ruft den vorhandenen Process-Endpunkt auf,
zeigt Fortschritt, ersetzt den Import durch den Ergebnisstatus und zeigt
parsed oder failed transparent an.

Nicht Teil sind Autoprocessing nach Upload, Polling, Worker, Retry,
Binärformate, KI, Mapping oder Kalkulation.

Folgeleaves:

1. 3.1.77.2 – API-Client-, UI- und Fehlervertrag für die Prozessaktion definieren.
2. 3.1.77.3 – Clientaktion und Widgettests für parsed und failed implementieren.
3. 3.1.77.4 – Clientaktion und Nachweise auditieren.
