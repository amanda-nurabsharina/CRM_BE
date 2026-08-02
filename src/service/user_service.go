package service

import (
	"crm-be/src/model"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserService interface {
	GetByID(id uuid.UUID) (*model.User, error)
	GetByEmail(email string) (*model.User, error)
}

type userService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) UserService {
	return &userService{db: db}
}

func (s *userService) GetByID(id uuid.UUID) (*model.User, error) {
	var user model.User
	if err := s.db.Where("id = ? AND is_active = ?", id, true).First(&user).Error; err != nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	return &user, nil
}

func (s *userService) GetByEmail(email string) (*model.User, error) {
	var user model.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, fiber.NewError(fiber.StatusNotFound, "User not found")
	}
	return &user, nil
}
