package core

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"dagger.io/dagger"
	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/internal/testutil"
	"github.com/moby/buildkit/identity"
	"github.com/stretchr/testify/require"
)

func connect(t *testing.T, opts ...dagger.ClientOpt) (*dagger.Client, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	opts = append([]dagger.ClientOpt{
		dagger.WithLogOutput(newTWriter(t)),
	}, opts...)

	client, err := dagger.Connect(ctx, opts...)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	return client, ctx
}

func newCache(t *testing.T) core.CacheID {
	var res struct {
		CacheVolume struct {
			ID core.CacheID
		}
	}

	err := testutil.Query(`
		query CreateCache($key: String!) {
			cacheVolume(key: $key) {
				id
			}
		}
	`, &res, &testutil.QueryOptions{Variables: map[string]any{
		"key": identity.NewID(),
	}})
	require.NoError(t, err)

	return res.CacheVolume.ID
}

func newDirWithFile(t *testing.T, path, contents string) core.DirectoryID {
	dirRes := struct {
		Directory struct {
			WithNewFile struct {
				ID core.DirectoryID
			}
		}
	}{}

	err := testutil.Query(
		`query Test($path: String!, $contents: String!) {
			directory {
				withNewFile(path: $path, contents: $contents) {
					id
				}
			}
		}`, &dirRes, &testutil.QueryOptions{Variables: map[string]any{
			"path":     path,
			"contents": contents,
		}})
	require.NoError(t, err)

	return dirRes.Directory.WithNewFile.ID
}

func newFile(t *testing.T, path, contents string) core.FileID {
	var secretRes struct {
		Directory struct {
			WithNewFile struct {
				File struct {
					ID core.FileID
				}
			}
		}
	}

	err := testutil.Query(
		`query Test($path: String!, $contents: String!) {
			directory {
				withNewFile(path: $path, contents: $contents) {
					file(path: "some-file") {
						id
					}
				}
			}
		}`, &secretRes, &testutil.QueryOptions{Variables: map[string]any{
			"path":     path,
			"contents": contents,
		}})
	require.NoError(t, err)

	fileID := secretRes.Directory.WithNewFile.File.ID
	require.NotEmpty(t, fileID)

	return fileID
}

const (
	registryHost        = "registry:5000"
	privateRegistryHost = "privateregistry:5000"
)

func registryRef(name string) string {
	return fmt.Sprintf("%s/%s:%s", registryHost, name, identity.NewID())
}

func privateRegistryRef(name string) string {
	return fmt.Sprintf("%s/%s:%s", privateRegistryHost, name, identity.NewID())
}

func ls(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(ents))
	for i, ent := range ents {
		names[i] = ent.Name()
	}
	return names, nil
}

func tarEntries(t *testing.T, path string) []string {
	f, err := os.Open(path)
	require.NoError(t, err)

	entries := []string{}
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err)
		}

		entries = append(entries, hdr.Name)
	}

	return entries
}

func readTarFile(t *testing.T, pathToTar, pathInTar string) []byte {
	f, err := os.Open(pathToTar)
	require.NoError(t, err)

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err)
		}

		if hdr.Name == pathInTar {
			b, err := io.ReadAll(tr)
			require.NoError(t, err)
			return b
		}
	}

	return nil
}

func checkNotDisabled(t *testing.T, env string) { //nolint:unparam
	if os.Getenv(env) == "0" {
		t.Skipf("disabled via %s=0", env)
	}
}

func computeMD5FromReader(reader io.Reader) string {
	h := md5.New()
	io.Copy(h, reader)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func daggerCliPath(t *testing.T) string {
	t.Helper()
	cliPath := os.Getenv("_EXPERIMENTAL_DAGGER_CLI_BIN")
	if cliPath == "" {
		var err error
		cliPath, err = exec.LookPath("dagger")
		require.NoError(t, err)
	}
	if cliPath == "" {
		t.Log("missing _EXPERIMENTAL_DAGGER_CLI_BIN")
		t.FailNow()
	}
	return cliPath
}

func daggerCliFile(t *testing.T, c *dagger.Client) *dagger.File {
	t.Helper()
	return c.Host().File(daggerCliPath(t))
}

const testCLIBinPath = "/bin/dagger"

func CLITestContainer(ctx context.Context, t *testing.T, c *dagger.Client) *DaggerCLIContainer {
	t.Helper()
	ctr := c.Container().From(alpineImage).
		WithMountedFile(testCLIBinPath, daggerCliFile(t, c))

	return &DaggerCLIContainer{
		Container: ctr,
		ctx:       ctx,
		t:         t,
		c:         c,
	}
}

type DaggerCLIContainer struct {
	*dagger.Container
	ctx context.Context
	t   *testing.T
	c   *dagger.Client

	// common
	ProjectArg string

	// "do"
	OutputArg string
	TargetArg string
	UserArgs  map[string]string

	// "project init"
	SDKArg  string
	NameArg string
	RootArg string
}

const cliContainerRepoMntPath = "/src"

func (ctr DaggerCLIContainer) WithLoadedProject(
	projectPath string,
	convertToGitProject bool,
) *DaggerCLIContainer {
	ctr.t.Helper()
	thisRepoPath, err := filepath.Abs("../..")
	require.NoError(ctr.t, err)

	thisRepoDir := ctr.c.Host().Directory(thisRepoPath, dagger.HostDirectoryOpts{
		Include: []string{"core", "sdk", "go.mod", "go.sum"},
	})
	projectArg := filepath.Join(cliContainerRepoMntPath, projectPath)

	baseCtr := ctr.Container
	if convertToGitProject {
		gitSvc, _ := gitService(ctr.ctx, ctr.t, ctr.c, thisRepoDir)
		baseCtr = baseCtr.WithServiceBinding("git", gitSvc)

		endpoint, err := gitSvc.Endpoint(ctr.ctx)
		require.NoError(ctr.t, err)
		projectArg = "git://" + endpoint + "/repo.git" + "?ref=main&protocol=git"
		if projectPath != "" {
			projectArg += "&subpath=" + projectPath
		}
	} else {
		baseCtr = baseCtr.WithMountedDirectory(cliContainerRepoMntPath, thisRepoDir)
	}

	ctr.Container = baseCtr
	ctr.ProjectArg = projectArg
	return &ctr
}

func (ctr DaggerCLIContainer) WithProjectArg(projectArg string) *DaggerCLIContainer {
	ctr.ProjectArg = projectArg
	return &ctr
}

func (ctr DaggerCLIContainer) WithOutputArg(outputArg string) *DaggerCLIContainer {
	ctr.OutputArg = outputArg
	return &ctr
}

func (ctr DaggerCLIContainer) WithTarget(target string) *DaggerCLIContainer {
	ctr.TargetArg = target
	return &ctr
}

func (ctr DaggerCLIContainer) WithUserArg(key, value string) *DaggerCLIContainer {
	if ctr.UserArgs == nil {
		ctr.UserArgs = map[string]string{}
	}
	ctr.UserArgs[key] = value
	return &ctr
}

func (ctr DaggerCLIContainer) WithSDKArg(sdk string) *DaggerCLIContainer {
	ctr.SDKArg = sdk
	return &ctr
}

func (ctr DaggerCLIContainer) WithNameArg(name string) *DaggerCLIContainer {
	ctr.NameArg = name
	return &ctr
}

func (ctr DaggerCLIContainer) CallDo() *DaggerCLIContainer {
	args := []string{testCLIBinPath, "--debug", "do"}
	if ctr.ProjectArg != "" {
		args = append(args, "--project", ctr.ProjectArg)
	}
	if ctr.OutputArg != "" {
		args = append(args, "--output", ctr.OutputArg)
	}
	args = append(args, ctr.TargetArg)
	for k, v := range ctr.UserArgs {
		args = append(args, "--"+k, v)
	}
	ctr.Container = ctr.Container.WithExec(args, dagger.ContainerWithExecOpts{ExperimentalPrivilegedNesting: true})
	return &ctr
}

func (ctr DaggerCLIContainer) CallProject() *DaggerCLIContainer {
	args := []string{testCLIBinPath, "project"}
	if ctr.ProjectArg != "" {
		args = append(args, "--project", ctr.ProjectArg)
	}
	ctr.Container = ctr.WithExec(args, dagger.ContainerWithExecOpts{ExperimentalPrivilegedNesting: true})
	return &ctr
}

func (ctr DaggerCLIContainer) CallProjectInit() *DaggerCLIContainer {
	args := []string{testCLIBinPath, "project", "init"}
	if ctr.ProjectArg != "" {
		args = append(args, "--project", ctr.ProjectArg)
	}
	if ctr.SDKArg != "" {
		args = append(args, "--sdk", ctr.SDKArg)
	}
	if ctr.NameArg != "" {
		args = append(args, "--name", ctr.NameArg)
	}
	if ctr.RootArg != "" {
		args = append(args, "--root", ctr.RootArg)
	}
	ctr.Container = ctr.WithExec(args, dagger.ContainerWithExecOpts{ExperimentalPrivilegedNesting: true})
	return &ctr
}

func goCache(c *dagger.Client) dagger.WithContainerFunc {
	return func(ctr *dagger.Container) *dagger.Container {
		return ctr.
			WithMountedCache("/go/pkg/mod", c.CacheVolume("go-mod")).
			WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
			WithMountedCache("/go/build-cache", c.CacheVolume("go-build")).
			WithEnvVariable("GOCACHE", "/go/build-cache")
	}
}

// tWriter is a writer that writes to testing.T
type tWriter struct {
	t   *testing.T
	buf bytes.Buffer
	mu  sync.Mutex
}

// newTWriter creates a new TWriter
func newTWriter(t *testing.T) *tWriter {
	tw := &tWriter{t: t}
	t.Cleanup(tw.flush)
	return tw
}

// Write writes data to the testing.T
func (tw *tWriter) Write(p []byte) (n int, err error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	tw.t.Helper()

	if n, err = tw.buf.Write(p); err != nil {
		return n, err
	}

	for {
		line, err := tw.buf.ReadBytes('\n')
		if err == io.EOF {
			// If we've reached the end of the buffer, write it back, because it doesn't have a newline
			tw.buf.Write(line)
			break
		}
		if err != nil {
			return n, err
		}

		tw.t.Log(strings.TrimSuffix(string(line), "\n"))
	}
	return n, nil
}

func (tw *tWriter) flush() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.t.Log(tw.buf.String())
}

type safeBuffer struct {
	bu bytes.Buffer
	mu sync.Mutex
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bu.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bu.String()
}
