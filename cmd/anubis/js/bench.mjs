import processFast from "./proof-of-work.mjs";
import processSlow from "./proof-of-work-slow.mjs";

const defaultDifficulty = 4;
const algorithms = {
  fast: processFast,
  slow: processSlow,
};

const status = document.getElementById("status");
const difficultyInput = document.getElementById("difficulty-input");
const algorithmSelect = document.getElementById("algorithm-select");
const results = document.getElementById("results");

const setupControls = () => {
  difficultyInput.value = defaultDifficulty;
  Object.keys(algorithms).forEach((alg) => {
    const option = document.createElement("option");
    option.value = option.innerText = alg;
    algorithmSelect.append(option);
  });
};

const benchmarkTrial = async (signal) => {
  const difficulty = Number(difficultyInput.value);
  if (!(difficulty >= 1)) {
    throw new Error(`Invalid difficulty: ${difficultyInput.value}`);
  }
  const process = algorithms[algorithmSelect.value];
  if (process == null) {
    throw new Error(`Unknown algorithm: ${algorithmSelect.value}`);
  }

  const rawChallenge = new Uint8Array(32);
  crypto.getRandomValues(rawChallenge);
  const challenge = Array.from(rawChallenge)
    .map((c) => c.toString(16).padStart(2, "0"))
    .join("");

  const t0 = Date.now();
  const { hash, nonce } = await process(challenge, difficulty, signal);
  const t1 = Date.now();
  console.log({ hash, nonce });

  return { difficulty, time: t1 - t0, nonce };
};

const tableCell = (text) => {
  const td = document.createElement("td");
  td.innerText = text;
  td.style.padding = "0 0.25rem";
  return td;
};

let controller = null;
let totalTime = 0;
let totalIters = 0;
const startBenchmark = async (signal) => {
  if (controller != null) {
    controller.abort();
  }
  controller = new AbortController();

  if (totalTime == 0) {
    status.innerText = "Benchmarking...";
  }

  try {
    const { difficulty, time, nonce } = await benchmarkTrial(controller.signal);

    totalTime += time;
    totalIters += nonce;

    const tr = document.createElement("tr");
    tr.style.display = "contents";
    tr.append(tableCell(difficulty), tableCell(`${time}ms`), tableCell(nonce));

    // auto-scroll to new rows
    const atBottom =
      results.scrollHeight - results.clientHeight <= results.scrollTop;
    results.append(tr);
    if (atBottom) {
      results.scrollTop = results.scrollHeight - results.clientHeight;
    }

    status.innerText = `Average hashrate: ${(totalIters / totalTime).toFixed(3)}kH/s`;
  } catch (e) {
    if (e !== false) {
      status.innerText = e;
    }
    controller = null;
    return;
  }

  startBenchmark();
};

setupControls();
difficultyInput.addEventListener("change", startBenchmark);
algorithmSelect.addEventListener("change", () => {
  totalTime = 0;
  totalIters = 0;
  results.innerHTML = status.innerText = "";
  startBenchmark();
});
startBenchmark();
