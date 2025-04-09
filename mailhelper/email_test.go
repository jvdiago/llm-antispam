package mailhelper

import (
	"bytes"
	"encoding/json"
	"io"
	"net/mail"
	"os"
	"testing"

	"github.com/emersion/go-imap"
)

type fakeLiteral struct {
	io.Reader
}

func (f fakeLiteral) Len() int {
	return 0
}
func createIMAPMessage(raw string) *imap.Message {
	section := &imap.BodySectionName{Peek: false}
	message := imap.NewMessage(0, nil)
	message.Body = map[*imap.BodySectionName]imap.Literal{
		section: fakeLiteral{bytes.NewReader([]byte(raw))},
	}
	return message
}

// --- Tests for NewEmail ---

func TestNewEmail_Success(t *testing.T) {
	raw := "From: test@example.com\r\n" +
		"Subject: Test Email\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n\r\n" +
		"Hello, world!"
	msg := createIMAPMessage(raw)
	email, err := NewEmail(msg)
	if err != nil {
		t.Fatalf("NewEmail returned error: %v", err)
	}
	if email == nil {
		t.Fatal("NewEmail returned a nil Email")
	}
	if email.GetSubject() != "Test Email" {
		t.Errorf("Expected subject %q, got %q", "Test Email", email.GetSubject())
	}
	body, err := email.GetBody()
	if err != nil {
		t.Fatalf("GetBody returned error: %v", err)
	}
	if string(body) != "Hello, world!" {
		t.Errorf("Expected body %q, got %q", "Hello, world!", string(body))
	}
}

// --- Helper for constructing an Email from a raw RFC822 message ---

func createTestEmail(raw string) *Email {
	msg, err := mail.ReadMessage(bytes.NewReader([]byte(raw)))
	if err != nil {
		panic("Failed to read message: " + err.Error())
	}
	return &Email{msg: msg, rawMsg: []byte(raw)}
}

// --- Tests for Email methods ---

func TestGetHeadersAndGetHeader(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"Subject: Hello\r\n" +
		"X-Test: Value\r\n\r\n" +
		"Body"
	email := createTestEmail(raw)

	headers := email.GetHeaders()
	if headers.Get("X-Test") != "Value" {
		t.Errorf("Expected X-Test header %q, got %q", "Value", headers.Get("X-Test"))
	}

	if email.GetHeader("Subject") != "Hello" {
		t.Errorf("Expected Subject header %q, got %q", "Hello", email.GetHeader("Subject"))
	}
}

func TestGetSender(t *testing.T) {
	// Test when the Sender header is present.
	rawWithSender := "Sender: sender1@example.com\r\n" +
		"From: sender2@example.com\r\n\r\nBody"
	email1 := createTestEmail(rawWithSender)
	sender, err := email1.GetSender()
	if err != nil {
		t.Fatalf("GetSender returned error: %v", err)
	}
	if sender.Address != "sender1@example.com" {
		t.Errorf("Expected sender %q, got %q", "sender1@example.com", sender.Address)
	}

	// Test when only the From header is available.
	rawFromOnly := "From: sender3@example.com\r\n\r\nBody"
	email2 := createTestEmail(rawFromOnly)
	sender, err = email2.GetSender()
	if err != nil {
		t.Fatalf("GetSender returned error: %v", err)
	}
	if sender.Address != "sender3@example.com" {
		t.Errorf("Expected sender %q, got %q", "sender3@example.com", sender.Address)
	}
}

func TestGetSubject(t *testing.T) {
	raw := "Subject: Test Subject\r\n\r\nBody"
	email := createTestEmail(raw)
	if email.GetSubject() != "Test Subject" {
		t.Errorf("Expected subject %q, got %q", "Test Subject", email.GetSubject())
	}
}

func TestGetRawEmail(t *testing.T) {
	raw := "Subject: Raw Test\r\n\r\nBody content"
	email := createTestEmail(raw)
	if string(email.GetRawEmail()) != raw {
		t.Errorf("Expected raw email %q, got %q", raw, string(email.GetRawEmail()))
	}
}

func TestGetBody(t *testing.T) {
	raw := "Subject: Body Test\r\n\r\nThis is the body."
	email := createTestEmail(raw)
	body, err := email.GetBody()
	if err != nil {
		t.Fatalf("GetBody returned error: %v", err)
	}
	if string(body) != "This is the body." {
		t.Errorf("Expected body %q, got %q", "This is the body.", string(body))
	}
}

// --- Tests for ExtractSpamStatus ---

func TestExtractSpamStatus(t *testing.T) {
	// Valid spam header with positive hits.
	raw := "X-Spam-Status: Yes, hits=5.5 required=5.0 tests=TEST\r\n\r\nBody"
	email := createTestEmail(raw)
	hits, err := ExtractSpamStatus(email)
	if err != nil {
		t.Fatalf("ExtractSpamStatus returned error: %v", err)
	}
	if hits != 5.5 {
		t.Errorf("Expected spam hits 5.5, got %v", hits)
	}

	// Valid spam header with a negative hit value.
	rawNeg := "X-Spam-Status: No, hits=-2.0 required=5.0 tests=TEST\r\n\r\nBody"
	email = createTestEmail(rawNeg)
	hits, err = ExtractSpamStatus(email)
	if err != nil {
		t.Fatalf("ExtractSpamStatus returned error: %v", err)
	}
	if hits != -2.0 {
		t.Errorf("Expected spam hits -2.0, got %v", hits)
	}

	// Spam header that does not contain a valid hits value.
	rawNoHits := "X-Spam-Status: Unknown\r\n\r\nBody"
	email = createTestEmail(rawNoHits)
	hits, err = ExtractSpamStatus(email)
	if err != nil {
		t.Fatalf("ExtractSpamStatus returned error: %v", err)
	}
	if hits != 0 {
		t.Errorf("Expected spam hits 0, got %v", hits)
	}
}

// --- Tests for LastProcessed and its UpdateLastProcessed method ---

func TestLastProcessed_NewAndUpdate(t *testing.T) {
	// Create a temporary file.
	tmpFile, err := os.CreateTemp("", "lastprocessed_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	filename := tmpFile.Name()
	// Ensure the file is removed after the test.
	defer os.Remove(filename)
	tmpFile.Close()

	// Write initial JSON content to the file.
	initial := LastProcessed{LastProcessedID: 10, Filename: filename}
	data, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("Failed to marshal initial LastProcessed: %v", err)
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Test NewLastProcessed.
	cfg, err := NewLastProcessed(filename)
	if err != nil {
		t.Fatalf("NewLastProcessed returned error: %v", err)
	}
	if cfg.LastProcessedID != 10 {
		t.Errorf("Expected LastProcessedID 10, got %d", cfg.LastProcessedID)
	}
	if cfg.Filename != filename {
		t.Errorf("Expected Filename %q, got %q", filename, cfg.Filename)
	}

	// Update the LastProcessedID and call UpdateLastProcessed.
	cfg.LastProcessedID = 20
	if err := cfg.UpdateLastProcessed(); err != nil {
		t.Fatalf("UpdateLastProcessed returned error: %v", err)
	}
	// Read the file and verify its content.
	updatedData, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}
	var updatedCfg LastProcessed
	if err := json.Unmarshal(updatedData, &updatedCfg); err != nil {
		t.Fatalf("Failed to unmarshal updated data: %v", err)
	}
	if updatedCfg.LastProcessedID != 20 {
		t.Errorf("Expected updated LastProcessedID 20, got %d", updatedCfg.LastProcessedID)
	}
}
