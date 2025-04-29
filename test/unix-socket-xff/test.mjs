async function testWithUserAgent(userAgent) {
  const statusCode =
    await fetch("https://relayd.local.cetacean.club:3004/reqmeta", {
      headers: {
        "User-Agent": userAgent,
      }
    })
      .then(resp => resp.status);
  return statusCode;
}

const codes = {
  allow: await testWithUserAgent("ALLOW"),
  challenge: await testWithUserAgent("CHALLENGE"),
  deny: await testWithUserAgent("DENY")
}

const expected = {
  allow: 200,
  challenge: 401,
  deny: 403,
};

console.log("ALLOW:    ", codes.allow);
console.log("CHALLENGE:", codes.challenge);
console.log("DENY:     ", codes.deny);

if (JSON.stringify(codes) !== JSON.stringify(expected)) {
  throw new Error(`wanted ${JSON.stringify(expected)}, got: ${JSON.stringify(codes)}`);
}