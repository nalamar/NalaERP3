import 'dart:ui';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import '../api.dart';
import 'materialwirtschaft_screen.dart';
import 'contacts_screen.dart';
import 'settings_page.dart';
import 'projects_page.dart';
import 'invoices_page.dart';
import 'bank_statements_page.dart';
import 'employees_page.dart';

class DashboardPage extends StatelessWidget {
  const DashboardPage({super.key});

  @override
  Widget build(BuildContext context) {
    final color = Theme.of(context).colorScheme;
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _Header(color: color),
              const SizedBox(height: 22),
              Expanded(
                child: Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 1200),
                    child: Wrap(
                      alignment: WrapAlignment.start,
                      runSpacing: 22,
                      spacing: 22,
                      children: [
                        _DashCard(
                          title: 'Materialwirtschaft',
                          subtitle: 'Bestände & Warenflüsse',
                          icon: Icons.inventory_2_rounded,
                          color: color.primary,
                          onTap: () {
                            Navigator.of(context).push(
                              MaterialPageRoute(
                                builder: (_) =>
                                    MaterialwirtschaftScreen(api: ApiClient()),
                              ),
                            );
                          },
                        ),
                        _DashCard(
                          title: 'Projekte',
                          subtitle: 'Prozesse & Aufträge',
                          icon: Icons.workspaces_rounded,
                          color: color.primary,
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
                          subtitle: 'CRM & Stammdaten',
                          icon: Icons.people_alt_rounded,
                          color: color.primary,
                          onTap: () {
                            Navigator.of(context).push(
                              MaterialPageRoute(
                                builder: (_) =>
                                    ContactsScreen(api: ApiClient()),
                              ),
                            );
                          },
                        ),
                        _DashCard(
                          title: 'Finanzen',
                          subtitle: 'AR-Rechnungen & Zahlungseingänge',
                          icon: Icons.receipt_long_rounded,
                          color: color.primary,
                          onTap: () {
                            Navigator.of(context).push(
                              MaterialPageRoute(
                                builder: (_) => InvoicesPage(api: ApiClient()),
                              ),
                            );
                          },
                        ),
                        _DashCard(
                          title: 'Bank',
                          subtitle: 'OPOS & Kontoabgleich',
                          icon: Icons.account_balance_rounded,
                          color: color.primary,
                          onTap: () {
                            Navigator.of(context).push(
                              MaterialPageRoute(
                                builder: (_) =>
                                    BankStatementsPage(api: ApiClient()),
                              ),
                            );
                          },
                        ),
                        _DashCard(
                          title: 'Personal',
                          subtitle: 'Teams & Verfügbarkeit',
                          icon: Icons.badge_rounded,
                          color: color.primary,
                          onTap: () {
                            Navigator.of(context).push(
                              MaterialPageRoute(
                                builder: (_) => EmployeesPage(api: ApiClient()),
                              ),
                            );
                          },
                        ),
                        _DashCard(
                          title: 'Einstellungen',
                          subtitle: 'Konten, Steuern, Rollen',
                          icon: Icons.settings_rounded,
                          color: color.primary,
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
            ],
          ),
        ),
      ),
    );
  }
}

class _Header extends StatelessWidget {
  const _Header({required this.color});
  final ColorScheme color;

  @override
  Widget build(BuildContext context) {
    return ClipRRect(
      borderRadius: BorderRadius.circular(18),
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: 14, sigmaY: 14),
        child: Container(
          padding: const EdgeInsets.all(18),
          decoration: BoxDecoration(
            gradient: LinearGradient(
              colors: [
                Colors.white.withValues(alpha: 0.12),
                Colors.white.withValues(alpha: 0.05)
              ],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
            borderRadius: BorderRadius.circular(18),
            border: Border.all(color: Colors.white.withValues(alpha: 0.12)),
            boxShadow: [
              BoxShadow(
                  color: Colors.black.withValues(alpha: 0.25),
                  blurRadius: 24,
                  offset: const Offset(0, 18)),
              BoxShadow(
                  color: color.primary.withValues(alpha: 0.18),
                  blurRadius: 28,
                  offset: const Offset(-10, -6))
            ],
          ),
          child: Row(
            children: [
              Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                decoration: BoxDecoration(
                  color: color.primary.withValues(alpha: 0.14),
                  borderRadius: BorderRadius.circular(12),
                  border:
                      Border.all(color: color.primary.withValues(alpha: 0.24)),
                ),
                child: Row(
                  children: [
                    Icon(Icons.auto_awesome, color: color.primary),
                    const SizedBox(width: 8),
                    Text('NalaERP | modern workspace',
                        style: TextStyle(
                            color: Colors.white.withValues(alpha: 0.86),
                            fontWeight: FontWeight.w700)),
                  ],
                ),
              ),
              const Spacer(),
              Row(
                children: [
                  Icon(Icons.cloud_done_rounded,
                      color: Colors.white.withValues(alpha: 0.7)),
                  const SizedBox(width: 6),
                  Text('Bereit',
                      style: TextStyle(
                          color: Colors.white.withValues(alpha: 0.8),
                          fontWeight: FontWeight.w600)),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _DashCard extends StatefulWidget {
  const _DashCard(
      {required this.title,
      required this.subtitle,
      required this.icon,
      required this.color,
      required this.onTap});
  final String title;
  final String subtitle;
  final IconData icon;
  final Color color;
  final VoidCallback onTap;

  @override
  State<_DashCard> createState() => _DashCardState();
}

class _DashCardState extends State<_DashCard> {
  bool hovering = false;
  double tiltX = 0;
  double tiltY = 0;
  final GlobalKey _key = GlobalKey();

  void _updateTilt(PointerHoverEvent e) {
    final box = _key.currentContext?.findRenderObject() as RenderBox?;
    if (box == null) return;
    final size = box.size;
    final dx = (e.localPosition.dx / size.width) - 0.5;
    final dy = (e.localPosition.dy / size.height) - 0.5;
    setState(() {
      tiltY = dx * 0.18;
      tiltX = -dy * 0.18;
    });
  }

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      onEnter: (_) => setState(() => hovering = true),
      onExit: (_) => setState(() {
        hovering = false;
        tiltX = 0;
        tiltY = 0;
      }),
      onHover: _updateTilt,
      child: GestureDetector(
        onTap: widget.onTap,
        child: AnimatedContainer(
          key: _key,
          duration: const Duration(milliseconds: 240),
          curve: Curves.easeOut,
          width: 280,
          height: 170,
          transform: Matrix4.identity()
            ..setEntry(3, 2, 0.0012)
            ..rotateX(tiltX)
            ..rotateY(tiltY)
            ..setTranslationRaw(0.0, hovering ? -4.0 : 0.0, 0.0),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(18),
            gradient: LinearGradient(
              colors: [
                Colors.white.withValues(alpha: 0.16),
                Colors.white.withValues(alpha: 0.05)
              ],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
            border: Border.all(color: Colors.white.withValues(alpha: 0.14)),
            boxShadow: [
              BoxShadow(
                  color: Colors.black.withValues(alpha: 0.25),
                  blurRadius: 22,
                  spreadRadius: 2,
                  offset: const Offset(0, 18)),
              BoxShadow(
                  color: widget.color.withValues(alpha: 0.18),
                  blurRadius: 32,
                  offset: const Offset(-10, -10)),
            ],
          ),
          child: Stack(
            children: [
              Positioned(
                top: 16,
                right: 16,
                child: Container(
                  width: 42,
                  height: 42,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    gradient: RadialGradient(
                      colors: [
                        widget.color.withValues(alpha: 0.7),
                        widget.color.withValues(alpha: 0.25)
                      ],
                    ),
                    boxShadow: [
                      BoxShadow(
                          color: widget.color.withValues(alpha: 0.35),
                          blurRadius: 18,
                          spreadRadius: 2)
                    ],
                  ),
                  child: Icon(widget.icon, color: Colors.black87, size: 24),
                ),
              ),
              Padding(
                padding:
                    const EdgeInsets.symmetric(horizontal: 16, vertical: 18),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(widget.title,
                            style: const TextStyle(
                                fontSize: 20,
                                color: Colors.white,
                                fontWeight: FontWeight.w700)),
                        const SizedBox(height: 6),
                        Text(widget.subtitle,
                            style: TextStyle(
                                color: Colors.white.withValues(alpha: 0.75))),
                      ],
                    ),
                    Row(
                      children: [
                        Icon(Icons.arrow_forward_rounded,
                            color: Colors.white.withValues(alpha: 0.8),
                            size: 20),
                        const SizedBox(width: 6),
                        Text('Loslegen',
                            style: TextStyle(
                                color: Colors.white.withValues(alpha: 0.9),
                                fontWeight: FontWeight.w600)),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
