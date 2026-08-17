package repo

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	agentsvc "github.com/holmes89/grey-seal/lib/greyseal/agent"
	greysealv1 "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AgentRunRepo struct {
	*Conn
}

var _ agentsvc.AgentRunRepository = (*AgentRunRepo)(nil)

func (r *AgentRunRepo) Create(ctx context.Context, run *greysealv1.AgentRun) error {
	_, err := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).Insert("agent_runs").
		Columns("uuid", "provider", "repo_url", "status", "session_id", "pr_url", "error", "created_at", "updated_at").
		Values(
			run.Uuid,
			run.Provider,
			run.RepoUrl,
			run.Status,
			run.SessionId,
			run.PrUrl,
			run.Error,
			run.CreatedAt.AsTime(),
			run.UpdatedAt.AsTime(),
		).
		RunWith(r.conn).Exec()
	return err
}

func (r *AgentRunRepo) Update(ctx context.Context, id string, run *greysealv1.AgentRun) error {
	query, args, err := sq.Update("agent_runs").
		Set("status", run.Status).
		Set("pr_url", run.PrUrl).
		Set("error", run.Error).
		Set("updated_at", run.UpdatedAt.AsTime()).
		Where(sq.Eq{"uuid": id}).
		PlaceholderFormat(sq.Dollar).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(ctx, query, args...)
	return err
}

func (r *AgentRunRepo) Get(ctx context.Context, id string) (*greysealv1.AgentRun, error) {
	run := &greysealv1.AgentRun{}
	var createdAtDt, updatedAtDt time.Time
	err := sq.StatementBuilder.
		PlaceholderFormat(sq.Dollar).
		Select("uuid", "provider", "repo_url", "status", "session_id", "pr_url", "error", "created_at", "updated_at").
		From("agent_runs").
		Where(sq.Eq{"uuid": id}).
		RunWith(r.conn).
		QueryRow().
		Scan(
			&run.Uuid,
			&run.Provider,
			&run.RepoUrl,
			&run.Status,
			&run.SessionId,
			&run.PrUrl,
			&run.Error,
			&createdAtDt,
			&updatedAtDt,
		)
	if err != nil {
		return nil, err
	}
	run.CreatedAt = timestamppb.New(createdAtDt)
	run.UpdatedAt = timestamppb.New(updatedAtDt)
	return run, nil
}
