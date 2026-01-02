import { render, h, Fragment } from "preact";
import { useState, useEffect, useRef, useCallback } from "preact/hooks";
import algorithms from "./algorithms";

/** @jsx h */
/** @jsxFrag Fragment */

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
  return JSON.parse(elem.textContent!);
};

const imageURL = (mood: string, cacheBuster: string, basePrefix: string) =>
  u(`${basePrefix}/.within.website/x/cmd/anubis/static/img/${mood}.webp`, {
    cacheBuster,
  });

interface ChallengeData {
  challenge: {
    id: string;
    randomData: string;
  };
  rules: {
    algorithm: string;
    difficulty: number;
  };
}

interface Translations {
  [key: string]: string;
}

type AppState =
  | { type: "loading" }
  | { type: "missing_feature"; name: string; message: string }
  | { type: "challenge_error"; message: string }
  | { type: "calculating"; difficulty: number }
  | { type: "success"; userReadDetails: boolean; onContinue: () => void }
  | { type: "error"; message: string };

const getAvailableLanguages = async (basePrefix: string): Promise<string[]> => {
  try {
    const response = await fetch(
      `${basePrefix}/.within.website/x/cmd/anubis/static/locales/manifest.json`
    );
    if (response.ok) {
      const manifest = await response.json();
      return manifest.supportedLanguages || ["en"];
    }
  } catch {
    // Fall through to default
  }
  return ["en"];
};

const loadTranslations = async (
  lang: string,
  basePrefix: string
): Promise<Translations> => {
  try {
    const response = await fetch(
      `${basePrefix}/.within.website/x/cmd/anubis/static/locales/${lang}.json`
    );
    return await response.json();
  } catch {
    if (lang !== "en") {
      return await loadTranslations("en", basePrefix);
    }
    throw new Error("Failed to load translations");
  }
};

const getRedirectUrl = (): string | null => {
  const publicUrl = j("anubis_public_url");
  if (publicUrl && window.location.href.startsWith(publicUrl)) {
    const urlParams = new URLSearchParams(window.location.search);
    return urlParams.get("redir");
  }
  return window.location.href;
};

const ProgressBar = ({
  progress,
  style,
}: {
  progress: number;
  style?: h.JSX.CSSProperties;
}) => (
  <div
    id="progress"
    role="progressbar"
    aria-valuenow={progress}
    style={style}
  >
    <div class="bar-inner" style={{ width: `${progress}%` }}></div>
  </div>
);

const ContinueButton = ({ onClick }: { onClick: () => void }) => (
  <div
    id="progress"
    role="button"
    onClick={onClick}
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
    }}
  />
);

const App = () => {
  const [state, setState] = useState<AppState>({ type: "loading" });
  const [translations, setTranslations] = useState<Translations>({});
  const [rateText, setRateText] = useState("0kH/s");
  const [progress, setProgress] = useState(0);
  const [showApology, setShowApology] = useState(false);
  const [continueText, setContinueText] = useState("");
  const userReadDetailsRef = useRef(false);

  const anubisVersion = j("anubis_version") || "";
  const basePrefix = j("anubis_base_prefix") || "";

  const t = useCallback(
    (key: string) => translations[`js_${key}`] || translations[key] || key,
    [translations]
  );

  const getImageSrc = useCallback(
    (mood: string) => imageURL(mood, anubisVersion, basePrefix),
    [anubisVersion, basePrefix]
  );

  // Initialize and run challenge
  useEffect(() => {
    const init = async () => {
      // Load translations
      const lang = document.documentElement.lang || "en";
      const trans = await loadTranslations(lang, basePrefix);
      setTranslations(trans);

      const tFunc = (key: string) => trans[`js_${key}`] || trans[key] || key;

      // Check dependencies
      const dependencies = [
        {
          name: "Web Workers",
          msg: tFunc("web_workers_error"),
          value: window.Worker,
        },
        {
          name: "Cookies",
          msg: tFunc("cookies_error"),
          value: navigator.cookieEnabled,
        },
      ];

      for (const { value, name, msg } of dependencies) {
        if (!value) {
          setState({ type: "missing_feature", name, message: msg });
          return;
        }
      }

      // Get challenge data
      const challengeData: ChallengeData | null = j("anubis_challenge");
      if (!challengeData) {
        setState({
          type: "challenge_error",
          message: tFunc("challenge_error_msg"),
        });
        return;
      }

      const { challenge, rules } = challengeData;
      const process = algorithms[rules.algorithm as keyof typeof algorithms];

      if (!process) {
        setState({
          type: "challenge_error",
          message: tFunc("challenge_error_msg"),
        });
        return;
      }

      setState({ type: "calculating", difficulty: rules.difficulty });
      setContinueText(tFunc("finished_reading"));

      // Set up details toggle listener
      const details = document.querySelector("details");
      if (details) {
        details.addEventListener("toggle", () => {
          if (details.open) {
            userReadDetailsRef.current = true;
          }
        });
      }

      // Run the challenge
      const likelihood = Math.pow(16, -rules.difficulty);
      let lastSpeedUpdate = 0;
      const t0 = Date.now();

      try {
        const { hash, nonce } = await process(
          { basePrefix, version: anubisVersion },
          challenge.randomData,
          rules.difficulty,
          null,
          (iters: number) => {
            const delta = Date.now() - t0;
            if (delta - lastSpeedUpdate > 1000) {
              lastSpeedUpdate = delta;
              setRateText(`${(iters / delta).toFixed(3)}kH/s`);
            }

            const probability = Math.pow(1 - likelihood, iters);
            const distance = (1 - Math.pow(probability, 2)) * 100;
            setProgress(distance);

            if (probability < 0.1) {
              setShowApology(true);
            }
          }
        );
        const t1 = Date.now();

        const redirectToChallenge = () => {
          const redir = getRedirectUrl();
          window.location.replace(
            u(
              `${basePrefix}/.within.website/x/cmd/anubis/api/pass-challenge`,
              {
                id: challenge.id,
                response: hash,
                nonce,
                redir,
                elapsedTime: t1 - t0,
              }
            )
          );
        };

        if (userReadDetailsRef.current) {
          setState({
            type: "success",
            userReadDetails: true,
            onContinue: redirectToChallenge,
          });
          setTimeout(redirectToChallenge, 30000);
        } else {
          redirectToChallenge();
        }
      } catch (err: any) {
        setState({
          type: "error",
          message: `${tFunc("calculation_error_msg")} ${err.message}`,
        });
      }
    };

    init();
  }, [basePrefix, anubisVersion]);

  // Render based on state
  if (state.type === "loading") {
    return (
      <>
        <img
          id="image"
          style={{ width: "100%", maxWidth: "256px" }}
          src={getImageSrc("pensive")}
        />
        <p id="status">{t("loading")}</p>
      </>
    );
  }

  if (state.type === "missing_feature") {
    return (
      <>
        <img
          id="image"
          style={{ width: "100%", maxWidth: "256px" }}
          src={getImageSrc("reject")}
        />
        <h1 id="title">
          {t("missing_feature")} {state.name}
        </h1>
        <p id="status">{state.message}</p>
      </>
    );
  }

  if (state.type === "challenge_error") {
    return (
      <>
        <img
          id="image"
          style={{ width: "100%", maxWidth: "256px" }}
          src={getImageSrc("reject")}
        />
        <h1 id="title">{t("challenge_error")}</h1>
        <p id="status">{state.message}</p>
      </>
    );
  }

  if (state.type === "error") {
    return (
      <>
        <img
          id="image"
          style={{ width: "100%", maxWidth: "256px" }}
          src={getImageSrc("reject")}
        />
        <h1 id="title">{t("calculation_error")}</h1>
        <p id="status">{state.message}</p>
      </>
    );
  }

  if (state.type === "success" && state.userReadDetails) {
    return (
      <>
        <img
          id="image"
          style={{ width: "100%", maxWidth: "256px" }}
          src={getImageSrc("pensive")}
        />
        <p id="status">{t("calculating")}</p>
        <ContinueButton onClick={state.onContinue} />
        <span>{continueText}</span>
      </>
    );
  }

  // calculating state
  return (
    <>
      <img
        id="image"
        style={{ width: "100%", maxWidth: "256px" }}
        src={getImageSrc("pensive")}
      />
      <p id="status">
        {t("calculating_difficulty")} {(state as any).difficulty},{" "}
        <ProgressBar progress={progress} style={{ display: "inline-block" }} />
        {t("speed")} {rateText}
        {showApology && (
          <>
            <br />
            {t("verification_longer")}
          </>
        )}
      </p>
    </>
  );
};

// Mount the app
const appElement = document.getElementById("app");
if (appElement) {
  render(<App />, appElement);
}
