package mailhelper

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	"golang.org/x/net/html"
)

func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		// Return trimmed text with a trailing space.
		return strings.TrimSpace(n.Data) + " "
	}

	// Skip over style, script, and img elements.
	if n.Type == html.ElementNode && (n.Data == "style" || n.Data == "script" || n.Data == "img") {
		return ""
	}

	var buf bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		buf.WriteString(extractText(c))
	}
	return buf.String()
}

func CleanEmailBody(email *Email) (string, error) {
	var builder strings.Builder
	ctHeader := email.GetHeader("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ctHeader)
	if err != nil {
		return "", fmt.Errorf("error parsing Content-Type header: %v", err)
	}

	// If the email is multipart, process each part.
	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(email.msg.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}

			partCt := part.Header.Get("Content-Type")
			partData, err := io.ReadAll(part)
			if err != nil {
				return "", err
			}

			if strings.Contains(partCt, "text/html") {
				// Parse the HTML part and extract plain text.
				doc, err := html.Parse(bytes.NewReader(partData))
				if err != nil {
					return "", fmt.Errorf("error parsing HTML part: %v", err)
				}
				cleanText := extractText(doc)
				builder.WriteString(cleanText)
			} else if strings.Contains(partCt, "text/plain") {
				// For plain text, print as is.
				builder.WriteString(string(partData))
			}
		}
	} else {
		// Single-part email: process based on the Content-Type.
		body, err := email.GetBody()
		if err != nil {
			return "", err
		}
		if strings.Contains(ctHeader, "text/html") {
			doc, err := html.Parse(bytes.NewReader(body))
			if err != nil {
				return "", fmt.Errorf("error parsing HTML: %v", err)
			}
			cleanText := extractText(doc)
			builder.WriteString(cleanText)
		} else {
			fmt.Println(string(body))
		}
	}
	return builder.String(), nil
}
