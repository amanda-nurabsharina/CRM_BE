package service

import (
	"crm-be/src/model"
	"crm-be/src/utils"
	"crm-be/src/validation"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type AuthService interface {
	Login(c *fiber.Ctx, req *validation.Login) (*model.User, error)
	Register(c *fiber.Ctx, req *validation.Register) (*model.User, error)
}

type authService struct {
	db *gorm.DB
}

func NewAuthService(db *gorm.DB) AuthService {
	return &authService{db: db}
}

func (s *authService) Login(c *fiber.Ctx, req *validation.Login) (*model.User, error) {
	var user model.User
	if err := s.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid email or password")
	}

	if !user.IsActive {
		return nil, fiber.NewError(fiber.StatusForbidden, "User account is inactive")
	}

	if !utils.CheckPasswordHash(req.Password, user.Password) {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid email or password")
	}

	return &user, nil
}

func (s *authService) Register(c *fiber.Ctx, req *validation.Register) (*model.User, error) {
	var count int64
	s.db.Model(&model.User{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		return nil, fiber.NewError(fiber.StatusConflict, "Email address already registered")
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to hash password")
	}

	role := req.Role
	if role == "" {
		role = "agent"
	}

	user := model.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
		Role:     role,
		IsActive: true,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to create user account")
	}

	return &user, nil
}
