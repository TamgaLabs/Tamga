# Tamga Console design system

Tamga Console uses shadcn/ui's `nova` preset as the visual reference while
retaining the repository's compatible `new-york` component contract. New
primitives are added individually and merged with local edits; generated files
are never applied as a wholesale overwrite.

Tailwind 4.3 is configured through `@tailwindcss/postcss` and the CSS-first
theme in `src/app/globals.css`. Semantic tokens are intentionally mapped there
so light/dark themes, sidebar colours, animations, radii, and code typography
remain available without a legacy JavaScript Tailwind config.

Typography is Geist Sans for application copy, Geist Mono for code, and Geist
Pixel Square only for the Tamga Console wordmark or short display headings.
