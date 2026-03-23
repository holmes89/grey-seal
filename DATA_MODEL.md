# Data Model

## Protobuf Entities (`schemas/greyseal/v1/`)

### Role

A named, reusable system prompt that can be applied to one or more conversations.

| Field | Proto type | Notes |
|---|---|---|
| `uuid` | `string` | Primary key (UUID) |
| `name` | `string` | Human-readable label |
| `system_prompt` | `string` | Text injected as the first system message in the LLM call |
| `created_at` | `google.protobuf.Timestamp` | Creation time |

### Resource

An ingested document registered in the knowledge base. Actual content and embeddings are managed externally (by the worker / shrike / Qdrant); this table holds only metadata.

| Field | Proto type | Notes |
|---|---|---|
| `uuid` | `string` | Primary key (UUID) |
| `name` | `string` | Human-readable label |
| `service` | `string` | Originating service identifier (free-form) |
| `entity` | `string` | Entity identifier within the service (free-form) |
| `source` | `Source` enum | `UNSPECIFIED=0`, `WEBSITE=1`, `PDF=2`, `TEXT=3` |
| `path` | `string` | URL or inline text content depending on `source` |
| `created_at` | `google.protobuf.Timestamp` | Ingestion time |
| `indexed_at` | `google.protobuf.Timestamp` | When embeddings were stored (nullable) |

### Conversation

A persistent chat session.

| Field | Proto type | Notes |
|---|---|---|
| `uuid` | `string` | Primary key (UUID) |
| `title` | `string` | Optional display name |
| `role_uuid` | `string` | FK to `Role`; empty means no system prompt |
| `resource_uuids` | `repeated string` | Scopes retrieval to these resources; empty = all |
| `summary` | `string` | Rolling compressed summary for context management |
| `messages` | `repeated Message` | Populated on `Get`; absent on `List` |
| `created_at` | `google.protobuf.Timestamp` | |
| `updated_at` | `google.protobuf.Timestamp` | Updated on every `Chat` call |

### Message

A single turn within a conversation.

| Field | Proto type | Notes |
|---|---|---|
| `uuid` | `string` | Primary key (UUID) |
| `conversation_uuid` | `string` | FK to `Conversation` (CASCADE DELETE) |
| `role` | `MessageRole` enum | `UNSPECIFIED=0`, `USER=1`, `ASSISTANT=2` |
| `content` | `string` | Full text of the message |
| `resource_uuids` | `repeated string` | Resources cited in an assistant reply |
| `feedback` | `int32` | User rating: `-1` negative, `0` neutral, `1` positive |
| `created_at` | `google.protobuf.Timestamp` | |

## PostgreSQL Schema

Migrations are in `lib/repo/migrations/` and are applied automatically by goose on startup.

### `roles`

```sql
CREATE TABLE roles (
    uuid        TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    system_prompt TEXT NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE INDEX idx_roles_created_at ON roles(created_at);
```

### `resources`

```sql
CREATE TABLE resources (
    uuid       TEXT PRIMARY KEY,
    name       TEXT NOT NULL DEFAULT '',
    service    TEXT NOT NULL DEFAULT '',
    entity     TEXT NOT NULL DEFAULT '',
    source     INTEGER NOT NULL DEFAULT 0,  -- proto enum value
    path       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    indexed_at TIMESTAMP WITH TIME ZONE     -- nullable
);
CREATE INDEX idx_resources_created_at ON resources(created_at);
```

### `conversations`

```sql
CREATE TABLE conversations (
    uuid           TEXT PRIMARY KEY,
    title          TEXT NOT NULL DEFAULT '',
    role_uuid      TEXT NOT NULL DEFAULT '',
    resource_uuids TEXT[] NOT NULL DEFAULT '{}',
    summary        TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at     TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE INDEX idx_conversations_updated_at ON conversations(updated_at);
CREATE INDEX idx_conversations_created_at ON conversations(created_at);
```

`resource_uuids` is a native PostgreSQL `TEXT[]` column. No enforced foreign-key constraint to the `resources` table.

### `messages`

```sql
CREATE TABLE messages (
    uuid              TEXT PRIMARY KEY,
    conversation_uuid TEXT NOT NULL REFERENCES conversations(uuid) ON DELETE CASCADE,
    role              INTEGER NOT NULL DEFAULT 0,  -- proto enum value
    content           TEXT NOT NULL,
    resource_uuids    TEXT[] NOT NULL DEFAULT '{}',
    feedback          INTEGER NOT NULL DEFAULT 0,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE INDEX idx_messages_conversation_uuid ON messages(conversation_uuid);
CREATE INDEX idx_messages_created_at ON messages(created_at);
```

`role` stores the `MessageRole` enum as an integer (0=unspecified, 1=user, 2=assistant). `messages` has a hard CASCADE DELETE constraint on `conversation_uuid`.

## Relationships

```
roles ◄──────────────── conversations ────────────── resources
 (role_uuid, soft ref)   (resource_uuids, soft ref)

conversations ──[1:N, CASCADE]──► messages
```

- `conversations.role_uuid` is a soft text reference to `roles.uuid`; no foreign key constraint is enforced by the database.
- `conversations.resource_uuids` and `messages.resource_uuids` are PostgreSQL arrays of text UUIDs with no FK enforcement.
- `messages.conversation_uuid` has a hard `REFERENCES conversations(uuid) ON DELETE CASCADE`.

## Service API Surface

### ConversationService

| RPC | Transport | Description |
|---|---|---|
| `CreateConversation` | Unary | Create a new conversation |
| `GetConversation` | Unary | Fetch conversation with messages |
| `ListConversations` | Unary | Paginated list (no messages) |
| `UpdateConversation` | Unary | Update title, role, resource scope |
| `DeleteConversation` | Unary | Delete conversation and messages (cascade) |
| `Chat` | Server-streaming | Send a user message; stream assistant tokens |
| `SubmitFeedback` | Unary | Record feedback on an assistant message |

### RoleService

| RPC | Transport | Description |
|---|---|---|
| `CreateRole` | Unary | |
| `GetRole` | Unary | |
| `ListRoles` | Unary | Paginated |
| `UpdateRole` | Unary | |
| `DeleteRole` | Unary | |

### ResourceService

| RPC | Transport | Description |
|---|---|---|
| `IngestResource` | Unary | Register a resource for indexing |
| `GetResource` | Unary | |
| `ListResources` | Unary | Paginated |
| `DeleteResource` | Unary | |
