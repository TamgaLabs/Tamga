# Browser E2E contract

The lifecycle has an explicit builder/tester handoff. The builder creates a
private disposable Tamga Compose project, chooses unique loopback ports, a
unique Compose project/network/images, and a private temporary data directory.
It records every exact resource in the one TEST-023 manifest and prints a
private handoff file. The tester consumes only that handoff; it never starts or
selects a stack. The builder performs final manifest cleanup after the full
cluster run. Neither mode reads or uses the developer's `localhost:80`,
`localhost:443`, `.env`, or running stack.

Run it only with an explicit ownership acknowledgement:

```sh
E2E_DISPOSABLE=1 E2E_MANIFEST=/tmp/tamga-sdlc-test-023.manifest \
make test-e2e-prepare
# Builder passes the printed path to the tester:
E2E_DISPOSABLE=1 E2E_MANIFEST=/tmp/tamga-sdlc-test-023.manifest \
E2E_HANDOFF_FILE=/tmp/tamga-sdlc-playwright.../handoff.env \
make test-e2e
```

The target rejects caller-provided `E2E_BASE_URL`, `E2E_ADMIN_PASSWORD`, and
`E2E_OWNED_STACK`: accepting them could turn a browser test into a request to a
shared stack. The fixture supplies its deterministic local-only credential to
the Playwright process after it creates a fresh database. Global setup verifies
that credential, and the journeys create no project, container, or other
persistent application data. HTTPS certificate errors are accepted only for
the disposable local Traefik fixture.

`E2E_MANIFEST` must already exist and is never created or deleted by either
mode. On success, failure, or interrupted Compose startup, prepare records the
exact fixture resources in that manifest. The handoff is a restricted private
file whose URL must be `https://localhost:<unique-port>` and whose manifest
must match. The TEST-023 builder then calls `scripts/sdlc-environment.sh
cleanup` once after all C3 runtime checks. This preserves an inspectable
single-owner cleanup boundary.

Failures retain traces, screenshots, videos, and the HTML report under the
ignored `test-results/` and `playwright-report/` directories. CI should upload
those paths as failed-job artifacts.
