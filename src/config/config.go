package config

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/spf13/viper"
)

var (
	AppHost                  string
	AppPort                  int
	AppEnv                   string
	DBDriver                 string
	DBName                   string
	DBHost                   string
	DBPort                   int
	DBUser                   string
	DBPassword               string
	DBSslMode                string
	JWTSecret                string
	JWTExpirationHours       int
	JWTRefreshExpirationDays int
	CORSOrigin               string

	// Official Meta WABA Configuration
	WAProviderType            string
	MetaWABAPhoneNumberID     string
	MetaWABABusinessAccountID string
	MetaWABAAccessToken       string
	MetaWABAVerifyToken       string
)

func LoadConfig() {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")

	viper.SetDefault("APP_HOST", "0.0.0.0")
	viper.SetDefault("APP_PORT", 8000)
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("DB_DRIVER", "sqlite")
	viper.SetDefault("DB_NAME", "crm.db")
	viper.SetDefault("JWT_SECRET", "supersecret_crm_jwt_key_2026")
	viper.SetDefault("JWT_EXPIRATION_HOURS", 24)
	viper.SetDefault("JWT_REFRESH_EXPIRATION_DAYS", 7)
	viper.SetDefault("CORS_ORIGIN", "*")
	viper.SetDefault("WA_PROVIDER_TYPE", "meta_waba")
	viper.SetDefault("META_WABA_VERIFY_TOKEN", "dgt_crm_meta_webhook_verify_token_2026")

	if err := viper.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			log.Println("No .env file found, using defaults / system environment variables")
		} else {
			log.Printf("Warning reading config file: %v", err)
		}
	}

	viper.AutomaticEnv()

	AppHost = viper.GetString("APP_HOST")
	AppPort = viper.GetInt("APP_PORT")
	AppEnv = viper.GetString("APP_ENV")
	DBDriver = viper.GetString("DB_DRIVER")
	DBName = viper.GetString("DB_NAME")
	DBHost = viper.GetString("DB_HOST")
	DBPort = viper.GetInt("DB_PORT")
	DBUser = viper.GetString("DB_USER")
	DBPassword = viper.GetString("DB_PASSWORD")
	DBSslMode = viper.GetString("DB_SSLMODE")
	JWTSecret = viper.GetString("JWT_SECRET")
	JWTExpirationHours = viper.GetInt("JWT_EXPIRATION_HOURS")
	JWTRefreshExpirationDays = viper.GetInt("JWT_REFRESH_EXPIRATION_DAYS")
	CORSOrigin = viper.GetString("CORS_ORIGIN")

	WAProviderType = viper.GetString("WA_PROVIDER_TYPE")
	MetaWABAPhoneNumberID = viper.GetString("META_WABA_PHONE_NUMBER_ID")
	MetaWABABusinessAccountID = viper.GetString("META_WABA_BUSINESS_ACCOUNT_ID")
	MetaWABAAccessToken = viper.GetString("META_WABA_ACCESS_TOKEN")
	MetaWABAVerifyToken = viper.GetString("META_WABA_VERIFY_TOKEN")
}

func FiberConfig() fiber.Config {
	return fiber.Config{
		AppName:      "WhatsApp CRM Backend v1.0",
		ServerHeader: "Fiber",
		ErrorHandler: customErrorHandler,
	}
}

func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"code":    code,
		"status":  "error",
		"message": message,
	})
}
