package handler

import (
	"strconv"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/service"
	"github.com/gofiber/fiber/v2"
)

type AuthContextHandler struct {
	svc *service.AuthContextService
}

func NewAuthContextHandler(svc *service.AuthContextService) *AuthContextHandler {
	return &AuthContextHandler{svc: svc}
}

// getUserFromLocals extracts user info from middleware locals
func getUserFromLocals(c *fiber.Ctx) (*models.User, bool) {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return nil, false
	}
	
	username, _ := c.Locals("username").(string)
	role, _ := c.Locals("role").(string)
	
	return &models.User{
		ID:       userID,
		Username: username,
		Role:     role,
	}, true
}

func (h *AuthContextHandler) Create(c *fiber.Ctx) error {
	var req service.CreateAuthContextRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	
	user, ok := getUserFromLocals(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}
	req.UserID = user.ID

	res, err := h.svc.CreateAuth(c.Context(), &req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(fiber.Map{"data": res, "status": "success"})
}

func (h *AuthContextHandler) List(c *fiber.Ctx) error {
	tid, err := strconv.Atoi(c.Params("target_id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target ID"})
	}
	
	user, ok := getUserFromLocals(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	list, err := h.svc.GetByTarget(c.Context(), uint(tid), user.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": list, "status": "success"})
}

func (h *AuthContextHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID"})
	}
	
	user, ok := getUserFromLocals(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}
	
	if err := h.svc.Delete(c.Context(), uint(id), user.ID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "Context deleted"})
}
