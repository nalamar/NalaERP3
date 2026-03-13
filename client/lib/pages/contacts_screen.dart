import 'package:flutter/material.dart';
import '../api.dart';
import 'contact_detail_screen.dart';

class ContactsScreen extends StatefulWidget {
  const ContactsScreen({super.key, required this.api});
  final ApiClient api;

  @override
  State<ContactsScreen> createState() => _ContactsScreenState();
}

class _ContactsScreenState extends State<ContactsScreen> {
  List<dynamic> items = [];
  bool loading = false;
  final searchCtrl = TextEditingController();
  String? filterRolle;
  String? filterStatus;
  String? filterTyp;
  List<String> rollen = [];
  List<String> statuses = [];
  List<String> typen = [];
  int limit = 20;
  int offset = 0;

  // Create dialog controllers
  final formKey = GlobalKey<FormState>();
  String newTyp = 'org';
  String newRolle = 'customer';
  String newStatus = 'active';
  final nameCtrl = TextEditingController();
  final emailCtrl = TextEditingController();
  final phoneCtrl = TextEditingController();
  final ustCtrl = TextEditingController();
  final taxCtrl = TextEditingController();
  final currCtrl = TextEditingController(text: 'EUR');
  final paymentTermsCtrl = TextEditingController();
  final debtorCtrl = TextEditingController();
  final creditorCtrl = TextEditingController();
  final taxCountryCtrl = TextEditingController(text: 'DE');
  bool taxExempt = false;
  // Address
  bool addrPrimary = true;
  String addrArt = 'billing';
  final addrZ1 = TextEditingController();
  final addrZ2 = TextEditingController();
  final addrPLZ = TextEditingController();
  final addrOrt = TextEditingController();
  final addrLand = TextEditingController(text: 'DE');
  // Person
  bool personPrimary = true;
  final pAnrede = TextEditingController();
  final pVorname = TextEditingController();
  final pNachname = TextEditingController();
  final pPosition = TextEditingController();
  final pEmail = TextEditingController();
  final pPhone = TextEditingController();
  final pMobil = TextEditingController();

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

  String _errorMessage(Object error, {String fallback = 'Vorgang fehlgeschlagen'}) {
    if (error is ApiException) {
      switch (error.code) {
        case 'validation_error':
          if (error.message.toLowerCase().contains('bereits vorhanden')) {
            return 'Mögliche Dublette: ${error.message}';
          }
          return error.message;
        case 'not_found':
          return 'Kontakt nicht gefunden oder nicht mehr verfügbar.';
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
    _loadFacets();
    _reload();
  }

  Future<void> _loadFacets() async {
    try {
      final r = await widget.api.listContactRoles();
      final s = await widget.api.listContactStatuses();
      final t = await widget.api.listContactTypes();
      setState(() { rollen = r; statuses = s; typen = t; });
    } catch (e) { debugPrint('Facets error: $e'); }
  }

  Future<void> _reload() async {
    setState(() => loading = true);
    try {
      offset = 0;
      items = await widget.api.listContacts(
        q: searchCtrl.text.trim(), rolle: filterRolle, status: filterStatus, typ: filterTyp, limit: limit, offset: offset,
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(_errorMessage(e, fallback: 'Kontakte konnten nicht geladen werden'))),
        );
      }
    } finally { setState(() => loading = false); }
  }

  Future<void> _loadMore() async {
    final next = await widget.api.listContacts(
      q: searchCtrl.text.trim(), rolle: filterRolle, status: filterStatus, typ: filterTyp, limit: limit, offset: offset + limit,
    );
    if (next.isNotEmpty) {
      offset += limit;
      setState(() { items.addAll(next); });
    }
  }

  Future<void> _openCreateDialog() async {
    await showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Kontakt anlegen'),
        content: SizedBox(
          width: 700,
          child: Form(
            key: formKey,
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Wrap(spacing: 12, runSpacing: 12, children: [
                    SizedBox(
                      width: 180,
                      child: DropdownButtonFormField<String>(
                        value: newTyp,
                        items: [for (final t in (typen.isEmpty ? ['org','person']: typen)) DropdownMenuItem(value: t, child: Text(_typeLabel(t)))],
                        onChanged: (v)=> setState(()=> newTyp = v ?? 'org'),
                        decoration: const InputDecoration(labelText: 'Typ'),
                      ),
                    ),
                    SizedBox(
                      width: 200,
                      child: DropdownButtonFormField<String>(
                        value: newRolle,
                        items: [for (final r in (rollen.isEmpty ? ['customer','supplier','partner','both','other'] : rollen)) DropdownMenuItem(value: r, child: Text(_roleLabel(r)))],
                        onChanged: (v)=> setState(()=> newRolle = v ?? 'customer'),
                        decoration: const InputDecoration(labelText: 'Rolle'),
                      ),
                    ),
                    SizedBox(
                      width: 180,
                      child: DropdownButtonFormField<String>(
                        value: newStatus,
                        items: [for (final s in (statuses.isEmpty ? ['lead','active','inactive','blocked'] : statuses)) DropdownMenuItem(value: s, child: Text(_statusLabel(s)))],
                        onChanged: (v)=> setState(()=> newStatus = v ?? 'active'),
                        decoration: const InputDecoration(labelText: 'Status'),
                      ),
                    ),
                    SizedBox(width: 260, child: TextFormField(controller: nameCtrl, decoration: const InputDecoration(labelText: 'Name'), validator: (v)=> (v==null||v.trim().isEmpty)?'Pflichtfeld':null)),
                    SizedBox(width: 260, child: TextFormField(controller: emailCtrl, decoration: const InputDecoration(labelText: 'E-Mail'))),
                    SizedBox(width: 180, child: TextFormField(controller: phoneCtrl, decoration: const InputDecoration(labelText: 'Telefon'))),
                    SizedBox(width: 180, child: TextFormField(controller: ustCtrl, decoration: const InputDecoration(labelText: 'USt-IdNr.'))),
                    SizedBox(width: 180, child: TextFormField(controller: taxCtrl, decoration: const InputDecoration(labelText: 'Steuernummer'))),
                    SizedBox(width: 120, child: TextFormField(controller: currCtrl, decoration: const InputDecoration(labelText: 'Währung'))),
                    SizedBox(width: 220, child: TextFormField(controller: paymentTermsCtrl, decoration: const InputDecoration(labelText: 'Zahlungsbedingungen'))),
                    SizedBox(width: 180, child: TextFormField(controller: debtorCtrl, decoration: const InputDecoration(labelText: 'Debitor-Nr.'))),
                    SizedBox(width: 180, child: TextFormField(controller: creditorCtrl, decoration: const InputDecoration(labelText: 'Kreditor-Nr.'))),
                    SizedBox(width: 120, child: TextFormField(controller: taxCountryCtrl, decoration: const InputDecoration(labelText: 'Steuerland'))),
                    Row(children: [Checkbox(value: taxExempt, onChanged: (v)=> setState(()=> taxExempt = v ?? false)), const Text('Steuerbefreit')]),
                  ]),
                  const SizedBox(height: 12),
                  const Text('Primäradresse', style: TextStyle(fontWeight: FontWeight.bold)),
                  const SizedBox(height: 8),
                  Wrap(spacing: 12, runSpacing: 12, children: [
                    SizedBox(width: 320, child: TextFormField(controller: addrZ1, decoration: const InputDecoration(labelText: 'Zeile 1'))),
                    SizedBox(width: 320, child: TextFormField(controller: addrZ2, decoration: const InputDecoration(labelText: 'Zeile 2'))),
                    SizedBox(width: 140, child: TextFormField(controller: addrPLZ, decoration: const InputDecoration(labelText: 'PLZ'))),
                    SizedBox(width: 220, child: TextFormField(controller: addrOrt, decoration: const InputDecoration(labelText: 'Ort'))),
                    SizedBox(width: 120, child: TextFormField(controller: addrLand, decoration: const InputDecoration(labelText: 'Land'))),
                    SizedBox(
                      width: 200,
                      child: DropdownButtonFormField<String>(
                        value: addrArt,
                        items: const [
                          DropdownMenuItem(value: 'billing', child: Text('Rechnung')),
                          DropdownMenuItem(value: 'shipping', child: Text('Lieferung')),
                          DropdownMenuItem(value: 'other', child: Text('Sonstige')),
                        ],
                        onChanged: (v)=> setState(()=> addrArt = v ?? 'billing'),
                        decoration: const InputDecoration(labelText: 'Art'),
                      ),
                    ),
                    Row(children: [Checkbox(value: addrPrimary, onChanged: (v)=> setState(()=> addrPrimary = v ?? true)), const Text('Primär')]),
                  ]),
                  const SizedBox(height: 12),
                  const Text('Ansprechpartner (optional)', style: TextStyle(fontWeight: FontWeight.bold)),
                  const SizedBox(height: 8),
                  Wrap(spacing: 12, runSpacing: 12, children: [
                    SizedBox(width: 120, child: TextFormField(controller: pAnrede, decoration: const InputDecoration(labelText: 'Anrede'))),
                    SizedBox(width: 180, child: TextFormField(controller: pVorname, decoration: const InputDecoration(labelText: 'Vorname'))),
                    SizedBox(width: 180, child: TextFormField(controller: pNachname, decoration: const InputDecoration(labelText: 'Nachname'))),
                    SizedBox(width: 220, child: TextFormField(controller: pPosition, decoration: const InputDecoration(labelText: 'Position'))),
                    SizedBox(width: 220, child: TextFormField(controller: pEmail, decoration: const InputDecoration(labelText: 'E-Mail'))),
                    SizedBox(width: 180, child: TextFormField(controller: pPhone, decoration: const InputDecoration(labelText: 'Telefon'))),
                    SizedBox(width: 180, child: TextFormField(controller: pMobil, decoration: const InputDecoration(labelText: 'Mobil'))),
                    Row(children: [Checkbox(value: personPrimary, onChanged: (v)=> setState(()=> personPrimary = v ?? true)), const Text('Primär')]),
                  ]),
                ],
              ),
            ),
          ),
        ),
        actions: [
          TextButton(onPressed: ()=> Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
          FilledButton.icon(
            onPressed: () async {
              if (!formKey.currentState!.validate()) return;
              try {
                final contact = await widget.api.createContact({
                  'typ': newTyp,
                  'rolle': newRolle,
                  'status': newStatus,
                  'name': nameCtrl.text.trim(),
                  'email': emailCtrl.text.trim(),
                  'telefon': phoneCtrl.text.trim(),
                  'ust_id': ustCtrl.text.trim(),
                  'steuernummer': taxCtrl.text.trim(),
                  'waehrung': currCtrl.text.trim().isEmpty ? 'EUR' : currCtrl.text.trim().toUpperCase(),
                  'zahlungsbedingungen': paymentTermsCtrl.text.trim(),
                  'debitor_nr': debtorCtrl.text.trim(),
                  'kreditor_nr': creditorCtrl.text.trim(),
                  'steuer_land': taxCountryCtrl.text.trim().isEmpty ? 'DE' : taxCountryCtrl.text.trim().toUpperCase(),
                  'steuerbefreit': taxExempt,
                });
                final id = contact['id'] as String;
                if (addrZ1.text.trim().isNotEmpty) {
                  await widget.api.createContactAddress(id, {
                    'art': addrArt,
                    'zeile1': addrZ1.text.trim(),
                    'zeile2': addrZ2.text.trim(),
                    'plz': addrPLZ.text.trim(),
                    'ort': addrOrt.text.trim(),
                    'land': addrLand.text.trim(),
                    'is_primary': addrPrimary,
                  });
                }
                if (pVorname.text.trim().isNotEmpty || pNachname.text.trim().isNotEmpty) {
                  await widget.api.createContactPerson(id, {
                    'anrede': pAnrede.text.trim(),
                    'vorname': pVorname.text.trim(),
                    'nachname': pNachname.text.trim(),
                    'position': pPosition.text.trim(),
                    'email': pEmail.text.trim(),
                    'telefon': pPhone.text.trim(),
                    'mobil': pMobil.text.trim(),
                    'is_primary': personPrimary,
                  });
                }
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Kontakt angelegt')));
                  Navigator.of(ctx).pop();
                  await _reload();
                }
              } catch (e) {
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(_errorMessage(e))),
                  );
                }
              }
            },
            icon: const Icon(Icons.check), label: const Text('Anlegen'),
          )
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    final canWrite = widget.api.hasPermission('contacts.write');
    return Scaffold(
      appBar: AppBar(
        backgroundColor: color,
        foregroundColor: Colors.white,
        title: const Text('Kontakte'),
      ),
      floatingActionButtonLocation: FloatingActionButtonLocation.startFloat,
      floatingActionButton: canWrite
          ? FloatingActionButton(onPressed: _openCreateDialog, child: const Icon(Icons.add))
          : null,
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(12),
            child: Row(
              children: [
                const Text('Liste', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                const Spacer(),
                SizedBox(
                  width: 260,
                  child: TextField(
                    controller: searchCtrl,
                    decoration: InputDecoration(isDense: true, prefixIcon: const Icon(Icons.search), hintText: 'Suchen (Name/E-Mail/Telefon)',
                      suffixIcon: IconButton(icon: const Icon(Icons.clear), onPressed: () { searchCtrl.clear(); _reload(); })),
                    onSubmitted: (_) => _reload(),
                  ),
                ),
                const SizedBox(width: 8),
                SizedBox(
                  width: 160,
                  child: InputDecorator(
                    decoration: const InputDecoration(isDense: true, labelText: 'Rolle'),
                    child: DropdownButton<String?>(
                      isExpanded: true,
                      value: filterRolle,
                      hint: const Text('Alle'),
                      items: [
                        const DropdownMenuItem<String?>(value: null, child: Text('Alle')),
                        for (final r in (rollen.isEmpty? ['customer','supplier','partner','both','other']:rollen)) DropdownMenuItem<String?>(value: r, child: Text(_roleLabel(r))),
                      ],
                      onChanged: (v){ setState(()=> filterRolle = v); _reload(); },
                      underline: const SizedBox.shrink(),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                SizedBox(
                  width: 160,
                  child: InputDecorator(
                    decoration: const InputDecoration(isDense: true, labelText: 'Status'),
                    child: DropdownButton<String?>(
                      isExpanded: true,
                      value: filterStatus,
                      hint: const Text('Alle'),
                      items: [
                        const DropdownMenuItem<String?>(value: null, child: Text('Alle')),
                        for (final s in (statuses.isEmpty? ['lead','active','inactive','blocked']:statuses)) DropdownMenuItem<String?>(value: s, child: Text(_statusLabel(s))),
                      ],
                      onChanged: (v){ setState(()=> filterStatus = v); _reload(); },
                      underline: const SizedBox.shrink(),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                SizedBox(
                  width: 160,
                  child: InputDecorator(
                    decoration: const InputDecoration(isDense: true, labelText: 'Typ'),
                    child: DropdownButton<String?>(
                      isExpanded: true,
                      value: filterTyp,
                      hint: const Text('Alle'),
                      items: [
                        const DropdownMenuItem<String?>(value: null, child: Text('Alle')),
                        for (final t in (typen.isEmpty? ['org','person']:typen)) DropdownMenuItem<String?>(value: t, child: Text(_typeLabel(t))),
                      ],
                      onChanged: (v){ setState(()=> filterTyp = v); _reload(); },
                      underline: const SizedBox.shrink(),
                    ),
                  ),
                ),
              ],
            ),
          ),
          if (loading) const LinearProgressIndicator(minHeight: 2),
          Expanded(
            child: ListView.builder(
              itemCount: items.length + 1,
              itemBuilder: (ctx, i) {
                if (i < items.length) {
                  final c = items[i] as Map<String, dynamic>;
                  final rolle = (c['rolle'] ?? '').toString();
                  final typ = (c['typ'] ?? '').toString();
                  return ListTile(
                    leading: Icon(typ=='person'? Icons.person : Icons.apartment),
                    title: Text((c['name'] ?? '').toString()),
                    subtitle: Text('${c['email'] ?? ''}  •  ${c['telefon'] ?? ''}  •  ${rolle.isNotEmpty? _roleLabel(rolle) : _typeLabel(typ)}  •  ${_statusLabel((c['status'] ?? 'active').toString())}'),
                    onTap: () {
                      Navigator.of(context).push(MaterialPageRoute(builder: (_) => ContactDetailScreen(api: widget.api, id: c['id'] as String)))
                        .then((_) => _reload());
                    },
                  );
                }
                final canLoadMore = items.isNotEmpty && items.length % limit == 0;
                if (!canLoadMore) return const SizedBox.shrink();
                return Padding(
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  child: Center(
                    child: FilledButton.icon(onPressed: _loadMore, icon: const Icon(Icons.expand_more), label: const Text('Mehr laden')),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
