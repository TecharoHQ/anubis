// Smoke test that Anubis correctly proxies gitweb's semicolon-delimited query
// strings to the backend.
//
// gitweb uses ';' as a query separator, e.g. /?p=testing.git;a=summary. If
// Anubis drops or re-encodes the query string (as httputil.ReverseProxy's
// Rewrite mode does by default), gitweb never sees p=testing.git and falls back
// to serving the project-list page at /. In that failure mode the "summary"
// response is byte-for-byte identical to the front page, so this test both
// asserts the summary page loads AND that it differs from the front page.

const BASE = "http://localhost:8005";
const UA = "Mozilla/5.0 (compatible; AnubisGitwebSmoke/1.0)";

async function get(path) {
  const resp = await fetch(`${BASE}${path}`, {
    headers: { "User-Agent": UA },
    redirect: "manual",
  });
  if (resp.status !== 200) {
    throw new Error(`GET ${path}: wanted status 200, got ${resp.status}`);
  }
  return resp.text();
}

const frontPage = await get("/");
const summaryPage = await get("/?p=testing.git;a=summary");

let failed = false;

// The summary page must actually be the repo summary, not the project list
// that gitweb serves when the query string is lost.
if (summaryPage === frontPage) {
  console.log(
    "FAIL: /?p=testing.git;a=summary returned the same body as / " +
      "(the ';'-delimited query was dropped before reaching gitweb)",
  );
  failed = true;
} else {
  console.log("PASS: summary page differs from the front page");
}

// And it must link onward to the commit view, proving gitweb rendered the repo.
const fragment = "/?p=testing.git;a=commit";
if (!summaryPage.includes(fragment)) {
  console.log(`FAIL: summary page did not contain expected fragment: ${fragment}`);
  console.log(summaryPage);
  failed = true;
} else {
  console.log(`PASS: summary page contains ${fragment}`);
}

process.exit(failed ? 1 : 0);
