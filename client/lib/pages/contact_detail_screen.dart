import 'package:flutter/material.dart';
import '../api.dart';

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
  bool loading = false;

  @override
  void initState() {
    super.initState();
    _loadAll();
  }

  Future<void> _loadAll() async {
    setState(()=> loading = true);
    try {
      final c = await widget.api.getContact(widget.id);
      final a = await widget.api.listContactAddresses(widget.id);
      final p = await widget.api.listContactPersons(widget.id);
      setState(() { contact = c; addresses = a; persons = p; });
    } catch (e) {
      if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Laden fehlgeschlagen: $e'))); }
    } finally { setState(()=> loading = false); }
  }

  Future<void> _editContact() async {
    if (contact == null) return;
    final c = contact!;
    String typ = (c['typ'] ?? 'org').toString();
    String rolle = (c['rolle'] ?? 'other').toString();
    final name = TextEditingController(text: (c['name'] ?? '').toString());
    final email = TextEditingController(text: (c['email'] ?? '').toString());
    final tel = TextEditingController(text: (c['telefon'] ?? '').toString());
    final ust = TextEditingController(text: (c['ust_id'] ?? '').toString());
    final tax = TextEditingController(text: (c['steuernummer'] ?? '').toString());
    final cur = TextEditingController(text: (c['waehrung'] ?? 'EUR').toString());
    await showDialog(context: context, builder: (ctx) => AlertDialog(
      title: const Text('Kontakt bearbeiten'),
      content: SizedBox(
        width: 600,
        child: SingleChildScrollView(
          child: Wrap(spacing: 12, runSpacing: 12, children: [
            SizedBox(width: 160, child: DropdownButtonFormField<String>(value: typ, items: const [DropdownMenuItem(value:'org',child:Text('org')), DropdownMenuItem(value:'person',child:Text('person'))], onChanged: (v){ typ = v ?? 'org'; }, decoration: const InputDecoration(labelText: 'Typ'))),
            SizedBox(width: 200, child: DropdownButtonFormField<String>(value: rolle, items: const [DropdownMenuItem(value:'customer',child:Text('customer')), DropdownMenuItem(value:'supplier',child:Text('supplier')), DropdownMenuItem(value:'both',child:Text('both')), DropdownMenuItem(value:'other',child:Text('other'))], onChanged: (v){ rolle = v ?? 'other'; }, decoration: const InputDecoration(labelText: 'Rolle'))),
            SizedBox(width: 260, child: TextFormField(controller: name, decoration: const InputDecoration(labelText: 'Name'))),
            SizedBox(width: 260, child: TextFormField(controller: email, decoration: const InputDecoration(labelText: 'E-Mail'))),
            SizedBox(width: 180, child: TextFormField(controller: tel, decoration: const InputDecoration(labelText: 'Telefon'))),
            SizedBox(width: 180, child: TextFormField(controller: ust, decoration: const InputDecoration(labelText: 'USt-IdNr.'))),
            SizedBox(width: 180, child: TextFormField(controller: tax, decoration: const InputDecoration(labelText: 'Steuernummer'))),
            SizedBox(width: 120, child: TextFormField(controller: cur, decoration: const InputDecoration(labelText: 'Währung'))),
          ]),
        ),
      ),
      actions: [
        TextButton(onPressed: ()=> Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
        FilledButton.icon(onPressed: () async {
          try {
            final patch = {
              'typ': typ,
              'rolle': rolle,
              'name': name.text.trim(),
              'email': email.text.trim(),
              'telefon': tel.text.trim(),
              'ust_id': ust.text.trim(),
              'steuernummer': tax.text.trim(),
              'waehrung': cur.text.trim().isEmpty? 'EUR' : cur.text.trim().toUpperCase(),
            };
            final updated = await widget.api.updateContact(widget.id, patch);
            setState(()=> contact = updated);
            if (mounted) Navigator.of(ctx).pop();
          } catch (e) {
            if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e'))); }
          }
        }, icon: const Icon(Icons.check), label: const Text('Speichern')),
      ],
    ));
  }

  Future<void> _confirmDelete() async {
    final ok = await showDialog<bool>(context: context, builder: (ctx) => AlertDialog(
      title: const Text('Kontakt löschen'),
      content: const Text('Soll der Kontakt (soft delete) deaktiviert werden?'),
      actions: [
        TextButton(onPressed: ()=> Navigator.of(ctx).pop(false), child: const Text('Abbrechen')),
        FilledButton(onPressed: ()=> Navigator.of(ctx).pop(true), child: const Text('Löschen')),
      ],
    ));
    if (ok == true) {
      try { await widget.api.deleteContact(widget.id); if (mounted) Navigator.of(context).pop(); } catch (e) { if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e'))); } }
    }
  }

  Future<void> _editAddress(Map<String,dynamic>? addr) async {
    String art = (addr?['art'] ?? 'billing').toString();
    final z1 = TextEditingController(text: (addr?['zeile1'] ?? '').toString());
    final z2 = TextEditingController(text: (addr?['zeile2'] ?? '').toString());
    final plz = TextEditingController(text: (addr?['plz'] ?? '').toString());
    final ort = TextEditingController(text: (addr?['ort'] ?? '').toString());
    final land = TextEditingController(text: (addr?['land'] ?? '').toString());
    bool primary = (addr?['is_primary'] ?? false) == true;
    final isNew = addr == null;
    await showDialog(context: context, builder: (ctx) => AlertDialog(
      title: Text(isNew? 'Adresse hinzufügen' : 'Adresse bearbeiten'),
      content: SizedBox(
        width: 600,
        child: SingleChildScrollView(
          child: Wrap(spacing: 12, runSpacing: 12, children: [
            SizedBox(width: 200, child: DropdownButtonFormField<String>(value: art, items: const [DropdownMenuItem(value:'billing',child:Text('Rechnung')), DropdownMenuItem(value:'shipping',child:Text('Lieferung')), DropdownMenuItem(value:'other',child:Text('Sonstige'))], onChanged: (v){ art = v ?? 'billing'; }, decoration: const InputDecoration(labelText: 'Art'))),
            SizedBox(width: 320, child: TextFormField(controller: z1, decoration: const InputDecoration(labelText: 'Zeile 1'))),
            SizedBox(width: 320, child: TextFormField(controller: z2, decoration: const InputDecoration(labelText: 'Zeile 2'))),
            SizedBox(width: 140, child: TextFormField(controller: plz, decoration: const InputDecoration(labelText: 'PLZ'))),
            SizedBox(width: 220, child: TextFormField(controller: ort, decoration: const InputDecoration(labelText: 'Ort'))),
            SizedBox(width: 120, child: TextFormField(controller: land, decoration: const InputDecoration(labelText: 'Land'))),
            Row(children: [Checkbox(value: primary, onChanged: (v)=> setState(()=> primary = v ?? primary)), const Text('Primär')]),
          ]),
        ),
      ),
      actions: [
        if (!isNew) TextButton(onPressed: () async { try { await widget.api.deleteContactAddress(widget.id, (addr!['id'] as String)); Navigator.of(ctx).pop(); await _loadAll(); } catch (e) { if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e'))); } } }, child: const Text('Löschen')),
        TextButton(onPressed: ()=> Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
        FilledButton(onPressed: () async {
          try {
            if (isNew) {
              await widget.api.createContactAddress(widget.id, {'art': art, 'zeile1': z1.text.trim(), 'zeile2': z2.text.trim(), 'plz': plz.text.trim(), 'ort': ort.text.trim(), 'land': land.text.trim(), 'is_primary': primary});
            } else {
              await widget.api.updateContactAddress(widget.id, (addr!['id'] as String), {'art': art, 'zeile1': z1.text.trim(), 'zeile2': z2.text.trim(), 'plz': plz.text.trim(), 'ort': ort.text.trim(), 'land': land.text.trim(), 'is_primary': primary});
            }
            if (mounted) Navigator.of(ctx).pop();
            await _loadAll();
          } catch (e) { if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e'))); } }
        }, child: const Text('Speichern')),
      ],
    ));
  }

  Future<void> _editPerson(Map<String,dynamic>? pers) async {
    final isNew = pers == null;
    final anr = TextEditingController(text: (pers?['anrede'] ?? '').toString());
    final v = TextEditingController(text: (pers?['vorname'] ?? '').toString());
    final n = TextEditingController(text: (pers?['nachname'] ?? '').toString());
    final pos = TextEditingController(text: (pers?['position'] ?? '').toString());
    final em = TextEditingController(text: (pers?['email'] ?? '').toString());
    final ph = TextEditingController(text: (pers?['telefon'] ?? '').toString());
    final mo = TextEditingController(text: (pers?['mobil'] ?? '').toString());
    bool primary = (pers?['is_primary'] ?? false) == true;
    await showDialog(context: context, builder: (ctx) => AlertDialog(
      title: Text(isNew? 'Ansprechpartner hinzufügen' : 'Ansprechpartner bearbeiten'),
      content: SizedBox(
        width: 600,
        child: SingleChildScrollView(
          child: Wrap(spacing: 12, runSpacing: 12, children: [
            SizedBox(width: 120, child: TextFormField(controller: anr, decoration: const InputDecoration(labelText: 'Anrede'))),
            SizedBox(width: 180, child: TextFormField(controller: v, decoration: const InputDecoration(labelText: 'Vorname'))),
            SizedBox(width: 180, child: TextFormField(controller: n, decoration: const InputDecoration(labelText: 'Nachname'))),
            SizedBox(width: 220, child: TextFormField(controller: pos, decoration: const InputDecoration(labelText: 'Position'))),
            SizedBox(width: 220, child: TextFormField(controller: em, decoration: const InputDecoration(labelText: 'E-Mail'))),
            SizedBox(width: 180, child: TextFormField(controller: ph, decoration: const InputDecoration(labelText: 'Telefon'))),
            SizedBox(width: 180, child: TextFormField(controller: mo, decoration: const InputDecoration(labelText: 'Mobil'))),
            Row(children: [Checkbox(value: primary, onChanged: (val)=> setState(()=> primary = val ?? primary)), const Text('Primär')]),
          ]),
        ),
      ),
      actions: [
        if (!isNew) TextButton(onPressed: () async { try { await widget.api.deleteContactPerson(widget.id, (pers!['id'] as String)); Navigator.of(ctx).pop(); await _loadAll(); } catch (e) { if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e'))); } } }, child: const Text('Löschen')),
        TextButton(onPressed: ()=> Navigator.of(ctx).pop(), child: const Text('Abbrechen')),
        FilledButton(onPressed: () async {
          try {
            if (isNew) {
              await widget.api.createContactPerson(widget.id, {'anrede': anr.text.trim(), 'vorname': v.text.trim(), 'nachname': n.text.trim(), 'position': pos.text.trim(), 'email': em.text.trim(), 'telefon': ph.text.trim(), 'mobil': mo.text.trim(), 'is_primary': primary});
            } else {
              await widget.api.updateContactPerson(widget.id, (pers!['id'] as String), {'anrede': anr.text.trim(), 'vorname': v.text.trim(), 'nachname': n.text.trim(), 'position': pos.text.trim(), 'email': em.text.trim(), 'telefon': ph.text.trim(), 'mobil': mo.text.trim(), 'is_primary': primary});
            }
            if (mounted) Navigator.of(ctx).pop();
            await _loadAll();
          } catch (e) { if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e'))); } }
        }, child: const Text('Speichern')),
      ],
    ));
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    final c = contact;
    return Scaffold(
      appBar: AppBar(
        backgroundColor: color,
        foregroundColor: Colors.white,
        title: Text(c==null? 'Kontakt' : (c['name'] ?? 'Kontakt').toString()),
        actions: [
          IconButton(onPressed: _loadAll, icon: const Icon(Icons.refresh)),
          IconButton(onPressed: _editContact, icon: const Icon(Icons.edit)),
          IconButton(onPressed: _confirmDelete, icon: const Icon(Icons.delete_outline)),
        ],
      ),
      floatingActionButtonLocation: FloatingActionButtonLocation.endFloat,
      floatingActionButton: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          FloatingActionButton.small(heroTag: 'addr_add', onPressed: ()=> _editAddress(null), child: const Icon(Icons.add_location_alt)),
          const SizedBox(height: 8),
          FloatingActionButton.small(heroTag: 'pers_add', onPressed: ()=> _editPerson(null), child: const Icon(Icons.person_add_alt)),
        ],
      ),
      body: loading && c==null
        ? const Center(child: CircularProgressIndicator())
        : Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (c != null) ...[
                  Text('Typ: ${c['typ'] ?? ''}  •  Rolle: ${c['rolle'] ?? ''}  •  Währung: ${c['waehrung'] ?? 'EUR'}'),
                  const SizedBox(height: 8),
                  Text('E-Mail: ${c['email'] ?? ''}  •  Telefon: ${c['telefon'] ?? ''}'),
                  const Divider(),
                ],
                Expanded(
                  child: Row(
                    children: [
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            const Text('Adressen', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                            const SizedBox(height: 6),
                            Expanded(
                              child: ListView.builder(
                                itemCount: addresses.length,
                                itemBuilder: (ctx, i) {
                                  final a = addresses[i] as Map<String, dynamic>;
                                  return ListTile(
                                    dense: true,
                                    title: Text('${a['zeile1'] ?? ''}${(a['zeile2']??'') != '' ? ' • ${a['zeile2']}' : ''}'),
                                    subtitle: Text('${a['plz'] ?? ''} ${a['ort'] ?? ''} ${a['land'] ?? ''}  •  ${a['art'] ?? ''}${a['is_primary']==true ? ' • Primär' : ''}'),
                                    trailing: IconButton(icon: const Icon(Icons.edit), onPressed: ()=> _editAddress(a)),
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
                            const Text('Ansprechpartner', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                            const SizedBox(height: 6),
                            Expanded(
                              child: ListView.builder(
                                itemCount: persons.length,
                                itemBuilder: (ctx, i) {
                                  final p = persons[i] as Map<String, dynamic>;
                                  final name = '${p['vorname'] ?? ''} ${p['nachname'] ?? ''}'.trim();
                                  return ListTile(
                                    dense: true,
                                    title: Text(name.isEmpty? (p['email']??'').toString() : name),
                                    subtitle: Text('${p['position'] ?? ''}  •  ${p['email'] ?? ''}  •  ${p['telefon'] ?? ''}${p['is_primary']==true ? ' • Primär' : ''}'),
                                    trailing: IconButton(icon: const Icon(Icons.edit), onPressed: ()=> _editPerson(p)),
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

