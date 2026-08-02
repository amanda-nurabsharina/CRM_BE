package service

import (
	"crm-be/src/config"
	"crm-be/src/model"
	"crm-be/src/response"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type TokenService interface {
	GenerateAuthTokens(c *fiber.Ctx, user *model.User) (*response.TokenPair, error)
	ValidateRefreshToken(c *fiber.Ctx, tokenStr string) (*model.RefreshToken, error)
	RevokeRefreshToken(c *fiber.Ctx, tokenStr string) error
}

type tokenService struct {
	db *gorm.DB
}

func NewTokenService(db *gorm.DB) TokenService {
	return &tokenService{db: db}
}

func (s *tokenService) GenerateAuthTokens(c *fiber.Ctx, user *model.User) (*response.TokenPair, error) {
	// Access token payload
	accessExpiry := time.Now().Add(time.Duration(config.JWTExpirationHours) * time.Hour)
	accessClaims := jwt.MapClaims{
		"sub":   user.ID.String(),
		"name":  user.Name,
		"email": user.Email,
		"role":  user.Role,
		"exp":   accessExpiry.Unix(),
		"iat":   time.Now().Unix(),
	}

	accessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err := accessTokenObj.SignedString([]byte(config.JWTSecret))
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to generate access token")
	}

	// Refresh token payload
	refreshExpiry := time.Now().AddDate(0, 0, config.JWTRefreshExpirationDays)
	refreshClaims := jwt.MapClaims{
		"sub": user.ID.String(),
		"exp": refreshExpiry.Unix(),
		"iat": time.Now().Unix(),
	}

	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenStr, err := refreshTokenObj.SignedString([]byte(config.JWTSecret))
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to generate refresh token")
	}

	// Store refresh token in database
	rtModel := model.RefreshToken{
		UserID:    user.ID,
		Token:     refreshTokenStr,
		ExpiresAt: refreshExpiry,
		IsRevoked: false,
	}

	if err := s.db.Create(&rtModel).Error; err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to store refresh token")
	}

	return &response.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		TokenType:    "Bearer",
		ExpiresIn:    int64(config.JWTExpirationHours * 3600),
	}, nil
}

func (s *tokenService) ValidateRefreshToken(c *fiber.Ctx, tokenStr string) (*model.RefreshToken, error) {
	var rt model.RefreshToken
	err := s.db.Preload("User").Where("token = ? AND is_revoked = ?", tokenStr, false).First(&rt).Error
	if err != nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid or revoked refresh token")
	}

	if time.Now().After(rt.ExpiresAt) {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "Refresh token expired")
	}

	return &rt, nil
}

func (s *tokenService) RevokeRefreshToken(c *fiber.Ctx, tokenStr string) error {
	return s.db.Model(&model.RefreshToken{}).
		Where("token = ?", tokenStr).
		Update("is_revoked", true).Error
}
