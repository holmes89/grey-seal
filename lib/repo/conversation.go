package repo

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"github.com/lib/pq"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MessageRepo handles persistence of individual chat messages.
type MessageRepo struct {
	*Conn
}

var _ base.Repository[*Message] = (*MessageRepo)(nil)

func (r *MessageRepo) Create(ctx context.Context, b *Message) error {
	resourceUUIDs := b.ResourceUuids
	if resourceUUIDs == nil {
		resourceUUIDs = []string{}
	}
	_, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).Insert("messages").
		Columns("uuid", "conversation_uuid", "role", "content", "resource_uuids", "feedback", "created_at").
		Values(
			b.Uuid,
			b.ConversationUuid,
			int32(b.Role),
			b.Content,
			pq.Array(resourceUUIDs),
			b.Feedback,
			b.CreatedAt.AsTime()).
		RunWith(r.conn).Exec()
	return err
}

func (r *MessageRepo) Update(ctx context.Context, id string, b *Message) error {
	resourceUUIDs := b.ResourceUuids
	if resourceUUIDs == nil {
		resourceUUIDs = []string{}
	}
	query, args, err := sq.Update("messages").
		Set("role", int32(b.Role)).
		Set("content", b.Content).
		Set("resource_uuids", pq.Array(resourceUUIDs)).
		Set("feedback", b.Feedback).
		Where(sq.Eq{"uuid": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *MessageRepo) Delete(ctx context.Context, id string) error {
	query, args, err := sq.Delete("messages").
		Where(sq.Eq{"uuid": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *MessageRepo) Get(ctx context.Context, id string) (*Message, error) {
	message := &Message{}
	var roleVal int32
	var createdAtDt time.Time
	err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "conversation_uuid", "role", "content", "resource_uuids", "feedback", "created_at").
		From("messages").
		Where(sq.Eq{"uuid": id}).
		RunWith(r.conn).
		QueryRow().
		Scan(
			&message.Uuid,
			&message.ConversationUuid,
			&roleVal,
			&message.Content,
			pq.Array(&message.ResourceUuids),
			&message.Feedback,
			&createdAtDt,
		)
	if err != nil {
		fmt.Println("error getting message", err)
		return nil, err
	}
	message.Role = MessageRole(roleVal)
	message.CreatedAt = timestamppb.New(createdAtDt)
	return message, nil
}

func (r *MessageRepo) List(ctx context.Context, cursor string, limit uint, filter map[string][]any) ([]*Message, error) {
	q := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "conversation_uuid", "role", "content", "resource_uuids", "feedback", "created_at").
		From("messages").
		OrderBy("created_at ASC")

	if convUUIDs, ok := filter["conversation_uuid"]; ok && len(convUUIDs) > 0 {
		q = q.Where(sq.Eq{"conversation_uuid": convUUIDs[0]})
	}

	rows, err := q.RunWith(r.conn).Query()
	if err != nil {
		fmt.Println("error listing messages", err)
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		message := &Message{}
		var roleVal int32
		var createdAtDt time.Time
		err := rows.Scan(
			&message.Uuid,
			&message.ConversationUuid,
			&roleVal,
			&message.Content,
			pq.Array(&message.ResourceUuids),
			&message.Feedback,
			&createdAtDt,
		)
		if err != nil {
			fmt.Println("error scanning message", err)
			return nil, err
		}
		message.Role = MessageRole(roleVal)
		message.CreatedAt = timestamppb.New(createdAtDt)
		messages = append(messages, message)
	}
	return messages, nil
}

// ListByConversation fetches all messages for a given conversation UUID ordered by created_at.
func (r *MessageRepo) ListByConversation(ctx context.Context, conversationUUID string) ([]*Message, error) {
	return r.List(ctx, "", 0, map[string][]any{"conversation_uuid": {conversationUUID}})
}

// UpdateFeedback sets the feedback value on a single message.
func (r *MessageRepo) UpdateFeedback(ctx context.Context, messageUUID string, feedback int32) error {
	query, args, err := sq.Update("messages").
		Set("feedback", feedback).
		Where(sq.Eq{"uuid": messageUUID}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

// ConversationRepo handles persistence of conversation sessions.
// Messages are stored in their own table and loaded separately.
type ConversationRepo struct {
	*Conn
	messages *MessageRepo
}

func NewConversationRepo(conn *Conn) *ConversationRepo {
	return &ConversationRepo{
		Conn:     conn,
		messages: &MessageRepo{Conn: conn},
	}
}

var _ base.Repository[*Conversation] = (*ConversationRepo)(nil)

func (r *ConversationRepo) Create(ctx context.Context, b *Conversation) error {
	resourceUUIDs := b.ResourceUuids
	if resourceUUIDs == nil {
		resourceUUIDs = []string{}
	}
	_, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).Insert("conversations").
		Columns("uuid", "title", "role_uuid", "resource_uuids", "summary", "created_at", "updated_at").
		Values(
			b.Uuid,
			b.Title,
			b.RoleUuid,
			pq.Array(resourceUUIDs),
			b.Summary,
			b.CreatedAt.AsTime(),
			b.UpdatedAt.AsTime()).
		RunWith(r.conn).Exec()
	return err
}

func (r *ConversationRepo) Update(ctx context.Context, id string, b *Conversation) error {
	resourceUUIDs := b.ResourceUuids
	if resourceUUIDs == nil {
		resourceUUIDs = []string{}
	}
	query, args, err := sq.Update("conversations").
		Set("title", b.Title).
		Set("role_uuid", b.RoleUuid).
		Set("resource_uuids", pq.Array(resourceUUIDs)).
		Set("summary", b.Summary).
		Set("updated_at", b.UpdatedAt.AsTime()).
		Where(sq.Eq{"uuid": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *ConversationRepo) Delete(ctx context.Context, id string) error {
	query, args, err := sq.Delete("conversations").
		Where(sq.Eq{"uuid": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *ConversationRepo) Get(ctx context.Context, id string) (*Conversation, error) {
	conversation := &Conversation{}
	var createdAtDt time.Time
	var updatedAtDt time.Time
	err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "title", "role_uuid", "resource_uuids", "summary", "created_at", "updated_at").
		From("conversations").
		Where(sq.Eq{"uuid": id}).
		RunWith(r.conn).
		QueryRow().
		Scan(
			&conversation.Uuid,
			&conversation.Title,
			&conversation.RoleUuid,
			pq.Array(&conversation.ResourceUuids),
			&conversation.Summary,
			&createdAtDt,
			&updatedAtDt,
		)
	if err != nil {
		fmt.Println("error getting conversation", err)
		return nil, err
	}
	conversation.CreatedAt = timestamppb.New(createdAtDt)
	conversation.UpdatedAt = timestamppb.New(updatedAtDt)

	msgs, err := r.messages.ListByConversation(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load messages: %w", err)
	}
	conversation.Messages = msgs
	return conversation, nil
}

func (r *ConversationRepo) List(ctx context.Context, cursor string, limit uint, filter map[string][]any) ([]*Conversation, error) {
	rows, err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "title", "role_uuid", "resource_uuids", "summary", "created_at", "updated_at").
		From("conversations").
		OrderBy("updated_at DESC").
		RunWith(r.conn).
		Query()
	if err != nil {
		fmt.Println("error listing conversations", err)
		return nil, err
	}
	defer rows.Close()

	var conversations []*Conversation
	for rows.Next() {
		conversation := &Conversation{}
		var createdAtDt time.Time
		var updatedAtDt time.Time
		err := rows.Scan(
			&conversation.Uuid,
			&conversation.Title,
			&conversation.RoleUuid,
			pq.Array(&conversation.ResourceUuids),
			&conversation.Summary,
			&createdAtDt,
			&updatedAtDt,
		)
		if err != nil {
			fmt.Println("error scanning conversation", err)
			return nil, err
		}
		conversation.CreatedAt = timestamppb.New(createdAtDt)
		conversation.UpdatedAt = timestamppb.New(updatedAtDt)
		conversations = append(conversations, conversation)
	}
	return conversations, nil
}

