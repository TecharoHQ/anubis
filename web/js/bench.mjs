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
  for (const alg of Object.keys(algorithms)) {
    const option = document.createElement("option");
    option.value = option.innerText = alg;
    algorithmSelect.append(option);
  }
};

const benchmarkTrial = async (stats, difficulty, algorithm, signal) => {
  if (!(difficulty >= 1)) {
    throw new Error(`Invalid difficulty: ${difficulty}`);
  }
  const process = algorithms[algorithm];
  if (process == null) {
    throw new Error(`Unknown algorithm: ${algorithm}`);
  }

  const rawChallenge = new Uint8Array(32);
  crypto.getRandomValues(rawChallenge);
  const challenge = Array.from(rawChallenge)
    .map((c) => c.toString(16).padStart(2, "0"))
    .join("");

  const t0 = performance.now();
  const { hash, nonce } = await process(challenge, Number(difficulty), signal);
  const t1 = performance.now();
  console.log({ hash, nonce });

  stats.time += t1 - t0;
  stats.iters += nonce;

  return { time: t1 - t0, nonce };
};

const stats = { time: 0, iters: 0 };
const updateStatus = () => {
  const rate = stats.iters / stats.time;
  if (Number.isFinite(rate)) {
    status.innerText = `Average hashrate: ${rate.toFixed(3)}kH/s`;
  } else {
    status.innerText = "Benchmarking...";
  }
};

const tableCell = (text) => {
  const td = document.createElement("td");
  td.innerText = text;
  td.style.padding = "0 0.25rem";
  return td;
};

const benchmarkLoop = async (controller) => {
  const difficulty = difficultyInput.value;
  const algorithm = algorithmSelect.value;
  updateStatus();

  try {
    const { time, nonce } = await benchmarkTrial(
      stats,
      difficulty,
      algorithm,
      controller.signal,
    );

    const tr = document.createElement("tr");
    tr.style.display = "contents";
    tr.append(tableCell(`${time}ms`), tableCell(nonce));

    // auto-scroll to new rows
    const atBottom =
      results.scrollHeight - results.clientHeight <= results.scrollTop;
    results.append(tr);
    if (atBottom) {
      results.scrollTop = results.scrollHeight - results.clientHeight;
    }

    updateStatus();
  } catch (e) {
    if (e !== false) {
      status.innerText = e;
    }
    return;
  }

  benchmarkLoop(controller);
};

let controller = null;
const reset = () => {
  stats.time = stats.iters = 0;
  results.innerHTML = status.innerText = "";

  if (controller != null) {
    controller.abort();
  }
  controller = new AbortController();

  benchmarkLoop(controller);
};

setupControls();
difficultyInput.addEventListener("change", reset);
algorithmSelect.addEventListener("change", reset);
reset();