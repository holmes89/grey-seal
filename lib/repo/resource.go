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

type ResourceRepo struct {
	*Conn
}

var _ base.Repository[*Resource] = (*ResourceRepo)(nil)

func (r *ResourceRepo) Create(ctx context.Context, b *Resource) error {
	_, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).Insert("resources").
		Columns("uuid", "created_at", "service", "entity", "source", "path").
		Values(
			b.Uuid,
			b.CreatedAt.AsTime(),
			b.Service,
			b.Entity,
			b.Source.String(),
			b.Path).
		RunWith(r.conn).Exec()
	if err != nil {
		return err
	}

	return nil
}

func (r *ResourceRepo) Update(ctx context.Context, id string, b *Resource) error {
	query, args, err := sq.Update("resources").
		Set("service", b.Service).
		Set("entity", b.Entity).
		Set("source", b.Source.String()).
		Set("path", b.Path).
		Where(sq.Eq{"uuid": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *ResourceRepo) Delete(ctx context.Context, id string) error {
	query, args, err := sq.Delete("resources").
		Where(sq.Eq{"uuid": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *ResourceRepo) Get(ctx context.Context, id string) (*Resource, error) {
	resource := &Resource{}
	var created_atDt time.Time
	err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "created_at", "service", "entity", "path").
		From("resources").
		Where(sq.Eq{"uuid": id}).
		RunWith(r.conn).
		QueryRow().
		Scan(
			&resource.Uuid,
			&created_atDt,
			&resource.Service,
			&resource.Entity,
			&resource.Path,
		)
	if err != nil {
		fmt.Println("error getting resource", err)
		return nil, err
	}
	resource.CreatedAt = timestamppb.New(created_atDt)
	return resource, nil
}

func (r *ResourceRepo) List(ctx context.Context, cursor string, limit uint, filter map[string][]any) ([]*Resource, error) {
	var resources []*Resource

	rows, err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "created_at", "service", "entity", "path").
		From("resources").
		RunWith(r.conn).
		Query()
	if err != nil {
		fmt.Println("error listing resources", err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		resource := &Resource{}
		var created_atDt time.Time
		err := rows.Scan(
			&resource.Uuid,
			&created_atDt,
			&resource.Service,
			&resource.Entity,
			&resource.Path,
		)
		if err != nil {
			fmt.Println("error getting resource", err)
			return nil, err
		}
		resource.CreatedAt = timestamppb.New(created_atDt)
		resources = append(resources, resource)
	}
	return resources, nil
}
