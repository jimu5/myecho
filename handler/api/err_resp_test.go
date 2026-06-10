package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestErrorResponseHelpers(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		handler    fiber.Handler
		wantStatus int
		wantCode   int
		wantMsg    string
	}{
		{
			name:       "parse",
			path:       "/parse",
			handler:    func(c *fiber.Ctx) error { return ParseErrorResponse(c, "bad json") },
			wantStatus: fiber.StatusBadRequest,
			wantCode:   ParseError,
			wantMsg:    "bad json",
		},
		{
			name:       "not found",
			path:       "/not-found",
			handler:    func(c *fiber.Ctx) error { return NotFoundErrorResponse(c, "missing") },
			wantStatus: fiber.StatusNotFound,
			wantCode:   NotFound,
			wantMsg:    "missing",
		},
		{
			name:       "validate",
			path:       "/validate",
			handler:    func(c *fiber.Ctx) error { return ValidateErrorResponse(c, "invalid") },
			wantStatus: fiber.StatusForbidden,
			wantCode:   ValidateError,
			wantMsg:    "invalid",
		},
		{
			name:       "unauthorized",
			path:       "/unauthorized",
			handler:    func(c *fiber.Ctx) error { return UnauthorizedErrorResponse(c) },
			wantStatus: fiber.StatusUnauthorized,
			wantCode:   Unauthorized,
			wantMsg:    UnauthorizedErrorMsg,
		},
		{
			name:       "internal",
			path:       "/internal",
			handler:    func(c *fiber.Ctx) error { return InternalErrorResponse(c, InternalSQLError, "db failed") },
			wantStatus: fiber.StatusInternalServerError,
			wantCode:   InternalSQLError,
			wantMsg:    "db failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get(tt.path, tt.handler)

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, tt.path, nil))
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			var body Error
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if body.Code != tt.wantCode || body.Msg != tt.wantMsg {
				t.Fatalf("body = %+v, want code=%d msg=%q", body, tt.wantCode, tt.wantMsg)
			}
		})
	}
}
