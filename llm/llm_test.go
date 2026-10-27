package llm

import (
	"context"
	"testing"

	"llm-antispam/mailhelper"
)

// mockClient is a mock implementation of the Client.
// Its ClassifyEmail method returns a fixed score.
type mockClient struct {
	score float64
}

func (m *mockClient) ClassifyEmail(ctx context.Context, email *mailhelper.Email) (float64, error) {
	return m.score, nil
}

func TestClassifyEmail(t *testing.T) {
	mockClient := &mockClient{score: 0.1}
	email := &mailhelper.Email{
		From:    "test@example.com",
		Subject: "Test Email",
		Body:    "This is a test email.",
	}

	// Call the function under test.
	score, err := mockClient.ClassifyEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("ClassifyEmail returned error: %v", err)
	}
	// Check score equals 0.1.
	if score != 0.1 {
		t.Errorf("Expected score to be 0.1, got %f", score)
	}
}
