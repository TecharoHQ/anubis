package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/facebookgo/flagenv"
)

var (
	dockerAnnotations = flag.String("docker-annotations", os.Getenv("DOCKER_METADATA_OUTPUT_ANNOTATIONS"), "Docker image annotations")
	dockerLabels      = flag.String("docker-labels", os.Getenv("DOCKER_METADATA_OUTPUT_LABELS"), "Docker image labels")
	dockerRepo        = flag.String("docker-repo", "registry.int.xeserv.us/techaro/anubis", "Docker image repository for Anubis")
	dockerTags        = flag.String("docker-tags", os.Getenv("DOCKER_METADATA_OUTPUT_TAGS"), "newline separated docker tags including the registry name")
	githubEventName   = flag.String("github-event-name", "", "GitHub event name")
	pullRequestID     = flag.Int("pull-request-id", -1, "GitHub pull request ID")
	slogLevel         = flag.String("slog-level", "INFO", "logging level (see https://pkg.go.dev/log/slog#hdr-Levels)")
)

func main() {
	flagenv.Parse()
	flag.Parse()

	slog.SetDefault(internal.InitSlog(*slogLevel, os.Stderr))

	koDockerRepo := strings.TrimSuffix(*dockerRepo, "/"+filepath.Base(*dockerRepo))

	if *githubEventName == "pull_request" && *pullRequestID != -1 {
		*dockerRepo = fmt.Sprintf("ttl.sh/techaro/pr-%d/anubis", *pullRequestID)
		*dockerTags = fmt.Sprintf("ttl.sh/techaro/pr-%d/anubis:24h", *pullRequestID)
		koDockerRepo = fmt.Sprintf("ttl.sh/techaro/pr-%d", *pullRequestID)

		slog.Info(
			"Building image for pull request",
			"docker-repo", *dockerRepo,
			"docker-tags", *dockerTags,
			"github-event-name", *githubEventName,
			"pull-request-id", *pullRequestID,
		)
	}

	if strings.Contains(*dockerTags, ",") {
		newTags := strings.Join(strings.Split(*dockerTags, ","), "\n")
		dockerTags = &newTags
	}

	setOutput("docker_image", strings.SplitN(*dockerTags, "\n", 2)[0])

	version, err := run("git describe --tags --always --dirty")
	if err != nil {
		log.Fatal(err)
	}

	commitTimestamp, err := run("git log -1 --format='%ct'")
	if err != nil {
		log.Fatal(err)
	}

	slog.Debug(
		"ko env",
		"KO_DOCKER_REPO", koDockerRepo,
		"SOURCE_DATE_EPOCH", commitTimestamp,
		"VERSION", version,
	)

	os.Setenv("KO_DOCKER_REPO", koDockerRepo)
	os.Setenv("SOURCE_DATE_EPOCH", commitTimestamp)
	os.Setenv("VERSION", version)

	setOutput("version", version)

	if *dockerTags == "" {
		log.Fatal("Must set --docker-tags or DOCKER_METADATA_OUTPUT_TAGS")
	}

	images, err := parseImageList(*dockerTags)
	if err != nil {
		log.Fatalf("can't parse images: %v", err)
	}

	for _, img := range images {
		if img.repository != *dockerRepo {
			slog.Error(
				"Something weird is going on. Wanted docker repo differs from contents of --docker-tags. Did a flag get set incorrectly?",
				"wanted", *dockerRepo,
				"got", img.repository,
				"docker-tags", *dockerTags,
			)
			os.Exit(2)
		}
	}

	var tags []string
	for _, img := range images {
		tags = append(tags, img.tag)
	}

	output, err := run(fmt.Sprintf("ko build --platform=all --base-import-paths --tags=%q --image-user=1000 --image-annotation=%q --image-label=%q ./cmd/anubis | tail -n1", strings.Join(tags, ","), *dockerAnnotations, *dockerLabels))
	if err != nil {
		log.Fatalf("can't run ko build, check stderr: %v", err)
	}

	sp := strings.SplitN(output, "@", 2)

	setOutput("digest", sp[1])
}

type image struct {
	repository string
	tag        string
}

func parseImageList(imageList string) ([]image, error) {
	images := strings.Split(imageList, "\n")
	var result []image
	for _, img := range images {
		if img == "" {
			continue
		}

		// reg.xeiaso.net/techaro/anubis:latest
		// repository: reg.xeiaso.net/techaro/anubis
		// tag:        latest
		index := strings.LastIndex(img, ":")
		result = append(result, image{
			repository: img[:index],
			tag:        img[index+1:],
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no images provided, bad flags")
	}

	return result, nil
}

// run executes a command and returns the trimmed output.
func run(command string) (string, error) {
	bin, err := exec.LookPath("sh")
	if err != nil {
		return "", err
	}
	slog.Debug("running command", "command", command)
	cmd := exec.Command(bin, "-c", command)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func setOutput(key, val string) {
	fmt.Printf("::set-output name=%s::%s\n", key, val)
}
