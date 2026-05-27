package middleware

import "github.com/gofiber/fiber/v2"

func MWViewChange(c *fiber.Ctx) error {
	// TODO: 判断 header 中是否携带
	mMode, ok := c.GetReqHeaders()["m-mode"]
	if !ok || mMode == "" {
		// 没有携带默认到模版渲染模式
		return c.Next()
	}
	return c.Next()
}
