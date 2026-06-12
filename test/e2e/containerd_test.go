package e2e

import (
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	testImage      string
	testTags       []string
	imageRef       string
	registryDigest string
)

var _ = Describe("Containerd", Ordered, func() {
	BeforeEach(func() {
		// prune all images
		_ = exec.Command("crictl", "rmi", "--prune").Run()
		_ = exec.Command("podman", "rmi", "-a", "-f").Run()
	})

	BeforeAll(func() {
		// define testImage and testTags
		testImage = os.Getenv("TEST_IMAGE")
		if testImage == "" {
			testImage = "docker.io/library/alpine"
		}

		tagList := os.Getenv("TEST_TAGS")
		if tagList == "" {
			testTags = []string{"3.19", "3.20", "3.21", "3.22"}
		} else {
			testTags = strings.Split(tagList, ",")
		}

		imageRef = registryEndpoint + "/" + testImage

		resp, err := client.Head("https://" + registryEndpoint + "/v2/" + testImage + "/manifests/" + testTags[0])
		Expect(err).NotTo(HaveOccurred(), "failed to check if the image exists in the registry %s", err)
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(200), "failed to check if the image exists in the registry, status code: %d", resp.StatusCode)

		registryDigest = resp.Header.Get("Docker-Content-Digest")
		Expect(registryDigest).NotTo(BeEmpty(), "failed to get the image digest from the registry")
	})

	Context("Discovering an image tags", Ordered, func() {
		It("should discover the image tags successfully on podman", func() {
			podmanArgs := []string{
				"search",
				"--list-tags",
				imageRef,
				"--format", "{{.Tag}}",
			}
			out, err := exec.Command("podman", podmanArgs...).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "failed to discover image tags %s", string(out))

			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			Expect(lines).To(ContainElements(testTags), "failed to discover the correct image tags, expected: %v, got: %v", testTags, lines)
		})
	})

	Context("Pulling an image by tag", Ordered, func() {
		It("should pull the image successfully on containerd", func() {
			crictlPull(imageRef + ":" + testTags[0])
		})
		It("should pull the image successfully on podman", func() {
			podmanPull(imageRef + ":" + testTags[0])
		})
	})

	Context("Pulling an image by digest", Ordered, func() {
		It("should pull the image successfully on containerd", func() {
			crictlPull(imageRef + "@" + registryDigest)
		})

		It("should pull the image successfully on podman", func() {
			podmanPull(imageRef + "@" + registryDigest)
		})
	})

	Context("Pulling an image idempotently", Ordered, func() {
		It("should pull the image successfully on containerd", func() {
			crictlPull(imageRef + ":" + testTags[0])
			_ = exec.Command("crictl", "rmi", "--prune").Run()
			crictlPull(imageRef + ":" + testTags[0])
		})

		It("should pull the image successfully on podman", func() {
			podmanPull(imageRef + ":" + testTags[0])
			_ = exec.Command("podman", "rmi", "-a", "-f").Run()
			podmanPull(imageRef + ":" + testTags[0])
		})
	})
})

func podmanPull(ref string) {
	GinkgoHelper()
	podmanArgs := []string{
		"pull",
		ref,
	}
	out, err := exec.Command("podman", podmanArgs...).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "failed to pull image %s", string(out))

	podmanArgs = []string{
		"inspect",
		ref,
		"--format", "{{index .RepoDigests 0}}",
	}
	out, err = exec.Command("podman", podmanArgs...).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "failed to inspect image %s", string(out))
	Expect(out).To(ContainSubstring(registryDigest), "failed to pull the correct image by digest %s", string(out))
}

func crictlPull(ref string) {
	GinkgoHelper()
	crictlArgs := []string{
		"pull",
		ref,
	}
	out, err := exec.Command("crictl", crictlArgs...).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "failed to pull image %s", string(out))

	crictlArgs = []string{
		"inspecti",
		"-o", "go-template",
		"--template", "{{index .status.repoDigests 0}}",
		ref,
	}
	out, err = exec.Command("crictl", crictlArgs...).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "failed to inspect image %s", string(out))
	Expect(out).To(ContainSubstring(registryDigest), "failed to pull the correct image by digest %s", string(out))
}
