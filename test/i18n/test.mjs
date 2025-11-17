async function fetchLanguages() {
  return fetch("http://localhost:8923/.within.website/x/cmd/anubis/static/locales/manifest.json")
    .then(resp => {
      if (resp.status !== 200) {
        throw new Error(`wanted status 200, got status: ${resp.status}`);
      }
      return resp;
    })
    .then(resp => resp.json());
}

async function getChallengePage(lang) {
  return fetch("http://localhost:8923/reqmeta", {
    headers: {
      "Accept-Language": lang,
      "User-Agent": "CHALLENGE",
    }
  })
    .then(resp => {
      if (resp.status !== 200) {
        throw new Error(`wanted status 200, got status: ${resp.status}`);
      }
      return resp;
    })
    .then(resp => resp.text());
}

(async () => {
  const languages = await fetchLanguages();
  console.log(languages);

  const { supportedLanguages } = languages;

  if (supportedLanguages.length === 0) {
    throw new Error(`no languages defined`);
  }

  const resultSheet = {};
  let failed = false;

  for (const lang of supportedLanguages) {
    console.log(`getting for ${lang}`);
    const page = await getChallengePage(lang);

    resultSheet[lang] = page.includes(`<html lang="${lang}">`)
  }

  for (const [lang, result] of Object.entries(resultSheet)) {
    if (!result) {
      failed = true;
      console.log(`${lang} did not show up in challenge page`);
    }
  }

  console.log(resultSheet);

  if (failed) {
    throw new Error("i18n smoke test failed");
  }

  process.exit(0);
})();