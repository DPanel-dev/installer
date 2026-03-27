package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed translations/*.json
var translationsFS embed.FS

type Translator struct {
	lang  string
	words map[string]string
	mu    sync.RWMutex
}

var (
	defaultTranslator *Translator
	mu                sync.RWMutex
)

// Init initializes the global translator with the specified language
// Can be called multiple times to change language
func Init(lang string) error {
	t, err := NewTranslator(lang)
	if err != nil {
		return err
	}

	mu.Lock()
	defaultTranslator = t
	mu.Unlock()

	return nil
}

// NewTranslator creates a new translator for the specified language
func NewTranslator(lang string) (*Translator, error) {
	t := &Translator{
		lang:  lang,
		words: make(map[string]string),
	}

	if err := t.loadTranslations(); err != nil {
		return nil, err
	}

	return t, nil
}

func (t *Translator) loadTranslations() error {
	filename := fmt.Sprintf("translations/%s.json", t.lang)
	data, err := translationsFS.ReadFile(filename)
	if err != nil {
		// Fallback to English if translation file not found
		data, err = translationsFS.ReadFile("translations/en.json")
		if err != nil {
			return fmt.Errorf("failed to load translations: %w", err)
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if err := json.Unmarshal(data, &t.words); err != nil {
		return fmt.Errorf("failed to parse translations: %w", err)
	}

	return nil
}

// T returns the translation for the given key
func (t *Translator) T(key string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if val, ok := t.words[key]; ok {
		return val
	}
	return key
}

// Tf returns the formatted translation for the given key
func (t *Translator) Tf(key string, args ...interface{}) string {
	return fmt.Sprintf(t.T(key), args...)
}

// Global functions for convenience

// T returns the translation for the given key using the global translator
func T(key string) string {
	if defaultTranslator == nil {
		return key
	}
	return defaultTranslator.T(key)
}
