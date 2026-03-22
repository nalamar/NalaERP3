class SalesOrderCommercialStats {
  const SalesOrderCommercialStats({
    this.followUpCount = 0,
    this.partialCount = 0,
    this.remainingGross = 0,
  });

  final int followUpCount;
  final int partialCount;
  final double remainingGross;
}

class ProjectCommercialStats {
  const ProjectCommercialStats({
    this.quoteCount = 0,
    this.quotesWithFollowUp = 0,
    this.salesOrderCount = 0,
    this.partialSalesOrderCount = 0,
    this.remainingGrossAmount = 0,
  });

  final int quoteCount;
  final int quotesWithFollowUp;
  final int salesOrderCount;
  final int partialSalesOrderCount;
  final double remainingGrossAmount;
}

SalesOrderCommercialStats summarizeSalesOrders(List<dynamic> salesOrders) {
  var partialCount = 0;
  var remainingGross = 0.0;
  var followUpCount = 0;

  for (final entry in salesOrders) {
    final item = (entry as Map).cast<String, dynamic>();
    final invoiceCount =
        toCommercialDouble(item['related_invoice_count']).round();
    final remaining = toCommercialDouble(item['remaining_gross_amount']);
    if (invoiceCount > 0) {
      followUpCount += 1;
    }
    if (invoiceCount > 0 && remaining > 0.0001) {
      partialCount += 1;
      remainingGross += remaining;
    }
  }

  return SalesOrderCommercialStats(
    followUpCount: followUpCount,
    partialCount: partialCount,
    remainingGross: remainingGross,
  );
}

ProjectCommercialStats summarizeProjectCommercialContext(
  List<dynamic> quotes,
  List<dynamic> salesOrders,
) {
  var quotesWithFollowUp = 0;
  for (final entry in quotes) {
    final item = (entry as Map).cast<String, dynamic>();
    if ((item['linked_sales_order_id'] ?? '').toString().trim().isNotEmpty ||
        (item['linked_invoice_out_id'] ?? '').toString().trim().isNotEmpty) {
      quotesWithFollowUp += 1;
    }
  }

  final salesOrderStats = summarizeSalesOrders(salesOrders);

  return ProjectCommercialStats(
    quoteCount: quotes.length,
    quotesWithFollowUp: quotesWithFollowUp,
    salesOrderCount: salesOrders.length,
    partialSalesOrderCount: salesOrderStats.partialCount,
    remainingGrossAmount: salesOrderStats.remainingGross,
  );
}

double toCommercialDouble(dynamic value) {
  if (value is num) return value.toDouble();
  return double.tryParse(value?.toString() ?? '') ?? 0;
}
