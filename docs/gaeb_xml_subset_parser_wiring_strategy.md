# GAEB: Aktivierungsstrategie für den XML-Subset-Parser

Subtask 3.1.76.2 ist abgeschlossen. NewV1Router soll den vorhandenen
GAEBXMLSubsetParser als Standard über NewV1RouterWithOptions aktivieren.
Explizit übergebene V1RouterOptions behalten Vorrang; damit bleiben Tests und
spätere Parseradapter injizierbar.

Der Produktionsrouter verarbeitet damit nur die dokumentierte XML-Subset-
Quelle. Ein POST process auf gültige XML führt nach parsed, während
unvollständige XML über den bestehenden Service nach failed führt. X83 und
andere nicht unterstützte Formate werden nicht als Erfolg behandelt.

3.1.76.3 ergänzt nur diese Defaultoption und einen HTTP-Ende-zu-Ende-Test mit
einer echten XML-Quelle. Keine Änderung an Upload, Worker, Client, Persistenz
oder KI.
