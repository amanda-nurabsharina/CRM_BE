package provider

import (
	"bytes"
	"context"
	"crm-be/src/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type MetaWABAProvider struct {
	PhoneNumberID     string
	BusinessAccountID string
	AccessToken       string
	HTTPClient        *http.Client
}

func NewMetaWABAProvider(phoneNumberID, businessAccountID, accessToken string) *MetaWABAProvider {
	return &MetaWABAProvider{
		PhoneNumberID:     phoneNumberID,
		BusinessAccountID: businessAccountID,
		AccessToken:       accessToken,
		HTTPClient:        &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *MetaWABAProvider) SendText(ctx context.Context, req SendTextRequest) (*SendResponse, error) {
	cleanPhone := strings.ReplaceAll(req.ToPhone, "+", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
	cleanPhone = strings.TrimSpace(cleanPhone)
	if strings.HasPrefix(cleanPhone, "0") {
		cleanPhone = "62" + cleanPhone[1:]
	}

	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages", p.PhoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                 cleanPhone,
		"type":              "text",
		"text": map[string]string{
			"body": req.Text,
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.AccessToken != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.AccessToken))
	}

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		utils.Log.Errorf("[META-WABA] Error making HTTP request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	utils.Log.Infof("[META-WABA] Outbound Meta Cloud API response (%d): %s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode >= 400 {
		utils.Log.Warnf("[META-WABA] Text message failed with status %d, attempting fallback template 'hello_world'", resp.StatusCode)
		templatePayload := map[string]interface{}{
			"messaging_product": "whatsapp",
			"recipient_type":    "individual",
			"to":                 cleanPhone,
			"type":              "template",
			"template": map[string]interface{}{
				"name": "hello_world",
				"language": map[string]string{
					"code": "en_US",
				},
			},
		}
		tBytes, _ := json.Marshal(templatePayload)
		tReq, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(tBytes))
		tReq.Header.Set("Content-Type", "application/json")
		if p.AccessToken != "" {
			tReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.AccessToken))
		}
		tResp, errT := p.HTTPClient.Do(tReq)
		if errT == nil {
			defer tResp.Body.Close()
			tBody, _ := io.ReadAll(tResp.Body)
			utils.Log.Infof("[META-WABA] Fallback Template Response (%d): %s", tResp.StatusCode, string(tBody))
			if tResp.StatusCode == 200 {
				var tMetaResp struct {
					Messages []struct {
						ID string `json:"id"`
					} `json:"messages"`
				}
				json.Unmarshal(tBody, &tMetaResp)
				msgID := fmt.Sprintf("META-WABA-%d", time.Now().UnixNano())
				if len(tMetaResp.Messages) > 0 {
					msgID = tMetaResp.Messages[0].ID
				}
				return &SendResponse{ExternalMessageID: msgID, Status: "SENT"}, nil
			}
		}
	}

	var metaResp struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	json.Unmarshal(bodyBytes, &metaResp)

	if resp.StatusCode >= 400 && metaResp.Error != nil {
		return nil, fmt.Errorf("Meta WABA Error: %s", metaResp.Error.Message)
	}

	msgID := fmt.Sprintf("META-WABA-%d", time.Now().UnixNano())
	if len(metaResp.Messages) > 0 {
		msgID = metaResp.Messages[0].ID
	}

	return &SendResponse{
		ExternalMessageID: msgID,
		Status:            "SENT",
	}, nil
}

func (p *MetaWABAProvider) SendMedia(ctx context.Context, req SendMediaRequest) (*SendResponse, error) {
	cleanPhone := strings.ReplaceAll(req.ToPhone, "+", "")
	if strings.HasPrefix(cleanPhone, "0") {
		cleanPhone = "62" + cleanPhone[1:]
	}

	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages", p.PhoneNumberID)

	mediaTypeKey := "image"
	if strings.ToUpper(req.MediaType) == "DOCUMENT" {
		mediaTypeKey = "document"
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                 cleanPhone,
		"type":              mediaTypeKey,
		mediaTypeKey: map[string]string{
			"link":    req.MediaURL,
			"caption": req.Caption,
		},
	}

	jsonBytes, _ := json.Marshal(payload)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	if p.AccessToken != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.AccessToken))
	}

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return &SendResponse{
		ExternalMessageID: fmt.Sprintf("META-WABA-MEDIA-%d", time.Now().UnixNano()),
		Status:            "SENT",
	}, nil
}

func (p *MetaWABAProvider) SendTemplate(ctx context.Context, req SendTemplateRequest) (*SendResponse, error) {
	cleanPhone := strings.ReplaceAll(req.ToPhone, "+", "")
	if strings.HasPrefix(cleanPhone, "0") {
		cleanPhone = "62" + cleanPhone[1:]
	}

	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages", p.PhoneNumberID)

	langCode := req.LanguageCode
	if langCode == "" {
		langCode = "id"
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                 cleanPhone,
		"type":              "template",
		"template": map[string]interface{}{
			"name": req.TemplateName,
			"language": map[string]string{
				"code": langCode,
			},
		},
	}

	jsonBytes, _ := json.Marshal(payload)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	httpReq.Header.Set("Content-Type", "application/json")
	if p.AccessToken != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.AccessToken))
	}

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return &SendResponse{
		ExternalMessageID: fmt.Sprintf("META-WABA-TPL-%d", time.Now().UnixNano()),
		Status:            "SENT",
	}, nil
}
