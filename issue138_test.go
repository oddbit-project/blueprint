package blueprint

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// providerDirs138 returns the directories under provider/ that are Go modules, and those
// that are not (core packages). The published core module carries no provider modules, so
// there is nothing to check there; any tree that has them, or is a checkout, is checked
func providerDirs138(t *testing.T) (modules, corePackages []string) {
	entries, err := os.ReadDir("provider")
	require.NoError(t, err)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join("provider", e.Name(), "go.mod")); err == nil {
			modules = append(modules, e.Name())
		} else {
			corePackages = append(corePackages, e.Name())
		}
	}
	if len(modules) == 0 {
		if _, err := os.Stat(".git"); err != nil {
			t.Skip("the published module has no provider modules")
		}
		t.Fatal("no provider/*/go.mod in a repository checkout")
	}
	sort.Strings(modules)
	return modules, corePackages
}

var providerLoop138 = regexp.MustCompile(`for provider in ([^;]*); do`)

// make138 returns a dry run of the Makefile targets in dir (the package directory when
// empty). A parent make's command line (make test PROVIDERS=…) must not reach it, so its
// flag variables are dropped from the environment
func make138(t *testing.T, dir string, args ...string) *exec.Cmd {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make is not installed")
	}
	// the Makefile uses GNU make functions; BSD make cannot parse it
	if version, _ := exec.Command("make", "--version").CombinedOutput(); !strings.Contains(string(version), "GNU Make") {
		t.Skip("make is not GNU make")
	}
	makefile, err := filepath.Abs("Makefile")
	require.NoError(t, err)
	// read by the test itself, so that the test cache sees a change to the Makefile
	_, err = os.ReadFile(makefile)
	require.NoError(t, err)

	cmd := exec.Command("make", append([]string{"-n", "-f", makefile}, args...)...)
	cmd.Dir = dir
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "MAKEFLAGS", "MFLAGS", "MAKEOVERRIDES", "GNUMAKEFLAGS":
		default:
			cmd.Env = append(cmd.Env, kv)
		}
	}
	return cmd
}

// makeLoops138 dry-runs a Makefile target and returns the provider list of each
// "for provider in …" loop it would run, sorted
func makeLoops138(t *testing.T, args ...string) [][]string {
	out, err := make138(t, "", args...).CombinedOutput()
	require.NoError(t, err, string(out))
	loops := make([][]string, 0)
	for _, m := range providerLoop138.FindAllStringSubmatch(string(out), -1) {
		names := strings.Fields(m[1])
		sort.Strings(names)
		loops = append(loops, names)
	}
	return loops
}

// Issue #138 claims 1, 2 and 4: make test runs the tests of every provider module, which
// includes provider/prometheus and provider/jwtprovider
func TestIssue138_MakeTestCoversEveryProviderModule(t *testing.T) {
	modules, _ := providerDirs138(t)
	for _, target := range []string{"test", "test-providers"} {
		t.Run(target, func(t *testing.T) {
			loops := makeLoops138(t, target)
			require.Len(t, loops, 1)
			assert.Equal(t, modules, loops[0], "the tested providers are not the provider/*/go.mod modules")
		})
	}
}

// Issue #138 claim 3: build-all, tidy-all and sbom cover every provider module
func TestIssue138_BuildTidyAndSbomCoverEveryProviderModule(t *testing.T) {
	modules, _ := providerDirs138(t)
	for _, target := range []string{"build-all", "tidy-all", "sbom"} {
		t.Run(target, func(t *testing.T) {
			loops := makeLoops138(t, target)
			require.Len(t, loops, 1)
			assert.Equal(t, modules, loops[0])
		})
	}
}

// Issue #138 claim 5: the release targets act on every provider module
func TestIssue138_ReleaseTargetsCoverEveryProviderModule(t *testing.T) {
	modules, _ := providerDirs138(t)
	for _, target := range []string{"update-deps", "tag-version"} {
		t.Run(target, func(t *testing.T) {
			loops := makeLoops138(t, target, "VERSION=v0.0.0")
			require.Len(t, loops, 1)
			assert.Equal(t, modules, loops[0])
		})
	}
}

// Guard: directories under provider/ that are not modules (core packages) are not listed
func TestIssue138_GuardCorePackagesAreNotProviderModules(t *testing.T) {
	_, corePackages := providerDirs138(t)
	require.NotEmpty(t, corePackages, "provider/ has no core package left to guard against")
	loops := makeLoops138(t, "test")
	require.Len(t, loops, 1)
	for _, corePkg := range corePackages {
		assert.NotContains(t, loops[0], corePkg)
	}
}

// Guard: run from a directory with no provider modules, every target that loops over them
// fails instead of silently doing nothing; targets that do not use them still work
func TestIssue138_GuardEmptyProviderListFails(t *testing.T) {
	providerDirs138(t)
	elsewhere := t.TempDir()
	for _, target := range []string{"test", "test-providers", "build-all", "tidy-all", "sbom", "update-deps", "tag-version"} {
		t.Run(target, func(t *testing.T) {
			out, err := make138(t, elsewhere, target, "VERSION=v0.0.0").CombinedOutput()
			require.Error(t, err, string(out))
			assert.Contains(t, string(out), "no provider modules found")
		})
	}
	t.Run("help", func(t *testing.T) {
		out, err := make138(t, elsewhere, "help").CombinedOutput()
		require.NoError(t, err, string(out))
	})
	// an empty PROVIDERS given on the command line, at the repository root
	t.Run("PROVIDERS= at the root", func(t *testing.T) {
		out, err := make138(t, "", "test", "PROVIDERS=").CombinedOutput()
		require.Error(t, err, string(out))
		assert.Contains(t, string(out), "no provider modules found")
	})
}
