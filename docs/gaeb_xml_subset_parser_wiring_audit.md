# GAEB: Audit der XML-Subset-Parserverdrahtung

Subtask 3.1.76.4 ist ohne Befund abgeschlossen. NewV1Router aktiviert nur den
vorhandenen GAEBXMLSubsetParser; explizite Routeroptionen bleiben unverändert.
Der HTTP-Integrationstest belegt eine echte XML-Quelle über den
Standardrouter bis parsed mit Parserkennung und persistierter Position.
Gleichzeitig bleibt der explizite Router ohne Parser beim kontrollierten
500-Fehler ohne Mutation. Der gezielte Go-Test und git diff --check sind
gruen. Keine Binärformate, Automationen oder KI wurden ergänzt.
