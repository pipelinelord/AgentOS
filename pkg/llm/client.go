package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type Client struct {
	client *genai.Client
	model  *genai.GenerativeModel
	cs     *genai.ChatSession
}

func NewClient(ctx context.Context, modelName string, systemInstruction string) (*Client, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Use user-specified model, e.g., "gemini-1.5-pro"
	if modelName == "" {
		modelName = "gemini-3.5-flash"
	}
	model := client.GenerativeModel(modelName)
	if systemInstruction != "" {
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(systemInstruction)},
		}
	}

	cs := model.StartChat()

	return &Client{
		client: client,
		model:  model,
		cs:     cs,
	}, nil
}

func (c *Client) SendMessage(ctx context.Context, msg string) (string, error) {
	resp, err := c.cs.SendMessage(ctx, genai.Text(msg))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates returned")
	}

	var out string
	for _, part := range resp.Candidates[0].Content.Parts {
		if t, ok := part.(genai.Text); ok {
			out += string(t)
		}
	}

	return out, nil
}

func (c *Client) Reset() {
	c.cs = c.model.StartChat()
}

func (c *Client) Close() {
	c.client.Close()
}
