# Ukraine Mineral Deal Polymarket Dispute — Clear Polarity Version

This example is a clearer restatement of `examples/ukraine-mineral-deal`.

It uses the same evidence packet and the same underlying Polymarket dispute, but it avoids the double-negative framing that produced vote/rationale mismatches in repeated trial runs.

## Decision Question

Did official U.S. and Ukrainian government information show, by March 31, 2025, 11:59 PM ET, that the United States and Ukraine had agreed to any deal explicitly involving Ukrainian rare earth elements?

## Proposition Used for Arbitration

The Polymarket market should have resolved `No`.

By March 31, 2025, 11:59 PM ET, official U.S. and Ukrainian government information did not show that the United States and Ukraine had agreed to any deal explicitly involving Ukrainian rare earth elements.

## Intended Vote Mapping

- `demonstrated` means the market should have resolved `No`.
- `not_demonstrated` means the record does not establish that the market should have resolved `No`; the final `Yes` resolution may stand.

## Evidence Packet

The copied `.txt` files preserve the same starting record as the original example:

- `market-page.txt` — Polymarket market text, deadline, source rule, and final Yes sequence.
- `official-source-record.txt` — captured official-source record, including the later White House agreement date and Ukrainian government source gap.
- `reporting-coindesk.txt` — CoinDesk report on the disputed resolution.
- `reporting-defiant.txt` — The Defiant report on alleged UMA/oracle issues.
- `reporting-web3isgoinggreat.txt` — Web3 Is Going Great summary.
- `unresolved-record-gaps.txt` — primary-source gaps still worth filling.

## Suggested Run Command

From `arb/`:

```bash
./arbitrate.sh examples/ukraine-mineral-deal-clear out/ukraine-mineral-deal-clear-r1 'openai://gpt-5?tools=search'
```

Use a unique output directory for every trial. `arbitrate.sh` removes the selected output directory before running.
