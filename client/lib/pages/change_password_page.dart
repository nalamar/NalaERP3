import 'package:flutter/material.dart';
import '../api.dart';

// Passwort aendern (Backlog 0.49.2), Route aus 0.49.1. Zwei Verwendungen:
//   - erzwungen (forced): nach der Anmeldung, solange der Server
//     must_change_password meldet - etwa beim ersten Admin "Admin"/"Admin".
//     Bis zum Wechsel sperrt der Server alle anderen Routen; die Seite
//     bietet nur Wechsel oder Abmelden.
//   - freiwillig: aus dem Dashboard; nach Erfolg schliesst die Seite.

const minPasswordLength = 10;

// mustChangePassword liest das Kennzeichen aus /auth/me bzw. dem Login.
bool mustChangePassword(Map<String, dynamic>? identity) {
  final user = identity?['user'];
  return user is Map && user['must_change_password'] == true;
}

// validatePasswordChange prueft die Eingaben wie der Server und dazu die
// Wiederholung; null = in Ordnung.
String? validatePasswordChange({required String current, required String next, required String repeat}) {
  if (current.isEmpty) return 'Bitte das aktuelle Passwort angeben.';
  if (next.trim().length < minPasswordLength) return 'Das neue Passwort muss mindestens $minPasswordLength Zeichen lang sein.';
  if (next != repeat) return 'Die Wiederholung stimmt nicht mit dem neuen Passwort überein.';
  if (next.trim() == current.trim()) return 'Das neue Passwort muss sich vom bisherigen unterscheiden.';
  return null;
}

class ChangePasswordPage extends StatefulWidget {
  const ChangePasswordPage({super.key, required this.api, this.forced = false, this.onChanged, this.onLogout});
  final ApiClient api;
  final bool forced;
  // Nach erfolgreichem Wechsel (erzwungen: Identitaet neu laden).
  final Future<void> Function()? onChanged;
  final Future<void> Function()? onLogout;

  @override
  State<ChangePasswordPage> createState() => _ChangePasswordPageState();
}

class _ChangePasswordPageState extends State<ChangePasswordPage> {
  final currentCtrl = TextEditingController();
  final nextCtrl = TextEditingController();
  final repeatCtrl = TextEditingController();
  String? error;
  bool busy = false;

  @override
  void dispose() {
    currentCtrl.dispose();
    nextCtrl.dispose();
    repeatCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final problem = validatePasswordChange(current: currentCtrl.text, next: nextCtrl.text, repeat: repeatCtrl.text);
    if (problem != null) {
      setState(() => error = problem);
      return;
    }
    setState(() {
      busy = true;
      error = null;
    });
    try {
      await widget.api.changePassword(currentCtrl.text, nextCtrl.text);
      if (!mounted) return;
      if (widget.forced) {
        await widget.onChanged?.call();
        return;
      }
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Passwort geändert')));
      await widget.onChanged?.call();
      if (mounted) Navigator.of(context).pop(true);
    } on ApiException catch (e) {
      if (mounted) setState(() => error = e.message);
    } catch (e) {
      if (mounted) setState(() => error = '$e');
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Passwort ändern'),
        actions: [
          if (widget.forced && widget.onLogout != null)
            TextButton(onPressed: () => widget.onLogout!(), child: const Text('Abmelden')),
        ],
      ),
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 440),
            child: ListView(
              shrinkWrap: true,
              padding: const EdgeInsets.all(24),
              children: [
                if (widget.forced)
                  const Padding(
                    padding: EdgeInsets.only(bottom: 12),
                    child: Text('Bevor Sie weiterarbeiten können, müssen Sie Ihr Passwort ändern.',
                        style: TextStyle(fontWeight: FontWeight.bold)),
                  ),
                TextField(controller: currentCtrl, obscureText: true, decoration: const InputDecoration(labelText: 'Aktuelles Passwort')),
                TextField(
                  controller: nextCtrl,
                  obscureText: true,
                  decoration: const InputDecoration(labelText: 'Neues Passwort (mindestens $minPasswordLength Zeichen)'),
                ),
                TextField(
                  controller: repeatCtrl,
                  obscureText: true,
                  decoration: const InputDecoration(labelText: 'Neues Passwort wiederholen'),
                  onSubmitted: (_) => _submit(),
                ),
                if (error != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 12),
                    child: Text(error!, style: const TextStyle(color: Color(0xFFB00020))),
                  ),
                const SizedBox(height: 16),
                FilledButton(onPressed: busy ? null : _submit, child: const Text('Passwort ändern')),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
