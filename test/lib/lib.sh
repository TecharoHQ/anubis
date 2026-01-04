REPO_ROOT=$(git rev-parse --show-toplevel)
(cd $REPO_ROOT && go install ./utils/cmd/...)

mkdir -p pki
echo '*' >>./pki/.gitignore

function cleanup() {
	set +e

	pkill -P $$

	if [ -f "docker-compose.yaml" ]; then
		docker compose down -t 1 || :
		docker compose rm -f || :
	fi
}

trap cleanup EXIT SIGINT

function build_anubis_ko() {
	(
		cd $REPO_ROOT && npm ci && npm run assets
	)
	(
		cd $REPO_ROOT &&
			VERSION=devel ko build \
				--platform=all \
				--base-import-paths \
				--tags="latest" \
				--image-user=1000 \
				--image-annotation="" \
				--image-label="" \
				./cmd/anubis \
				--local
	)
}

function mint_cert() {
	if [ "$#" -ne 1 ]; then
		echo "Usage: mint_cert <domain.name>"
	fi

	domainName="$1"

	# If the transient local TLS certificate doesn't exist, mint a new one
	if [ ! -f "./pki/${domainName}/cert.pem" ]; then
		# Subshell to contain the directory change
		(
			cd ./pki &&
				mkdir -p "${domainName}" &&
				go tool minica -domains "${domainName}" &&
				cd "${domainName}" &&
				chmod 666 *
		)
	fi
}
