package mailhelper

import (
	"fmt"
	"sync"

	"github.com/emersion/go-imap"
	"github.com/tmc/langchaingo/llms"

	"llm-antispam/llm"
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

func ClassifySpam(
	messages <-chan *imap.Message,
	ids []uint32,
	whitelisted_domains []string,
	threshold float64,
	lastProcessedID uint32,
	llmClassifier llms.Model,
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

		subject := email.GetSubject()
		sender, err := email.GetSender()

		if err != nil {
			log.Printf("Error getting sender: %v", err)
			continue
		}
		whitelisted := IsWhitelistedEmail(sender.Address, whitelisted_domains)

		if whitelisted {
			continue
		}

		spamStatus, err := ExtractSpamStatus(email)
		if err != nil {
			log.Printf("Error parsing Spam Status: %v", err)
			continue
		}

		bodyText, err := CleanEmailBody(email)
		if err != nil {
			log.Printf("Error cleaning email: %v", err)
			continue
		}

		cleanedMail := fmt.Sprintf(
			`FROM: %s
SUBJECT: %s

%s`,
			sender, subject, bodyText)

		wg.Add(1)
		go func() {
			defer wg.Done()
			score, reason, err := llm.ClassifyEmail(llmClassifier, cleanedMail)
			result := llmResult{
				score:      score,
				err:        err,
				id:         id,
				spamStatus: spamStatus,
				reason:     reason,
				sender:     sender.Address,
				subject:    subject,
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
