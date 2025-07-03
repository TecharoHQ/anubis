package localization

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func TestLocalizationService(t *testing.T) {
	service := NewLocalizationService()

	t.Run("English localization", func(t *testing.T) {
		localizer := service.GetLocalizer("en")
		result := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "loading"})
		if result != "Loading..." {
			t.Errorf("Expected 'Loading...', got '%s'", result)
		}
	})

	t.Run("French localization", func(t *testing.T) {
		localizer := service.GetLocalizer("fr")
		result := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "loading"})
		if result != "Chargement..." {
			t.Errorf("Expected 'Chargement...', got '%s'", result)
		}
	})

	t.Run("German localization", func(t *testing.T) {
		localizer := service.GetLocalizer("de")
		result := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "loading"})
		if result != "Ladevorgang..." {
			t.Errorf("Expected 'Ladevorgang...', got '%s'", result)
		}
	})

	t.Run("Turkish localization", func(t *testing.T) {
		localizer := service.GetLocalizer("tr")
		result := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "loading"})
		if result != "Yükleniyor..." {
			t.Errorf("Expected 'Yükleniyor...', got '%s'", result)
		}
	})

	t.Run("All required keys exist in English", func(t *testing.T) {
		localizer := service.GetLocalizer("en")
		requiredKeys := []string{
			"loading", "why_am_i_seeing", "protected_by", "made_with",
			"mascot_design", "try_again", "go_home", "javascript_required",
		}

		for _, key := range requiredKeys {
			result := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: key})
			if result == "" {
				t.Errorf("Key '%s' returned empty string", key)
			}
		}
	})

	t.Run("All required keys exist in French", func(t *testing.T) {
		localizer := service.GetLocalizer("fr")
		requiredKeys := []string{
			"loading", "why_am_i_seeing", "protected_by", "made_with",
			"mascot_design", "try_again", "go_home", "javascript_required",
		}

		for _, key := range requiredKeys {
			result := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: key})
			if result == "" {
				t.Errorf("Key '%s' returned empty string", key)
			}
		}
	})

	t.Run("All required keys exist in Turkish", func(t *testing.T) {
		localizer := service.GetLocalizer("tr")
		requiredKeys := []string{
			"loading", "why_am_i_seeing", "protected_by", "made_with",
			"mascot_design", "try_again", "go_home", "javascript_required",
		}

		for _, key := range requiredKeys {
			result := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: key})
			if result == "" {
				t.Errorf("Key '%s' returned empty string", key)
			}
		}
	})
}

type manifest struct {
	SupportedLanguages []string `json:"supported_languages"`
}

func loadManifest(t *testing.T) manifest {
	t.Helper()

	fin, err := localeFS.Open("locales/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	defer fin.Close()

	var result manifest
	if err := json.NewDecoder(fin).Decode(&result); err != nil {
		t.Fatal(err)
	}

	return result
}

func TestComprehensiveTranslations(t *testing.T) {
	service := NewLocalizationService()

	var translations = map[string]any{}
	fin, err := localeFS.Open("locales/en.json")
	if err != nil {
		t.Fatal(err)
	}
	defer fin.Close()

	if err := json.NewDecoder(fin).Decode(&translations); err != nil {
		t.Fatal(err)
	}

	var keys []string
	for k := range translations {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, lang := range loadManifest(t).SupportedLanguages {
		t.Run(lang, func(t *testing.T) {
			loc := service.GetLocalizer(lang)
			sl := SimpleLocalizer{Localizer: loc}
			for _, key := range keys {
				t.Run(key, func(t *testing.T) {
					if result := sl.T(key); result == "" {
						t.Error("key not defined")
					}
				})
			}
		})
	}
}
