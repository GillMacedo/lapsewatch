---
name: finding-lapsed-customers
description: Finds at-risk and lost customers in an orders CSV by comparing each customer with their own buying pattern, with revenue at risk and repeat rate. Use when the user asks about customer retention, churn, inactive customers, win-back lists, repeat purchase rate or customer concentration.
---

# Finding lapsed customers

Call `customers_check_lapsed` instead of working from the rows yourself, because each customer needs their own pattern. If it can't find columns, the error says how to name them, and `customers_inspect_csv` shows what it detected. If dates are ambiguous, ask whether they're day-first or month-first.

## Reporting results
- Lead with the yearly value at risk, then at-risk customers by value.
- Mention lost customers, the repeat rate, and revenue concentration if it's high.
- Report `skipped_rows`.

Yearly values are estimates, and a seasonal buyer can look lapsed, so present these as customers to check. Results contain customer details, so don't send them anywhere else. Don't contact customers without approval.
