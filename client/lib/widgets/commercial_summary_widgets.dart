import 'package:flutter/material.dart';

class CommercialSummaryText extends StatelessWidget {
  const CommercialSummaryText({
    super.key,
    required this.lines,
    this.textAlign = TextAlign.center,
    this.style,
  });

  final List<String> lines;
  final TextAlign textAlign;
  final TextStyle? style;

  @override
  Widget build(BuildContext context) {
    return Text(
      lines.join('\n'),
      textAlign: textAlign,
      style: style,
    );
  }
}

class CommercialSummaryCard extends StatelessWidget {
  const CommercialSummaryCard({
    super.key,
    required this.headline,
    required this.lines,
    this.footer,
  });

  final String headline;
  final List<String> lines;
  final Widget? footer;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              headline,
              style: theme.textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: 8),
            CommercialSummaryText(
              lines: lines,
              textAlign: TextAlign.left,
            ),
            if (footer != null) footer!,
          ],
        ),
      ),
    );
  }
}
