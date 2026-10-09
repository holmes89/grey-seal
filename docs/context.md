# Context

Authoritative product-level documents live in the narwhal catalog and are linked here, not copied. This repo's own docs (designs, features, specs) are authoritative in git.

## Product-level documents

- [Domain-Driven Design Reference — joel.holmes.haus](https://github.com/holmes89/narwhal/blob/main/designs/joel.holmes.haus/DDD.md) — `joel.holmes.haus/DDD.md`
- [Feature MVP: Orb-weaver — Entity Graph Service](https://github.com/holmes89/narwhal/blob/main/designs/joel.holmes.haus/features/04-orb-weaver-mvp.md) — `joel.holmes.haus/features/04-orb-weaver-mvp.md`
- [Feature Discovery: Termite - Autonomous Service Building Agent](https://github.com/holmes89/narwhal/blob/main/designs/joel.holmes.haus/features/12-termite-mvp.md) — `joel.holmes.haus/features/12-termite-mvp.md`
- [LLM Infrastructure Research](https://github.com/holmes89/narwhal/blob/main/designs/joel.holmes.haus/research/llm_infrastructure.md) — `joel.holmes.haus/research/llm_infrastructure.md`
- [Nautilus System Design](https://github.com/holmes89/narwhal/blob/main/designs/nautilus/system.md) — `nautilus/system.md`
- [Ptah — LLM Context](https://github.com/holmes89/narwhal/blob/main/designs/ptah.holmes.haus/CLAUDE.md) — `ptah.holmes.haus/CLAUDE.md`
- [Feature 4: AI Studio — Staged Development Pipeline](https://github.com/holmes89/narwhal/blob/main/designs/ptah.holmes.haus/features/4-ai-pipeline.md) — `ptah.holmes.haus/features/4-ai-pipeline.md`
- [Ptah — System Design](https://github.com/holmes89/narwhal/blob/main/designs/ptah.holmes.haus/system.md) — `ptah.holmes.haus/system.md`

## Docs in this repo

Each doc carries its platform `uuid`; the portal link is the system-of-record view. `body_sha256` in the frontmatter detects drift.

| Doc | Kind | UUID | Portal | Design | File |
|---|---|---|---|---|---|
| Grey Seal — RAG Chat Backend | design | `ccf69d41-9b0b-5169-a9a4-8bbe07487c6c` | [open](https://joel.holmes.haus/discovery/ccf69d41-9b0b-5169-a9a4-8bbe07487c6c) | [03a248a4](https://joel.holmes.haus/designs/03a248a4-16e9-520e-91ad-66d197425b2b) | `docs/designs/0-mvp.md` |
| Feature: Grey-seal — Conversational Knowledge Assistant | feature | `2c866742-e11c-5323-a603-e4105addd44b` | [open](https://joel.holmes.haus/discovery/2c866742-e11c-5323-a603-e4105addd44b) | [aead0381](https://joel.holmes.haus/designs/aead0381-278c-500e-901f-f649417a3a77) | `docs/features/08-grey-seal-rag.md` |
| Feature: Contextual Compression & Re-ranking in the RAG Pipeline | feature | `e260fd0f-82b2-5d9d-8447-751f89ba0b4a` | [open](https://joel.holmes.haus/discovery/e260fd0f-82b2-5d9d-8447-751f89ba0b4a) | [4e11da25](https://joel.holmes.haus/designs/4e11da25-604c-5135-92cd-4e0f2fe85068) | `docs/features/15-contextual-compression.md` |
| Feature: Grey-seal — Conversation Transcripts | feature | `c7850c17-a0ae-54f8-9de8-c80f8c15142e` | [open](https://joel.holmes.haus/discovery/c7850c17-a0ae-54f8-9de8-c80f8c15142e) | [9ecb519f](https://joel.holmes.haus/designs/9ecb519f-1192-52d5-a317-1591bf0196ac) | `docs/features/19-grey-seal-transcripts.md` |
