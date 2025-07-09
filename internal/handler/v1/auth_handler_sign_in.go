package v1

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	initdata "github.com/telegram-mini-apps/init-data-golang"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	auth "gitlab.com/duel-duck/duel-duck-api/pkg/jwt"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
)

const (
	TelegramAuthDateLifetime = 4 * time.Hour
)

var SkipTelegramValidationUsers = map[string]struct{}{
	"1027183873": {},
	"889281928":  {},
	"7208477230": {},
}

// SignInWithTelegramMiniApp godoc
//
//	@Summary		Sign in user with Telegram Mini App
//	@Description	Authenticates a user using Telegram Mini App init data. Validates the init data hash and creates/updates user account. Returns user data and JWT tokens.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		model.SignInWithTelegramMiniAppReq																	true	"Telegram init data and additional parameters"
//	@Success		200		{object}	object{user=model.User,jwt_info=object{access_token=string,refresh_token=string},daily_reward=int}	"Successfully authenticated"
//	@Failure		400		{object}	apperrors.ErrorPublic																				"Invalid request data or failed to parse init data"
//	@Failure		401		{object}	apperrors.ErrorPublic																				"Invalid telegram init data hash or login timeout"
//	@Failure		500		{object}	apperrors.ErrorPublic																				"Internal server error"
//	@Router			/auth/sign-in-telegram [post]
func (h *AuthHandler) SignInWithTelegramMiniApp(c fiber.Ctx) error {
	var req model.SignInWithTelegramMiniAppReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	initData, err := initdata.Parse(req.InitDataRaw)
	if err != nil {
		return apperrors.BadRequest("failed to parse init data", err)
	}

	telegramID := strconv.FormatInt(initData.User.ID, 10)

	// Literally backdoor here
	if h.env != config.EnvironmentProduction {
		if _, ok := SkipTelegramValidationUsers[telegramID]; !ok {
			err = initdata.Validate(req.InitDataRaw, h.MiniAppAPIKey, TelegramAuthDateLifetime)
			if errors.Is(err, initdata.ErrExpired) {
				return apperrors.LoginTimeout("telegram auth date expired")
			}

			if err != nil {
				return apperrors.Unauthorized("invalid telegram init data hash", err)
			}
		}
	}

	authTG := model.NewSignInWithTelegramMiniApp(initData, req)

	user, err := h.UserService.SignInWithTelegram(c.Context(), authTG)
	if err != nil {
		return apperrors.Unauthorized("failed to login user")
	}

	claims, ok := auth.NewTokenClaimsByUser(user)
	if !ok {
		return apperrors.Internal("failed to generate token claims for user")
	}

	tokenPair, err := h.JWTService.GenerateTokenPair(c.Context(), claims)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"user":         user,
		"jwt_info":     tokenPair,
		"daily_reward": model.CycleDayStreak(user),
	})
}

// SignInWithTelegramWeb godoc
//
//	@Summary		Sign in user with Telegram Web authentication
//	@Description	Authenticates a user using previously generated Telegram authentication data. Validates the provided code against stored code for the given Telegram ID and creates/updates user account. Returns user data and JWT tokens.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		model.SignInWithTelegramWeb											true	"Telegram authentication data"
//	@Success		200		{object}	object{user=model.User,jwt_info=auth.TokenPair,daily_reward=int}	"Successfully authenticated"
//	@Failure		400		{object}	apperrors.ErrorPublic												"Invalid request data"
//	@Failure		401		{object}	apperrors.ErrorPublic												"Invalid or expired authentication code"
//	@Failure		404		{object}	apperrors.ErrorPublic												"User not found"
//	@Failure		500		{object}	apperrors.ErrorPublic												"Internal server error"
//	@Router			/auth/sign-in-telegram-web [post]
func (h *AuthHandler) SignInWithTelegramWeb(c fiber.Ctx) error {
	var authTG model.SignInWithTelegramWeb
	if err := c.Bind().JSON(&authTG); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	ok, err := h.AuthService.CheckUsersTelegramCode(c.Context(), authTG.TelegramID, authTG.Code)
	if err != nil {
		return apperrors.Unauthorized("provided telegram id is not stored")
	}

	if !ok {
		return apperrors.Unauthorized("provided code doesn't match stored one")
	}

	user, err := h.UserService.FindByTelegramID(c.Context(), authTG.TelegramID)
	if err != nil {
		return apperrors.Internal("failed to find user by telegram id", err)
	}

	if user == nil {
		return apperrors.NotFound("user not found")
	}

	claims, ok := auth.NewTokenClaimsByUser(user)
	if !ok {
		return apperrors.Internal("failed to generate token claims for user")
	}

	tokenPair, err := h.JWTService.GenerateTokenPair(c.Context(), claims)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"user":         user,
		"jwt_info":     tokenPair,
		"daily_reward": model.CycleDayStreak(user),
	})
}

func (h *AuthHandler) SignInWithWallet(c fiber.Ctx) error {
	var req model.AuthWithWallet
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	user, err := h.UserService.GetByPublicAddress(c.Context(), req.Address, req.Secret)
	if err != nil {
		return apperrors.NotFound("user with email not found", err)
	}

	claims, ok := auth.NewTokenClaimsByUser(user)
	if !ok {
		return apperrors.Internal("failed to generate token claims for user")
	}

	tokenPair, err := h.JWTService.GenerateTokenPair(c.Context(), claims)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"user":         user,
		"jwt_info":     tokenPair,
		"daily_reward": model.CycleDayStreak(user),
	})
}

// SignInWithGoogle godoc
//
//	@Summary		Sign in user with Google authentication
//	@Description	Authenticates a user using Google credentials and returns user data with JWT tokens. The email is validated through Google's authentication system and a new session is created.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		GoogleAuth
//	@Param			request	body		model.SignInWithGoogle												true	"Google sign in request"
//	@Success		200		{object}	object{user=model.User,jwt_info=auth.TokenPair,daily_reward=int}	"Successfully authenticated"
//	@Failure		400		{object}	apperrors.ErrorPublic												"Invalid request data or missing email"
//	@Failure		401		{object}	apperrors.ErrorPublic												"Unauthorized - Invalid Google token"
//	@Failure		404		{object}	apperrors.ErrorPublic												"User not found"
//	@Failure		500		{object}	apperrors.ErrorPublic												"Internal server error"
//	@Router			/auth/sign-in-google [post]
func (h *AuthHandler) SignInWithGoogle(c fiber.Ctx) error {
	email, ok := c.Locals("email").(mtype.Email)
	if !ok {
		return apperrors.BadRequest("email is missing")
	}

	var req model.SignInWithGoogle
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	user, err := h.UserService.SignInWithEmail(c.Context(), email, req.ReferrerToken)
	if err != nil {
		return err
	}

	claims, ok := auth.NewTokenClaimsByUser(user)
	if !ok {
		return apperrors.Internal("failed to generate token claims for user")
	}

	tokenPair, err := h.JWTService.GenerateTokenPair(c.Context(), claims)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"user":         user,
		"jwt_info":     tokenPair,
		"daily_reward": model.CycleDayStreak(user),
	})
}

// RefreshTokens godoc
//
//	@Summary		Refresh JWT tokens
//	@Description	Refreshes the user's JWT tokens using the provided refresh token from the Authorization header. Returns a new pair of access and refresh tokens.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string							true	"Bearer refresh token"	default(Bearer <refresh_token>)
//	@Success		200				{object}	object{jwt_info=auth.TokenPair}	"Successfully refreshed tokens"
//	@Failure		401				{object}	apperrors.ErrorPublic			"Unauthorized - Invalid or expired refresh token"
//	@Failure		500				{object}	apperrors.ErrorPublic			"Internal server error"
//	@Router			/auth/refresh [post]
func (h *AuthHandler) RefreshTokens(c fiber.Ctx) error {
	token := c.Get("Authorization")

	tokenPair, err := h.JWTService.RefreshSession(c.Context(), token)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"jwt_info": tokenPair,
	})
}
