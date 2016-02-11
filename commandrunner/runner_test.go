package commandrunner_test

import (
	"bytes"
	"time"

	"github.com/cloudfoundry-incubator/route-registrar/commandrunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"io/ioutil"
	"os"
	"path/filepath"
)

var _ = Describe("CommandRunner", func() {
	var (
		scriptPath string
		tmpDir     string
		outbuf     bytes.Buffer
		errbuf     bytes.Buffer
		r          commandrunner.Runner
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir(os.TempDir(), "healthchecker-test")
		Expect(err).ToNot(HaveOccurred())

		scriptPath = filepath.Join(tmpDir, "healthchecker.sh")

		outbuf = bytes.Buffer{}
		errbuf = bytes.Buffer{}

		r = commandrunner.NewRunner(scriptPath)
	})

	Describe("Run", func() {
		BeforeEach(func() {
			scriptText := "#!/bin/bash\necho 'my-stdout'; >&2 echo 'my-stderr'; exit 0\n"
			ioutil.WriteFile(scriptPath, []byte(scriptText), os.ModePerm)
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

			Eventually(r.CommandErrorChannel()).Should(Receive())
		})

		Context("when the script fails to start", func() {
			BeforeEach(func() {
				ioutil.WriteFile(scriptPath, []byte(""), 0666)
			})

			It("returns error", func() {
				err := r.Run(&outbuf, &errbuf)
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when the script exits with a non-zero code", func() {
			BeforeEach(func() {
				scriptText := "#!/bin/bash\n exit 1\n"
				ioutil.WriteFile(scriptPath, []byte(scriptText), os.ModePerm)
			})

			It("places the error on the error chan", func() {
				err := r.Run(&outbuf, &errbuf)
				Expect(err).NotTo(HaveOccurred())

				Eventually(r.CommandErrorChannel()).Should(Receive(HaveOccurred()))
			})
		})
	})

	Describe("Kill", func() {
		BeforeEach(func() {
		})

		Context("when the kill succeeds", func() {
			BeforeEach(func() {
				scriptText := "#!/bin/bash\nsleep 10; exit 0\n"
				ioutil.WriteFile(scriptPath, []byte(scriptText), os.ModePerm)

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
				scriptText := "#!/bin/bash\nexit 0\n"
				ioutil.WriteFile(scriptPath, []byte(scriptText), os.ModePerm)

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
