package router

import (
	"crm-be/src/controller"
	"crm-be/src/middleware"
	"crm-be/src/provider"
	"crm-be/src/service"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func Routes(app *fiber.App, db *gorm.DB) {
	// Initialize WhatsApp Provider & Services (Connected to local WA Bridge)
	waProvider := provider.NewRealWhatsAppProvider("http://localhost:3001")
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
	v1.Post("/webhooks/whatsapp", crmCtrl.WhatsAppWebhook)

	// Auth routes
	auth := v1.Group("/auth")
	auth.Post("/login", authCtrl.Login)
	auth.Post("/register", authCtrl.Register)
	auth.Post("/refresh", authCtrl.Refresh)
	auth.Post("/logout", authCtrl.Logout)

	// Protected routes
	protected := v1.Group("/", middleware.Protected())
	protected.Get("/auth/me", authCtrl.Me)

	// Branch routes
	protected.Get("/branches", crmCtrl.GetBranches)
	protected.Post("/branches", crmCtrl.CreateBranch)
	protected.Put("/branches/:id", crmCtrl.UpdateBranch)

	// Lead Pipeline routes
	protected.Get("/leads", crmCtrl.GetLeads)
	protected.Post("/leads", crmCtrl.CreateLead)
	protected.Patch("/leads/:id/status", crmCtrl.UpdateLeadStatus)
	protected.Post("/leads/:id/handover", crmCtrl.HandoverLead)
	protected.Post("/leads/merge", crmCtrl.MergeLeads)

	// Conversation & Message routes
	protected.Get("/conversations", crmCtrl.GetConversations)
	protected.Get("/conversations/:id/messages", crmCtrl.GetMessages)
	protected.Post("/conversations/:id/messages", crmCtrl.SendMessage)
	protected.Delete("/conversations/:id", crmCtrl.DeleteConversation)

	// Packages & Quotations
	protected.Get("/packages", crmCtrl.GetTourPackages)
	protected.Post("/quotations", crmCtrl.CreateQuotation)

	// Invoices & Dual-Check Payments
	protected.Get("/invoices", crmCtrl.GetInvoices)
	protected.Post("/invoices", crmCtrl.CreateInvoice)
	protected.Post("/payment-terms/:term_id/proof", crmCtrl.UploadPaymentProof)
	protected.Post("/payment-proofs/:proof_id/verify", crmCtrl.VerifyPaymentProof)

	// Executive Analytics & Audit Trail
	protected.Get("/analytics/dashboard", crmCtrl.GetDashboardKPIs)
	protected.Get("/audit-logs", crmCtrl.GetAuditLogs)
}

