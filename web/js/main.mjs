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
  {
    name: "Cookies",
    msg: "Your browser doesn't store cookies. Anubis uses cookies to determine which clients have passed challenges by storing a signed token in a cookie. Please enable storing cookies for this domain. The names of the cookies Anubis stores may vary without notice. Cookie names and values are not part of the public API.",
    value: navigator.cookieEnabled,
  },
];

(async () => {
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
      titleMsg: "Your context is not secure!",
      statusMsg: `Try connecting over HTTPS or let the admin know to set up HTTPS. For more information, see <a href="https://developer.mozilla.org/en-US/docs/Web/Security/Secure_Contexts#when_is_a_context_considered_secure">MDN</a>.`,
      imageSrc: imageURL("reject", anubisVersion, basePrefix),
    });
    return;
  }

  status.innerHTML = "Calculating...";

  for (const { value, name, msg } of dependencies) {
    if (!value) {
      ohNoes({
        titleMsg: `Missing feature ${name}`,
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
      titleMsg: "Challenge error!",
      statusMsg: `Failed to resolve check algorithm. You may want to reload the page.`,
      imageSrc: imageURL("reject", anubisVersion, basePrefix),
    });
    return;
  }

  status.innerHTML = `Calculating...<br/>Difficulty: ${rules.report_as}, `;
  progress.style.display = "inline-block";

  // the whole text, including "Speed:", as a single node, because some browsers
  // (Firefox mobile) present screen readers with each node as a separate piece
  // of text.
  const rateText = document.createTextNode("Speed: 0kH/s");
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
          rateText.data = `Speed: ${(iters / delta).toFixed(3)}kH/s`;
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
            document.createTextNode(
              "Verification is taking longer than expected. Please do not refresh the page.",
            ),
          );
          showingApology = true;
        }
      },
    );
    const t1 = Date.now();
    console.log({ hash, nonce });

    title.innerHTML = "Success!";
    status.innerHTML = `Done! Took ${t1 - t0}ms, ${nonce} iterations`;
    image.src = imageURL("happy", anubisVersion, basePrefix);
    progress.style.display = "none";

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
      container.innerHTML = "I've finished reading, continue →";

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
      setTimeout(() => {
        const redir = window.location.href;
        window.location.replace(
          u(`${basePrefix}/.within.website/x/cmd/anubis/api/pass-challenge`, {
            response: hash,
            nonce,
            redir,
            elapsedTime: t1 - t0,
          }),
        );
      }, 250);
    }
  } catch (err) {
    ohNoes({
      titleMsg: "Calculation error!",
      statusMsg: `Failed to calculate challenge: ${err.message}`,
      imageSrc: imageURL("reject", anubisVersion, basePrefix),
    });
  }
})();
