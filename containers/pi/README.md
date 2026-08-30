# Pi Container Image

`containers/pi/` builds the local Podman image used by the ADC, ARB, AARD, and unified Quick launchers.  The image installs [Pi 0.84.3](https://www.npmjs.com/package/@earendil-works/pi-coding-agent/v/0.84.3), Pi MCP Adapter 2.27.0, and Pi Web Access 0.24.2, and uses `agentcourt-pi-sandbox` as its default name.  Each launcher starts Pi containers directly, mounts a private agent home directory, writes the applicable Pi and extension configuration there, and supplies the current role instructions.

## Files

| Path | Purpose |
| --- | --- |
| `Dockerfile` | Builds a local image with upstream Pi, the pinned Pi MCP adapter, pinned Pi Web Access, and runtime dependencies. |
| `build-image.sh` | Runs `podman build` for the local image. |

## Build

Run `./build-image.sh` from `containers/pi/`.  From the repository root, run `containers/pi/build-image.sh`.  Set `PI_CONTAINER_IMAGE` to override the default tag.

```bash
./build-image.sh
PI_CONTAINER_IMAGE=my-pi-agent ./build-image.sh
```

## Runtime Use

The four launcher forms use the image for Pi lawyers, jurors, or council members.  Each agent receives a private `/home/user` mount containing its settings, MCP server configuration, model request, and role instructions.  The launcher selects another image when its `--pi-image` option is set.

| Runtime | Agent role | Case adapter | Search extension |
| --- | --- | --- | --- |
| `adc-run` | Lawyers and jurors | `/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter` | Pi Web Access for lawyers when enabled |
| `aar-run` | Lawyers and council members | `/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter` | Pi Web Access for lawyers when enabled |
| `aard-run` | Lawyers and council members | `/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter` | Pi Web Access for lawyers when enabled |
| `adjudicate --proc quick` | Plaintiff and defendant lawyers | `/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter` | `/opt/pi-extensions/pi-web-access/node_modules/pi-web-access/index.ts` when enabled |

Pi model inference uses the provider authentication selected by its pool entry or agent profile.  Current council and juror pool records use OpenRouter and therefore require `OPENROUTER_API_KEY`.  An OpenAI lawyer profile can use Codex subscription authentication from `~/.codex/auth.json`, while an API-key profile names its source environment variable.  The records and profiles also supply the model, endpoint constraints, request parameters, and persona path.

## Lawyer Web Search

Quick, AAR, AARD, and ADC enable lawyer search by default through their procedure settings.  For a Pi lawyer, the launcher loads the exact Pi Web Access entry point and preserves extension and skill discovery from the retained Pi state.  An explicit false setting omits Pi Web Access and records `enabled: false` for that participant.

The generated `searchRouting.providers` value is `["openai", "exa"]`.  Pi Web Access uses existing OpenAI authentication when available and advances to its key-free Exa MCP provider after transient, quota, network, or invalid-response failures.  Quick therefore requires no search-specific credential, and the participant record retains the ordered provider list beside `mechanism: "pi_web_access"`.

The Pi lawyer command sets `defaultTools` to `read`, `bash`, `edit`, `write`, `grep`, `find`, and `ls`, and loads the case `mcp` proxy for every assignment.  Search-enabled assignments also load `web_search`, `source_check`, `fetch_content`, `get_search_content`, image handling, GitHub cloning, YouTube, video, and PDF analysis with workflow `none` and automatic browser opening disabled.  The launcher removes `EXA_API_KEY`, `PI_ALLOW_BROWSER_COOKIES`, and `FEYNMAN_ALLOW_BROWSER_COOKIES` from the child environment.

Pi runs as container root under rootless Podman so a lawyer can install a needed program during a turn.  The retained Pi home and case workspace preserve user-installed Pi packages, skills, downloads, programs, and analysis files across later invocations, while a system package installed into the temporary container filesystem lasts for that invocation.  Canonical evidence appears through a separate read-only mount at `/home/user/evidence`.

[Pi Web Access](https://github.com/nicobailon/pi-web-access) is a third-party Pi extension and executes with the authority of the Pi process.  The Dockerfile pins its [0.24.2 npm release](https://www.npmjs.com/package/pi-web-access/v/0.24.2) by tarball URL and SHA-512 digest and disables lifecycle scripts during installation.  Pi's [package security guidance](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/README.md#pi-packages) describes the authority granted to an installed extension.
