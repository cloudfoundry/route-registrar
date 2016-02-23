package commandrunner_test

import (
	"bytes"
	"strings"
	"time"

	. "github.com/cloudfoundry-incubator/route-registrar/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry-incubator/route-registrar/Godeps/_workspace/src/github.com/onsi/gomega"
	"github.com/cloudfoundry-incubator/route-registrar/commandrunner"
	"github.com/onsi/gomega/gexec"

	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	golangExecutable = `
package main

import "fmt"

func main() {
	fmt.Println("Hello from a binary")
}`
)

var _ = Describe("CommandRunner", func() {
	var (
		executable   string
		tmpDir       string
		tmpGoPkg     string
		tmpGoPkgPath string
		outbuf       bytes.Buffer
		errbuf       bytes.Buffer
		r            commandrunner.Runner
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "healthchecker-test")
		Expect(err).NotTo(HaveOccurred())

		gopathEnv := os.Getenv("GOPATH")
		gopathArray := strings.SplitN(gopathEnv, ":", 1)
		gopath := gopathArray[0]
		Expect(gopath).NotTo(BeEmpty())

		tmpGoPkg = "tmp-foo"
		tmpGoPkgPath = filepath.Join(gopath, "src", tmpGoPkg)
		err = os.MkdirAll(tmpGoPkgPath, os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		executable = filepath.Join(tmpDir, "healthchecker.sh")
		scriptText := "echo 'my-stdout'; >&2 echo 'my-stderr'; exit 0\n"
		ioutil.WriteFile(executable, []byte(scriptText), os.ModePerm)

		outbuf = bytes.Buffer{}
		errbuf = bytes.Buffer{}
	})

	AfterEach(func() {
		err := os.RemoveAll(tmpDir)
		Expect(err).ShouldNot(HaveOccurred())

		err = os.RemoveAll(tmpGoPkg)
		Expect(err).ShouldNot(HaveOccurred())
	})

	Describe("Run", func() {
		JustBeforeEach(func() {
			r = commandrunner.NewRunner(executable)
		})

		It("captures stdout and stderr", func() {
			err := r.Run(&outbuf, &errbuf)
			Expect(err).NotTo(HaveOccurred())

			Eventually(outbuf.String).Should(ContainSubstring("my-stdout"))
			Eventually(errbuf.String).Should(ContainSubstring("my-stderr"))
		})

		It("runs the command in the background", func() {
			err := r.Run(&outbuf, &errbuf)
			Expect(err).NotTo(HaveOccurred())

			err = r.Wait()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the script exits with a non-zero code", func() {
			BeforeEach(func() {
				scriptText := "exit 1\n"
				ioutil.WriteFile(executable, []byte(scriptText), os.ModePerm)
			})

			It("places the error on the error chan", func() {
				err := r.Run(&outbuf, &errbuf)
				Expect(err).NotTo(HaveOccurred())

				err = r.Wait()
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("running a binary", func() {
			BeforeEach(func() {
				executableFilepath := filepath.Join(tmpGoPkgPath, "main.go")
				err := ioutil.WriteFile(executableFilepath, []byte(golangExecutable), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				executable, err = gexec.Build(tmpGoPkg)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("runs a binary without error", func() {
				err := r.Run(&outbuf, &errbuf)
				Expect(err).NotTo(HaveOccurred())

				err = r.Wait()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Kill", func() {
		BeforeEach(func() {
			r = commandrunner.NewRunner(executable)
		})
		Context("when the kill succeeds", func() {
			BeforeEach(func() {
				scriptText := "sleep 10; exit 0\n"
				ioutil.WriteFile(executable, []byte(scriptText), os.ModePerm)

				var outbuf, errbuf bytes.Buffer
				r.Run(&outbuf, &errbuf)
			})

			It("returns no error", func() {
				err := r.Kill()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the kill does not succeed", func() {
			BeforeEach(func() {
				scriptText := "exit 0\n"
				ioutil.WriteFile(executable, []byte(scriptText), os.ModePerm)

				var outbuf, errbuf bytes.Buffer
				r.Run(&outbuf, &errbuf)
			})

			It("places an error on the errChan", func() {
				time.Sleep(10 * time.Millisecond) // Give the script time to complete so `kill` returns an error
				err := r.Kill()
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
