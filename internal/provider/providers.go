package provider

import (
	"fmt"
	"os"
	"strings"

	"github.com/damiaoterto/jandaira/internal/brain"
	"github.com/damiaoterto/jandaira/internal/i18n"
	"github.com/damiaoterto/jandaira/internal/security"
)

type Factory struct {
	EnvKey       string
	DefaultModel string
	Build        func(apiKey, model string) (brain.Brain, brain.Brain, error)
}

func makeFactories(maxTokensFn func() int, vault *security.Vault) map[string]Factory {
	return map[string]Factory{
		"anthropic": {
			EnvKey:       "ANTHROPIC_API_KEY",
			DefaultModel: "claude-sonnet-4-6",
			Build: func(apiKey, model string) (brain.Brain, brain.Brain, error) {
				ab := brain.NewAnthropicBrain(apiKey, model)
				ab.MaxTokensFn = maxTokensFn
				if oaiKey := resolveAPIKey("OPENAI_API_KEY", vault); oaiKey != "" {
					return ab, brain.NewOpenAIBrain(oaiKey, "gpt-4o-mini"), nil
				}
				return ab, ab, nil
			},
		},
		"gemini": {
			EnvKey:       "GEMINI_API_KEY",
			DefaultModel: "gemini-2.0-flash",
			Build: func(apiKey, model string) (brain.Brain, brain.Brain, error) {
				gb, err := brain.NewGeminiBrain(apiKey, model)
				if err != nil {
					return nil, nil, err
				}
				gb.MaxTokensFn = maxTokensFn
				return gb, gb, nil
			},
		},
		"openrouter": {
			EnvKey:       "OPENROUTER_API_KEY",
			DefaultModel: "gpt-4o-mini",
			Build: func(apiKey, model string) (brain.Brain, brain.Brain, error) {
				rb := brain.NewOpenRouterBrain(apiKey, model)
				rb.MaxTokensFn = maxTokensFn
				return rb, rb, nil
			},
		},
		"groq": {
			EnvKey:       "GROQ_API_KEY",
			DefaultModel: "gpt-4o-mini",
			Build: func(apiKey, model string) (brain.Brain, brain.Brain, error) {
				gb := brain.NewGroqBrain(apiKey, model)
				gb.MaxTokensFn = maxTokensFn
				return gb, gb, nil
			},
		},
		"openai": {
			EnvKey:       "OPENAI_API_KEY",
			DefaultModel: "gpt-4o-mini",
			Build: func(apiKey, model string) (brain.Brain, brain.Brain, error) {
				ob := brain.NewOpenAIBrain(apiKey, model)
				ob.MaxTokensFn = maxTokensFn
				return ob, ob, nil
			},
		},
	}
}

func resolveAPIKey(envKey string, vault *security.Vault) string {
	key := strings.TrimSpace(os.Getenv(envKey))
	if key == "" && vault != nil {
		if k, err := vault.GetSecret(envKey); err == nil {
			key = strings.TrimSpace(k)
		}
	}
	return key
}

func factoryFor(providerName string, factories map[string]Factory) Factory {
	f, ok := factories[providerName]
	if !ok {
		return factories["openai"]
	}
	return f
}

// BuildBrains resolves the API key from env/vault and builds active + embed brains.
func BuildBrains(providerName, configuredModel string, vault *security.Vault, maxTokensFn func() int) (brain.Brain, brain.Brain, error) {
	factories := makeFactories(maxTokensFn, vault)
	factory := factoryFor(providerName, factories)

	modelType := factory.DefaultModel
	if configuredModel != "" {
		modelType = configuredModel
	}

	apiKey := resolveAPIKey(factory.EnvKey, vault)
	if apiKey == "" {
		fmt.Println(i18n.T("warn_api_key_not_set"))
		apiKey = "sk-mock-key-for-testing"
	} else {
		os.Setenv(factory.EnvKey, apiKey)
	}

	return factory.Build(apiKey, modelType)
}

// BuildBrainsWithKey saves apiKey to vault, sets env var, and builds active + embed brains.
func BuildBrainsWithKey(providerName, apiKey, model string, vault *security.Vault, maxTokensFn func() int) (brain.Brain, brain.Brain, error) {
	factories := makeFactories(maxTokensFn, vault)
	factory := factoryFor(providerName, factories)

	if vault != nil {
		_ = vault.SaveSecret(factory.EnvKey, apiKey)
	}
	os.Setenv(factory.EnvKey, apiKey)

	return factory.Build(apiKey, model)
}

// DefaultModel returns the default model name for the given provider.
func DefaultModel(providerName string) string {
	factories := makeFactories(nil, nil)
	return factoryFor(providerName, factories).DefaultModel
}

// IsValid reports whether providerName is a known provider.
func IsValid(providerName string) bool {
	factories := makeFactories(nil, nil)
	_, ok := factories[providerName]
	return ok
}
