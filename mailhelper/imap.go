package mailhelper

import (
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type IMAP struct {
	User      string
	Password  string
	Server    string
	Mailboxes []string
}

func (i *IMAP) Connect() (*client.Client, error) {
	// Dial the IMAP server over TLS.
	c, err := client.DialTLS(i.Server, nil)
	if err != nil {
		return nil, err
	}

	// Login with user credentials.
	if err := c.Login(i.User, i.Password); err != nil {
		return nil, err
	}

	return c, nil
}

// fetchUnreadEmails selects the given mailbox and prints the subject of unread emails.
func FetchUnreadEmails(c *client.Client, mailbox string) (<-chan *imap.Message, []uint32, <-chan error) {
	// Select the mailbox (read-only)
	done := make(chan error, 1)
	_, err := c.Select(mailbox, true)
	if err != nil {
		done <- fmt.Errorf("unable to select mailbox %q: %v", mailbox, err)
		return nil, nil, done
	}

	// Set up search criteria for unread messages (i.e. messages without the \Seen flag)
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{"\\Seen"}
	ids, err := c.Search(criteria)
	if err != nil {
		done <- fmt.Errorf("search failed in mailbox %q: %v", mailbox, err)
		return nil, nil, done
	}

	if len(ids) == 0 {
		fmt.Printf("No unread messages in %s\n", mailbox)
		return nil, nil, nil
	}

	// Create a set of message sequence numbers to fetch
	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)

	// Fetch the envelope (header) of each unread message
	messages := make(chan *imap.Message, 10)
	section := &imap.BodySectionName{Peek: false}
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{section.FetchItem(), imap.FetchUid}, messages)
	}()

	return messages, ids, done
}

func MoveEmails(c *client.Client, seqset *imap.SeqSet, destinationMailbox string, originMailbox string) error {
	if seqset.Empty() {
		return nil
	}
	_, err := c.Select(originMailbox, false)
	if err != nil {
		return fmt.Errorf("error opening the mailbox %s in Read-Write: %v", originMailbox, err)
	}
	// Step 1: Copy the message to the SPAM folder.
	err = c.Copy(seqset, destinationMailbox)
	if err != nil {
		return fmt.Errorf("error copying message to SPAM: %v", err)
	}

	// Step 2: Mark the original message as deleted.
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.DeletedFlag}

	err = c.Store(seqset, item, flags, nil)
	if err != nil {
		return fmt.Errorf("error marking message as deleted: %v", err)
	}

	// Step 3: Permanently remove (expunge) messages flagged as deleted.
	err = c.Expunge(nil)
	if err != nil {
		return fmt.Errorf("error expunging messages: %v", err)
	}
	return nil
}
