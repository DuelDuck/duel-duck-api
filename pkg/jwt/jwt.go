package auth

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"strings"
	"time"

	"github.com/google/uuid"
)

type jwtAuthenticator struct {
	signKey         string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewJWTAuth(c *config.Config) JWTAuthenticator {
	return &jwtAuthenticator{
		signKey:         c.Auth.SecretSignKey,
		accessTokenTTL:  c.Auth.AccessTokenTTL,
		refreshTokenTTL: c.Auth.RefreshTokenTTL,
	}
}

type TokenJWTClaims struct {
	TokenClaims
	jwt.RegisteredClaims
}

type TokenExpiration struct {
	Token     string
	ExpiresAt time.Time
}

func (a *jwtAuthenticator) GenerateTokenPair(claims TokenClaims) (*TokenPairWithExpiration, error) {
	refreshToken, err := a.generateToken(claims, a.refreshTokenTTL)
	if err != nil {
		return nil, err
	}

	accessToken, err := a.generateToken(claims, a.accessTokenTTL)
	if err != nil {
		return nil, err
	}

	return &TokenPairWithExpiration{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (a *jwtAuthenticator) generateToken(
	claims TokenClaims,
	tokenLifetime time.Duration,
) (*TokenExpiration, error) {
	if claims.UserID == uuid.Nil {
		return nil, apperrors.Unauthorized(ErrInvalidTokenClaims)
	}

	mySigningKey := []byte(a.signKey)
	now := time.Now()
	expiresAt := now.Add(tokenLifetime)

	jwtClaims := TokenJWTClaims{
		TokenClaims: claims,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "duelduck-api",
			Subject:   "client",
			ID:        claims.SessionID.String(),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims).SignedString(mySigningKey)
	if err != nil {
		return nil, apperrors.Unauthorized(ErrGenerateToken, err)
	}

	return &TokenExpiration{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (a *jwtAuthenticator) ParseToken(authToken string) (*TokenClaims, error) {
	authToken = strings.TrimPrefix(authToken, "Bearer ")

	token, err := jwt.Parse(
		authToken,
		a.GetKey,
		jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		if errors.Is(err, jwt.ErrSignatureInvalid) || errors.Is(err, jwt.ErrTokenExpired) {
			return nil, apperrors.Unauthorized(ErrInvalidToken, err)
		}

		return nil, apperrors.Unauthorized("failed to parse jwt token", err)
	}

	if token == nil || !token.Valid {
		return nil, apperrors.Unauthorized(ErrInvalidToken)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, apperrors.Unauthorized(ErrInvalidTokenClaims)
	}

	return ExtractClaims(claims)
}

func ExtractClaims(claims jwt.MapClaims) (*TokenClaims, error) {
	premium, ok := ValueFromJWTClaims[bool](claims, "premium")
	if !ok {
		return nil, apperrors.Unauthorized("invalid premium claim")
	}

	roleFloat, ok := ValueFromJWTClaims[float64](claims, "role")
	if !ok {
		return nil, apperrors.Unauthorized("invalid role claim")
	}

	role := mtype.Role(roleFloat)
	if !role.IsValid() {
		return nil, apperrors.Unauthorized("invalid role claim value")
	}

	userID, ok := extractUUID(claims, "user_id")
	if !ok {
		return nil, apperrors.Unauthorized("invalid user_id claim")
	}

	publicAddress, ok := ValueFromJWTClaims[string](claims, "public_address")
	if !ok {
		return nil, apperrors.Unauthorized("invalid public_address claim")
	}

	sessionID, ok := extractUUID(claims, "session_id")
	if !ok {
		return nil, apperrors.Unauthorized("invalid session_id claim")
	}

	parsed := &TokenClaims{
		UserID:        userID,
		SessionID:     sessionID,
		PublicAddress: publicAddress,
		Premium:       premium,
		Role:          role,
	}

	return parsed, nil
}

func ValueFromJWTClaims[T any](claims jwt.MapClaims, value string) (T, bool) {
	var zero T
	v, ok := claims[value]
	if !ok {
		return zero, false
	}

	typedValue, ok := v.(T)
	if !ok {
		return zero, false
	}

	return typedValue, true
}

func extractUUID(claims jwt.MapClaims, value string) (uuid.UUID, bool) {
	v, ok := claims[value]
	if !ok {
		return uuid.Nil, false
	}

	stringUUID, ok := v.(string)
	if !ok {
		return uuid.Nil, false
	}

	parsed, err := uuid.Parse(stringUUID)
	if err != nil {
		return uuid.Nil, false
	}

	return parsed, true
}

func (a *jwtAuthenticator) GetKey(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, apperrors.Unauthorized(ErrInvalidSigningMethod)
	}

	return []byte(a.signKey), nil
}
