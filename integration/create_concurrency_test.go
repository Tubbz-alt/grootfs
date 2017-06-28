package integration_test

import (
	"fmt"
	"path/filepath"
	"sync"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent creations", func() {
	BeforeEach(func() {
		integration.SkipIfNonRootAndNotBTRFS(GrootfsTestUid, Driver)
		err := Runner.RunningAsUser(0, 0).InitStore(manager.InitSpec{
			UIDMappings: []groot.IDMappingSpec{
				{HostID: GrootUID, NamespaceID: 0, Size: 1},
				{HostID: 100000, NamespaceID: 1, Size: 65000},
			},
			GIDMappings: []groot.IDMappingSpec{
				{HostID: GrootGID, NamespaceID: 0, Size: 1},
				{HostID: 100000, NamespaceID: 1, Size: 65000},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Runner = Runner.SkipInitStore()
	})

	Context("warm cache", func() {
		BeforeEach(func() {
			// run this to setup the store before concurrency!
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "test-pre-warm",
				BaseImage: "docker:///cfgarden/empty",
				Mount:     true,
			})

			Expect(err).NotTo(HaveOccurred())
		})

		It("can create multiple rootfses of the same image concurrently", func() {
			wg := new(sync.WaitGroup)

			for i := 0; i < 200; i++ {
				wg.Add(1)
				go func(wg *sync.WaitGroup, idx int) {
					defer GinkgoRecover()
					defer wg.Done()
					runner := Runner.WithLogLevel(lager.ERROR) // clone runner to avoid data-race on stdout
					image, err := runner.Create(groot.CreateSpec{
						ID:        fmt.Sprintf("test-%d", idx),
						BaseImage: "docker:///cfgarden/empty",
						Mount:     true,
						DiskLimit: 2*1024*1024 + 512*1024,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(writeMegabytes(filepath.Join(image.Rootfs, "hello"), 2)).To(Succeed())
				}(wg, i)
			}

			wg.Wait()
		})

		Describe("parallel create and clean", func() {
			It("works in parallel, without errors", func() {
				wg := new(sync.WaitGroup)

				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()

					for i := 0; i < 100; i++ {
						runner := Runner.WithLogLevel(lager.ERROR) // clone runner to avoid data-race on stdout
						image, err := runner.Create(groot.CreateSpec{
							ID:        fmt.Sprintf("test-%d", i),
							BaseImage: "docker:///cfgarden/empty",
							Mount:     true,
							DiskLimit: 2*1024*1024 + 512*1024,
						})
						Expect(err).NotTo(HaveOccurred())
						Expect(writeMegabytes(filepath.Join(image.Rootfs, "hello"), 2)).To(Succeed())
					}
				}()

				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					runner := Runner.WithLogLevel(lager.ERROR) // clone runner to avoid data-race on stdout
					_, err := runner.Clean(0, []string{})
					Expect(err).To(Succeed())
				}()

				wg.Wait()
			})
		})
	})

	Context("cold cache", func() {
		BeforeEach(func() {
			// run this to setup the store before concurrency!
			// avoids the concurrent creation of namespace.json file
			// can be removed after #142588813 is delivered
			_, err := Runner.Create(groot.CreateSpec{
				ID:        "test-pre-warm",
				BaseImage: "docker:///cfgarden/empty",
				Mount:     true,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("can create multiple rootfses of the same image concurrently", func() {
			wg := new(sync.WaitGroup)

			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func(wg *sync.WaitGroup, idx int) {
					defer GinkgoRecover()
					defer wg.Done()
					runner := Runner.WithLogLevel(lager.ERROR) // clone runner to avoid data-race on stdout
					image, err := runner.Create(groot.CreateSpec{
						ID:                        fmt.Sprintf("test-%d", idx),
						BaseImage:                 "docker:///ubuntu",
						Mount:                     true,
						DiskLimit:                 2*1024*1024 + 512*1024,
						ExcludeBaseImageFromQuota: true,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(writeMegabytes(filepath.Join(image.Rootfs, "hello"), 2)).To(Succeed())
				}(wg, i)
			}

			wg.Wait()
		})
	})
})
