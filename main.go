package main

import (
	"flag"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"myecho/config"
	"myecho/middleware"
	"myecho/router"
)

var (
	port = flag.String("port", ":2999", "Port Listen on")
	prod = flag.Bool("prod", false, "Enable perfork in Production")
)

func main() {
	flag.Parse()
	app := fiber.New(fiber.Config{
		Prefork: *prod,
	})
	app.Use(recover.New())
	app.Use(logger.New())
	app.Static("/", "./static")
	config.ConnectDB()
	router.SetupApiRouter(app)
	app.Use(middleware.Custom404ErrorHandler)
	log.Fatal(app.Listen(*port))
}
