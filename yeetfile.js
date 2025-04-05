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