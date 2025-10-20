package utils

import (
	"os"
	"os/exec"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/scality/static-oci-registry/pkg/domain"
	apperrors "github.com/scality/static-oci-registry/pkg/errors"
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
		Logger *zerolog.Logger
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

// in case we want to use another logger
func NewTestSuiteNoLogger(name string) *TestSuite {
	suite := &TestSuite{Name: name}

	suite.InitFsRoot()

	return suite
}

func (s *TestSuite) InitFsRoot() {
	tmpdir, err := os.MkdirTemp("/tmp", "test-static-oci-registry-" + s.Name + "-*")
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
	l := zerolog.New(os.Stdout).Level(zerolog.FatalLevel).
		With().Timestamp().Str("root", s.FsRoot).Str("test_suite", s.Name).Logger()
	s.Logger = &l
}

func (s *TestSuite) FetchImage(re *RegistryEntry) {
	// create dirs and files as needed
	os.MkdirAll(re.ImagePath(s.FsRoot), 0o700)

	// call skopeo to copy from registry
	cmd := exec.Command(
		"skopeo", "copy",
		"docker://"+string(re.Image)+":"+re.Tag,
		"dir:"+re.FullPath(s.FsRoot),
	)

	Expect(cmd.Run()).To(Succeed())
}

func (s *TestSuite) ClearImage(re *RegistryEntry) {
	targetDir := re.ImagePath(s.FsRoot)
	// remove the image dir
	_, err := os.Stat(targetDir)
	if !os.IsNotExist(err) || err == nil {
		os.Chmod(targetDir, 0o700)
		Expect(os.RemoveAll(targetDir)).To(Succeed())
	}
}
