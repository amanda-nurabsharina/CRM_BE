package router

import (
	"crm-be/src/config"
	"crm-be/src/controller"
	"crm-be/src/middleware"
	"crm-be/src/provider"
	"crm-be/src/service"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func Routes(app *fiber.App, db *gorm.DB) {
	// Dynamically initialize WhatsApp Provider (Official Meta WABA Cloud API or Local Baileys Bridge)
	var waProvider provider.WhatsAppProvider
	if config.WAProviderType == "meta_waba" {
		waProvider = provider.NewMetaWABAProvider(
			config.MetaWABAPhoneNumberID,
			config.MetaWABABusinessAccountID,
			config.MetaWABAAccessToken,
		)
	} else {
		waProvider = provider.NewRealWhatsAppProvider("http://localhost:3001")
	}

	crmService := service.NewCRMService(db, waProvider)

	userService := service.NewUserService(db)
	tokenService := service.NewTokenService(db)
	authService := service.NewAuthService(db)

	// Initialize controllers
	authCtrl := controller.NewAuthController(authService, userService, tokenService)
	healthCtrl := controller.NewHealthCheckController()
	crmCtrl := controller.NewCRMController(crmService)

	v1 := app.Group("/v1")

	// Health route
	v1.Get("/health", healthCtrl.Check)

	// Public Webhooks & Forms
	v1.Get("/webhooks/whatsapp", crmCtrl.VerifyMetaWebhook)
	v1.Post("/webhooks/whatsapp", crmCtrl.WhatsAppWebhook)
	v1.Post("/admin/clear-inbox", crmCtrl.ClearInbox)
	v1.Post("/webhooks/voip", crmCtrl.VoIPWebhook)
	v1.Post("/webhooks/voice", crmCtrl.VoiceWebhook)
	v1.Get("/calls", crmCtrl.GetCallLogs)
	v1.Get("/calls/:id", crmCtrl.GetCallLogByID)
	v1.Post("/calls/route", crmCtrl.RouteCall)
	v1.Post("/calls/:id/transfer", crmCtrl.TransferCall)

	// Auth routes
	auth := v1.Group("/auth")
	auth.Post("/login", authCtrl.Login)
	auth.Post("/register", authCtrl.Register)
	auth.Post("/refresh", authCtrl.Refresh)
	auth.Post("/logout", authCtrl.Logout)

	// Protected routes
	protected := v1.Group("", middleware.Protected())
	protected.Get("/auth/me", authCtrl.Me)

	// Branch & User Management routes
	protected.Get("/branches", crmCtrl.GetBranches)
	protected.Post("/branches", crmCtrl.CreateBranch)
	protected.Put("/branches/:id", crmCtrl.UpdateBranch)
	protected.Get("/users", crmCtrl.GetUsers)
	protected.Post("/users", crmCtrl.CreateUser)

	// Lead Pipeline routes
	protected.Get("/leads", crmCtrl.GetLeads)
	protected.Post("/leads", crmCtrl.CreateLead)
	protected.Patch("/leads/:id/status", crmCtrl.UpdateLeadStatus)
	protected.Post("/leads/:id/handover", crmCtrl.HandoverLead)
	protected.Post("/leads/merge", crmCtrl.MergeLeads)

	// Conversation & Message routes
	protected.Get("/conversations", crmCtrl.GetConversations)
	protected.Post("/conversations/new", crmCtrl.CreateNewConversation)
	protected.Get("/conversations/:id/messages", crmCtrl.GetMessages)
	protected.Post("/conversations/:id/messages", crmCtrl.SendMessage)
	protected.Delete("/conversations/:id", crmCtrl.DeleteConversation)

	// Packages & Quotations
	protected.Get("/packages", crmCtrl.GetTourPackages)
	protected.Post("/packages", crmCtrl.CreateTourPackage)
	protected.Put("/packages/:id", crmCtrl.UpdateTourPackage)
	protected.Delete("/packages/:id", crmCtrl.DeleteTourPackage)
	protected.Post("/upload", crmCtrl.UploadFile)
	protected.Post("/quotations", crmCtrl.CreateQuotation)

	// Invoices & Dual-Check Payments
	protected.Get("/invoices", crmCtrl.GetInvoices)
	protected.Post("/invoices", crmCtrl.CreateInvoice)
	protected.Post("/payment-terms/:term_id/proof", crmCtrl.UploadPaymentProof)
	protected.Post("/payment-proofs/:proof_id/verify", crmCtrl.VerifyPaymentProof)

	// Booking Travelers & Passport/KTP Documents
	protected.Get("/travelers", crmCtrl.GetTravelers)
	protected.Post("/travelers", crmCtrl.CreateTraveler)
	protected.Put("/travelers/:id", crmCtrl.UpdateTraveler)
	protected.Delete("/travelers/:id", crmCtrl.DeleteTraveler)

	// Executive Analytics & Audit Trail
	protected.Get("/analytics/dashboard", crmCtrl.GetDashboardKPIs)
	protected.Get("/audit-logs", crmCtrl.GetAuditLogs)
}

