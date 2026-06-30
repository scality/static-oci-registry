// nolint: revive // var-naming: this is okay, test utils is straight forward
package utils

import (
	"log/slog"
	"os"
	"os/exec"

	// nolint: revive,staticcheck // only gomega and ginkgo are to be used as dot imports
	. "github.com/onsi/gomega"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
)

const (
	PermissionOK     os.FileMode = 0o700
	PermissionNoRead os.FileMode = 0o300
	PermissionNone   os.FileMode = 0o000
)

type (
	RegistryEntry struct {
		Solution string
		Version  string
		Image    domain.ImageName
		Tag      string
	}

	TestSuite struct {
		FsRoot string
		Logger *slog.Logger
		Name   string
	}

	QueryParams map[string]string
)

func (re *RegistryEntry) ImagePath(root string) string {
	return root + "/" + re.Solution + "/" + re.Version + "/" + string(re.Image)
}

func ValidateError(err error) {
	// should not be nil
	Expect(err).To(HaveOccurred())

	// should either be an internal registry error, or have a non-nil ociError field
	if errors.Is(err, domain.ErrRegistryInternal) {
		return
	}

	ociErr, ok := ocierrors.AsOCIError(err)
	// should have ociError field
	Expect(ok).To(BeTrue())

	// ociError should be correctly filled
	Expect(ociErr).NotTo(BeNil())
	Expect(ociErr.Code).NotTo(BeEmpty())
}

func NewTestSuite(name string) *TestSuite {
	suite := NewTestSuiteNoLogger(name)

	suite.InitLogger()

	return suite
}

// in case we want to use another logger.
func NewTestSuiteNoLogger(name string) *TestSuite {
	suite := &TestSuite{Name: name}

	suite.InitFsRoot()

	return suite
}

func (s *TestSuite) InitFsRoot() {
	tmpdir, err := os.MkdirTemp("/tmp", "test-static-oci-registry-"+s.Name+"-*")
	Expect(err).NotTo(HaveOccurred())

	s.FsRoot = tmpdir
}

func (s *TestSuite) CleanupFsRoot() {
	_, err := os.Stat(s.FsRoot)
	if !os.IsNotExist(err) {
		// clean up
		Expect(os.RemoveAll(s.FsRoot)).To(Succeed())
		return
	}

	Expect(err).NotTo(HaveOccurred())
}

func (s *TestSuite) InitLogger() {
	// Suppress logs below a synthetic level above Error to mimic zerolog's FatalLevel filter.
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError + 1})
	s.Logger = slog.New(handler).With(
		slog.String("root", s.FsRoot),
		slog.String("test_suite", s.Name),
	)
}

func (s *TestSuite) FetchImage(re *RegistryEntry) {
	Expect(os.MkdirAll(re.ImagePath(s.FsRoot), PermissionOK)).To(Succeed())

	// skopeo copy --all (multi-arch) from a public registry into a shared OCI
	// Image Layout; the tag becomes the index.json ref.name annotation. Repeated
	// calls with the same ImagePath accumulate tags into one layout (shared blobs).
	// nolint: gosec // G204: test-only
	cmd := exec.Command(
		"skopeo", "copy",
		"--all",
		"--insecure-policy",
		"docker://"+string(re.Image)+":"+re.Tag,
		"oci:"+re.ImagePath(s.FsRoot)+":"+re.Tag,
	)

	out, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "skopeo copy failed: %s", string(out))
}

// BuildImage builds a controlled, single-architecture image locally from the
// test Dockerfile (TARGET_DOCKERFILE) and converts it into an OCI Image Layout.
// Unlike FetchImage it does not depend on any public registry, so it is
// deterministic and free of Docker Hub rate limits; use it for the core flows
// and FetchImage for real multi-arch coverage.
func (s *TestSuite) BuildImage(re *RegistryEntry) {
	Expect(os.MkdirAll(re.ImagePath(s.FsRoot), PermissionOK)).To(Succeed())

	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = "unix:///var/run/docker.sock"
	}

	targetDockerfile := os.Getenv("TARGET_DOCKERFILE")
	Expect(targetDockerfile).NotTo(BeEmpty(),
		"TARGET_DOCKERFILE env var must point to the test Dockerfile")

	// nolint: gosec // G204: this is acceptable since it's for tests only
	buildCmd := exec.Command(
		"docker", "build",
		"-f", targetDockerfile,
		"--build-arg", "VERSION="+re.Tag,
		"-t", "test-image:"+re.Tag,
		".",
	)

	out, err := buildCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "docker build failed: %s", string(out))

	// nolint: gosec // G204: this is acceptable since it's for tests only
	cmd := exec.Command(
		"skopeo", "copy",
		"--src-daemon-host", dockerHost,
		"--insecure-policy",
		"docker-daemon:test-image:"+re.Tag,
		"oci:"+re.ImagePath(s.FsRoot)+":"+re.Tag,
	)
	cmd.Env = append(os.Environ(), "DOCKER_HOST="+dockerHost)

	out, err = cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "skopeo copy failed: %s", string(out))
}

func (s *TestSuite) ClearImage(re *RegistryEntry) {
	targetDir := re.ImagePath(s.FsRoot)
	// remove the image dir
	_, err := os.Stat(targetDir)
	if err == nil || !os.IsNotExist(err) {
		Expect(os.Chmod(targetDir, PermissionOK)).To(Succeed())
		Expect(os.RemoveAll(targetDir)).To(Succeed())
	}
}
