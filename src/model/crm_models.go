package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Branch represents a DGT branch location
type Branch struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id"`
	Name          string         `gorm:"type:varchar(100);not null" json:"name"`
	Code          string         `gorm:"type:varchar(10);uniqueIndex;not null" json:"code"` // e.g. JKT_PST, JKT_SEL, JKT_UTR, MDN, TGR
	WAPhoneNumber string         `gorm:"type:varchar(30);uniqueIndex" json:"wa_phone_number"`
	CoverageAreas string         `gorm:"type:text" json:"coverage_areas"` // comma separated area list
	IsActive      bool           `gorm:"default:true" json:"is_active"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (b *Branch) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return
}

// Lead represents a potential customer inquiry & pipeline state
type Lead struct {
	ID              uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id"`
	CustomerName    string         `gorm:"type:varchar(100);not null" json:"customer_name"`
	PhoneNumber     string         `gorm:"type:varchar(30);not null;index" json:"phone_number"`
	Domicile        string         `gorm:"type:varchar(100)" json:"domicile"`
	Source          string         `gorm:"type:varchar(50);default:'WHATSAPP'" json:"source"` // WHATSAPP, WEBSITE, ADS, REFERRAL
	Status          string         `gorm:"type:varchar(30);default:'NEW'" json:"status"`      // NEW, QUALIFIED, QUOTATION_SENT, NEGOTIATION, DEAL, LOST, PAYMENT_PENDING, PAID, DOKUMEN, FULFILLMENT, COMPLETED, CANCELLED
	BranchID        *uuid.UUID     `gorm:"type:uuid;index" json:"branch_id"`
	Branch          *Branch        `gorm:"foreignKey:BranchID" json:"branch,omitempty"`
	AssignedUserID  *uuid.UUID     `gorm:"type:uuid;index" json:"assigned_user_id"`
	AssignedUser    *User          `gorm:"foreignKey:AssignedUserID" json:"assigned_user,omitempty"`
	FirstResponseAt *time.Time     `json:"first_response_at"`
	HandoverNote    string         `gorm:"type:text" json:"handover_note"`
	IsMerged        bool           `gorm:"default:false" json:"is_merged"`
	MergedToLeadID  *uuid.UUID     `gorm:"type:uuid" json:"merged_to_lead_id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (l *Lead) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return
}

// Conversation links Lead with WhatsApp messages
type Conversation struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id"`
	LeadID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"lead_id"`
	Lead          *Lead          `gorm:"foreignKey:LeadID" json:"lead,omitempty"`
	BranchID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"branch_id"`
	Status        string         `gorm:"type:varchar(20);default:'ACTIVE'" json:"status"`
	LastMessageAt time.Time      `json:"last_message_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (c *Conversation) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return
}

// Message represents an individual text/media/template chat message
type Message struct {
	ID                uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	ConversationID    uuid.UUID `gorm:"type:uuid;not null;index" json:"conversation_id"`
	SenderType        string    `gorm:"type:varchar(20);not null" json:"sender_type"` // CUSTOMER, ADMIN, SYSTEM
	SenderID          string    `gorm:"type:varchar(100)" json:"sender_id"`
	Direction         string    `gorm:"type:varchar(10);not null" json:"direction"`   // INBOUND, OUTBOUND
	MessageType       string    `gorm:"type:varchar(20);not null" json:"message_type"`// TEXT, IMAGE, DOCUMENT, TEMPLATE
	Content           string    `gorm:"type:text" json:"content"`
	MediaURL          string    `gorm:"type:varchar(500)" json:"media_url"`
	ExternalMessageID string    `gorm:"type:varchar(100)" json:"external_message_id"`
	Status            string    `gorm:"type:varchar(20);default:'SENT'" json:"status"` // PENDING, SENT, DELIVERED, READ, FAILED
	SentAt            time.Time `json:"sent_at"`
	CreatedAt         time.Time `json:"created_at"`
}

func (m *Message) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return
}

// TourPackage represents a standard travel package catalog item
type TourPackage struct {
	ID              uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id"`
	Title           string         `gorm:"type:varchar(150);not null" json:"title"`
	Destination     string         `gorm:"type:varchar(100);not null" json:"destination"`
	DurationDays    int            `gorm:"not null" json:"duration_days"`
	BasePrice       float64        `gorm:"type:decimal(15,2);not null" json:"base_price"`
	ItineraryJSON   string         `gorm:"type:text" json:"itinerary_json"`
	TermsConditions string         `gorm:"type:text" json:"terms_conditions"`
	PdfUrl          string         `gorm:"type:varchar(500)" json:"pdf_url"`
	WaTemplate      string         `gorm:"type:text" json:"wa_template"`
	IsActive        bool           `gorm:"default:true" json:"is_active"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (tp *TourPackage) BeforeCreate(tx *gorm.DB) (err error) {
	if tp.ID == uuid.Nil {
		tp.ID = uuid.New()
	}
	return
}

// Quotation represents an official offer generated for a Lead
type Quotation struct {
	ID                uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id"`
	QuotationNumber   string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"quotation_number"`
	LeadID            uuid.UUID      `gorm:"type:uuid;not null;index" json:"lead_id"`
	Lead              *Lead          `gorm:"foreignKey:LeadID" json:"lead,omitempty"`
	PackageID         uuid.UUID      `gorm:"type:uuid;not null" json:"package_id"`
	Package           *TourPackage   `gorm:"foreignKey:PackageID" json:"package,omitempty"`
	BranchID          uuid.UUID      `gorm:"type:uuid;not null;index" json:"branch_id"`
	CreatedByUserID   uuid.UUID      `gorm:"type:uuid;not null" json:"created_by_user_id"`
	PaxCount          int            `gorm:"not null" json:"pax_count"`
	PricePerPax       float64        `gorm:"type:decimal(15,2);not null" json:"price_per_pax"`
	AddOnsJSON        string         `gorm:"type:text" json:"add_ons_json"`
	TotalAmount       float64        `gorm:"type:decimal(15,2);not null" json:"total_amount"`
	CustomPriceReason string         `gorm:"type:text" json:"custom_price_reason"`
	ValidUntil        time.Time      `gorm:"not null" json:"valid_until"`
	Status            string         `gorm:"type:varchar(20);default:'ACTIVE'" json:"status"` // ACTIVE, EXPIRED, ACCEPTED, REJECTED
	PDFUrl            string         `gorm:"type:varchar(500)" json:"pdf_url"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

func (q *Quotation) BeforeCreate(tx *gorm.DB) (err error) {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return
}

// Invoice represents a billing record per branch with unique numbering
type Invoice struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id"`
	InvoiceNumber string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"invoice_number"` // Format: DGT-JKT-0825-0001
	LeadID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"lead_id"`
	Lead          *Lead          `gorm:"foreignKey:LeadID" json:"lead,omitempty"`
	QuotationID   *uuid.UUID     `gorm:"type:uuid" json:"quotation_id"`
	BranchID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"branch_id"`
	PaymentType   string         `gorm:"type:varchar(20);not null;default:'FULL'" json:"payment_type"` // FULL, INSTALLMENT
	TotalAmount   float64        `gorm:"type:decimal(15,2);not null" json:"total_amount"`
	PaidAmount    float64        `gorm:"type:decimal(15,2);default:0" json:"paid_amount"`
	Status        string         `gorm:"type:varchar(30);default:'DRAFT'" json:"status"` // DRAFT, SENT, PENDING_PAYMENT, PARTIAL_PAID, PAID, CANCELLED
	Terms         []PaymentTerm  `gorm:"foreignKey:InvoiceID" json:"terms,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (i *Invoice) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return
}

// PaymentTerm handles DP & Installments (Termin Cicilan)
type PaymentTerm struct {
	ID         uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id"`
	InvoiceID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"invoice_id"`
	TermNumber int            `gorm:"not null" json:"term_number"`
	Amount     float64        `gorm:"type:decimal(15,2);not null" json:"amount"`
	DueDate    time.Time      `gorm:"not null" json:"due_date"`
	Status     string         `gorm:"type:varchar(20);default:'PENDING'" json:"status"` // PENDING, PROOF_UPLOADED, VERIFIED, OVERDUE
	Proofs     []PaymentProof `gorm:"foreignKey:PaymentTermID" json:"proofs,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (pt *PaymentTerm) BeforeCreate(tx *gorm.DB) (err error) {
	if pt.ID == uuid.Nil {
		pt.ID = uuid.New()
	}
	return
}

// PaymentProof stores uploaded TF proof & Dual-Check verification state
type PaymentProof struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	PaymentTermID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"payment_term_id"`
	UploadedByUserID   uuid.UUID  `gorm:"type:uuid;not null" json:"uploaded_by_user_id"`
	ProofImageURL      string     `gorm:"type:varchar(500);not null" json:"proof_image_url"`
	AmountTransferred  float64    `gorm:"type:decimal(15,2);not null" json:"amount_transferred"`
	BankName           string     `gorm:"type:varchar(50)" json:"bank_name"`
	TransferDate       time.Time  `json:"transfer_date"`
	VerifiedByPusatID  *uuid.UUID `gorm:"type:uuid" json:"verified_by_pusat_id"`
	VerificationStatus string     `gorm:"type:varchar(30);default:'PENDING_PUSAT'" json:"verification_status"` // PENDING_PUSAT, APPROVED, REJECTED
	VerificationNotes  string     `gorm:"type:text" json:"verification_notes"`
	VerifiedAt         *time.Time `json:"verified_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

func (pp *PaymentProof) BeforeCreate(tx *gorm.DB) (err error) {
	if pp.ID == uuid.Nil {
		pp.ID = uuid.New()
	}
	return
}

// BookingTraveler stores multi-traveler info per lead booking
type BookingTraveler struct {
	ID               uuid.UUID          `gorm:"type:uuid;primary_key;" json:"id"`
	LeadID           uuid.UUID          `gorm:"type:uuid;not null;index" json:"lead_id"`
	Lead             *Lead              `gorm:"foreignKey:LeadID" json:"lead,omitempty"`
	FullName         string             `gorm:"type:varchar(150);not null" json:"full_name"`
	IDCardNumber     string             `gorm:"type:varchar(30)" json:"id_card_number"`
	PassportNumber   string             `gorm:"type:varchar(30)" json:"passport_number"`
	PassportExpiry   *time.Time         `json:"passport_expiry"`
	BirthDate        *time.Time         `json:"birth_date"`
	KtpPhotoUrl      string             `gorm:"type:varchar(500)" json:"ktp_photo_url"`
	PassportPhotoUrl string             `gorm:"type:varchar(500)" json:"passport_photo_url"`
	Documents        []TravelerDocument `gorm:"foreignKey:TravelerID" json:"documents,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

func (bt *BookingTraveler) BeforeCreate(tx *gorm.DB) (err error) {
	if bt.ID == uuid.Nil {
		bt.ID = uuid.New()
	}
	return
}

// TravelerDocument stores encrypted passport/KTP scans
type TravelerDocument struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	TravelerID  uuid.UUID `gorm:"type:uuid;not null;index" json:"traveler_id"`
	DocType     string    `gorm:"type:varchar(20);not null" json:"doc_type"` // PASSPORT, KTP
	FilePath    string    `gorm:"type:varchar(500);not null" json:"file_path"`
	IsEncrypted bool      `gorm:"default:true" json:"is_encrypted"`
	CreatedAt   time.Time `json:"created_at"`
}

func (td *TravelerDocument) BeforeCreate(tx *gorm.DB) (err error) {
	if td.ID == uuid.Nil {
		td.ID = uuid.New()
	}
	return
}

// AuditLog is an immutable, read-only system audit log
type AuditLog struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key;" json:"id"`
	UserID          *uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	User            *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	BranchID        *uuid.UUID `gorm:"type:uuid;index" json:"branch_id"`
	Branch          *Branch    `gorm:"foreignKey:BranchID" json:"branch,omitempty"`
	ActionType      string     `gorm:"type:varchar(50);not null" json:"action_type"` // e.g. PAYMENT_VERIFIED, QUOTE_MODIFIED, LEAD_REASSIGNED
	EntityName      string     `gorm:"type:varchar(50);not null" json:"entity_name"`
	EntityID        string     `gorm:"type:varchar(100);not null" json:"entity_id"`
	BeforeValueJSON string     `gorm:"type:text" json:"before_value_json"`
	AfterValueJSON  string     `gorm:"type:text" json:"after_value_json"`
	IPAddress       string     `gorm:"type:varchar(45)" json:"ip_address"`
	CreatedAt       time.Time  `json:"created_at"`
}

func (al *AuditLog) BeforeCreate(tx *gorm.DB) (err error) {
	if al.ID == uuid.Nil {
		al.ID = uuid.New()
	}
	return
}
