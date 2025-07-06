import processFast from "./proof-of-work.mjs";
import processSlow from "./proof-of-work-slow.mjs";

const algorithms = {
  fast: processFast,
  slow: processSlow,
};

// from Xeact
const u = (url = "", params = {}) => {
  let result = new URL(url, window.location.href);
  Object.entries(params).forEach(([k, v]) => result.searchParams.set(k, v));
  return result.toString();
};

const imageURL = (mood, cacheBuster, basePrefix) =>
  u(`${basePrefix}/.within.website/x/cmd/anubis/static/img/${mood}.webp`, {
    cacheBuster,
  });

// Detect available languages by loading the manifest
const getAvailableLanguages = async () => {
  const basePrefix = JSON.parse(
    document.getElementById("anubis_base_prefix").textContent,
  );

  try {
    const response = await fetch(`${basePrefix}/.within.website/x/cmd/anubis/static/locales/manifest.json`);
    if (response.ok) {
      const manifest = await response.json();
      return manifest.supportedLanguages || ['en'];
    }
  } catch (error) {
    console.warn('Failed to load language manifest, falling back to default languages');
  }

  // Fallback to default languages if manifest loading fails
  return ['en'];
};

// Detect browser language
const getBrowserLanguage = async () => {
  const lang = navigator.language || navigator.userLanguage;
  const availableLanguages = await getAvailableLanguages();

  // Extract the language code (first 2 characters)
  const langCode = lang.substring(0, 2).toLowerCase();

  // Return the language if supported, or use English
  return availableLanguages.includes(langCode) ? langCode : 'en';
};

// Load translations from JSON files
const loadTranslations = async (lang) => {
  const basePrefix = JSON.parse(
    document.getElementById("anubis_base_prefix").textContent,
  );
  try {
    const response = await fetch(`${basePrefix}/.within.website/x/cmd/anubis/static/locales/${lang}.json`);
    return await response.json();
  } catch (error) {
    console.warn(`Failed to load translations for ${lang}, falling back to English`);
    if (lang !== 'en') {
      return await loadTranslations('en');
    }
    throw error;
  }
};

let translations = {};
let currentLang;

// Initialize translations
const initTranslations = async () => {
  currentLang = await getBrowserLanguage();
  translations = await loadTranslations(currentLang);
};

const t = (key) => translations[`js_${key}`] || translations[key] || key;

(async () => {
  // Initialize translations first
  await initTranslations();

  const dependencies = [
    {
      name: "WebCrypto",
      msg: t('web_crypto_error'),
      value: window.crypto,
    },
    {
      name: "Web Workers",
      msg: t('web_workers_error'),
      value: window.Worker,
    },
    {
      name: "Cookies",
      msg: t('cookies_error'),
      value: navigator.cookieEnabled,
    },
  ];
  const status = document.getElementById("status");
  const image = document.getElementById("image");
  const title = document.getElementById("title");
  const progress = document.getElementById("progress");
  const anubisVersion = JSON.parse(
    document.getElementById("anubis_version").textContent,
  );
  const basePrefix = JSON.parse(
    document.getElementById("anubis_base_prefix").textContent,
  );
  const details = document.querySelector("details");
  let userReadDetails = false;

  if (details) {
    details.addEventListener("toggle", () => {
      if (details.open) {
        userReadDetails = true;
      }
    });
  }

  const ohNoes = ({ titleMsg, statusMsg, imageSrc }) => {
    title.innerHTML = titleMsg;
    status.innerHTML = statusMsg;
    image.src = imageSrc;
    progress.style.display = "none";
  };

  if (!window.isSecureContext) {
    ohNoes({
      titleMsg: t('context_not_secure'),
      statusMsg: t('context_not_secure_msg'),
      imageSrc: imageURL("reject", anubisVersion, basePrefix),
    });
    return;
  }

  status.innerHTML = t('calculating');

  for (const { value, name, msg } of dependencies) {
    if (!value) {
      ohNoes({
        titleMsg: `${t('missing_feature')} ${name}`,
        statusMsg: msg,
        imageSrc: imageURL("reject", anubisVersion, basePrefix),
      });
      return;
    }
  }

  const { challenge, rules } = JSON.parse(
    document.getElementById("anubis_challenge").textContent,
  );

  const process = algorithms[rules.algorithm];
  if (!process) {
    ohNoes({
      titleMsg: t('challenge_error'),
      statusMsg: t('challenge_error_msg'),
      imageSrc: imageURL("reject", anubisVersion, basePrefix),
    });
    return;
  }

  status.innerHTML = `${t('calculating_difficulty')} ${rules.report_as}, `;
  progress.style.display = "inline-block";

  // the whole text, including "Speed:", as a single node, because some browsers
  // (Firefox mobile) present screen readers with each node as a separate piece
  // of text.
  const rateText = document.createTextNode(`${t('speed')} 0kH/s`);
  status.appendChild(rateText);

  let lastSpeedUpdate = 0;
  let showingApology = false;
  const likelihood = Math.pow(16, -rules.report_as);

  try {
    const t0 = Date.now();
    const { hash, nonce } = await process(
      challenge,
      rules.difficulty,
      null,
      (iters) => {
        const delta = Date.now() - t0;
        // only update the speed every second so it's less visually distracting
        if (delta - lastSpeedUpdate > 1000) {
          lastSpeedUpdate = delta;
          rateText.data = `${t('speed')} ${(iters / delta).toFixed(3)}kH/s`;
        }
        // the probability of still being on the page is (1 - likelihood) ^ iters.
        // by definition, half of the time the progress bar only gets to half, so
        // apply a polynomial ease-out function to move faster in the beginning
        // and then slow down as things get increasingly unlikely. quadratic felt
        // the best in testing, but this may need adjustment in the future.

        const probability = Math.pow(1 - likelihood, iters);
        const distance = (1 - Math.pow(probability, 2)) * 100;
        progress["aria-valuenow"] = distance;
        progress.firstElementChild.style.width = `${distance}%`;

        if (probability < 0.1 && !showingApology) {
          status.append(
            document.createElement("br"),
            document.createTextNode(t('verification_longer')),
          );
          showingApology = true;
        }
      },
    );
    const t1 = Date.now();
    console.log({ hash, nonce });

    if (userReadDetails) {
      const container = document.getElementById("progress");

      // Style progress bar as a continue button
      container.style.display = "flex";
      container.style.alignItems = "center";
      container.style.justifyContent = "center";
      container.style.height = "2rem";
      container.style.borderRadius = "1rem";
      container.style.cursor = "pointer";
      container.style.background = "#b16286";
      container.style.color = "white";
      container.style.fontWeight = "bold";
      container.style.outline = "4px solid #b16286";
      container.style.outlineOffset = "2px";
      container.style.width = "min(20rem, 90%)";
      container.style.margin = "1rem auto 2rem";
      container.innerHTML = t('finished_reading');

      function onDetailsExpand() {
        const redir = window.location.href;
        window.location.replace(
          u(`${basePrefix}/.within.website/x/cmd/anubis/api/pass-challenge`, {
            response: hash,
            nonce,
            redir,
            elapsedTime: t1 - t0,
          }),
        );
      }

      container.onclick = onDetailsExpand;
      setTimeout(onDetailsExpand, 30000);
    } else {
      const redir = window.location.href;
      window.location.replace(
        u(`${basePrefix}/.within.website/x/cmd/anubis/api/pass-challenge`, {
          response: hash,
          nonce,
          redir,
          elapsedTime: t1 - t0,
        }),
      );
    }
  } catch (err) {
    ohNoes({
      titleMsg: t('calculation_error'),
      statusMsg: `${t('calculation_error_msg')} ${err.message}`,
      imageSrc: imageURL("reject", anubisVersion, basePrefix),
    });
  }
})();
