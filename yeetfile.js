$`npm run assets`;

const methods = [
  // [goos, goarch, [build, methods]]
  ["freebsd", "amd64", [tarball]],
  ["freebsd", "arm64", [tarball]],
  ["linux", "amd64", [deb, rpm, tarball]],
  ["linux", "arm64", [deb, rpm, tarball]],
  ["linux", "ppc64le", [deb, rpm, tarball]],
  ["linux", "riscv64", [deb, rpm, tarball]],
  ["windows", "amd64", [tarball]],
  ["windows", "arm64", [tarball]],
]

const packages = methods.map(([goos, goarch, methods]) => {
  return methods.map(method => {
    const exe = goos == "windows" ? ".exe" : "";

    method.build({
      name: "anubis",
      description:
        "Anubis weighs the souls of incoming HTTP requests and uses a sha256 proof-of-work challenge in order to protect upstream resources from scraper bots.",
      homepage: "https://anubis.techaro.lol",
      license: "MIT",
      platform: goos,
      goarch,

      documentation: {
        "./README.md": "README.md",
        "./LICENSE": "LICENSE",
        "./data/botPolicies.yaml": "botPolicies.yaml",
      },

      build: ({ bin, etc, systemd, doc }) => {
        $`go build -trimpath -o ${bin}/anubis${exe} -ldflags '-s -w -extldflags "-static"' ./cmd/anubis`;
        $`go build -trimpath -o ${bin}/anubis-robots2policy${exe} -ldflags '-s -w -extldflags "-static"' ./cmd/robots2policy`;

        if (goos == "linux") {
          file.install("./run/anubis@.service", `${systemd}/anubis@.service`);
          file.install("./run/default.env", `${etc}/default.env`);
        }

        if (goos == "linux" && method.name == "tarball") {
          $`mkdir -p ${etc}/openrc`
          const openrc = `${etc}/openrc`
          file.install("./run/openrc/anubis.confd", `${openrc}/anubis.confd`);
          file.install("./run/openrc/anubis.initd", `${openrc}/anubis.initd`);
        }

        if (goos == "freebsd") {
          $`mkdir -p ${etc}/rc`;
          const rc = `${etc}/rc`;
          file.install("./run/anubis.freebsd", `${rc}/anubis`);
        }

        $`mkdir -p ${doc}/docs`;
        $`cp -a docs/docs ${doc}`;
        $`find ${doc} -name _category_.json -delete`;
        $`mkdir -p ${doc}/data`;
        $`cp -a data/apps ${doc}/data/apps`;
        $`cp -a data/bots ${doc}/data/bots`;
        $`cp -a data/clients ${doc}/data/clients`;
        $`cp -a data/common ${doc}/data/common`;
        $`cp -a data/crawlers ${doc}/data/crawlers`;
        $`cp -a data/meta ${doc}/data/meta`;
      },
    });
  });
});



// NOTE(Xe): Fixes #217. This is a "half baked" tarball that includes the harder
// parts for deterministic distros already done. Distributions like NixOS, Gentoo
// and *BSD ports have a difficult time fitting the square peg of their dependency
// model into the bazaar of round holes that various modern languages use. Needless
// to say, this makes adoption easier.
tarball.build({
  name: "anubis-src-vendor",
  license: "MIT",
  // XXX(Xe): This is needed otherwise go will be very sad.
  platform: yeet.goos,
  goarch: yeet.goarch,

  build: ({ out }) => {
    // prepare clean checkout in $out
    $`git archive --format=tar HEAD | tar xC ${out}`;
    // vendor Go dependencies
    $`cd ${out} && go mod vendor`;
    // write VERSION file
    $`echo ${git.tag()} > ${out}/VERSION`;
  },

  mkFilename: ({ name, version }) => `${name}-${version}`,
});

tarball.build({
  name: "anubis-src-vendor-npm",
  license: "MIT",
  // XXX(Xe): This is needed otherwise go will be very sad.
  platform: yeet.goos,
  goarch: yeet.goarch,

  build: ({ out }) => {
    // prepare clean checkout in $out
    $`git archive --format=tar HEAD | tar xC ${out}`;
    // vendor Go dependencies
    $`cd ${out} && go mod vendor`;
    // build NPM-bound dependencies
    $`cd ${out} && npm ci && npm run assets && rm -rf node_modules`;
    // write VERSION file
    $`echo ${git.tag()} > ${out}/VERSION`;
  },

  mkFilename: ({ name, version }) => `${name}-${version}`,
});
