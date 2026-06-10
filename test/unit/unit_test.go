package unit

import (
	"os"
	"testing"

	// nolint: revive,staticcheck // only gomega and ginkgo are to be used as dot imports
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/static-oci-registry/test/utils"
)

var suite *utils.TestSuite

func openRoot(path string) *os.Root {
	r, err := os.OpenRoot(path)
	Expect(err).NotTo(HaveOccurred())

	return r
}

func TestUnit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(nil, "Unit Suite")
}

var _ = BeforeSuite(func() {
	suite = utils.NewTestSuite("unit")
})

var _ = AfterSuite(func() {
	// delete contents of fsRoot
	suite.CleanupFsRoot()
})
