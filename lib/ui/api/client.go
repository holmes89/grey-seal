//go:build ignore

package api

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	servicesv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services"
	servicesv1connect "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1/services/servicesv1connect"
	"github.com/holmes89/grey-seal/lib/ui/config"
)

// Client wraps all the Connect-Go service clients
type Client struct {
	MessageClient servicesv1connect.MessageServiceClient
	ConversationClient servicesv1connect.ConversationServiceClient
	ResourceClient servicesv1connect.ResourceServiceClient
	RoleClient servicesv1connect.RoleServiceClient
}

// New creates a new API client with the given base URL
func New(baseURL string) *Client {
	httpClient := &http.Client{}

	return &Client{
		MessageClient: servicesv1connect.NewMessageServiceClient(httpClient, baseURL),
		ConversationClient: servicesv1connect.NewConversationServiceClient(httpClient, baseURL),
		ResourceClient: servicesv1connect.NewResourceServiceClient(httpClient, baseURL),
		RoleClient: servicesv1connect.NewRoleServiceClient(httpClient, baseURL),
	}
}

// Default client instance
var defaultClient *Client

func getClient() *Client {
	if defaultClient == nil {
		defaultClient = New(config.ApiEndpoint)
	}
	return defaultClient
}

// Message service methods

func ListMessages(ctx context.Context, count int32) (*servicesv1.ListMessagesResponse, error) {
	req := connect.NewRequest(&servicesv1.ListMessagesRequest{
		Count: &count,
	})
	resp, err := getClient().MessageClient.ListMessages(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func GetMessage(ctx context.Context, uuid string) (*servicesv1.GetMessageResponse, error) {
	req := connect.NewRequest(&servicesv1.GetMessageRequest{
		Uuid: uuid,
	})
	resp, err := getClient().MessageClient.GetMessage(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func CreateMessage(ctx context.Context, data *servicesv1.CreateMessageRequest) (*servicesv1.CreateMessageResponse, error) {
	req := connect.NewRequest(data)
	resp, err := getClient().MessageClient.CreateMessage(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func UpdateMessage(ctx context.Context, uuid string, data *servicesv1.UpdateMessageRequest) (*servicesv1.UpdateMessageResponse, error) {
	data.Uuid = uuid
	req := connect.NewRequest(data)
	resp, err := getClient().MessageClient.UpdateMessage(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func DeleteMessage(ctx context.Context, uuid string) error {
	req := connect.NewRequest(&servicesv1.DeleteMessageRequest{
		Uuid: uuid,
	})
	_, err := getClient().MessageClient.DeleteMessage(ctx, req)
	return err
}

// Conversation service methods

func ListConversations(ctx context.Context, count int32) (*servicesv1.ListConversationsResponse, error) {
	req := connect.NewRequest(&servicesv1.ListConversationsRequest{
		Count: &count,
	})
	resp, err := getClient().ConversationClient.ListConversations(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func GetConversation(ctx context.Context, uuid string) (*servicesv1.GetConversationResponse, error) {
	req := connect.NewRequest(&servicesv1.GetConversationRequest{
		Uuid: uuid,
	})
	resp, err := getClient().ConversationClient.GetConversation(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func CreateConversation(ctx context.Context, data *servicesv1.CreateConversationRequest) (*servicesv1.CreateConversationResponse, error) {
	req := connect.NewRequest(data)
	resp, err := getClient().ConversationClient.CreateConversation(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func UpdateConversation(ctx context.Context, uuid string, data *servicesv1.UpdateConversationRequest) (*servicesv1.UpdateConversationResponse, error) {
	data.Uuid = uuid
	req := connect.NewRequest(data)
	resp, err := getClient().ConversationClient.UpdateConversation(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func DeleteConversation(ctx context.Context, uuid string) error {
	req := connect.NewRequest(&servicesv1.DeleteConversationRequest{
		Uuid: uuid,
	})
	_, err := getClient().ConversationClient.DeleteConversation(ctx, req)
	return err
}

// Resource service methods

func ListResources(ctx context.Context, count int32) (*servicesv1.ListResourcesResponse, error) {
	req := connect.NewRequest(&servicesv1.ListResourcesRequest{
		Count: &count,
	})
	resp, err := getClient().ResourceClient.ListResources(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func GetResource(ctx context.Context, uuid string) (*servicesv1.GetResourceResponse, error) {
	req := connect.NewRequest(&servicesv1.GetResourceRequest{
		Uuid: uuid,
	})
	resp, err := getClient().ResourceClient.GetResource(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func CreateResource(ctx context.Context, data *servicesv1.CreateResourceRequest) (*servicesv1.CreateResourceResponse, error) {
	req := connect.NewRequest(data)
	resp, err := getClient().ResourceClient.CreateResource(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func UpdateResource(ctx context.Context, uuid string, data *servicesv1.UpdateResourceRequest) (*servicesv1.UpdateResourceResponse, error) {
	data.Uuid = uuid
	req := connect.NewRequest(data)
	resp, err := getClient().ResourceClient.UpdateResource(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func DeleteResource(ctx context.Context, uuid string) error {
	req := connect.NewRequest(&servicesv1.DeleteResourceRequest{
		Uuid: uuid,
	})
	_, err := getClient().ResourceClient.DeleteResource(ctx, req)
	return err
}

// Role service methods

func ListRoles(ctx context.Context, count int32) (*servicesv1.ListRolesResponse, error) {
	req := connect.NewRequest(&servicesv1.ListRolesRequest{
		Count: &count,
	})
	resp, err := getClient().RoleClient.ListRoles(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func GetRole(ctx context.Context, uuid string) (*servicesv1.GetRoleResponse, error) {
	req := connect.NewRequest(&servicesv1.GetRoleRequest{
		Uuid: uuid,
	})
	resp, err := getClient().RoleClient.GetRole(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func CreateRole(ctx context.Context, data *servicesv1.CreateRoleRequest) (*servicesv1.CreateRoleResponse, error) {
	req := connect.NewRequest(data)
	resp, err := getClient().RoleClient.CreateRole(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func UpdateRole(ctx context.Context, uuid string, data *servicesv1.UpdateRoleRequest) (*servicesv1.UpdateRoleResponse, error) {
	data.Uuid = uuid
	req := connect.NewRequest(data)
	resp, err := getClient().RoleClient.UpdateRole(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func DeleteRole(ctx context.Context, uuid string) error {
	req := connect.NewRequest(&servicesv1.DeleteRoleRequest{
		Uuid: uuid,
	})
	_, err := getClient().RoleClient.DeleteRole(ctx, req)
	return err
}

