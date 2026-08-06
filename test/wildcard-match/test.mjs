async function fetchLanguages() {
  return fetch(
    "http://localhost:8923/.within.website/x/cmd/anubis/static/locales/manifest.json",
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
  const languages = await fetchLanguages();
  console.log("Anubis is running, which means that importing configuration worked");

  process.exit(0);
})();
