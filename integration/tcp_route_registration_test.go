package integration

import (
	"code.cloudfoundry.org/route-registrar/config"
	"code.cloudfoundry.org/routing-api/models"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os/exec"
	"strconv"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TCP Route Registration", func() {
	var (
		server   *httptest.Server
		natsCmd  *exec.Cmd
		bodyChan chan []byte
	)

	BeforeEach(func() {
		bodyChan = make(chan []byte, 1)
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch req.URL.Path {
			case "/oauth/token":
				_, err := w.Write([]byte(`{
					"access_token": "some-access-token",
					"token_type": "bearer",
					"expires_in": 3600
				}`))
				Expect(err).ToNot(HaveOccurred())
			case "/routing/v1/router_groups":
				_, err :=  w.Write([]byte(`[{
					"guid": "router-group-guid",
					"name": "my-router-group",
					"type": "tcp",
					"reservable_ports": "1024-1025"
				}]`))
				Expect(err).ToNot(HaveOccurred())
			case "/routing/v1/tcp_routes/create":
				bs, err := ioutil.ReadAll(req.Body)
				Expect(err).NotTo(HaveOccurred())

				bodyChan <- bs
			default:
				out, err := httputil.DumpRequest(req, true)
				Expect(err).NotTo(HaveOccurred())
				Fail(fmt.Sprintf("unexpected request: %s", out))
			}
		}))

		rootConfig := initConfig()
		rootConfig.RoutingAPI.APIURL = server.URL
		rootConfig.RoutingAPI.ClientID = "my-client"
		rootConfig.RoutingAPI.ClientSecret = "my-secret"
		rootConfig.RoutingAPI.OAuthURL = server.URL
		var port = 1234
		var externalPort = 5678
		rootConfig.Routes = []config.RouteSchema{{
			Name:                 "my-route",
			Type:                 "tcp",
			Port:                 &port,
			ExternalPort:         &externalPort,
			URIs:                 []string{"my-host"},
			RouterGroup:          "my-router-group",
			RegistrationInterval: "100ns",
		}}
		writeConfig(rootConfig)
		natsCmd = startNats()
	})

	AfterEach(func() {
		close(bodyChan)
		stopNats(natsCmd)
	})

	Context("when provided a tcp route", func() {
		var session *gexec.Session

		It("registers it with the routing API", func() {
			command := exec.Command(
				routeRegistrarBinPath,
				fmt.Sprintf("-pidfile=%s", pidFile),
				fmt.Sprintf("-configPath=%s", configFile),
			)

			var err error
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Out).Should(gbytes.Say("Initializing"))
			Eventually(session.Out).Should(gbytes.Say("creating routing API connection"))
			Eventually(session.Out).Should(gbytes.Say("Writing pid"))
			Eventually(session.Out).Should(gbytes.Say("Running"))
			Eventually(session.Out).Should(gbytes.Say("Mapped new router group"))
			Eventually(session.Out).Should(gbytes.Say("Upserted route"))
			session.Kill().Wait()
			Eventually(session).Should(gexec.Exit())

			var bodyBytes []byte
			Eventually(bodyChan, "10s").Should(Receive(&bodyBytes))
			server.Close()
			stopNats(natsCmd)

			var routeMappings []models.TcpMappingEntity
			err = json.Unmarshal(bodyBytes, &routeMappings)
			Expect(err).NotTo(HaveOccurred())

			Expect(routeMappings[0].RouterGroupGuid).To(Equal("router-group-guid"))
			Expect(routeMappings[0].HostPort).To(Equal(uint16(1234)))
			Expect(routeMappings[0].HostIP).To(Equal("127.0.0.1"))
			Expect(routeMappings[0].ExternalPort).To(Equal(uint16(5678)))
		})
	})
})

func startNats() *exec.Cmd {
	natsUsername := "nats"
	natsPassword := "nats"

	natsCmd := exec.Command(
		"gnatsd",
		"-p", strconv.Itoa(natsPort),
		"--user", natsUsername,
		"--pass", natsPassword,
	)

	err := natsCmd.Start()
	Expect(err).NotTo(HaveOccurred())

	natsAddress := fmt.Sprintf("127.0.0.1:%d", natsPort)

	Eventually(func() error {
		_, err := net.Dial("tcp", natsAddress)
		return err
	}).Should(Succeed())

	return natsCmd
}

func stopNats(natsCmd *exec.Cmd) {
	Expect(natsCmd.Process.Kill()).To(Succeed())
}
