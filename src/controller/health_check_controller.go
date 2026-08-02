package controller

import (
	"crm-be/src/response"

	"github.com/gofiber/fiber/v2"
)

type HealthCheckController struct{}

func NewHealthCheckController() *HealthCheckController {
	return &HealthCheckController{}
}

func (h *HealthCheckController) Check(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(response.StandardResponse{
		Code:    fiber.StatusOK,
		Status:  "success",
		Message: "WhatsApp CRM Backend API is healthy and operational",
		Data: fiber.Map{
			"version": "1.0.0",
			"status":  "up",
		},
	})
}
