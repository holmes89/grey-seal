package pages

import (
	"context"
	"errors"
	"strings"
	"testing"

	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	"github.com/holmes89/grey-seal/lib/ui/pages/mocks"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ConversationListComponent
// ---------------------------------------------------------------------------

func TestConversationListComponent_LoadsConversations(t *testing.T) {
	svc := mocks.NewMockConversationService(t)
	svc.On("ListConversations", mock.Anything, int32(10)).Return(
		&servicesv1.ListConversationsResponse{
			Data: []*greysealv1.Conversation{
				{Uuid: "c-1", Title: "Planning Chat"},
				{Uuid: "c-2", Title: "Code Review"},
			},
		}, nil)

	comp := &ConversationListComponent{ConversationSvc: svc}
	items, err := comp.loadData(context.Background())

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "Planning Chat", items[0].Title)
}

func TestConversationListComponent_PropagatesServiceError(t *testing.T) {
	svc := mocks.NewMockConversationService(t)
	svc.On("ListConversations", mock.Anything, int32(10)).Return(nil, errors.New("service unavailable"))

	comp := &ConversationListComponent{ConversationSvc: svc}
	items, err := comp.loadData(context.Background())

	require.Error(t, err)
	assert.Equal(t, "service unavailable", err.Error())
	assert.Nil(t, items)
}

// ---------------------------------------------------------------------------
// ConversationCreateComponent
// ---------------------------------------------------------------------------

func TestConversationCreateComponent_BuildCreateRequest_PopulatesFields(t *testing.T) {
	comp := &ConversationCreateComponent{
		title:          "My Conversation",
		role_uuid:      "role-uuid-1",
		resource_uuids: "res-1,res-2",
	}

	req := comp.buildCreateRequest()

	require.NotNil(t, req)
	assert.Equal(t, "My Conversation", req.Title)
	assert.Equal(t, "role-uuid-1", req.RoleUuid)
	assert.Equal(t, strings.Split("res-1,res-2", ","), req.ResourceUuids)
}

func TestConversationCreateComponent_Render_HasCreateHeading(t *testing.T) {
	comp := &ConversationCreateComponent{}

	err := app.Match(app.H1().Text("Create Conversation"), comp.Render(), 0)
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// ConversationGetComponent
// ---------------------------------------------------------------------------

func TestConversationGetComponent_LoadsConversation(t *testing.T) {
	const convID = "conv-uuid-1"
	svc := mocks.NewMockConversationService(t)
	svc.On("GetConversation", mock.Anything, convID).Return(
		&servicesv1.GetConversationResponse{
			Data: &greysealv1.Conversation{Uuid: convID, Title: "Planning Chat"},
		}, nil)

	comp := &ConversationGetComponent{ConversationSvc: svc}
	conv, err := comp.loadData(context.Background(), convID)

	require.NoError(t, err)
	require.NotNil(t, conv)
	assert.Equal(t, "Planning Chat", conv.Title)
}

func TestConversationGetComponent_PropagatesNotFound(t *testing.T) {
	svc := mocks.NewMockConversationService(t)
	svc.On("GetConversation", mock.Anything, "missing-id").Return(nil, errors.New("not found"))

	comp := &ConversationGetComponent{ConversationSvc: svc}
	conv, err := comp.loadData(context.Background(), "missing-id")

	require.Error(t, err)
	assert.Equal(t, "not found", err.Error())
	assert.Nil(t, conv)
}

func TestConversationGetComponent_LoadData_ReturnsMessages(t *testing.T) {
	const convID = "conv-uuid-1"
	svc := mocks.NewMockConversationService(t)
	svc.On("GetConversation", mock.Anything, convID).Return(
		&servicesv1.GetConversationResponse{
			Data: &greysealv1.Conversation{
				Uuid:  convID,
				Title: "Planning Chat",
				Messages: []*greysealv1.Message{
					{Uuid: "m-1", Role: greysealv1.MessageRole_MESSAGE_ROLE_USER, Content: "hello"},
					{Uuid: "m-2", Role: greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT, Content: "hi there", ResourceUuids: []string{"r-1", "r-2"}},
				},
			},
		}, nil)

	comp := &ConversationGetComponent{ConversationSvc: svc}
	conv, err := comp.loadData(context.Background(), convID)

	require.NoError(t, err)
	require.Len(t, conv.Messages, 2)
	assert.Equal(t, greysealv1.MessageRole_MESSAGE_ROLE_USER, conv.Messages[0].Role)
	assert.Equal(t, greysealv1.MessageRole_MESSAGE_ROLE_ASSISTANT, conv.Messages[1].Role)
	assert.Equal(t, []string{"r-1", "r-2"}, conv.Messages[1].ResourceUuids)
}

// ---------------------------------------------------------------------------
// ConversationUpdateComponent
// ---------------------------------------------------------------------------

func TestConversationUpdateComponent_LoadsConversation(t *testing.T) {
	const convID = "conv-uuid-1"
	svc := mocks.NewMockConversationService(t)
	svc.On("GetConversation", mock.Anything, convID).Return(
		&servicesv1.GetConversationResponse{
			Data: &greysealv1.Conversation{Uuid: convID, Title: "Updated Chat"},
		}, nil)

	comp := &ConversationUpdateComponent{ConversationSvc: svc}
	conv, err := comp.loadData(context.Background(), convID)

	require.NoError(t, err)
	require.NotNil(t, conv)
	assert.Equal(t, "Updated Chat", conv.Title)
}

func TestConversationUpdateComponent_BuildUpdateRequest_PopulatesFields(t *testing.T) {
	comp := &ConversationUpdateComponent{
		id:             "conv-uuid-1",
		title:          "Updated Chat",
		role_uuid:      "role-uuid-1",
		resource_uuids: "res-1,res-2",
	}

	req := comp.buildUpdateRequest()

	require.NotNil(t, req)
	assert.Equal(t, "conv-uuid-1", req.Uuid)
	assert.Equal(t, "Updated Chat", req.GetTitle())
	assert.Equal(t, strings.Split("res-1,res-2", ","), req.ResourceUuids)
}

// ---------------------------------------------------------------------------
// RoleListComponent
// ---------------------------------------------------------------------------

func TestRoleListComponent_LoadsRoles(t *testing.T) {
	svc := mocks.NewMockRoleService(t)
	svc.On("ListRoles", mock.Anything, int32(10)).Return(
		&servicesv1.ListRolesResponse{
			Data: []*greysealv1.Role{
				{Uuid: "r-1", Name: "Assistant"},
				{Uuid: "r-2", Name: "Reviewer"},
			},
		}, nil)

	comp := &RoleListComponent{RoleSvc: svc}
	items, err := comp.loadData(context.Background())

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "Assistant", items[0].Name)
}

func TestRoleListComponent_PropagatesServiceError(t *testing.T) {
	svc := mocks.NewMockRoleService(t)
	svc.On("ListRoles", mock.Anything, int32(10)).Return(nil, errors.New("service unavailable"))

	comp := &RoleListComponent{RoleSvc: svc}
	items, err := comp.loadData(context.Background())

	require.Error(t, err)
	assert.Equal(t, "service unavailable", err.Error())
	assert.Nil(t, items)
}

// ---------------------------------------------------------------------------
// RoleCreateComponent
// ---------------------------------------------------------------------------

func TestRoleCreateComponent_BuildCreateRequest_PopulatesFields(t *testing.T) {
	comp := &RoleCreateComponent{
		name:          "Assistant",
		system_prompt: "You are a helpful assistant.",
	}

	req := comp.buildCreateRequest()

	require.NotNil(t, req.Data)
	assert.Equal(t, "Assistant", req.Data.Name)
	assert.Equal(t, "You are a helpful assistant.", req.Data.SystemPrompt)
	assert.NotNil(t, req.Data.CreatedAt)
}

func TestRoleCreateComponent_Render_HasCreateHeading(t *testing.T) {
	comp := &RoleCreateComponent{}

	err := app.Match(app.H1().Text("Create Role"), comp.Render(), 0)
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// RoleGetComponent
// ---------------------------------------------------------------------------

func TestRoleGetComponent_LoadsRole(t *testing.T) {
	const roleID = "role-uuid-1"
	svc := mocks.NewMockRoleService(t)
	svc.On("GetRole", mock.Anything, roleID).Return(
		&servicesv1.GetRoleResponse{
			Data: &greysealv1.Role{Uuid: roleID, Name: "Assistant"},
		}, nil)

	comp := &RoleGetComponent{RoleSvc: svc}
	role, err := comp.loadData(context.Background(), roleID)

	require.NoError(t, err)
	require.NotNil(t, role)
	assert.Equal(t, "Assistant", role.Name)
}

func TestRoleGetComponent_PropagatesNotFound(t *testing.T) {
	svc := mocks.NewMockRoleService(t)
	svc.On("GetRole", mock.Anything, "missing-id").Return(nil, errors.New("not found"))

	comp := &RoleGetComponent{RoleSvc: svc}
	role, err := comp.loadData(context.Background(), "missing-id")

	require.Error(t, err)
	assert.Equal(t, "not found", err.Error())
	assert.Nil(t, role)
}

// ---------------------------------------------------------------------------
// RoleUpdateComponent
// ---------------------------------------------------------------------------

func TestRoleUpdateComponent_LoadsRole(t *testing.T) {
	const roleID = "role-uuid-1"
	svc := mocks.NewMockRoleService(t)
	svc.On("GetRole", mock.Anything, roleID).Return(
		&servicesv1.GetRoleResponse{
			Data: &greysealv1.Role{Uuid: roleID, Name: "Updated Assistant"},
		}, nil)

	comp := &RoleUpdateComponent{RoleSvc: svc}
	role, err := comp.loadData(context.Background(), roleID)

	require.NoError(t, err)
	require.NotNil(t, role)
	assert.Equal(t, "Updated Assistant", role.Name)
}

func TestRoleUpdateComponent_BuildUpdateRequest_PopulatesFields(t *testing.T) {
	comp := &RoleUpdateComponent{
		id:            "role-uuid-1",
		name:          "Updated Assistant",
		system_prompt: "You are an updated assistant.",
	}

	req := comp.buildUpdateRequest()

	require.NotNil(t, req.Data)
	assert.Equal(t, "role-uuid-1", req.Uuid)
	assert.Equal(t, "Updated Assistant", req.Data.Name)
	assert.Equal(t, "You are an updated assistant.", req.Data.SystemPrompt)
}
