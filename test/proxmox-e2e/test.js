try {
  host$`cd ../.. && rm -rf ./var/proxmox-test && go tool yeet --package-dest-dir ./var/proxmox-test --filter 'method == "deb" && goos == "linux" && goarch == "amd64"'`;
  const deb = host$`ls ../../var/proxmox-test/*.deb`.trim().split("\n")[0];
  console.log("package:", deb);
  scp(deb, "/home/ci/anubis.deb");

  $`sudo apt-get -f install /home/ci/anubis.deb`;
  $`sudo cp /etc/anubis/default.env /etc/anubis/web.env`;
  $`sudo cp /usr/share/doc/anubis/botPolicies.yaml /etc/anubis/web.botPolicies.yaml`;
  $`sudo systemctl enable --now anubis@web.service`;
  console.log($`sudo systemctl status anubis@web.service`);
} catch (e) {
  console.log($`journalctl --no-pager`);
  throw e;
}
