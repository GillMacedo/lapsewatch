# 0001: Judge each customer against their own pattern

**Decision:** statuses come from multiples of a customer's median order gap, with a 30-day minimum.

**Why:** a fixed cut-off such as 90 days flags quarterly buyers too soon and notices weekly buyers far too late.

**Trade-off:** customers with one order have no pattern, so they get separate new and one-time statuses.

**Alternatives:** standard RFM scores, which rank customers against each other and don't say when one person has changed.
