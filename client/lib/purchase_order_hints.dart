// Hinweise der Nachweise an der Bestellung (ADR 0035, Backlog H.4.5a).
// Der Server berechnet sie beim Abruf (Feld `hinweise`); der Client
// uebersetzt nur die Codes.

String purchaseOrderHintLabel(String code) {
  switch (code) {
    case 'zertifikat_fehlt':
      return 'EN-1090-Zertifikat des Lieferanten fehlt';
    case 'zertifikat_ungueltig':
      return 'EN-1090-Zertifikat deckt den Zeitraum nicht ab';
    case 'leistungserklaerung_fehlt':
      return 'Leistungserklärung fehlt';
  }
  return code;
}

List<String> purchaseOrderHints(Map<String, dynamic> order) {
  final raw = order['hinweise'];
  if (raw is! List) return const [];
  return [
    for (final h in raw)
      if (h.toString().trim().isNotEmpty) purchaseOrderHintLabel(h.toString())
  ];
}
