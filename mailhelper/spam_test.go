package mailhelper

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/emersion/go-imap"
	"github.com/tmc/langchaingo/llms"
)

// createIMAPMessageWithUID constructs an *imap.Message with the given UID and raw RFC822 content.
func createIMAPMessageWithUID(uid uint32, raw string) *imap.Message {
	// Use a BodySectionName with Peek false.
	section := &imap.BodySectionName{Peek: false}
	return &imap.Message{
		Uid: uid,
		Body: map[*imap.BodySectionName]imap.Literal{
			// We wrap the raw email bytes in a fakeLiteral.
			section: fakeLiteral{bytes.NewReader([]byte(raw))},
		},
	}
}

// fakeLLM is a mock implementation of llms.LLM.
// Its GenerateContent method returns a string representing a score depending on the prompt.
type fakeLLM struct{}

// GenerateContent mocks the LLM call by checking the prompt content.
func (f fakeLLM) GenerateContent(
	ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	var content string

	if strings.Contains(llms.TextContent(messages[0].Parts[0].(llms.TextContent)).Text, "Ham") {
		content = "```json\n{\n        \"SpamScore\": \"0\",\n        \"Reason\": \"NOTSPAM\"\n}```"
	} else {
		content = "```json\n{\n        \"SpamScore\": \"10\",\n        \"Reason\": \"SPAM\"\n}```"
	}
	return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: content}}}, nil
}

func (f fakeLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", nil
}

// To satisfy the llms.LLM interface, fakeLLM can implement other methods as needed.
// For our test, only GenerateContent is used via llm.ClassifyEmail.

// TestClassifySpam verifies that ClassifySpam classifies messages correctly.
func TestClassifySpam(t *testing.T) {
	// Define raw emails for our test cases.
	raw1 := "From: user@notwhitelisted.com\r\n" +
		"Subject: Spam Email\r\n" +
		"X-Spam-Status: No, hits=0.0 required=5.0 tests=TEST\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n\r\n" +
		"<html><body><p>Spam Content</p></body></html>"

	raw2 := "From: user@notwhitelisted.com\r\n" +
		"Subject: Ham Email\r\n" +
		"X-Spam-Status: No, hits=0.0 required=5.0 tests=TEST\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n\r\n" +
		"<html><body><p>Ham Content</p></body></html>"

	raw3 := "From: user@notwhitelisted.com\r\n" +
		"Subject: Old Email\r\n" +
		"X-Spam-Status: No, hits=0.0 required=5.0 tests=TEST\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n\r\n" +
		"<html><body><p>Old Content</p></body></html>"

	raw4 := "From: user@whitelist.com\r\n" +
		"Subject: Whitelisted Email\r\n" +
		"X-Spam-Status: No, hits=0.0 required=5.0 tests=TEST\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n\r\n" +
		"<html><body><p>Whitelisted Content</p></body></html>"

	// Create test messages with UIDs.
	msg1 := createIMAPMessageWithUID(101, raw1)
	msg2 := createIMAPMessageWithUID(102, raw2)
	msg3 := createIMAPMessageWithUID(100, raw3) // Old email (UID <= lastProcessedID)
	msg4 := createIMAPMessageWithUID(103, raw4) // Whitelisted email

	// Prepare a channel of messages.
	messages := make(chan *imap.Message, 4)
	messages <- msg1
	messages <- msg2
	messages <- msg3
	messages <- msg4
	close(messages)

	// Prepare a slice of IDs (one per message in order).
	ids := []uint32{10, 20, 30, 40}

	// Define whitelisted domains, threshold, and lastProcessedID.
	whitelistedDomains := []string{"whitelist.com"}
	threshold := 0.5
	lastProcessedID := uint32(100)

	// Use our fake LLM classifier.
	mockLLM := fakeLLM{}

	// Call the function under test.
	spamSeqset, notSpamSeqset, lastUid, err := ClassifySpam(messages, ids, whitelistedDomains, threshold, lastProcessedID, mockLLM, true)
	if err != nil {
		t.Fatalf("ClassifySpam returned error: %v", err)
	}

	// Expected behavior:
	// - msg1 (UID 101) is processed and classified with score 0.9 (> threshold), so its id (ids[0] = 10) goes to spam.
	// - msg2 (UID 102) is processed and classified with score 0.3 (<= threshold), so its id (ids[1] = 20) goes to not spam.
	// - msg3 is skipped because its UID (100) <= lastProcessedID.
	// - msg4 is skipped because sender domain is whitelisted.
	//
	// Also, lastUid should be the UID of the last message from the channel (msg4: 103).

	// Check spamSeqset contains only id 10.
	if len(spamSeqset.Set) != 1 || spamSeqset.Set[0].Start != 10 {
		t.Errorf("Expected spamSeqset to contain [10], got %v", spamSeqset.Set)
	}

	// Check notSpamSeqset contains only id 20.
	if len(notSpamSeqset.Set) != 1 || notSpamSeqset.Set[0].Start != 20 {
		t.Errorf("Expected notSpamSeqset to contain [20], got %v", notSpamSeqset.Set)
	}

	// Check lastUid equals 103.
	if lastUid != 103 {
		t.Errorf("Expected lastUid to be 103, got %d", lastUid)
	}
}
