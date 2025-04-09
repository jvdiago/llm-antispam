package main

import (
	"flag"
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
	"github.com/tmc/langchaingo/llms"
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
	Provider string `yaml:"provider"`
	ModelID  string `yaml:"model_id"`
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
	defer file.Close()

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

// ParseFlags will create and parse the CLI flags
// and return the path to be used elsewhere
func ParseFlags() (string, error) {
	// String that contains the configured configuration path
	var configPath string

	// Set up a CLI flag called "-config" to allow users
	// to supply the configuration file
	flag.StringVar(&configPath, "config", "./config.yaml", "path to config file")

	// Actually parse the flags
	flag.Parse()

	// Validate the path first
	if err := ValidateConfigPath(configPath); err != nil {
		return "", err
	}

	// Return the configuration path
	return configPath, nil
}

func RunRule(c *client.Client, config Rule, domains []string, llmClassifier llms.LLM, concurrency bool, UidFilesPath string) error {
	// Retrieve unread emails from the origin folder.
	messages, ids, done := FetchUnreadEmails(c, config.Origin)

	lastProcessed, err := NewLastProcessed(fmt.Sprintf("%slast_processed_%s.json", UidFilesPath, config.Origin))
	if err != nil {
		log.Printf("Error reading previous last processed ID: %v", err)
	}

	spamSeqSet, notSpamSeqSet, lastUid, err := ClassifySpam(
		messages, ids, domains, config.Threshold, lastProcessed.LastProcessedID, llmClassifier, concurrency)
	if err != nil {
		return fmt.Errorf("Error classifying spam: %v", err)
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
		lastProcessed.UpdateLastProcessed()
	}
	return nil
}
func main() {
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
		log.Fatal("imap_PASSWORD env var not found")
	}

	// Generate our config based on the config supplied
	// by the user in the flags
	cfgPath, err := ParseFlags()
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := NewConfig(cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	// Create a channel to listen for the SIGTSTP signal.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTSTP, syscall.SIGTERM)

	// Create a ticker to process emails periodically (e.g., every 300 seconds).
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
		log.Fatal("Error connecting to IMAP server:", err)
	}
	defer c.Logout()

	log.Println("Connected to IMAP server successfully!")

	// Run the processing loop until SIGTSTP is received.
	for {
		select {
		case <-sigChan:
			log.Println("Signal received, shutting down gracefully...")
			return
		case <-ticker.C:
			llmClassifier, err := llm.LLMFactory(cfg.LLM.Provider, cfg.LLM.ModelID)
			if err != nil {
				log.Fatalf("Error creating LLM: %v", err)
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
