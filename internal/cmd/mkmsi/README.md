# mkmsi

Converts a yeet-generated .zip file to an MSI using [msitools](https://gitlab.gnome.org/GNOME/msitools).

It needs `wixl`, `wixl-heat`, `msibuild` and `msiinfo` on PATH. Homebrew ships
them all in `msitools`. Debian and Ubuntu split them: `msibuild` and `msiinfo`
are in `msitools`, but `wixl` and `wixl-heat` are in the separate `wixl`
package. Install both.

> [!NOTE]
> This code was generated using Claude Opus 5 as a correctness fuzzer to ensure that this output is valid, works on Windows systems, and generally behaves like a well-behaved Windows installer. Due to facts and circumstances beyond my control with how these tools and the MSI format works, it is not currently possible to make these MSI builds reproducible. I tried.

This is intended to be run by `yeet` at package build time:

```js
// Build MSI installers from the Windows zips that were just built.
packages
  .flat()
  .filter((pkg) => pkg.includes("windows"))
  .filter((pkg) => pkg.endsWith(".zip"))
  .forEach((zip) => {
    const msiPath = zip.replace(/\.zip$/, ".msi");
    $`go run ./internal/cmd/mkmsi --zip ${zip} --out ${msiPath}`;
  });
```
