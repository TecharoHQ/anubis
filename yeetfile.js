$`npm run assets`;

[
    "amd64",
    "arm64",
    "ppc64le",
    "riscv64",
].forEach(goarch => {
    [deb, rpm, tarball].forEach(method => method.build({
        name: "anubis",
        description: "Anubis weighs the souls of incoming HTTP requests and uses a sha256 proof-of-work challenge in order to protect upstream resources from scraper bots.",
        homepage: "https://anubis.techaro.lol",
        license: "MIT",
        goarch,

        documentation: {
            "./README.md": "README.md",
            "./LICENSE": "LICENSE",
            "./data/botPolicies.json": "botPolicies.json",
            "./data/botPolicies.yaml": "botPolicies.yaml",
        },

        build: ({ bin, etc, systemd, doc }) => {
            $`go build -o ${bin}/anubis -ldflags '-s -w -extldflags "-static" -X "github.com/TecharoHQ/anubis.Version=${git.tag()}"' ./cmd/anubis`;

            file.install("./run/anubis@.service", `${systemd}/anubis@.service`);
            file.install("./run/default.env", `${etc}/default.env`);

            $`mkdir -p ${doc}/docs`
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
    }));
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
        $`cd ${out} && npm ci && npm run assets && rm -rf node_modules`
        // write VERSION file
        $`echo ${git.tag()} > ${out}/VERSION`;
    },

    mkFilename: ({ name, version }) => `${name}-${version}`,
});