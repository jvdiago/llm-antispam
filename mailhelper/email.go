package mailhelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/emersion/go-imap"
	"io"
	"net/mail"
	"os"
	"regexp"
	"strconv"
)

type Email struct {
	msg    *mail.Message
	rawMsg []byte
}

func NewEmail(msg *imap.Message) (*Email, error) {
	section := &imap.BodySectionName{Peek: false}
	emailBody := msg.GetBody(section)
	if emailBody == nil {
		return nil, fmt.Errorf("server didn't return message body")
	}

	// Read the entire raw email into a byte slice.
	rawEmail, err := io.ReadAll(emailBody)
	if err != nil {
		return nil, err
	}

	msgReader := bytes.NewReader(rawEmail)
	parsedMsg, err := mail.ReadMessage(msgReader)
	if err != nil {
		return nil, err
	}

	return &Email{msg: parsedMsg, rawMsg: rawEmail}, nil
}

func (m *Email) GetHeaders() mail.Header {
	return m.msg.Header
}

func (m *Email) GetHeader(header string) string {
	return m.msg.Header.Get(header)
}

func (m *Email) GetSender() (*mail.Address, error) {
	sender, err := m.msg.Header.AddressList("Sender")
	if err != nil {
		sender, err := m.msg.Header.AddressList("From")
		if err != nil {
			return nil, err
		}
		return sender[0], nil
	}
	return sender[0], nil
}

func (m *Email) GetSubject() string {
	return m.msg.Header.Get("Subject")
}
func (m *Email) GetRawEmail() []byte {
	return m.rawMsg
}

func (m *Email) GetBody() ([]byte, error) {
	body, err := io.ReadAll(m.msg.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func ExtractSpamStatus(email *Email) (float64, error) {
	value := email.GetHeader("X-Spam-Status")
	re := regexp.MustCompile(`hits=(\-?[\d\.]+)`)
	matches := re.FindStringSubmatch(value)
	if len(matches) >= 2 {
		// Parse the captured string as a float64.
		hits, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			return 0, fmt.Errorf("error parsing float: %v", err)
		}
		return hits, nil
	}
	return 0, nil

}

type LastProcessed struct {
	LastProcessedID uint32 `json:"last_processed_id"`
	Filename        string `json:"-"`
}

func NewLastProcessed(filename string) (LastProcessed, error) {
	cfg := LastProcessed{Filename: filename, LastProcessedID: 0}
	data, err := os.ReadFile(filename)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err

}

func (c LastProcessed) UpdateLastProcessed() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.Filename, data, 0644)
}
