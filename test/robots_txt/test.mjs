async function getRobotsTxt() {
  return fetch("http://localhost:8923/robots.txt", {
    headers: {
      "Accept-Language": "en",
      "User-Agent": "Mozilla/5.0",
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
  const page = await getRobotsTxt();

  if (page.includes(`<html>`)) {
    console.log(page)
    throw new Error("serve robots.txt smoke test failed");
  }

  console.log("serve-robots-txt serves robots.txt");
  process.exit(0);
})();