# GAEB-Preisvorschlag nach gesetztem Material: Audit

## Ziel dieses Audits

Dieses Audit bewertet den jetzt umgesetzten kleinen Preisvorschlagspfad nach
gesetztem Material.

Geprueft wird, ob innerhalb dieses engen Scopes vor spaeterer
Preisuebernahme-, Ranking-, Historien- oder Automatiklogik noch genau ein
kleiner Haertungsschritt mit gutem Signal uebrig ist.

## 1. Umgesetzter Scope

Der aktuelle Block deckt jetzt bewusst nur den kleinsten Preisvorschlagspfad
ab:

- read-only Preisanker fuer genau eine bereits gemappte Draft-Quote-Position
- Preisquelle ausschliesslich aus dem vorhandenen Materialstamm
- positionsnahe Anzeige im bestehenden Quote-Editor
- weiterhin keine Preisuebernahme im selben Block

## 2. Was jetzt bereits sauber funktioniert

Der Block ist in seinem engen Zuschnitt konsistent:

- Backend liefert einen kleinen Preisanker nur fuer genau eine Position
- Guard Rails verhindern Zugriff auf historische, nicht bearbeitbare oder
  nicht gemappte Positionen
- der Vorschlag ist als read-only Hilfe sichtbar und veraendert `unit_price`
  nicht
- die UI oeffnet keine zweite Preisoberflaeche, sondern bleibt an der
  bestehenden Quote-Position

Damit ist die beabsichtigte Minimalwirkung erreicht:

- nach gesetztem Material ist ein kleiner kommerzieller Anschluss sichtbar
- ohne bereits in Preisuebernahme oder Kalkulationslogik abzugleiten

## 3. Gepruefte moegliche Restschritte innerhalb dieses Scopes

Als moegliche Resthaertung innerhalb desselben Blocks kommen theoretisch noch
in Frage:

- kosmetische Umbenennung oder Textschaerfung der Preisquelle
- zusaetzliche Leerzustands-Varianten im Client
- weitere kleine Read-only-Metadaten am Preisanker
- fruehe Preisuebernahme direkt aus dem Preisvorschlag

Diese Kandidaten liefern in diesem Moment aber keinen besseren Signalwert:

- reine Text- oder Anzeigejustierung verbessert den fachlichen Fluss kaum
- weitere Read-only-Metadaten wuerden den Scope aufblasen, ohne den naechsten
  operativen Schmerzpunkt zu loesen
- Preisuebernahme waere bereits der naechste Folgeschritt ausserhalb dieses
  Read-only-Blocks

## 4. Audit-Entscheidung

Die Entscheidung dieses Audits ist:

- der Block `kleiner Preisvorschlag` ist in seinem engen Scope sauber
  abgeschlossen
- innerhalb dieses Blocks bleibt vor spaeterer Preisuebernahme-, Ranking-,
  Historien- oder Automatiklogik kein weiterer kleiner Haertungsschritt mit
  gutem Signal uebrig

## 5. Warum der Block jetzt endet

Der aktuelle Block sollte hier enden, weil sein Zweck erreicht ist:

- Material ist gesetzt
- ein kleiner Preisanker ist sichtbar
- der Preis wird noch nicht automatisch oder halbautomatisch gesetzt

Alles, was jetzt echten Zusatznutzen stiftet, verlaesst bereits diesen engen
Read-only-Scope und gehoert in die naechste Folgestufe.

## 6. Naechste sinnvolle Folgerichtung

Nach diesem Audit ist der naechste sinnvolle Schritt nicht weiterer
Read-only-Feinschliff, sondern eine neue Folgestufe ausserhalb dieses Blocks.

Der naechste sinnvolle Inventurpunkt ist:

- fachlich entscheiden, ob als naechstes eine enge explizite Preisuebernahme
  aus dem sichtbaren Preisanker oder ein anderer minimaler kommerzieller
  Folgeausbau den besten Signalwert hat
