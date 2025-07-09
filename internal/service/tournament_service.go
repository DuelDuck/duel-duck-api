package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type TournamentService struct {
	TournamentRepository *repository.TournamentRepository
	TransactionManager   *repo.TransactionManager
}

func NewTournamentService(
	tournamentRepository *repository.TournamentRepository,
	transactionManager *repo.TransactionManager,
) *TournamentService {
	return &TournamentService{
		TournamentRepository: tournamentRepository,
		TransactionManager:   transactionManager,
	}
}

func (s *TournamentService) GetAll(
	ctx context.Context,
	req *repo.Options,
) ([]model.Tournament, error) {
	tournaments, err := s.TournamentRepository.GetAll(ctx, req)
	if err != nil {
		return nil, apperrors.Internal("", err)
	}

	return tournaments, nil
}

func (s *TournamentService) Leaderboard(
	ctx context.Context,
	req *model.TournamentLeaderboardReq,
) ([]model.TournamentLeaderboard, int, error) {
	opts := &req.Opts
	if !opts.Order.IsValid() {
		opts.Order = repo.Order{OrderBy: "pnl", OrderType: "desc"}
	}

	leaders, leadersCount, err := s.TournamentRepository.Leaderboard(ctx, req.TournamentID, opts)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to get tournament leaderboard", err)
	}

	return leaders, leadersCount, nil
}

func (s *TournamentService) TournamentRank(
	ctx context.Context,
	userID uuid.UUID,
	req *model.TournamentLeaderboardReq,
) (*model.TournamentRank, error) {
	opts := &req.Opts
	if !opts.Order.IsValid() {
		opts.Order = repo.Order{OrderBy: "pnl", OrderType: "desc"}
	}

	leader, err := s.TournamentRepository.TournamentRank(ctx, req.TournamentID, userID, opts)
	if err != nil {
		return nil, apperrors.Internal("failed to get tournament leaderboard", err)
	}

	if leader == nil {
		return nil, apperrors.NotFound("leader not found in leaderboard")
	}

	return leader, nil
}

func (s *TournamentService) UserTournaments(
	ctx context.Context,
	opts *repo.Options,
	userID uuid.UUID,
) ([]model.TournamentUserShow, error) {
	tournaments, err := s.TournamentRepository.UserTournaments(ctx, opts, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to get tournaments user participate in", err)
	}

	return tournaments, nil
}

func (s *TournamentService) Create(
	ctx context.Context,
	data *model.TournamentCreateData,
	rewards model.TournamentRewards,
	userID uuid.UUID,
) error {
	tournament := model.TournamentByCreateReq(data, userID)

	for i := range len(rewards) {
		rewards[i].TournamentID = tournament.ID
	}

	err := s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		if err := s.TournamentRepository.WithTx(tx).Create(ctx, tournament); err != nil {
			return apperrors.Internal("failed to create tournament", err)
		}

		if len(rewards) > 0 {
			if err := s.TournamentRepository.WithTx(tx).CreateRewards(ctx, rewards); err != nil {
				return apperrors.Internal("failed to create tournament rewards", err)
			}
		}

		return nil
	})

	return err
}

func (s *TournamentService) CreateOffer(ctx context.Context, createOffer *model.TournamentOfferReq) (*model.TournamentOffer, error) {
	offer := model.NewTournamentOffer(createOffer)
	if err := s.TournamentRepository.CreateOffer(ctx, offer); err != nil {
		return nil, apperrors.Internal("failed to create tournament offer", err)
	}

	return offer, nil
}

func (s *TournamentService) GetAllOffers(
	ctx context.Context,
) ([]model.TournamentOffer, error) {
	offers, err := s.TournamentRepository.GetAllOffers(ctx)
	if err != nil {
		return nil, apperrors.Internal("failed to get tournament offers", err)
	}

	return offers, nil
}

func (s *TournamentService) Rewards(
	ctx context.Context,
	tournamentID uuid.UUID,
) (model.TournamentRewards, error) {
	rewards, err := s.TournamentRepository.Rewards(ctx, tournamentID)
	if err != nil {
		return nil, apperrors.Internal("failed to get tournament rewards", err)
	}

	return rewards, nil
}

func (s *TournamentService) UpdateRewards(
	ctx context.Context,
	rewards model.TournamentRewards,
	tournamentID uuid.UUID,
	userID uuid.UUID,
) error {
	ok, err := s.TournamentRepository.UserOwnsTournament(ctx, tournamentID, userID)
	if err != nil {
		return apperrors.Internal("failed to check if user owns a tournament", err)
	}

	if !ok {
		return apperrors.Forbidden("user does not own a tournament")
	}

	for i := range rewards {
		rewards[i].TournamentID = tournamentID
	}

	err = s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		if err = s.TournamentRepository.DeleteRewards(ctx, tournamentID); err != nil {
			return err
		}

		if len(rewards) > 0 {
			if err = s.TournamentRepository.WithTx(tx).CreateRewards(ctx, rewards); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return apperrors.Internal("failed to update tournament rewards", err)
	}

	return nil
}
