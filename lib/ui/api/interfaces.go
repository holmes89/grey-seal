package api

import (
	"context"

	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
)

type ConversationService interface {
	ListConversations(ctx context.Context, count int32) (*servicesv1.ListConversationsResponse, error)
	GetConversation(ctx context.Context, uuid string) (*servicesv1.GetConversationResponse, error)
	CreateConversation(ctx context.Context, req *servicesv1.CreateConversationRequest) (*servicesv1.CreateConversationResponse, error)
	UpdateConversation(ctx context.Context, uuid string, req *servicesv1.UpdateConversationRequest) (*servicesv1.UpdateConversationResponse, error)
	DeleteConversation(ctx context.Context, uuid string) error
}

type RoleService interface {
	ListRoles(ctx context.Context, count int32) (*servicesv1.ListRolesResponse, error)
	GetRole(ctx context.Context, uuid string) (*servicesv1.GetRoleResponse, error)
	CreateRole(ctx context.Context, req *servicesv1.CreateRoleRequest) (*servicesv1.CreateRoleResponse, error)
	UpdateRole(ctx context.Context, uuid string, req *servicesv1.UpdateRoleRequest) (*servicesv1.UpdateRoleResponse, error)
	DeleteRole(ctx context.Context, uuid string) error
}
