package handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"myecho/dal/mysql"
	apierrors "myecho/handler/api/errors"
	"myecho/model"
)

func TestSuccessHelpersWriteCommonEnvelope(t *testing.T) {
	app := fiber.New()
	app.Get("/success", func(c *fiber.Ctx) error {
		return SuccessWithStatus(c, fiber.StatusCreated, map[string]string{"name": "myecho"})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusCreated)
	}

	var body struct {
		Code int                    `json:"code"`
		Msg  string                 `json:"msg"`
		Data map[string]string      `json:"data"`
		Meta map[string]interface{} `json:"meta"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if body.Code != 0 || body.Msg != "ok" || body.Data["name"] != "myecho" || body.Meta == nil {
		t.Fatalf("unexpected success body: %+v", body)
	}

	common := GetSuccessCommonResp("done")
	if common.Code != 0 || common.Msg != "ok" || common.Data != "done" || common.Meta == nil {
		t.Fatalf("GetSuccessCommonResp() = %+v", common)
	}
}

func TestPaginateDataWritesMeta(t *testing.T) {
	app := fiber.New()
	app.Get("/items", func(c *fiber.Ctx) error {
		return PaginateData(c, mysql.PageInfo{Total: 12, Page: 2, PageSize: 5}, []int{1, 2})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/items", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}

	var body struct {
		Code int                    `json:"code"`
		Msg  string                 `json:"msg"`
		Data []int                  `json:"data"`
		Meta map[string]interface{} `json:"meta"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if body.Code != 0 || body.Msg != "ok" || len(body.Data) != 2 {
		t.Fatalf("unexpected body: %+v", body)
	}
	if body.Meta["total"] != float64(12) || body.Meta["page"] != float64(2) || body.Meta["page_size"] != float64(5) {
		t.Fatalf("unexpected meta: %+v", body.Meta)
	}
}

func TestParsePageFindParamAndPageFind(t *testing.T) {
	app := fiber.New()
	app.Get("/search", func(c *fiber.Ctx) error {
		result, param, err := PageFind(c, func(page *mysql.PageFindParam, prefix string) ([]string, error) {
			if page.Page != 3 || page.PageSize != 7 || !page.NoPage {
				t.Fatalf("PageFindParam = %+v", *page)
			}
			return []string{prefix, "ok"}, nil
		}, "article")
		if err != nil {
			return err
		}
		if param.Page != 3 || param.PageSize != 7 || !param.NoPage {
			t.Fatalf("returned PageFindParam = %+v", param)
		}
		return Success(c, result)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/search?page=3&page_size=7&no_page=true", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestDetailPreHandleByParamRejectsInvalidID(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			if errors.Is(err, apierrors.ErrorIDNotFound) {
				return c.SendStatus(fiber.StatusNotFound)
			}
			return c.SendStatus(fiber.StatusInternalServerError)
		},
	})
	app.Get("/items/:id", func(c *fiber.Ctx) error {
		var category model.Category
		return DetailPreHandleByParam(c, &category)
	})

	for _, path := range []string{"/items/not-a-number", "/items/0"} {
		t.Run(path, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, nil))
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			if resp.StatusCode != fiber.StatusNotFound {
				t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
			}
		})
	}
}

func TestGetUserFromCtx(t *testing.T) {
	app := fiber.New()
	app.Get("/me", func(c *fiber.Ctx) error {
		want := &model.User{Name: "admin"}
		c.Locals("user", want)
		got := GetUserFromCtx(c)
		if got != want {
			t.Fatalf("GetUserFromCtx() = %p, want %p", got, want)
		}
		return Success(c, got.Name)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/me", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}
