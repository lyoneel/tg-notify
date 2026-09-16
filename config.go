package tgnotify

import (
	"errors"
	"os"
)

// Environment variables read by FromEnv.
const (
	EnvToken   = "TELEGRAM_BOT_TOKEN"
	EnvChatID  = "TELEGRAM_CHAT_ID"
	EnvBaseURL = "TELEGRAM_BASE_URL"
	EnvProxy   = "TELEGRAM_PROXY"
)

// EnvConfig holds the values resolved from the environment.
type EnvConfig struct {
	Token   string // EnvToken; required
	ChatID  string // EnvChatID; required
	BaseURL string // EnvBaseURL; optional
	Proxy   string // EnvProxy; optional
}

// FromEnv builds a Bot and the target chat ID from the environment
// variables EnvToken and EnvChatID (both required), and applies
// EnvBaseURL and EnvProxy when set. It is the library-side mirror of
// what the CLI resolves from the environment; callers that need flag
// overrides read the variables themselves and configure the Bot with
// the setters.
func FromEnv() (*Bot, string, error) {
	cfg := EnvConfig{
		Token:   os.Getenv(EnvToken),
		ChatID:  os.Getenv(EnvChatID),
		BaseURL: os.Getenv(EnvBaseURL),
		Proxy:   os.Getenv(EnvProxy),
	}
	if cfg.Token == "" {
		return nil, "", errors.New("bot token required: set " + EnvToken)
	}
	if cfg.ChatID == "" {
		return nil, "", errors.New("chat ID required: set " + EnvChatID)
	}
	bot := New(cfg.Token)
	if cfg.BaseURL != "" {
		bot.SetBaseURL(cfg.BaseURL)
	}
	if cfg.Proxy != "" {
		if err := bot.SetProxy(cfg.Proxy); err != nil {
			return nil, "", err
		}
	}
	return bot, cfg.ChatID, nil
}
