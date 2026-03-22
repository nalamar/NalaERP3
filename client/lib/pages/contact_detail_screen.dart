import 'package:flutter/material.dart';
import '../api.dart';
import '../web/browser.dart' as browser;

class ContactDetailScreen extends StatefulWidget {
  const ContactDetailScreen({super.key, required this.api, required this.id});
  final ApiClient api;
  final String id;

  @override
  State<ContactDetailScreen> createState() => _ContactDetailScreenState();
}

class _ContactDetailScreenState extends State<ContactDetailScreen> {
  Map<String, dynamic>? contact;
  List<dynamic> addresses = [];
  List<dynamic> persons = [];
  List<dynamic> notes = [];
  List<dynamic> tasks = [];
  List<dynamic> documents = [];
  List<dynamic> activity = [];
  bool loading = false;

  String _roleLabel(String value) {
    switch (value) {
      case 'customer':
        return 'Kunde';
      case 'supplier':
        return 'Lieferant';
      case 'partner':
        return 'Partner';
      case 'both':
        return 'Kunde & Lieferant';
      case 'other':
        return 'Sonstige';
      default:
        return value;
    }
  }

  String _statusLabel(String value) {
    switch (value) {
      case 'lead':
        return 'Interessent';
      case 'active':
        return 'Aktiv';
      case 'inactive':
        return 'Inaktiv';
      case 'blocked':
        return 'Gesperrt';
      default:
        return value;
    }
  }

  String _typeLabel(String value) {
    switch (value) {
      case 'org':
        return 'Organisation';
      case 'person':
        return 'Person';
      default:
        return value;
    }
  }

  String _taskStatusLabel(String value) {
    switch (value) {
      case 'open':
        return 'Offen';
      case 'in_progress':
        return 'In Bearbeitung';
      case 'done':
        return 'Erledigt';
      case 'canceled':
        return 'Abgebrochen';
      default:
        return value;
    }
  }

  String _personRoleLabel(String value) {
    switch (value) {
      case 'management':
        return 'Geschäftsführung';
      case 'purchasing':
        return 'Einkauf';
      case 'sales':
        return 'Vertrieb';
      case 'accounting':
        return 'Buchhaltung';
      case 'project':
        return 'Projekt';
      case 'technical':
        return 'Technik';
      case 'other':
        return 'Sonstige';
      default:
        return value;
    }
  }

  String _channelLabel(String value) {
    switch (value) {
      case 'email':
        return 'E-Mail';
      case 'phone':
        return 'Telefon';
      case 'mobile':
        return 'Mobil';
      case 'whatsapp':
        return 'WhatsApp';
      case 'teams':
        return 'Teams';
      case 'other':
        return 'Sonstiges';
      default:
        return value;
    }
  }

  String _formatDateTime(String? value) {
    if (value == null || value.trim().isEmpty) {
      return '—';
    }
    final parsed = DateTime.tryParse(value);
    if (parsed == null) {
      return value;
    }
    final local = parsed.toLocal();
    final day = local.day.toString().padLeft(2, '0');
    final month = local.month.toString().padLeft(2, '0');
    final year = local.year.toString();
    return '$day.$month.$year';
  }

  String _activityTitle(Map<String, dynamic> item) {
    final title = (item['titel'] ?? '').toString().trim();
    if (title.isNotEmpty) {
      return title;
    }
    switch ((item['quelle'] ?? '').toString()) {
      case 'note':
        return 'Notiz';
      case 'task':
        return 'Aufgabe';
      case 'document':
        return 'Dokument';
      default:
        return 'Aktivität';
    }
  }

  String _activitySourceLabel(String value) {
    switch (value) {
      case 'contact':
        return 'Kontakt';
      case 'note':
        return 'Notiz';
      case 'task':
        return 'Aufgabe';
      case 'document':
        return 'Dokument';
      default:
        return value;
    }
  }

  String _errorMessage(Object error,
      {String fallback = 'Vorgang fehlgeschlagen'}) {
    if (error is ApiException) {
      switch (error.code) {
        case 'validation_error':
          if (error.message.toLowerCase().contains('bereits vorhanden')) {
            return 'Mögliche Dublette: ${error.message}';
          }
          return error.message;
        case 'not_found':
          return 'Kontakt oder Unterobjekt nicht gefunden oder nicht mehr verfügbar.';
        case 'internal_error':
          return 'Serverfehler. Bitte erneut versuchen.';
      }
      return error.message;
    }
    return '$fallback: $error';
  }

  @override
  void initState() {
    super.initState();
    _loadAll();
  }

  Future<void> _loadAll() async {
    setState(() => loading = true);
    try {
      final c = await widget.api.getContact(widget.id);
      final a = await widget.api.listContactAddresses(widget.id);
      final p = await widget.api.listContactPersons(widget.id);
      final n = await widget.api.listContactNotes(widget.id);
      final t = await widget.api.listContactTasks(widget.id);
      final d = await widget.api.listContactDocuments(widget.id);
      final h = await widget.api.listContactActivity(widget.id);
      setState(() {
        contact = c;
        addresses = a;
        persons = p;
        notes = n;
        tasks = t;
        documents = d;
        activity = h;
      });
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
              content:
                  Text(_errorMessage(e, fallback: 'Laden fehlgeschlagen'))),
        );
      }
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> _editContact() async {
    if (contact == null) return;
    final c = contact!;
    String typ = (c['typ'] ?? 'org').toString();
    String rolle = (c['rolle'] ?? 'other').toString();
    String status = (c['status'] ?? 'active').toString();
    final name = TextEditingController(text: (c['name'] ?? '').toString());
    final email = TextEditingController(text: (c['email'] ?? '').toString());
    final tel = TextEditingController(text: (c['telefon'] ?? '').toString());
    final ust = TextEditingController(text: (c['ust_id'] ?? '').toString());
    final tax =
        TextEditingController(text: (c['steuernummer'] ?? '').toString());
    final cur =
        TextEditingController(text: (c['waehrung'] ?? 'EUR').toString());
    final paymentTerms = TextEditingController(
        text: (c['zahlungsbedingungen'] ?? '').toString());
    final debtor =
        TextEditingController(text: (c['debitor_nr'] ?? '').toString());
    final creditor =
        TextEditingController(text: (c['kreditor_nr'] ?? '').toString());
    final taxCountry =
        TextEditingController(text: (c['steuer_land'] ?? 'DE').toString());
    bool taxExempt = (c['steuerbefreit'] ?? false) == true;
    await showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
              title: const Text('Kontakt bearbeiten'),
              content: SizedBox(
                width: 600,
                child: SingleChildScrollView(
                  child: Wrap(spacing: 12, runSpacing: 12, children: [
                    SizedBox(
                        width: 160,
                        child: DropdownButtonFormField<String>(
                            value: typ,
                            items: [
                              const DropdownMenuItem(
                                  value: 'org', child: Text('Organisation')),
                              const DropdownMenuItem(
                                  value: 'person', child: Text('Person'))
                            ],
                            onChanged: (v) {
                              typ = v ?? 'org';
                            },
                            decoration:
                                const InputDecoration(labelText: 'Typ'))),
                    SizedBox(
                        width: 200,
                        child: DropdownButtonFormField<String>(
                            value: rolle,
                            items: [
                              const DropdownMenuItem(
                                  value: 'customer', child: Text('Kunde')),
                              const DropdownMenuItem(
                                  value: 'supplier', child: Text('Lieferant')),
                              const DropdownMenuItem(
                                  value: 'partner', child: Text('Partner')),
                              const DropdownMenuItem(
                                  value: 'both',
                                  child: Text('Kunde & Lieferant')),
                              const DropdownMenuItem(
                                  value: 'other', child: Text('Sonstige'))
                            ],
                            onChanged: (v) {
                              rolle = v ?? 'other';
                            },
                            decoration:
                                const InputDecoration(labelText: 'Rolle'))),
                    SizedBox(
                        width: 180,
                        child: DropdownButtonFormField<String>(
                            value: status,
                            items: [
                              const DropdownMenuItem(
                                  value: 'lead', child: Text('Interessent')),
                              const DropdownMenuItem(
                                  value: 'active', child: Text('Aktiv')),
                              const DropdownMenuItem(
                                  value: 'inactive', child: Text('Inaktiv')),
                              const DropdownMenuItem(
                                  value: 'blocked', child: Text('Gesperrt'))
                            ],
                            onChanged: (v) {
                              status = v ?? 'active';
                            },
                            decoration:
                                const InputDecoration(labelText: 'Status'))),
                    SizedBox(
                        width: 260,
                        child: TextFormField(
                            controller: name,
                            decoration:
                                const InputDecoration(labelText: 'Name'))),
                    SizedBox(
                        width: 260,
                        child: TextFormField(
                            controller: email,
                            decoration:
                                const InputDecoration(labelText: 'E-Mail'))),
                    SizedBox(
                        width: 180,
                        child: TextFormField(
                            controller: tel,
                            decoration:
                                const InputDecoration(labelText: 'Telefon'))),
                    SizedBox(
                        width: 180,
                        child: TextFormField(
                            controller: ust,
                            decoration:
                                const InputDecoration(labelText: 'USt-IdNr.'))),
                    SizedBox(
                        width: 180,
                        child: TextFormField(
                            controller: tax,
                            decoration: const InputDecoration(
                                labelText: 'Steuernummer'))),
                    SizedBox(
                        width: 120,
                        child: TextFormField(
                            controller: cur,
                            decoration:
                                const InputDecoration(labelText: 'Währung'))),
                    SizedBox(
                        width: 220,
                        child: TextFormField(
                            controller: paymentTerms,
                            decoration: const InputDecoration(
                                labelText: 'Zahlungsbedingungen'))),
                    SizedBox(
                        width: 180,
                        child: TextFormField(
                            controller: debtor,
                            decoration: const InputDecoration(
                                labelText: 'Debitor-Nr.'))),
                    SizedBox(
                        width: 180,
                        child: TextFormField(
                            controller: creditor,
                            decoration: const InputDecoration(
                                labelText: 'Kreditor-Nr.'))),
                    SizedBox(
                        width: 120,
                        child: TextFormField(
                            controller: taxCountry,
                            decoration: const InputDecoration(
                                labelText: 'Steuerland'))),
                    Row(children: [
                      Checkbox(
                          value: taxExempt,
                          onChanged: (v) {
                            setState(() => taxExempt = v ?? false);
                          }),
                      const Text('Steuerbefreit')
                    ]),
                  ]),
                ),
              ),
              actions: [
                TextButton(
                    onPressed: () => Navigator.of(ctx).pop(),
                    child: const Text('Abbrechen')),
                FilledButton.icon(
                    onPressed: () async {
                      try {
                        final patch = {
                          'typ': typ,
                          'rolle': rolle,
                          'status': status,
                          'name': name.text.trim(),
                          'email': email.text.trim(),
                          'telefon': tel.text.trim(),
                          'ust_id': ust.text.trim(),
                          'steuernummer': tax.text.trim(),
                          'waehrung': cur.text.trim().isEmpty
                              ? 'EUR'
                              : cur.text.trim().toUpperCase(),
                          'zahlungsbedingungen': paymentTerms.text.trim(),
                          'debitor_nr': debtor.text.trim(),
                          'kreditor_nr': creditor.text.trim(),
                          'steuer_land': taxCountry.text.trim().isEmpty
                              ? 'DE'
                              : taxCountry.text.trim().toUpperCase(),
                          'steuerbefreit': taxExempt,
                        };
                        final updated =
                            await widget.api.updateContact(widget.id, patch);
                        setState(() => contact = updated);
                        if (mounted) Navigator.of(ctx).pop();
                      } catch (e) {
                        if (mounted) {
                          ScaffoldMessenger.of(context).showSnackBar(
                            SnackBar(content: Text(_errorMessage(e))),
                          );
                        }
                      }
                    },
                    icon: const Icon(Icons.check),
                    label: const Text('Speichern')),
              ],
            ));
  }

  Future<void> _confirmDelete() async {
    final ok = await showDialog<bool>(
        context: context,
        builder: (ctx) => AlertDialog(
              title: const Text('Kontakt löschen'),
              content: const Text(
                  'Soll der Kontakt (soft delete) deaktiviert werden?'),
              actions: [
                TextButton(
                    onPressed: () => Navigator.of(ctx).pop(false),
                    child: const Text('Abbrechen')),
                FilledButton(
                    onPressed: () => Navigator.of(ctx).pop(true),
                    child: const Text('Löschen')),
              ],
            ));
    if (ok == true) {
      try {
        await widget.api.deleteContact(widget.id);
        if (mounted) Navigator.of(context).pop();
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(_errorMessage(e))),
          );
        }
      }
    }
  }

  Future<void> _editAddress(Map<String, dynamic>? addr) async {
    String art = (addr?['art'] ?? 'billing').toString();
    final z1 = TextEditingController(text: (addr?['zeile1'] ?? '').toString());
    final z2 = TextEditingController(text: (addr?['zeile2'] ?? '').toString());
    final plz = TextEditingController(text: (addr?['plz'] ?? '').toString());
    final ort = TextEditingController(text: (addr?['ort'] ?? '').toString());
    final land = TextEditingController(text: (addr?['land'] ?? '').toString());
    bool primary = (addr?['is_primary'] ?? false) == true;
    final isNew = addr == null;
    await showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
              title: Text(isNew ? 'Adresse hinzufügen' : 'Adresse bearbeiten'),
              content: SizedBox(
                width: 600,
                child: SingleChildScrollView(
                  child: Wrap(spacing: 12, runSpacing: 12, children: [
                    SizedBox(
                        width: 200,
                        child: DropdownButtonFormField<String>(
                            value: art,
                            items: const [
                              DropdownMenuItem(
                                  value: 'billing', child: Text('Rechnung')),
                              DropdownMenuItem(
                                  value: 'shipping', child: Text('Lieferung')),
                              DropdownMenuItem(
                                  value: 'other', child: Text('Sonstige'))
                            ],
                            onChanged: (v) {
                              art = v ?? 'billing';
                            },
                            decoration:
                                const InputDecoration(labelText: 'Art'))),
                    SizedBox(
                        width: 320,
                        child: TextFormField(
                            controller: z1,
                            decoration:
                                const InputDecoration(labelText: 'Zeile 1'))),
                    SizedBox(
                        width: 320,
                        child: TextFormField(
                            controller: z2,
                            decoration:
                                const InputDecoration(labelText: 'Zeile 2'))),
                    SizedBox(
                        width: 140,
                        child: TextFormField(
                            controller: plz,
                            decoration:
                                const InputDecoration(labelText: 'PLZ'))),
                    SizedBox(
                        width: 220,
                        child: TextFormField(
                            controller: ort,
                            decoration:
                                const InputDecoration(labelText: 'Ort'))),
                    SizedBox(
                        width: 120,
                        child: TextFormField(
                            controller: land,
                            decoration:
                                const InputDecoration(labelText: 'Land'))),
                    Row(children: [
                      Checkbox(
                          value: primary,
                          onChanged: (v) =>
                              setState(() => primary = v ?? primary)),
                      const Text('Primär')
                    ]),
                  ]),
                ),
              ),
              actions: [
                if (!isNew)
                  TextButton(
                      onPressed: () async {
                        try {
                          await widget.api.deleteContactAddress(
                              widget.id, (addr!['id'] as String));
                          Navigator.of(ctx).pop();
                          await _loadAll();
                        } catch (e) {
                          if (mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text(_errorMessage(e))),
                            );
                          }
                        }
                      },
                      child: const Text('Löschen')),
                TextButton(
                    onPressed: () => Navigator.of(ctx).pop(),
                    child: const Text('Abbrechen')),
                FilledButton(
                    onPressed: () async {
                      try {
                        if (isNew) {
                          await widget.api.createContactAddress(widget.id, {
                            'art': art,
                            'zeile1': z1.text.trim(),
                            'zeile2': z2.text.trim(),
                            'plz': plz.text.trim(),
                            'ort': ort.text.trim(),
                            'land': land.text.trim(),
                            'is_primary': primary
                          });
                        } else {
                          await widget.api.updateContactAddress(
                              widget.id, (addr!['id'] as String), {
                            'art': art,
                            'zeile1': z1.text.trim(),
                            'zeile2': z2.text.trim(),
                            'plz': plz.text.trim(),
                            'ort': ort.text.trim(),
                            'land': land.text.trim(),
                            'is_primary': primary
                          });
                        }
                        if (mounted) Navigator.of(ctx).pop();
                        await _loadAll();
                      } catch (e) {
                        if (mounted) {
                          ScaffoldMessenger.of(context).showSnackBar(
                            SnackBar(content: Text(_errorMessage(e))),
                          );
                        }
                      }
                    },
                    child: const Text('Speichern')),
              ],
            ));
  }

  Future<void> _editPerson(Map<String, dynamic>? pers) async {
    final isNew = pers == null;
    final anr = TextEditingController(text: (pers?['anrede'] ?? '').toString());
    final v = TextEditingController(text: (pers?['vorname'] ?? '').toString());
    final n = TextEditingController(text: (pers?['nachname'] ?? '').toString());
    final pos =
        TextEditingController(text: (pers?['position'] ?? '').toString());
    String role = (pers?['rolle'] ?? 'other').toString();
    String channel = (pers?['bevorzugter_kanal'] ?? '').toString();
    final em = TextEditingController(text: (pers?['email'] ?? '').toString());
    final ph = TextEditingController(text: (pers?['telefon'] ?? '').toString());
    final mo = TextEditingController(text: (pers?['mobil'] ?? '').toString());
    bool primary = (pers?['is_primary'] ?? false) == true;
    await showDialog(
        context: context,
        builder: (ctx) => StatefulBuilder(
              builder: (ctx, setModalState) => AlertDialog(
                title: Text(isNew
                    ? 'Ansprechpartner hinzufügen'
                    : 'Ansprechpartner bearbeiten'),
                content: SizedBox(
                  width: 640,
                  child: SingleChildScrollView(
                    child: Wrap(spacing: 12, runSpacing: 12, children: [
                      SizedBox(
                          width: 120,
                          child: TextFormField(
                              controller: anr,
                              decoration:
                                  const InputDecoration(labelText: 'Anrede'))),
                      SizedBox(
                          width: 180,
                          child: TextFormField(
                              controller: v,
                              decoration:
                                  const InputDecoration(labelText: 'Vorname'))),
                      SizedBox(
                          width: 180,
                          child: TextFormField(
                              controller: n,
                              decoration: const InputDecoration(
                                  labelText: 'Nachname'))),
                      SizedBox(
                          width: 220,
                          child: TextFormField(
                              controller: pos,
                              decoration: const InputDecoration(
                                  labelText: 'Position'))),
                      SizedBox(
                          width: 180,
                          child: DropdownButtonFormField<String>(
                            value: role,
                            items: const [
                              DropdownMenuItem(
                                  value: 'management',
                                  child: Text('Geschäftsführung')),
                              DropdownMenuItem(
                                  value: 'purchasing', child: Text('Einkauf')),
                              DropdownMenuItem(
                                  value: 'sales', child: Text('Vertrieb')),
                              DropdownMenuItem(
                                  value: 'accounting',
                                  child: Text('Buchhaltung')),
                              DropdownMenuItem(
                                  value: 'project', child: Text('Projekt')),
                              DropdownMenuItem(
                                  value: 'technical', child: Text('Technik')),
                              DropdownMenuItem(
                                  value: 'other', child: Text('Sonstige')),
                            ],
                            onChanged: (val) =>
                                setModalState(() => role = val ?? 'other'),
                            decoration:
                                const InputDecoration(labelText: 'Rolle'),
                          )),
                      SizedBox(
                          width: 180,
                          child: DropdownButtonFormField<String>(
                            value: channel.isEmpty ? null : channel,
                            items: const [
                              DropdownMenuItem(
                                  value: 'email', child: Text('E-Mail')),
                              DropdownMenuItem(
                                  value: 'phone', child: Text('Telefon')),
                              DropdownMenuItem(
                                  value: 'mobile', child: Text('Mobil')),
                              DropdownMenuItem(
                                  value: 'whatsapp', child: Text('WhatsApp')),
                              DropdownMenuItem(
                                  value: 'teams', child: Text('Teams')),
                              DropdownMenuItem(
                                  value: 'other', child: Text('Sonstiges')),
                            ],
                            onChanged: (val) =>
                                setModalState(() => channel = val ?? ''),
                            decoration: const InputDecoration(
                                labelText: 'Bevorzugter Kanal'),
                          )),
                      SizedBox(
                          width: 220,
                          child: TextFormField(
                              controller: em,
                              decoration:
                                  const InputDecoration(labelText: 'E-Mail'))),
                      SizedBox(
                          width: 180,
                          child: TextFormField(
                              controller: ph,
                              decoration:
                                  const InputDecoration(labelText: 'Telefon'))),
                      SizedBox(
                          width: 180,
                          child: TextFormField(
                              controller: mo,
                              decoration:
                                  const InputDecoration(labelText: 'Mobil'))),
                      Row(children: [
                        Checkbox(
                            value: primary,
                            onChanged: (val) =>
                                setModalState(() => primary = val ?? primary)),
                        const Text('Primär')
                      ]),
                    ]),
                  ),
                ),
                actions: [
                  if (!isNew)
                    TextButton(
                        onPressed: () async {
                          try {
                            await widget.api.deleteContactPerson(
                                widget.id, (pers!['id'] as String));
                            Navigator.of(ctx).pop();
                            await _loadAll();
                          } catch (e) {
                            if (mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(content: Text(_errorMessage(e))),
                              );
                            }
                          }
                        },
                        child: const Text('Löschen')),
                  TextButton(
                      onPressed: () => Navigator.of(ctx).pop(),
                      child: const Text('Abbrechen')),
                  FilledButton(
                      onPressed: () async {
                        try {
                          final payload = {
                            'anrede': anr.text.trim(),
                            'vorname': v.text.trim(),
                            'nachname': n.text.trim(),
                            'position': pos.text.trim(),
                            'rolle': role,
                            'bevorzugter_kanal': channel,
                            'email': em.text.trim(),
                            'telefon': ph.text.trim(),
                            'mobil': mo.text.trim(),
                            'is_primary': primary,
                          };
                          if (isNew) {
                            await widget.api
                                .createContactPerson(widget.id, payload);
                          } else {
                            await widget.api.updateContactPerson(
                                widget.id, (pers!['id'] as String), payload);
                          }
                          if (mounted) Navigator.of(ctx).pop();
                          await _loadAll();
                        } catch (e) {
                          if (mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text(_errorMessage(e))),
                            );
                          }
                        }
                      },
                      child: const Text('Speichern')),
                ],
              ),
            ));
  }

  Future<void> _editNote(Map<String, dynamic>? note) async {
    final isNew = note == null;
    final titel =
        TextEditingController(text: (note?['titel'] ?? '').toString());
    final inhalt =
        TextEditingController(text: (note?['inhalt'] ?? '').toString());
    await showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
              title: Text(isNew ? 'Notiz hinzufügen' : 'Notiz bearbeiten'),
              content: SizedBox(
                width: 560,
                child: SingleChildScrollView(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      TextFormField(
                          controller: titel,
                          decoration:
                              const InputDecoration(labelText: 'Titel')),
                      const SizedBox(height: 12),
                      TextFormField(
                          controller: inhalt,
                          minLines: 4,
                          maxLines: 8,
                          decoration:
                              const InputDecoration(labelText: 'Inhalt')),
                    ],
                  ),
                ),
              ),
              actions: [
                if (!isNew)
                  TextButton(
                      onPressed: () async {
                        try {
                          await widget.api.deleteContactNote(
                              widget.id, (note!['id'] as String));
                          if (mounted) Navigator.of(ctx).pop();
                          await _loadAll();
                        } catch (e) {
                          if (mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text(_errorMessage(e))),
                            );
                          }
                        }
                      },
                      child: const Text('Löschen')),
                TextButton(
                    onPressed: () => Navigator.of(ctx).pop(),
                    child: const Text('Abbrechen')),
                FilledButton(
                    onPressed: () async {
                      try {
                        if (isNew) {
                          await widget.api.createContactNote(widget.id, {
                            'titel': titel.text.trim(),
                            'inhalt': inhalt.text.trim()
                          });
                        } else {
                          await widget.api.updateContactNote(
                              widget.id, (note!['id'] as String), {
                            'titel': titel.text.trim(),
                            'inhalt': inhalt.text.trim()
                          });
                        }
                        if (mounted) Navigator.of(ctx).pop();
                        await _loadAll();
                      } catch (e) {
                        if (mounted) {
                          ScaffoldMessenger.of(context).showSnackBar(
                            SnackBar(content: Text(_errorMessage(e))),
                          );
                        }
                      }
                    },
                    child: const Text('Speichern')),
              ],
            ));
  }

  Future<void> _editTask(Map<String, dynamic>? task) async {
    final isNew = task == null;
    final titel =
        TextEditingController(text: (task?['titel'] ?? '').toString());
    final beschreibung =
        TextEditingController(text: (task?['beschreibung'] ?? '').toString());
    String status = (task?['status'] ?? 'open').toString();
    DateTime? dueDate =
        DateTime.tryParse((task?['faellig_am'] ?? '').toString())?.toLocal();
    await showDialog(
        context: context,
        builder: (ctx) => StatefulBuilder(
              builder: (ctx, setModalState) => AlertDialog(
                title:
                    Text(isNew ? 'Aufgabe hinzufügen' : 'Aufgabe bearbeiten'),
                content: SizedBox(
                  width: 560,
                  child: SingleChildScrollView(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        TextFormField(
                            controller: titel,
                            decoration:
                                const InputDecoration(labelText: 'Titel')),
                        const SizedBox(height: 12),
                        DropdownButtonFormField<String>(
                          value: status,
                          items: const [
                            DropdownMenuItem(
                                value: 'open', child: Text('Offen')),
                            DropdownMenuItem(
                                value: 'in_progress',
                                child: Text('In Bearbeitung')),
                            DropdownMenuItem(
                                value: 'done', child: Text('Erledigt')),
                            DropdownMenuItem(
                                value: 'canceled', child: Text('Abgebrochen')),
                          ],
                          onChanged: (v) =>
                              setModalState(() => status = v ?? 'open'),
                          decoration:
                              const InputDecoration(labelText: 'Status'),
                        ),
                        const SizedBox(height: 12),
                        TextFormField(
                            controller: beschreibung,
                            minLines: 3,
                            maxLines: 6,
                            decoration: const InputDecoration(
                                labelText: 'Beschreibung')),
                        const SizedBox(height: 12),
                        Row(
                          children: [
                            Expanded(
                              child: Text(
                                  'Fällig am: ${dueDate == null ? '—' : _formatDateTime(dueDate!.toUtc().toIso8601String())}'),
                            ),
                            TextButton.icon(
                              onPressed: () async {
                                final picked = await showDatePicker(
                                  context: ctx,
                                  initialDate: dueDate ?? DateTime.now(),
                                  firstDate: DateTime(2020),
                                  lastDate: DateTime(2100),
                                );
                                if (picked != null) {
                                  setModalState(() => dueDate = picked);
                                }
                              },
                              icon: const Icon(Icons.event),
                              label: const Text('Datum'),
                            ),
                            if (dueDate != null)
                              IconButton(
                                onPressed: () =>
                                    setModalState(() => dueDate = null),
                                icon: const Icon(Icons.clear),
                                tooltip: 'Fälligkeit entfernen',
                              ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ),
                actions: [
                  if (!isNew)
                    TextButton(
                        onPressed: () async {
                          try {
                            await widget.api.deleteContactTask(
                                widget.id, (task!['id'] as String));
                            if (mounted) Navigator.of(ctx).pop();
                            await _loadAll();
                          } catch (e) {
                            if (mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(content: Text(_errorMessage(e))),
                              );
                            }
                          }
                        },
                        child: const Text('Löschen')),
                  TextButton(
                      onPressed: () => Navigator.of(ctx).pop(),
                      child: const Text('Abbrechen')),
                  FilledButton(
                      onPressed: () async {
                        try {
                          final payload = {
                            'titel': titel.text.trim(),
                            'beschreibung': beschreibung.text.trim(),
                            'status': status,
                            'faellig_am': dueDate == null
                                ? ''
                                : DateTime(dueDate!.year, dueDate!.month,
                                        dueDate!.day)
                                    .toUtc()
                                    .toIso8601String(),
                          };
                          if (isNew) {
                            await widget.api
                                .createContactTask(widget.id, payload);
                          } else {
                            await widget.api.updateContactTask(
                                widget.id, (task!['id'] as String), payload);
                          }
                          if (mounted) Navigator.of(ctx).pop();
                          await _loadAll();
                        } catch (e) {
                          if (mounted) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text(_errorMessage(e))),
                            );
                          }
                        }
                      },
                      child: const Text('Speichern')),
                ],
              ),
            ));
  }

  Future<void> _uploadDocument() async {
    final picked = await browser.pickFile(accept: '*/*');
    if (picked == null) return;
    try {
      await widget.api.uploadContactDocument(
        widget.id,
        picked.filename,
        picked.bytes,
        contentType: picked.contentType,
      );
      await _loadAll();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Dokument hochgeladen')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
              content: Text(_errorMessage(e,
                  fallback: 'Dokument-Upload fehlgeschlagen'))),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    final canWrite = widget.api.hasPermission('contacts.write');
    final c = contact;
    return Scaffold(
      appBar: AppBar(
        backgroundColor: color,
        foregroundColor: Colors.white,
        title:
            Text(c == null ? 'Kontakt' : (c['name'] ?? 'Kontakt').toString()),
        actions: [
          IconButton(onPressed: _loadAll, icon: const Icon(Icons.refresh)),
          if (canWrite) ...[
            IconButton(onPressed: _editContact, icon: const Icon(Icons.edit)),
            IconButton(
                onPressed: _confirmDelete,
                icon: const Icon(Icons.delete_outline)),
          ],
        ],
      ),
      floatingActionButtonLocation: FloatingActionButtonLocation.endFloat,
      floatingActionButton: canWrite
          ? Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                FloatingActionButton.small(
                    heroTag: 'addr_add',
                    onPressed: () => _editAddress(null),
                    child: const Icon(Icons.add_location_alt)),
                const SizedBox(height: 8),
                FloatingActionButton.small(
                    heroTag: 'pers_add',
                    onPressed: () => _editPerson(null),
                    child: const Icon(Icons.person_add_alt)),
                const SizedBox(height: 8),
                FloatingActionButton.small(
                    heroTag: 'note_add',
                    onPressed: () => _editNote(null),
                    child: const Icon(Icons.note_add_outlined)),
                const SizedBox(height: 8),
                FloatingActionButton.small(
                    heroTag: 'task_add',
                    onPressed: () => _editTask(null),
                    child: const Icon(Icons.task_alt_outlined)),
                const SizedBox(height: 8),
                FloatingActionButton.small(
                    heroTag: 'doc_add',
                    onPressed: _uploadDocument,
                    child: const Icon(Icons.upload_file_outlined)),
              ],
            )
          : null,
      body: loading && c == null
          ? const Center(child: CircularProgressIndicator())
          : Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (c != null) ...[
                    Text(
                        'Typ: ${_typeLabel((c['typ'] ?? '').toString())}  •  Rolle: ${_roleLabel((c['rolle'] ?? '').toString())}  •  Status: ${_statusLabel((c['status'] ?? 'active').toString())}  •  Währung: ${c['waehrung'] ?? 'EUR'}'),
                    const SizedBox(height: 8),
                    Text(
                        'E-Mail: ${c['email'] ?? ''}  •  Telefon: ${c['telefon'] ?? ''}'),
                    const SizedBox(height: 8),
                    Text(
                        'Zahlungsbedingungen: ${(c['zahlungsbedingungen'] ?? '').toString().isEmpty ? '—' : c['zahlungsbedingungen']}  •  Debitor: ${(c['debitor_nr'] ?? '').toString().isEmpty ? '—' : c['debitor_nr']}  •  Kreditor: ${(c['kreditor_nr'] ?? '').toString().isEmpty ? '—' : c['kreditor_nr']}'),
                    const SizedBox(height: 8),
                    Text(
                        'Steuerland: ${(c['steuer_land'] ?? 'DE')}  •  Steuerbefreit: ${(c['steuerbefreit'] ?? false) == true ? 'Ja' : 'Nein'}'),
                    const Divider(),
                  ],
                  Expanded(
                    child: Row(
                      children: [
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text('Adressen',
                                  style: TextStyle(
                                      fontSize: 16,
                                      fontWeight: FontWeight.bold)),
                              const SizedBox(height: 6),
                              Expanded(
                                child: ListView.builder(
                                  itemCount: addresses.length,
                                  itemBuilder: (ctx, i) {
                                    final a =
                                        addresses[i] as Map<String, dynamic>;
                                    return ListTile(
                                      dense: true,
                                      title: Text(
                                          '${a['zeile1'] ?? ''}${(a['zeile2'] ?? '') != '' ? ' • ${a['zeile2']}' : ''}'),
                                      subtitle: Text(
                                          '${a['plz'] ?? ''} ${a['ort'] ?? ''} ${a['land'] ?? ''}  •  ${a['art'] ?? ''}${a['is_primary'] == true ? ' • Primär' : ''}'),
                                      trailing: canWrite
                                          ? IconButton(
                                              icon: const Icon(Icons.edit),
                                              onPressed: () => _editAddress(a))
                                          : null,
                                    );
                                  },
                                ),
                              ),
                            ],
                          ),
                        ),
                        const VerticalDivider(width: 1),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text('Ansprechpartner',
                                  style: TextStyle(
                                      fontSize: 16,
                                      fontWeight: FontWeight.bold)),
                              const SizedBox(height: 6),
                              Expanded(
                                child: ListView.builder(
                                  itemCount: persons.length,
                                  itemBuilder: (ctx, i) {
                                    final p =
                                        persons[i] as Map<String, dynamic>;
                                    final name =
                                        '${p['vorname'] ?? ''} ${p['nachname'] ?? ''}'
                                            .trim();
                                    final role =
                                        (p['rolle'] ?? '').toString().trim();
                                    final channel =
                                        (p['bevorzugter_kanal'] ?? '')
                                            .toString()
                                            .trim();
                                    return ListTile(
                                      dense: true,
                                      title: Text(name.isEmpty
                                          ? (p['email'] ?? '').toString()
                                          : name),
                                      subtitle: Text(
                                          '${p['position'] ?? ''}${role.isNotEmpty ? '  •  ${_personRoleLabel(role)}' : ''}${channel.isNotEmpty ? '  •  ${_channelLabel(channel)}' : ''}  •  ${p['email'] ?? ''}  •  ${p['telefon'] ?? ''}${p['is_primary'] == true ? ' • Primär' : ''}'),
                                      trailing: canWrite
                                          ? IconButton(
                                              icon: const Icon(Icons.edit),
                                              onPressed: () => _editPerson(p))
                                          : null,
                                    );
                                  },
                                ),
                              ),
                            ],
                          ),
                        ),
                        const VerticalDivider(width: 1),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text('Notizen',
                                  style: TextStyle(
                                      fontSize: 16,
                                      fontWeight: FontWeight.bold)),
                              const SizedBox(height: 6),
                              Expanded(
                                child: ListView.builder(
                                  itemCount: notes.length,
                                  itemBuilder: (ctx, i) {
                                    final n = notes[i] as Map<String, dynamic>;
                                    final title =
                                        (n['titel'] ?? '').toString().trim();
                                    final body =
                                        (n['inhalt'] ?? '').toString().trim();
                                    return ListTile(
                                      dense: true,
                                      title: Text(
                                          title.isEmpty ? 'Ohne Titel' : title),
                                      subtitle: Text(
                                        body.isEmpty ? '—' : body,
                                        maxLines: 4,
                                        overflow: TextOverflow.ellipsis,
                                      ),
                                      trailing: canWrite
                                          ? IconButton(
                                              icon: const Icon(Icons.edit),
                                              onPressed: () => _editNote(n))
                                          : null,
                                    );
                                  },
                                ),
                              ),
                            ],
                          ),
                        ),
                        const VerticalDivider(width: 1),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text('Aufgaben',
                                  style: TextStyle(
                                      fontSize: 16,
                                      fontWeight: FontWeight.bold)),
                              const SizedBox(height: 6),
                              Expanded(
                                child: ListView.builder(
                                  itemCount: tasks.length,
                                  itemBuilder: (ctx, i) {
                                    final t = tasks[i] as Map<String, dynamic>;
                                    final due = _formatDateTime(
                                        (t['faellig_am'] ?? '').toString());
                                    final done = _formatDateTime(
                                        (t['erledigt_am'] ?? '').toString());
                                    final description =
                                        (t['beschreibung'] ?? '')
                                            .toString()
                                            .trim();
                                    return ListTile(
                                      dense: true,
                                      title: Text((t['titel'] ?? '')
                                              .toString()
                                              .trim()
                                              .isEmpty
                                          ? 'Ohne Titel'
                                          : (t['titel'] ?? '').toString()),
                                      subtitle: Text(
                                        'Status: ${_taskStatusLabel((t['status'] ?? 'open').toString())}  •  Fällig: $due${done != '—' ? '  •  Erledigt: $done' : ''}${description.isNotEmpty ? '\n$description' : ''}',
                                        maxLines: 4,
                                        overflow: TextOverflow.ellipsis,
                                      ),
                                      trailing: canWrite
                                          ? IconButton(
                                              icon: const Icon(Icons.edit),
                                              onPressed: () => _editTask(t))
                                          : null,
                                    );
                                  },
                                ),
                              ),
                            ],
                          ),
                        ),
                        const VerticalDivider(width: 1),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text('Dokumente',
                                  style: TextStyle(
                                      fontSize: 16,
                                      fontWeight: FontWeight.bold)),
                              const SizedBox(height: 6),
                              Expanded(
                                child: ListView.builder(
                                  itemCount: documents.length,
                                  itemBuilder: (ctx, i) {
                                    final d =
                                        documents[i] as Map<String, dynamic>;
                                    return ListTile(
                                      dense: true,
                                      leading: const Icon(
                                          Icons.description_outlined),
                                      title: Text(
                                          (d['filename'] ?? d['document_id'])
                                              .toString()),
                                      subtitle: Text(
                                          '${d['content_type'] ?? ''}  •  ${d['length'] ?? 0} B'),
                                      trailing: IconButton(
                                        icon: const Icon(Icons.download),
                                        onPressed: () =>
                                            widget.api.downloadDocument(
                                          (d['document_id'] ?? '').toString(),
                                          filename: (d['filename'] ?? '')
                                                  .toString()
                                                  .isEmpty
                                              ? null
                                              : (d['filename'] ?? '')
                                                  .toString(),
                                        ),
                                      ),
                                    );
                                  },
                                ),
                              ),
                            ],
                          ),
                        ),
                        const VerticalDivider(width: 1),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text('Verlauf',
                                  style: TextStyle(
                                      fontSize: 16,
                                      fontWeight: FontWeight.bold)),
                              const SizedBox(height: 6),
                              Expanded(
                                child: activity.isEmpty
                                    ? const Center(
                                        child: Text('Noch keine Aktivitäten'))
                                    : ListView.builder(
                                        itemCount: activity.length,
                                        itemBuilder: (ctx, i) {
                                          final item = activity[i]
                                              as Map<String, dynamic>;
                                          final title = _activityTitle(item);
                                          final description =
                                              (item['beschreibung'] ?? '')
                                                  .toString()
                                                  .trim();
                                          final source = _activitySourceLabel(
                                              (item['quelle'] ?? '')
                                                  .toString());
                                          final when = _formatDateTime(
                                              (item['zeitpunkt'] ?? '')
                                                  .toString());
                                          return ListTile(
                                            dense: true,
                                            title: Text(title),
                                            subtitle: Text(
                                              '$source  •  $when${description.isNotEmpty ? '\n$description' : ''}',
                                              maxLines: 4,
                                              overflow: TextOverflow.ellipsis,
                                            ),
                                          );
                                        },
                                      ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
    );
  }
}
