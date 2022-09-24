package main

import (
	"flag"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"log"
	"myecho/config"
	"myecho/dal/connect"
	"myecho/dal/mysql"

	"myecho/middleware"
)

var (
	port = flag.String("port", ":2999", "Port Listen on")
	prod = flag.Bool("prod", false, "Enable perfork in Production")
)

func main() {
	flag.Parse()
	app := fiber.New(fiber.Config{
		Prefork:   *prod,
		BodyLimit: 1024 * 1024 * 1024,
	})
	connect.ConnectDB()
	mysql.InitDB()
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))
	app.Use(logger.New())
	app.Static("/", "./static")
	app.Static("/mos", config.StorageRootPath)
	SetupApiRouter(app)
	app.Use(middleware.Custom404ErrorHandler)
	log.Fatal(app.Listen(*port))
}
