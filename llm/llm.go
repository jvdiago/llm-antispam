package llm

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/bedrock"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/outputparser"
)

// LLMType is an enum representing the supported LLM providers.
type LLMType int

const (
	LLMTypeBedrock LLMType = iota
	LLMTypeOpenAI
	LLMTypeOllama
)

func LLMFactory(provider string, modelId string) (llms.Model, error) {
	var providerType LLMType
	switch provider {
	case "ollama":
		providerType = LLMTypeOllama
	case "openai":
		providerType = LLMTypeOpenAI
	case "bedrock":
		providerType = LLMTypeBedrock
	default:
		return nil, fmt.Errorf("provider %s not found", provider)
	}

	llmClassifier, err := NewLLM(providerType, modelId)

	if err != nil {
		return nil, fmt.Errorf("error creating LLM: %v", err)
	}
	return llmClassifier, nil

}

func NewLLM(llmType LLMType, modelId string) (llms.Model, error) {
	switch llmType {
	case LLMTypeBedrock:
		// Load AWS configuration with the desired region.
		cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
		// Create a Bedrock client with the custom configuration.
		client := bedrockruntime.NewFromConfig(cfg)
		myLLM, err := bedrock.New(
			bedrock.WithModel(modelId),
			bedrock.WithClient(client),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create Bedrock LLM: %w", err)
		}
		return myLLM, nil
	case LLMTypeOpenAI:
		// Create an OpenAI LLM.
		// Make sure your API key is set in the environment or passed as an option.
		myLLM, err := openai.New(
			openai.WithModel(modelId),
			// Optionally, you can provide additional configuration, e.g., API key:
			// openai.WithAPIKey("your-api-key"),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OpenAI LLM: %w", err)
		}
		return myLLM, nil
	case LLMTypeOllama:
		myLLM, err := ollama.New(ollama.WithModel(modelId))
		if err != nil {
			return nil, fmt.Errorf("failed to create Ollama LLM with model ID %s: %w", modelId, err)
		}
		return myLLM, nil
	default:
		return nil, fmt.Errorf("unsupported LLM type")
	}
}

func ClassifyEmail(llm llms.Model, body string) (float64, string, error) {
	responseSchema := []outputparser.ResponseSchema{
		{Name: "SpamScore", Description: "Spam Score as Float between 0 and 10, less than 5 is considered not Spam, converted to string"},
		{Name: "Reason", Description: "Brief 1 line sentence explaining the SPAM score assigned"},
	}
	parser := outputparser.NewStructured(responseSchema)

	// Construct the prompt by including the email headers and body.
	prompt := fmt.Sprintf(
		`You are an Email Spam classifier. Analyze the following email and calculate the SPAM numerical score.
Focus on the content and the intent of the email, and verify if they come from well-known domains and companies.
Do not categorize as Spam emails from well-known organizations like github.com, meetup.com, etc. or well-known email providers like hotmail.com or gmail.com.
Check for any links in the body and verify if they are legitimate.
The HTML tags and images have been removed for simplicity.
Only return the output as specified below.

Output:
%s

Email Body:
%s`,
		parser.GetFormatInstructions(), body,
	)

	ctx := context.Background()
	resp, err := llm.GenerateContent(
		ctx,
		[]llms.MessageContent{
			{
				Role: llms.ChatMessageTypeHuman,
				Parts: []llms.ContentPart{
					llms.TextPart(prompt),
				},
			},
		},
		llms.WithTemperature(0.1),
	)
	if err != nil {
		return 0, "", err
	}

	choices := resp.Choices
	if len(choices) < 1 {
		return 0, "", fmt.Errorf("empty response from model")
	}

	parsed, err := parser.Parse(choices[0].Content)
	if err != nil {
		return 0, "", fmt.Errorf("error parsing LLM result: %v", err)
	}

	// Assert that parsedAny is a map[string]interface{}
	resultMap, ok := parsed.(map[string]string)
	if !ok {
		return 0, "", fmt.Errorf("failed to assert parsed result as map[string]string")
	}

	//fmt.Println(resultMap)
	score, err := strconv.ParseFloat(resultMap["SpamScore"], 64)
	if err != nil {
		return 0, "", fmt.Errorf("error converting score to float: %v", err)
	}

	reason := resultMap["Reason"]
	return score, reason, nil

}
