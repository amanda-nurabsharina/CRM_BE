package main

import (
	"context"
	"crm-be/src/config"
	"crm-be/src/database"
	"crm-be/src/middleware"
	"crm-be/src/router"
	"crm-be/src/utils"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"gorm.io/gorm"
)

func main() {
	utils.InitLogger()
	config.LoadConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := setupFiberApp()
	db := database.Connect()

	router.Routes(app, db)

	address := fmt.Sprintf("%s:%d", config.AppHost, config.AppPort)
	utils.Log.Infof("Starting WhatsApp CRM Backend server on %s", address)

	serverErrors := make(chan error, 1)
	go startServer(app, address, serverErrors)

	handleGracefulShutdown(ctx, app, db, serverErrors)
}

func setupFiberApp() *fiber.App {
	app := fiber.New(config.FiberConfig())

	app.Use(cors.New(cors.Config{
		AllowOrigins: config.CORSOrigin,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, HEAD, PUT, DELETE, PATCH, OPTIONS",
	}))
	app.Use("/v1/auth", middleware.LimiterConfig())
	app.Use(middleware.LoggerConfig())
	app.Use(helmet.New())
	app.Use(compress.New())
	app.Use(middleware.RecoverConfig())

	return app
}

func startServer(app *fiber.App, address string, errs chan<- error) {
	if err := app.Listen(address); err != nil {
		errs <- fmt.Errorf("error starting server: %w", err)
	}
}

func handleGracefulShutdown(ctx context.Context, app *fiber.App, db *gorm.DB, errs <-chan error) {
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errs:
		utils.Log.Errorf("Server error: %v", err)
	case sig := <-shutdown:
		utils.Log.Infof("Received shutdown signal: %v. Initiating graceful shutdown...", sig)

		if err := app.Shutdown(); err != nil {
			utils.Log.Errorf("Error during Fiber app shutdown: %v", err)
		}

		sqlDB, errDB := db.DB()
		if errDB == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}

		utils.Log.Info("Server stopped gracefully")
	}
}
