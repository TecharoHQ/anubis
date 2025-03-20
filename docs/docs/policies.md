---
sidebar_position: 30
---

# Policy Definitions

Out of the box, Anubis is pretty heavy-handed. It will aggressively challenge everything that might be a browser (usually indicated by having `Mozilla` in its user agent). However, some bots are smart enough to get past the challenge. Some things that look like bots may actually be fine (IE: RSS readers). Some resources need to be visible no matter what. Some resources and remotes are fine to begin with.

Bot policies let you customize the rules that Anubis uses to allow, deny, or challenge incoming requests. Currently you can set policies by the following matches:

- Request path
- User agent string

Here's an example rule that denies [Amazonbot](https://developer.amazon.com/en/amazonbot):

```json
{
  "name": "amazonbot",
  "user_agent_regex": "Amazonbot",
  "action": "DENY"
}
```

When this rule is evaluated, Anubis will check the `User-Agent` string of the request. If it contains `Amazonbot`, Anubis will send an error page to the user saying that access is denied, but in such a way that makes scrapers think they have correctly loaded the webpage.

Right now the only kinds of policies you can write are bot policies. Other forms of policies will be added in the future.

Here is a minimal policy file that will protect against most scraper bots:

```json
{
  "bots": [
    {
      "name": "well-known",
      "path_regex": "^/.well-known/.*$",
      "action": "ALLOW"
    },
    {
      "name": "favicon",
      "path_regex": "^/favicon.ico$",
      "action": "ALLOW"
    },
    {
      "name": "robots-txt",
      "path_regex": "^/robots.txt$",
      "action": "ALLOW"
    },
    {
      "name": "generic-browser",
      "user_agent_regex": "Mozilla",
      "action": "CHALLENGE"
    }
  ]
}
```

This allows requests to [`/.well-known`](https://en.wikipedia.org/wiki/Well-known_URI), `/favicon.ico`, `/robots.txt`, and challenges any request that has the word `Mozilla` in its User-Agent string. The [default policy file](https://github.com/TecharoHQ/anubis/blob/main/cmd/anubis/botPolicies.json) is a bit more cohesive, but this should be more than enough for most users.

If no rules match the request, it is allowed through.

## Writing your own rules

There are three actions that can be returned from a rule:

| Action      | Effects                                                                           |
| :---------- | :-------------------------------------------------------------------------------- |
| `ALLOW`     | Bypass all further checks and send the request to the backend.                    |
| `DENY`      | Deny the request and send back an error message that scrapers think is a success. |
| `CHALLENGE` | Show a challenge page and/or validate that clients have passed a challenge.       |

Name your rules in lower case using kebab-case. Rule names will be exposed in Prometheus metrics.

In case your service needs it for risk calculation reasons, Anubis exposes information about the rules that any requests match using a few headers:

| Header            | Explanation                                          | Example          |
| :---------------- | :--------------------------------------------------- | :--------------- |
| `X-Anubis-Rule`   | The name of the rule that was matched                | `bot/lightpanda` |
| `X-Anubis-Action` | The action that Anubis took in response to that rule | `CHALLENGE`      |
| `X-Anubis-Status` | The status and how strict Anubis was in its checks   | `PASS-FULL`      |

Policy rules are matched using [Go's standard library regular expressions package](https://pkg.go.dev/regexp). You can mess around with the syntax at [regex101.com](https://regex101.com), make sure to select the Golang option.
