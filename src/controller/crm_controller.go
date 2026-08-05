package controller

import (
	"crm-be/src/config"
	"crm-be/src/model"
	"crm-be/src/response"
	"crm-be/src/service"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type CRMController struct {
	crmService *service.CRMService
}

func NewCRMController(crmService *service.CRMService) *CRMController {
	return &CRMController{crmService: crmService}
}

func success(c *fiber.Ctx, code int, msg string, data interface{}) error {
	return c.Status(code).JSON(response.StandardResponse{
		Code:    code,
		Status:  "success",
		Message: msg,
		Data:    data,
	})
}

// ----------------- Branches -----------------
func (c *CRMController) GetBranches(ctx *fiber.Ctx) error {
	branches, err := c.crmService.GetBranches()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Branches retrieved successfully", branches)
}

func (c *CRMController) UpdateBranch(ctx *fiber.Ctx) error {
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid branch ID")
	}

	var req struct {
		Name            string `json:"name"`
		Code            string `json:"code"`
		WAPhoneNumber   string `json:"wa_phone_number"`
		VoIPPhoneNumber string `json:"voip_phone_number"`
		CoverageAreas   string `json:"coverage_areas"`
		IsActive        bool   `json:"is_active"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userIDStr, _ := ctx.Locals("userId").(string)
	userID, _ := uuid.Parse(userIDStr)

	b, err := c.crmService.UpdateBranch(id, req.Name, req.Code, req.WAPhoneNumber, req.VoIPPhoneNumber, req.CoverageAreas, req.IsActive, userID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Branch configuration updated", b)
}

func (c *CRMController) CreateBranch(ctx *fiber.Ctx) error {
	var req struct {
		Name            string `json:"name"`
		Code            string `json:"code"`
		WAPhoneNumber   string `json:"wa_phone_number"`
		VoIPPhoneNumber string `json:"voip_phone_number"`
		CoverageAreas   string `json:"coverage_areas"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userIDStr, _ := ctx.Locals("userId").(string)
	userID, _ := uuid.Parse(userIDStr)

	b, err := c.crmService.CreateBranch(req.Name, req.Code, req.WAPhoneNumber, req.VoIPPhoneNumber, req.CoverageAreas, userID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusCreated, "Branch created successfully", b)
}

func (c *CRMController) GetUsers(ctx *fiber.Ctx) error {
	users, err := c.crmService.GetUsers()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Users fetched successfully", users)
}

func (c *CRMController) CreateUser(ctx *fiber.Ctx) error {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		BranchID string `json:"branch_id"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.Role == "" {
		req.Role = "ADMIN_CABANG"
	}
	if req.Password == "" {
		req.Password = "password123"
	}

	var branchID *uuid.UUID
	if req.BranchID != "" {
		bID, err := uuid.Parse(req.BranchID)
		if err == nil {
			branchID = &bID
		}
	}

	user, err := c.crmService.CreateUser(req.Name, req.Email, req.Password, req.Role, branchID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusCreated, "User account created successfully", user)
}

// ----------------- Leads -----------------
func (c *CRMController) GetLeads(ctx *fiber.Ctx) error {
	status := ctx.Query("status")
	var branchID *uuid.UUID

	if bStr := ctx.Query("branch_id"); bStr != "" && bStr != "ALL" {
		if id, err := uuid.Parse(bStr); err == nil {
			branchID = &id
		}
	}

	// Enforce branch filtering if logged in user is ADMIN_CABANG / SALES_AGENT
	if userIDStr, ok := ctx.Locals("userId").(string); ok && userIDStr != "" {
		if uID, err := uuid.Parse(userIDStr); err == nil {
			var user model.User
			if errU := c.crmService.GetUserByID(uID, &user); errU == nil {
				if user.Role != "ADMIN_PUSAT" && user.BranchID != nil {
					branchID = user.BranchID
				}
			}
		}
	}

	leads, err := c.crmService.GetLeads(branchID, status)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Leads retrieved successfully", leads)
}

func (c *CRMController) CreateLead(ctx *fiber.Ctx) error {
	var req struct {
		CustomerName string `json:"customer_name"`
		PhoneNumber  string `json:"phone_number"`
		Domicile     string `json:"domicile"`
		Source       string `json:"source"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	lead, err := c.crmService.RouteAndCreateLead(req.CustomerName, req.PhoneNumber, req.Domicile, req.Source)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusCreated, "Lead created and routed successfully", lead)
}

func (c *CRMController) UpdateLeadStatus(ctx *fiber.Ctx) error {
	leadID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid lead ID")
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userIDStr, _ := ctx.Locals("userId").(string)
	userID, _ := uuid.Parse(userIDStr)

	lead, err := c.crmService.UpdateLeadStatus(leadID, req.Status, userID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Lead status updated", lead)
}

func (c *CRMController) HandoverLead(ctx *fiber.Ctx) error {
	leadID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid lead ID")
	}

	var req struct {
		BranchID string `json:"branch_id"`
		Note     string `json:"note"`
	}
	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	branchID, err := uuid.Parse(req.BranchID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid branch ID")
	}

	userIDStr, _ := ctx.Locals("userId").(string)
	userID, _ := uuid.Parse(userIDStr)

	lead, err := c.crmService.HandoverLead(leadID, branchID, req.Note, userID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Lead handover completed successfully", lead)
}

func (c *CRMController) MergeLeads(ctx *fiber.Ctx) error {
	var req struct {
		PrimaryLeadID   string `json:"primary_lead_id"`
		DuplicateLeadID string `json:"duplicate_lead_id"`
	}
	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	pID, _ := uuid.Parse(req.PrimaryLeadID)
	dID, _ := uuid.Parse(req.DuplicateLeadID)
	userIDStr, _ := ctx.Locals("userId").(string)
	userID, _ := uuid.Parse(userIDStr)

	if err := c.crmService.MergeLeads(pID, dID, userID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Leads merged successfully", nil)
}

// ----------------- Conversations & Messages -----------------
func (c *CRMController) GetConversations(ctx *fiber.Ctx) error {
	var branchID *uuid.UUID
	if bStr := ctx.Query("branch_id"); bStr != "" && bStr != "ALL" {
		if id, err := uuid.Parse(bStr); err == nil {
			branchID = &id
		}
	}

	// Enforce branch filtering if logged in user is ADMIN_CABANG / SALES_AGENT
	if userIDStr, ok := ctx.Locals("userId").(string); ok && userIDStr != "" {
		if uID, err := uuid.Parse(userIDStr); err == nil {
			var user model.User
			if errU := c.crmService.GetUserByID(uID, &user); errU == nil {
				if user.Role != "ADMIN_PUSAT" && user.BranchID != nil {
					branchID = user.BranchID
				}
			}
		}
	}

	convs, err := c.crmService.GetConversations(branchID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Conversations retrieved", convs)
}

func (c *CRMController) ClearInbox(ctx *fiber.Ctx) error {
	if err := c.crmService.ClearInbox(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Inbox cleared successfully", nil)
}

func (c *CRMController) CreateNewConversation(ctx *fiber.Ctx) error {
	var req struct {
		Phone string `json:"phone"`
		Name  string `json:"name"`
		Text  string `json:"text"`
	}
	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.Phone == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Phone number is required")
	}

	userIDStr, _ := ctx.Locals("userId").(string)

	conv, err := c.crmService.CreateNewConversation(req.Phone, req.Name, req.Text, userIDStr)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusCreated, "Conversation created & message sent", conv)
}

func (c *CRMController) GetMessages(ctx *fiber.Ctx) error {
	convID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid conversation ID")
	}

	msgs, err := c.crmService.GetMessages(convID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Messages retrieved", msgs)
}

func (c *CRMController) SendMessage(ctx *fiber.Ctx) error {
	convID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid conversation ID")
	}

	var req struct {
		Text string `json:"text"`
	}
	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid body")
	}

	userIDStr, _ := ctx.Locals("userId").(string)

	msg, err := c.crmService.SendOutboundMessage(convID, userIDStr, req.Text)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Message sent", msg)
}

func (c *CRMController) DeleteConversation(ctx *fiber.Ctx) error {
	convID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid conversation ID")
	}

	userIDStr, _ := ctx.Locals("userId").(string)
	userID, _ := uuid.Parse(userIDStr)

	// Enforce role permission: Only ADMIN_PUSAT can delete WhatsApp conversations
	var user model.User
	if errU := c.crmService.GetUserByID(userID, &user); errU == nil {
		if user.Role != "ADMIN_PUSAT" {
			return fiber.NewError(fiber.StatusForbidden, "Hanya Admin Pusat yang memiliki wewenang untuk menghapus percakapan WhatsApp")
		}
	}

	if err := c.crmService.DeleteConversation(convID, userID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Conversation deleted successfully", nil)
}

func (c *CRMController) WhatsAppWebhook(ctx *fiber.Ctx) error {
	var req struct {
		FromPhone  string `json:"from_phone"`
		SenderName string `json:"sender_name"`
		Content    string `json:"content"`
		Direction  string `json:"direction"`
		MediaType  string `json:"media_type"`
		MediaURL   string `json:"media_url"`
		IsHistory  bool   `json:"is_history"`
		AvatarURL  string `json:"avatar_url"`
		SentAt     int64  `json:"sent_at"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid payload")
	}

	if req.MediaType == "" {
		req.MediaType = "TEXT"
	}
	if req.SenderName == "" {
		req.SenderName = "WhatsApp User (" + req.FromPhone + ")"
	}

	msg, err := c.crmService.ProcessInboundWebhook(req.FromPhone, req.SenderName, req.Content, req.MediaType, req.MediaURL, req.Direction, req.IsHistory, req.AvatarURL, req.SentAt)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return success(ctx, fiber.StatusOK, "Webhook processed successfully", msg)
}

func (c *CRMController) VoIPWebhook(ctx *fiber.Ctx) error {
	var req struct {
		FromPhone  string `json:"from_phone"`
		CallerName string `json:"caller_name"`
		SIPLine    string `json:"sip_line"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid payload")
	}

	if req.CallerName == "" {
		req.CallerName = "Panggilan Telepon VoIP (" + req.FromPhone + ")"
	}
	if req.SIPLine == "" {
		req.SIPLine = "SIP Line Kantor"
	}

	msgContent := fmt.Sprintf("📞 Panggilan VoIP Masuk dari %s (%s) [%s]", req.CallerName, req.FromPhone, req.SIPLine)
	msg, err := c.crmService.ProcessInboundWebhook(req.FromPhone, req.CallerName, msgContent, "CALL", "", "INBOUND", false, "", 0)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return success(ctx, fiber.StatusOK, "VoIP Inbound Call Webhook processed", msg)
}

func (c *CRMController) GetCallLogs(ctx *fiber.Ctx) error {
	logs, err := c.crmService.GetCallLogs()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Call logs fetched successfully", logs)
}

func (c *CRMController) GetCallLogByID(ctx *fiber.Ctx) error {
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid call log ID")
	}
	log, err := c.crmService.GetCallLogByID(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Call log not found")
	}
	return success(ctx, fiber.StatusOK, "Call log details fetched", log)
}

func (c *CRMController) RouteCall(ctx *fiber.Ctx) error {
	var req struct {
		CallerNumber string `json:"caller_number"`
		SIPLine      string `json:"sip_line"`
	}
	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	branch, lead, err := c.crmService.RouteCall(req.CallerNumber, req.SIPLine)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return success(ctx, fiber.StatusOK, "Call routed successfully", fiber.Map{
		"branch": branch,
		"lead":   lead,
	})
}

func (c *CRMController) TransferCall(ctx *fiber.Ctx) error {
	callIDStr := ctx.Params("id")
	var req struct {
		TargetAgentID   string `json:"target_agent_id"`
		TargetBranchID  string `json:"target_branch_id"`
		TargetExtension string `json:"target_extension"`
		Note            string `json:"note"`
		Reason          string `json:"reason"`
	}
	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	callID, err := uuid.Parse(callIDStr)
	if err == nil {
		var agentID uuid.UUID
		if req.TargetAgentID != "" {
			agentID, _ = uuid.Parse(req.TargetAgentID)
		}
		var branchID uuid.UUID
		if req.TargetBranchID != "" {
			branchID, _ = uuid.Parse(req.TargetBranchID)
		}
		note := req.Note
		if note == "" {
			note = req.Reason
		}
		callLog, errT := c.crmService.TransferCall(callID, agentID, branchID, note)
		if errT == nil {
			return success(ctx, fiber.StatusOK, "Call transferred successfully", callLog)
		}
	}

	meta := fmt.Sprintf("Transfer to Ext %s - Reason: %s", req.TargetExtension, req.Reason)
	if errE := c.crmService.RecordCallEvent(callIDStr, "TRANSFER", meta); errE != nil {
		return fiber.NewError(fiber.StatusInternalServerError, errE.Error())
	}

	return success(ctx, fiber.StatusOK, "Call transfer recorded successfully", nil)
}

// ----------------- Tour Packages & Quotations -----------------
func (c *CRMController) GetTourPackages(ctx *fiber.Ctx) error {
	pkgs, err := c.crmService.GetTourPackages()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Tour packages retrieved", pkgs)
}

func (c *CRMController) CreateTourPackage(ctx *fiber.Ctx) error {
	var req struct {
		Title           string  `json:"title"`
		Destination     string  `json:"destination"`
		DurationDays    int     `json:"duration_days"`
		BasePrice       float64 `json:"base_price"`
		ItineraryJSON   string  `json:"itinerary_json"`
		TermsConditions string  `json:"terms_conditions"`
		PdfUrl          string  `json:"pdf_url"`
		WaTemplate      string  `json:"wa_template"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid payload")
	}

	pkg, err := c.crmService.CreateTourPackage(req.Title, req.Destination, req.DurationDays, req.BasePrice, req.ItineraryJSON, req.TermsConditions, req.PdfUrl, req.WaTemplate)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusCreated, "Package created successfully", pkg)
}

func (c *CRMController) UpdateTourPackage(ctx *fiber.Ctx) error {
	idStr := ctx.Params("id")
	pkgID, err := uuid.Parse(idStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid package ID")
	}

	var req struct {
		Title           string  `json:"title"`
		Destination     string  `json:"destination"`
		DurationDays    int     `json:"duration_days"`
		BasePrice       float64 `json:"base_price"`
		ItineraryJSON   string  `json:"itinerary_json"`
		TermsConditions string  `json:"terms_conditions"`
		PdfUrl          string  `json:"pdf_url"`
		WaTemplate      string  `json:"wa_template"`
		IsActive        bool    `json:"is_active"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid payload")
	}

	pkg, err := c.crmService.UpdateTourPackage(pkgID, req.Title, req.Destination, req.DurationDays, req.BasePrice, req.ItineraryJSON, req.TermsConditions, req.PdfUrl, req.WaTemplate, req.IsActive)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Package updated successfully", pkg)
}

func (c *CRMController) DeleteTourPackage(ctx *fiber.Ctx) error {
	idStr := ctx.Params("id")
	pkgID, err := uuid.Parse(idStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid package ID")
	}

	if err := c.crmService.DeleteTourPackage(pkgID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Package deleted successfully", nil)
}

func (c *CRMController) UploadFile(ctx *fiber.Ctx) error {
	file, err := ctx.FormFile("file")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "No file uploaded")
	}

	uploadDir := "./uploads/brochures"
	os.MkdirAll(uploadDir, 0755)

	filename := fmt.Sprintf("%d_%s", time.Now().Unix(), filepath.Base(file.Filename))
	savePath := filepath.Join(uploadDir, filename)

	if err := ctx.SaveFile(file, savePath); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to save file")
	}

	fileURL := fmt.Sprintf("http://localhost:8000/uploads/brochures/%s", filename)
	return success(ctx, fiber.StatusOK, "File uploaded successfully", fiber.Map{
		"url":       fileURL,
		"file_name": file.Filename,
	})
}

func (c *CRMController) CreateQuotation(ctx *fiber.Ctx) error {
	var req struct {
		LeadID            string  `json:"lead_id"`
		PackageID         string  `json:"package_id"`
		PaxCount          int     `json:"pax_count"`
		PricePerPax       float64 `json:"price_per_pax"`
		AddOnsJSON        string  `json:"add_ons_json"`
		CustomPriceReason string  `json:"custom_price_reason"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid payload")
	}

	leadID, _ := uuid.Parse(req.LeadID)
	pkgID, _ := uuid.Parse(req.PackageID)
	userIDStr, _ := ctx.Locals("userId").(string)
	userID, _ := uuid.Parse(userIDStr)

	quote, err := c.crmService.CreateQuotation(leadID, pkgID, userID, req.PaxCount, req.PricePerPax, req.AddOnsJSON, req.CustomPriceReason)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusCreated, "Quotation generated successfully", quote)
}

// ----------------- Invoices & Dual-Check Payments -----------------
func (c *CRMController) GetInvoices(ctx *fiber.Ctx) error {
	var branchID *uuid.UUID
	if bStr := ctx.Query("branch_id"); bStr != "" {
		if id, err := uuid.Parse(bStr); err == nil {
			branchID = &id
		}
	}

	invs, err := c.crmService.GetInvoices(branchID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Invoices retrieved", invs)
}

func (c *CRMController) CreateInvoice(ctx *fiber.Ctx) error {
	var req struct {
		LeadID      string `json:"lead_id"`
		QuotationID string `json:"quotation_id"`
		PaymentType string `json:"payment_type"`
		TermsCount  int    `json:"terms_count"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid payload")
	}

	leadID, _ := uuid.Parse(req.LeadID)
	quoteID, _ := uuid.Parse(req.QuotationID)
	userIDStr, _ := ctx.Locals("userId").(string)
	userID, _ := uuid.Parse(userIDStr)

	inv, err := c.crmService.CreateInvoice(leadID, quoteID, userID, req.PaymentType, req.TermsCount)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusCreated, "Invoice generated", inv)
}

func (c *CRMController) UploadPaymentProof(ctx *fiber.Ctx) error {
	termID, err := uuid.Parse(ctx.Params("term_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid payment term ID")
	}

	var req struct {
		ProofImageURL string  `json:"proof_image_url"`
		Amount        float64 `json:"amount"`
		BankName      string  `json:"bank_name"`
	}
	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid payload")
	}

	userIDStr, _ := ctx.Locals("userId").(string)
	userID, _ := uuid.Parse(userIDStr)

	proof, err := c.crmService.UploadPaymentProof(termID, userID, req.ProofImageURL, req.Amount, req.BankName)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusCreated, "Proof uploaded, pending Pusat verification", proof)
}

func (c *CRMController) VerifyPaymentProof(ctx *fiber.Ctx) error {
	proofID, err := uuid.Parse(ctx.Params("proof_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid proof ID")
	}

	var req struct {
		Approved bool   `json:"approved"`
		Notes    string `json:"notes"`
	}
	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid payload")
	}

	userIDStr, _ := ctx.Locals("userId").(string)
	pusatUserID, _ := uuid.Parse(userIDStr)

	if err := c.crmService.VerifyPaymentProof(proofID, pusatUserID, req.Approved, req.Notes); err != nil {
		return fiber.NewError(fiber.StatusForbidden, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Payment verification updated", nil)
}

// ----------------- Executive Analytics & Audit Trail -----------------
func (c *CRMController) GetDashboardKPIs(ctx *fiber.Ctx) error {
	var branchID *uuid.UUID
	if bStr := ctx.Query("branch_id"); bStr != "" {
		if id, err := uuid.Parse(bStr); err == nil {
			branchID = &id
		}
	}

	kpi, err := c.crmService.GetDashboardKPIs(branchID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Dashboard KPIs retrieved", kpi)
}

func (c *CRMController) GetAuditLogs(ctx *fiber.Ctx) error {
	logs, err := c.crmService.GetAuditLogs()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Audit logs retrieved", logs)
}

// ----------------- Traveler Documents -----------------
func (c *CRMController) GetTravelers(ctx *fiber.Ctx) error {
	var leadID *uuid.UUID
	if lStr := ctx.Query("lead_id"); lStr != "" {
		if id, err := uuid.Parse(lStr); err == nil {
			leadID = &id
		}
	}

	travelers, err := c.crmService.GetTravelers(leadID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Travelers retrieved", travelers)
}

func (c *CRMController) CreateTraveler(ctx *fiber.Ctx) error {
	var req struct {
		LeadID           string `json:"lead_id"`
		FullName         string `json:"full_name"`
		IDCardNumber     string `json:"id_card_number"`
		PassportNumber   string `json:"passport_number"`
		PassportExpiry   string `json:"passport_expiry"`
		BirthDate        string `json:"birth_date"`
		KtpPhotoUrl      string `json:"ktp_photo_url"`
		PassportPhotoUrl string `json:"passport_photo_url"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid payload")
	}

	leadID, err := uuid.Parse(req.LeadID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid Lead ID")
	}

	var passExp *time.Time
	if req.PassportExpiry != "" {
		if t, errP := time.Parse("2006-01-02", req.PassportExpiry); errP == nil {
			passExp = &t
		}
	}

	var bDate *time.Time
	if req.BirthDate != "" {
		if t, errB := time.Parse("2006-01-02", req.BirthDate); errB == nil {
			bDate = &t
		}
	}

	traveler, err := c.crmService.CreateTraveler(leadID, req.FullName, req.IDCardNumber, req.PassportNumber, passExp, bDate, req.KtpPhotoUrl, req.PassportPhotoUrl)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusCreated, "Traveler created successfully", traveler)
}

func (c *CRMController) UpdateTraveler(ctx *fiber.Ctx) error {
	idStr := ctx.Params("id")
	tID, err := uuid.Parse(idStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid Traveler ID")
	}

	var req struct {
		FullName         string `json:"full_name"`
		IDCardNumber     string `json:"id_card_number"`
		PassportNumber   string `json:"passport_number"`
		PassportExpiry   string `json:"passport_expiry"`
		BirthDate        string `json:"birth_date"`
		KtpPhotoUrl      string `json:"ktp_photo_url"`
		PassportPhotoUrl string `json:"passport_photo_url"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid payload")
	}

	var passExp *time.Time
	if req.PassportExpiry != "" {
		if t, errP := time.Parse("2006-01-02", req.PassportExpiry); errP == nil {
			passExp = &t
		}
	}

	var bDate *time.Time
	if req.BirthDate != "" {
		if t, errB := time.Parse("2006-01-02", req.BirthDate); errB == nil {
			bDate = &t
		}
	}

	traveler, err := c.crmService.UpdateTraveler(tID, req.FullName, req.IDCardNumber, req.PassportNumber, passExp, bDate, req.KtpPhotoUrl, req.PassportPhotoUrl)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Traveler updated successfully", traveler)
}

func (c *CRMController) DeleteTraveler(ctx *fiber.Ctx) error {
	idStr := ctx.Params("id")
	tID, err := uuid.Parse(idStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid Traveler ID")
	}

	if err := c.crmService.DeleteTraveler(tID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return success(ctx, fiber.StatusOK, "Traveler deleted successfully", nil)
}

// ----------------- Meta WABA Webhook Verification -----------------
func (c *CRMController) VerifyMetaWebhook(ctx *fiber.Ctx) error {
	mode := ctx.Query("hub.mode")
	token := ctx.Query("hub.verify_token")
	challenge := ctx.Query("hub.challenge")

	if mode == "subscribe" && token == config.MetaWABAVerifyToken {
		return ctx.Status(fiber.StatusOK).SendString(challenge)
	}
	return ctx.Status(fiber.StatusForbidden).SendString("Verification token mismatch")
}

// ----------------- Production Voice Webhook & Call Routing (PDF Spec 5) -----------------
func (c *CRMController) VoiceWebhook(ctx *fiber.Ctx) error {
	var req struct {
		Provider          string `json:"provider"`
		ProviderCallID    string `json:"provider_call_id"`
		Channel           string `json:"channel"`
		Direction         string `json:"direction"`
		CallerNumber      string `json:"caller_number"`
		DestinationNumber string `json:"destination_number"`
		Status            string `json:"status"`
		DurationSeconds   int    `json:"duration_seconds"`
		RecordingURL      string `json:"recording_url"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid voice webhook payload")
	}

	if req.Provider == "" {
		req.Provider = "WHATSAPP_BUSINESS"
	}
	if req.Channel == "" {
		req.Channel = "WHATSAPP"
	}
	if req.Direction == "" {
		req.Direction = "INBOUND"
	}
	if req.Status == "" {
		req.Status = "RINGING"
	}
	if req.ProviderCallID == "" {
		req.ProviderCallID = fmt.Sprintf("CALL-%d", time.Now().UnixNano())
	}

	callLog, err := c.crmService.ProcessVoiceWebhook(req.Provider, req.ProviderCallID, req.Channel, req.Direction, req.CallerNumber, req.DestinationNumber, req.Status, req.DurationSeconds, req.RecordingURL)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return success(ctx, fiber.StatusOK, "Voice webhook processed successfully", callLog)
}
