---
id: FEAT-027
type: feature
title: Parse the supported docker-compose subset (compose-go/v2)
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C2 cluster"}
  - {date: 2026-07-10, stage: development, by: architect, note: "assigned (C2 compose parsing)"}
  - {date: 2026-07-10, stage: review, by: architect, note: "compose-go/v2 parser + ComposeService model + rejections done; C2 HOLD pending TEST-014"}
  - {date: 2026-07-10, stage: hold, by: architect, note: "review PASS (profiles-drop fix confirmed live by reviewer); holding for TEST-014"}
  - {date: 2026-07-11, stage: done, by: architect, note: "C2 cluster integration test TEST-014 PASS; complete"}
---

**Part of:** C2-compose-deploy
**Depends on:** (none — parallel with FEAT-025/026)

## Summary
Parse a project's compose YAML into an internal service model the deploy
engine (FEAT-028) consumes, using `github.com/compose-spec/compose-go/v2`
(TEST-011's recommendation — the library `docker compose` itself uses).
Only the supported subset is honored; unsupported features are rejected
with clear errors.

## Requirements
- Add `compose-spec/compose-go/v2` and parse a compose YAML string into the
  project's services. Extract, per service: `image`, `ports`,
  `environment`, `volumes`, `networks`, `depends_on`. Normalize both the
  short and long syntaxes compose allows (e.g. `"8080:80"` vs the long port
  mapping; list vs map env) — this is exactly why we use the library, don't
  hand-parse.
- **Reject the out-of-scope features with a clear, actionable error** (not
  silent ignore): `build:`, `profiles:`, `secrets:`, healthcheck
  conditions, and any other unsupported key that would change runtime
  behavior. A user submitting `build:` must get "build is not supported yet;
  use a prebuilt image" rather than a broken deploy.
- Output an internal `[]ComposeService` (or reuse FEAT-025's domain type if
  it fits) with the extracted, normalized fields — the contract FEAT-028
  builds containers from. Coordinate the type with FEAT-025 (if FEAT-025
  defined ComposeService, produce that; else define here and FEAT-025/028
  consume it — note the seam).
- Validate: at least one service; a referenced `depends_on` target exists;
  duplicate service names rejected.
- Tests: black-box — parse valid subset composes (short+long syntax), assert
  the normalized model; assert each unsupported feature yields the expected
  error; validation errors (missing service, bad depends_on).

## Out of Scope
- Deploying the parsed model (FEAT-028); docker primitives (FEAT-026);
  schema (FEAT-025). Just YAML → internal model + validation.

## Proposed Solution / Approach
Read the sibling C2 tasks (both `in-review`/holding) first to confirm the
seam: FEAT-025 deliberately did **not** define a `ComposeService` type
("Out of Scope explicitly defers compose parsing to FEAT-027... no
speculative abstraction"), and FEAT-026's `service.TopoSortServices` takes
a minimal `ComposeServiceDep{Name, DependsOn []string}` with no dependency
on either FEAT-025's or this task's types. So there's no existing type to
extend - this task defines `domain.ComposeService` fresh, in
`backend/internal/domain/compose_service.go` alongside FEAT-025's
`ServiceContainer`, and documents in its doc comment exactly how it feeds
`service.ComposeServiceDep` (a one-line, no-transformation conversion:
`ComposeServiceDep{Name: cs.Name, DependsOn: cs.DependsOn}`) - the seam
FEAT-028 needs.

**Library integration.** `compose-go/v2`'s `loader.LoadWithContext` takes
a `types.ConfigDetails{ConfigFiles: [{Filename, Content: []byte(yaml)}]}`
- it accepts in-memory YAML content directly, no temp file needed.
Empirically verified against the actual library (v2.13.0, via a scratch
Go program) three load-time gotchas the task's own hints didn't spell out,
each resolved deliberately:
1. The loader requires a non-empty project name (`o.SetProjectName(...,
   true)`) or it hard-errors - Tamga has no equivalent concept, so a fixed
   placeholder name is passed that's never surfaced anywhere.
2. By default, a service with a `profiles:` entry is silently dropped
   from `project.Services` entirely (compose's own "profile not active"
   filtering) *before* this function ever sees it - which would mean
   `profiles:` rejection could never actually fire. Fixed by loading with
   `Profiles: []string{"*"}` (compose-go's own wildcard - "activate every
   profile"), so a profile-gated service still appears and can be
   rejected.
3. `depends_on`'s list form (`- db`) and long map form (`db: {condition:
   ...}`) both normalize to the same `DependsOnConfig` (`map[string]
   ServiceDependency`) before this function's code ever runs - confirmed
   by reading `transform/canonical.go`'s `transformDependsOn` registration
   - so no branching on syntax is needed on Tamga's side at all.

**Consistency-check boundary.** compose-go's own `checkConsistency` (run
unless `SkipConsistencyCheck: true`) already validates "depends_on target
exists" and more, but per TEST-011 §2f ("compose-go parses the full spec,
Tamga's own code is what enforces the subset boundary, not the library")
and to keep one consistent error-message style/one place tests assert
against, `SkipConsistencyCheck: true` is set and Tamga's own code
re-implements the two validations Requirements actually ask for
(≥1 service, depends_on target exists) with its own wording. Duplicate
service names need no code at all: verified empirically that compose-go's
underlying YAML parser rejects a duplicate `services:` mapping key at
parse time (`"mapping key \"web\" already defined"`) - Go maps can't hold
two entries under one key either, so "no duplicate names" is structurally
guaranteed end-to-end, not something ParseComposeYAML has to check itself
(documented + covered by a black-box test asserting the error surfaces).

**Unsupported-feature detection**, per the task's own hint ("the library
populates Build, Profiles, Secrets fields - check for non-empty"):
non-nil/non-empty checks on `ServiceConfig.Build`, `.Profiles`,
`.Secrets`, `.HealthCheck` after parsing - four fields, four clear
"X is not supported yet; do Y instead" errors, matching the task's own
example message for `build:` verbatim. `depends_on`'s `condition:`
sub-field is deliberately *not* rejected (Requirements: "conditions
themselves are out of scope, just the dependency edge") - it's read by
compose-go and simply not carried into `domain.ComposeService.DependsOn`
(names only). Everything else compose-go parses but this function doesn't
map into the 6-field subset (`hostname:`, `labels:`, `restart:`, etc.) is
likewise not carried forward - not rejected, just not part of the
supported subset, per this task's own Out of Scope ("just YAML → internal
model + validation", not full compose fidelity).

**Normalization**: ports → `{Published, Target, Protocol}` (Published ""
when a port isn't host-published); environment → `map[string]string`
(a bare/nil-valued entry like `BAZ` normalizes to `""` rather than being
resolved against any host environment - Tamga has no host-process env to
inherit from and wouldn't want to leak the API server's own env into a
user's container); volumes → `{Type, Source, Target, ReadOnly}`; networks
→ sorted `[]string` of attached network names (including compose-go's
implicit "default" network when a service declares none explicitly -
faithful to real compose semantics, documented for FEAT-028).

## Affected Areas
- `go.mod`/`go.sum` - added `github.com/compose-spec/compose-go/v2 v2.13.0`
  (direct) plus its transitive indirect deps (`go mod tidy`).
- `backend/internal/domain/compose_service.go` (new) - `ComposeService`,
  `ComposePort`, `ComposeVolume` types (the FEAT-028 seam).
- `backend/internal/service/compose_parser.go` (new) -
  `ParseComposeYAML(yamlContent string) ([]domain.ComposeService, error)`,
  the four unsupported-feature rejection checks, and the
  ≥1-service/depends_on-target-exists validations.
- `backend/internal/tests/service/compose_parser_test.go` (new) -
  black-box tests (`package service_test`, per FEAT-021 convention):
  short-syntax parse, long-syntax parse (ports/env/volumes/networks/
  depends_on-with-condition), implicit-default-network, each of the four
  rejections, empty-services, missing-depends_on-target,
  duplicate-service-name, invalid-YAML.
- Not touched (deliberately, per Out of Scope): FEAT-028's deploy engine,
  FEAT-026's docker-client/topo-sort, FEAT-025's schema/storage - this
  task is only YAML → internal model + validation.

## Acceptance Criteria / Definition of Done
- [ ] compose-go/v2 added; a valid subset compose (both short + long syntaxes) parses into the normalized internal model
- [ ] Each unsupported feature (build/profiles/secrets/...) is rejected with a clear error, not silently dropped
- [ ] Validation: ≥1 service, depends_on targets exist, no dup names
- [ ] `go build/vet/test` pass; black-box tests cover valid parse + each rejection + validation
- [ ] Code follows KISS/YAGNI (lean on the library; don't reimplement compose)

## Test Plan
Black-box: a table of compose inputs → expected model or expected error.

## Implementation Notes
Implemented directly (no delegation - `complexity: standard`).

- **`domain.ComposeService`** (`domain/compose_service.go`): `Name,
  Image string`; `Ports []ComposePort`; `Environment map[string]string`;
  `Volumes []ComposeVolume`; `Networks []string`; `DependsOn []string`.
  `ComposePort{Published, Protocol string; Target uint32}`.
  `ComposeVolume{Type, Source, Target string; ReadOnly bool}`. This is the
  exact seam FEAT-028 consumes - doc comment spells out the
  no-transformation conversion to FEAT-026's `service.ComposeServiceDep`.
- **`service.ParseComposeYAML`** (`service/compose_parser.go`): loads via
  `loader.LoadWithContext` with `SkipConsistencyCheck: true`,
  `Profiles: []string{"*"}` (so profile-gated services aren't dropped
  before they can be rejected), and a fixed placeholder project name
  (`SetProjectName`, required by the library, never surfaced). Iterates
  services in sorted-name order for deterministic output; rejects
  build/profiles/secrets/healthcheck per-service (non-nil/non-empty
  checks on compose-go's own parsed fields); validates ≥1 service and
  every `depends_on` target exists against the service-name set; then
  normalizes each service's fields via four small helpers
  (`normalizePorts`/`normalizeEnvironment`/`normalizeVolumes`/
  `normalizeNetworks`).
- **Verified against the real library** (not just skimmed docs): wrote a
  disposable scratch Go program (outside the repo, in the session
  scratchpad) exercising `loader.LoadWithContext` directly against
  v2.13.0 to confirm exact behavior before writing production code -
  project-name requirement, profile-filtering-before-rejection gotcha,
  depends_on list-vs-map normalizing to one shape pre-parse, port/volume/
  network normalization shapes, the empty-`services: {}` non-error case
  (needing our own explicit ≥1-service check), and the duplicate-service-
  name YAML-parse-time rejection. All of these directly shaped the
  Proposed Solution and the code, not just retrofitted after the fact.
- **`go.mod`**: `go get github.com/compose-spec/compose-go/v2@v2.13.0`
  then `go mod tidy` (had to re-run `go get` once after an earlier `tidy`
  transiently dropped the unused-at-the-time entry, before
  `compose_parser.go` existed to reference it - final `go.mod`/`go.sum`
  diff is exactly the direct dependency plus its real transitive closure,
  confirmed via `git diff -- go.mod`).
- **Build/test evidence**: `go build ./...` clean, `go vet ./...` clean,
  `go test ./... -count=1` - all packages pass, including the new
  `TestParseComposeYAML*` suite (11 cases) in
  `internal/tests/service/compose_parser_test.go`, alongside every
  pre-existing package (`internal/service`, `internal/tests/{handler,
  repository,service,sqlite}`, `cmd/egress-proxy`) unaffected. Did not
  touch or restart the live stack, per instructions.

**Deviations from Requirements**: none of substance. One judgment call
worth flagging: `expose:` (container-only port hints, no host publish) is
not extracted - Requirements' field list is exactly `image, ports,
environment, volumes, networks, depends_on` and doesn't mention `expose:`,
and per TEST-011 §2b the deploy model never publishes to the host anyway
(container-to-container only), so `expose:` carries no information
`ports:` doesn't already cover for Tamga's purposes - left out rather than
speculatively added.

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

## Review Notes

**2026-07-10 — reviewer**

Verdict: PASS

Checked out the diff scope, the model, the parser, the rejection/validation
logic, and the test suite, and ran the build/vet/test toolchain myself.

**Scope.** New files match exactly what the Implementation Notes and
Affected Areas claim: `backend/internal/domain/compose_service.go`,
`backend/internal/service/compose_parser.go`,
`backend/internal/tests/service/compose_parser_test.go`, plus the
`go.mod`/`go.sum` diff for `compose-spec/compose-go/v2 v2.13.0`. The
working tree also has other untracked/modified files (`service_container.go`,
`compose_order.go`, migrations 000016, `docker_client_test.go`, etc.) — these
belong to sibling in-review tasks FEAT-025/FEAT-026, not scope creep by this
task; this task's own diff is clean and self-contained.

**1. Model (`domain/compose_service.go`).** `ComposeService{Name, Image,
Ports []ComposePort, Environment map[string]string, Volumes
[]ComposeVolume, Networks []string, DependsOn []string}` is minimal and
sane. `DependsOn []string` maps to FEAT-026's
`service.ComposeServiceDep{Name, DependsOn []string}` with the exact
one-line, no-transformation conversion the doc comment describes — verified
by reading `compose_order.go` directly, field names/types line up exactly.
Carries what FEAT-028 needs to create containers: image, ports
(published+target+protocol), env, volumes (type/source/target/read-only),
networks, deps. Reasonable scope; no speculative fields.

**2. Parser — syntax normalization.** `compose_parser.go` uses
`loader.LoadWithContext` on in-memory `types.ConfigDetails` content, no temp
file. Tests genuinely exercise both syntaxes and assert normalized output:
`TestParseComposeYAMLShortSyntax` (ports `"8080:80"`, env list w/ bare key,
volumes short bind + anonymous) and `TestParseComposeYAMLLongSyntax` (ports
long mapping incl. a container-only/no-`published` entry, env map w/ null,
volumes long form, `networks:` list, `depends_on` map w/ `condition:`) both
assert exact expected structs via `reflect.DeepEqual`, not just "no error."
Ran `go test -run TestParseComposeYAML -v`: all 11 pass.

**Profiles gotcha — traced and empirically re-verified independently.** I
wrote my own scratch program against the same installed v2.13.0 loader,
loading a `profiles: ["dev"]` service once *without* `Profiles:
[]string{"*"}` and once *with* it:
- Without the fix: `project.Services` comes back with **0** entries — the
  profile-gated service is silently dropped before `ParseComposeYAML`'s
  code ever runs, exactly as the dev's Proposed Solution claims. If this
  fix weren't present, a compose file using `profiles:` would hit the
  generic "compose file declares no services" error instead of the
  intended "profiles is not supported yet" rejection — wrong error,
  same failure mode as silent drop for the user's purposes.
- With `Profiles: []string{"*"}` (what the code actually does): the
  service survives into `project.Services` with `Profiles: [dev]`
  populated, so `rejectUnsupportedFeatures`'s `len(svc.Profiles) > 0` check
  fires and the user gets the correct, specific "profiles is not supported
  yet" error.
This confirms the fix is load-bearing and the ordering the task worried
about (profile-drop happening *before* rejection can fire) is correctly
resolved — `TestParseComposeYAMLRejectsProfiles` passing alone wouldn't
have proven this (it doesn't test what happens *without* the fix), so this
was worth checking independently rather than trusting the docstring.

**3. Rejections.** Checked compose-go v2.13.0's actual `types.ServiceConfig`
field declarations directly (`go.pkg.mod` source): `Build *BuildConfig`
(nil check correct), `Profiles []string` (len check correct), `Secrets
[]ServiceSecretConfig` (len check correct), `HealthCheck
*HealthCheckConfig` (nil check correct) — all four checks in
`rejectUnsupportedFeatures` match the real field types, none of them is a
check that could silently miss a populated-but-zero-value field. The
`build:` error message ("build is not supported yet; use a prebuilt image")
matches the task's example verbatim. `depends_on`'s `condition:` is read by
compose-go and simply not copied into `domain.ComposeService.DependsOn`
(names only) rather than rejected — correct per Requirements ("conditions
themselves are out of scope, just the dependency edge"), and
`TestParseComposeYAMLLongSyntax` explicitly asserts the condition is
dropped while the edge survives.

**4. Validation.** ≥1 service and depends_on-target-exists are both
Tamga's own checks (`SkipConsistencyCheck: true` deliberately opts out of
compose-go's equivalent, per the stated one-message-style rationale — a
reasonable, documented tradeoff). The depends_on-target check here and
FEAT-026's `TopoSortServices`' equivalent check are consistent (same
"service %q depends_on undefined service %q" framing) and harmless as a
double-check: `ParseComposeYAML` needs to be independently correct/usable
without assuming callers always feed its output through `TopoSortServices`,
so this isn't redundant complexity, it's each layer being defensively
correct on its own. Duplicate-name rejection is proven, not just asserted:
`TestParseComposeYAMLRejectsDuplicateServiceNames` feeds two `web:` keys
and asserts on the YAML parser's own `"already defined"` error text, which
I confirmed by running the test rather than taking the docstring's word for
it.

**5. Tests.** 11 cases as claimed, table-free but each is a distinct,
meaningful scenario (not tautological): 2 syntax-normalization cases
asserting exact structs, 1 implicit-default-network case, 4 rejection
cases each asserting the specific error substring, 3 validation-error
cases (empty services, missing depends_on target, duplicate name), 1
invalid-YAML case. Ran the full suite (`go test ./... -count=1`): all
packages pass, nothing broken elsewhere.

**6. go.mod/go.sum.** `compose-spec/compose-go/v2 v2.13.0` added as a
direct dependency; `git diff -- go.mod` shows exactly that plus a
plausible transitive closure (mapstructure/v2, go-shellwords, jsonschema/v6,
logrus, str2duration/v2, go.yaml.in/yaml/v4, x/sync, x/text) — all
expected compose-go dependencies, nothing alarming pulled in. `go build
./...`, `go vet ./...`, and `go test ./... -count=1` all ran clean in my
own check, matching the dev's claimed evidence.

**FEAT-028 fitness verdict.** The model is a clean, minimal seam:
image/ports/env/volumes/networks give FEAT-028 everything needed to call
the Docker client (FEAT-026) to create containers, and `DependsOn []string`
converts to `service.ComposeServiceDep` with zero transformation for
`TopoSortServices`. The profiles-drop-vs-reject race the task specifically
flagged as a risk is correctly resolved and I verified it myself
independently of the dev's own docstring claim, not just re-read the
comment. No blocking issues found.

Non-blocking / minor observations (not required to fix):
- The `expose:`-not-extracted judgment call is reasonable and well
  justified (Requirements' field list doesn't include it, and Tamga's
  deploy model doesn't do host-only exposure hints); worth a one-line
  mention in FEAT-028's own task when it lands, in case someone later
  wonders why `expose:` has no representation.
- `rejectUnsupportedFeatures` and the four `normalize*` helpers are private
  (lowercase) with no exported test seam of their own — fine, since they're
  only exercised via the black-box `ParseComposeYAML` entry point per the
  stated `package service_test` convention, consistent with FEAT-021.
