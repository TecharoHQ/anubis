import processFast from "./proof-of-work.mjs";
import processSlow from "./proof-of-work-slow.mjs";
import { testVideo } from "./video.mjs";

const algorithms = {
  "fast": processFast,
  "slow": processSlow,
}

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

(async () => {
  const status = document.getElementById('status');
  const image = document.getElementById('image');
  const title = document.getElementById('title');
  const progress = document.getElementById('progress');
  const anubisVersion = JSON.parse(document.getElementById('anubis_version').textContent);

  // const testarea = document.getElementById('testarea');

  // const videoWorks = await testVideo(testarea);
  // console.log(`videoWorks: ${videoWorks}`);

  // if (!videoWorks) {
  //   title.innerHTML = "Oh no!";
  //   status.innerHTML = "Checks failed. Please check your browser's settings and try again.";
  //   image.src = imageURL("sad");
  //   progress.style.display = "none";
  //   return;
  // }

  status.innerHTML = 'Calculating...';

  const { challenge, rules } = await fetch("/.within.website/x/cmd/anubis/api/make-challenge", { method: "POST" })
    .then(r => {
      if (!r.ok) {
        throw new Error("Failed to fetch config");
      }
      return r.json();
    })
    .catch(err => {
      title.innerHTML = "Oh no!";
      status.innerHTML = `Failed to fetch config: ${err.message}`;
      image.src = imageURL("sad", anubisVersion);
      progress.style.display = "none";
      throw err;
    });

  const process = algorithms[rules.algorithm];
  if (!process) {
    title.innerHTML = "Oh no!";
    status.innerHTML = `Failed to resolve check algorithm. You may want to reload the page.`;
    image.src = imageURL("sad", anubisVersion);
    progress.style.display = "none";
    return;
  }

  status.innerHTML = `Calculating...<br/>Difficulty: ${rules.report_as}, Speed: `;
  const rateText = document.createTextNode("0kH/s");
  status.appendChild(rateText);

  let lastSpeedUpdate = 0;
  let showingApology = false;
  const likelihood = Math.pow(16, -rules.report_as);

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
        rateText.data = `${(iters / delta).toFixed(3)}kH/s`;
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
  image.src = imageURL("happy", anubisVersion);
  progress.style.display = "none";

  setTimeout(() => {
    const redir = window.location.href;
    window.location.href = u("/.within.website/x/cmd/anubis/api/pass-challenge", { response: hash, nonce, redir, elapsedTime: t1 - t0 });
  }, 250);
})();