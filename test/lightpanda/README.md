# Lightpanda

This test makes sure that [Lightpanda](https://github.com/lightpanda-io/browser) clients are blocked by Anubis. It does so by downloading the latest nightly release of Lightpanda and testing it against an actual Anubis install.

## Tale of woe

On August 6th, 2026, administrators of the Haskell gitlab server [noticed a massive flood of requests](https://mailman.haskell.org/archives/list/ghc-devs@haskell.org/thread/AKWY3G76BMMOS6CNV5PZ64PHNWGDK3MM/) with the User-Agent `Lightpanda` in them. This confused me, as they use Anubis and I knew that Anubis had a rule to block Lightpanda [since some time in 2025 before configuration snippet parsing was added](https://github.com/TecharoHQ/anubis/commit/74e11505c6133ee1107811e81a0fd53e1d7876dd). At some point between when that rule was written and this test was made, Lightpanda changed their User-Agent string and this rule no longer worked.

It was at least working on [March 31, 2025](https://github.com/lightpanda-io/browser/issues/500), when a user asked if Lightpanda could do User-Agent spoofing.

This smoke test will let us know when Lightpanda changes their User-Agent string again.
