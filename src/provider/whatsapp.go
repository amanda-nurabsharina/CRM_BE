package provider

import (
	"context"
)

type SendTextRequest struct {
	ToPhone       string `json:"to_phone"`
	BranchCode    string `json:"branch_code"`
	Text          string `json:"text"`
	ConversationID string `json:"conversation_id"`
}

type SendMediaRequest struct {
	ToPhone       string `json:"to_phone"`
	BranchCode    string `json:"branch_code"`
	MediaType     string `json:"media_type"` // IMAGE, DOCUMENT
	MediaURL      string `json:"media_url"`
	Caption       string `json:"caption"`
	ConversationID string `json:"conversation_id"`
}

type SendTemplateRequest struct {
	ToPhone       string            `json:"to_phone"`
	BranchCode    string            `json:"branch_code"`
	TemplateName  string            `json:"template_name"`
	LanguageCode  string            `json:"language_code"`
	Parameters    map[string]string `json:"parameters"`
	ConversationID string            `json:"conversation_id"`
}

type SendResponse struct {
	ExternalMessageID string `json:"external_message_id"`
	Status            string `json:"status"`
}

type WhatsAppProvider interface {
	SendText(ctx context.Context, req SendTextRequest) (*SendResponse, error)
	SendMedia(ctx context.Context, req SendMediaRequest) (*SendResponse, error)
	SendTemplate(ctx context.Context, req SendTemplateRequest) (*SendResponse, error)
}
