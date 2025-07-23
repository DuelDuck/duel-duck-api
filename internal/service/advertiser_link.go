package service

import (
	"context"
	"errors"

	"github.com/gagliardetto/solana-go"
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type AdvertiserLinkService struct {
	WalletService            *WalletService
	AdvertiserLinkRepository *repository.AdvertiserLinkRepository
	TransactionManager       *repo.TransactionManager

	RedirectDomain string
}

func NewAdvertiserLinkService(
	c *config.Config,
	walletService *WalletService,
	advertiserLinkRepository *repository.AdvertiserLinkRepository,
	transactionManager *repo.TransactionManager,
) *AdvertiserLinkService {
	return &AdvertiserLinkService{
		WalletService:            walletService,
		AdvertiserLinkRepository: advertiserLinkRepository,
		TransactionManager:       transactionManager,
		RedirectDomain:           c.App.RedirectDuelDuckDomain,
	}
}

func (s *AdvertiserLinkService) CreateAdvertiserLink(
	ctx context.Context,
	req *model.CreateAdvertiserLinkReq,
) (*model.AdvertiserLink, error) {

	baseLink := req.Link

	advertiserLink := model.NewAdvertiserLink(req, s.RedirectDomain)

	link, err := s.AdvertiserLinkRepository.
		FindAdvertiserLinkByNameAndLink(ctx, req.Name, baseLink)
	if err != nil {
		return nil, apperrors.Internal("failed to find advertiser link by name and link", err)
	}
	if link != nil {
		return nil, apperrors.AlreadyExist("advertiser link with the same name and link already exists")
	}

	advertiserLink, err = s.AdvertiserLinkRepository.Create(ctx, advertiserLink)
	if err != nil {
		return nil, apperrors.Internal("failed to create advertiser link", err)
	}

	return advertiserLink, nil
}

func (s *AdvertiserLinkService) GetAllAdvertiserLinks(
	ctx context.Context,
	req *model.GetAllAdvertiserLinksReq,
) ([]*model.AdvertiserLinkInfo, error) {
	opts := &req.Opts

	if !opts.Order.IsValid() {
		opts.Order = repo.Order{OrderBy: "created_at", OrderType: "desc"}
	}

	links, err := s.AdvertiserLinkRepository.GetAllAdvertiserLinks(ctx, opts)
	if err != nil {
		return nil, apperrors.Internal("failed to get all advertiser links", err)
	}

	for _, l := range links {
		var usersBalanceUSDC uint64 = 0
		for _, a := range l.UsersPublicAddresses {

			address, err := solana.PublicKeyFromBase58(a)
			if err != nil || address == ZeroValuePublicKey {
				return nil, apperrors.BadRequest("user public address is not valid solana address")
			}

			ata, _, err := solana.FindAssociatedTokenAddress(address, USDCMintAddress)
			if err != nil {
				return nil, apperrors.BadRequest("failed to get sender's associated token account", err)
			}

			balance, err := s.WalletService.GetTokenBalance(ctx, ata)
			if err != nil && !errors.Is(err, model.ErrAccountUnitialized) {
				return nil, err
			}
			usersBalanceUSDC += balance
		}

		l.UsersBalanceUSDC = float64(usersBalanceUSDC) / float64(USDCMintDecimals)
	}

	return links, nil
}

func (s *AdvertiserLinkService) EditAdvertiserLink(ctx context.Context,
	req *model.EditAdvertiserLinkReq,
) error {
	err := s.AdvertiserLinkRepository.Update(ctx, req)
	if err != nil {
		return apperrors.Internal("failed to edit duel", err)
	}

	return nil
}

func (s *AdvertiserLinkService) DeleteAdvertiserLink(
	ctx context.Context,
	linkID uuid.UUID,
) error {

	link, err := s.AdvertiserLinkRepository.FindAdvertiserLinkByID(ctx, linkID)
	if err != nil {
		return apperrors.Internal("failed to find advertiser link", err)
	}
	if link == nil {
		return apperrors.NotFound("advertiser link not found")
	}

	err = s.AdvertiserLinkRepository.Delete(ctx, linkID)
	if err != nil {
		return apperrors.Internal("failed to delete advertiser link", err)
	}

	return nil
}
