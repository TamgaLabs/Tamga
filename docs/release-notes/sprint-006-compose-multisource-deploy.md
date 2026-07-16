# SPRINT-006 — Multi-source Compose Build & Deploy (2026-07-16)

## Added
- Projects can now contain multiple remote Git sources, each cloned into an owned workspace path and managed independently from the Configure page.
- Configure can detect a source Compose file for approval, provide a Next.js starter when appropriate, and show source refresh failures directly in the project workflow.
- Projects can build every configured Compose service separately. A successful, current build is required before deployment is available.
- Configure now supports assigning eligible Compose services to hostname-only subdomains.
- Environment management now supports global values plus service-specific overrides, with separate drafts and clear completion feedback for each form.

## Changed
- Compose configuration is now an explicit approval step between fetched source files and a buildable project. Accepted configuration identifies the services that can be built, routed, and receive service-level environment overrides.
- Saving a new Compose configuration invalidates the prior build so deployment always uses images produced from the current configuration.

## Fixed
- A malformed newly detected Compose file no longer blocks a project that already has a valid accepted configuration from building.
- A build that overlaps a configuration change cannot mark the newer configuration as successfully built.
- Route configuration now rejects paths, IP addresses, port notation, invalid hostnames, and services absent from the configured Compose project.
- Deleting a global environment variable is confined to its project, and deleting a project also removes its service-level environment values.
- The full multi-source Compose lifecycle has passed 32 browser/runtime acceptance checks.
