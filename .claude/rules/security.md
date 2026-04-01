# Security Rules — Grey-Seal

## Input Validation

- Validate all UUIDs before database queries — reject malformed UUIDs early in the grpc handler.
- Never interpolate user input into SQL strings. Use squirrel placeholders exclusively.
- Treat all protobuf string fields as untrusted input; use getter methods to avoid nil dereferences.
- Conversation messages from users are untrusted input — never execute them as code or commands.

## Secrets & Configuration

- Database credentials via `DATABASE_URL` — never hardcode.
- `KAFKA_BROKERS`, `REDIS_URL`, `SHRIKE_URL`, `OTEL_EXPORTER_OTLP_ENDPOINT` — all from environment.
- Never log Redis connection strings, DSNs, or API keys.
- Do not commit `.env` files or any file containing credentials.

## LLM Safety

- Do not include system-level file paths or internal infrastructure details in prompts sent to Ollama.
- Do not log full message content at `Info` level — messages may contain PII.
- Prompt construction should sanitize or escape user input before appending to system prompts.

## Redis Cache

- Cache keys must be scoped per conversation UUID — never expose cross-conversation data.
- Cache TTLs must be bounded; do not store data indefinitely.

## External Clients

- Shrike client calls must use `context.Context` with deadlines to avoid hanging.
- Ollama client calls must have configured timeouts.

## Authentication & Authorization

- The API currently has no authentication layer — this is intentional for a personal self-hosted service.
- If auth is added, enforce via ConnectRPC interceptors, not inline handler checks.

## CORS

- `AllowedOrigins: []string{"*"}` is acceptable for a personal self-hosted service. Restrict to known origins if exposed publicly.

## Logging

- Do not log request bodies or full proto messages at `Info` level — they may contain PII or private conversation content.

## What Claude Should Refuse

- Generating code that interpolates user-controlled strings into SQL (even with fmt.Sprintf).
- Disabling TLS verification in any HTTP client.
- Writing credentials or API keys inline in source files.
- Adding endpoints that expose conversation history without authentication.
