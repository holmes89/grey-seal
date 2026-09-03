# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [schemas/greyseal/v1/agent.proto](#schemas_greyseal_v1_agent-proto)
    - [AgentRun](#schemas-greyseal-v1-AgentRun)
  
- [schemas/greyseal/v1/conversation.proto](#schemas_greyseal_v1_conversation-proto)
    - [Conversation](#schemas-greyseal-v1-Conversation)
    - [Message](#schemas-greyseal-v1-Message)
  
    - [MessageRole](#schemas-greyseal-v1-MessageRole)
  
- [schemas/greyseal/v1/resource.proto](#schemas_greyseal_v1_resource-proto)
    - [Resource](#schemas-greyseal-v1-Resource)
  
    - [Source](#schemas-greyseal-v1-Source)
  
- [schemas/greyseal/v1/role.proto](#schemas_greyseal_v1_role-proto)
    - [Role](#schemas-greyseal-v1-Role)
  
- [schemas/greyseal/v1/services/agent.proto](#schemas_greyseal_v1_services_agent-proto)
    - [AgentRunEvent](#schemas-greyseal-services-v1-AgentRunEvent)
    - [GetAgentRunRequest](#schemas-greyseal-services-v1-GetAgentRunRequest)
    - [GetAgentRunResponse](#schemas-greyseal-services-v1-GetAgentRunResponse)
    - [ListAgentRunsRequest](#schemas-greyseal-services-v1-ListAgentRunsRequest)
    - [ListAgentRunsResponse](#schemas-greyseal-services-v1-ListAgentRunsResponse)
    - [RunAgentTaskRequest](#schemas-greyseal-services-v1-RunAgentTaskRequest)
    - [RunAgentTaskResponse](#schemas-greyseal-services-v1-RunAgentTaskResponse)
    - [StreamAgentRunRequest](#schemas-greyseal-services-v1-StreamAgentRunRequest)
  
    - [AgentService](#schemas-greyseal-services-v1-AgentService)
  
- [schemas/greyseal/v1/services/conversation.proto](#schemas_greyseal_v1_services_conversation-proto)
    - [ChatRequest](#schemas-greyseal-services-v1-ChatRequest)
    - [ChatResponse](#schemas-greyseal-services-v1-ChatResponse)
    - [CreateConversationRequest](#schemas-greyseal-services-v1-CreateConversationRequest)
    - [CreateConversationResponse](#schemas-greyseal-services-v1-CreateConversationResponse)
    - [DeleteConversationRequest](#schemas-greyseal-services-v1-DeleteConversationRequest)
    - [DeleteConversationResponse](#schemas-greyseal-services-v1-DeleteConversationResponse)
    - [GetConversationRequest](#schemas-greyseal-services-v1-GetConversationRequest)
    - [GetConversationResponse](#schemas-greyseal-services-v1-GetConversationResponse)
    - [ListConversationsRequest](#schemas-greyseal-services-v1-ListConversationsRequest)
    - [ListConversationsResponse](#schemas-greyseal-services-v1-ListConversationsResponse)
    - [SubmitFeedbackRequest](#schemas-greyseal-services-v1-SubmitFeedbackRequest)
    - [SubmitFeedbackResponse](#schemas-greyseal-services-v1-SubmitFeedbackResponse)
    - [UpdateConversationRequest](#schemas-greyseal-services-v1-UpdateConversationRequest)
    - [UpdateConversationResponse](#schemas-greyseal-services-v1-UpdateConversationResponse)
  
    - [ConversationService](#schemas-greyseal-services-v1-ConversationService)
  
- [schemas/greyseal/v1/services/resource.proto](#schemas_greyseal_v1_services_resource-proto)
    - [DeleteResourceRequest](#schemas-greyseal-services-v1-DeleteResourceRequest)
    - [DeleteResourceResponse](#schemas-greyseal-services-v1-DeleteResourceResponse)
    - [GetResourceRequest](#schemas-greyseal-services-v1-GetResourceRequest)
    - [GetResourceResponse](#schemas-greyseal-services-v1-GetResourceResponse)
    - [IngestResourceRequest](#schemas-greyseal-services-v1-IngestResourceRequest)
    - [IngestResourceResponse](#schemas-greyseal-services-v1-IngestResourceResponse)
    - [ListResourcesRequest](#schemas-greyseal-services-v1-ListResourcesRequest)
    - [ListResourcesResponse](#schemas-greyseal-services-v1-ListResourcesResponse)
  
    - [ResourceService](#schemas-greyseal-services-v1-ResourceService)
  
- [schemas/greyseal/v1/services/role.proto](#schemas_greyseal_v1_services_role-proto)
    - [CreateRoleRequest](#schemas-greyseal-services-v1-CreateRoleRequest)
    - [CreateRoleResponse](#schemas-greyseal-services-v1-CreateRoleResponse)
    - [DeleteRoleRequest](#schemas-greyseal-services-v1-DeleteRoleRequest)
    - [DeleteRoleResponse](#schemas-greyseal-services-v1-DeleteRoleResponse)
    - [GetRoleRequest](#schemas-greyseal-services-v1-GetRoleRequest)
    - [GetRoleResponse](#schemas-greyseal-services-v1-GetRoleResponse)
    - [ListRolesRequest](#schemas-greyseal-services-v1-ListRolesRequest)
    - [ListRolesResponse](#schemas-greyseal-services-v1-ListRolesResponse)
    - [UpdateRoleRequest](#schemas-greyseal-services-v1-UpdateRoleRequest)
    - [UpdateRoleResponse](#schemas-greyseal-services-v1-UpdateRoleResponse)
  
    - [RoleService](#schemas-greyseal-services-v1-RoleService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="schemas_greyseal_v1_agent-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## schemas/greyseal/v1/agent.proto



<a name="schemas-greyseal-v1-AgentRun"></a>

### AgentRun
AgentRun is the persisted state of one agentic coding-task run.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |
| provider | [string](#string) |  | provider: &#34;claude&#34; (Managed Agents) — the only implemented value in Phase 1. &#34;ollama:&lt;model&gt;&#34; is a reserved, not-yet-implemented value. |
| repo_url | [string](#string) |  |  |
| status | [string](#string) |  | status: &#34;running&#34; | &#34;idle&#34; | &#34;terminated&#34; | &#34;error&#34;. |
| session_id | [string](#string) |  | session_id is the provider&#39;s session identifier — for Claude this is a Managed Agents session ID, usable to build a Console trace-view link. |
| pr_url | [string](#string) |  | pr_url is populated once the agent opens a pull request, if it does. |
| error | [string](#string) |  | error holds a human-readable failure reason when status is &#34;error&#34;. |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| updated_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |





 

 

 

 



<a name="schemas_greyseal_v1_conversation-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## schemas/greyseal/v1/conversation.proto



<a name="schemas-greyseal-v1-Conversation"></a>

### Conversation
Conversation is a chat session that persists and can be resumed.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |
| title | [string](#string) |  | title is an optional display name; can be set manually or auto-generated from the first exchange. |
| role_uuid | [string](#string) |  | role_uuid optionally links to a Role that sets the system prompt. |
| resource_uuids | [string](#string) | repeated | resource_uuids optionally scopes retrieval to a specific set of resources. When empty, all indexed resources are searched. |
| summary | [string](#string) |  | summary holds a rolling compressed summary of older messages to manage context window length. |
| messages | [Message](#schemas-greyseal-v1-Message) | repeated |  |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| updated_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |






<a name="schemas-greyseal-v1-Message"></a>

### Message
Message is a single turn in a conversation.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |
| conversation_uuid | [string](#string) |  |  |
| role | [MessageRole](#schemas-greyseal-v1-MessageRole) |  |  |
| content | [string](#string) |  |  |
| resource_uuids | [string](#string) | repeated | resource_uuids holds references to indexed resources used to generate this response (populated for ASSISTANT messages). |
| feedback | [int32](#int32) |  | feedback allows simple quality tracking: -1 negative, 0 neutral, 1 positive. |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |





 


<a name="schemas-greyseal-v1-MessageRole"></a>

### MessageRole


| Name | Number | Description |
| ---- | ------ | ----------- |
| MESSAGE_ROLE_UNSPECIFIED | 0 |  |
| MESSAGE_ROLE_USER | 1 |  |
| MESSAGE_ROLE_ASSISTANT | 2 |  |


 

 

 



<a name="schemas_greyseal_v1_resource-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## schemas/greyseal/v1/resource.proto



<a name="schemas-greyseal-v1-Resource"></a>

### Resource
Resource represents an ingested and indexed document used as conversation context.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |
| name | [string](#string) |  |  |
| service | [string](#string) |  |  |
| entity | [string](#string) |  |  |
| source | [Source](#schemas-greyseal-v1-Source) |  |  |
| path | [string](#string) |  |  |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |
| indexed_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |





 


<a name="schemas-greyseal-v1-Source"></a>

### Source


| Name | Number | Description |
| ---- | ------ | ----------- |
| SOURCE_UNSPECIFIED | 0 |  |
| SOURCE_WEBSITE | 1 |  |
| SOURCE_PDF | 2 |  |
| SOURCE_TEXT | 3 |  |


 

 

 



<a name="schemas_greyseal_v1_role-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## schemas/greyseal/v1/role.proto



<a name="schemas-greyseal-v1-Role"></a>

### Role
Role is a reusable named system prompt that can be assigned to a conversation
to shape how the chatbot responds. Leaving role_uuid blank on a conversation
means no system prompt is applied.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |
| name | [string](#string) |  |  |
| system_prompt | [string](#string) |  |  |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  |  |





 

 

 

 



<a name="schemas_greyseal_v1_services_agent-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## schemas/greyseal/v1/services/agent.proto



<a name="schemas-greyseal-services-v1-AgentRunEvent"></a>

### AgentRunEvent
AgentRunEvent is one relayed event from a running agent session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [string](#string) |  | type: the provider event type, e.g. &#34;agent.message&#34;, &#34;agent.tool_use&#34;, &#34;session.status_idle&#34;, &#34;session.status_terminated&#34;. |
| message | [string](#string) |  | message is human-readable text for message/tool-use events. |
| status | [string](#string) |  | status is populated on status-change events: &#34;running&#34; | &#34;idle&#34; | &#34;terminated&#34; | &#34;error&#34;. |






<a name="schemas-greyseal-services-v1-GetAgentRunRequest"></a>

### GetAgentRunRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |






<a name="schemas-greyseal-services-v1-GetAgentRunResponse"></a>

### GetAgentRunResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.AgentRun](#schemas-greyseal-v1-AgentRun) |  |  |






<a name="schemas-greyseal-services-v1-ListAgentRunsRequest"></a>

### ListAgentRunsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| status | [string](#string) | optional | status filters to exactly this value (e.g. &#34;running&#34;, &#34;idle&#34;, &#34;terminated&#34;) when set; unset/empty returns every run. |
| count | [int32](#int32) | optional | count caps the number of runs returned (newest-first); 0/unset means no cap. |






<a name="schemas-greyseal-services-v1-ListAgentRunsResponse"></a>

### ListAgentRunsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.AgentRun](#schemas-greyseal-v1-AgentRun) | repeated |  |
| count | [int32](#int32) |  |  |






<a name="schemas-greyseal-services-v1-RunAgentTaskRequest"></a>

### RunAgentTaskRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| provider | [string](#string) |  | provider: &#34;claude&#34; (only implemented value); &#34;ollama:&lt;model&gt;&#34; reserved. |
| repo_url | [string](#string) |  |  |
| github_token | [string](#string) |  | github_token authorizes cloning (and, for Claude, opening a PR against) repo_url. Never persisted — used only to start the provider session. |
| branch | [string](#string) |  | branch to check out; defaults to the repository&#39;s default branch. |
| task_description | [string](#string) |  | task_description is what the agent should do. |
| rubric | [string](#string) |  | rubric is the grading criteria for the provider&#39;s outcome-graded loop, e.g. &#34;go build ./... succeeds, go test ./... passes, no TODO(agent) markers remain&#34;. |






<a name="schemas-greyseal-services-v1-RunAgentTaskResponse"></a>

### RunAgentTaskResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.AgentRun](#schemas-greyseal-v1-AgentRun) |  |  |






<a name="schemas-greyseal-services-v1-StreamAgentRunRequest"></a>

### StreamAgentRunRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |





 

 

 


<a name="schemas-greyseal-services-v1-AgentService"></a>

### AgentService


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| RunAgentTask | [RunAgentTaskRequest](#schemas-greyseal-services-v1-RunAgentTaskRequest) | [RunAgentTaskResponse](#schemas-greyseal-services-v1-RunAgentTaskResponse) | RunAgentTask starts a new agentic coding-task run and returns immediately with its initial state; the run continues asynchronously on the provider&#39;s side. |
| GetAgentRun | [GetAgentRunRequest](#schemas-greyseal-services-v1-GetAgentRunRequest) | [GetAgentRunResponse](#schemas-greyseal-services-v1-GetAgentRunResponse) | GetAgentRun returns the current state of a previously started run. |
| ListAgentRuns | [ListAgentRunsRequest](#schemas-greyseal-services-v1-ListAgentRunsRequest) | [ListAgentRunsResponse](#schemas-greyseal-services-v1-ListAgentRunsResponse) | ListAgentRuns returns runs newest-first, optionally filtered by status. |
| StreamAgentRun | [StreamAgentRunRequest](#schemas-greyseal-services-v1-StreamAgentRunRequest) | [AgentRunEvent](#schemas-greyseal-services-v1-AgentRunEvent) stream | StreamAgentRun streams events for a run as they occur, until the run reaches a terminal status. |

 



<a name="schemas_greyseal_v1_services_conversation-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## schemas/greyseal/v1/services/conversation.proto



<a name="schemas-greyseal-services-v1-ChatRequest"></a>

### ChatRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| conversation_uuid | [string](#string) |  |  |
| content | [string](#string) |  |  |






<a name="schemas-greyseal-services-v1-ChatResponse"></a>

### ChatResponse
ChatResponse is streamed; each message carries one token or chunk of content.
The final message in the stream includes the fully-populated Message with
resource references and uuid set.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| token | [string](#string) |  |  |
| final_message | [schemas.greyseal.v1.Message](#schemas-greyseal-v1-Message) |  | final_message is populated only on the last streamed response. |






<a name="schemas-greyseal-services-v1-CreateConversationRequest"></a>

### CreateConversationRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| title | [string](#string) |  | title is optional; left blank it can be auto-generated after the first exchange. |
| role_uuid | [string](#string) |  | role_uuid optionally assigns a Role system prompt to this conversation. |
| resource_uuids | [string](#string) | repeated | resource_uuids optionally scopes retrieval to specific resources. |






<a name="schemas-greyseal-services-v1-CreateConversationResponse"></a>

### CreateConversationResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Conversation](#schemas-greyseal-v1-Conversation) |  |  |






<a name="schemas-greyseal-services-v1-DeleteConversationRequest"></a>

### DeleteConversationRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |






<a name="schemas-greyseal-services-v1-DeleteConversationResponse"></a>

### DeleteConversationResponse







<a name="schemas-greyseal-services-v1-GetConversationRequest"></a>

### GetConversationRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |






<a name="schemas-greyseal-services-v1-GetConversationResponse"></a>

### GetConversationResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Conversation](#schemas-greyseal-v1-Conversation) |  |  |






<a name="schemas-greyseal-services-v1-ListConversationsRequest"></a>

### ListConversationsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| count | [int32](#int32) | optional |  |
| cursor | [string](#string) | optional |  |






<a name="schemas-greyseal-services-v1-ListConversationsResponse"></a>

### ListConversationsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Conversation](#schemas-greyseal-v1-Conversation) | repeated | Returns conversations without their messages for efficiency. |
| cursor | [string](#string) |  |  |
| count | [int32](#int32) |  |  |






<a name="schemas-greyseal-services-v1-SubmitFeedbackRequest"></a>

### SubmitFeedbackRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| message_uuid | [string](#string) |  |  |
| feedback | [int32](#int32) |  | feedback: -1 negative, 0 neutral, 1 positive. |






<a name="schemas-greyseal-services-v1-SubmitFeedbackResponse"></a>

### SubmitFeedbackResponse







<a name="schemas-greyseal-services-v1-UpdateConversationRequest"></a>

### UpdateConversationRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |
| title | [string](#string) | optional | Fields that can be mutated after creation. |
| role_uuid | [string](#string) | optional |  |
| resource_uuids | [string](#string) | repeated |  |






<a name="schemas-greyseal-services-v1-UpdateConversationResponse"></a>

### UpdateConversationResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Conversation](#schemas-greyseal-v1-Conversation) |  |  |





 

 

 


<a name="schemas-greyseal-services-v1-ConversationService"></a>

### ConversationService


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateConversation | [CreateConversationRequest](#schemas-greyseal-services-v1-CreateConversationRequest) | [CreateConversationResponse](#schemas-greyseal-services-v1-CreateConversationResponse) |  |
| GetConversation | [GetConversationRequest](#schemas-greyseal-services-v1-GetConversationRequest) | [GetConversationResponse](#schemas-greyseal-services-v1-GetConversationResponse) |  |
| ListConversations | [ListConversationsRequest](#schemas-greyseal-services-v1-ListConversationsRequest) | [ListConversationsResponse](#schemas-greyseal-services-v1-ListConversationsResponse) |  |
| UpdateConversation | [UpdateConversationRequest](#schemas-greyseal-services-v1-UpdateConversationRequest) | [UpdateConversationResponse](#schemas-greyseal-services-v1-UpdateConversationResponse) |  |
| DeleteConversation | [DeleteConversationRequest](#schemas-greyseal-services-v1-DeleteConversationRequest) | [DeleteConversationResponse](#schemas-greyseal-services-v1-DeleteConversationResponse) |  |
| Chat | [ChatRequest](#schemas-greyseal-services-v1-ChatRequest) | [ChatResponse](#schemas-greyseal-services-v1-ChatResponse) stream | Chat sends a user message and streams back the assistant response token by token. |
| SubmitFeedback | [SubmitFeedbackRequest](#schemas-greyseal-services-v1-SubmitFeedbackRequest) | [SubmitFeedbackResponse](#schemas-greyseal-services-v1-SubmitFeedbackResponse) | SubmitFeedback records user feedback on an assistant message. |

 



<a name="schemas_greyseal_v1_services_resource-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## schemas/greyseal/v1/services/resource.proto



<a name="schemas-greyseal-services-v1-DeleteResourceRequest"></a>

### DeleteResourceRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |






<a name="schemas-greyseal-services-v1-DeleteResourceResponse"></a>

### DeleteResourceResponse







<a name="schemas-greyseal-services-v1-GetResourceRequest"></a>

### GetResourceRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |






<a name="schemas-greyseal-services-v1-GetResourceResponse"></a>

### GetResourceResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Resource](#schemas-greyseal-v1-Resource) |  |  |






<a name="schemas-greyseal-services-v1-IngestResourceRequest"></a>

### IngestResourceRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Resource](#schemas-greyseal-v1-Resource) |  |  |






<a name="schemas-greyseal-services-v1-IngestResourceResponse"></a>

### IngestResourceResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Resource](#schemas-greyseal-v1-Resource) |  |  |






<a name="schemas-greyseal-services-v1-ListResourcesRequest"></a>

### ListResourcesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| count | [int32](#int32) | optional |  |
| cursor | [string](#string) | optional |  |






<a name="schemas-greyseal-services-v1-ListResourcesResponse"></a>

### ListResourcesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Resource](#schemas-greyseal-v1-Resource) | repeated |  |
| cursor | [string](#string) |  |  |
| count | [int32](#int32) |  |  |





 

 

 


<a name="schemas-greyseal-services-v1-ResourceService"></a>

### ResourceService


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| IngestResource | [IngestResourceRequest](#schemas-greyseal-services-v1-IngestResourceRequest) | [IngestResourceResponse](#schemas-greyseal-services-v1-IngestResourceResponse) | IngestResource ingests a magpie resource, chunks it, and stores embeddings. |
| GetResource | [GetResourceRequest](#schemas-greyseal-services-v1-GetResourceRequest) | [GetResourceResponse](#schemas-greyseal-services-v1-GetResourceResponse) |  |
| ListResources | [ListResourcesRequest](#schemas-greyseal-services-v1-ListResourcesRequest) | [ListResourcesResponse](#schemas-greyseal-services-v1-ListResourcesResponse) |  |
| DeleteResource | [DeleteResourceRequest](#schemas-greyseal-services-v1-DeleteResourceRequest) | [DeleteResourceResponse](#schemas-greyseal-services-v1-DeleteResourceResponse) |  |

 



<a name="schemas_greyseal_v1_services_role-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## schemas/greyseal/v1/services/role.proto



<a name="schemas-greyseal-services-v1-CreateRoleRequest"></a>

### CreateRoleRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Role](#schemas-greyseal-v1-Role) |  |  |






<a name="schemas-greyseal-services-v1-CreateRoleResponse"></a>

### CreateRoleResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Role](#schemas-greyseal-v1-Role) |  |  |






<a name="schemas-greyseal-services-v1-DeleteRoleRequest"></a>

### DeleteRoleRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |






<a name="schemas-greyseal-services-v1-DeleteRoleResponse"></a>

### DeleteRoleResponse







<a name="schemas-greyseal-services-v1-GetRoleRequest"></a>

### GetRoleRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |






<a name="schemas-greyseal-services-v1-GetRoleResponse"></a>

### GetRoleResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Role](#schemas-greyseal-v1-Role) |  |  |






<a name="schemas-greyseal-services-v1-ListRolesRequest"></a>

### ListRolesRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| count | [int32](#int32) | optional |  |
| cursor | [string](#string) | optional |  |






<a name="schemas-greyseal-services-v1-ListRolesResponse"></a>

### ListRolesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Role](#schemas-greyseal-v1-Role) | repeated |  |
| cursor | [string](#string) |  |  |
| count | [int32](#int32) |  |  |






<a name="schemas-greyseal-services-v1-UpdateRoleRequest"></a>

### UpdateRoleRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uuid | [string](#string) |  |  |
| data | [schemas.greyseal.v1.Role](#schemas-greyseal-v1-Role) |  |  |






<a name="schemas-greyseal-services-v1-UpdateRoleResponse"></a>

### UpdateRoleResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [schemas.greyseal.v1.Role](#schemas-greyseal-v1-Role) |  |  |





 

 

 


<a name="schemas-greyseal-services-v1-RoleService"></a>

### RoleService


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateRole | [CreateRoleRequest](#schemas-greyseal-services-v1-CreateRoleRequest) | [CreateRoleResponse](#schemas-greyseal-services-v1-CreateRoleResponse) |  |
| GetRole | [GetRoleRequest](#schemas-greyseal-services-v1-GetRoleRequest) | [GetRoleResponse](#schemas-greyseal-services-v1-GetRoleResponse) |  |
| ListRoles | [ListRolesRequest](#schemas-greyseal-services-v1-ListRolesRequest) | [ListRolesResponse](#schemas-greyseal-services-v1-ListRolesResponse) |  |
| UpdateRole | [UpdateRoleRequest](#schemas-greyseal-services-v1-UpdateRoleRequest) | [UpdateRoleResponse](#schemas-greyseal-services-v1-UpdateRoleResponse) |  |
| DeleteRole | [DeleteRoleRequest](#schemas-greyseal-services-v1-DeleteRoleRequest) | [DeleteRoleResponse](#schemas-greyseal-services-v1-DeleteRoleResponse) |  |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

