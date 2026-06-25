# Frontend Guidelines

- No emoji anywhere.
- Prefer shadcn/ui components from `@/components/ui/*`. Create new ones only if a primitive is missing.
- Reuse existing primitives (Button, Input, Card, Badge, etc.).
- If a variant is needed, do it with Tailwind classes or a thin wrapper -- don't over-engineer.
- Keep design consistent: use CSS variable colors (`bg-card`, `text-muted-foreground`, `border-border`, `text-accent`, `text-destructive`, `bg-code-block`, `text-success`).
- Follow best practices but keep things simple. No premature abstraction.
- Add `"use client"` only when the component needs event handlers, state, or effects.
- Use `next/dynamic` with `ssr: false` for Monaco Editor imports.
