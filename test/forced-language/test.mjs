async function getChallengePage() {
  return fetch("http://localhost:8923/reqmeta", {
    headers: {
      "Accept-Language": "en",
      "User-Agent": "CHALLENGE",
    }
  })
    .then(resp => {
      if (resp.status !== 200) {
        throw new Error(`wanted status 200, got status: ${resp.status}`);
      }
      return resp;
    })
    .then(resp => resp.text());
}

(async () => {
  const page = await getChallengePage();

  if (!page.includes(`<html lang="de">`)) {
    console.log(page)
    throw new Error("force language smoke test failed");
  }

  console.log("FORCED_LANGUAGE=de caused a page to be rendered in german");
  process.exit(0);
})();