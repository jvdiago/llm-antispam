package mailhelper

import (
	"context"
	"sync"

	"github.com/emersion/go-imap"
	"log"
)

type llmResult struct {
	id         uint32
	score      float64
	reason     string
	err        error
	spamStatus float64
	subject    string
	sender     string
}

// ClassifierFunc is a function that classifies an email and returns a spam score and reason.
type ClassifierFunc func(ctx context.Context, email *Email) (float64, error)

// ClassifySpam processes email messages and classifies them as spam or not spam.
// It accepts a classifier function for dependency injection, making it testable.
func ClassifySpam(
	messages <-chan *imap.Message,
	ids []uint32,
	whitelisted_domains []string,
	threshold float64,
	lastProcessedID uint32,
	classifier ClassifierFunc,
	concurrency bool,
) (*imap.SeqSet, *imap.SeqSet, uint32, error) {
	count := 0

	spamSeqset := new(imap.SeqSet)
	notSpamSeqset := new(imap.SeqSet)
	var lastUid uint32 = 0
	var wg sync.WaitGroup
	scoreChannel := make(chan llmResult, 10)

	for msg := range messages {
		id := ids[count]
		lastUid = msg.Uid

		// Early to avoid any continue not incrementing it
		count++

		if lastUid <= lastProcessedID {
			continue
		}
		email, err := NewEmail(msg)
		if err != nil {
			log.Printf("Error converting message to Email %v", err)
			continue
		}

		whitelisted := IsWhitelistedEmail(email.From, whitelisted_domains)

		if whitelisted {
			continue
		}

		spamStatus, err := ExtractSpamStatus(email)
		if err != nil {
			log.Printf("Error parsing Spam Status: %v", err)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			score, err := classifier(ctx, email)
			result := llmResult{
				score:      score,
				err:        err,
				id:         id,
				spamStatus: spamStatus,
				sender:     email.From,
				subject:    email.Subject,
			}
			scoreChannel <- result
		}()

		// If we cannot afford concurrency just wait after each goroutine
		if !concurrency {
			wg.Wait()
		}

	}
	go func() {
		defer close(scoreChannel)
		wg.Wait()
	}()

	for result := range scoreChannel {
		if result.err != nil {
			log.Println(result.err)
			continue
		}
		log.Printf(
			"New email processed. From: %s. Subject: %s. Old Spam Score: %f. New Spam Score: %f. Reason: %s",
			result.sender, result.subject, result.spamStatus, result.score, result.reason,
		)
		if result.score > threshold {
			spamSeqset.AddNum(result.id)
		} else {
			notSpamSeqset.AddNum(result.id)
		}
	}
	return spamSeqset, notSpamSeqset, lastUid, nil
}
