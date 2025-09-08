# Pale Moon CI tests

Pale Moon has exposed [some pretty bad bugs](https://anubis.techaro.lol/blog/release/v1.21.1#fix-event-loop-thrashing-when-solving-a-proof-of-work-challenge) in Anubis. As such, we're running Pale Moon against Anubis in CI to ensure that it keeps working.

This test is a fork of [dtinth/xtigervnc-docker](https://github.com/dtinth/xtigervnc-docker) but focused on Pale Moon.
