package repo

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

type QuestionRepo struct {
	*Conn
}

var _ base.Repository[*Question] = (*QuestionRepo)(nil)

func (r *QuestionRepo) Create(ctx context.Context, b *Question) error {
	_, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).Insert("questions").
		Columns("uuid", "role_description", "content").
		Values(
			b.Uuid,
			b.RoleDescription,
			b.Content).
		RunWith(r.conn).Exec()
	if err != nil {
		return err
	}

	return nil
}

func (r *QuestionRepo) Update(ctx context.Context, id string, b *Question) error {
	query, args, err := sq.Update("questions").
		Set("role_description", b.RoleDescription).
		Set("content", b.Content).
		Where(sq.Eq{"uuid": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *QuestionRepo) Delete(ctx context.Context, id string) error {
	query, args, err := sq.Delete("questions").
		Where(sq.Eq{"uuid": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *QuestionRepo) SaveResponse(ctx context.Context, questionUUID, response string, references []string) error {
	_, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).Insert("question_responses").
		Columns("question_uuid", "response").
		Values(
			questionUUID,
			response).
		RunWith(r.conn).Exec()
	if err != nil {
		return err
	}
	var builder sq.InsertBuilder
	for _, ref := range references {
		builder = sq.StatementBuilder.PlaceholderFormat(sq.Dollar).Insert("question_references").
			Columns("question_uuid", "resource_uuid").
			Values(
				questionUUID,
				ref)
	}
	_, err = builder.RunWith(r.conn).Exec()
	if err != nil {
		return err
	}
	return nil
}

func (r *QuestionRepo) Get(ctx context.Context, id string) (*Question, error) {
	question := &Question{}
	err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "role_description", "content").
		From("questions").
		Where(sq.Eq{"uuid": id}).
		RunWith(r.conn).
		QueryRow().
		Scan(
			&question.Uuid,
			&question.RoleDescription,
			&question.Content,
		)
	if err != nil {
		fmt.Println("error getting question", err)
		return nil, err
	}
	return question, nil
}

func (r *QuestionRepo) List(ctx context.Context, cursor string, limit uint, filter map[string][]any) ([]*Question, error) {
	var questions []*Question

	rows, err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "role_description", "content").
		From("questions").
		RunWith(r.conn).
		Query()
	if err != nil {
		fmt.Println("error listing questions", err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		question := &Question{}
		err := rows.Scan(
			&question.Uuid,
			&question.RoleDescription,
			&question.Content,
		)
		if err != nil {
			fmt.Println("error getting question", err)
			return nil, err
		}
		questions = append(questions, question)
	}
	return questions, nil
}
