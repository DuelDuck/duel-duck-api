package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type DuelEntityRepository struct {
	repository.Generic[model.DuelEntity, uint64]
}

func NewDuelEntityRepository(
	genericRepository repository.Generic[model.DuelEntity, uint64],
) *DuelEntityRepository {
	return &DuelEntityRepository{
		Generic: genericRepository,
	}
}

func (r *DuelEntityRepository) WithTx(tx bun.Tx) *DuelEntityRepository {
	return &DuelEntityRepository{Generic: r.Generic.WithTx(tx)}
}

func (r *DuelEntityRepository) FindAll(
	ctx context.Context,
	options *repository.Options,
) ([]model.DuelEntity, error) {
	entities := make([]model.DuelEntity, 0)

	q := r.DB.NewSelect().
		Model(&entities).
		ColumnExpr("duel_entities.*").
		Group(
			"duel_entities.subtopic_id",
			"duel_entities.name",
			"duel_entities.id")
	q = options.Apply(q)

	err := q.Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return entities, nil
}

func (r *DuelEntityRepository) FindAllDuelSubtopics(
	ctx context.Context,
	options *repository.Options,
) ([]model.DuelSubtopic, error) {
	subtopics := make([]model.DuelSubtopic, 0)

	q := r.DB.NewSelect().
		Model(&subtopics).
		ColumnExpr("duel_subtopics.*").
		Group(
			"duel_subtopics.topic_id",
			"duel_subtopics.id")
	q = options.Apply(q)

	err := q.Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return subtopics, nil
}

func (r *DuelEntityRepository) Create(
	ctx context.Context,
	entity *model.DuelEntity,
) (*model.DuelEntity, error) {
	returning := new(model.DuelEntity)

	_, err := r.DB.NewInsert().
		Model(entity).
		Returning("*").
		Exec(ctx, returning)
	if err != nil {
		return nil, err
	}

	return returning, nil
}

func (r *DuelEntityRepository) CreateDuelSubtopic(
	ctx context.Context,
	subtopic *model.DuelSubtopic,
) (*model.DuelSubtopic, error) {
	returning := new(model.DuelSubtopic)

	_, err := r.DB.NewInsert().
		Model(subtopic).
		Returning("*").
		Exec(ctx, returning)
	if err != nil {
		return nil, err
	}

	return returning, nil
}

func (r *DuelEntityRepository) GetDuelSubtopic(
	ctx context.Context,
	subtopicID uint64,
) (*model.DuelSubtopic, error) {
	subtopic := new(model.DuelSubtopic)

	err := r.DB.NewSelect().
		Model(subtopic).
		Where("id = ?", subtopicID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return subtopic, nil
}

func (r *DuelEntityRepository) GetDuelEntityType(
	ctx context.Context,
	duelEntityTypeID uint64,
) (string, error) {
	var duelEntityType string

	err := r.DB.NewSelect().
		Table("duel_entity_types").
		Column("type").
		Where("id = ?", duelEntityTypeID).
		Scan(ctx, &duelEntityType)

	if err != nil {
		return "", err
	}

	return duelEntityType, nil
}

func (r *DuelEntityRepository) UpdateDuelSubtopic(
	ctx context.Context,
	subtopic *model.DuelSubtopic,
) error {
	_, err := r.DB.NewUpdate().
		Model(subtopic).
		OmitZero().
		WherePK().
		Exec(ctx)

	return err
}
