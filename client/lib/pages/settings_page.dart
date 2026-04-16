import 'package:flutter/material.dart';
import '../api.dart';
import '../commercial_destinations.dart';
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
  final salesOrderHeaderCtrl = TextEditingController();
  final salesOrderFooterCtrl = TextEditingController();
  final salesOrderTopFirstCtrl = TextEditingController(text: '30');
  final salesOrderTopOtherCtrl = TextEditingController(text: '20');
  String salesOrderEffectiveHeaderText = '';
  String salesOrderEffectiveFooterText = '';
  String salesOrderEffectiveDisplayName = '';
  String salesOrderEffectiveClaim = '';
  String salesOrderEffectivePrimaryColor = '';
  String salesOrderEffectiveAccentColor = '';
  String? salesOrderLogoDocId;
  String? salesOrderBgFirstDocId;
  String? salesOrderBgOtherDocId;

  // Einheiten
  List<Map<String, dynamic>> _units = [];
  final _unitCodeCtrl = TextEditingController();
  final _unitNameCtrl = TextEditingController();
  bool _unitsLoading = false;
  List<Map<String, dynamic>> _materialGroups = [];
  final _materialGroupCodeCtrl = TextEditingController();
  final _materialGroupNameCtrl = TextEditingController();
  final _materialGroupDescriptionCtrl = TextEditingController();
  final _materialGroupSortOrderCtrl = TextEditingController(text: '0');
  bool _materialGroupIsActive = true;
  bool _materialGroupsLoading = false;
  List<Map<String, dynamic>> _quoteTextBlocks = [];
  final _quoteTextBlockIdCtrl = TextEditingController();
  final _quoteTextBlockCodeCtrl = TextEditingController();
  final _quoteTextBlockNameCtrl = TextEditingController();
  final _quoteTextBlockBodyCtrl = TextEditingController();
  final _quoteTextBlockSortOrderCtrl = TextEditingController(text: '0');
  String _quoteTextBlockCategory = 'intro';
  bool _quoteTextBlockIsActive = true;
  bool _quoteTextBlocksLoading = false;
  static const List<String> _quoteTextBlockCategories = <String>[
    'intro',
    'scope',
    'closing',
    'legal',
  ];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => loading = true);
    try {
      await _loadCompanyProfile();
      await _loadBranches();
      await _loadLocalizationSettings();
      await _loadBrandingSettings();
      await _loadNumberingPattern(
        'purchase_order',
        poPatternCtrl,
        'PO-{YYYY}-{NNNN}',
      );
      await _loadNumberingPattern(
        'project',
        prjPatternCtrl,
        'PRJ-{YYYY}-{NNNN}',
      );
      await _updatePreviewPO();
      await _updatePreviewPRJ();
      await _loadPdfTemplate('purchase_order');
      await _loadPdfTemplate('invoice_out');
      await _loadPdfTemplate('quote');
      await _loadPdfTemplate('sales_order');
      await _loadUnits();
      await _loadMaterialGroups();
      await _loadQuoteTextBlocks();
    } catch (e) {/* ignore */}
    setState(() => loading = false);
  }

  Future<void> _loadNumberingPattern(
    String entity,
    TextEditingController controller,
    String fallbackPattern,
  ) async {
    final config = await widget.api.getNumberingConfig(entity);
    controller.text = (config['pattern'] ?? fallbackPattern).toString();
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

  void _showSettingsSuccess(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text(message)));
  }

  void _showSettingsError(Object error) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text('Fehler: $error')));
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
      _showSettingsSuccess('Firmenprofil gespeichert');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _loadLocalizationSettings() async {
    try {
      final p = await widget.api.getLocalizationSettings();
      localizationCurrencyCtrl.text =
          (p['default_currency'] ?? 'EUR').toString();
      localizationTaxCountryCtrl.text = (p['tax_country'] ?? 'DE').toString();
      localizationVatRateCtrl.text =
          (p['standard_vat_rate'] ?? '19.00').toString();
      localizationLocaleCtrl.text = (p['locale'] ?? 'de-DE').toString();
      localizationTimezoneCtrl.text =
          (p['timezone'] ?? 'Europe/Berlin').toString();
      localizationDateFormatCtrl.text =
          (p['date_format'] ?? 'dd.MM.yyyy').toString();
      localizationNumberFormatCtrl.text =
          (p['number_format'] ?? 'de-DE').toString();
    } catch (_) {
      // ignore for now
    }
  }

  Future<void> _saveLocalizationSettings() async {
    try {
      final vatRate = double.tryParse(
              localizationVatRateCtrl.text.trim().replaceAll(',', '.')) ??
          19.0;
      await widget.api.updateLocalizationSettings({
        'default_currency': localizationCurrencyCtrl.text,
        'tax_country': localizationTaxCountryCtrl.text,
        'standard_vat_rate': vatRate,
        'locale': localizationLocaleCtrl.text,
        'timezone': localizationTimezoneCtrl.text,
        'date_format': localizationDateFormatCtrl.text,
        'number_format': localizationNumberFormatCtrl.text,
      });
      _showSettingsSuccess('Lokalisierung gespeichert');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _loadBrandingSettings() async {
    try {
      final p = await widget.api.getBrandingSettings();
      brandingDisplayNameCtrl.text = (p['display_name'] ?? '').toString();
      brandingClaimCtrl.text = (p['claim'] ?? '').toString();
      brandingPrimaryColorCtrl.text =
          (p['primary_color'] ?? '#1F4B99').toString();
      brandingAccentColorCtrl.text =
          (p['accent_color'] ?? '#6B7280').toString();
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
      _showSettingsSuccess('Branding gespeichert');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _loadBranches() async {
    try {
      setState(() => _branchesLoading = true);
      final items = await widget.api.listCompanyBranches();
      setState(() => _branches =
          items.map((e) => (e as Map).cast<String, dynamic>()).toList());
    } catch (_) {
      // ignore for now
    } finally {
      if (mounted) setState(() => _branchesLoading = false);
    }
  }

  Future<void> _editBranch([Map<String, dynamic>? existing]) async {
    final codeCtrl =
        TextEditingController(text: (existing?['code'] ?? '').toString());
    final nameCtrl =
        TextEditingController(text: (existing?['name'] ?? '').toString());
    final streetCtrl =
        TextEditingController(text: (existing?['street'] ?? '').toString());
    final postalCtrl = TextEditingController(
        text: (existing?['postal_code'] ?? '').toString());
    final cityCtrl =
        TextEditingController(text: (existing?['city'] ?? '').toString());
    final countryCtrl =
        TextEditingController(text: (existing?['country'] ?? 'DE').toString());
    final emailCtrl =
        TextEditingController(text: (existing?['email'] ?? '').toString());
    final phoneCtrl =
        TextEditingController(text: (existing?['phone'] ?? '').toString());
    bool isDefault = existing?['is_default'] == true;

    final ok = await showDialog<bool>(
      context: context,
      builder: (_) => StatefulBuilder(
        builder: (dialogContext, setLocalState) => AlertDialog(
          title: Text(existing == null
              ? 'Niederlassung anlegen'
              : 'Niederlassung bearbeiten'),
          content: SizedBox(
            width: 760,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  _buildTwoFieldRow(
                    left: TextField(
                      controller: codeCtrl,
                      decoration: const InputDecoration(labelText: 'Code'),
                    ),
                    right: TextField(
                      controller: nameCtrl,
                      decoration: const InputDecoration(labelText: 'Name'),
                    ),
                  ),
                  const SizedBox(height: 12),
                  _buildAddressFieldRow(
                    street: TextField(
                      controller: streetCtrl,
                      decoration: const InputDecoration(labelText: 'Straße'),
                    ),
                    postalCode: TextField(
                      controller: postalCtrl,
                      decoration: const InputDecoration(labelText: 'PLZ'),
                    ),
                    city: TextField(
                      controller: cityCtrl,
                      decoration: const InputDecoration(labelText: 'Ort'),
                    ),
                    country: TextField(
                      controller: countryCtrl,
                      decoration: const InputDecoration(labelText: 'Land'),
                    ),
                    postalWidth: 120,
                    countryWidth: 90,
                  ),
                  const SizedBox(height: 12),
                  _buildTwoFieldRow(
                    left: TextField(
                      controller: emailCtrl,
                      decoration: const InputDecoration(labelText: 'E-Mail'),
                    ),
                    right: TextField(
                      controller: phoneCtrl,
                      decoration: const InputDecoration(labelText: 'Telefon'),
                    ),
                  ),
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
          actions: _buildDialogActions(
            dialogContext: dialogContext,
            confirmLabel: 'Speichern',
            onConfirm: () => Navigator.pop(dialogContext, true),
          ),
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
        await widget.api
            .updateCompanyBranch((existing['id'] ?? '').toString(), body);
      }
      await _loadBranches();
      _showSettingsSuccess(existing == null
          ? 'Niederlassung gespeichert'
          : 'Niederlassung aktualisiert');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _deleteBranch(Map<String, dynamic> branch) async {
    final id = (branch['id'] ?? '').toString();
    final name = (branch['name'] ?? '').toString();
    final ok = await _confirmDelete(
      title: 'Niederlassung löschen',
      message: 'Niederlassung "$name" wirklich löschen?',
    );
    if (ok != true) return;
    try {
      await widget.api.deleteCompanyBranch(id);
      await _loadBranches();
      _showSettingsSuccess('Niederlassung gelöscht');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _loadUnits() async {
    try {
      setState(() => _unitsLoading = true);
      final list = await widget.api.listUnits();
      setState(() => _units = list);
    } catch (e) {/* ignore */} finally {
      setState(() => _unitsLoading = false);
    }
  }

  Future<void> _saveUnit() async {
    final code = _unitCodeCtrl.text.trim();
    final name = _unitNameCtrl.text.trim();
    if (code.isEmpty) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Code erforderlich')));
      return;
    }
    try {
      await widget.api.upsertUnit(code, name);
      _unitCodeCtrl.clear();
      _unitNameCtrl.clear();
      await _loadUnits();
      _showSettingsSuccess('Einheit gespeichert');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _deleteUnit(String code) async {
    final ok = await _confirmDelete(
      title: 'Einheit löschen',
      message: 'Code "$code" wirklich löschen?',
    );
    if (ok != true) return;
    try {
      await widget.api.deleteUnit(code);
      await _loadUnits();
      _showSettingsSuccess('Einheit gelöscht');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _loadMaterialGroups() async {
    try {
      setState(() => _materialGroupsLoading = true);
      final list = await widget.api.listMaterialGroups();
      setState(() => _materialGroups = list);
    } catch (e) {/* ignore */} finally {
      setState(() => _materialGroupsLoading = false);
    }
  }

  Future<void> _saveMaterialGroup() async {
    final code = _materialGroupCodeCtrl.text.trim();
    final name = _materialGroupNameCtrl.text.trim();
    final description = _materialGroupDescriptionCtrl.text.trim();
    final sortOrder =
        int.tryParse(_materialGroupSortOrderCtrl.text.trim()) ?? 0;
    if (code.isEmpty) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Code erforderlich')));
      return;
    }
    if (name.isEmpty) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Name erforderlich')));
      return;
    }
    try {
      await widget.api.upsertMaterialGroup(
        code: code,
        name: name,
        description: description,
        sortOrder: sortOrder,
        isActive: _materialGroupIsActive,
      );
      _materialGroupCodeCtrl.clear();
      _materialGroupNameCtrl.clear();
      _materialGroupDescriptionCtrl.clear();
      _materialGroupSortOrderCtrl.text = '0';
      setState(() => _materialGroupIsActive = true);
      await _loadMaterialGroups();
      _showSettingsSuccess('Materialgruppe gespeichert');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _deleteMaterialGroup(String code) async {
    final ok = await _confirmDelete(
      title: 'Materialgruppe löschen',
      message: 'Code "$code" wirklich löschen?',
    );
    if (ok != true) return;
    try {
      await widget.api.deleteMaterialGroup(code);
      await _loadMaterialGroups();
      _showSettingsSuccess('Materialgruppe gelöscht');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _loadQuoteTextBlocks() async {
    try {
      setState(() => _quoteTextBlocksLoading = true);
      final list = await widget.api.listQuoteTextBlocks();
      setState(() => _quoteTextBlocks = list);
    } catch (e) {/* ignore */} finally {
      setState(() => _quoteTextBlocksLoading = false);
    }
  }

  void _resetQuoteTextBlockForm() {
    _quoteTextBlockIdCtrl.clear();
    _quoteTextBlockCodeCtrl.clear();
    _quoteTextBlockNameCtrl.clear();
    _quoteTextBlockBodyCtrl.clear();
    _quoteTextBlockSortOrderCtrl.text = '0';
    setState(() {
      _quoteTextBlockCategory = 'intro';
      _quoteTextBlockIsActive = true;
    });
  }

  Future<void> _saveQuoteTextBlock() async {
    final code = _quoteTextBlockCodeCtrl.text.trim();
    final name = _quoteTextBlockNameCtrl.text.trim();
    final body = _quoteTextBlockBodyCtrl.text.trim();
    final sortOrder =
        int.tryParse(_quoteTextBlockSortOrderCtrl.text.trim()) ?? 0;
    if (code.isEmpty) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Code erforderlich')));
      return;
    }
    if (name.isEmpty) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Name erforderlich')));
      return;
    }
    if (body.isEmpty) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Text erforderlich')));
      return;
    }
    try {
      await widget.api.upsertQuoteTextBlock(
        id: _quoteTextBlockIdCtrl.text.trim(),
        code: code,
        name: name,
        category: _quoteTextBlockCategory,
        body: body,
        sortOrder: sortOrder,
        isActive: _quoteTextBlockIsActive,
      );
      _resetQuoteTextBlockForm();
      await _loadQuoteTextBlocks();
      _showSettingsSuccess('Textbaustein gespeichert');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _deleteQuoteTextBlock(Map<String, dynamic> block) async {
    final id = (block['id'] ?? '').toString();
    final code = (block['code'] ?? '').toString();
    final ok = await _confirmDelete(
      title: 'Textbaustein löschen',
      message: 'Textbaustein "$code" wirklich löschen?',
    );
    if (ok != true) return;
    try {
      await widget.api.deleteQuoteTextBlock(id);
      if (_quoteTextBlockIdCtrl.text.trim() == id) {
        _resetQuoteTextBlockForm();
      }
      await _loadQuoteTextBlocks();
      _showSettingsSuccess('Textbaustein gelöscht');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<bool?> _confirmDelete({
    required String title,
    required String message,
  }) {
    return showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: Text(message),
        actions: _buildDialogActions(
          dialogContext: dialogContext,
          confirmLabel: 'Löschen',
          onConfirm: () => Navigator.pop(dialogContext, true),
        ),
      ),
    );
  }

  Future<void> _updateNumberingPreview(
    String entity,
    void Function(String value) setPreview,
  ) async {
    try {
      final preview = await widget.api.previewNumbering(entity);
      setState(() => setPreview(preview));
    } catch (e) {
      setState(() => setPreview(''));
    }
  }

  Future<void> _updatePreviewPO() {
    return _updateNumberingPreview(
      'purchase_order',
      (value) => previewPO = value,
    );
  }

  Future<void> _updatePreviewPRJ() {
    return _updateNumberingPreview(
      'project',
      (value) => previewPRJ = value,
    );
  }

  Future<void> _saveNumberingPattern(
    String entity,
    TextEditingController controller,
    Future<void> Function() refreshPreview,
  ) async {
    try {
      await widget.api.updateNumberingPattern(entity, controller.text.trim());
      await refreshPreview();
      _showSettingsSuccess('Gespeichert');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _savePO() {
    return _saveNumberingPattern(
      'purchase_order',
      poPatternCtrl,
      _updatePreviewPO,
    );
  }

  Future<void> _savePRJ() {
    return _saveNumberingPattern(
      'project',
      prjPatternCtrl,
      _updatePreviewPRJ,
    );
  }

  void _applyPdfTemplateData(
    Map<String, dynamic> template, {
    required TextEditingController headerCtrl,
    required TextEditingController footerCtrl,
    required TextEditingController topFirstCtrl,
    required TextEditingController topOtherCtrl,
    required void Function(String value) setEffectiveHeaderText,
    required void Function(String value) setEffectiveFooterText,
    required void Function(String value) setEffectiveDisplayName,
    required void Function(String value) setEffectiveClaim,
    required void Function(String value) setEffectivePrimaryColor,
    required void Function(String value) setEffectiveAccentColor,
    required void Function(String? value) setLogoDocId,
    required void Function(String? value) setBgFirstDocId,
    required void Function(String? value) setBgOtherDocId,
  }) {
    final headerText = (template['header_text'] ?? '').toString();
    final footerText = (template['footer_text'] ?? '').toString();
    final topFirstMm =
        double.tryParse('${template['top_first_mm'] ?? '30'}') ?? 30;
    final topOtherMm =
        double.tryParse('${template['top_other_mm'] ?? '20'}') ?? 20;
    headerCtrl.text = headerText;
    footerCtrl.text = footerText;
    topFirstCtrl.text = topFirstMm.toStringAsFixed(0);
    topOtherCtrl.text = topOtherMm.toStringAsFixed(0);
    setEffectiveHeaderText(
        (template['effective_header_text'] ?? '').toString());
    setEffectiveFooterText(
        (template['effective_footer_text'] ?? '').toString());
    setEffectiveDisplayName(
        (template['effective_display_name'] ?? '').toString());
    setEffectiveClaim((template['effective_claim'] ?? '').toString());
    setEffectivePrimaryColor(
        (template['effective_primary_color'] ?? '').toString());
    setEffectiveAccentColor(
        (template['effective_accent_color'] ?? '').toString());
    setLogoDocId(template['logo_doc_id'] as String?);
    setBgFirstDocId(template['bg_first_doc_id'] as String?);
    setBgOtherDocId(template['bg_other_doc_id'] as String?);
  }

  ({
    TextEditingController headerCtrl,
    TextEditingController footerCtrl,
    TextEditingController topFirstCtrl,
    TextEditingController topOtherCtrl,
  }) _pdfTemplateControllers(String entity) {
    final isPurchaseOrder = entity == 'purchase_order';
    final isInvoiceOut = entity == 'invoice_out';
    final isQuote = entity == 'quote';
    return (
      headerCtrl: isPurchaseOrder
          ? poHeaderCtrl
          : isInvoiceOut
              ? invoiceHeaderCtrl
              : isQuote
                  ? quoteHeaderCtrl
                  : salesOrderHeaderCtrl,
      footerCtrl: isPurchaseOrder
          ? poFooterCtrl
          : isInvoiceOut
              ? invoiceFooterCtrl
              : isQuote
                  ? quoteFooterCtrl
                  : salesOrderFooterCtrl,
      topFirstCtrl: isPurchaseOrder
          ? poTopFirstCtrl
          : isInvoiceOut
              ? invoiceTopFirstCtrl
              : isQuote
                  ? quoteTopFirstCtrl
                  : salesOrderTopFirstCtrl,
      topOtherCtrl: isPurchaseOrder
          ? poTopOtherCtrl
          : isInvoiceOut
              ? invoiceTopOtherCtrl
              : isQuote
                  ? quoteTopOtherCtrl
                  : salesOrderTopOtherCtrl,
    );
  }

  void _setPdfTemplateDocumentId(
    String entity,
    String kind,
    String? value,
  ) {
    if (entity == 'purchase_order') {
      if (kind == 'logo') poLogoDocId = value;
      if (kind == 'bg-first') poBgFirstDocId = value;
      if (kind == 'bg-other') poBgOtherDocId = value;
    } else if (entity == 'invoice_out') {
      if (kind == 'logo') invoiceLogoDocId = value;
      if (kind == 'bg-first') invoiceBgFirstDocId = value;
      if (kind == 'bg-other') invoiceBgOtherDocId = value;
    } else if (entity == 'quote') {
      if (kind == 'logo') quoteLogoDocId = value;
      if (kind == 'bg-first') quoteBgFirstDocId = value;
      if (kind == 'bg-other') quoteBgOtherDocId = value;
    } else if (entity == 'sales_order') {
      if (kind == 'logo') salesOrderLogoDocId = value;
      if (kind == 'bg-first') salesOrderBgFirstDocId = value;
      if (kind == 'bg-other') salesOrderBgOtherDocId = value;
    }
  }

  Future<void> _loadPdfTemplate(String entity) async {
    try {
      final t = await widget.api.getPdfTemplate(entity);
      if (entity == 'purchase_order') {
        _applyPdfTemplateData(
          t,
          headerCtrl: poHeaderCtrl,
          footerCtrl: poFooterCtrl,
          topFirstCtrl: poTopFirstCtrl,
          topOtherCtrl: poTopOtherCtrl,
          setEffectiveHeaderText: (value) => poEffectiveHeaderText = value,
          setEffectiveFooterText: (value) => poEffectiveFooterText = value,
          setEffectiveDisplayName: (value) => poEffectiveDisplayName = value,
          setEffectiveClaim: (value) => poEffectiveClaim = value,
          setEffectivePrimaryColor: (value) => poEffectivePrimaryColor = value,
          setEffectiveAccentColor: (value) => poEffectiveAccentColor = value,
          setLogoDocId: (value) => poLogoDocId = value,
          setBgFirstDocId: (value) => poBgFirstDocId = value,
          setBgOtherDocId: (value) => poBgOtherDocId = value,
        );
      } else if (entity == 'invoice_out') {
        _applyPdfTemplateData(
          t,
          headerCtrl: invoiceHeaderCtrl,
          footerCtrl: invoiceFooterCtrl,
          topFirstCtrl: invoiceTopFirstCtrl,
          topOtherCtrl: invoiceTopOtherCtrl,
          setEffectiveHeaderText: (value) => invoiceEffectiveHeaderText = value,
          setEffectiveFooterText: (value) => invoiceEffectiveFooterText = value,
          setEffectiveDisplayName: (value) =>
              invoiceEffectiveDisplayName = value,
          setEffectiveClaim: (value) => invoiceEffectiveClaim = value,
          setEffectivePrimaryColor: (value) =>
              invoiceEffectivePrimaryColor = value,
          setEffectiveAccentColor: (value) =>
              invoiceEffectiveAccentColor = value,
          setLogoDocId: (value) => invoiceLogoDocId = value,
          setBgFirstDocId: (value) => invoiceBgFirstDocId = value,
          setBgOtherDocId: (value) => invoiceBgOtherDocId = value,
        );
      } else if (entity == 'quote') {
        _applyPdfTemplateData(
          t,
          headerCtrl: quoteHeaderCtrl,
          footerCtrl: quoteFooterCtrl,
          topFirstCtrl: quoteTopFirstCtrl,
          topOtherCtrl: quoteTopOtherCtrl,
          setEffectiveHeaderText: (value) => quoteEffectiveHeaderText = value,
          setEffectiveFooterText: (value) => quoteEffectiveFooterText = value,
          setEffectiveDisplayName: (value) => quoteEffectiveDisplayName = value,
          setEffectiveClaim: (value) => quoteEffectiveClaim = value,
          setEffectivePrimaryColor: (value) =>
              quoteEffectivePrimaryColor = value,
          setEffectiveAccentColor: (value) => quoteEffectiveAccentColor = value,
          setLogoDocId: (value) => quoteLogoDocId = value,
          setBgFirstDocId: (value) => quoteBgFirstDocId = value,
          setBgOtherDocId: (value) => quoteBgOtherDocId = value,
        );
      } else if (entity == 'sales_order') {
        _applyPdfTemplateData(
          t,
          headerCtrl: salesOrderHeaderCtrl,
          footerCtrl: salesOrderFooterCtrl,
          topFirstCtrl: salesOrderTopFirstCtrl,
          topOtherCtrl: salesOrderTopOtherCtrl,
          setEffectiveHeaderText: (value) =>
              salesOrderEffectiveHeaderText = value,
          setEffectiveFooterText: (value) =>
              salesOrderEffectiveFooterText = value,
          setEffectiveDisplayName: (value) =>
              salesOrderEffectiveDisplayName = value,
          setEffectiveClaim: (value) => salesOrderEffectiveClaim = value,
          setEffectivePrimaryColor: (value) =>
              salesOrderEffectivePrimaryColor = value,
          setEffectiveAccentColor: (value) =>
              salesOrderEffectiveAccentColor = value,
          setLogoDocId: (value) => salesOrderLogoDocId = value,
          setBgFirstDocId: (value) => salesOrderBgFirstDocId = value,
          setBgOtherDocId: (value) => salesOrderBgOtherDocId = value,
        );
      }
      if (mounted) setState(() {});
    } catch (_) {
      // ignore
    }
  }

  Future<void> _savePdfTemplate(String entity) async {
    try {
      final controllers = _pdfTemplateControllers(entity);
      final tf = double.tryParse(
              controllers.topFirstCtrl.text.trim().replaceAll(',', '.')) ??
          30;
      final to = double.tryParse(
              controllers.topOtherCtrl.text.trim().replaceAll(',', '.')) ??
          20;
      await widget.api.updatePdfTemplate(
        entity,
        headerText: controllers.headerCtrl.text,
        footerText: controllers.footerCtrl.text,
        topFirstMm: tf,
        topOtherMm: to,
      );
      await _loadPdfTemplate(entity);
      _showSettingsSuccess('PDF-Template gespeichert');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _pickAndUpload(String entity, String kind) async {
    final picked = await browser.pickFile(accept: 'image/*,application/pdf');
    if (picked == null) return;
    try {
      final res = await widget.api.uploadPdfImage(
          entity, kind, picked.filename, picked.bytes,
          contentType: picked.contentType);
      final id = (res['document_id'] ?? '').toString();
      setState(() {
        _setPdfTemplateDocumentId(entity, kind, id);
      });
      _showSettingsSuccess('Upload erfolgreich');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _deleteImage(String entity, String kind) async {
    try {
      await widget.api.deletePdfImage(entity, kind);
      setState(() {
        _setPdfTemplateDocumentId(entity, kind, null);
      });
      _showSettingsSuccess('Bild entfernt');
    } catch (e) {
      _showSettingsError(e);
    }
  }

  Future<void> _openSalesOrders() async {
    await Navigator.of(context).push(
      MaterialPageRoute(builder: (_) => buildSalesOrdersPage(api: widget.api)),
    );
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
                title: _buildSectionTitle('Firmenprofil'),
                initiallyExpanded: true,
                children: [
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          _buildTwoFieldRow(
                            left: TextField(
                              controller: companyNameCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Firmenname',
                              ),
                            ),
                            right: TextField(
                              controller: companyLegalFormCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Rechtsform',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          _buildTwoFieldRow(
                            left: TextField(
                              controller: companyBranchCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Niederlassung / Standort',
                              ),
                            ),
                            right: TextField(
                              controller: companyInvoiceEmailCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Rechnungs-E-Mail',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          _buildAddressFieldRow(
                            street: TextField(
                              controller: companyStreetCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Straße',
                              ),
                            ),
                            postalCode: TextField(
                              controller: companyPostalCodeCtrl,
                              decoration: const InputDecoration(
                                labelText: 'PLZ',
                              ),
                            ),
                            city: TextField(
                              controller: companyCityCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Ort',
                              ),
                            ),
                            country: TextField(
                              controller: companyCountryCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Land',
                              ),
                            ),
                            postalWidth: 140,
                            countryWidth: 100,
                          ),
                          const SizedBox(height: 12),
                          _buildThreeFieldRow(
                            first: TextField(
                              controller: companyEmailCtrl,
                              decoration: const InputDecoration(
                                labelText: 'E-Mail',
                              ),
                            ),
                            second: TextField(
                              controller: companyPhoneCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Telefon',
                              ),
                            ),
                            third: TextField(
                              controller: companyWebsiteCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Website',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          _buildTwoFieldRow(
                            left: TextField(
                              controller: companyTaxNoCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Steuernummer',
                              ),
                            ),
                            right: TextField(
                              controller: companyVatIdCtrl,
                              decoration: const InputDecoration(
                                labelText: 'USt-IdNr.',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          const Text('Bankdaten',
                              style: TextStyle(fontWeight: FontWeight.bold)),
                          const SizedBox(height: 8),
                          _buildTwoFieldRow(
                            left: TextField(
                              controller: companyBankNameCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Bank',
                              ),
                            ),
                            right: TextField(
                              controller: companyAccountHolderCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Kontoinhaber',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          _buildTwoFieldRow(
                            left: TextField(
                              controller: companyIbanCtrl,
                              decoration: const InputDecoration(
                                labelText: 'IBAN',
                              ),
                            ),
                            right: TextField(
                              controller: companyBicCtrl,
                              decoration: const InputDecoration(
                                labelText: 'BIC',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          _buildSectionSaveAction(
                            onPressed: _saveCompanyProfile,
                            icon: const Icon(Icons.save),
                            label: const Text('Speichern'),
                          ),
                          const SizedBox(height: 16),
                          const Divider(),
                          const SizedBox(height: 8),
                          const Text('Steuer, Währung und Lokalisierung',
                              style: TextStyle(fontWeight: FontWeight.bold)),
                          const SizedBox(height: 8),
                          _buildThreeFieldRow(
                            first: TextField(
                              controller: localizationCurrencyCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Standardwährung',
                              ),
                            ),
                            second: TextField(
                              controller: localizationTaxCountryCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Steuerland',
                              ),
                            ),
                            third: TextField(
                              controller: localizationVatRateCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Standard-USt. %',
                              ),
                              keyboardType: TextInputType.number,
                            ),
                          ),
                          const SizedBox(height: 12),
                          _buildTwoFieldRow(
                            left: TextField(
                              controller: localizationLocaleCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Locale',
                              ),
                            ),
                            right: TextField(
                              controller: localizationTimezoneCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Zeitzone',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          _buildTwoFieldRow(
                            left: TextField(
                              controller: localizationDateFormatCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Datumsformat',
                              ),
                            ),
                            right: TextField(
                              controller: localizationNumberFormatCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Zahlenformat',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          _buildSectionSaveAction(
                            onPressed: _saveLocalizationSettings,
                            icon: const Icon(Icons.language),
                            label: const Text('Lokalisierung speichern'),
                          ),
                          const SizedBox(height: 16),
                          Row(
                            children: [
                              const Text('Niederlassungen',
                                  style:
                                      TextStyle(fontWeight: FontWeight.bold)),
                              const Spacer(),
                              OutlinedButton.icon(
                                onPressed: () => _editBranch(),
                                icon: const Icon(Icons.add_business_outlined),
                                label: const Text('Niederlassung anlegen'),
                              ),
                            ],
                          ),
                          const SizedBox(height: 8),
                          if (_branchesLoading) _buildInlineLoadingIndicator(),
                          if (!_branchesLoading && _branches.isEmpty)
                            const Padding(
                              padding: EdgeInsets.symmetric(vertical: 8),
                              child:
                                  Text('Noch keine Niederlassungen angelegt.'),
                            ),
                          ..._branches.map((b) {
                            final title = (b['name'] ?? '').toString();
                            final code = (b['code'] ?? '').toString();
                            final city = (b['city'] ?? '').toString();
                            final country = (b['country'] ?? '').toString();
                            final isDefault = b['is_default'] == true;
                            final subtitleParts = <String>[
                              if (code.isNotEmpty) 'Code: $code',
                              if (city.isNotEmpty || country.isNotEmpty)
                                [city, country]
                                    .where((e) => e.isNotEmpty)
                                    .join(', '),
                              if ((b['email'] ?? '').toString().isNotEmpty)
                                (b['email'] ?? '').toString(),
                            ];
                            return ListTile(
                              contentPadding: EdgeInsets.zero,
                              leading: Icon(isDefault
                                  ? Icons.apartment_rounded
                                  : Icons.business_outlined),
                              title:
                                  Text(isDefault ? '$title (Standard)' : title),
                              subtitle: subtitleParts.isNotEmpty
                                  ? Text(subtitleParts.join(' • '))
                                  : null,
                              trailing: Wrap(
                                spacing: 4,
                                children: [
                                  _buildListTileActionButton(
                                    icon: const Icon(Icons.edit_outlined),
                                    onPressed: () => _editBranch(b),
                                  ),
                                  _buildListTileActionButton(
                                    icon: const Icon(Icons.delete_outline),
                                    onPressed: () => _deleteBranch(b),
                                  ),
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
                title: _buildSectionTitle('Branding & Dokumentlayout'),
                initiallyExpanded: false,
                children: [
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          _buildTwoFieldRow(
                            left: TextField(
                              controller: brandingDisplayNameCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Brand-Name',
                              ),
                            ),
                            right: TextField(
                              controller: brandingClaimCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Claim / Zusatzzeile',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          _buildTwoFieldRow(
                            left: TextField(
                              controller: brandingPrimaryColorCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Primärfarbe (Hex)',
                              ),
                            ),
                            right: TextField(
                              controller: brandingAccentColorCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Akzentfarbe (Hex)',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          TextField(
                            controller: brandingHeaderCtrl,
                            maxLines: 3,
                            decoration: const InputDecoration(
                                labelText: 'Standard-Kopftext für Dokumente'),
                          ),
                          const SizedBox(height: 12),
                          TextField(
                            controller: brandingFooterCtrl,
                            maxLines: 3,
                            decoration: const InputDecoration(
                                labelText: 'Standard-Fußtext für Dokumente'),
                          ),
                          const SizedBox(height: 12),
                          _buildSectionSaveAction(
                            onPressed: _saveBrandingSettings,
                            icon: const Icon(Icons.palette_outlined),
                            label: const Text('Branding speichern'),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              ExpansionTile(
                title: _buildSectionTitle('Nummernkreise'),
                initiallyExpanded: false,
                childrenPadding: const EdgeInsets.only(bottom: 8),
                children: _buildNumberingCards(),
              ),
              const SizedBox(height: 12),
              ExpansionTile(
                title: _buildSectionTitle('Maßeinheiten'),
                initiallyExpanded: false,
                children: [
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Row(children: [
                              Expanded(
                                  child: TextField(
                                      controller: _unitCodeCtrl,
                                      decoration: const InputDecoration(
                                          labelText: 'Code (z. B. kg, mm)'))),
                              const SizedBox(width: 8),
                              Expanded(
                                  child: TextField(
                                      controller: _unitNameCtrl,
                                      decoration: const InputDecoration(
                                          labelText:
                                              'Name (optional, z. B. Kilogramm)'))),
                              const SizedBox(width: 8),
                              FilledButton.icon(
                                  onPressed: _saveUnit,
                                  icon: const Icon(Icons.save),
                                  label: const Text('Speichern')),
                            ]),
                            const SizedBox(height: 12),
                            if (_unitsLoading) _buildInlineLoadingIndicator(),
                            ListView.builder(
                              shrinkWrap: true,
                              physics: const NeverScrollableScrollPhysics(),
                              itemCount: _units.length,
                              itemBuilder: (ctx, i) {
                                final u = _units[i];
                                final code = (u['code'] ?? '').toString();
                                final name = (u['name'] ?? '').toString();
                                return ListTile(
                                  dense: true,
                                  leading: const Icon(Icons.straighten_rounded),
                                  title: Text(code),
                                  subtitle: name.isNotEmpty ? Text(name) : null,
                                  trailing: _buildListTileActionButton(
                                    icon: const Icon(Icons.delete_outline),
                                    onPressed: () => _deleteUnit(code),
                                  ),
                                  onTap: () {
                                    _unitCodeCtrl.text = code;
                                    _unitNameCtrl.text = name;
                                  },
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
                title: _buildSectionTitle('Materialgruppen'),
                initiallyExpanded: false,
                children: [
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          _buildTwoFieldRow(
                            left: TextField(
                              controller: _materialGroupCodeCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Code',
                              ),
                            ),
                            right: TextField(
                              controller: _materialGroupNameCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Name',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          TextField(
                            controller: _materialGroupDescriptionCtrl,
                            decoration: const InputDecoration(
                              labelText: 'Beschreibung',
                            ),
                          ),
                          const SizedBox(height: 12),
                          Row(
                            children: [
                              SizedBox(
                                width: 140,
                                child: TextField(
                                  controller: _materialGroupSortOrderCtrl,
                                  decoration: const InputDecoration(
                                    labelText: 'Sortierung',
                                  ),
                                  keyboardType: TextInputType.number,
                                ),
                              ),
                              const SizedBox(width: 12),
                              Expanded(
                                child: SwitchListTile(
                                  contentPadding: EdgeInsets.zero,
                                  title: const Text('Aktiv'),
                                  value: _materialGroupIsActive,
                                  onChanged: (value) => setState(
                                    () => _materialGroupIsActive = value,
                                  ),
                                ),
                              ),
                            ],
                          ),
                          _buildSectionSaveAction(
                            onPressed: _saveMaterialGroup,
                            icon: const Icon(Icons.save),
                            label: const Text('Speichern'),
                          ),
                          const SizedBox(height: 12),
                          if (_materialGroupsLoading)
                            _buildInlineLoadingIndicator(),
                          ListView.builder(
                            shrinkWrap: true,
                            physics: const NeverScrollableScrollPhysics(),
                            itemCount: _materialGroups.length,
                            itemBuilder: (ctx, i) {
                              final group = _materialGroups[i];
                              final code = (group['code'] ?? '').toString();
                              final name = (group['name'] ?? '').toString();
                              final description =
                                  (group['description'] ?? '').toString();
                              final sortOrder =
                                  (group['sort_order'] ?? 0).toString();
                              final isActive = group['is_active'] != false;
                              final subtitleParts = <String>[
                                if (description.isNotEmpty) description,
                                'Sortierung: $sortOrder',
                                if (!isActive) 'inaktiv',
                              ];
                              return ListTile(
                                dense: true,
                                leading: Icon(
                                  isActive
                                      ? Icons.category_outlined
                                      : Icons.category_outlined,
                                ),
                                title: Text(code),
                                subtitle: Text(
                                  subtitleParts.isEmpty
                                      ? name
                                      : '$name • ${subtitleParts.join(' • ')}',
                                ),
                                trailing: _buildListTileActionButton(
                                  icon: const Icon(Icons.delete_outline),
                                  onPressed: () => _deleteMaterialGroup(code),
                                ),
                                onTap: () {
                                  _materialGroupCodeCtrl.text = code;
                                  _materialGroupNameCtrl.text = name;
                                  _materialGroupDescriptionCtrl.text =
                                      description;
                                  _materialGroupSortOrderCtrl.text = sortOrder;
                                  setState(
                                    () => _materialGroupIsActive = isActive,
                                  );
                                },
                              );
                            },
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              ExpansionTile(
                title: _buildSectionTitle('Angebots-Textbausteine'),
                initiallyExpanded: false,
                children: [
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          _buildTwoFieldRow(
                            left: TextField(
                              controller: _quoteTextBlockCodeCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Code',
                              ),
                            ),
                            right: TextField(
                              controller: _quoteTextBlockNameCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Name',
                              ),
                            ),
                          ),
                          const SizedBox(height: 12),
                          Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Expanded(
                                child: DropdownButtonFormField<String>(
                                  initialValue: _quoteTextBlockCategory,
                                  decoration: const InputDecoration(
                                    labelText: 'Kategorie',
                                  ),
                                  items: _quoteTextBlockCategories
                                      .map(
                                        (category) => DropdownMenuItem<String>(
                                          value: category,
                                          child: Text(category),
                                        ),
                                      )
                                      .toList(),
                                  onChanged: (value) {
                                    if (value == null) return;
                                    setState(
                                      () => _quoteTextBlockCategory = value,
                                    );
                                  },
                                ),
                              ),
                              const SizedBox(width: 12),
                              SizedBox(
                                width: 140,
                                child: TextField(
                                  controller: _quoteTextBlockSortOrderCtrl,
                                  decoration: const InputDecoration(
                                    labelText: 'Sortierung',
                                  ),
                                  keyboardType: TextInputType.number,
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: 12),
                          TextField(
                            controller: _quoteTextBlockBodyCtrl,
                            minLines: 4,
                            maxLines: 8,
                            decoration: const InputDecoration(
                              labelText: 'Textbaustein',
                              alignLabelWithHint: true,
                            ),
                          ),
                          const SizedBox(height: 12),
                          SwitchListTile(
                            contentPadding: EdgeInsets.zero,
                            title: const Text('Aktiv'),
                            value: _quoteTextBlockIsActive,
                            onChanged: (value) => setState(
                              () => _quoteTextBlockIsActive = value,
                            ),
                          ),
                          Row(
                            children: [
                              _buildSectionSaveAction(
                                onPressed: _saveQuoteTextBlock,
                                icon: const Icon(Icons.save),
                                label: const Text('Speichern'),
                              ),
                              const SizedBox(width: 12),
                              TextButton.icon(
                                onPressed: _resetQuoteTextBlockForm,
                                icon: const Icon(Icons.clear),
                                label: const Text('Zurücksetzen'),
                              ),
                            ],
                          ),
                          const SizedBox(height: 12),
                          if (_quoteTextBlocksLoading)
                            _buildInlineLoadingIndicator(),
                          ListView.builder(
                            shrinkWrap: true,
                            physics: const NeverScrollableScrollPhysics(),
                            itemCount: _quoteTextBlocks.length,
                            itemBuilder: (ctx, i) {
                              final block = _quoteTextBlocks[i];
                              final id = (block['id'] ?? '').toString();
                              final code = (block['code'] ?? '').toString();
                              final name = (block['name'] ?? '').toString();
                              final category =
                                  (block['category'] ?? '').toString();
                              final body = (block['body'] ?? '').toString();
                              final sortOrder =
                                  (block['sort_order'] ?? 0).toString();
                              final isActive = block['is_active'] != false;
                              final preview = body.replaceAll('\n', ' ').trim();
                              final subtitleParts = <String>[
                                if (category.isNotEmpty) category,
                                'Sortierung: $sortOrder',
                                if (!isActive) 'inaktiv',
                                if (preview.isNotEmpty) preview,
                              ];
                              return ListTile(
                                dense: true,
                                leading: const Icon(Icons.short_text_rounded),
                                title: Text('$code • $name'),
                                subtitle: Text(subtitleParts.join(' • ')),
                                trailing: _buildListTileActionButton(
                                  icon: const Icon(Icons.delete_outline),
                                  onPressed: () => _deleteQuoteTextBlock(block),
                                ),
                                onTap: () {
                                  _quoteTextBlockIdCtrl.text = id;
                                  _quoteTextBlockCodeCtrl.text = code;
                                  _quoteTextBlockNameCtrl.text = name;
                                  _quoteTextBlockBodyCtrl.text = body;
                                  _quoteTextBlockSortOrderCtrl.text = sortOrder;
                                  setState(() {
                                    _quoteTextBlockCategory =
                                        _quoteTextBlockCategories
                                                .contains(category)
                                            ? category
                                            : 'intro';
                                    _quoteTextBlockIsActive = isActive;
                                  });
                                },
                              );
                            },
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              ExpansionTile(
                title: _buildSectionTitle('PDF-Templates'),
                initiallyExpanded: false,
                children: [
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: _buildPdfTemplateCards(),
                      ),
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

  Widget _buildNumberingCard({
    required String title,
    required TextEditingController controller,
    required String preview,
    required String hintText,
    required VoidCallback onChanged,
    required VoidCallback onSave,
  }) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title),
            const SizedBox(height: 8),
            TextField(
              controller: controller,
              decoration: InputDecoration(
                labelText: 'Pattern',
                hintText: hintText,
              ),
              onChanged: (_) {
                onChanged();
              },
            ),
            const SizedBox(height: 8),
            Text('Vorschau: $preview'),
            const SizedBox(height: 8),
            const Text(
              'Variablen: {YYYY}, {YY}, {MM}, {DD}, {NN}, {NNN}, {NNNN}',
            ),
            const SizedBox(height: 8),
            _buildSectionSaveAction(
              onPressed: onSave,
              icon: const Icon(Icons.save),
              label: const Text('Speichern'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSectionTitle(String title) {
    return Text(
      title,
      style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
    );
  }

  Widget _buildTwoFieldRow({
    required Widget left,
    required Widget right,
    double spacing = 12,
  }) {
    return Row(
      children: [
        Expanded(child: left),
        SizedBox(width: spacing),
        Expanded(child: right),
      ],
    );
  }

  Widget _buildThreeFieldRow({
    required Widget first,
    required Widget second,
    required Widget third,
    double spacing = 12,
  }) {
    return Row(
      children: [
        Expanded(child: first),
        SizedBox(width: spacing),
        Expanded(child: second),
        SizedBox(width: spacing),
        Expanded(child: third),
      ],
    );
  }

  Widget _buildAddressFieldRow({
    required Widget street,
    required Widget postalCode,
    required Widget city,
    required Widget country,
    required double postalWidth,
    required double countryWidth,
    double spacing = 12,
  }) {
    return Row(
      children: [
        Expanded(child: street),
        SizedBox(width: spacing),
        SizedBox(width: postalWidth, child: postalCode),
        SizedBox(width: spacing),
        Expanded(child: city),
        SizedBox(width: spacing),
        SizedBox(width: countryWidth, child: country),
      ],
    );
  }

  Widget _buildInlineLoadingIndicator() {
    return const LinearProgressIndicator(minHeight: 2);
  }

  List<Widget> _buildDialogActions({
    required BuildContext dialogContext,
    required String confirmLabel,
    required VoidCallback onConfirm,
    String cancelLabel = 'Abbrechen',
  }) {
    return [
      TextButton(
        onPressed: () => Navigator.pop(dialogContext, false),
        child: Text(cancelLabel),
      ),
      FilledButton(
        onPressed: onConfirm,
        child: Text(confirmLabel),
      ),
    ];
  }

  Widget _buildListTileActionButton({
    required Widget icon,
    required VoidCallback onPressed,
  }) {
    return IconButton(
      icon: icon,
      onPressed: onPressed,
    );
  }

  List<Widget> _buildNumberingCards() {
    final configs = _numberingCardConfigs();
    return [
      for (var i = 0; i < configs.length; i++) ...[
        _buildNumberingCard(
          title: configs[i].title,
          controller: configs[i].controller,
          preview: configs[i].preview,
          hintText: configs[i].hintText,
          onChanged: configs[i].onChanged,
          onSave: configs[i].onSave,
        ),
        if (i < configs.length - 1) const SizedBox(height: 12),
      ],
    ];
  }

  Widget _buildSectionSaveAction({
    required VoidCallback onPressed,
    required Widget icon,
    required Widget label,
  }) {
    return Align(
      alignment: Alignment.centerRight,
      child: FilledButton.icon(
        onPressed: onPressed,
        icon: icon,
        label: label,
      ),
    );
  }

  List<
      ({
        String title,
        TextEditingController controller,
        String preview,
        String hintText,
        VoidCallback onChanged,
        VoidCallback onSave,
      })> _numberingCardConfigs() {
    return [
      (
        title: 'Bestellungen',
        controller: poPatternCtrl,
        preview: previewPO,
        hintText: 'z. B. PO-{YYYY}-{NNNN}',
        onChanged: _updatePreviewPO,
        onSave: _savePO,
      ),
      (
        title: 'Projekte',
        controller: prjPatternCtrl,
        preview: previewPRJ,
        hintText: 'z. B. PRJ-{YYYY}-{NNNN}',
        onChanged: _updatePreviewPRJ,
        onSave: _savePRJ,
      ),
    ];
  }

  List<Widget> _buildPdfTemplateCards() {
    final cards = _pdfTemplateCardConfigs();
    return [
      for (var index = 0; index < cards.length; index++) ...[
        if (index > 0) const Divider(height: 32),
        _buildPdfTemplateCard(
          title: cards[index].title,
          entity: cards[index].entity,
          headerCtrl: cards[index].headerCtrl,
          footerCtrl: cards[index].footerCtrl,
          topFirstCtrl: cards[index].topFirstCtrl,
          topOtherCtrl: cards[index].topOtherCtrl,
          effectiveHeaderText: cards[index].effectiveHeaderText,
          effectiveFooterText: cards[index].effectiveFooterText,
          effectiveDisplayName: cards[index].effectiveDisplayName,
          effectiveClaim: cards[index].effectiveClaim,
          effectivePrimaryColor: cards[index].effectivePrimaryColor,
          effectiveAccentColor: cards[index].effectiveAccentColor,
          logoDocId: cards[index].logoDocId,
          bgFirstDocId: cards[index].bgFirstDocId,
          bgOtherDocId: cards[index].bgOtherDocId,
        ),
      ],
    ];
  }

  List<
      ({
        String title,
        String entity,
        TextEditingController headerCtrl,
        TextEditingController footerCtrl,
        TextEditingController topFirstCtrl,
        TextEditingController topOtherCtrl,
        String effectiveHeaderText,
        String effectiveFooterText,
        String effectiveDisplayName,
        String effectiveClaim,
        String effectivePrimaryColor,
        String effectiveAccentColor,
        String? logoDocId,
        String? bgFirstDocId,
        String? bgOtherDocId,
      })> _pdfTemplateCardConfigs() {
    return [
      (
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
      (
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
      (
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
      (
        title: 'Aufträge (sales_order)',
        entity: 'sales_order',
        headerCtrl: salesOrderHeaderCtrl,
        footerCtrl: salesOrderFooterCtrl,
        topFirstCtrl: salesOrderTopFirstCtrl,
        topOtherCtrl: salesOrderTopOtherCtrl,
        effectiveHeaderText: salesOrderEffectiveHeaderText,
        effectiveFooterText: salesOrderEffectiveFooterText,
        effectiveDisplayName: salesOrderEffectiveDisplayName,
        effectiveClaim: salesOrderEffectiveClaim,
        effectivePrimaryColor: salesOrderEffectivePrimaryColor,
        effectiveAccentColor: salesOrderEffectiveAccentColor,
        logoDocId: salesOrderLogoDocId,
        bgFirstDocId: salesOrderBgFirstDocId,
        bgOtherDocId: salesOrderBgOtherDocId,
      ),
    ];
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
    final showSalesOrderNavigation = entity == 'sales_order';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(title),
        const SizedBox(height: 8),
        if (showSalesOrderNavigation) ...[
          Row(
            children: [
              Expanded(
                child: Text(
                  'Dieses Template steuert den PDF-/Druckpfad fuer Auftraege.',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
              const SizedBox(width: 12),
              OutlinedButton.icon(
                onPressed: _openSalesOrders,
                icon: const Icon(Icons.receipt_long_outlined),
                label: const Text('Auftraege oeffnen'),
              ),
            ],
          ),
          const SizedBox(height: 12),
        ],
        TextField(
          controller: headerCtrl,
          maxLines: 3,
          decoration: const InputDecoration(
            labelText: 'Kopftext',
            hintText: 'z. B. Firmenname, Adresse, Kontaktdaten',
          ),
        ),
        ..._buildPdfTemplateEffectiveHint(
          'Effektiver Kopftext',
          effectiveHeaderText,
        ),
        ..._buildPdfTemplateEffectiveHint(
          'Effektiver Brand-Name',
          effectiveDisplayName,
        ),
        ..._buildPdfTemplateEffectiveHint('Effektiver Claim', effectiveClaim),
        const SizedBox(height: 8),
        TextField(
          controller: footerCtrl,
          maxLines: 2,
          decoration: const InputDecoration(
            labelText: 'Fußtext',
            hintText: 'z. B. Bankdaten, USt-IdNr.',
          ),
        ),
        ..._buildPdfTemplateEffectiveHint(
          'Effektiver Fußtext',
          effectiveFooterText,
        ),
        ..._buildPdfTemplateEffectiveHint(
          'Effektive Primärfarbe',
          effectivePrimaryColor,
        ),
        ..._buildPdfTemplateEffectiveHint(
          'Effektive Akzentfarbe',
          effectiveAccentColor,
        ),
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
          _buildPdfTemplateImageSlot(
            entity: entity,
            label: 'Logo',
            kind: 'logo',
            docId: logoDocId,
          ),
          _buildPdfTemplateImageSlot(
            entity: entity,
            label: 'Hintergrund (Seite 1)',
            kind: 'bg-first',
            docId: bgFirstDocId,
          ),
          _buildPdfTemplateImageSlot(
            entity: entity,
            label: 'Hintergrund (Folge)',
            kind: 'bg-other',
            docId: bgOtherDocId,
          ),
        ]),
        const SizedBox(height: 12),
        _buildSectionSaveAction(
          onPressed: () => _savePdfTemplate(entity),
          icon: const Icon(Icons.save),
          label: const Text('Speichern'),
        ),
      ],
    );
  }

  Widget _buildPdfTemplateImageSlot({
    required String entity,
    required String label,
    required String kind,
    required String? docId,
  }) {
    return _imageRow(
      label,
      docId,
      onUpload: () => _pickAndUpload(entity, kind),
      onDelete: () => _deleteImage(entity, kind),
    );
  }

  List<Widget> _buildPdfTemplateEffectiveHint(String label, String value) {
    if (value.isEmpty) {
      return const [];
    }
    return [
      const SizedBox(height: 6),
      Text(
        '$label: $value',
        style: Theme.of(context).textTheme.bodySmall,
      ),
    ];
  }

  Widget _imageRow(String label, String? docId,
      {required VoidCallback onUpload, required VoidCallback onDelete}) {
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
          _buildImageTextAction(
            onPressed: () {
              widget.api.downloadDocument(docId, filename: 'preview');
            },
            icon: const Icon(Icons.visibility),
            label: const Text('Anzeigen'),
          ),
          const SizedBox(width: 8),
          _buildImageTextAction(
            onPressed: onDelete,
            icon: const Icon(Icons.delete),
            label: const Text('Entfernen'),
          ),
        ] else ...[
          const Text('— nicht gesetzt —'),
        ],
        const SizedBox(width: 8),
        _buildImageOutlinedAction(
          onPressed: onUpload,
          icon: const Icon(Icons.upload),
          label: const Text('Hochladen'),
        ),
      ],
    );
  }

  Widget _buildImageTextAction({
    required VoidCallback onPressed,
    required Widget icon,
    required Widget label,
  }) {
    return TextButton.icon(
      onPressed: onPressed,
      icon: icon,
      label: label,
    );
  }

  Widget _buildImageOutlinedAction({
    required VoidCallback onPressed,
    required Widget icon,
    required Widget label,
  }) {
    return OutlinedButton.icon(
      onPressed: onPressed,
      icon: icon,
      label: label,
    );
  }
}
