---
id: FEAT-005
type: feature
title: Bundle Claude Code/Codex/Gemini CLI into agent sandbox image
status: done
complexity: simple
assignee: sdlc-developer
sprint: SPRINT-001
created: 2026-07-04
history:
  - {date: 2026-07-04, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-05, stage: in-development, by: architect, note: "FEAT-004 already handled the CMD change (tail -f /dev/null); assigned to sdlc-developer for the CLI bundling"}
  - {date: 2026-07-05, stage: in-review, by: architect, note: "moved to review"}
  - {date: 2026-07-05, stage: in-test, by: architect, note: "review PASSED (package names verified against npm registry); moved to test"}
  - {date: 2026-07-05, stage: done, by: architect, note: "test PASSED (independent rebuild + all 4 CLIs verified); moved to done"}
---

## Summary
`deploy/Dockerfile.agent` currently only installs `opencode-ai`.
architecture.md's sandbox image is meant to bundle every supported agent CLI
so the user can pick which one to run inside the terminal, with no
per-session install step. Add Claude Code, Codex CLI, and Gemini CLI to the
image alongside the existing OpenCode install.

## Requirements
- Add `@anthropic-ai/claude-code` to the image
- Add Codex CLI to the image
- Add Gemini CLI to the image
- OpenCode stays installed (already present)
- Image no longer runs `agent-server`'s Node.js process as its CMD — since
  FEAT-004 replaces exec/attach-based interaction, the image just needs to
  stay alive for `docker exec` to work (e.g. `tail -f /dev/null` or
  `sleep infinity`); coordinate with FEAT-004 on the exact CMD, but this
  task should make the Dockerfile change since it owns the image

## Out of Scope
- The WebSocket terminal / exec plumbing itself — see FEAT-004
- Any per-CLI configuration UI — user runs CLIs manually in the terminal

## Proposed Solution / Approach
Verified via `npm view` that all three CLIs are published as global npm
packages with the expected names: `@anthropic-ai/claude-code` (bin `claude`),
`@openai/codex` (bin `codex`), and `@google/gemini-cli` (bin `gemini`). Since
the base image is already `node:22-alpine` with `opencode-ai` installed via
`npm i -g`, the simplest KISS-compliant approach is to add the three new
packages to the same `npm i -g` layer (single RUN, one npm resolution pass,
no extra layers/base image/package manager needed). The CMD change FEAT-004
already made stays as-is.

## Affected Areas
- `deploy/Dockerfile.agent`

## Acceptance Criteria / Definition of Done
- [ ] Building the image succeeds and produces a container with `claude`, `codex`, `gemini`, and `opencode` all on PATH
- [ ] Image size increase is reasonable (no duplicated base layers, use judgment)
- [ ] Container built from this image stays running without the old `agent-server` CMD
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
`docker build -f deploy/Dockerfile.agent .`, run the resulting image, exec
into it and confirm `claude --version`, `codex --version`, `gemini
--version`, `opencode --version` (or equivalents) all succeed.

## Implementation Notes
Changed `deploy/Dockerfile.agent`'s single `npm i -g` line to install all four
CLIs together: `opencode-ai@latest @anthropic-ai/claude-code@latest
@openai/codex@latest @google/gemini-cli@latest`. No other changes were
needed — FEAT-004 already replaced the CMD with `tail -f /dev/null`.

Verified by building the image (`docker build -f deploy/Dockerfile.agent -t
tamga-agent-test .`) and running each `--version` check plus `which` inside a
running container:
- `claude --version` -> `2.1.201 (Claude Code)`
- `codex --version` -> `codex-cli 0.142.5`
- `gemini --version` -> `0.49.0`
- `opencode --version` -> `1.17.13`
All four resolve to `/usr/local/bin/<name>` on PATH, and the container stays
up (`tail -f /dev/null`) allowing `docker exec` to work. Resulting image is
~3.1GB, all installed in a single RUN layer (no duplicated base layers).

Note: bridge networking in the sandboxed dev environment used for this build
couldn't create veth pairs (`operation not supported`), unrelated to the
Dockerfile itself; builds/runs were done with `--network host` to work around
that local environment limitation.

## Review Notes
<Filled in by the reviewer.>

## Test Notes
<Filled in by the tester.>

### 2026-07-05 — reviewer

Verdict: PASS

Checked:
- Diff is exactly as described: `deploy/Dockerfile.agent`'s single `RUN npm i -g` line extended from `opencode-ai@latest` to `opencode-ai@latest @anthropic-ai/claude-code@latest @openai/codex@latest @google/gemini-cli@latest`. Nothing else in the file changed (confirmed via `git diff HEAD -- deploy/Dockerfile.agent`).
- Package names spot-checked against the live npm registry: `@anthropic-ai/claude-code` (bin `claude`), `@openai/codex` (bin `codex`), `@google/gemini-cli` (bin `gemini`), `opencode-ai` (bin `opencode`) — all four names, bins, and `latest` versions match. Notably the registry's current `latest` versions (2.1.201 / 0.142.5 / 0.49.0 / 1.17.13) match the exact `--version` output the developer reported, corroborating that the build/verification was actually performed.
- No duplicated layers or extra base images/package managers — all four CLIs installed in the same single `RUN npm i -g` layer as before, consistent with the file's existing convention and KISS.
- CMD requirement: `CMD ["tail", "-f", "/dev/null"]` (from FEAT-004) is untouched and still present; WORKDIR and apk layer also untouched.
- Acceptance criteria walkthrough:
  - [x] Build produces container with all four CLIs on PATH — verified by developer's `--version`/`which` output, plausible and consistent with registry data.
  - [x] Image size increase reasonable, no duplicated base layers — single RUN layer, ~3.1GB reported, judgment call accepted for four full CLI toolchains.
  - [x] Container stays running without old `agent-server` CMD — CMD line confirmed intact from FEAT-004.
  - [x] KISS/YAGNI — minimal one-line change, no speculative abstraction.

No blocking issues. Non-blocking/minor: the note about `--network host` being needed in the sandboxed dev environment is an environment quirk unrelated to the Dockerfile and doesn't affect correctness of the change.

### 2026-07-05 — tester

Verdict: PASS

Independently rebuilt and exercised the image from scratch (own tag/container, not reusing developer/reviewer artifacts):

- Build: `docker build --network host -f deploy/Dockerfile.agent -t tamga-agent-qa-test .` — succeeded (bridge networking failed with veth creation errors in this sandboxed environment, same known limitation noted by prior agents; `--network host` worked around it). Build log shows the single `RUN npm i -g opencode-ai@latest @anthropic-ai/claude-code@latest @openai/codex@latest @google/gemini-cli@latest` layer completing cleanly ("added 14 packages in 1m"), no errors.
- Ran container: `docker run -d --network host --name tamga-agent-qa-container tamga-agent-qa-test`, confirmed `docker ps` shows `Up`.
- Exec'd each version check directly against the running container (not just at build time):
  - `docker exec tamga-agent-qa-container claude --version` -> `2.1.201 (Claude Code)`
  - `docker exec tamga-agent-qa-container codex --version` -> `codex-cli 0.142.5`
  - `docker exec tamga-agent-qa-container gemini --version` -> `0.49.0`
  - `docker exec tamga-agent-qa-container opencode --version` -> `1.17.13`
  - `docker exec tamga-agent-qa-container sh -c "which claude codex gemini opencode"` -> all four resolve to `/usr/local/bin/<name>`
- Re-checked `docker ps` after all exec calls: container still `Up`, confirming it stays alive via `tail -f /dev/null` and doesn't exit after exec sessions end (validates the FEAT-004 CMD requirement from this task's AC).
- Image size: `docker images tamga-agent-qa-test` reported `3.1GB`, single RUN layer, consistent with developer/reviewer reports — no duplicated base layers.
- Cleanup: `docker rm -f tamga-agent-qa-container` and `docker rmi tamga-agent-qa-test` — both removed; confirmed via `docker ps -a --filter name=tamga-agent-qa` and `docker images --filter reference=tamga-agent-qa*` returning empty (no leftover artifacts).

All four Acceptance Criteria items were personally observed against a freshly built image/container: all four CLIs present and functional on PATH, image size reasonable with no duplicated layers, container stays running without the old `agent-server` CMD, and the change itself is a minimal one-line Dockerfile edit (KISS/YAGNI satisfied).

No discrepancies from the developer's or reviewer's findings.
