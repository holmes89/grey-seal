package repo

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RoleRepo struct {
	*Conn
}

var _ base.Repository[*Role] = (*RoleRepo)(nil)

func (r *RoleRepo) Create(ctx context.Context, b *Role) error {
	_, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).Insert("roles").
		Columns("uuid", "name", "system_prompt", "created_at").
		Values(
			b.Uuid,
			b.Name,
			b.SystemPrompt,
			b.CreatedAt.AsTime()).
		RunWith(r.conn).Exec()
	if err != nil {
		return err
	}

	return nil
}

func (r *RoleRepo) Update(ctx context.Context, id string, b *Role) error {
	query, args, err := sq.Update("roles").
		Set("name", b.Name).
		Set("system_prompt", b.SystemPrompt).
		Where(sq.Eq{"uuid": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *RoleRepo) Delete(ctx context.Context, id string) error {
	query, args, err := sq.Delete("roles").
		Where(sq.Eq{"uuid": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *RoleRepo) Get(ctx context.Context, id string) (*Role, error) {
	role := &Role{}
	var created_atDt time.Time
	err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "name", "system_prompt", "created_at").
		From("roles").
		Where(sq.Eq{"uuid": id}).
		RunWith(r.conn).
		QueryRow().
		Scan(
			&role.Uuid,
			&role.Name,
			&role.SystemPrompt,
			&created_atDt,
		)
	if err != nil {
		fmt.Println("error getting role", err)
		return nil, err
	}
	role.CreatedAt = timestamppb.New(created_atDt)
	return role, nil
}

func (r *RoleRepo) List(ctx context.Context, cursor string, limit uint, filter map[string][]any) ([]*Role, error) {
	var roles []*Role

	rows, err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "name", "system_prompt", "created_at").
		From("roles").
		RunWith(r.conn).
		Query()
	if err != nil {
		fmt.Println("error listing roles", err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		role := &Role{}
		var created_atDt time.Time
		err := rows.Scan(
			&role.Uuid,
			&role.Name,
			&role.SystemPrompt,
			&created_atDt,
		)
		if err != nil {
			fmt.Println("error getting role", err)
			return nil, err
		}
		role.CreatedAt = timestamppb.New(created_atDt)
		roles = append(roles, role)
	}
	return roles, nil
}

