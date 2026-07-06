package domain

import "time"

type ApiKey struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider"`
	KeyEnc    string    `json:"-"`
	Label     string    `json:"label,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ApiKeyResponse struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider"`
	Label     string    `json:"label,omitempty"`
	HasKey    bool      `json:"has_key"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var ProviderEnvVars = map[string]string{
	"anthropic":  "ANTHROPIC_API_KEY",
	"openai":     "OPENAI_API_KEY",
	"google":     "GOOGLE_API_KEY",
	"groq":       "GROQ_API_KEY",
	"deepseek":   "DEEPSEEK_API_KEY",
	"mistral":    "MISTRAL_API_KEY",
	"cohere":     "COHERE_API_KEY",
	"together":   "TOGETHER_API_KEY",
	"openrouter": "OPENROUTER_API_KEY",
	"xai":        "XAI_API_KEY",
	"huggingface": "HUGGINGFACE_API_KEY",
}
