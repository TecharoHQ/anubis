$`npm run assets`;

["amd64", "arm64", "riscv64"].forEach(goarch => {
    [deb, rpm, tarball].forEach(method => method.build({
        name: "anubis",
        description: "Anubis weighs the souls of incoming HTTP requests and uses a sha256 proof-of-work challenge in order to protect upstream resources from scraper bots.",
        homepage: "https://anubis.techaro.lol",
        license: "MIT",
        goarch,

        documentation: {
            "./README.md": "README.md",
            "./LICENSE": "LICENSE",
            "./docs/docs/CHANGELOG.md": "CHANGELOG.md",
            "./docs/docs/admin/policies.md": "policies.md",
            "./docs/docs/admin/native-install.mdx": "native-install.mdx",
            "./data/botPolicies.json": "botPolicies.json",
        },

        build: ({ bin, etc, systemd, out }) => {
            $`go build -o ${bin}/anubis -ldflags '-s -w -extldflags "-static" -X "github.com/TecharoHQ/anubis.Version=${git.tag()}"' ./cmd/anubis`;

            file.install("./run/anubis@.service", `${systemd}/anubis@.service`);
            file.install("./run/default.env", `${etc}/default.env`);
        },
    }));
});

// NOTE(Xe): Fixes #217. This is a "half baked" tarball that includes the harder
// parts for deterministic distros already done. Distributions like NixOS, Gentoo
// and *BSD ports have a difficult time fitting the square peg of their dependency
// model into the bazarr of round holes that various modern languages use. Needless
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
        $`cd ${out} && npm ci && npm run assets && rm -rf node_modules`
        // write VERSION file
        $`echo ${git.tag()} > ${out}/VERSION`;
    },

    mkFilename: ({ name, version }) => `${name}-${version}`,
});