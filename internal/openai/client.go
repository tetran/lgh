package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ChatRequest struct {
	Model       string     `json:"model"`
	Messages    []*Message `json:"messages"`
	Temperature float64    `json:"temperature"`
}

type ChatResponse struct {
	Choices []*Choice `json:"choices"`
	Usage   *Usage    `json:"usage"`
}

type Choice struct {
	Message *Message `json:"message"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Client struct {
	ApiKey string
	Model  string
	Debug  bool
}

func (c *Client) Chat(messages []*Message) (*ChatResponse, error) {
	url := "https://api.openai.com/v1/chat/completions"
	creq := &ChatRequest{
		Model:       c.Model,
		Messages:    messages,
		Temperature: 0.7,
	}
	if c.Debug {
		creq.print()
	}
	body, err := json.Marshal(creq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		url,
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.ApiKey)

	client := http.Client{
		Timeout: 60 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	cres := &ChatResponse{}
	err = json.NewDecoder(res.Body).Decode(&cres)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		cres.print()
	}

	return cres, nil
}

func (r *ChatRequest) print() {
	fmt.Printf("\n## Chat Request\n")
	fmt.Println("### Model")
	fmt.Println(r.Model)
	fmt.Println("### Prompts")
	for _, m := range r.Messages {
		fmt.Printf("- %s \n%s\n", m.Role, m.Content)
	}
}

func (r *ChatResponse) print() {
	fmt.Printf("\n## Chat Response\n")
	fmt.Printf("### Choice\n%s\n", r.Choices[0].Message.Content)
	fmt.Printf(
		"\n### Usages\ntotal: %d (prompt: %d, completion: %d)\n",
		r.Usage.TotalTokens,
		r.Usage.PromptTokens,
		r.Usage.CompletionTokens)
}
