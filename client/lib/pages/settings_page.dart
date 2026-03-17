import 'package:flutter/material.dart';
import '../api.dart';
import 'dart:typed_data';
import '../web/browser.dart' as browser;

class SettingsPage extends StatefulWidget {
  const SettingsPage({super.key, required this.api});
  final ApiClient api;

  @override
  State<SettingsPage> createState() => _SettingsPageState();
}

class _SettingsPageState extends State<SettingsPage> {
  final companyNameCtrl = TextEditingController();
  final companyLegalFormCtrl = TextEditingController();
  final companyBranchCtrl = TextEditingController();
  final companyStreetCtrl = TextEditingController();
  final companyPostalCodeCtrl = TextEditingController();
  final companyCityCtrl = TextEditingController();
  final companyCountryCtrl = TextEditingController(text: 'DE');
  final companyEmailCtrl = TextEditingController();
  final companyPhoneCtrl = TextEditingController();
  final companyWebsiteCtrl = TextEditingController();
  final companyInvoiceEmailCtrl = TextEditingController();
  final companyTaxNoCtrl = TextEditingController();
  final companyVatIdCtrl = TextEditingController();
  final companyBankNameCtrl = TextEditingController();
  final companyAccountHolderCtrl = TextEditingController();
  final companyIbanCtrl = TextEditingController();
  final companyBicCtrl = TextEditingController();
  final localizationCurrencyCtrl = TextEditingController(text: 'EUR');
  final localizationTaxCountryCtrl = TextEditingController(text: 'DE');
  final localizationVatRateCtrl = TextEditingController(text: '19.00');
  final localizationLocaleCtrl = TextEditingController(text: 'de-DE');
  final localizationTimezoneCtrl = TextEditingController(text: 'Europe/Berlin');
  final localizationDateFormatCtrl = TextEditingController(text: 'dd.MM.yyyy');
  final localizationNumberFormatCtrl = TextEditingController(text: 'de-DE');
  final brandingDisplayNameCtrl = TextEditingController();
  final brandingClaimCtrl = TextEditingController();
  final brandingPrimaryColorCtrl = TextEditingController(text: '#1F4B99');
  final brandingAccentColorCtrl = TextEditingController(text: '#6B7280');
  final brandingHeaderCtrl = TextEditingController();
  final brandingFooterCtrl = TextEditingController();
  List<Map<String, dynamic>> _branches = [];
  bool _branchesLoading = false;
  final poPatternCtrl = TextEditingController(text: 'PO-{YYYY}-{NNNN}');
  final prjPatternCtrl = TextEditingController(text: 'PRJ-{YYYY}-{NNNN}');
  String previewPO = '';
  String previewPRJ = '';
  bool loading = false;
  // PDF Template Controls (purchase_order)
  final poHeaderCtrl = TextEditingController();
  final poFooterCtrl = TextEditingController();
  final poTopFirstCtrl = TextEditingController(text: '30');
  final poTopOtherCtrl = TextEditingController(text: '20');
  String poEffectiveHeaderText = '';
  String poEffectiveFooterText = '';
  String poEffectiveDisplayName = '';
  String poEffectiveClaim = '';
  String poEffectivePrimaryColor = '';
  String poEffectiveAccentColor = '';
  String? poLogoDocId;
  String? poBgFirstDocId;
  String? poBgOtherDocId;
  final invoiceHeaderCtrl = TextEditingController();
  final invoiceFooterCtrl = TextEditingController();
  final invoiceTopFirstCtrl = TextEditingController(text: '30');
  final invoiceTopOtherCtrl = TextEditingController(text: '20');
  String invoiceEffectiveHeaderText = '';
  String invoiceEffectiveFooterText = '';
  String invoiceEffectiveDisplayName = '';
  String invoiceEffectiveClaim = '';
  String invoiceEffectivePrimaryColor = '';
  String invoiceEffectiveAccentColor = '';
  String? invoiceLogoDocId;
  String? invoiceBgFirstDocId;
  String? invoiceBgOtherDocId;
  final quoteHeaderCtrl = TextEditingController();
  final quoteFooterCtrl = TextEditingController();
  final quoteTopFirstCtrl = TextEditingController(text: '30');
  final quoteTopOtherCtrl = TextEditingController(text: '20');
  String quoteEffectiveHeaderText = '';
  String quoteEffectiveFooterText = '';
  String quoteEffectiveDisplayName = '';
  String quoteEffectiveClaim = '';
  String quoteEffectivePrimaryColor = '';
  String quoteEffectiveAccentColor = '';
  String? quoteLogoDocId;
  String? quoteBgFirstDocId;
  String? quoteBgOtherDocId;

  // Einheiten
  List<Map<String, dynamic>> _units = [];
  final _unitCodeCtrl = TextEditingController();
  final _unitNameCtrl = TextEditingController();
  bool _unitsLoading = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(()=> loading = true);
    try {
      await _loadCompanyProfile();
      await _loadBranches();
      await _loadLocalizationSettings();
      await _loadBrandingSettings();
      final cfg = await widget.api.getNumberingConfig('purchase_order');
      poPatternCtrl.text = (cfg['pattern'] ?? 'PO-{YYYY}-{NNNN}').toString();
      final pcfg = await widget.api.getNumberingConfig('project');
      prjPatternCtrl.text = (pcfg['pattern'] ?? 'PRJ-{YYYY}-{NNNN}').toString();
      await _updatePreviewPO();
      await _updatePreviewPRJ();
      await _loadPdfTemplate('purchase_order');
      await _loadPdfTemplate('invoice_out');
      await _loadPdfTemplate('quote');
      await _loadUnits();
    } catch (e) { /* ignore */ }
    setState(()=> loading = false);
  }

  Future<void> _loadCompanyProfile() async {
    try {
      final p = await widget.api.getCompanyProfile();
      companyNameCtrl.text = (p['name'] ?? '').toString();
      companyLegalFormCtrl.text = (p['legal_form'] ?? '').toString();
      companyBranchCtrl.text = (p['branch_name'] ?? '').toString();
      companyStreetCtrl.text = (p['street'] ?? '').toString();
      companyPostalCodeCtrl.text = (p['postal_code'] ?? '').toString();
      companyCityCtrl.text = (p['city'] ?? '').toString();
      companyCountryCtrl.text = (p['country'] ?? 'DE').toString();
      companyEmailCtrl.text = (p['email'] ?? '').toString();
      companyPhoneCtrl.text = (p['phone'] ?? '').toString();
      companyWebsiteCtrl.text = (p['website'] ?? '').toString();
      companyInvoiceEmailCtrl.text = (p['invoice_email'] ?? '').toString();
      companyTaxNoCtrl.text = (p['tax_no'] ?? '').toString();
      companyVatIdCtrl.text = (p['vat_id'] ?? '').toString();
      companyBankNameCtrl.text = (p['bank_name'] ?? '').toString();
      companyAccountHolderCtrl.text = (p['account_holder'] ?? '').toString();
      companyIbanCtrl.text = (p['iban'] ?? '').toString();
      companyBicCtrl.text = (p['bic'] ?? '').toString();
    } catch (_) {
      // ignore for now
    }
  }

  Future<void> _saveCompanyProfile() async {
    try {
      await widget.api.updateCompanyProfile({
        'name': companyNameCtrl.text,
        'legal_form': companyLegalFormCtrl.text,
        'branch_name': companyBranchCtrl.text,
        'street': companyStreetCtrl.text,
        'postal_code': companyPostalCodeCtrl.text,
        'city': companyCityCtrl.text,
        'country': companyCountryCtrl.text,
        'email': companyEmailCtrl.text,
        'phone': companyPhoneCtrl.text,
        'website': companyWebsiteCtrl.text,
        'invoice_email': companyInvoiceEmailCtrl.text,
        'tax_no': companyTaxNoCtrl.text,
        'vat_id': companyVatIdCtrl.text,
        'bank_name': companyBankNameCtrl.text,
        'account_holder': companyAccountHolderCtrl.text,
        'iban': companyIbanCtrl.text,
        'bic': companyBicCtrl.text,
      });
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Firmenprofil gespeichert')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _loadLocalizationSettings() async {
    try {
      final p = await widget.api.getLocalizationSettings();
      localizationCurrencyCtrl.text = (p['default_currency'] ?? 'EUR').toString();
      localizationTaxCountryCtrl.text = (p['tax_country'] ?? 'DE').toString();
      localizationVatRateCtrl.text = (p['standard_vat_rate'] ?? '19.00').toString();
      localizationLocaleCtrl.text = (p['locale'] ?? 'de-DE').toString();
      localizationTimezoneCtrl.text = (p['timezone'] ?? 'Europe/Berlin').toString();
      localizationDateFormatCtrl.text = (p['date_format'] ?? 'dd.MM.yyyy').toString();
      localizationNumberFormatCtrl.text = (p['number_format'] ?? 'de-DE').toString();
    } catch (_) {
      // ignore for now
    }
  }

  Future<void> _saveLocalizationSettings() async {
    try {
      final vatRate = double.tryParse(localizationVatRateCtrl.text.trim().replaceAll(',', '.')) ?? 19.0;
      await widget.api.updateLocalizationSettings({
        'default_currency': localizationCurrencyCtrl.text,
        'tax_country': localizationTaxCountryCtrl.text,
        'standard_vat_rate': vatRate,
        'locale': localizationLocaleCtrl.text,
        'timezone': localizationTimezoneCtrl.text,
        'date_format': localizationDateFormatCtrl.text,
        'number_format': localizationNumberFormatCtrl.text,
      });
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Lokalisierung gespeichert')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _loadBrandingSettings() async {
    try {
      final p = await widget.api.getBrandingSettings();
      brandingDisplayNameCtrl.text = (p['display_name'] ?? '').toString();
      brandingClaimCtrl.text = (p['claim'] ?? '').toString();
      brandingPrimaryColorCtrl.text = (p['primary_color'] ?? '#1F4B99').toString();
      brandingAccentColorCtrl.text = (p['accent_color'] ?? '#6B7280').toString();
      brandingHeaderCtrl.text = (p['document_header_text'] ?? '').toString();
      brandingFooterCtrl.text = (p['document_footer_text'] ?? '').toString();
    } catch (_) {
      // ignore for now
    }
  }

  Future<void> _saveBrandingSettings() async {
    try {
      await widget.api.updateBrandingSettings({
        'display_name': brandingDisplayNameCtrl.text,
        'claim': brandingClaimCtrl.text,
        'primary_color': brandingPrimaryColorCtrl.text,
        'accent_color': brandingAccentColorCtrl.text,
        'document_header_text': brandingHeaderCtrl.text,
        'document_footer_text': brandingFooterCtrl.text,
      });
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Branding gespeichert')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _loadBranches() async {
    try {
      setState(() => _branchesLoading = true);
      final items = await widget.api.listCompanyBranches();
      setState(() => _branches = items.map((e) => (e as Map).cast<String, dynamic>()).toList());
    } catch (_) {
      // ignore for now
    } finally {
      if (mounted) setState(() => _branchesLoading = false);
    }
  }

  Future<void> _editBranch([Map<String, dynamic>? existing]) async {
    final codeCtrl = TextEditingController(text: (existing?['code'] ?? '').toString());
    final nameCtrl = TextEditingController(text: (existing?['name'] ?? '').toString());
    final streetCtrl = TextEditingController(text: (existing?['street'] ?? '').toString());
    final postalCtrl = TextEditingController(text: (existing?['postal_code'] ?? '').toString());
    final cityCtrl = TextEditingController(text: (existing?['city'] ?? '').toString());
    final countryCtrl = TextEditingController(text: (existing?['country'] ?? 'DE').toString());
    final emailCtrl = TextEditingController(text: (existing?['email'] ?? '').toString());
    final phoneCtrl = TextEditingController(text: (existing?['phone'] ?? '').toString());
    bool isDefault = existing?['is_default'] == true;

    final ok = await showDialog<bool>(
      context: context,
      builder: (_) => StatefulBuilder(
        builder: (context, setLocalState) => AlertDialog(
          title: Text(existing == null ? 'Niederlassung anlegen' : 'Niederlassung bearbeiten'),
          content: SizedBox(
            width: 760,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(children: [
                    Expanded(child: TextField(controller: codeCtrl, decoration: const InputDecoration(labelText: 'Code'))),
                    const SizedBox(width: 12),
                    Expanded(child: TextField(controller: nameCtrl, decoration: const InputDecoration(labelText: 'Name'))),
                  ]),
                  const SizedBox(height: 12),
                  Row(children: [
                    Expanded(child: TextField(controller: streetCtrl, decoration: const InputDecoration(labelText: 'Straße'))),
                    const SizedBox(width: 12),
                    SizedBox(width: 120, child: TextField(controller: postalCtrl, decoration: const InputDecoration(labelText: 'PLZ'))),
                    const SizedBox(width: 12),
                    Expanded(child: TextField(controller: cityCtrl, decoration: const InputDecoration(labelText: 'Ort'))),
                    const SizedBox(width: 12),
                    SizedBox(width: 90, child: TextField(controller: countryCtrl, decoration: const InputDecoration(labelText: 'Land'))),
                  ]),
                  const SizedBox(height: 12),
                  Row(children: [
                    Expanded(child: TextField(controller: emailCtrl, decoration: const InputDecoration(labelText: 'E-Mail'))),
                    const SizedBox(width: 12),
                    Expanded(child: TextField(controller: phoneCtrl, decoration: const InputDecoration(labelText: 'Telefon'))),
                  ]),
                  const SizedBox(height: 8),
                  SwitchListTile(
                    contentPadding: EdgeInsets.zero,
                    title: const Text('Als Standard-Niederlassung markieren'),
                    value: isDefault,
                    onChanged: (v) => setLocalState(() => isDefault = v),
                  ),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Abbrechen')),
            FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Speichern')),
          ],
        ),
      ),
    );
    if (ok != true) return;
    final body = {
      'code': codeCtrl.text,
      'name': nameCtrl.text,
      'street': streetCtrl.text,
      'postal_code': postalCtrl.text,
      'city': cityCtrl.text,
      'country': countryCtrl.text,
      'email': emailCtrl.text,
      'phone': phoneCtrl.text,
      'is_default': isDefault,
    };
    try {
      if (existing == null) {
        await widget.api.createCompanyBranch(body);
      } else {
        await widget.api.updateCompanyBranch((existing['id'] ?? '').toString(), body);
      }
      await _loadBranches();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(existing == null ? 'Niederlassung gespeichert' : 'Niederlassung aktualisiert')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _deleteBranch(Map<String, dynamic> branch) async {
    final id = (branch['id'] ?? '').toString();
    final name = (branch['name'] ?? '').toString();
    final ok = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        title: const Text('Niederlassung löschen'),
        content: Text('Niederlassung "$name" wirklich löschen?'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Abbrechen')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Löschen')),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await widget.api.deleteCompanyBranch(id);
      await _loadBranches();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Niederlassung gelöscht')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _loadUnits() async {
    try {
      setState(()=> _unitsLoading = true);
      final list = await widget.api.listUnits();
      setState(()=> _units = list);
    } catch (e) { /* ignore */ }
    finally { setState(()=> _unitsLoading = false); }
  }

  Future<void> _saveUnit() async {
    final code = _unitCodeCtrl.text.trim();
    final name = _unitNameCtrl.text.trim();
    if (code.isEmpty) { ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Code erforderlich'))); return; }
    try {
      await widget.api.upsertUnit(code, name);
      _unitCodeCtrl.clear();
      _unitNameCtrl.clear();
      await _loadUnits();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Einheit gespeichert')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
    }
  }

  Future<void> _deleteUnit(String code) async {
    final ok = await showDialog<bool>(context: context, builder: (_) => AlertDialog(title: const Text('Einheit löschen'), content: Text('Code "$code" wirklich löschen?'), actions: [TextButton(onPressed: ()=> Navigator.pop(context, false), child: const Text('Abbrechen')), FilledButton(onPressed: ()=> Navigator.pop(context, true), child: const Text('Löschen'))]));
    if (ok != true) return;
    try {
      await widget.api.deleteUnit(code);
      await _loadUnits();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Einheit gelöscht')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
    }
  }

  Future<void> _updatePreviewPO() async {
    try {
      final p = await widget.api.previewNumbering('purchase_order');
      setState(()=> previewPO = p);
    } catch (e) { setState(()=> previewPO = ''); }
  }
  Future<void> _updatePreviewPRJ() async {
    try {
      final p = await widget.api.previewNumbering('project');
      setState(()=> previewPRJ = p);
    } catch (e) { setState(()=> previewPRJ = ''); }
  }

  Future<void> _savePO() async {
    try {
      await widget.api.updateNumberingPattern('purchase_order', poPatternCtrl.text.trim());
      await _updatePreviewPO();
      if (mounted) { ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Gespeichert'))); }
    } catch (e) {
      if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e'))); }
    }
  }
  Future<void> _savePRJ() async {
    try {
      await widget.api.updateNumberingPattern('project', prjPatternCtrl.text.trim());
      await _updatePreviewPRJ();
      if (mounted) { ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Gespeichert'))); }
    } catch (e) {
      if (mounted) { ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e'))); }
    }
  }

  Future<void> _loadPdfTemplate(String entity) async {
    try {
      final t = await widget.api.getPdfTemplate(entity);
      final headerText = (t['header_text'] ?? '').toString();
      final footerText = (t['footer_text'] ?? '').toString();
      final tf = double.tryParse('${t['top_first_mm'] ?? '30'}') ?? 30;
      final to = double.tryParse('${t['top_other_mm'] ?? '20'}') ?? 20;
      if (entity == 'purchase_order') {
        poHeaderCtrl.text = headerText;
        poFooterCtrl.text = footerText;
        poTopFirstCtrl.text = tf.toStringAsFixed(0);
        poTopOtherCtrl.text = to.toStringAsFixed(0);
        poEffectiveHeaderText = (t['effective_header_text'] ?? '').toString();
        poEffectiveFooterText = (t['effective_footer_text'] ?? '').toString();
        poEffectiveDisplayName = (t['effective_display_name'] ?? '').toString();
        poEffectiveClaim = (t['effective_claim'] ?? '').toString();
        poEffectivePrimaryColor =
            (t['effective_primary_color'] ?? '').toString();
        poEffectiveAccentColor = (t['effective_accent_color'] ?? '').toString();
        poLogoDocId = t['logo_doc_id'] as String?;
        poBgFirstDocId = t['bg_first_doc_id'] as String?;
        poBgOtherDocId = t['bg_other_doc_id'] as String?;
      } else if (entity == 'invoice_out') {
        invoiceHeaderCtrl.text = headerText;
        invoiceFooterCtrl.text = footerText;
        invoiceTopFirstCtrl.text = tf.toStringAsFixed(0);
        invoiceTopOtherCtrl.text = to.toStringAsFixed(0);
        invoiceEffectiveHeaderText =
            (t['effective_header_text'] ?? '').toString();
        invoiceEffectiveFooterText =
            (t['effective_footer_text'] ?? '').toString();
        invoiceEffectiveDisplayName =
            (t['effective_display_name'] ?? '').toString();
        invoiceEffectiveClaim = (t['effective_claim'] ?? '').toString();
        invoiceEffectivePrimaryColor =
            (t['effective_primary_color'] ?? '').toString();
        invoiceEffectiveAccentColor =
            (t['effective_accent_color'] ?? '').toString();
        invoiceLogoDocId = t['logo_doc_id'] as String?;
        invoiceBgFirstDocId = t['bg_first_doc_id'] as String?;
        invoiceBgOtherDocId = t['bg_other_doc_id'] as String?;
      } else if (entity == 'quote') {
        quoteHeaderCtrl.text = headerText;
        quoteFooterCtrl.text = footerText;
        quoteTopFirstCtrl.text = tf.toStringAsFixed(0);
        quoteTopOtherCtrl.text = to.toStringAsFixed(0);
        quoteEffectiveHeaderText = (t['effective_header_text'] ?? '').toString();
        quoteEffectiveFooterText = (t['effective_footer_text'] ?? '').toString();
        quoteEffectiveDisplayName = (t['effective_display_name'] ?? '').toString();
        quoteEffectiveClaim = (t['effective_claim'] ?? '').toString();
        quoteEffectivePrimaryColor =
            (t['effective_primary_color'] ?? '').toString();
        quoteEffectiveAccentColor =
            (t['effective_accent_color'] ?? '').toString();
        quoteLogoDocId = t['logo_doc_id'] as String?;
        quoteBgFirstDocId = t['bg_first_doc_id'] as String?;
        quoteBgOtherDocId = t['bg_other_doc_id'] as String?;
      }
      if (mounted) setState((){});
    } catch (_) {
      // ignore
    }
  }

  Future<void> _savePdfTemplate(String entity) async {
    try {
      final isPurchaseOrder = entity == 'purchase_order';
      final isInvoiceOut = entity == 'invoice_out';
      final headerCtrl = isPurchaseOrder
          ? poHeaderCtrl
          : isInvoiceOut
              ? invoiceHeaderCtrl
              : quoteHeaderCtrl;
      final footerCtrl = isPurchaseOrder
          ? poFooterCtrl
          : isInvoiceOut
              ? invoiceFooterCtrl
              : quoteFooterCtrl;
      final topFirstCtrl = isPurchaseOrder
          ? poTopFirstCtrl
          : isInvoiceOut
              ? invoiceTopFirstCtrl
              : quoteTopFirstCtrl;
      final topOtherCtrl = isPurchaseOrder
          ? poTopOtherCtrl
          : isInvoiceOut
              ? invoiceTopOtherCtrl
              : quoteTopOtherCtrl;
      final tf =
          double.tryParse(topFirstCtrl.text.trim().replaceAll(',', '.')) ?? 30;
      final to =
          double.tryParse(topOtherCtrl.text.trim().replaceAll(',', '.')) ?? 20;
      await widget.api.updatePdfTemplate(
        entity,
        headerText: headerCtrl.text,
        footerText: footerCtrl.text,
        topFirstMm: tf,
        topOtherMm: to,
      );
      await _loadPdfTemplate(entity);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('PDF-Template gespeichert')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
      }
    }
  }

  Future<void> _pickAndUpload(String entity, String kind) async {
    final picked = await browser.pickFile(accept: 'image/*,application/pdf');
    if (picked == null) return;
    try {
      final res = await widget.api.uploadPdfImage(entity, kind, picked.filename, picked.bytes, contentType: picked.contentType);
      final id = (res['document_id'] ?? '').toString();
      setState((){
        if (entity == 'purchase_order') {
          if (kind == 'logo') poLogoDocId = id;
          if (kind == 'bg-first') poBgFirstDocId = id;
          if (kind == 'bg-other') poBgOtherDocId = id;
        } else if (entity == 'invoice_out') {
          if (kind == 'logo') invoiceLogoDocId = id;
          if (kind == 'bg-first') invoiceBgFirstDocId = id;
          if (kind == 'bg-other') invoiceBgOtherDocId = id;
        } else if (entity == 'quote') {
          if (kind == 'logo') quoteLogoDocId = id;
          if (kind == 'bg-first') quoteBgFirstDocId = id;
          if (kind == 'bg-other') quoteBgOtherDocId = id;
        }
      });
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Upload erfolgreich')));
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
    }
  }

  Future<void> _deleteImage(String entity, String kind) async {
    try {
      await widget.api.deletePdfImage(entity, kind);
      setState((){
        if (entity == 'purchase_order') {
          if (kind == 'logo') poLogoDocId = null;
          if (kind == 'bg-first') poBgFirstDocId = null;
          if (kind == 'bg-other') poBgOtherDocId = null;
        } else if (entity == 'invoice_out') {
          if (kind == 'logo') invoiceLogoDocId = null;
          if (kind == 'bg-first') invoiceBgFirstDocId = null;
          if (kind == 'bg-other') invoiceBgOtherDocId = null;
        } else if (entity == 'quote') {
          if (kind == 'logo') quoteLogoDocId = null;
          if (kind == 'bg-first') quoteBgFirstDocId = null;
          if (kind == 'bg-other') quoteBgOtherDocId = null;
        }
      });
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Bild entfernt')));
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Fehler: $e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    return Scaffold(
      appBar: AppBar(
        backgroundColor: color,
        foregroundColor: Colors.white,
        title: const Text('Einstellungen'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              ExpansionTile(
                title: const Text('Firmenprofil', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                initiallyExpanded: true,
                children: [
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(children: [
                            Expanded(child: TextField(controller: companyNameCtrl, decoration: const InputDecoration(labelText: 'Firmenname'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: companyLegalFormCtrl, decoration: const InputDecoration(labelText: 'Rechtsform'))),
                          ]),
                          const SizedBox(height: 12),
                          Row(children: [
                            Expanded(child: TextField(controller: companyBranchCtrl, decoration: const InputDecoration(labelText: 'Niederlassung / Standort'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: companyInvoiceEmailCtrl, decoration: const InputDecoration(labelText: 'Rechnungs-E-Mail'))),
                          ]),
                          const SizedBox(height: 12),
                          Row(children: [
                            Expanded(child: TextField(controller: companyStreetCtrl, decoration: const InputDecoration(labelText: 'Straße'))),
                            const SizedBox(width: 12),
                            SizedBox(width: 140, child: TextField(controller: companyPostalCodeCtrl, decoration: const InputDecoration(labelText: 'PLZ'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: companyCityCtrl, decoration: const InputDecoration(labelText: 'Ort'))),
                            const SizedBox(width: 12),
                            SizedBox(width: 100, child: TextField(controller: companyCountryCtrl, decoration: const InputDecoration(labelText: 'Land'))),
                          ]),
                          const SizedBox(height: 12),
                          Row(children: [
                            Expanded(child: TextField(controller: companyEmailCtrl, decoration: const InputDecoration(labelText: 'E-Mail'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: companyPhoneCtrl, decoration: const InputDecoration(labelText: 'Telefon'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: companyWebsiteCtrl, decoration: const InputDecoration(labelText: 'Website'))),
                          ]),
                          const SizedBox(height: 12),
                          Row(children: [
                            Expanded(child: TextField(controller: companyTaxNoCtrl, decoration: const InputDecoration(labelText: 'Steuernummer'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: companyVatIdCtrl, decoration: const InputDecoration(labelText: 'USt-IdNr.'))),
                          ]),
                          const SizedBox(height: 12),
                          const Text('Bankdaten', style: TextStyle(fontWeight: FontWeight.bold)),
                          const SizedBox(height: 8),
                          Row(children: [
                            Expanded(child: TextField(controller: companyBankNameCtrl, decoration: const InputDecoration(labelText: 'Bank'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: companyAccountHolderCtrl, decoration: const InputDecoration(labelText: 'Kontoinhaber'))),
                          ]),
                          const SizedBox(height: 12),
                          Row(children: [
                            Expanded(child: TextField(controller: companyIbanCtrl, decoration: const InputDecoration(labelText: 'IBAN'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: companyBicCtrl, decoration: const InputDecoration(labelText: 'BIC'))),
                          ]),
                          const SizedBox(height: 12),
                          Align(
                            alignment: Alignment.centerRight,
                            child: FilledButton.icon(onPressed: _saveCompanyProfile, icon: const Icon(Icons.save), label: const Text('Speichern')),
                          ),
                          const SizedBox(height: 16),
                          const Divider(),
                          const SizedBox(height: 8),
                          const Text('Steuer, Währung und Lokalisierung', style: TextStyle(fontWeight: FontWeight.bold)),
                          const SizedBox(height: 8),
                          Row(children: [
                            Expanded(child: TextField(controller: localizationCurrencyCtrl, decoration: const InputDecoration(labelText: 'Standardwährung'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: localizationTaxCountryCtrl, decoration: const InputDecoration(labelText: 'Steuerland'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: localizationVatRateCtrl, decoration: const InputDecoration(labelText: 'Standard-USt. %'), keyboardType: TextInputType.number)),
                          ]),
                          const SizedBox(height: 12),
                          Row(children: [
                            Expanded(child: TextField(controller: localizationLocaleCtrl, decoration: const InputDecoration(labelText: 'Locale'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: localizationTimezoneCtrl, decoration: const InputDecoration(labelText: 'Zeitzone'))),
                          ]),
                          const SizedBox(height: 12),
                          Row(children: [
                            Expanded(child: TextField(controller: localizationDateFormatCtrl, decoration: const InputDecoration(labelText: 'Datumsformat'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: localizationNumberFormatCtrl, decoration: const InputDecoration(labelText: 'Zahlenformat'))),
                          ]),
                          const SizedBox(height: 12),
                          Align(
                            alignment: Alignment.centerRight,
                            child: FilledButton.icon(onPressed: _saveLocalizationSettings, icon: const Icon(Icons.language), label: const Text('Lokalisierung speichern')),
                          ),
                          const SizedBox(height: 16),
                          Row(
                            children: [
                              const Text('Niederlassungen', style: TextStyle(fontWeight: FontWeight.bold)),
                              const Spacer(),
                              OutlinedButton.icon(
                                onPressed: () => _editBranch(),
                                icon: const Icon(Icons.add_business_outlined),
                                label: const Text('Niederlassung anlegen'),
                              ),
                            ],
                          ),
                          const SizedBox(height: 8),
                          if (_branchesLoading) const LinearProgressIndicator(minHeight: 2),
                          if (!_branchesLoading && _branches.isEmpty)
                            const Padding(
                              padding: EdgeInsets.symmetric(vertical: 8),
                              child: Text('Noch keine Niederlassungen angelegt.'),
                            ),
                          ..._branches.map((b) {
                            final title = (b['name'] ?? '').toString();
                            final code = (b['code'] ?? '').toString();
                            final city = (b['city'] ?? '').toString();
                            final country = (b['country'] ?? '').toString();
                            final isDefault = b['is_default'] == true;
                            final subtitleParts = <String>[
                              if (code.isNotEmpty) 'Code: $code',
                              if (city.isNotEmpty || country.isNotEmpty) [city, country].where((e) => e.isNotEmpty).join(', '),
                              if ((b['email'] ?? '').toString().isNotEmpty) (b['email'] ?? '').toString(),
                            ];
                            return ListTile(
                              contentPadding: EdgeInsets.zero,
                              leading: Icon(isDefault ? Icons.apartment_rounded : Icons.business_outlined),
                              title: Text(isDefault ? '$title (Standard)' : title),
                              subtitle: subtitleParts.isNotEmpty ? Text(subtitleParts.join(' • ')) : null,
                              trailing: Wrap(
                                spacing: 4,
                                children: [
                                  IconButton(icon: const Icon(Icons.edit_outlined), onPressed: () => _editBranch(b)),
                                  IconButton(icon: const Icon(Icons.delete_outline), onPressed: () => _deleteBranch(b)),
                                ],
                              ),
                            );
                          }),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              ExpansionTile(
                title: const Text('Branding & Dokumentlayout', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                initiallyExpanded: false,
                children: [
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(children: [
                            Expanded(child: TextField(controller: brandingDisplayNameCtrl, decoration: const InputDecoration(labelText: 'Brand-Name'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: brandingClaimCtrl, decoration: const InputDecoration(labelText: 'Claim / Zusatzzeile'))),
                          ]),
                          const SizedBox(height: 12),
                          Row(children: [
                            Expanded(child: TextField(controller: brandingPrimaryColorCtrl, decoration: const InputDecoration(labelText: 'Primärfarbe (Hex)'))),
                            const SizedBox(width: 12),
                            Expanded(child: TextField(controller: brandingAccentColorCtrl, decoration: const InputDecoration(labelText: 'Akzentfarbe (Hex)'))),
                          ]),
                          const SizedBox(height: 12),
                          TextField(
                            controller: brandingHeaderCtrl,
                            maxLines: 3,
                            decoration: const InputDecoration(labelText: 'Standard-Kopftext für Dokumente'),
                          ),
                          const SizedBox(height: 12),
                          TextField(
                            controller: brandingFooterCtrl,
                            maxLines: 3,
                            decoration: const InputDecoration(labelText: 'Standard-Fußtext für Dokumente'),
                          ),
                          const SizedBox(height: 12),
                          Align(
                            alignment: Alignment.centerRight,
                            child: FilledButton.icon(
                              onPressed: _saveBrandingSettings,
                              icon: const Icon(Icons.palette_outlined),
                              label: const Text('Branding speichern'),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              ExpansionTile(
                title: const Text('Nummernkreise', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                initiallyExpanded: false,
                childrenPadding: const EdgeInsets.only(bottom: 8),
                children: [
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text('Bestellungen'),
                          const SizedBox(height: 8),
                          TextField(
                            controller: poPatternCtrl,
                            decoration: const InputDecoration(labelText: 'Pattern', hintText: 'z. B. PO-{YYYY}-{NNNN}'),
                            onChanged: (_) { _updatePreviewPO(); },
                          ),
                          const SizedBox(height: 8),
                          Text('Vorschau: $previewPO'),
                          const SizedBox(height: 8),
                          const Text('Variablen: {YYYY}, {YY}, {MM}, {DD}, {NN}, {NNN}, {NNNN}'),
                          const SizedBox(height: 8),
                          Align(
                            alignment: Alignment.centerRight,
                            child: FilledButton.icon(onPressed: _savePO, icon: const Icon(Icons.save), label: const Text('Speichern')),
                          ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 12),
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text('Projekte'),
                          const SizedBox(height: 8),
                          TextField(
                            controller: prjPatternCtrl,
                            decoration: const InputDecoration(labelText: 'Pattern', hintText: 'z. B. PRJ-{YYYY}-{NNNN}'),
                            onChanged: (_) { _updatePreviewPRJ(); },
                          ),
                          const SizedBox(height: 8),
                          Text('Vorschau: $previewPRJ'),
                          const SizedBox(height: 8),
                          const Text('Variablen: {YYYY}, {YY}, {MM}, {DD}, {NN}, {NNN}, {NNNN}'),
                          const SizedBox(height: 8),
                          Align(
                            alignment: Alignment.centerRight,
                            child: FilledButton.icon(onPressed: _savePRJ, icon: const Icon(Icons.save), label: const Text('Speichern')),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              ExpansionTile(
                title: const Text('Maßeinheiten', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                initiallyExpanded: false,
                children: [
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                        Row(children: [
                          Expanded(child: TextField(controller: _unitCodeCtrl, decoration: const InputDecoration(labelText: 'Code (z. B. kg, mm)'))),
                          const SizedBox(width: 8),
                          Expanded(child: TextField(controller: _unitNameCtrl, decoration: const InputDecoration(labelText: 'Name (optional, z. B. Kilogramm)'))),
                          const SizedBox(width: 8),
                          FilledButton.icon(onPressed: _saveUnit, icon: const Icon(Icons.save), label: const Text('Speichern')),
                        ]),
                        const SizedBox(height: 12),
                        if (_unitsLoading) const LinearProgressIndicator(minHeight: 2),
                        ListView.builder(
                          shrinkWrap: true,
                          physics: const NeverScrollableScrollPhysics(),
                          itemCount: _units.length,
                          itemBuilder: (ctx, i){
                            final u = _units[i];
                            final code = (u['code'] ?? '').toString();
                            final name = (u['name'] ?? '').toString();
                            return ListTile(
                              dense: true,
                              leading: const Icon(Icons.straighten_rounded),
                              title: Text(code),
                              subtitle: name.isNotEmpty ? Text(name) : null,
                              trailing: IconButton(icon: const Icon(Icons.delete_outline), onPressed: ()=> _deleteUnit(code)),
                              onTap: (){ _unitCodeCtrl.text = code; _unitNameCtrl.text = name; },
                            );
                          },
                        ),
                      ]),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              ExpansionTile(
                title: const Text('PDF-Templates', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                initiallyExpanded: false,
                children: [
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                        _buildPdfTemplateCard(
                          title: 'Bestellungen (purchase_order)',
                          entity: 'purchase_order',
                          headerCtrl: poHeaderCtrl,
                          footerCtrl: poFooterCtrl,
                          topFirstCtrl: poTopFirstCtrl,
                          topOtherCtrl: poTopOtherCtrl,
                          effectiveHeaderText: poEffectiveHeaderText,
                          effectiveFooterText: poEffectiveFooterText,
                          effectiveDisplayName: poEffectiveDisplayName,
                          effectiveClaim: poEffectiveClaim,
                          effectivePrimaryColor: poEffectivePrimaryColor,
                          effectiveAccentColor: poEffectiveAccentColor,
                          logoDocId: poLogoDocId,
                          bgFirstDocId: poBgFirstDocId,
                          bgOtherDocId: poBgOtherDocId,
                        ),
                        const Divider(height: 32),
                        _buildPdfTemplateCard(
                          title: 'Ausgangsrechnungen (invoice_out)',
                          entity: 'invoice_out',
                          headerCtrl: invoiceHeaderCtrl,
                          footerCtrl: invoiceFooterCtrl,
                          topFirstCtrl: invoiceTopFirstCtrl,
                          topOtherCtrl: invoiceTopOtherCtrl,
                          effectiveHeaderText: invoiceEffectiveHeaderText,
                          effectiveFooterText: invoiceEffectiveFooterText,
                          effectiveDisplayName: invoiceEffectiveDisplayName,
                          effectiveClaim: invoiceEffectiveClaim,
                          effectivePrimaryColor: invoiceEffectivePrimaryColor,
                          effectiveAccentColor: invoiceEffectiveAccentColor,
                          logoDocId: invoiceLogoDocId,
                          bgFirstDocId: invoiceBgFirstDocId,
                          bgOtherDocId: invoiceBgOtherDocId,
                        ),
                        const Divider(height: 32),
                        _buildPdfTemplateCard(
                          title: 'Angebote (quote)',
                          entity: 'quote',
                          headerCtrl: quoteHeaderCtrl,
                          footerCtrl: quoteFooterCtrl,
                          topFirstCtrl: quoteTopFirstCtrl,
                          topOtherCtrl: quoteTopOtherCtrl,
                          effectiveHeaderText: quoteEffectiveHeaderText,
                          effectiveFooterText: quoteEffectiveFooterText,
                          effectiveDisplayName: quoteEffectiveDisplayName,
                          effectiveClaim: quoteEffectiveClaim,
                          effectivePrimaryColor: quoteEffectivePrimaryColor,
                          effectiveAccentColor: quoteEffectiveAccentColor,
                          logoDocId: quoteLogoDocId,
                          bgFirstDocId: quoteBgFirstDocId,
                          bgOtherDocId: quoteBgOtherDocId,
                        ),
                      ]),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildPdfTemplateCard({
    required String title,
    required String entity,
    required TextEditingController headerCtrl,
    required TextEditingController footerCtrl,
    required TextEditingController topFirstCtrl,
    required TextEditingController topOtherCtrl,
    required String effectiveHeaderText,
    required String effectiveFooterText,
    required String effectiveDisplayName,
    required String effectiveClaim,
    required String effectivePrimaryColor,
    required String effectiveAccentColor,
    required String? logoDocId,
    required String? bgFirstDocId,
    required String? bgOtherDocId,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(title),
        const SizedBox(height: 8),
        TextField(
          controller: headerCtrl,
          maxLines: 3,
          decoration: const InputDecoration(
            labelText: 'Kopftext',
            hintText: 'z. B. Firmenname, Adresse, Kontaktdaten',
          ),
        ),
        if (effectiveHeaderText.isNotEmpty) ...[
          const SizedBox(height: 6),
          Text(
            'Effektiver Kopftext: $effectiveHeaderText',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
        if (effectiveDisplayName.isNotEmpty) ...[
          const SizedBox(height: 6),
          Text(
            'Effektiver Brand-Name: $effectiveDisplayName',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
        if (effectiveClaim.isNotEmpty) ...[
          const SizedBox(height: 6),
          Text(
            'Effektiver Claim: $effectiveClaim',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
        const SizedBox(height: 8),
        TextField(
          controller: footerCtrl,
          maxLines: 2,
          decoration: const InputDecoration(
            labelText: 'Fußtext',
            hintText: 'z. B. Bankdaten, USt-IdNr.',
          ),
        ),
        if (effectiveFooterText.isNotEmpty) ...[
          const SizedBox(height: 6),
          Text(
            'Effektiver Fußtext: $effectiveFooterText',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
        if (effectivePrimaryColor.isNotEmpty) ...[
          const SizedBox(height: 6),
          Text(
            'Effektive Primärfarbe: $effectivePrimaryColor',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
        if (effectiveAccentColor.isNotEmpty) ...[
          const SizedBox(height: 6),
          Text(
            'Effektive Akzentfarbe: $effectiveAccentColor',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
        const SizedBox(height: 12),
        Row(children: [
          Expanded(
            child: TextField(
              controller: topFirstCtrl,
              decoration:
                  const InputDecoration(labelText: 'Start Höhe Seite 1 (mm)'),
              keyboardType: TextInputType.number,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: TextField(
              controller: topOtherCtrl,
              decoration: const InputDecoration(
                  labelText: 'Start Höhe Folgeseiten (mm)'),
              keyboardType: TextInputType.number,
            ),
          ),
        ]),
        const SizedBox(height: 12),
        Wrap(spacing: 12, runSpacing: 8, children: [
          _imageRow(
            'Logo',
            logoDocId,
            onUpload: () => _pickAndUpload(entity, 'logo'),
            onDelete: () => _deleteImage(entity, 'logo'),
          ),
          _imageRow(
            'Hintergrund (Seite 1)',
            bgFirstDocId,
            onUpload: () => _pickAndUpload(entity, 'bg-first'),
            onDelete: () => _deleteImage(entity, 'bg-first'),
          ),
          _imageRow(
            'Hintergrund (Folge)',
            bgOtherDocId,
            onUpload: () => _pickAndUpload(entity, 'bg-other'),
            onDelete: () => _deleteImage(entity, 'bg-other'),
          ),
        ]),
        const SizedBox(height: 12),
        Align(
          alignment: Alignment.centerRight,
          child: FilledButton.icon(
            onPressed: () => _savePdfTemplate(entity),
            icon: const Icon(Icons.save),
            label: const Text('Speichern'),
          ),
        ),
      ],
    );
  }

  Widget _imageRow(String label, String? docId, {required VoidCallback onUpload, required VoidCallback onDelete}) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(label),
        const SizedBox(width: 8),
        if (docId != null) ...[
          const Icon(Icons.check_circle, color: Colors.green, size: 18),
          const SizedBox(width: 4),
          Text(docId.substring(0, docId.length >= 8 ? 8 : docId.length)),
          const SizedBox(width: 8),
          TextButton.icon(onPressed: (){ widget.api.downloadDocument(docId, filename: 'preview'); }, icon: const Icon(Icons.visibility), label: const Text('Anzeigen')),
          const SizedBox(width: 8),
          TextButton.icon(onPressed: onDelete, icon: const Icon(Icons.delete), label: const Text('Entfernen')),
        ] else ...[
          const Text('— nicht gesetzt —'),
        ],
        const SizedBox(width: 8),
        OutlinedButton.icon(onPressed: onUpload, icon: const Icon(Icons.upload), label: const Text('Hochladen')),
      ],
    );
  }
}
