package client

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
)

const (
	DockerImageProvider = "docker-image"
	// NOTE: this needs to be consistent with engineDefaultStateDir in internal/mage/engine.go
	DefaultStateDir = "/var/lib/dagger"

	ServicesDNSEnvName    = "_EXPERIMENTAL_DAGGER_SERVICES_DNS"
	DaggerCloudCacheToken = "_EXPERIMENTAL_DAGGER_CACHESERVICE_TOKEN"
	DaggerCloudToken      = "DAGGER_CLOUD_TOKEN"

	// trim image digests to 16 characters to makeoutput more readable
	hashLen             = 16
	containerNamePrefix = "dagger-engine-"
)

// Pull the image and run it with a unique name tied to the pinned
// sha of the image. Remove any other containers leftover from
// previous executions of the engine at different versions (which
// are identified by looking for containers with the prefix
// "dagger-engine-").
func dockerImageProvider(ctx context.Context, runnerHost *url.URL, userAgent string) (string, error) {
	imageRef := runnerHost.Host + runnerHost.Path

	// Get the SHA digest of the image to use as an ID for the container we'll create
	var id string
	fallbackToLeftoverEngine := false
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return "", errors.Wrap(err, "parsing image reference")
	}
	if d, ok := ref.(name.Digest); ok {
		// We already have the digest as part of the image ref
		id = d.DigestStr()
	} else {
		// We only have a tag in the image ref, so resolve it to a digest. The default
		// auth keychain parses the same docker credentials as used by the buildkit
		// session attachable.
		if img, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithUserAgent(userAgent)); err != nil {
			fmt.Fprintf(os.Stderr, "failed to resolve image digest: %v\n", err)
			if strings.Contains(err.Error(), "DENIED") {
				fmt.Fprintf(os.Stderr, "check your docker ghcr creds, it might be incorrect or expired\n")
			}
			fmt.Fprintf(os.Stderr, "falling back to leftover engine\n")
			fallbackToLeftoverEngine = true
		} else {
			id = img.Digest.String()
		}
	}

	// We collect leftover engine anyway since we garbage collect them at the end
	// And check if we are in a fallback case then perform fallback to most recent engine
	leftoverEngines, err := collectLeftoverEngines(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to list containers: %s\n", err)
		leftoverEngines = []string{}
	}
	if fallbackToLeftoverEngine {
		if len(leftoverEngines) == 0 {
			return "", errors.Errorf("no fallback container found")
		}

		// the first leftover engine may not be running, so make sure to start it
		firstEngine := leftoverEngines[0]
		cmd := exec.CommandContext(ctx, "docker", "start", firstEngine)
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", errors.Wrapf(err, "failed to start container: %s", output)
		}

		garbageCollectEngines(ctx, leftoverEngines[1:])

		return "docker-container://" + firstEngine, nil
	}

	_, id, ok := strings.Cut(id, "sha256:")
	if !ok {
		return "", errors.Errorf("invalid image reference %q", imageRef)
	}
	id = id[:hashLen]

	// add DAGGER_CLOUD_TOKEN in backwards compat way.
	// TODO: deprecate in a future release
	cloudToken := DaggerCloudCacheToken
	if _, ok := os.LookupEnv(DaggerCloudToken); ok {
		cloudToken = DaggerCloudToken
	}

	// run the container using that id in the name
	containerName := containerNamePrefix + id

	for i, leftoverEngine := range leftoverEngines {
		// if we already have a container with that name, attempt to start it
		if leftoverEngine == containerName {
			cmd := exec.CommandContext(ctx, "docker", "start", leftoverEngine)
			if output, err := cmd.CombinedOutput(); err != nil {
				return "", errors.Wrapf(err, "failed to start container: %s", output)
			}
			garbageCollectEngines(ctx, append(leftoverEngines[:i], leftoverEngines[i+1:]...))
			return "docker-container://" + containerName, nil
		}
	}

	runArgs := []string{
		"run",
		"--name", containerName,
		"-d",
		"--restart", "always",
		"-e", ServicesDNSEnvName,
		"-e", cloudToken,
		"-v", DefaultStateDir,
		"--privileged",
	}
	runArgs = append(runArgs, imageRef, "--debug")

	if output, err := exec.CommandContext(ctx, "docker", runArgs...).CombinedOutput(); err != nil {
		if !isContainerAlreadyInUseOutput(string(output)) {
			return "", errors.Wrapf(err, "failed to run container: %s", output)
		}
	}

	// garbage collect any other containers with the same name pattern, which
	// we assume to be leftover from previous runs of the engine using an older
	// version
	garbageCollectEngines(ctx, leftoverEngines)

	return "docker-container://" + containerName, nil
}

func garbageCollectEngines(ctx context.Context, engines []string) {
	for _, engine := range engines {
		if engine == "" {
			continue
		}
		if output, err := exec.CommandContext(ctx,
			"docker", "rm", "-fv", engine,
		).CombinedOutput(); err != nil {
			if !strings.Contains(string(output), "already in progress") {
				fmt.Fprintf(os.Stderr, "failed to remove old container %s: %s\n", engine, output)
			}
		}
	}
}

func collectLeftoverEngines(ctx context.Context) ([]string, error) {
	output, err := exec.CommandContext(ctx,
		"docker", "ps",
		"-a",
		"--no-trunc",
		"--filter", "name=^/"+containerNamePrefix,
		"--format", "{{.Names}}",
	).CombinedOutput()
	output = bytes.TrimSpace(output)

	if len(output) == 0 {
		return nil, err
	}

	engineNames := strings.Split(string(output), "\n")
	return engineNames, err
}

func isContainerAlreadyInUseOutput(output string) bool {
	switch {
	// docker cli output
	case strings.Contains(output, "is already in use"):
		return true
	// nerdctl cli output
	case strings.Contains(output, "is already used"):
		return true
	}
	return false
}
