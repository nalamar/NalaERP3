import 'package:flutter/material.dart';
import '../api.dart';
import 'materialwirtschaft_screen.dart';
import 'contacts_screen.dart';
import 'settings_page.dart';
import 'projects_page.dart';

class DashboardPage extends StatelessWidget {
  const DashboardPage({super.key});

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme.primary;
    return Scaffold(
      appBar: AppBar(
        title: const Text('NalaERP3'),
        backgroundColor: color,
        foregroundColor: Colors.white,
      ),
      body: Container(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            colors: [color.withOpacity(0.08), Colors.white],
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
          ),
        ),
        alignment: Alignment.center,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 900),
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Wrap(
              alignment: WrapAlignment.center,
              runSpacing: 24,
              spacing: 24,
              children: [
                _DashCard(
                  title: 'Materialwirtschaft',
                  icon: Icons.inventory_2_rounded,
                  color: color,
                  onTap: () {
                    Navigator.of(context).push(
                      MaterialPageRoute(
                        builder: (_) => MaterialwirtschaftScreen(api: ApiClient()),
                      ),
                    );
                  },
                ),
                _DashCard(
                  title: 'Projekte',
                  icon: Icons.workspaces_rounded,
                  color: color,
                  onTap: () {
                    Navigator.of(context).push(
                      MaterialPageRoute(
                        builder: (_) => ProjectsPage(api: ApiClient()),
                      ),
                    );
                  },
                ),
                _DashCard(
                  title: 'Kontakte',
                  icon: Icons.people_alt_rounded,
                  color: color,
                  onTap: () {
                    Navigator.of(context).push(
                      MaterialPageRoute(
                        builder: (_) => ContactsScreen(api: ApiClient()),
                      ),
                    );
                  },
                ),
                _DashCard(
                  title: 'Einstellungen',
                  icon: Icons.settings_rounded,
                  color: color,
                  onTap: () {
                    Navigator.of(context).push(
                      MaterialPageRoute(
                        builder: (_) => SettingsPage(api: ApiClient()),
                      ),
                    );
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _DashCard extends StatelessWidget {
  const _DashCard({required this.title, required this.icon, required this.color, required this.onTap});
  final String title;
  final IconData icon;
  final Color color;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(16),
      child: Ink(
        width: 260,
        height: 160,
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(16),
          boxShadow: [BoxShadow(color: Colors.black12, blurRadius: 12, offset: const Offset(0,6))],
          border: Border.all(color: color.withOpacity(0.2)),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            CircleAvatar(backgroundColor: color.withOpacity(0.12), radius: 28, child: Icon(icon, color: color, size: 30)),
            const SizedBox(height: 12),
            Text(title, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
          ],
        ),
      ),
    );
  }
}
