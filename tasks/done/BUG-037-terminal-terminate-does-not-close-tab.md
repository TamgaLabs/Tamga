---
id: BUG-037
type: bug
title: Terminating an agent session does not remove its Code-page terminal tab
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-13
history:
  - {date: 2026-07-13, stage: created, by: architect, note: "filed from user-reported terminal workflow"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned as first eligible C2 terminal reliability part"}
  - {date: 2026-07-13, stage: review, by: architect, note: "terminate snapshot-race fix submitted for standard review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires preserve concurrent real tabs and functional active-tab fallback"}
  - {date: 2026-07-13, stage: review, by: architect, note: "terminal tab concurrency rework submitted for review"}
  - {date: 2026-07-13, stage: development-complete, by: developer_standard, note: "prevented stale entry session snapshot from resurrecting successfully terminated tabs"}
  - {date: 2026-07-13, stage: rework-complete, by: developer_standard, note: "preserved concurrently resolved real tabs and used functional active-tab fallback"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-021 integration"}
  - {date: 2026-07-13, stage: test-pass, by: architect, note: "TEST-021 C2 integration verified"}
  - {date: 2026-07-13, stage: done, by: architect, note: "C2 cluster complete"}
---

## Summary
Choosing Terminate ends the server-side agent terminal session, but the
corresponding tab remains visible in the Code workspace. The UI then represents
a session that no longer exists.

**Part of:** C2 Terminal interaction reliability

**Cluster Test:** TEST-021

**Depends on:** none

## Steps to Reproduce
1. Open a project Code page and create or reattach an agent terminal tab.
2. Use the tab’s terminate action and confirm the dialog.
3. Observe the server session is terminated while the tab remains in the UI.

## Expected Behavior
After a successful termination response, the terminated tab is removed and a
valid remaining tab (or the empty-terminal state) becomes active.

## Actual Behavior
The terminal is terminated, but its Code-page tab remains visible.

## Environment / Context
User-reported against the project Code workspace. Current source already
attempts local tab removal after `terminateAgentSession`, so the implementation
must trace the real async/session/tab state path instead of assuming the
visible source branch is sufficient.

## Root Cause
The initial `listAgentSessions` request is intentionally allowed to resolve
after the Code page mounts. If a user terminated a tab before that request
resolved, its older response still contained that session and the local merge
added the terminated tab back. The DELETE had succeeded; the stale snapshot
overwrote the UI consequence of that success.

## Proposed Solution
Record each successfully terminated session id locally, exclude those ids from
the one-time session-snapshot merge, and derive tab removal/active-tab fallback
through small pure helpers. Do not record an id when DELETE fails, so existing
error feedback and the tab/session remain intact.

## Affected Areas
- `frontend/src/app/(main)/code/[id]/page.tsx`
- `frontend/src/components/agent-terminal.tsx`
- `frontend/src/lib/api.ts`
- terminal session API/handler/service paths (read for response semantics)

## Acceptance Criteria
- [ ] Original reproduction removes the matching tab only after successful
      server termination.
- [ ] Active-tab selection falls back deterministically to a remaining tab or
      the empty-terminal state.
- [ ] A failed termination leaves the tab/session available and shows existing
      error feedback.
- [ ] Pending/new terminal tab behavior is not regressed.

## Test Plan
Run only through TEST-021 with FEAT-050: create/re-attach sessions, terminate
active and inactive tabs, verify REST/session list and rendered tab state, and
cover one forced termination error without creating unrelated project data.

## Implementation Notes
- Added `terminal-tabs` helpers for snapshot merging and deterministic active
  tab fallback, with focused unit coverage for stale snapshot, active, and
  inactive tab termination.
- The Code page records an id only after `terminateAgentSession` resolves,
  removes the matching tab, and prevents the initial in-flight list response
  from re-adding it. Pending/new tab and websocket handling are unchanged.
- Rework: snapshot reconciliation now preserves all local, non-terminated real
  tabs when an older concurrent response omits them. The successful-delete
  fallback reads active-tab state through a functional updater, so a newer user
  selection made during DELETE is retained.
- Checks passed: `npm run test:unit` (3 files, 8 tests), `npm run lint`, and
  `npm run build` in `frontend/`.

## Review Notes
### CHANGES_REQUESTED — 2026-07-13
- `mergeTerminalTabs` hâlâ entry-time snapshot ile eşzamanlı çözülen yeni/pending terminali düşürebilir: `handleSessionResolved` pending id'yi gerçek id'ye çevirirse, eski snapshot bu id'yi içermediğinde helper yalnız pending ve snapshot'tan yeni gelenleri döndürüyor; mevcut gerçek tab korunmuyor. Bu, helper üstündeki "existing tabs" yorumuyla ve pending/new tab gerilemesin kabul kriteriyle çelişiyor.
- Mevcut gerçek, terminate edilmemiş tabları snapshot merge'inde koruyun; yalnız `terminatedSessionIds` içindekileri dışarıda bırakın. Bu eşzamanlı resolve + eski snapshot dizisini doğrudan kapsayan unit test ekleyin.
- Başarılı DELETE sonrasında aktif tab seçimini `activeTabId` async closure'ından değil functional state update ile türetin; istek sürerken kullanıcı başka bir taba geçerse arka plan tabının silinmesi aktif seçimi eski değere geri döndürmemeli.
- Doğrulananlar: başarısız DELETE catch'i tab/ref'i değiştirmiyor, hata görünür kalıyor ve tekrar terminate/reattach mümkün; backend DELETE başarısızlığı non-2xx döndürüyor. `npm run test:unit` (3 dosya, 7 test) ve `npm run lint` PASS.

### PASS — 2026-07-13
- Rework, stale entry snapshot'in localde bu sırada resolved olmuş gerçek tabı düşürmesini önlüyor; terminated id yine hem mevcut hem snapshot kaynaklarından dışlanıyor. Yeni focused test bu yarışın merge sonrasındaki gerçek durumunu kapsıyor.
- Başarılı DELETE sonrası active-tab güncellemesi functional updater ile anlık seçimden türetiliyor; istek sürerken yapılmış daha yeni seçim korunuyor. DELETE hata yolunda ref/tab değişmediğinden mevcut hata, retry ve reattach semantiği aynen kalıyor.
- Doğrulandı: `npm run test:unit` (3 dosya, 8 test), `npm run lint`, `npm run build` PASS.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED | n/a | n/a | 1 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS | n/a | n/a | 1 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS | n/a | n/a | 1 |
