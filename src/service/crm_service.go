package service

import (
	"context"
	"crm-be/src/model"
	"crm-be/src/provider"
	"crm-be/src/utils"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CRMService struct {
	db       *gorm.DB
	waClient provider.WhatsAppProvider
}

func NewCRMService(db *gorm.DB, waClient provider.WhatsAppProvider) *CRMService {
	return &CRMService{
		db:       db,
		waClient: waClient,
	}
}

// ----------------- Audit Trail Helper -----------------
func (s *CRMService) LogAudit(userID *uuid.UUID, branchID *uuid.UUID, actionType, entityName, entityID string, beforeVal, afterVal interface{}, ip string) {
	beforeJSON, _ := json.Marshal(beforeVal)
	afterJSON, _ := json.Marshal(afterVal)

	log := model.AuditLog{
		UserID:          userID,
		BranchID:        branchID,
		ActionType:      actionType,
		EntityName:      entityName,
		EntityID:        entityID,
		BeforeValueJSON: string(beforeJSON),
		AfterValueJSON:  string(afterJSON),
		IPAddress:       ip,
	}
	s.db.Create(&log)
}

// ----------------- Branches -----------------
func (s *CRMService) GetBranches() ([]model.Branch, error) {
	var branches []model.Branch
	err := s.db.Find(&branches).Error
	return branches, err
}

func (s *CRMService) UpdateBranch(id uuid.UUID, name, code, waPhone, coverageAreas string, isActive bool, userID uuid.UUID) (*model.Branch, error) {
	var b model.Branch
	if err := s.db.First(&b, id).Error; err != nil {
		return nil, err
	}
	old := b
	b.Name = name
	b.Code = code
	b.WAPhoneNumber = waPhone
	b.CoverageAreas = coverageAreas
	b.IsActive = isActive

	if err := s.db.Save(&b).Error; err != nil {
		return nil, err
	}

	s.LogAudit(&userID, &b.ID, "BRANCH_UPDATED", "branches", b.ID.String(), old, b, "")
	return &b, nil
}

func (s *CRMService) GetUsers() ([]model.User, error) {
	var users []model.User
	err := s.db.Preload("Branch").Order("created_at desc").Find(&users).Error
	return users, err
}

func (s *CRMService) GetUserByID(id uuid.UUID, user *model.User) error {
	return s.db.Preload("Branch").First(user, id).Error
}

func (s *CRMService) CreateUser(name, email, password, role string, branchID *uuid.UUID) (*model.User, error) {
	hashedPass, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := model.User{
		Name:     name,
		Email:    email,
		Password: hashedPass,
		Role:     role,
		BranchID: branchID,
		IsActive: true,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	s.LogAudit(nil, branchID, "USER_CREATED", "users", user.ID.String(), nil, user, "Created user account")
	return &user, nil
}

func (s *CRMService) CreateBranch(name, code, waPhone, coverageAreas string, userID uuid.UUID) (*model.Branch, error) {
	b := model.Branch{
		Name:          name,
		Code:          code,
		WAPhoneNumber: waPhone,
		CoverageAreas: coverageAreas,
		IsActive:      true,
	}
	if err := s.db.Create(&b).Error; err != nil {
		return nil, err
	}

	s.LogAudit(&userID, &b.ID, "BRANCH_CREATED", "branches", b.ID.String(), nil, b, "")
	return &b, nil
}

// ----------------- Lead Auto Routing & Duplicate Engine -----------------
func (s *CRMService) RouteAndCreateLead(customerName, phone, domicile, source string) (*model.Lead, error) {
	if phone == "status" || strings.Contains(phone, "@g.us") || len(phone) > 17 {
		return nil, fmt.Errorf("ignoring non-individual WA contact: %s", phone)
	}

	// 1. Determine Branch by Domicile / Content keywords
	var matchedBranch *model.Branch
	var branches []model.Branch
	s.db.Where("is_active = ?", true).Find(&branches)

	domicileLower := strings.ToLower(domicile)
	if domicileLower != "" {
		for i, b := range branches {
			areas := strings.Split(strings.ToLower(b.CoverageAreas), ",")
			for _, area := range areas {
				cleanArea := strings.TrimSpace(area)
				if cleanArea != "" && strings.Contains(domicileLower, cleanArea) {
					matchedBranch = &branches[i]
					break
				}
			}
			if matchedBranch != nil {
				break
			}
		}
	}

	// 2. Check duplicate lead by phone number
	var existingLead model.Lead
	err := s.db.Where("phone_number = ? AND is_merged = ?", phone, false).First(&existingLead).Error
	if err == nil {
		// Existing active lead found — if a specific branch match (e.g. BSD -> Tangerang) was found, update lead's branch!
		if matchedBranch != nil && (existingLead.BranchID == nil || *existingLead.BranchID != matchedBranch.ID) {
			existingLead.BranchID = &matchedBranch.ID
			existingLead.Domicile = matchedBranch.Name
			s.db.Save(&existingLead)

			// Update active conversation branch as well
			s.db.Model(&model.Conversation{}).Where("lead_id = ?", existingLead.ID).Update("branch_id", matchedBranch.ID)
		}
		return &existingLead, nil
	}

	if matchedBranch == nil {
		var pusatBranch model.Branch
		if errP := s.db.Where("code = ?", "PUSAT").First(&pusatBranch).Error; errP == nil {
			matchedBranch = &pusatBranch
		} else if len(branches) > 0 {
			matchedBranch = &branches[0]
		}
	}

	var branchID *uuid.UUID
	displayDomicile := "Pusat"
	if matchedBranch != nil {
		branchID = &matchedBranch.ID
		displayDomicile = matchedBranch.Name
	}

	lead := model.Lead{
		CustomerName: customerName,
		PhoneNumber:  phone,
		Domicile:     displayDomicile,
		Source:       source,
		Status:       "NEW",
		BranchID:     branchID,
	}

	if err := s.db.Create(&lead).Error; err != nil {
		return nil, err
	}

	// Create initial conversation
	if branchID != nil {
		conv := model.Conversation{
			LeadID:        lead.ID,
			BranchID:      *branchID,
			Status:        "ACTIVE",
			LastMessageAt: time.Now(),
		}
		s.db.Create(&conv)
	}

	s.LogAudit(nil, branchID, "LEAD_CREATED", "leads", lead.ID.String(), nil, lead, "SYSTEM")

	return &lead, nil
}

func (s *CRMService) GetLeads(branchID *uuid.UUID, status string) ([]model.Lead, error) {
	var leads []model.Lead
	query := s.db.Preload("Branch").Preload("AssignedUser").Where("is_merged = ?", false)

	if branchID != nil {
		query = query.Where("branch_id = ?", branchID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Order("updated_at desc").Find(&leads).Error
	return leads, err
}

func (s *CRMService) UpdateLeadStatus(leadID uuid.UUID, newStatus string, userID uuid.UUID) (*model.Lead, error) {
	var lead model.Lead
	if err := s.db.First(&lead, leadID).Error; err != nil {
		return nil, err
	}

	oldStatus := lead.Status
	lead.Status = newStatus

	if oldStatus == "NEW" && lead.FirstResponseAt == nil {
		now := time.Now()
		lead.FirstResponseAt = &now
	}

	if err := s.db.Save(&lead).Error; err != nil {
		return nil, err
	}

	s.LogAudit(&userID, lead.BranchID, "LEAD_STATUS_CHANGED", "leads", lead.ID.String(), map[string]string{"status": oldStatus}, map[string]string{"status": newStatus}, "")

	return &lead, nil
}

func (s *CRMService) HandoverLead(leadID uuid.UUID, targetBranchID uuid.UUID, note string, userID uuid.UUID) (*model.Lead, error) {
	var lead model.Lead
	if err := s.db.First(&lead, leadID).Error; err != nil {
		return nil, err
	}

	var targetBranch model.Branch
	if err := s.db.First(&targetBranch, targetBranchID).Error; err == nil {
		lead.Domicile = targetBranch.Name
	}

	oldBranchID := lead.BranchID
	lead.BranchID = &targetBranchID
	if note != "" {
		lead.HandoverNote = note
	}

	if err := s.db.Save(&lead).Error; err != nil {
		return nil, err
	}

	// Update active conversation's branch ID as well
	s.db.Model(&model.Conversation{}).Where("lead_id = ?", leadID).Update("branch_id", targetBranchID)

	s.LogAudit(&userID, &targetBranchID, "LEAD_HANDOVER", "leads", lead.ID.String(), map[string]interface{}{"branch_id": oldBranchID}, map[string]interface{}{"branch_id": targetBranchID, "note": note}, "Handover between branches")

	return &lead, nil
}

func (s *CRMService) MergeLeads(primaryLeadID, duplicateLeadID uuid.UUID, userID uuid.UUID) error {
	var primary, duplicate model.Lead
	if err := s.db.First(&primary, primaryLeadID).Error; err != nil {
		return err
	}
	if err := s.db.First(&duplicate, duplicateLeadID).Error; err != nil {
		return err
	}

	duplicate.IsMerged = true
	duplicate.MergedToLeadID = &primaryLeadID
	s.db.Save(&duplicate)

	// Transfer conversations to primary lead
	s.db.Model(&model.Conversation{}).Where("lead_id = ?", duplicateLeadID).Update("lead_id", primaryLeadID)

	s.LogAudit(&userID, primary.BranchID, "LEAD_MERGED", "leads", primary.ID.String(), duplicate, primary, "")
	return nil
}

// ----------------- Conversations & Messages -----------------
func (s *CRMService) GetConversations(branchID *uuid.UUID) ([]model.Conversation, error) {
	var convs []model.Conversation
	query := s.db.Preload("Lead").Preload("Lead.Branch")

	if branchID != nil {
		query = query.Where("branch_id = ?", branchID)
	}

	err := query.Order("last_message_at desc").Find(&convs).Error
	return convs, err
}

func (s *CRMService) GetMessages(convID uuid.UUID) ([]model.Message, error) {
	var msgs []model.Message
	err := s.db.Where("conversation_id = ?", convID).Order("created_at asc").Find(&msgs).Error
	return msgs, err
}

func (s *CRMService) DeleteConversation(convID uuid.UUID, userID uuid.UUID) error {
	var conv model.Conversation
	if err := s.db.First(&conv, convID).Error; err != nil {
		return err
	}

	// Delete all messages in conversation
	s.db.Where("conversation_id = ?", convID).Unscoped().Delete(&model.Message{})

	// Delete associated lead
	if conv.LeadID != uuid.Nil {
		s.db.Where("id = ?", conv.LeadID).Unscoped().Delete(&model.Lead{})
	}

	// Delete conversation
	if err := s.db.Unscoped().Delete(&conv).Error; err != nil {
		return err
	}

	s.LogAudit(&userID, &conv.ID, "CONVERSATION_DELETED", "conversations", convID.String(), conv, nil, "Chat deleted for testing")
	return nil
}

func (s *CRMService) SendOutboundMessage(convID uuid.UUID, senderID string, text string) (*model.Message, error) {
	var conv model.Conversation
	if err := s.db.Preload("Lead").Preload("Lead.Branch").First(&conv, convID).Error; err != nil {
		return nil, err
	}

	branchCode := "JKT_PST"
	if conv.Lead != nil && conv.Lead.Branch != nil {
		branchCode = conv.Lead.Branch.Code
	}

	resp, err := s.waClient.SendText(context.Background(), provider.SendTextRequest{
		ToPhone:        conv.Lead.PhoneNumber,
		BranchCode:     branchCode,
		Text:           text,
		ConversationID: conv.ID.String(),
	})
	if err != nil {
		return nil, err
	}

	msg := model.Message{
		ConversationID:    convID,
		SenderType:        "ADMIN",
		SenderID:          senderID,
		Direction:         "OUTBOUND",
		MessageType:       "TEXT",
		Content:           text,
		ExternalMessageID: resp.ExternalMessageID,
		Status:            resp.Status,
		SentAt:            time.Now(),
	}

	s.db.Create(&msg)

	// Update conversation last message timestamp
	conv.LastMessageAt = time.Now()
	s.db.Save(&conv)

	return &msg, nil
}

func (s *CRMService) ProcessInboundWebhook(phone, senderName, content, mediaType, mediaURL, direction string) (*model.Message, error) {
	if direction == "" {
		direction = "INBOUND"
	}
	senderType := "CUSTOMER"
	if direction == "OUTBOUND" {
		senderType = "ADMIN"
	}

	// 1. Find or create lead via auto-routing using message content for location detection
	lead, err := s.RouteAndCreateLead(senderName, phone, content, "WHATSAPP")
	if err != nil {
		return nil, err
	}

	// 2. Find conversation
	var conv model.Conversation
	err = s.db.Where("lead_id = ?", lead.ID).First(&conv).Error
	if err != nil {
		var branchID uuid.UUID
		if lead.BranchID != nil {
			branchID = *lead.BranchID
		} else {
			var fallbackBranch model.Branch
			if errB := s.db.First(&fallbackBranch).Error; errB == nil {
				branchID = fallbackBranch.ID
				lead.BranchID = &fallbackBranch.ID
				s.db.Save(lead)
			}
		}

		conv = model.Conversation{
			LeadID:        lead.ID,
			BranchID:      branchID,
			Status:        "ACTIVE",
			LastMessageAt: time.Now(),
		}
		s.db.Create(&conv)
	}

	msgType := mediaType
	if msgType == "" || msgType == "CALL" {
		msgType = "DOCUMENT"
	}

	msg := model.Message{
		ConversationID: conv.ID,
		SenderType:     senderType,
		SenderID:       phone,
		Direction:      direction,
		MessageType:    msgType,
		Content:        content,
		MediaURL:       mediaURL,
		Status:         "DELIVERED",
		SentAt:         time.Now(),
	}

	s.db.Create(&msg)

	conv.LastMessageAt = time.Now()
	s.db.Save(&conv)

	// Log Audit Trail for Inbound WhatsApp Calls
	if mediaType == "CALL" || strings.Contains(content, "Panggilan") || strings.Contains(content, "Call") {
		auditDesc := fmt.Sprintf("Panggilan WhatsApp Masuk: %s (+%s)", lead.CustomerName, phone)
		s.LogAudit(nil, lead.BranchID, "INBOUND_CALL", "conversations", conv.ID.String(), nil, auditDesc, "SYSTEM")
	}

	return &msg, nil
}

// ----------------- Tour Packages & Quotations -----------------
func (s *CRMService) GetTourPackages() ([]model.TourPackage, error) {
	var pkgs []model.TourPackage
	err := s.db.Find(&pkgs).Error
	return pkgs, err
}

func (s *CRMService) CreateTourPackage(title, destination string, durationDays int, basePrice float64, itineraryJSON, termsConditions, pdfUrl, waTemplate string) (*model.TourPackage, error) {
	pkg := model.TourPackage{
		Title:           title,
		Destination:     destination,
		DurationDays:    durationDays,
		BasePrice:       basePrice,
		ItineraryJSON:   itineraryJSON,
		TermsConditions: termsConditions,
		PdfUrl:          pdfUrl,
		WaTemplate:      waTemplate,
		IsActive:        true,
	}
	if err := s.db.Create(&pkg).Error; err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (s *CRMService) UpdateTourPackage(id uuid.UUID, title, destination string, durationDays int, basePrice float64, itineraryJSON, termsConditions, pdfUrl, waTemplate string, isActive bool) (*model.TourPackage, error) {
	var pkg model.TourPackage
	if err := s.db.First(&pkg, id).Error; err != nil {
		return nil, err
	}

	pkg.Title = title
	pkg.Destination = destination
	pkg.DurationDays = durationDays
	pkg.BasePrice = basePrice
	pkg.ItineraryJSON = itineraryJSON
	pkg.TermsConditions = termsConditions
	pkg.PdfUrl = pdfUrl
	pkg.WaTemplate = waTemplate
	pkg.IsActive = isActive

	if err := s.db.Save(&pkg).Error; err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (s *CRMService) DeleteTourPackage(id uuid.UUID) error {
	return s.db.Delete(&model.TourPackage{}, id).Error
}

func (s *CRMService) CreateQuotation(leadID, packageID, userID uuid.UUID, pax int, pricePerPax float64, addOnsJSON, customReason string) (*model.Quotation, error) {
	var lead model.Lead
	if err := s.db.First(&lead, leadID).Error; err != nil {
		return nil, err
	}

	var pkg model.TourPackage
	if err := s.db.First(&pkg, packageID).Error; err != nil {
		return nil, err
	}

	total := pricePerPax * float64(pax)
	quoteNo := fmt.Sprintf("Q-%s-%d", time.Now().Format("060102"), time.Now().Unix()%10000)

	branchID := uuid.Nil
	if lead.BranchID != nil {
		branchID = *lead.BranchID
	} else {
		var pusat model.Branch
		if errP := s.db.Where("code = ?", "PUSAT").First(&pusat).Error; errP == nil {
			branchID = pusat.ID
		}
	}

	if userID == uuid.Nil {
		var adminPusat model.User
		if errU := s.db.Where("role = ?", "ADMIN_PUSAT").First(&adminPusat).Error; errU == nil {
			userID = adminPusat.ID
		}
	}

	quote := model.Quotation{
		QuotationNumber:   quoteNo,
		LeadID:            leadID,
		PackageID:         packageID,
		BranchID:          branchID,
		CreatedByUserID:   userID,
		PaxCount:          pax,
		PricePerPax:       pricePerPax,
		AddOnsJSON:        addOnsJSON,
		TotalAmount:       total,
		CustomPriceReason: customReason,
		ValidUntil:        time.Now().AddDate(0, 0, 3), // Default 3 days expiry
		Status:            "ACTIVE",
		PDFUrl:            fmt.Sprintf("/documents/quotes/%s.pdf", quoteNo),
	}

	if err := s.db.Create(&quote).Error; err != nil {
		return nil, err
	}

	// Update lead status to QUOTATION_SENT
	lead.Status = "QUOTATION_SENT"
	s.db.Save(&lead)

	// Compose & Send Official WhatsApp Quotation Message (Support custom template & PDF brochure)
	quoteMsg := ""
	if pkg.WaTemplate != "" {
		quoteMsg = pkg.WaTemplate
		quoteMsg = strings.ReplaceAll(quoteMsg, "{customer_name}", lead.CustomerName)
		quoteMsg = strings.ReplaceAll(quoteMsg, "{quote_no}", quoteNo)
		quoteMsg = strings.ReplaceAll(quoteMsg, "{package_title}", pkg.Title)
		quoteMsg = strings.ReplaceAll(quoteMsg, "{pax}", fmt.Sprintf("%d", pax))
		quoteMsg = strings.ReplaceAll(quoteMsg, "{price_per_pax}", formatRupiah(pricePerPax))
		quoteMsg = strings.ReplaceAll(quoteMsg, "{total_price}", formatRupiah(total))
		quoteMsg = strings.ReplaceAll(quoteMsg, "{valid_until}", quote.ValidUntil.Format("02 Jan 2006"))
		pdfLink := pkg.PdfUrl
		if pdfLink == "" {
			pdfLink = quote.PDFUrl
		}
		quoteMsg = strings.ReplaceAll(quoteMsg, "{pdf_url}", pdfLink)
	} else {
		pdfNotice := ""
		if pkg.PdfUrl != "" {
			pdfNotice = fmt.Sprintf("\n• Brosur / Itinerary PDF: %s", pkg.PdfUrl)
		}
		quoteMsg = fmt.Sprintf(
			"📄 *PENAWARAN HARGA RESMI (%s)*\n\n"+
				"Halo Kak %s,\n"+
				"Berikut detail penawaran resmi untuk paket *%s*:\n\n"+
				"• Jumlah Pax: %d Orang\n"+
				"• Harga per Pax: Rp %s\n"+
				"• *Total Biaya: Rp %s*\n"+
				"• Masa Berlaku: s/d %s%s\n\n"+
				"Silakan hubungi kami melalui pesan ini untuk konfirmasi pesanan & rincian pembayaran. Terima kasih!",
			quoteNo,
			lead.CustomerName,
			pkg.Title,
			pax,
			formatRupiah(pricePerPax),
			formatRupiah(total),
			quote.ValidUntil.Format("02 Jan 2006"),
			pdfNotice,
		)
	}

	// Get or Create Conversation
	var conv model.Conversation
	if errC := s.db.Where("lead_id = ?", leadID).First(&conv).Error; errC != nil {
		conv = model.Conversation{
			LeadID:        leadID,
			BranchID:      branchID,
			Status:        "OPEN",
			LastMessageAt: time.Now(),
		}
		s.db.Create(&conv)
	}

	branchCode := "PUSAT"
	if lead.BranchID != nil {
		var b model.Branch
		if errB := s.db.First(&b, *lead.BranchID).Error; errB == nil {
			branchCode = b.Code
		}
	}

	waResp, errWa := s.waClient.SendText(context.Background(), provider.SendTextRequest{
		ToPhone:        lead.PhoneNumber,
		BranchCode:     branchCode,
		Text:           quoteMsg,
		ConversationID: conv.ID.String(),
	})

	extMsgID := ""
	statusMsg := "DELIVERED"
	if errWa == nil && waResp != nil {
		extMsgID = waResp.ExternalMessageID
		if waResp.Status != "" {
			statusMsg = waResp.Status
		}
	}

	// Save outbound message to conversation record
	msg := model.Message{
		ConversationID:    conv.ID,
		SenderType:        "ADMIN",
		SenderID:          userID.String(),
		Direction:         "OUTBOUND",
		MessageType:       "TEXT",
		Content:           quoteMsg,
		ExternalMessageID: extMsgID,
		Status:            statusMsg,
		SentAt:            time.Now(),
	}
	s.db.Create(&msg)

	conv.LastMessageAt = time.Now()
	s.db.Save(&conv)

	s.LogAudit(&userID, &branchID, "QUOTATION_CREATED", "quotations", quote.ID.String(), nil, quote, "")

	return &quote, nil
}

func formatRupiah(amount float64) string {
	in := int64(amount)
	str := fmt.Sprintf("%d", in)
	n := len(str)
	if n <= 3 {
		return str
	}
	var out []string
	for i := n; i > 0; i -= 3 {
		if i-3 < 0 {
			out = append([]string{str[0:i]}, out...)
		} else {
			out = append([]string{str[i-3 : i]}, out...)
		}
	}
	return strings.Join(out, ".")
}

// ----------------- Invoices & Dual-Check Payments -----------------
func (s *CRMService) CreateInvoice(leadID, quotationID, userID uuid.UUID, paymentType string, termsCount int) (*model.Invoice, error) {
	var lead model.Lead
	if err := s.db.Preload("Branch").First(&lead, leadID).Error; err != nil {
		return nil, err
	}

	var totalAmount float64 = 0
	if quotationID != uuid.Nil {
		var quote model.Quotation
		if err := s.db.First(&quote, quotationID).Error; err == nil {
			totalAmount = quote.TotalAmount
		}
	}

	branchCode := "JKT"
	if lead.Branch != nil {
		branchCode = lead.Branch.Code
	}

	invNo := fmt.Sprintf("DGT-%s-%s-%04d", branchCode, time.Now().Format("0106"), time.Now().Unix()%10000)

	inv := model.Invoice{
		InvoiceNumber: invNo,
		LeadID:        leadID,
		QuotationID:   &quotationID,
		BranchID:      *lead.BranchID,
		PaymentType:   paymentType,
		TotalAmount:   totalAmount,
		Status:        "SENT",
	}

	if err := s.db.Create(&inv).Error; err != nil {
		return nil, err
	}

	// Generate Payment Terms
	if paymentType == "INSTALLMENT" && termsCount > 1 {
		termAmount := totalAmount / float64(termsCount)
		for i := 1; i <= termsCount; i++ {
			dueDate := time.Now().AddDate(0, 0, 14*i)
			term := model.PaymentTerm{
				InvoiceID:  inv.ID,
				TermNumber: i,
				Amount:     termAmount,
				DueDate:    dueDate,
				Status:     "PENDING",
			}
			s.db.Create(&term)
		}
	} else {
		term := model.PaymentTerm{
			InvoiceID:  inv.ID,
			TermNumber: 1,
			Amount:     totalAmount,
			DueDate:    time.Now().AddDate(0, 0, 7),
			Status:     "PENDING",
		}
		s.db.Create(&term)
	}

	lead.Status = "PAYMENT_PENDING"
	s.db.Save(&lead)

	s.LogAudit(&userID, lead.BranchID, "INVOICE_CREATED", "invoices", inv.ID.String(), nil, inv, "")

	return &inv, nil
}

func (s *CRMService) GetInvoices(branchID *uuid.UUID) ([]model.Invoice, error) {
	var invoices []model.Invoice
	query := s.db.Preload("Lead").Preload("Terms").Preload("Terms.Proofs")

	if branchID != nil {
		query = query.Where("branch_id = ?", branchID)
	}

	err := query.Order("created_at desc").Find(&invoices).Error
	return invoices, err
}

func (s *CRMService) UploadPaymentProof(termID, userID uuid.UUID, proofURL string, amount float64, bank string) (*model.PaymentProof, error) {
	proof := model.PaymentProof{
		PaymentTermID:      termID,
		UploadedByUserID:   userID,
		ProofImageURL:      proofURL,
		AmountTransferred:  amount,
		BankName:           bank,
		TransferDate:       time.Now(),
		VerificationStatus: "PENDING_PUSAT",
	}

	if err := s.db.Create(&proof).Error; err != nil {
		return nil, err
	}

	// Update PaymentTerm status to PROOF_UPLOADED
	s.db.Model(&model.PaymentTerm{}).Where("id = ?", termID).Update("status", "PROOF_UPLOADED")

	s.LogAudit(&userID, nil, "PAYMENT_PROOF_UPLOADED", "payment_proofs", proof.ID.String(), nil, proof, "")

	return &proof, nil
}

func (s *CRMService) VerifyPaymentProof(proofID, pusatUserID uuid.UUID, approve bool, notes string) error {
	var proof model.PaymentProof
	if err := s.db.First(&proof, proofID).Error; err != nil {
		return err
	}

	// Check if user is ADMIN_PUSAT
	var user model.User
	if err := s.db.First(&user, pusatUserID).Error; err != nil || user.Role != "ADMIN_PUSAT" {
		return errors.New("unauthorized: Only Admin Pusat can verify payments")
	}

	now := time.Now()
	proof.VerifiedByPusatID = &pusatUserID
	proof.VerifiedAt = &now
	proof.VerificationNotes = notes

	if approve {
		proof.VerificationStatus = "APPROVED"
		s.db.Save(&proof)

		// Update term status
		var term model.PaymentTerm
		s.db.First(&term, proof.PaymentTermID)
		term.Status = "VERIFIED"
		s.db.Save(&term)

		// Check if all terms in invoice are verified
		var invoice model.Invoice
		s.db.Preload("Terms").First(&invoice, term.InvoiceID)

		allVerified := true
		var totalPaid float64 = 0
		for _, t := range invoice.Terms {
			if t.Status == "VERIFIED" {
				totalPaid += t.Amount
			} else {
				allVerified = false
			}
		}

		invoice.PaidAmount = totalPaid
		if allVerified {
			invoice.Status = "PAID"
			// Update lead status to PAID
			s.db.Model(&model.Lead{}).Where("id = ?", invoice.LeadID).Update("status", "PAID")
		} else {
			invoice.Status = "PARTIAL_PAID"
		}
		s.db.Save(&invoice)

	} else {
		proof.VerificationStatus = "REJECTED"
		s.db.Save(&proof)
	}

	s.LogAudit(&pusatUserID, nil, "PAYMENT_VERIFIED", "payment_proofs", proof.ID.String(), nil, proof, "")

	return nil
}

// ----------------- Executive Analytics Dashboard -----------------
type DashboardKPIs struct {
	TotalLeads       int64            `json:"total_leads"`
	ConversionRate   float64          `json:"conversion_rate"`
	TotalRevenue     float64          `json:"total_revenue"`
	OutstandingAR    float64          `json:"outstanding_ar"`
	LeadsByBranch    map[string]int64 `json:"leads_by_branch"`
	RevenueByBranch  map[string]float64`json:"revenue_by_branch"`
	PipelineFunnel   map[string]int64 `json:"pipeline_funnel"`
}

func (s *CRMService) GetDashboardKPIs(branchID *uuid.UUID) (*DashboardKPIs, error) {
	kpi := &DashboardKPIs{
		LeadsByBranch:   make(map[string]int64),
		RevenueByBranch: make(map[string]float64),
		PipelineFunnel:  make(map[string]int64),
	}

	// 1. Total Leads
	var leadCount int64
	queryLead := s.db.Model(&model.Lead{}).Where("is_merged = ?", false)
	if branchID != nil {
		queryLead = queryLead.Where("branch_id = ?", branchID)
	}
	queryLead.Count(&leadCount)
	kpi.TotalLeads = leadCount

	// 2. Conversion & Funnel
	var leads []model.Lead
	queryLead.Find(&leads)
	var dealCount int64 = 0
	for _, l := range leads {
		kpi.PipelineFunnel[l.Status]++
		if l.Status == "PAID" || l.Status == "DOKUMEN" || l.Status == "FULFILLMENT" || l.Status == "COMPLETED" {
			dealCount++
		}
	}

	if leadCount > 0 {
		kpi.ConversionRate = (float64(dealCount) / float64(leadCount)) * 100
	}

	// 3. Revenue & AR
	var invoices []model.Invoice
	queryInv := s.db.Model(&model.Invoice{})
	if branchID != nil {
		queryInv = queryInv.Where("branch_id = ?", branchID)
	}
	queryInv.Find(&invoices)

	for _, inv := range invoices {
		kpi.TotalRevenue += inv.PaidAmount
		if inv.TotalAmount > inv.PaidAmount {
			kpi.OutstandingAR += (inv.TotalAmount - inv.PaidAmount)
		}
	}

	return kpi, nil
}

func (s *CRMService) GetAuditLogs() ([]model.AuditLog, error) {
	var logs []model.AuditLog
	err := s.db.Preload("User").Preload("Branch").Order("created_at desc").Limit(100).Find(&logs).Error
	return logs, err
}

// ----------------- Booking Travelers & Documents -----------------
func (s *CRMService) GetTravelers(leadID *uuid.UUID) ([]model.BookingTraveler, error) {
	var travelers []model.BookingTraveler
	query := s.db.Preload("Lead").Preload("Lead.Branch").Preload("Documents")
	if leadID != nil {
		query = query.Where("lead_id = ?", leadID)
	}
	err := query.Order("created_at desc").Find(&travelers).Error
	return travelers, err
}

func (s *CRMService) CreateTraveler(leadID uuid.UUID, fullName, idCardNumber, passportNumber string, passportExpiry, birthDate *time.Time, ktpPhotoUrl, passportPhotoUrl string) (*model.BookingTraveler, error) {
	traveler := model.BookingTraveler{
		LeadID:           leadID,
		FullName:         fullName,
		IDCardNumber:     idCardNumber,
		PassportNumber:   passportNumber,
		PassportExpiry:   passportExpiry,
		BirthDate:        birthDate,
		KtpPhotoUrl:      ktpPhotoUrl,
		PassportPhotoUrl: passportPhotoUrl,
	}
	if err := s.db.Create(&traveler).Error; err != nil {
		return nil, err
	}
	return &traveler, nil
}

func (s *CRMService) UpdateTraveler(id uuid.UUID, fullName, idCardNumber, passportNumber string, passportExpiry, birthDate *time.Time, ktpPhotoUrl, passportPhotoUrl string) (*model.BookingTraveler, error) {
	var traveler model.BookingTraveler
	if err := s.db.First(&traveler, id).Error; err != nil {
		return nil, err
	}

	traveler.FullName = fullName
	traveler.IDCardNumber = idCardNumber
	traveler.PassportNumber = passportNumber
	traveler.PassportExpiry = passportExpiry
	traveler.BirthDate = birthDate
	if ktpPhotoUrl != "" {
		traveler.KtpPhotoUrl = ktpPhotoUrl
	}
	if passportPhotoUrl != "" {
		traveler.PassportPhotoUrl = passportPhotoUrl
	}

	if err := s.db.Save(&traveler).Error; err != nil {
		return nil, err
	}
	return &traveler, nil
}

func (s *CRMService) DeleteTraveler(id uuid.UUID) error {
	return s.db.Delete(&model.BookingTraveler{}, id).Error
}
