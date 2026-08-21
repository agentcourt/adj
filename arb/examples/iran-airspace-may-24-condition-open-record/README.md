# Iran airspace May 24 condition

This example runs an open-record arbitration of the factual condition behind Polymarket's May 24 market in the "Iran closes its airspace by...?" event.

It is intentionally a minimal upstream-style case. The checked-in input should be limited to the proposition and the market-rule record. The complaint is generated from `situation.md`, and `market-page.txt` is passed explicitly as the only initial case-packet file.

## Proposition

By May 24, 2026, 11:59 PM ET, Iran initiated a major non-weather closure of its airspace within the meaning of the market rule.

## Run

From `arb/`:

```bash
stamp="$(date -u +%Y%m%d-%H%M%S)"
batch="out/_batch-iran-airspace-may24-upstream-no-claw-$stamp"
out="out/iran-airspace-may24-upstream-no-claw-$stamp"
mkdir -p "$batch"

.bin/aar complain \
  --situation examples/iran-airspace-may-24-condition-open-record/situation.md \
  --out "$batch/complaint.md"

.bin/aar case \
  --complaint "$batch/complaint.md" \
  --out-dir "$out" \
  --attorney-model 'openai://gpt-5?tools=search' \
  --file examples/iran-airspace-may-24-condition-open-record/market-page.txt \
  --invalid-attempt-limit 5
```

The explicit `--file` argument prevents automatic directory scanning from admitting local notes or generated artifacts as initial evidence.
