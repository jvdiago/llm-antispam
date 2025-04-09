package llm

import (
	"context"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

// fakeLLM is a mock implementation of llms.LLM.
// Its GenerateContent method returns a string representing a score depending on the prompt.
type fakeLLM struct {
	content string
}

// GenerateContent mocks the LLM call by checking the prompt content.
func (f fakeLLM) GenerateContent(
	ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: f.content}}}, nil
}

func (f fakeLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", nil
}

func TestClassifySpam(t *testing.T) {
	mockLLM := fakeLLM{content: "```json\n{\n        \"SpamScore\": \"10\",\n        \"Reason\": \"SPAM\"\n}```"}
	// Call the function under test.
	score, reason, err := ClassifyEmail(mockLLM, "Mock string")
	if err != nil {
		t.Fatalf("ClassifySpam returned error: %v", err)
	}
	// Check lastUid equals 103.
	if score != 10 {
		t.Errorf("Expected score to be 10, got %f", score)
	}
	if reason != "SPAM" {
		t.Errorf("Expected reason to be SPM, got %s", reason)
	}

}
