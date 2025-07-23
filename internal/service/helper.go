package service

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
)

const (
	MediaFilesDirectory = "resources/static/profile-icons/"
)

func (s *UserService) createUserWithEmail(ctx context.Context, email mtype.Email) (*model.User, error) {
	username, ok := email.Username()
	if !ok {
		return nil, apperrors.BadRequest("failed to get username from email")
	}

	profileImage, err := s.FileService.getRandomIconName()
	if err != nil {
		return nil, err
	}

	referralToken, err := s.generateReferralToken(ctx)
	if err != nil {
		return nil, err
	}

	user := model.NewUser(
		username,
		referralToken,
		profileImage)

	user.Email = email

	return user, nil
}

func (s *UserService) createUserWithWallet(
	ctx context.Context,
	pubAddress string,
) (*model.User, error) {

	profileImage, err := s.FileService.getRandomIconName()
	if err != nil {
		return nil, err
	}

	referralToken, err := s.generateReferralToken(ctx)
	if err != nil {
		return nil, err
	}

	randomNum, err := generateRandomNum()
	if err != nil {
		return nil, err
	}

	username, ok := mtype.NewUsername("duck_" + strconv.FormatUint(randomNum, 10))
	if !ok {
		return nil, apperrors.Internal("failed to generate username", err)
	}

	user := model.NewUser(
		username,
		referralToken,
		profileImage)

	user.PublicAddress = pubAddress

	return user, nil
}

func (s *UserService) createUserWithTelegram(
	ctx context.Context,
	authTg model.SignInWithTelegramMiniApp,
) (*model.User, error) {
	profileImage, err := s.FileService.getRandomIconName()
	if err != nil {
		return nil, err
	}

	referralToken, err := s.generateReferralToken(ctx)
	if err != nil {
		return nil, err
	}

	username, ok := mtype.NewUsername(authTg.Username)
	if !ok {
		username, ok = mtype.NewUsername("duck_" + authTg.TelegramID)
		if !ok {
			return nil, apperrors.Internal("invalid tg firstname: failed to generate username")
		}
	}

	user := model.NewUser(
		username,
		referralToken,
		profileImage)

	user.IsPremium = authTg.IsPremium
	user.TelegramID = authTg.TelegramID

	return user, nil
}

func generateRandomNum() (uint64, error) {
	randomNum, err := cryptoRand.Int(cryptoRand.Reader, big.NewInt(math.MaxUint32))
	if err != nil {
		return 0, apperrors.Internal("failed to generate num", err)
	}

	return randomNum.Uint64(), nil
}

func generateVerificationCode() (string, error) {
	num, err := generateRandomNum()
	if err != nil {
		return "", err
	}

	code := int(num)%900000 + 100000

	return fmt.Sprintf("%06d", code), nil
}

// TODO: load all file paths on start up instead of reading all files from directory on every req
func (s *FileService) getRandomIconName() (string, error) {
	files, err := os.ReadDir(MediaFilesDirectory)
	if err != nil {
		return "", apperrors.Internal("failed to chose random icon", err)
	}

	svgFiles := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".svg" {
			svgFiles = append(svgFiles, file.Name())
		}
	}

	if len(svgFiles) == 0 {
		err = fmt.Errorf("no SVG files found in directory: %s", MediaFilesDirectory)
		return "", apperrors.Internal("failed to chose random icon", err)
	}

	var seed int64
	err = binary.Read(cryptoRand.Reader, binary.LittleEndian, &seed)
	if err != nil {
		return "", apperrors.Internal("failed to chose random icon", err)
	}

	rand.New(rand.NewSource(seed))
	randomFile := svgFiles[rand.Intn(len(svgFiles))]

	return path.Join("/profile-icons", randomFile), nil
}
