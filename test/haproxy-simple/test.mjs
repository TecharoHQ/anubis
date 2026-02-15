#!/usr/bin/env node

async function main() {
  console.log("Starting HAProxy simple smoke test...");

  console.log("trying to hit backend through haproxy");
  let resp = await fetch(
    "https://localhost:8443",
    {
      headers: {
        "User-Agent": "Anubis testing",
      }
    }
  );

  if (resp.status !== 200) {
    throw new Error(`Expected 200, got ${resp.status}`);
  }
  console.log("Got 200 as expected");

  console.log("trying to get stopped by anubis");
  resp = await fetch(
    "https://localhost:8443",
    {
      headers: {
        "User-Agent": "Mozilla/5.0",
      }
    }
  );

  if (resp.status !== 401) {
    throw new Error(`Expected 401, got ${resp.status}`);
  }
  console.log("Got 401 as expected");

  console.log("All runtime tests passed successfully!");
}

await main();
