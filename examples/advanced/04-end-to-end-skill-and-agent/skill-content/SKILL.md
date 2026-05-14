---
name: financial-report-builder
description: Generate quarterly financial summaries from raw transaction CSVs. Use when the user asks for quarterly reports, revenue breakdowns, or expense categorization.
---

# Financial Report Builder

Procedures and templates for turning raw transaction data into a
quarterly financial summary.

## Inputs

Expect a CSV with at least the columns:

- `date` (ISO 8601)
- `amount` (decimal, signed; negative = expense)
- `category` (string)
- `description` (string, optional)

## Output

A markdown report containing:

1. Revenue summary (sum of positive amounts grouped by category).
2. Expense summary (sum of absolute negative amounts grouped by category).
3. Net cash flow.
4. Top 5 individual transactions by absolute amount.

See `template.md` for the canonical output structure.
