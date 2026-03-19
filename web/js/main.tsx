import { render } from "preact";
import { useState, useEffect, useRef } from "preact/hooks";
import algorithms from "./algorithms";

// from Xeact
const u = (url: string = "", params: Record<string, any> = {}) => {
  let result = new URL(url, window.location.href);
  Object.entries(params).forEach(([k, v]) => result.searchParams.set(k, v));
  return result.toString();
};

const j = (id: string): any | null => {
  const elem = document.getElementById(id);
  if (elem === null) {
    return null;
  }

  const text = elem.textContent;
  if (text == null || text.trim() === "") {
    return null;
  }

  return JSON.parse(text);
};

const imageURL = (
  mood: string,
  cacheBuster: string,
  basePrefix: string,
): string =>
  u(`${basePrefix}/.within.website/x/cmd/anubis/static/img/${mood}.webp`, {
    cacheBuster,
  });

// Use the browser language from the HTML lang attribute which is set by the server settings or request headers
const getBrowserLanguage = async () => document.documentElement.lang;

// Load translations from JSON files
const loadTranslations = async (lang: string) => {
  const basePrefix = j("anubis_base_prefix");
  if (basePrefix === null) {
    return;
  }

  try {
    const response = await fetch(
      `${basePrefix}/.within.website/x/cmd/anubis/static/locales/${lang}.json`,
    );
    return await response.json();
  } catch (error) {
    console.warn(
      `Failed to load translations for ${lang}, falling back to English`,
    );
    if (lang !== "en") {
      return await loadTranslations("en");
    }
    throw error;
  }
};

const getRedirectUrl = () => {
  const publicUrl = j("anubis_public_url");
  if (publicUrl === null) {
    return;
  }
  if (publicUrl && window.location.href.startsWith(publicUrl)) {
    const urlParams = new URLSearchParams(window.location.search);
    return urlParams.get("redir");
  }
  return window.location.href;
};

let translations: Record<string, string> = {};

// Initialize translations
const initTranslations = async () => {
  const currentLang = await getBrowserLanguage();
  translations = await loadTranslations(currentLang);
};

const t = (key: string): string =>
  translations[`js_${key}`] || translations[key] || key;

interface AppProps {
  anubisVersion: string;
  basePrefix: string;
}

function App({ anubisVersion, basePrefix }: AppProps) {
  const [phase, setPhase] = useState<
    "loading" | "computing" | "reading" | "error"
  >("loading");

  // Error info
  const [errorTitle, setErrorTitle] = useState("");
  const [errorMessage, setErrorMessage] = useState("");
  const [errorImage, setErrorImage] = useState("");

  // Computing info
  const [difficulty, setDifficulty] = useState(0);
  const [speed, setSpeed] = useState("0kH/s");
  const [progress, setProgress] = useState(0);
  const [showApology, setShowApology] = useState(false);

  // Reading redirect callback
  const redirectFn = useRef<(() => void) | null>(null);
  const detailsRead = useRef(false);

  // Sync <h1 id="title"> when entering error state (it's outside the Preact tree)
  useEffect(() => {
    if (phase === "error") {
      const titleEl = document.getElementById("title");
      if (titleEl) {
        titleEl.textContent = errorTitle;
      }
    }
  }, [phase, errorTitle]);

  // Main initialization
  useEffect(() => {
    const details = document.querySelector("details");
    if (details) {
      details.addEventListener("toggle", () => {
        if (details.open) {
          detailsRead.current = true;
        }
      });
    }

    const showError = (title: string, message: string, imageSrc: string) => {
      setErrorTitle(title);
      setErrorMessage(message);
      setErrorImage(imageSrc);
      setPhase("error");
    };

    const dependencies = [
      {
        name: "Web Workers",
        msg: t("web_workers_error"),
        value: window.Worker,
      },
      {
        name: "Cookies",
        msg: t("cookies_error"),
        value: navigator.cookieEnabled,
      },
    ];

    for (const { value, name, msg } of dependencies) {
      if (!value) {
        showError(
          `${t("missing_feature")} ${name}`,
          msg,
          imageURL("reject", anubisVersion, basePrefix),
        );
        return;
      }
    }

    const challengeData = j("anubis_challenge");
    if (!challengeData) {
      showError(
        t("challenge_error"),
        t("challenge_data_missing"),
        imageURL("reject", anubisVersion, basePrefix),
      );
      return;
    }
    const { challenge, rules } = challengeData;

    const process = algorithms[rules.algorithm];
    if (!process) {
      showError(
        t("challenge_error"),
        t("challenge_error_msg"),
        imageURL("reject", anubisVersion, basePrefix),
      );
      return;
    }

    setPhase("computing");
    setDifficulty(rules.difficulty);

    const likelihood = Math.pow(16, -rules.difficulty);
    let lastSpeedUpdate = 0;
    let apologyShown = false;
    const t0 = Date.now();

    process(
      { basePrefix, version: anubisVersion },
      challenge.randomData,
      rules.difficulty,
      null,
      (iters: number) => {
        const delta = Date.now() - t0;
        // only update the speed every second so it's less visually distracting
        if (delta - lastSpeedUpdate > 1000) {
          lastSpeedUpdate = delta;
          setSpeed(`${(iters / delta).toFixed(3)}kH/s`);
        }
        // the probability of still being on the page is (1 - likelihood) ^ iters.
        // by definition, half of the time the progress bar only gets to half, so
        // apply a polynomial ease-out function to move faster in the beginning
        // and then slow down as things get increasingly unlikely. quadratic felt
        // the best in testing, but this may need adjustment in the future.

        const probability = Math.pow(1 - likelihood, iters);
        const distance = (1 - Math.pow(probability, 2)) * 100;
        setProgress(distance);

        if (probability < 0.1 && !apologyShown) {
          apologyShown = true;
          setShowApology(true);
        }
      },
    )
      .then((result: any) => {
        const t1 = Date.now();
        const { hash, nonce } = result;
        console.log({ hash, nonce });

        const doRedirect = () => {
          const redir = getRedirectUrl();
          window.location.replace(
            u(`${basePrefix}/.within.website/x/cmd/anubis/api/pass-challenge`, {
              id: challenge.id,
              response: hash,
              nonce,
              redir,
              elapsedTime: t1 - t0,
            }),
          );
        };

        if (detailsRead.current) {
          redirectFn.current = doRedirect;
          setPhase("reading");
          setTimeout(doRedirect, 30000);
        } else {
          doRedirect();
        }
      })
      .catch((err: Error) => {
        showError(
          t("calculation_error"),
          `${t("calculation_error_msg")} ${err.message}`,
          imageURL("reject", anubisVersion, basePrefix),
        );
      });
  }, []);

  const pensiveURL = imageURL("pensive", anubisVersion, basePrefix);

  if (phase === "error") {
    return (
      <>
        <img style="width:100%;max-width:256px;" src={errorImage} />
        <p id="status">{errorMessage}</p>
      </>
    );
  }

  if (phase === "loading") {
    return (
      <>
        <img style="width:100%;max-width:256px;" src={pensiveURL} />
        <p id="status">{t("calculating")}</p>
      </>
    );
  }

  // computing or reading
  return (
    <>
      <img style="width:100%;max-width:256px;" src={pensiveURL} />
      <p id="status">
        {`${t("calculating_difficulty")} ${difficulty}, `}
        {`${t("speed")} ${speed}`}
        {showApology && (
          <>
            <br />
            {t("verification_longer")}
          </>
        )}
      </p>
      {phase === "reading" ? (
        <button
          id="progress"
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            height: "2rem",
            borderRadius: "1rem",
            cursor: "pointer",
            background: "#b16286",
            color: "white",
            fontWeight: "bold",
            outline: "4px solid #b16286",
            outlineOffset: "2px",
            width: "min(20rem, 90%)",
            margin: "1rem auto 2rem",
            border: "none",
            fontSize: "inherit",
            fontFamily: "inherit",
          }}
          onClick={() => redirectFn.current?.()}
        >
          {t("finished_reading")}
        </button>
      ) : (
        <div
          id="progress"
          role="progressbar"
          aria-labelledby="status"
          aria-valuenow={progress}
          style={{ display: "inline-block" }}
        >
          <div class="bar-inner" style={{ width: `${progress}%` }}></div>
        </div>
      )}
    </>
  );
}

// Bootstrap: init translations, then mount Preact
(async () => {
  await initTranslations();
  const anubisVersion = j("anubis_version");
  const basePrefix = j("anubis_base_prefix");
  const root = document.getElementById("app");
  if (root) {
    render(<App anubisVersion={anubisVersion} basePrefix={basePrefix} />, root);
  }
})();
