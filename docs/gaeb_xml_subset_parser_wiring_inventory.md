# GAEB: Folgeinventar nach XML-Subset-Parser

Subtask 3.1.76.1 ist abgeschlossen. Der Parser existiert und der Process-
Endpunkt kann Adapter injizieren, doch der Standardrouter konfiguriert ihn
noch nicht. Damit bleibt der reale XML-Pfad im laufenden API-Prozess auf 500
begrenzt.

Der kleinste Anschluss ist keine neue Funktion, sondern eine explizite
Produktbereitstellung von GAEBXMLSubsetParser am Router-Zusammensetzungspunkt.
Sie muss rückwärtskompatibel bleiben und darf nur den dokumentierten
xml-Subset aktivieren. Uploadcallback, Worker, Binärformate und KI bleiben
ausgeschlossen.

Folgeleaves:

1. 3.1.76.2 – Bereitstellungs- und Aktivierungsstrategie definieren.
2. 3.1.76.3 – Parser im Produktionsrouter verdrahten und Endpunktintegration
   gegen echte XML-Quelle testen.
3. 3.1.76.4 – Verdrahtung und Nachweise auditieren.
