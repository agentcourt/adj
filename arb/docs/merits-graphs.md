# Merits Graphs

A merits graph records the argumentative content of one arbitration run.  It separates documentary sources, source-backed facts, interpretive claims, complaint rules, and council vote rationales.  It preserves procedural order only where role or phase affects attribution.

The canonical evidence is JSON.  Mermaid is a projection of that JSON into one readable view.  One graph should support several views without changing the underlying propositions: an issue map, a source-to-fact map, and a vote-adoption map are the first useful ones.

## Schema

The root object should stay small.  It needs a schema version, a run identifier, a human title, the identifier of the proposition node, and the graph itself.  The graph then lives in two arrays: `nodes` and `edges`.

| Field | Meaning |
|---|---|
| `schema_version` | Schema identifier, for example `arb-merits-graph-v1` |
| `run_id` | Arbitration run identifier |
| `title` | Human label for the graph |
| `proposition_node_id` | Node id for the ultimate proposition |
| `nodes` | Normalized argumentative units |
| `edges` | Directed relations between nodes |

Each node represents one proposition or one documentary source.  A node is not a sentence, and it is not a filing block.  Repeated propositions across arguments, rebuttal, closing, and council votes should collapse into one node with multiple mentions.

| Node field | Meaning |
|---|---|
| `id` | Stable local identifier |
| `kind` | `proposition`, `issue`, `rule`, `source`, `fact`, `claim`, or `vote` |
| `label` | Short display label |
| `text` | One complete proposition or one source description |
| `mentions` | Optional array of packet-local attributions such as role, phase, member id, or evidence |
| `sources` | Optional array of source-node ids for fact nodes |
| `vote` | Optional final vote value for vote nodes |

Each edge states one argumentative relation.  The edge vocabulary should stay narrow.  Four relations are enough for a first version: `states`, `supports`, `attacks`, and `adopts`.

| Edge field | Meaning |
|---|---|
| `from` | Source node id |
| `to` | Target node id |
| `relation` | `states`, `supports`, `attacks`, or `adopts` |
| `note` | Optional short clarification when the relation would otherwise be ambiguous |

## Extraction Rules

The graph should normalize claims, not filing order.  If the plaintiff makes the same interpretive move in arguments, rebuttal, and closing, the graph should represent that move once and record three mentions.  If the later filing materially changes the claim, that change should produce a second node.

Fact nodes should stay literal and source-backed.  A fact such as “U.S. forces were on the ground for about two hours” belongs in a fact node because a source states it directly.  A proposition such as “temporary control of the site satisfies seize or hold territory” belongs in a claim node because it is an interpretive move rather than a source-stated fact.

Rule nodes should capture complaint criteria, carve-outs, fallback rules, and burden rules only when those rules affect the dispute.  They should not restate the whole complaint table.  Vote nodes should capture each council member’s stated rationale and final vote, then connect that vote to the claims or rules the member adopted.

The graph should not infer more than the packet supports.  If a filing gestures at a proposition but never states it cleanly, the graph should either omit that proposition or record a narrower one.  The graph is useful only if each node can be defended from the packet without interpretive embroidery.

## Placement

Merits-graph files belong in the run packet under `out/`, not in the example source directory.  The packet already contains the materials from which the graph is derived: `run.json`, `digest.md`, and `transcript.md`.  The derived files should sit beside those packet evidence as `out/<run>/merits-graph.json`, `out/<run>/merits-graph.mmd`, and, when rendered for inspection, `out/<run>/merits-graph.png`.

This placement follows from the graph's identity as a property of one run rather than the static example inputs.  A regenerated run can change the filings, the vote split, or the decisive claims.  Storing the graph under `examples/` makes it look like source material for the case rather than derived analysis of one packet.
