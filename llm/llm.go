package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"llm-antispam/mailhelper"
	"net/http"
	"strings"
	"time"
)

// LLMType is an enum representing the supported LLM providers.
type LLMType int

const (
	LLMTypeBedrock LLMType = iota
	LLMTypeOpenAI
	LLMTypeOllama
)

type systemOneRequest struct {
	State     mailhelper.Email    `json:"state"`
	Questions map[string]Question `json:"questions"`
	Model     string              `json:"model"`
}

type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
}

type systemOneResponse struct {
	Answers map[string]Answer `json:"answers"`
}

type Answer struct {
	Type string   `json:"type"`
	Noul *float64 `json:"noul"`
}

type Client struct {
	Endpoint   string
	HTTPClient *http.Client
	Model      string
}

func NewClient(baseURL string, model string) *Client {
	return &Client{
		Endpoint: strings.TrimRight(baseURL, "/") + "/v1/systemone",
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		Model: model,
	}
}

func (c *Client) ClassifyEmail(ctx context.Context, email *mailhelper.Email) (float64, error) {
	payload := systemOneRequest{
		Model: c.Model,
		State: *email,
		Questions: map[string]Question{
			"spam": {
				Type: "noul",
				Instructions: `Is this email spam? Answer yes if it is unsolicited, deceptive, malicious,
or bulk promotional email. Answer no if it is legitimate.
Focus on the content and the intent of the email, and verify if they come from well-known domains and companies.
Do not categorize as Spam emails from well-known organizations like github.com, meetup.com, etc. or well-known email providers
like hotmail.com or gmail.com. Check for any links in the body and verify if they are legitimate.
The HTML tags and images have been removed for simplicity.
`,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.Endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("call systemone: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf(
			"systemone returned HTTP %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(responseBody)),
		)
	}

	var result systemOneResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	answer, ok := result.Answers["spam"]
	if !ok {
		return 0, fmt.Errorf(`missing "spam" answer`)
	}

	if answer.Noul == nil {
		return 0, fmt.Errorf(`missing "noul" score`)
	}

	score := *answer.Noul
	if score < 0 || score > 1 {
		return 0, fmt.Errorf("invalid spam score %f", score)
	}

	return score, nil
}
