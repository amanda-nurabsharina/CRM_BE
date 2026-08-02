package controller

import (
	"crm-be/src/response"
	"crm-be/src/service"
	"crm-be/src/validation"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

var validate = validator.New()

type AuthController struct {
	AuthService  service.AuthService
	UserService  service.UserService
	TokenService service.TokenService
}

func NewAuthController(
	authService service.AuthService,
	userService service.UserService,
	tokenService service.TokenService,
) *AuthController {
	return &AuthController{
		AuthService:  authService,
		UserService:  userService,
		TokenService: tokenService,
	}
}

func (a *AuthController) Login(c *fiber.Ctx) error {
	req := new(validation.Login)
	if err := c.BodyParser(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if err := validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	user, err := a.AuthService.Login(c, req)
	if err != nil {
		return err
	}

	tokens, err := a.TokenService.GenerateAuthTokens(c, user)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(response.SuccessWithTokens{
		Code:    fiber.StatusOK,
		Status:  "success",
		Message: "Logged in successfully",
		User:    *user,
		Tokens:  *tokens,
	})
}

func (a *AuthController) Register(c *fiber.Ctx) error {
	req := new(validation.Register)
	if err := c.BodyParser(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if err := validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	user, err := a.AuthService.Register(c, req)
	if err != nil {
		return err
	}

	tokens, err := a.TokenService.GenerateAuthTokens(c, user)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(response.SuccessWithTokens{
		Code:    fiber.StatusCreated,
		Status:  "success",
		Message: "User registered successfully",
		User:    *user,
		Tokens:  *tokens,
	})
}

func (a *AuthController) Me(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("userId").(string)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "User context missing")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid user ID format")
	}

	user, err := a.UserService.GetByID(userID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(response.StandardResponse{
		Code:    fiber.StatusOK,
		Status:  "success",
		Message: "User profile retrieved",
		Data:    user,
	})
}

func (a *AuthController) Refresh(c *fiber.Ctx) error {
	req := new(validation.RefreshToken)
	if err := c.BodyParser(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	rt, err := a.TokenService.ValidateRefreshToken(c, req.RefreshToken)
	if err != nil {
		return err
	}

	// Revoke current refresh token
	_ = a.TokenService.RevokeRefreshToken(c, req.RefreshToken)

	// Issue new tokens
	user, err := a.UserService.GetByID(rt.UserID)
	if err != nil {
		return err
	}

	tokens, err := a.TokenService.GenerateAuthTokens(c, user)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(response.StandardResponse{
		Code:    fiber.StatusOK,
		Status:  "success",
		Message: "Token refreshed successfully",
		Data:    tokens,
	})
}

func (a *AuthController) Logout(c *fiber.Ctx) error {
	req := new(validation.RefreshToken)
	if err := c.BodyParser(req); err == nil && req.RefreshToken != "" {
		_ = a.TokenService.RevokeRefreshToken(c, req.RefreshToken)
	}

	return c.Status(fiber.StatusOK).JSON(response.StandardResponse{
		Code:    fiber.StatusOK,
		Status:  "success",
		Message: "Logged out successfully",
	})
}
