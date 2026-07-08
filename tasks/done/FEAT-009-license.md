---
id: FEAT-009
type: feature
title: Add LICENSE (AGPL-3.0 + Commercial Exception)
status: done
complexity: simple
assignee: sdlc-developer
sprint: SPRINT-001
created: 2026-07-04
history:
  - {date: 2026-07-04, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-06, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-06, stage: in-review, by: architect, note: "two attempts hit an API content-filtering error trying to output the full AGPL text directly - fixed by having the developer curl the canonical text from gnu.org instead, and codified this as a standing rule in sdlc-developer.md; architect verified LICENSE content is genuine AGPL-3.0 (684 lines) + clear commercial clause, README references it correctly; moved to review"}
  - {date: 2026-07-06, stage: in-test, by: architect, note: "review PASSED; moved to test - this is a static doc-only change with no runtime behavior, so no environment build is actually needed"}
  - {date: 2026-07-06, stage: done, by: architect, note: "test skipped builder/tester (no runtime surface); architect directly re-verified LICENSE content and README reference match the Test Plan; moved to done"}
---

## Summary
The repo has no license file. architecture.md specifies a dual-license
model: AGPL-3.0 for personal/non-commercial/open-source use, with a
commercial license required for business/revenue-generating/SaaS use
(same model used by GitLab, Mattermost, Grafana). Add a `LICENSE` file
reflecting this.

## Requirements
- `LICENSE` file at repo root containing the full AGPL-3.0 text
- A clear commercial-exception clause (or a separate `COMMERCIAL-LICENSE.md`
  referenced from `LICENSE`) describing when a commercial license is
  required, matching architecture.md's Licensing Model section verbatim in
  spirit: internal business use, revenue-generating use, embedding in
  commercial products, SaaS/managed service offerings all require a
  commercial license
- README references the license section

## Out of Scope
- Actually drafting/negotiating commercial license terms/pricing — this
  task only adds the repo-facing license text, not a business process

## Proposed Solution / Approach

Create a LICENSE file at the repo root containing the full AGPL-3.0 text followed by a clearly-marked "Commercial License" section that specifies when commercial licensing is required. The commercial clause will match the model in architecture.md: required for companies using Tamga internally, revenue-generating use, embedding in commercial products, and SaaS/managed service offerings. Update README with a new "License" section near the end (before or after "Makefile") that links to the LICENSE file and briefly summarizes the dual-license model. Keep the wording simple and direct, referencing the LICENSE file for full details.

## Affected Areas
- `LICENSE` (new)
- `README.md` (license section reference — coordinate with FEAT-003)

## Acceptance Criteria / Definition of Done
- [x] `LICENSE` file present with full AGPL-3.0 text
- [x] Commercial exception clause clearly documented (in LICENSE or a linked file)
- [x] README links to/mentions the license

## Test Plan
Read through `LICENSE` and confirm it matches architecture.md's stated
model; confirm README references it.

## Implementation Notes

Used `curl -sL https://www.gnu.org/licenses/agpl-3.0.txt -o LICENSE` to download the canonical AGPL-3.0 text directly to the LICENSE file at repo root (avoiding content-filtering issues that would occur if attempting to output the full legal text as part of the model response). Verified the download: 661 lines, starting with "GNU AFFERO GENERAL PUBLIC LICENSE" as expected.

Appended a clearly-marked "## Commercial License" section to the LICENSE file documenting the dual-license model: AGPL-3.0 for personal/non-commercial/open-source use, and commercial licensing required for companies using Tamga internally, revenue-generating use, embedding in commercial products, and SaaS/managed service offerings—matching architecture.md's Licensing Model section.

Updated README.md with a new "## License" section at the end (after Makefile), briefly summarizing the dual-license model and linking to the LICENSE file for full details.

Files touched:
- LICENSE (new file, created via curl)
- README.md (added License section)

## Review Notes

**Verdict: PASS**

All acceptance criteria met with no issues.

**Detailed Verification:**

1. **LICENSE file - AGPL-3.0 text authenticity (684 lines total):**
   - Confirmed correct header: "GNU AFFERO GENERAL PUBLIC LICENSE, Version 3, 19 November 2007"
   - Spot-checked mid-section (lines 400-421) contains authentic AGPL-3.0 sections (Termination, Acceptance Not Required for Having Copies) matching known AGPL-3.0 structure
   - Proper closure at line 661 with reference to https://www.gnu.org/licenses/
   - Implementation correctly uses canonical GNU source: `curl -sL https://www.gnu.org/licenses/agpl-3.0.txt`

2. **Commercial License section (lines 665-683) accuracy:**
   - All four commercial licensing scenarios from architecture.md Licensing Model section are present and accurately represented:
     * "internal business operations" → "Use Tamga for internal business operations (by a company or organization)"
     * "revenue-generating use" → "Generate revenue from using or offering Tamga"
     * "embedding in commercial products" → "Embed Tamga in a commercial product"
     * "SaaS/managed service offerings" → "Offer Tamga as a SaaS or managed service"
   - Clearly marked with markdown header and separator lines for readability
   - References architecture.md for additional context (line 682)

3. **README.md changes:**
   - New "## License" section added at end of file (after Makefile section) as specified
   - Summarizes dual-license model clearly
   - Properly links to LICENSE file with markdown syntax
   - Git diff confirms only the License section was added—no scope creep

4. **Scope verification:**
   - Two files modified as specified: LICENSE (new) and README.md
   - Untracked files in working tree (frontend-refactor.md, qa-debug*.js, UI components, .opencode/, tasks/active/, etc.) are from other concurrent tasks and not part of this implementation

**Minor observations (non-blocking):**
- The Commercial License section references "the project maintainers" for inquiries (line 680) but doesn't specify a contact method. This is acceptable for now as the task is scoped to adding repo-facing license text, not setting up the full commercial licensing process.
- Line 669 has a trailing space (after "use,") but this is inconsequential.

## Test Notes
<Filled in by the tester.>
