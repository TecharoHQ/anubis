---
title: Why is Anubis showing up on a website?
---

You are seeing Anubis because the administrator of that website has set up [Anubis](https://github.com/TecharoHQ/anubis) to protect the server against the scourge of [AI companies aggressively scraping websites](https://thelibre.news/foss-infrastructure-is-under-attack-by-ai-companies/). This can and does cause downtime for the websites, which makes their resources inaccessible for everyone.

Anubis is a compromise. Anubis uses a [proof-of-work](/docs/design/why-proof-of-work) scheme in the vein of [Hashcash](https://en.wikipedia.org/wiki/Hashcash), a proposed proof-of-work scheme for reducing email spam. The idea is that at individual scales the additional load is ignorable, but at mass scraper levels it adds up and makes scraping much more expensive.

Ultimately, this is a hack whose real purpose is to give a "good enough" placeholder solution so that more time can be spent on fingerprinting and identifying headless browsers (EG: via how they do font rendering) so that the challenge proof of work page doesn't need to be presented to users that are much more likely to be legitimate.
