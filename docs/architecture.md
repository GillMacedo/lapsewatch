# Architecture

    orders.csv ─> load ─> per-customer history ─> status ─> summary ─> CLI and MCP tools

**Loading:** finds the header in the first 20 rows. Amounts become integer cents. Unreadable rows are reported.

**History:** one customer per distinct value, ignoring case and spaces. Rows on the same day are one order. Refunds reduce totals but aren't orders.

**Status:** see the README. Each customer's gap is the median between their orders, so one long break doesn't distort it.

**Priority:** at risk first, by yearly value, because those are the customers it's still realistic to keep.

**Packaging:** one npm package holds all five binaries, and `npm/bin.js` picks the right one.

See `docs/decisions/`.
