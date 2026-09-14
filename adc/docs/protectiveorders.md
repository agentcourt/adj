# Protective Orders

ADC records judicial protective orders and their lifting in the case state, docket, and decision trace.  The [Lean core](../engine/ADC/Core.lean) implements `enter_protective_order` and `lift_protective_order`, and the [runtime tool schemas](../runtime/runner/tools.go) expose their arguments.

## Order Actions

Both actions require the judge role.  `enter_protective_order` accepts these fields:

| Field | Meaning |
| --- | --- |
| `scope` | Text describing the restriction's scope. |
| `target` | Text identifying the material or activity covered. |
| `allowed_roles` | Intended permitted roles.  The engine trims entries, removes empty entries, and requires at least one remaining entry. |
| `note` | Explanation.  The runtime tool schema requires the field, while the engine accepts omission as an empty string. |
| `order_id` | Optional identifier.  The engine generates an identifier when this field is absent or blank and rejects a duplicate. |

The engine stores the order with `active: true`.  `lift_protective_order` requires an existing nonblank `order_id`, accepts an optional `note`, and marks the order inactive.  Both actions append a docket entry and decision trace.  Their recorded entry and lifting dates use the case's `filed_on` value.

## Enforcement Limits

The runtime's file-access and role-view checks operate independently of `protective_orders`.  The stored `scope`, `target`, and `allowed_roles` express the order's terms.  Enforcing those terms against file reads, model inputs, retained sessions, published artifacts, or external processing remains unimplemented.  Confidentiality therefore depends on the existing case-file access rules and controls imposed by the operator.

The procedural references are [ARCP Rule 26](ARCP.md#rule-26-duty-to-disclose-general-provisions-governing-discovery) and [ARCP Rule 87](ARCP.md#rule-87-agent-assisted-litigation-orders).  The [implementation matrix](ARCP-matrix.md#priority-gaps-for-arcp-fidelity) identifies protective-order enforcement as an open requirement.  The [case-record publication reference](../../adjudication-cli.md#case-record-index) describes the access classes included in exported artifacts.
