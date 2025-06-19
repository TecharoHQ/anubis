package localization_test

import (
	"testing"

	"github.com/TecharoHQ/anubis/lib/localization"	
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func TestLocalizationService(t *testing.T) {
	service := localization.NewLocalizationService()

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
}
