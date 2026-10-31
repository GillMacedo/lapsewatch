# lapsewatch: find customers who stopped buying

[![npm](https://img.shields.io/npm/v/lapsewatch)](https://www.npmjs.com/package/lapsewatch) [![CI](https://github.com/GillMacedo/lapsewatch/actions/workflows/ci.yml/badge.svg)](https://github.com/GillMacedo/lapsewatch/actions/workflows/ci.yml) [![MIT license](https://img.shields.io/badge/license-MIT-blue)](LICENSE) [![MCP Registry](https://img.shields.io/badge/MCP_Registry-listed-green)](https://registry.modelcontextprotocol.io/v0/servers?search=io.github.GillMacedo/lapsewatch)

**Customer retention and churn analysis from an orders CSV: see who's at risk, who you've lost, and how much yearly revenue is in danger. For e-commerce stores, service businesses and small B2B companies.**

Keeping a customer usually costs less than finding one, but nobody notices when a regular quietly stops ordering. `lapsewatch` compares each customer with their own buying pattern, so someone who orders monthly is flagged sooner than someone who orders twice a year. It's an MCP server and a single Go binary, with no account, no API key and no network access.

## What you can ask your agent

- "Which customers have stopped buying?"
- "Who should I contact first to win back revenue?"
- "How much yearly revenue is at risk?"
- "What's my repeat customer rate, and how much do I depend on my top customers?"

## Install

As a Claude Code plugin, which adds the MCP server and a skill that tells Claude when to use it. You need Node, which runs the server through `npx`:

    claude plugin marketplace add GillMacedo/lapsewatch
    claude plugin install lapsewatch@lapsewatch-marketplace

For other MCP clients:

```json
{
  "mcpServers": {
    "lapsewatch": {
      "command": "npx",
      "args": ["-y", "lapsewatch", "serve"]
    }
  }
}
```

Or build the binary with Go 1.27 or newer:

    go install github.com/GillMacedo/lapsewatch@latest

## Usage

`lapsewatch cohorts orders.csv` prints monthly retention by first-order cohort. With `--as-of 2026-12-31`, it reports through that month; `--json` returns the same table as structured data. `M0` is the first-order month, and `M1`, `M2`, etc. are later calendar months. A cell is the percentage of customers in the original cohort with at least one positive order in that month; `—` marks months that have not elapsed yet. Same-day rows and refunds do not create extra returning customers.

Without installing anything, run `npx lapsewatch check orders.csv`. This example uses the made-up file in `examples/`:

    $ go run . check examples/orders.csv
    As of 2026-09-27: 8 customers, 75.0% bought more than once, 14598.00 revenue, top 10% bring in 49.3%
    AT RISK: contact these first (2)
      demo-bakery@example.com  197 days since last order, usually every 60  worth 5475.00 a year
      cafe@example.net  92 days since last order, usually every 30  worth 3771.67 a year
    LOST (1)
      erin@example.org  297 days since last order, usually every 20  worth 1368.75 a year
    ONE-TIME (1)
      finn@example.net  397 days since their only order  spent 60.00
    3 active, 1 new.
    SKIPPED row 68: no valid date, customer or amount
    9246.67 a year at risk from 2 customers.

Add `--as-of 2026-12-31` to measure from a specific date, `--top 50` to list more customers, and `--json` for machine-readable output.

## How it decides

| Status | Rule |
|---|---|
| **Active** | Ordered within twice their usual gap |
| **At risk** | Past twice their usual gap, and at least 30 days since their last order |
| **Lost** | Past four times their usual gap, and at least 30 days since their last order |
| **New** | One order in the last 90 days |
| **One-time** | One order, more than 90 days ago |

The usual gap is the median number of days between a customer's orders. Yearly value is their average order times 365 divided by that gap, so it's an estimate. Rows on the same day count as one order, and refunds reduce totals without counting as orders.

## MCP tools

| Tool | What it does |
|---|---|
| `customers_check_lapsed` | Returns the summary and customers by priority, with a limit for large files |
| `customers_inspect_csv` | Shows detected columns and row counts, without row contents |

Both are read-only, and neither can contact anyone. `customers_check_lapsed` returns customer names or emails to the MCP client, so they reach whichever AI model you use. The CLI keeps everything on your machine.

## FAQ

**How do I find inactive customers?** Export your orders as CSV and run `npx lapsewatch check orders.csv`.

**Does it work with Shopify, WooCommerce or Stripe exports?** It reads CSV and recognizes common column names such as `Email`, `Total` and `Created at`, and you can name others with `--columns date=When,customer=Who,amount=Sum`. It doesn't connect to those platforms, and it hasn't been tested on each one's real exports yet.

**Should I use names or emails?** Emails work best, because it treats each distinct value as one customer.

## Limits

- It finds who to look at, and can't tell why someone stopped. A one-off need or a seasonal buyer can look lapsed.
- A customer under two names or emails counts as two customers. [samecontact](https://github.com/GillMacedo/samecontact) can help clean a list first.
- It doesn't convert currencies.
- It reads CSV only, up to 50 MB.
- If you email customers from these lists, follow your local marketing and privacy rules.
- So far it has been tested on made-up data, not real exports. If yours fails, please open an issue with the column headers, not your data.

## Contributing

Contributions are welcome, from people and from people working with AI agents. See [CONTRIBUTING.md](CONTRIBUTING.md). Good first issues:

- Add column names from more platforms and languages
- Add `--out` to write a win-back list to a CSV
- Add seasonal buyer detection
- Add a cohort retention table
- Add a plugin icon

## AI disclosure

This project was largely generated with Claude Code and reviewed by its maintainer.

## About me

I'm Giulliano Macedo, and I build developer tools at GoTech Solutions. Follow along or get in touch:

- GitHub: [GillMacedo](https://github.com/GillMacedo)
- LinkedIn: [giulliano](https://www.linkedin.com/in/giulliano/)
- X: [@giumacedo](https://x.com/giumacedo)

If `lapsewatch` helped you win a customer back, a star helps others find it.

---

MIT licensed.

Built by [Giulliano Macedo](https://github.com/GillMacedo), GoTech Solutions · [LinkedIn](https://www.linkedin.com/in/giulliano/) · [X](https://x.com/giumacedo)
