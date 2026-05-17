package ia

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"

	"ozorg.xyz/flapjack/cache"
)

type IA struct {
	client   openai.Client
	model    string
	messages []openai.ChatCompletionMessageParamUnion
	hasKey   bool
}

func NewIA(system string, store *cache.Store) *IA {
	apiKey, _ := store.Get("GROQ_API_KEY")

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://api.groq.com/openai/v1"),
	)

	return &IA{
		client: client,
		model:  "llama-3.3-70b-versatile",
		hasKey: apiKey != "",
		messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(system),
		},
	}
}

func (ia *IA) Generate(
	ctx context.Context,
	prompt string,
) (string, error) {
	if !ia.hasKey {
		return "", fmt.Errorf("GROQ_API_KEY nao configurada. Use :config set GROQ_API_KEY <sua-chave>")
	}

	ia.messages = append(
		ia.messages,

		openai.UserMessage(prompt),
	)

	resp, err := ia.client.Chat.Completions.New(
		ctx,

		openai.ChatCompletionNewParams{
			Model: ia.model,

			Messages: ia.messages,
		},
	)

	if err != nil {
		return "", err
	}

	content := resp.Choices[0].Message.Content

	ia.messages = append(
		ia.messages,

		openai.AssistantMessage(content),
	)

	return content, nil
}

func (ia *IA) AddSystemContext(content string) {
	ia.messages = append(
		ia.messages,

		openai.SystemMessage(content),
	)
}

func (ia *IA) ClearContext() {
	if len(ia.messages) == 0 {
		return
	}

	system := ia.messages[0]

	ia.messages = []openai.ChatCompletionMessageParamUnion{
		system,
	}
}

func (ia *IA) Messages() []openai.ChatCompletionMessageParamUnion {
	return ia.messages
}

func (ia *IA) ExportMessages() []cache.StoredMessage {
	out := make([]cache.StoredMessage, 0, len(ia.messages))
	for _, m := range ia.messages {
		role := ""
		content := ""
		if m.OfSystem != nil {
			role = "system"
			if !param.IsOmitted(m.OfSystem.Content.OfString) {
				content = m.OfSystem.Content.OfString.Value
			}
		} else if m.OfUser != nil {
			role = "user"
			if !param.IsOmitted(m.OfUser.Content.OfString) {
				content = m.OfUser.Content.OfString.Value
			}
		} else if m.OfAssistant != nil {
			role = "assistant"
			if !param.IsOmitted(m.OfAssistant.Content.OfString) {
				content = m.OfAssistant.Content.OfString.Value
			}
		}
		if role != "" {
			out = append(out, cache.StoredMessage{Role: role, Content: content})
		}
	}
	return out
}

func (ia *IA) ImportMessages(msgs []cache.StoredMessage) {
	ia.messages = make([]openai.ChatCompletionMessageParamUnion, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "system":
			ia.messages = append(ia.messages, openai.SystemMessage(m.Content))
		case "user":
			ia.messages = append(ia.messages, openai.UserMessage(m.Content))
		case "assistant":
			ia.messages = append(ia.messages, openai.AssistantMessage(m.Content))
		}
	}
}
