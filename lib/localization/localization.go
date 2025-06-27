package localization

import (
	"embed"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

type LocalizationService struct {
	bundle *i18n.Bundle
}

var (
	globalService *LocalizationService
	once          sync.Once
)

func NewLocalizationService() *LocalizationService {
	once.Do(func() {
		bundle := i18n.NewBundle(language.English)
		bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

		// Read all JSON files from the locales directory
		entries, err := localeFS.ReadDir("locales")
		if err != nil {
			// Try fallback - create a minimal service with default messages
			globalService = &LocalizationService{bundle: bundle}
			return
		}

		loadedAny := false
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				filePath := "locales/" + entry.Name()
				_, err := bundle.LoadMessageFileFS(localeFS, filePath)
				if err != nil {
					// Log error but continue with other files
					continue
				}
				loadedAny = true
			}
		}

		if !loadedAny {
			// If no files were loaded successfully, create minimal service
			globalService = &LocalizationService{bundle: bundle}
			return
		}

		globalService = &LocalizationService{bundle: bundle}
	})
	
	// Safety check - if globalService is still nil, create a minimal one
	if globalService == nil {
		bundle := i18n.NewBundle(language.English)
		bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
		globalService = &LocalizationService{bundle: bundle}
	}
	
	return globalService
}

func (ls *LocalizationService) GetLocalizer(lang string) *i18n.Localizer {
	return i18n.NewLocalizer(ls.bundle, lang)
}

func (ls *LocalizationService) GetLocalizerFromRequest(r *http.Request) *i18n.Localizer {
	if ls == nil || ls.bundle == nil {
		// Fallback to a basic bundle if service is not properly initialized
		bundle := i18n.NewBundle(language.English)
		bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
		return i18n.NewLocalizer(bundle, "en")
	}
	acceptLanguage := r.Header.Get("Accept-Language")
	return i18n.NewLocalizer(ls.bundle, acceptLanguage, "en")
}

// SimpleLocalizer wraps i18n.Localizer with a more convenient API
type SimpleLocalizer struct {
	Localizer *i18n.Localizer
}

// T provides a concise way to localize messages
func (sl *SimpleLocalizer) T(messageID string) string {
	return sl.Localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: messageID})
}

// GetLocalizer creates a localizer based on the request's Accept-Language header
func GetLocalizer(r *http.Request) *SimpleLocalizer {
	localizer := NewLocalizationService().GetLocalizerFromRequest(r)
	return &SimpleLocalizer{Localizer: localizer}
}
