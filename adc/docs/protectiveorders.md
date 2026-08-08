# Protective Orders

Support for protective orders is in development.  The goal is to give courts and parties practical controls for handling confidential and restricted material in agent-assisted litigation, with audit-ready evidence about compliance.

The legal basis in this project is ordinary procedural authority.  Discovery control flows through [ARCP Rule 26](ARCP.md#rule-26-duty-to-disclose-general-provisions-governing-discovery), and case-specific agent-use controls flow through [ARCP Rule 87](ARCP.md#rule-87-agent-assisted-litigation-orders).

The implementation direction is two-part.  First, protective-order terms should be represented as explicit, enforceable constraints in case process.  Second, execution evidence should show whether those constraints were followed in practice.

Trusted execution environments (TEEs) are part of that second track.  TEE-backed attestations can provide verifiable evidence about where sensitive material was processed, what code processed it, and whether the execution environment changed during handling.

This area remains active work.  Current focus is on narrowing the gap between protective-order text and verifiable operational proof.
