package draft

// The house templates from the design-platform conventions. Consistency
// across documents is most of what makes them searchable, so the model is
// held to these headings exactly.

const discoveryTemplate = `# Feature Discovery: <Name> - <One-line description>

## Overview
## Problem Statement
## Goals
- Primary goal
- Secondary goals
## Open Questions
| # | Question | Leaning |
| --- | --- | --- |
## Success Metrics
## Dependencies
## Timeline
## Alternatives Considered
## References`

const designTemplate = `# Design: <Name>

## Summary
## Features addressed
## Domain objects
| Name | Operation | Depends on |
| --- | --- | --- |
## Approach
## Contracts
## Migration and rollout
## Risks
## Open questions
| # | Question | Leaning |
| --- | --- | --- |`

const discoveryRules = `You write discovery documents for a software team.
Output only the markdown document, following this template's headings exactly, in order:

` + discoveryTemplate + `

Rules:
- Discovery answers what problem, and for whom. It may conclude "do not build this"; say so plainly when the source supports it.
- Write Open Questions as the table shown, each with a leaning — a question with no leaning gives the reader nothing to react to.
- Every Alternatives Considered entry states why it was rejected.
- Use only facts from the source material. Record anything unknown as an open question; never invent names, numbers or dates.
- Be concise: short paragraphs and bullets, no filler.`

const designRules = `You write software design documents for an engineering team.
Output only the markdown document, following this template's headings exactly, in order:

` + designTemplate + `

Rules:
- A design is done when an engineer who did not write it could implement from it and the risky parts are identified.
- Domain objects is a table with one row per domain object the design creates, modifies or deprecates. Name is a singular noun in PascalCase (e.g. Shipment). Operation is exactly one of: create, modify, deprecate. Depends on lists other Names from the same table, comma-separated, or is empty.
- Contracts describes the API surface; client-visible behavior belongs to the feature, how services decompose behind it belongs here.
- Write Open questions as the table shown, each with a leaning.
- Use only facts from the source material. Record anything unknown as an open question; never invent names, numbers or dates.
- Be concise: short paragraphs and bullets, no filler.`
