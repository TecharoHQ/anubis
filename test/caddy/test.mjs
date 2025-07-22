async function testWithUserAgent(userAgent) {
  const statusCode =
    await fetch("https://caddy.local.cetacean.club:8443/reqmeta", {
      headers: {
        "User-Agent": userAgent,
      }
    })
      .then(resp => resp.status);
  return statusCode;
}

const codes = {
  Mozilla: await testWithUserAgent("Mozilla"),
  curl: await testWithUserAgent("curl"),
}

const expected = {
  Mozilla: 401,
  curl: 200,
};

console.log("Mozilla:", codes.Mozilla);
console.log("curl:   ", codes.curl);

if (JSON.stringify(codes) !== JSON.stringify(expected)) {
  throw new Error(`wanted ${JSON.stringify(expected)}, got: ${JSON.stringify(codes)}`);
}