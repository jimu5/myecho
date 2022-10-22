package main

import (
	"flag"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html"
	"log"
	"myecho/config/static_config"
	"myecho/config/yaml_config"
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
	yaml_config.ReadYAMLConfig()
	app := fiber.New(fiber.Config{
		Prefork:           *prod,
		BodyLimit:         1024 * 1024 * 1024,
		Views:             html.New("./views", ".html"),
		ProxyHeader:       "X-Real-IP",
		PassLocalsToViews: true, // 开启这个设置，将 ctx 里面的变量传递给模板
	})
	connect.ConnectDB()
	mysql.InitDB()
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))
	app.Use(middleware.MWRequestTimeCost)
	app.Use(logger.New())
	app.Use(cache.New(middleware.CacheConfig))
	app.Static("/admin", "./static/admin")
	app.Static("/static", "./views/static")
	app.Static("/mos", static_config.StorageRootPath)
	SetupApiRouter(app)
	SetupViewRouter(app)
	app.Use(middleware.Custom404ErrorHandler)
	log.Fatal(app.Listen(*port))
}
