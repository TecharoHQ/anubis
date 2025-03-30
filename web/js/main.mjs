import processFast from "./proof-of-work.mjs";
import processSlow from "./proof-of-work-slow.mjs";
import { testVideo } from "./video.mjs";

const algorithms = {
  "fast": processFast,
  "slow": processSlow,
};

// from Xeact
const u = (url = "", params = {}) => {
  let result = new URL(url, window.location.href);
  Object.entries(params).forEach((kv) => {
    let [k, v] = kv;
    result.searchParams.set(k, v);
  });
  return result.toString();
};

const imageURL = (mood, cacheBuster) =>
  u(`/.within.website/x/cmd/anubis/static/img/${mood}.webp`, { cacheBuster });

const dependencies = [
  {
    name: "WebCrypto",
    msg: "Your browser doesn't have a functioning web.crypto element. Are you viewing this over a secure context?",
    value: window.crypto,
  },
  {
    name: "Web Workers",
    msg: "Your browser doesn't support web workers (Anubis uses this to avoid freezing your browser). Do you have a plugin like JShelter installed?",
    value: window.Worker,
  },
];

(async () => {
  const status = document.getElementById('status');
  const image = document.getElementById('image');
  const title = document.getElementById('title');
  const spinner = document.getElementById('spinner');
  const anubisVersion = JSON.parse(document.getElementById('anubis_version').textContent);

  const ohNoes = ({
    titleMsg, statusMsg, imageSrc,
  }) => {
    title.innerHTML = titleMsg;
    status.innerHTML = statusMsg;
    image.src = imageSrc;
    spinner.innerHTML = "";
    spinner.style.display = "none";
  };

  if (!window.isSecureContext) {
    ohNoes({
      titleMsg: "Your context is not secure!",
      statusMsg: `Try connecting over HTTPS or let the admin know to set up HTTPS. For more information, see <a href="https://developer.mozilla.org/en-US/docs/Web/Security/Secure_Contexts#when_is_a_context_considered_secure">MDN</a>.`,
      imageSrc: imageURL("sad", anubisVersion),
    });
    return;
  }

  // const testarea = document.getElementById('testarea');

  // const videoWorks = await testVideo(testarea);
  // console.log(`videoWorks: ${videoWorks}`);

  // if (!videoWorks) {
  //   title.innerHTML = "Oh no!";
  //   status.innerHTML = "Checks failed. Please check your browser's settings and try again.";
  //   image.src = imageURL("sad");
  //   spinner.innerHTML = "";
  //   spinner.style.display = "none";
  //   return;
  // }

  status.innerHTML = 'Calculating...';

  for (const val of dependencies) {
    const { value, name, msg } = val;
    if (!value) {
      ohNoes({
        titleMsg: `Missing feature ${name}`,
        statusMsg: msg,
        imageSrc: imageURL("sad", anubisVersion),
      })
    }
  }

  const { challenge, rules } = await fetch("/.within.website/x/cmd/anubis/api/make-challenge", { method: "POST" })
    .then(r => {
      if (!r.ok) {
        throw new Error("Failed to fetch config");
      }
      return r.json();
    })
    .catch(err => {
      ohNoes({
        titleMsg: "Internal error!",
        statusMsg: `Failed to fetch challenge config: ${err.message}`,
        imageSrc: imageURL("sad", anubisVersion),
      });
      throw err;
    });

  const process = algorithms[rules.algorithm];
  if (!process) {
    ohNoes({
      titleMsg: "Challenge error!",
      statusMsg: `Failed to resolve check algorithm. You may want to reload the page.`,
      imageSrc: imageURL("sad", anubisVersion),
    });
    return;
  }

  status.innerHTML = `Calculating...<br/>Difficulty: ${rules.report_as}`;
  spinner.style.display = "block";

  try {
    const t0 = Date.now();
    const { hash, nonce } = await process(challenge, rules.difficulty);
    const t1 = Date.now();
    console.log({ hash, nonce });

    title.innerHTML = "Success!";
    status.innerHTML = `Done! Took ${t1 - t0}ms, ${nonce} iterations`;
    image.src = imageURL("happy", anubisVersion);
    spinner.innerHTML = "";
    spinner.style.display = "none";

    setTimeout(() => {
      const redir = window.location.href;

      window.location.replace(
        u("/.within.website/x/cmd/anubis/api/pass-challenge", {
          response: hash,
          nonce,
          redir,
          elapsedTime: t1 - t0
        }),
      );
    }, 250);
  } catch (err) {
    ohNoes({
      titleMsg: "Calculation error!",
      statusMsg: `Failed to calculate challenge: ${err.message}`,
      imageSrc: imageURL("sad", anubisVersion),
    });
  }
})();