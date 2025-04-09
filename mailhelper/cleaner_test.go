package mailhelper

import (
	"bytes"
	"fmt"
	"golang.org/x/net/html"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"testing"
)

func createEmail(contentType string, body []byte) *Email {
	h := mail.Header{}
	content := []string{contentType}
	h["Content-Type"] = content
	msg := &mail.Message{
		Header: h,
		Body:   bytes.NewReader(body),
	}
	return &Email{msg: msg, rawMsg: body}
}

func TestExtractText(t *testing.T) {
	// Helper function to create a text node.
	createText := func(data string) *html.Node {
		return &html.Node{
			Type: html.TextNode,
			Data: data,
		}
	}

	// Helper function to create an element node with given children.
	createElement := func(tag string, children ...*html.Node) *html.Node {
		n := &html.Node{
			Type: html.ElementNode,
			Data: tag,
		}
		if len(children) > 0 {
			n.FirstChild = children[0]
			for i := 0; i < len(children)-1; i++ {
				children[i].NextSibling = children[i+1]
			}
		}
		return n
	}

	// Define table-driven test cases.
	tests := []struct {
		name string
		node *html.Node
		want string
	}{
		{
			name: "single text node",
			node: createText("Hello"),
			want: "Hello ",
		},
		{
			name: "element with two text children",
			node: createElement("p", createText("Hello"), createText("World")),
			want: "Hello World ",
		},
		{
			name: "ignore script element",
			node: createElement("script", createText("alert('hi')")),
			want: "",
		},
		{
			name: "nested elements",
			node: createElement("div",
				createText("Hello"),
				createElement("span", createText("World")),
			),
			want: "Hello World ",
		},
		{
			name: "ignore style element inside nested elements",
			node: createElement("div",
				createText("Hello"),
				createElement("style", createText("body { color: red; }")),
				createText("World"),
			),
			want: "Hello World ",
		},
		{
			name: "ignore img element",
			node: createElement("img", createText("Not to be included")),
			want: "",
		},
		{
			name: "text node with extra whitespace",
			node: createText("  spaced  "),
			want: "spaced ",
		},
	}

	// Run each test case.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractText(tt.node)
			if got != tt.want {
				t.Errorf("extractText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanEmailBody(t *testing.T) {
	t.Run("multipart email with html and plain text parts", func(t *testing.T) {
		// Build a multipart message.
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		boundary := writer.Boundary()

		// Create the HTML part.
		htmlHeader := make(textproto.MIMEHeader)
		htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
		htmlPart, err := writer.CreatePart(htmlHeader)
		if err != nil {
			t.Fatalf("Error creating HTML part: %v", err)
		}
		// HTML content: expect extractText to output "Hello World " (trailing space added).
		htmlContent := `<html><body><p>Hello <b>World</b></p></body></html>`
		_, err = htmlPart.Write([]byte(htmlContent))
		if err != nil {
			t.Fatalf("Error writing HTML part: %v", err)
		}

		// Create the plain text part.
		textHeader := make(textproto.MIMEHeader)
		textHeader.Set("Content-Type", "text/plain; charset=utf-8")
		textPart, err := writer.CreatePart(textHeader)
		if err != nil {
			t.Fatalf("Error creating text part: %v", err)
		}
		textContent := "Plain text content"
		_, err = textPart.Write([]byte(textContent))
		if err != nil {
			t.Fatalf("Error writing text part: %v", err)
		}
		// Close the multipart writer to finalize the message.
		writer.Close()

		// Set the email header to indicate a multipart message.
		ctHeader := fmt.Sprintf("multipart/alternative; boundary=%s", boundary)
		email := createEmail(ctHeader, buf.Bytes())

		// Execute CleanEmailBody.
		result, err := CleanEmailBody(email)
		if err != nil {
			t.Fatalf("CleanEmailBody returned error: %v", err)
		}
		// Expected: the HTML part produces "Hello World " and the plain text part is appended.
		expected := "Hello World Plain text content"
		if result != expected {
			t.Errorf("Expected %q but got %q", expected, result)
		}
	})

	t.Run("single-part html email", func(t *testing.T) {
		// Build a single-part HTML email.
		htmlContent := `<html><body><p>Hello <i>Single Part</i></p></body></html>`
		contentType := "text/html; charset=utf-8"
		email := createEmail(contentType, []byte(htmlContent))

		result, err := CleanEmailBody(email)
		if err != nil {
			t.Fatalf("CleanEmailBody returned error: %v", err)
		}
		// The extracted text should combine the text nodes.
		expected := "Hello Single Part "
		if result != expected {
			t.Errorf("Expected %q but got %q", expected, result)
		}
	})

	t.Run("single-part plain text email", func(t *testing.T) {
		// For non-HTML single-part emails, CleanEmailBody doesn't append the body text (it only prints it).
		textContent := "Just plain text"
		contentType := "text/plain; charset=utf-8"
		email := createEmail(contentType, []byte(textContent))

		result, err := CleanEmailBody(email)
		if err != nil {
			t.Fatalf("CleanEmailBody returned error: %v", err)
		}
		// Expected output is empty because the plain text branch in single-part doesn't write to the builder.
		expected := ""
		if result != expected {
			t.Errorf("Expected %q but got %q", expected, result)
		}
	})
}
