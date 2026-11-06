package api

import (
	"errors"
	"strconv"

	"myecho/handler"
	"myecho/handler/rtype"
	"myecho/middleware"
	"myecho/model"
	"myecho/service"

	"github.com/gofiber/fiber/v2"
)

func AgentTokenList(c *fiber.Ctx) error {
	user, err := agentTokenOwner(c)
	if user == nil || err != nil {
		return err
	}
	tokens, err := service.ListAgentTokens(user.ID)
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	responses := make([]rtype.AgentTokenResponse, 0, len(tokens))
	for _, token := range tokens {
		responses = append(responses, rtype.ModelToAgentTokenResponse(token, ""))
	}
	return handler.Success(c, responses)
}

func AgentTokenCreate(c *fiber.Ctx) error {
	user, err := agentTokenOwner(c)
	if user == nil || err != nil {
		return err
	}
	var request rtype.AgentTokenCreateRequest
	if err := c.BodyParser(&request); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	token, rawToken, err := service.CreateAgentToken(user.ID, request.Name, request.Scopes, request.ExpiresInDays)
	if errors.Is(err, service.ErrInvalidAgentTokenInput) {
		return ParseErrorResponse(c, err.Error())
	}
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.SuccessWithStatus(c, fiber.StatusCreated, rtype.ModelToAgentTokenResponse(token, rawToken))
}

func AgentTokenRevoke(c *fiber.Ctx) error {
	user, err := agentTokenOwner(c)
	if user == nil || err != nil {
		return err
	}
	tokenID, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || tokenID == 0 {
		return ParseErrorResponse(c, "invalid agent token id")
	}
	token, err := service.RevokeAgentToken(user.ID, uint(tokenID))
	if errors.Is(err, service.ErrAgentTokenNotFound) {
		return NotFoundErrorResponse(c, err.Error())
	}
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.Success(c, rtype.ModelToAgentTokenResponse(token, ""))
}

func agentTokenOwner(c *fiber.Ctx) (*model.User, error) {
	if middleware.IsAgentRequest(c) {
		return nil, ErrorResponse(c, fiber.StatusForbidden, ValidateError, "agent credentials cannot manage agent tokens")
	}
	user, ok := c.Locals("user").(*model.User)
	if !ok || user == nil {
		return nil, UnauthorizedErrorResponse(c)
	}
	return user, nil
}
