package localization

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func TestLocalizationService(t *testing.T) {
	service := NewLocalizationService()

	loadingStrMap := map[string]string{
		"de":    "Ladevorgang...",
		"en":    "Loading...",
		"es":    "Cargando...",
		"et":    "Laadin...",
		"fr":    "Chargement...",
		"ja":    "ロード中...",
		"is":    "Hleður...",
		"pt-BR": "Carregando...",
		"tr":    "Yükleniyor...",
		"zh-CN": "加载中...",
		"zh-TW": "載入中...",
	}

<<<<<<< HEAD
	for lang, expected := range loadingStrMap {
		t.Run(fmt.Sprintf("%s localization", lang), func(t *testing.T) {
			localizer := service.GetLocalizer(lang)
			result := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "loading"})
			if result != expected {
				t.Errorf("Expected '%s', got '%s'", expected, result)
=======
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

	t.Run("Italian localization", func(t *testing.T) {
		localizer := service.GetLocalizer("it")
		result := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "loading"})
		if result != "Caricamento..." {
			t.Errorf("Expected 'Caricamento...', got '%s'", result)
		}
	})

	t.Run("Turkish localization", func(t *testing.T) {
		localizer := service.GetLocalizer("tr")
		result := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "loading"})
		if result != "Yükleniyor..." {
			t.Errorf("Expected 'Yükleniyor...', got '%s'", result)
		}
	})

	t.Run("Traditional Chinese localization", func(t *testing.T) {
		localizer := service.GetLocalizer("zh-TW")
		result := localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "loading"})
		if result != "載入中..." {
			t.Errorf("Expected '載入中...', got '%s'", result)
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
>>>>>>> 2bf62c1 (feat(localization): Add Italian language translation)
			}
		})
	}

	// Test for requiredKeys localization
	requiredKeys := []string{
		"loading", "why_am_i_seeing", "protected_by", "made_with",
		"mascot_design", "try_again", "go_home", "javascript_required",
	}

	for lang := range loadingStrMap {
		t.Run(fmt.Sprintf("All required keys exist in %s", lang), func(t *testing.T) {
			loc := service.GetLocalizer(lang)
			for _, key := range requiredKeys {
				result := loc.MustLocalize(&i18n.LocalizeConfig{MessageID: key})
				if result == "" {
					t.Errorf("Key '%s' returned empty string", key)
				}
			}
		})
	}
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
			service_lang := sl.GetLang()
			if service_lang != lang {
				t.Error("Localizer language not same as specified")
			}
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
