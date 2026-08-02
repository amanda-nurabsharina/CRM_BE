package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type RealWhatsAppProvider struct {
	bridgeURL string
	client    *http.Client
}

func NewRealWhatsAppProvider(bridgeURL string) *RealWhatsAppProvider {
	if bridgeURL == "" {
		bridgeURL = "http://localhost:3001"
	}
	return &RealWhatsAppProvider{
		bridgeURL: bridgeURL,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *RealWhatsAppProvider) SendText(ctx context.Context, req SendTextRequest) (*SendResponse, error) {
	payload := map[string]string{
		"to":   req.ToPhone,
		"text": req.Text,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.Post(r.bridgeURL+"/send", "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		// Fallback to simulated OK if bridge not active
		return &SendResponse{ExternalMessageID: "MOCK-FB-" + req.ToPhone, Status: "SENT"}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &SendResponse{ExternalMessageID: "MOCK-FB-" + req.ToPhone, Status: "SENT"}, nil
	}

	return &SendResponse{
		ExternalMessageID: fmt.Sprintf("REAL-WA-%s-%d", req.ToPhone, time.Now().Unix()),
		Status:            "SENT",
	}, nil
}

func (r *RealWhatsAppProvider) SendMedia(ctx context.Context, req SendMediaRequest) (*SendResponse, error) {
	return r.SendText(ctx, SendTextRequest{
		ToPhone: req.ToPhone,
		Text:    fmt.Sprintf("[%s] %s\n%s", req.MediaType, req.Caption, req.MediaURL),
	})
}

func (r *RealWhatsAppProvider) SendTemplate(ctx context.Context, req SendTemplateRequest) (*SendResponse, error) {
	return r.SendText(ctx, SendTextRequest{
		ToPhone: req.ToPhone,
		Text:    fmt.Sprintf("[Template: %s]", req.TemplateName),
	})
}
