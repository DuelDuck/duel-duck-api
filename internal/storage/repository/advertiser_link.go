package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type AdvertiserLinkRepository struct {
	DB repository.DB
}

func NewAdvertiserLinkRepository(
	db repository.DB,
) *AdvertiserLinkRepository {
	return &AdvertiserLinkRepository{
		DB: db,
	}
}

func (r *AdvertiserLinkRepository) WithTx(tx bun.Tx) *AdvertiserLinkRepository {
	return &AdvertiserLinkRepository{DB: r.DB.WithTx(tx)}
}

func (r *AdvertiserLinkRepository) Update(
	ctx context.Context,
	link *model.EditAdvertiserLinkReq,
) error {
	_, err := r.DB.NewUpdate().
		Model(link).
		OmitZero().
		WherePK().
		Exec(ctx)

	return err
}

func (r *AdvertiserLinkRepository) Create(
	ctx context.Context,
	link *model.AdvertiserLink,
) (*model.AdvertiserLink, error) {
	_, err := r.DB.NewInsert().
		Model(link).
		Returning("*").
		Exec(ctx, link)

	if err != nil {
		return nil, err
	}

	return link, nil
}

func (r *AdvertiserLinkRepository) Delete(
	ctx context.Context,
	linkID uuid.UUID,
) error {
	_, err := r.DB.NewDelete().
		Model((*model.AdvertiserLink)(nil)).
		Where("id = ?", linkID).
		Exec(ctx)

	return err
}

func (r *AdvertiserLinkRepository) FindAdvertiserLinkByNameAndLink(
	ctx context.Context,
	name string,
	link string,
) (*model.AdvertiserLink, error) {
	var advertiserLink = new(model.AdvertiserLink)
	fmt.Println(name, " ", link)
	err := r.DB.NewSelect().
		Model(advertiserLink).
		Where("name LIKE ?", name).
		Where("redirected_link LIKE ? || '?%'", link).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return advertiserLink, nil
}

func (r *AdvertiserLinkRepository) FindAdvertiserLinkByToken(
	ctx context.Context,
	token string,
) (*model.AdvertiserLink, error) {
	var advertiserLink = new(model.AdvertiserLink)

	err := r.DB.NewSelect().
		Model(advertiserLink).
		Where("token = ?", token).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return advertiserLink, nil
}

func (r *AdvertiserLinkRepository) FindAdvertiserLinkByID(
	ctx context.Context,
	linkID uuid.UUID,
) (*model.AdvertiserLink, error) {
	var advertiserLink = new(model.AdvertiserLink)

	err := r.DB.NewSelect().
		Model(advertiserLink).
		Where("id = ?", linkID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return advertiserLink, nil
}

func (r *AdvertiserLinkRepository) AttachUserToAdvertiserLink(
	ctx context.Context,
	linkReferral *model.AdvertiserLinkReferrals,
) error {

	_, err := r.DB.NewInsert().
		Model(linkReferral).
		Exec(ctx)

	return err
}

func (r *AdvertiserLinkRepository) GetAllAdvertiserLinks(
	ctx context.Context,
	opts *repository.Options,
) ([]*model.AdvertiserLinkInfo, error) {
	var linksInfo []*model.AdvertiserLinkInfo

	q := r.DB.NewSelect().
		Model((*model.AdvertiserLinkInfo)(nil)).
		TableExpr("advertiser_links AS al").
		ColumnExpr("al.id").
		ColumnExpr("al.link").
		ColumnExpr("al.redirected_link").
		ColumnExpr("al.name").
		ColumnExpr("al.visitors_count").
		ColumnExpr("al.created_at").
		ColumnExpr("al.updated_at").
		ColumnExpr(`(
		SELECT COUNT(*)
		FROM user_referrals ur
		WHERE ur.referrer_id IN (
			SELECT alr.user_id
			FROM advertiser_link_referrals alr
			WHERE alr.link_id = al.id
		)
	) AS referrals_count`).
		ColumnExpr(`(
		SELECT COUNT(*)
		FROM users u
		WHERE u.id IN (
			SELECT alr.user_id FROM advertiser_link_referrals alr WHERE alr.link_id = al.id
		)
	) AS registered_users_count`).
		ColumnExpr(`(
		SELECT COUNT(DISTINCT p.user_id)
		FROM players p
		WHERE p.user_id IN (
			SELECT alr.user_id FROM advertiser_link_referrals alr WHERE alr.link_id = al.id
		)
		AND p.created_at >= NOW() - INTERVAL '30 days'
	) AS active_users_count`).
		ColumnExpr(`(
		SELECT COALESCE(SUM(u.balance), 0)
		FROM users u
		WHERE u.id IN (
			SELECT alr.user_id FROM advertiser_link_referrals alr WHERE alr.link_id = al.id
		)
	) AS users_balance_ddp`).
		ColumnExpr(`(
		SELECT COUNT(*)
		FROM duels d
		WHERE d.payment_type = 0
		AND d.owner_id IN (
			SELECT alr.user_id FROM advertiser_link_referrals alr WHERE alr.link_id = al.id
		)
	) AS duels_ddp_count`).
		ColumnExpr(`(
		SELECT COUNT(*)
		FROM duels d
		WHERE d.owner_id IN (
			SELECT alr.user_id FROM advertiser_link_referrals alr WHERE alr.link_id = al.id
		)
	) AS duels_created_count`).
		ColumnExpr(`(
		SELECT COALESCE(json_agg(u.public_address), '[]')
		FROM advertiser_link_referrals alr
		JOIN users u ON u.id = alr.user_id
		WHERE alr.link_id = al.id
	) AS users_public_addresses`).
		GroupExpr("al.id")

	q = opts.Apply(q)

	err := q.Scan(ctx, &linksInfo)
	if err != nil {
		return nil, err
	}

	return linksInfo, nil
}
