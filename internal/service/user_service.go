package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"mime/multipart"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/cypher"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
	"go.uber.org/zap"
)

const (
	referralTokenLength = 8
)

type UserService struct {
	FileService                *FileService
	TaskService                *TaskService
	UserRepository             *repository.UserRepository
	ReferralRepository         *repository.ReferralRepository
	WalletRepository           *repository.WalletRepository
	WalletTokenRepository      *repository.WalletTokenRepository
	privateKeyRepository       *cypher.PrivateKeyRepository
	TransactionManager         *repo.TransactionManager
	referrerInvitationRewardDP uint64
}

func NewUserService(
	c *config.Config,
	fileService *FileService,
	taskService *TaskService,
	userRepository *repository.UserRepository,
	referralRepository *repository.ReferralRepository,
	walletRepository *repository.WalletRepository,
	walletTokenRepository *repository.WalletTokenRepository,
	privateKeyRepository *cypher.PrivateKeyRepository,
	transactionManager *repo.TransactionManager,
) *UserService {
	return &UserService{
		FileService:                fileService,
		TaskService:                taskService,
		UserRepository:             userRepository,
		ReferralRepository:         referralRepository,
		WalletRepository:           walletRepository,
		WalletTokenRepository:      walletTokenRepository,
		privateKeyRepository:       privateKeyRepository,
		TransactionManager:         transactionManager,
		referrerInvitationRewardDP: c.App.ReferrerInvitationRewardDP,
	}
}

func (s *UserService) CreateWithEmail(
	ctx context.Context,
	email mtype.Email,
	referrerToken string,
) (*model.User, error) {
	user, err := s.createUserWithEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	return s.createUser(ctx, user, referrerToken)
}

func (s *UserService) createUser(
	ctx context.Context,
	user *model.User,
	referrerToken string,
) (*model.User, error) {
	isReferral := false
	if referrerToken != "" {
		referrer, err := s.UserRepository.FindByReferralToken(ctx, referrerToken)
		if err != nil {
			return nil, apperrors.Internal("failed to find referrer", err)
		}

		if referrer != nil {
			isReferral = true

			if ok := user.Balance.Add(int(s.referrerInvitationRewardDP)); !ok {
				return nil, apperrors.Internal("failed to alter users balance")
			}
		}
	}

	mnemonic, err := generateMnemonic()
	if err != nil {
		return nil, err
	}

	privateKey, err := generatePrivateKeyFromMnemonic(mnemonic)
	if err != nil || privateKey == nil {
		return nil, apperrors.Internal("failed to generate wallet from mnemonic", err)
	}

	err = s.privateKeyRepository.WritePrivateKey(ctx, user.ID, mnemonic, ed25519.PrivateKey(privateKey))
	if err != nil {
		return nil, apperrors.Internal("failed to store private key", err)
	}

	user.PublicAddress = privateKey.PublicKey().String()
	walletTokens := DefaultWalletTokens(user.ID)

	err = s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		err = s.UserRepository.WithTx(tx).Create(ctx, user)
		if err != nil {
			return apperrors.Internal("failed to create user", err)
		}

		if err = s.WalletTokenRepository.WithTx(tx).CreateBulk(ctx, walletTokens); err != nil {
			return apperrors.Internal("failed to add tokens to user wallet", err)
		}

		if isReferral {
			err = s.ReferralRepository.WithTx(tx).Create(ctx, referrerToken, user.ID, s.referrerInvitationRewardDP)
			if err != nil {
				return apperrors.Internal("failed to register user as a referral", err)
			}
		}

		// Task completion
		err = s.TaskService.Complete(
			ctx,
			tx,
			model.NewCompletedTask(
				user.ID,
				model.TaskFinishTutorial,
			),
		)

		if err != nil {
			zap.L().Warn("failed to complete task of tutorial finishing", zap.Any("user_id", user.ID), zap.Error(err))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) generateReferralToken(ctx context.Context) (string, error) {
	buffer := make([]byte, referralTokenLength)

	_, err := rand.Read(buffer)
	if err != nil {
		return "", apperrors.Internal("failed to read buffer", err)
	}

	token := base64.URLEncoding.EncodeToString(buffer)[:referralTokenLength]

	user, err := s.UserRepository.FindByReferralToken(ctx, token)
	if err != nil {
		return "", apperrors.Internal("failed get user by referral token", err)
	}

	if user != nil {
		return s.generateReferralToken(ctx)
	}

	return token, nil
}

func (s *UserService) FindByEmail(ctx context.Context, email mtype.Email) (*model.User, error) {
	user, err := s.UserRepository.FindByEmail(ctx, email)
	if err != nil {
		return nil, apperrors.Internal("failed to find user by email", err)
	}

	return user, nil
}

// TODO: wallet sign-up is deprecated and wasn't tested

func (s *UserService) SignUpWithWallet(
	ctx context.Context,
	authWallet model.AuthWithWallet,
) error {
	if ok := VerifySecret(authWallet.Address, authWallet.Secret); !ok {
		return apperrors.BadRequest("secret is invalid")
	}

	user, err := s.createUserWithWallet(ctx, authWallet.Address)
	if err != nil {
		return err
	}

	wallet := model.NewWallet(user.ID, authWallet.Address, authWallet.WalletName)

	err = s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			err := s.UserRepository.WithTx(tx).Create(ctx, user)
			if err != nil {
				return err
			}

			err = s.WalletRepository.WithTx(tx).Create(ctx, wallet)
			if err != nil {
				return err
			}

			if authWallet.ReferrerToken != "" {
				return s.ReferralRepository.WithTx(tx).
					Create(ctx, authWallet.ReferrerToken, user.ID, s.referrerInvitationRewardDP)
			}

			return nil
		})

	if err != nil {
		return apperrors.Internal("failed to create user and wallet", err)
	}

	return nil
}

func (s *UserService) GetByPublicAddress(
	ctx context.Context,
	pubAddress string,
	secret string,
) (*model.User, error) {
	if ok := VerifySecret(pubAddress, secret); !ok {
		return nil, apperrors.BadRequest("secret is invalid")
	}

	user, err := s.UserRepository.GetByPublicAddress(ctx, pubAddress)
	if err != nil {
		return nil, apperrors.NotFound("user with public address not found", err)
	}

	return user, nil
}

func VerifySecret(publicKey, secret string) bool {
	re := regexp.MustCompile(`[0-9]+`)
	digitsStrArray := re.FindAllString(publicKey, -1)

	sumOfDigits := 0
	for _, digitStr := range digitsStrArray {
		digit, err := strconv.Atoi(digitStr)
		if err != nil {
			return false
		}
		sumOfDigits += digit
	}

	totalSum := sumOfDigits - 1

	message := publicKey[:4] + publicKey[5:9] + publicKey[10:] + strconv.Itoa(totalSum)

	hash := sha512.Sum512([]byte(message))
	hashStr := hex.EncodeToString(hash[:])

	return strings.EqualFold(hashStr, secret)
}

func (s *UserService) SignInWithTelegram(
	ctx context.Context,
	authTg model.SignInWithTelegramMiniApp,
) (*model.User, error) {
	user, err := s.FindByTelegramID(ctx, authTg.TelegramID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		user, err = s.CreateWithTelegram(ctx, authTg)
		if err != nil {
			return nil, err
		}
	}

	return user, nil
}

func (s *UserService) SignInWithEmail(
	ctx context.Context,
	email mtype.Email,
	referrerToken string,
) (*model.User, error) {
	user, err := s.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if user == nil {
		user, err = s.CreateWithEmail(ctx, email, referrerToken)
		if err != nil {
			return nil, err
		}
	}

	return user, nil
}

func (s *UserService) FindByTelegramID(
	ctx context.Context,
	telegramID string,
) (*model.User, error) {
	user, err := s.UserRepository.FindByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, apperrors.Internal("failed to get user by telegram id", err)
	}

	return user, nil
}

func (s *UserService) CreateWithTelegram(
	ctx context.Context,
	authTg model.SignInWithTelegramMiniApp,
) (*model.User, error) {
	user, err := s.createUserWithTelegram(ctx, authTg)
	if err != nil {
		return nil, err
	}

	return s.createUser(ctx, user, authTg.ReferrerToken)
}

func (s *UserService) ReferralStatsByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.UserReferralStats, error) {
	stats, err := s.UserRepository.ReferralStats(ctx, id)
	if err != nil {
		return nil, apperrors.Internal("failed to get user referral stats", err)
	}

	return stats, nil
}

func (s *UserService) UserReferralsByID(
	ctx context.Context,
	id uuid.UUID,
	opts *repo.Options,
) ([]*model.UserReferralShow, error) {
	stats, err := s.UserRepository.UserReferrals(ctx, id, opts)
	if err != nil {
		return nil, apperrors.Internal("failed to get user referral stats", err)
	}

	return stats, nil
}

func (s *UserService) CountAllUsers(
	ctx context.Context,
) (int, error) {
	count, err := s.UserRepository.CountUsers(ctx)
	if err != nil {
		return 0, apperrors.Internal("failed to count all users", err)
	}

	return count, nil
}
func (s *UserService) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*model.User, error) {
	user, err := s.UserRepository.GetByID(ctx, id)
	if err != nil {
		return nil, apperrors.Internal("failed to get user by id", err)
	}

	return user, nil
}

func (s *UserService) ChangeUsername(
	ctx context.Context,
	userID uuid.UUID,
	username mtype.Username,
) error {
	if !username.Valid() {
		return apperrors.BadRequest("invalid username")
	}
	user := &model.User{ID: userID, Username: username}

	err := s.UserRepository.Update(ctx, user)
	if err != nil {
		if repo.DuplicateKeyViolation(err) {
			return apperrors.AlreadyExist("this username is already taken")
		}

		return apperrors.Internal("failed to get user by id", err)
	}

	return nil
}

func (s *UserService) EditByID(
	ctx context.Context,
	userID uuid.UUID,
	req *model.UserEditReq,
) error {
	err := s.UserRepository.Edit(ctx, userID, req)
	if err != nil {
		return apperrors.Internal("failed to get user by id", err)
	}

	return nil
}

func (s *UserService) UpdateProfilePicture(
	ctx context.Context,
	userID uuid.UUID,
	file *multipart.FileHeader,
) (string, error) {
	user, err := s.UserRepository.GetByID(ctx, userID)
	if err != nil {
		return "", apperrors.Internal("failed to get user", err)
	}

	if user.ImageUrl != "" && !s.FileService.IsDefaultUserIcon(user.ImageUrl) {
		if err = s.FileService.RemoveUserFile(user.ImageUrl); err != nil {
			return "", err
		}
	}

	newURL, err := s.FileService.SaveUserProfilePicture(file)
	if err != nil {
		return "", err
	}

	user.ImageUrl = newURL
	if err = s.UserRepository.Update(ctx, user); err != nil {
		return "", apperrors.Internal("failed to update user profile picture", err)
	}

	return newURL, nil
}

func (s *UserService) GetByUsername(ctx context.Context, username mtype.Username) (*model.User, error) {
	user, err := s.UserRepository.FindByUsername(ctx, username)
	if err != nil {
		return nil, apperrors.Internal("failed to get user by username", err)
	}

	if user == nil {
		return nil, apperrors.NotFound("user not found")
	}

	return user, nil
}
