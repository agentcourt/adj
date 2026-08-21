# Ukraine Mineral Deal Polymarket Dispute

This example input directory is based on the disputed Polymarket resolution of:

https://polymarket.com/event/ukraine-agrees-to-give-trump-rare-earth-metals-before-april

The market asked whether Ukraine would agree to Trump's mineral deal before April 2025. The Polymarket page shows a final Yes outcome after two disputed Yes proposals. The dispute is useful for Agent Arbitration because it is a compact contract-interpretation and source-rule case:

- the written deadline was March 31, 2025, 11:59 PM ET;
- the rule required a U.S.-Ukraine agreement explicitly involving Ukrainian rare earth elements;
- the resolution source was official information from the governments of the United States and Ukraine;
- contemporary reporting said the market resolved Yes despite no official agreement having been reached;
- later official U.S. source material identifies April 30, 2025 as the date of the agreement.

The directory follows the `examples/ex03` pattern: `situation.md` states the proposition, and the remaining `.txt` files provide the starting record. `arbitrate.sh` will generate `complaint.md` from `situation.md` before running a case.

Suggested run command from `arb/`, if a run is authorized:

```bash
./arbitrate.sh examples/ukraine-mineral-deal out/ukraine-mineral-deal-r1 'openai://gpt-5?tools=search'
```

Search is recommended for the first run because the local packet is enough to frame the dispute but does not yet include the UMA voting record, primary Discord clarification, or direct Ukrainian government source excerpt.
