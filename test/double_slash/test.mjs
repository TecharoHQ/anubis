import { createReadStream } from "fs";
import { createInterface } from "readline";

async function getPage(path) {
  return fetch(`http://localhost:8923${path}`)
    .then((resp) => {
      if (resp.status !== 200) {
        throw new Error(`wanted status 200, got status: ${resp.status}`);
      }
      return resp;
    })
    .then((resp) => resp.text());
}

(async () => {
  const fin = createReadStream("input.txt");
  const rl = createInterface({
    input: fin,
    crlfDelay: Infinity,
  });

  const resultSheet = {};

  let failed = false;

  for await (const line of rl) {
    console.log(line);

    const resp = await getPage(line);
    resultSheet[line] = {
      match: resp.includes(`GET ${line}`),
      line: resp.split("\n")[0],
    };
  }

  for (let [k, v] of Object.entries(resultSheet)) {
    if (!v.match) {
      failed = true;
    }

    console.debug({ path: k, results: v });
  }

  process.exit(failed ? 1 : 0);
})();
