package unit

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/static-oci-registry/test/utils"
)

var suite *utils.TestSuite

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
