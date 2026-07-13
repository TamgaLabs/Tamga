# Frontend unit tests

Place Vitest files next to the code they cover as `*.test.ts` or
`*.test.tsx`. Tests run in jsdom and must not contact a Tamga API, Docker, or
a browser server. Mock browser-only and network integrations at their import
boundary (`next/navigation`, `fetch`, or a local API module), then assert the
visible or returned behavior. Shared cleanup belongs in `test/setup.ts`.

Run the suite with `npm run test:unit` or `make test-frontend-unit` from the
repository root.
