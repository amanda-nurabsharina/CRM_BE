package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MockWhatsAppProvider struct{}

func NewMockWhatsAppProvider() *MockWhatsAppProvider {
	return &MockWhatsAppProvider{}
}

func (m *MockWhatsAppProvider) SendText(ctx context.Context, req SendTextRequest) (*SendResponse, error) {
	mockID := fmt.Sprintf("MOCK-MSG-%s-%d", uuid.New().String()[:8], time.Now().Unix())
	return &SendResponse{
		ExternalMessageID: mockID,
		Status:            "SENT",
	}, nil
}

func (m *MockWhatsAppProvider) SendMedia(ctx context.Context, req SendMediaRequest) (*SendResponse, error) {
	mockID := fmt.Sprintf("MOCK-MEDIA-%s-%d", uuid.New().String()[:8], time.Now().Unix())
	return &SendResponse{
		ExternalMessageID: mockID,
		Status:            "SENT",
	}, nil
}

func (m *MockWhatsAppProvider) SendTemplate(ctx context.Context, req SendTemplateRequest) (*SendResponse, error) {
	mockID := fmt.Sprintf("MOCK-TPL-%s-%d", uuid.New().String()[:8], time.Now().Unix())
	return &SendResponse{
		ExternalMessageID: mockID,
		Status:            "SENT",
	}, nil
}
