// nolint: revive // var-naming: this is okay, test utils is straight forward
package utils

import (
	"log/slog"
	"os"
	"os/exec"

	// nolint: revive,staticcheck // only gomega and ginkgo are to be used as dot imports
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"github.com/scality/static-oci-registry/pkg/domain"
	apperrors "github.com/scality/static-oci-registry/pkg/errors"
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

func (re *RegistryEntry) FullPath(root string) string {
	return re.ImagePath(root) + "/" + re.Tag
}

func (re *RegistryEntry) ImagePath(root string) string {
	return root + "/" + re.Solution + "/" + re.Version + "/" + string(re.Image)
}

func ValidateError(err *apperrors.Error) {
	// should not be nil
	Expect(err).To(HaveOccurred())

	// should either be an internal registry error, or have a non-nil ociError field
	if errors.Is(errors.Cause(err), domain.ErrRegistryInternal) {
		return
	}

	ociErr, ok := apperrors.AsOCIError(err)
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
	// create dirs and files as needed
	Expect(os.MkdirAll(re.ImagePath(s.FsRoot), PermissionOK)).To(Succeed())

	// get DOCKER_HOST env var, or use default if not set
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = "unix:///var/run/docker.sock"
	}

	targetDockerfile := os.Getenv("TARGET_DOCKERFILE")
	Expect(targetDockerfile).NotTo(BeEmpty(),
		"TARGET_DOCKERFILE env var must be set to the path",
		" of the Dockerfile to build the test image")

	// docker build ../test.Dockerfile --build-arg VERSION=re.Tag -t test-image:re.Tag .
	// nolint: gosec // G204: this is acceptable since it's for tests only
	buildCmd := exec.Command(
		"docker", "build",
		"-f", targetDockerfile,
		"--build-arg", "VERSION="+re.Tag,
		"-t", "test-image:"+re.Tag,
		".",
	)
	Expect(buildCmd.Run()).To(Succeed())

	// skopeo copy with flags:
	// --format v2s2 --dest-compress --src-daemon-host <<DOCKER_HOST>> --insecure-policy
	skopeoArgs := []string{
		"copy",
		"--format", "v2s2",
		"--dest-compress",
		"--src-daemon-host", dockerHost,
		"--insecure-policy",
		"docker-daemon:test-image:" + re.Tag,
		"dir:" + re.FullPath(s.FsRoot),
	}
	// nolint: gosec // G204: this is acceptable since it's for tests only
	cmd := exec.Command("skopeo", skopeoArgs...)

	cmd.Env = append(os.Environ(), "DOCKER_HOST="+dockerHost)

	Expect(cmd.Run()).To(Succeed())
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
