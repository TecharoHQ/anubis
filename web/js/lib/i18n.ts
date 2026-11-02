import { fetchWithBackoff } from "./backoff";
import { j } from "./xeact.mjs";

export type Translator = (key: string) => string;

// Load translations from a JSON file.
//
// This runs before challenge and benchmark UI initialization, so it must never
// throw. Callers decide how to fall back if a catalogue cannot be loaded.
const loadTranslations = async (
  lang: string,
): Promise<Record<string, string>> => {
  const basePrefix = j("anubis_base_prefix");
  if (basePrefix === null) {
    return {};
  }

  try {
    const response = await fetchWithBackoff(
      `${basePrefix}/.within.website/x/cmd/anubis/static/locales/${lang}.json`,
    );
    return (await response.json()) as Record<string, string>;
  } catch (error) {
    console.warn(`Failed to load translations for ${lang}`, error);
    return {};
  }
};

export const loadTranslator = async (
  defaults: Record<string, string> = {},
): Promise<Translator> => {
  const lang = document.documentElement.lang || "en";
  const translationsPromise = loadTranslations(lang);
  const fallbackPromise =
    lang === "en" ? translationsPromise : loadTranslations("en");
  const [translations, fallback] = await Promise.all([
    translationsPromise,
    fallbackPromise,
  ]);

  return (key: string): string =>
    translations[`js_${key}`] ||
    translations[key] ||
    fallback[`js_${key}`] ||
    fallback[key] ||
    defaults[`js_${key}`] ||
    defaults[key] ||
    `unknown translatable string: ${key}`;
};
