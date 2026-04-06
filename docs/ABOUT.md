# About Grey Seal

Grey Seal is a Retrieval-Augmented Generation (RAG) chat backend — a service for having grounded conversations with a knowledge base I actually own and control.

## Why I Built It

I wanted to be able to ask questions across the documents, links, and notes stored in my self-hosted platform and get answers that cite real sources rather than hallucinated ones. Commercial RAG products exist, but they require sending your data to a third party. Grey Seal runs entirely on my own infrastructure: the LLM runs locally via Ollama, vector search is handled by Shrike, and the data never leaves the network.

It also gave me a concrete project to learn how RAG pipelines work in practice — chunking, embedding, retrieval, and prompt construction — beyond toy examples.

## What It Does

- Manages **conversations** — persistent chat sessions with a full message history.
- Manages **roles** — named system prompts that can be assigned to a conversation to specialise its behaviour.
- Manages **resources** — documents scoped to a conversation that constrain retrieval to a relevant subset of the knowledge base.
- Answers user queries by retrieving semantically relevant chunks from Shrike and injecting them into the LLM prompt context.
- Streams responses back to the client via a Connect-RPC server-streaming `Chat` RPC.
- Records per-message feedback (−1 / 0 / 1) for quality tracking.
- Provides a CLI (`ingest`) for submitting URLs or raw text to the knowledge base.

## Tech Stack

- **Backend:** Go, ConnectRPC, PostgreSQL
- **LLM inference:** Ollama (`deepseek-r1` by default)
- **Vector search:** Shrike (which in turn uses Qdrant + Ollama embeddings)
- **Messaging:** Kafka (Redpanda in local development)

For a deeper look at how the pieces fit together, see [ARCH.md](ARCH.md).
