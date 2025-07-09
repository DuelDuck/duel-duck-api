package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"strings"
)

type Generic[M any, IDType uuid.UUID | string | uint64] struct {
	DB DB
}

func (r Generic[M, IDType]) WithTx(tx bun.Tx) Generic[M, IDType] {
	return Generic[M, IDType]{
		DB: r.DB.WithTx(tx),
	}
}

func NewGenericRepository[
	M any,
	IDType uuid.UUID | string | uint64,
](db *bun.DB) Generic[M, IDType] {
	return Generic[M, IDType]{
		DB: DBWrapper{DBConn: db},
	}
}

func (r Generic[M, IDType]) Create(ctx context.Context, model *M) error {
	_, err := r.DB.NewInsert().Model(model).Exec(ctx)
	return err
}

func (r Generic[M, IDType]) GetByID(ctx context.Context, id IDType) (*M, error) {
	model := new(M)
	err := r.DB.NewSelect().Model(model).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}

	return model, nil
}

func (r Generic[M, IDType]) FindAll(ctx context.Context) ([]M, error) {
	models := make([]M, 0)

	err := r.DB.NewSelect().
		Model(models).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return models, nil
}

func (r Generic[M, IDType]) Update(ctx context.Context, model *M) error {
	_, err := r.DB.NewUpdate().
		Model(model).
		OmitZero().
		WherePK().
		Exec(ctx)
	return err
}

func (r Generic[M, IDType]) Delete(ctx context.Context, id IDType) error {
	var model *M
	_, err := r.DB.NewDelete().Model(model).Where("id = ?", id).Exec(ctx)

	return err
}

func (r Generic[M, IDType]) Exists(ctx context.Context, model *M) (bool, error) {
	ok, err := r.DB.NewSelect().Model(model).WherePK().Exists(ctx)
	return ok, err
}

func (r Generic[M, IDType]) FindAllWithOptions(
	ctx context.Context,
	options *Options,
) ([]M, error) {
	models := make([]M, 0)

	q := r.DB.NewSelect().Model(&models)
	q = options.Apply(q)

	err := q.Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return models, nil
}

func (r Generic[M, IDType]) CountWithOptions(
	ctx context.Context,
	options *Options,
) (int, error) {
	q := r.DB.NewSelect().Model((*M)(nil))
	q = options.Apply(q)

	count, err := q.Count(ctx)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (o *Options) Apply(q *bun.SelectQuery) *bun.SelectQuery {
	if o != nil {
		q = o.ApplyFilters(q)
		q = o.ApplyOrderBy(q)
		q = o.ApplyPagination(q)
	}

	return q
}

func (o *Options) ApplyPagination(q *bun.SelectQuery) *bun.SelectQuery {
	if o == nil || q == nil {
		return q
	}

	if o.Pagination.IsValid() {
		q = q.Offset(int((o.Pagination.PageNum - 1) * o.Pagination.PageSize))
		q = q.Limit(int(o.Pagination.PageSize))
	}

	return q
}

func (o *Options) ApplyOrderBy(q *bun.SelectQuery) *bun.SelectQuery {
	if o == nil || q == nil {
		return q
	}

	if o.Order.IsValid() {
		q = q.OrderExpr("? ?", bun.Ident(o.Order.OrderBy), bun.Safe(o.Order.OrderType))
	}

	return q
}

func (o *Options) ApplyFilters(q *bun.SelectQuery) *bun.SelectQuery {
	if o == nil || q == nil {
		return q
	}

	for _, filter := range o.Filters {
		filter.Operator = strings.ToLower(filter.Operator)
		if !filter.isValid() {
			continue
		}

		if filter.Operator == "like" || filter.Operator == "ilike" {
			filter.Value = fmt.Sprintf("%%%s%%", filter.Value)
		}

		if filter.Operator == "is" || filter.Operator == "is not" {
			if strings.ToLower(filter.Operator) == "null" {
				continue
			}

			q = q.Where("? ? ?", bun.Ident(filter.Column), bun.Safe(filter.Operator), bun.Safe(filter.Value))
			continue
		}

		if filter.WhereOr {
			q = q.WhereOr("? ? ?", bun.Ident(filter.Column), bun.Safe(filter.Operator), filter.Value)
		} else {
			q = q.Where("? ? ?", bun.Ident(filter.Column), bun.Safe(filter.Operator), filter.Value)
		}
	}

	return q
}
