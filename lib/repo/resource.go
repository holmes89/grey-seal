package repo

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/holmes89/archaea/base"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ResourceRepo struct {
	*Conn
}

var _ base.Repository[*greysealv1.Resource] = (*ResourceRepo)(nil)

func (r *ResourceRepo) Create(ctx context.Context, b *greysealv1.Resource) error {
	_, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).Insert("resources").
		Columns("uuid", "name", "service", "entity", "source", "path", "created_at", "indexed_at").
		Values(
			b.Uuid,
			b.Name,
			b.Service,
			b.Entity,
			int32(b.Source),
			b.Path,
			b.CreatedAt.AsTime(),
			b.IndexedAt.AsTime()).
		RunWith(r.conn).Exec()
	return err
}

func (r *ResourceRepo) Update(ctx context.Context, id string, b *greysealv1.Resource) error {
	query, args, err := sq.Update("resources").
		Set("name", b.Name).
		Set("service", b.Service).
		Set("entity", b.Entity).
		Set("source", int32(b.Source)).
		Set("path", b.Path).
		Set("indexed_at", b.IndexedAt.AsTime()).
		Where(sq.Eq{"uuid": id}).
		PlaceholderFormat(sq.Dollar).
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
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *ResourceRepo) Get(ctx context.Context, id string) (*greysealv1.Resource, error) {
	resource := &greysealv1.Resource{}
	var sourceVal int32
	var createdAtDt time.Time
	var indexedAtDt time.Time
	err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "name", "service", "entity", "source", "path", "created_at", "indexed_at").
		From("resources").
		Where(sq.Eq{"uuid": id}).
		RunWith(r.conn).
		QueryRow().
		Scan(
			&resource.Uuid,
			&resource.Name,
			&resource.Service,
			&resource.Entity,
			&sourceVal,
			&resource.Path,
			&createdAtDt,
			&indexedAtDt,
		)
	if err != nil {
		fmt.Println("error getting resource", err)
		return nil, err
	}
	resource.Source = greysealv1.Source(sourceVal)
	resource.CreatedAt = timestamppb.New(createdAtDt)
	resource.IndexedAt = timestamppb.New(indexedAtDt)
	return resource, nil
}

func (r *ResourceRepo) List(ctx context.Context, cursor string, limit uint, filter map[string][]any) ([]*greysealv1.Resource, error) {
	var resources []*greysealv1.Resource

	rows, err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "name", "service", "entity", "source", "path", "created_at", "indexed_at").
		From("resources").
		RunWith(r.conn).
		Query()
	if err != nil {
		fmt.Println("error listing resources", err)
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	for rows.Next() {
		resource := &greysealv1.Resource{}
		var sourceVal int32
		var createdAtDt time.Time
		var indexedAtDt time.Time
		err := rows.Scan(
			&resource.Uuid,
			&resource.Name,
			&resource.Service,
			&resource.Entity,
			&sourceVal,
			&resource.Path,
			&createdAtDt,
			&indexedAtDt,
		)
		if err != nil {
			fmt.Println("error scanning resource", err)
			return nil, err
		}
		resource.Source = greysealv1.Source(sourceVal)
		resource.CreatedAt = timestamppb.New(createdAtDt)
		resource.IndexedAt = timestamppb.New(indexedAtDt)
		resources = append(resources, resource)
	}
	return resources, nil
}
