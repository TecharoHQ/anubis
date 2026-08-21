async function getChallengePage() {
  return fetch("http://localhost:8923/reqmeta", {
    headers: {
      "Accept-Language": "en",
      "User-Agent": "CHALLENGE",
    },
  })
    .then((resp) => {
      if (resp.status !== 200) {
        throw new Error(`wanted status 200, got status: ${resp.status}`);
      }
      return resp;
    })
    .then((resp) => resp.text());
}

async function getFunFacts() {
  return fetch(
    "http://localhost:8923/.within.website/x/cmd/anubis/static/funfacts/en.json",
  )
    .then((resp) => {
      if (resp.status !== 200) {
        throw new Error(`wanted status 200, got status: ${resp.status}`);
      }
      return resp;
    })
    .then((resp) => resp.json());
}

(async () => {
  // Check 1: the challenge page HTML contains the fun-fact element.
  const page = await getChallengePage();

  if (!page.includes(`<p id="fun-fact"`)) {
    throw new Error("challenge page does not contain the fun-fact element");
  }

  console.log('challenge page contains <p id="fun-fact">');

  // Check 2: the funfacts/en.json static file is served and valid.
  const facts = await getFunFacts();

  if (!Array.isArray(facts)) {
    throw new Error(`funfacts/en.json is not an array`);
  }

  if (facts.length === 0) {
    throw new Error(`funfacts/en.json is empty`);
  }

  for (const fact of facts) {
    if (typeof fact !== "string" || fact.length === 0) {
      throw new Error(`funfacts/en.json contains a non-string or empty entry`);
    }
  }

  console.log(`funfacts/en.json served ${facts.length} facts`);

  process.exit(0);
})();
