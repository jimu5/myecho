package middleware

import (
	"github.com/gofiber/fiber/v2"
	"time"
)

type RequestTimeDuration struct {
	StartTime time.Time
}

func (t *RequestTimeDuration) GetTimeCost() time.Duration {
	return time.Since(t.StartTime)
}

func MWRequestTimeCost(ctx *fiber.Ctx) error {
	cost := &RequestTimeDuration{
		StartTime: time.Now(),
	}
	ctx.Locals("RequestTimeDuration", cost)
	return ctx.Next()
}
