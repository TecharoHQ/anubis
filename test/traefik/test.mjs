// Smoke test for https://github.com/TecharoHQ/anubis/issues/1628
//
// Traefik's forwardAuth middleware calls Anubis at the literal path
// /.within.website/x/cmd/anubis/api/check and conveys the original URL in the
// X-Forwarded-Uri header. Path-targeting policy rules must match that header
// (not r.URL.Path), otherwise every request looks like a request to /check.

const BASE = "http://localhost:8080";
const UA = "Mozilla/5.0 (compatible; AnubisTraefikSmoke/1.0)";

const cases = [
  { path: "/", expected: 307, why: "control: no DENY rule, default challenge redirect" },
  { path: "/free", expected: 307, why: "control: no DENY rule, default challenge redirect" },
  { path: "/admin", expected: 403, why: "path_regex must match X-Forwarded-Uri, not 307 or 200" },
  { path: "/admin/users", expected: 403, why: "path_regex must match X-Forwarded-Uri, not 307 or 200" },
  { path: "/api/secret", expected: 403, why: "CEL path must match X-Forwarded-Uri, not 307 or 200" },
];

let failed = false;

for (const c of cases) {
  const resp = await fetch(`${BASE}${c.path}`, {
    headers: { "User-Agent": UA },
    redirect: "manual",
  });
  const ok = resp.status === c.expected;
  console.log(
    `${ok ? "PASS" : "FAIL"}: GET ${c.path} → ${resp.status} (want ${c.expected}: ${c.why})`,
  );
  if (!ok) failed = true;
}

process.exit(failed ? 1 : 0);
