package v1

import (
	"context"
	"fmt"
	"strings"

	firebase "firebase.google.com/go/v4"
	firebaseAuth "firebase.google.com/go/v4/auth"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	auth "gitlab.com/duel-duck/duel-duck-api/pkg/jwt"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"google.golang.org/api/option"
)

const (
	jwtAuthType = "Bearer"
)

type AuthHandler struct {
	Firebase      *firebaseAuth.Client
	JWTService    *service.JWTService
	AuthService   *service.AuthService
	UserService   *service.UserService
	FAQService    *service.FAQService
	MiniAppAPIKey string
	env           string
}

func NewAuthHandler(
	c *config.Config,
	userService *service.UserService,
	authService *service.AuthService,
	jwtService *service.JWTService,
	faqService *service.FAQService,
) (*AuthHandler, error) {
	authHandler := &AuthHandler{
		UserService:   userService,
		JWTService:    jwtService,
		AuthService:   authService,
		FAQService:    faqService,
		MiniAppAPIKey: c.Telegram.MiniAppBotTgAPIKey,
		env:           c.App.Environment,
	}

	err := authHandler.setFirebaseAuth(c.App.FirebaseFilePath)
	if err != nil {
		return nil, err
	}

	return authHandler, nil
}

func (h *AuthHandler) setFirebaseAuth(firebaseFilePath string) error {
	opt := option.WithCredentialsFile(firebaseFilePath)

	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return fmt.Errorf("firebase.NewApp: %w", err)
	}

	h.Firebase, err = app.Auth(context.Background())
	if err != nil {
		return fmt.Errorf("app.Auth: %w", err)
	}

	return nil
}

func (h *AuthHandler) RegisterRoutes(app *fiber.App) {
	authGroup := app.Group("/auth")
	{
		authGroup.Post("/send-code", h.SendCode)

		authGroup.Post("/sign-in-email", h.SignInWithEmail)
		authGroup.Post("/sign-in-google", h.SignInWithGoogle, h.SignInWithGoogleMiddleware)

		authGroup.Post("/sign-in-telegram", h.SignInWithTelegramMiniApp)
		authGroup.Post("/sign-in-telegram-web", h.SignInWithTelegramWeb)

		authGroup.Post("/refresh", h.RefreshTokens)
	}
}

func (h *AuthHandler) SignInWithGoogleMiddleware(c fiber.Ctx) error {
	if err := h.authWithGoogle(c); err != nil {
		return err
	}

	return c.Next()
}

func (h *AuthHandler) authWithGoogle(c fiber.Ctx) error {
	strToken := c.Get("Authorization")
	if strToken == "" {
		return apperrors.Unauthorized("authorization token not found")
	}
	strToken = strings.TrimPrefix(strToken, jwtAuthType+" ")

	token, err := h.Firebase.VerifyIDTokenAndCheckRevoked(c.Context(), strToken)
	if err != nil {
		if firebaseAuth.IsIDTokenExpired(err) {
			return apperrors.Unauthorized("authorization token is expired")
		}

		return apperrors.Unauthorized("invalid authorization token", err)
	}

	emailRaw, ok := token.Claims["email"].(string)
	if !ok {
		return apperrors.Unauthorized("google token: email not found")
	}

	email, ok := mtype.NewEmail(emailRaw)
	if !ok {
		return apperrors.Unauthorized("google token: email is invalid")
	}
	c.Locals("email", email)

	return nil
}

// SendCode godoc
//
//	@Summary		Send verification code to email
//	@Description	Sends a verification code to the specified email address. The code is required for email-based sign-in or sign-up.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		model.SendCode			true	"Email address to send the verification code to"
//	@Success		200		{object}	nil						"Verification code sent successfully"
//	@Failure		400		{object}	apperrors.ErrorPublic	"Invalid request data or email format"
//	@Failure		500		{object}	apperrors.ErrorPublic	"Internal server error during code sending"
//	@Router			/auth/send-code [post]
func (h *AuthHandler) SendCode(c fiber.Ctx) error {
	var req model.SendCode
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	email, ok := mtype.NewEmail(req.Email)
	if !ok {
		return apperrors.BadRequest("invalid email")
	}

	err := h.AuthService.SendCodeOnEmail(c.Context(), "index.html", email)
	if err != nil {
		return err
	}

	return nil
}

// SignInWithEmail godoc
//
//	@Summary		User Sign-In/Sign-Up with Email
//	@Description	Authenticates a user or creates a new account using an email and a verification code.
//	@Description	A verification code must be obtained first via the /send-code endpoint.
//	@Description	If the user does not exist, a new account is created. An optional referrer token can be provided to link the new user to a referrer.
//	@Description	On successful authentication, it returns the user's profile, a JWT token pair (access and refresh), and information about their daily login streak reward.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		model.SignInWithEmail															true	"Sign-in request with email, verification code, and optional referrer token"
//	@Success		200		{object}	object{user=model.User,jwt_info=auth.TokenPair,daily_reward=model.DailyReward}	"Successful sign-in or sign-up, returning user data, JWT tokens, and daily reward info"
//	@Failure		400		{object}	apperrors.ErrorPublic															"Invalid request data or email format"
//	@Failure		401		{object}	apperrors.ErrorPublic															"Invalid verification code"
//	@Failure		404		{object}	apperrors.ErrorPublic															"User not found (error during sign-in process)"
//	@Failure		500		{object}	apperrors.ErrorPublic															"Internal server error during token generation or other processes"
//	@Router			/auth/sign-in-email [post]
func (h *AuthHandler) SignInWithEmail(c fiber.Ctx) error {
	var req model.SignInWithEmail
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	email, ok := mtype.NewEmail(req.Email)
	if !ok {
		return apperrors.BadRequest("invalid email")
	}

	ok, err := h.AuthService.CheckUsersEmailCode(c.Context(), email, req.Code)
	if err != nil {
		return err
	}

	if !ok {
		return apperrors.Unauthorized("invalid code")
	}

	user, err := h.UserService.SignInWithEmail(c.Context(), email, req.ReferrerToken)
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

func (h *AuthHandler) AuthMiddleware(c fiber.Ctx) error {
	token := c.Get("Authorization")

	claims, err := h.JWTService.ParseToken(token)
	if err != nil {
		return apperrors.Unauthorized("failed to parse token", err)
	}

	if claims == nil {
		return apperrors.Internal("failed to generate claims")
	}

	c.Locals("claims", *claims)

	return c.Next()
}

func (h *AuthHandler) FAQAuthMiddleware(c fiber.Ctx) error {
	token := c.Get("Authorization")

	claims, err := h.JWTService.ParseToken(token)

	if err == nil && claims != nil {
		c.Locals("claims", *claims)
		return c.Next()
	}

	anonUser, err := h.FAQService.GetFAQAAnonymousUser(
		c.Context(),
		&model.FAQAnonymousUser{
			Anonymous: true,
			IP:        c.IP(),
			Agent:     strings.Trim(c.Get("User-Agent"), "\""),
		})
	if err != nil {
		return apperrors.Unauthorized("failed to get anonymous user", err)
	}

	faqUser := model.FAQUser{
		Anonymous: true,
	}

	if anonUser.ID == uuid.Nil {
		anonUser.ID = uuid.New()
		user, err := h.FAQService.SaveFAQAAnonymousUser(c.Context(), anonUser)
		if err != nil {
			return apperrors.Internal("failed to save anonymous user", err)
		}

		faqUser.ID = user.ID
	} else {
		faqUser.ID = anonUser.ID
	}

	c.Locals("anonymous_claims", faqUser)
	return c.Next()
}

func (h *AuthHandler) SignUpWallet(c fiber.Ctx) error {
	var req model.AuthWithWallet
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid wallet data")
	}

	err := h.UserService.SignUpWithWallet(c.Context(), req)

	return err
}
