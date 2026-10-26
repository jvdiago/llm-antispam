package main

import (
	"context"
	"fmt"
	"gopkg.in/yaml.v2"
	"llm-antispam/llm"
	"llm-antispam/mailhelper"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/emersion/go-imap/client"
	"github.com/spf13/cobra"
)

// To mock functions in unit testing
var (
	FetchUnreadEmails = mailhelper.FetchUnreadEmails
	NewLastProcessed  = mailhelper.NewLastProcessed
	ClassifySpam      = mailhelper.ClassifySpam
	MoveEmails        = mailhelper.MoveEmails
)

type Config struct {
	Rules        []Rule   `yaml:"rules"`
	Domains      []string `yaml:"whitelisted_domains"`
	Interval     uint32   `yaml:"interval"`
	Concurrency  bool     `yaml:"concurrency"`
	UidFilesPath string   `yaml:"uid_files_path"`
	LLM          LLM      `yaml:"llm"`
}

type LLM struct {
	URL   string `yaml:"url"`
	Model string `yaml:"model"`
}

// Rule represents each rule in the YAML file.
type Rule struct {
	Origin      string  `yaml:"origin"`
	Destination string  `yaml:"destination"`
	Threshold   float64 `yaml:"threshold"`
	MoveNotSpam bool    `yaml:"move_not_spam"`
}

// NewConfig returns a new decoded Config struct
func NewConfig(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("error closing file: %v", err)
		}
	}()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

// ValidateConfigPath just makes sure, that the path provided is a file,
// that can be read
func ValidateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%s' is a directory, not a normal file", path)
	}
	return nil
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "llm-antispam",
		Short: "LLM-based antispam tool",
		RunE:  runDaemon,
	}

	scoreCmd := &cobra.Command{
		Use:   "score <email-file>",
		Short: "Score a single email file",
		Args:  cobra.ExactArgs(1),
		RunE:  runScore,
	}

	rootCmd.PersistentFlags().StringVarP(&configPathFlag, "config", "c", "./config.yaml", "path to config file")
	rootCmd.AddCommand(scoreCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var configPathFlag string

func runDaemon(cmd *cobra.Command, args []string) error {
	imapServer, exists := os.LookupEnv("IMAP_SERVER")
	if !exists {
		log.Fatal("IMAP_SERVER env var not found")
	}
	imapUser, exists := os.LookupEnv("IMAP_USER")
	if !exists {
		log.Fatal("IMAP_USER env var not found")
	}
	imapPassword, exists := os.LookupEnv("IMAP_PASSWORD")
	if !exists {
		log.Fatal("IMAP_PASSWORD env var not found")
	}

	cfg, err := NewConfig(configPathFlag)
	if err != nil {
		return err
	}

	// Create a channel to listen for the SIGTSTP signal.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTSTP, syscall.SIGTERM)

	// Create a ticker to process emails periodically.
	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Second)
	defer ticker.Stop()

	imapConfig := &mailhelper.IMAP{
		User:     imapUser,
		Password: imapPassword,
		Server:   imapServer,
	}

	// Connect to the IMAP server.
	c, err := imapConfig.Connect()
	if err != nil {
		return fmt.Errorf("error connecting to IMAP server: %w", err)
	}
	defer func() {
		if err := c.Logout(); err != nil {
			log.Printf("error logging out: %v", err)
		}
	}()

	log.Println("Connected to IMAP server successfully!")

	// Run the processing loop until SIGTSTP is received.
	for {
		select {
		case <-sigChan:
			log.Println("Signal received, shutting down gracefully...")
			return nil
		case <-ticker.C:
			client := llm.NewClient(cfg.LLM.URL, cfg.LLM.Model)
			// Classify the email
			llmClassifier := client.ClassifyEmail

			if err != nil {
				return fmt.Errorf("error creating LLM: %w", err)
			}
			for _, config := range cfg.Rules {
				err := RunRule(c, config, cfg.Domains, llmClassifier, cfg.Concurrency, cfg.UidFilesPath)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func runScore(cmd *cobra.Command, args []string) error {
	emailFile := args[0]

	// Read the email file
	rawEmail, err := os.ReadFile(emailFile)
	if err != nil {
		return fmt.Errorf("error reading email file: %w", err)
	}

	email, err := mailhelper.NewEmailFromBytes(rawEmail)
	if err != nil {
		return fmt.Errorf("error parsing email: %w", err)
	}

	// Load config to get LLM settings
	cfg, err := NewConfig(configPathFlag)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	client := llm.NewClient(cfg.LLM.URL, cfg.LLM.Model)
	// Classify the email
	ctx := context.Background()
	score, err := client.ClassifyEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("error classifying email: %w", err)
	}

	fmt.Printf("spam_score: %.1f\n", score)
	return nil
}

func RunRule(c *client.Client, config Rule, domains []string, llmClassifier mailhelper.ClassifierFunc, concurrency bool, UidFilesPath string) error {
	// Retrieve unread emails from the origin folder.
	messages, ids, done := FetchUnreadEmails(c, config.Origin)

	lastProcessed, err := NewLastProcessed(fmt.Sprintf("%slast_processed_%s.json", UidFilesPath, config.Origin))
	if err != nil {
		log.Printf("Error reading previous last processed ID: %v", err)
	}

	spamSeqSet, notSpamSeqSet, lastUid, err := ClassifySpam(
		messages, ids, domains, config.Threshold, lastProcessed.LastProcessedID, llmClassifier, concurrency)
	if err != nil {
		return fmt.Errorf("error classifying spam: %v", err)
	}

	mailSeqSet := spamSeqSet
	if config.MoveNotSpam {
		mailSeqSet = notSpamSeqSet
	}

	log.Printf("Spam: %v, Not Spam: %v", spamSeqSet.Set, notSpamSeqSet.Set)
	if len(mailSeqSet.Set) > 0 {
		log.Printf("Moving emails from %s to %s: %v", config.Origin, config.Destination, mailSeqSet)
		err = MoveEmails(c, mailSeqSet, config.Destination, config.Origin)
		if err != nil {
			log.Printf("Error moving emails %v: %v", mailSeqSet, err)
		}
	}
	if err := <-done; err != nil {
		log.Printf("Error during fetch in mailbox %q: %v", config.Origin, err)
	}

	// Update the last processed ID.
	if lastUid > 0 {
		lastProcessed.LastProcessedID = lastUid
		err := lastProcessed.UpdateLastProcessed()
		if err != nil {
			log.Printf("Error updating last processed %v", err)
		}

	}
	return nil
}
