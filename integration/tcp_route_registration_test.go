package integration

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	testhelpers "test-helpers"

	"code.cloudfoundry.org/route-registrar/config"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TCP Route Registration", func() {
	var (
		oauthServer      *ghttp.Server
		routingAPIServer *ghttp.Server
		natsCmd          *exec.Cmd
	)

	BeforeEach(func() {
		routingAPICAFileName, routingAPICAPrivateKey := testhelpers.GenerateCa()
		_, _, serverTLSConfig := testhelpers.GenerateCertAndKey(routingAPICAFileName, routingAPICAPrivateKey)
		routingAPIClientCertPath, routingAPIClientPrivateKeyPath, _ := testhelpers.GenerateCertAndKey(routingAPICAFileName, routingAPICAPrivateKey)

		routingAPIServer = ghttp.NewUnstartedServer()
		routingAPIServer.HTTPTestServer.TLS = &tls.Config{}
		routingAPIServer.HTTPTestServer.TLS.RootCAs = testhelpers.CertPool(routingAPICAFileName)
		routingAPIServer.HTTPTestServer.TLS.ClientCAs = testhelpers.CertPool(routingAPICAFileName)
		routingAPIServer.HTTPTestServer.TLS.ClientAuth = tls.RequireAndVerifyClientCert
		routingAPIServer.HTTPTestServer.TLS.CipherSuites = []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}
		routingAPIServer.HTTPTestServer.TLS.PreferServerCipherSuites = true
		routingAPIServer.HTTPTestServer.TLS.Certificates = []tls.Certificate{serverTLSConfig}

		routingAPIResponses := []http.HandlerFunc{
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/routing/v1/router_groups"),
				ghttp.RespondWith(200, `[{
					"guid": "router-group-guid",
					"name": "my-router-group",
					"type": "tcp",
					"reservable_ports": "1024-1025"
				}]`),
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/routing/v1/tcp_routes/create"),
				ghttp.VerifyJSON(`[{
					"router_group_guid":"router-group-guid",
					"backend_port":1234,
					"backend_ip":"127.0.0.1",
					"port":5678,
					"modification_tag":{
						"guid":"",
						"index":0
					},
					"ttl":0,
					"isolation_segment":""
				}]`),
				ghttp.RespondWith(200, ""),
			),
		}
		routingAPIServer.AppendHandlers(routingAPIResponses...)
		routingAPIServer.SetAllowUnhandledRequests(true) //sometimes multiple creates happen
		routingAPIServer.HTTPTestServer.StartTLS()

		oauthServer = ghttp.NewUnstartedServer()
		oauthServerResponse := []http.HandlerFunc{
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/oauth/token"),
				ghttp.RespondWith(200, `{
					"access_token": "some-access-token",
					"token_type": "bearer",
					"expires_in": 3600
				}`),
			),
		}
		oauthServer.AppendHandlers(oauthServerResponse...)
		oauthServer.Start()

		rootConfig := initConfig()
		rootConfig.RoutingAPI.APIURL = routingAPIServer.URL()
		rootConfig.RoutingAPI.ClientID = "my-client"
		rootConfig.RoutingAPI.ClientSecret = "my-secret"
		rootConfig.RoutingAPI.OAuthURL = oauthServer.URL()
		rootConfig.RoutingAPI.ClientCertificatePath = routingAPIClientCertPath
		rootConfig.RoutingAPI.ClientPrivateKeyPath = routingAPIClientPrivateKeyPath
		rootConfig.RoutingAPI.ServerCACertificatePath = routingAPICAFileName

		var port = 1234
		var externalPort = 5678
		routes := []config.RouteSchema{{
			Name:                 "my-route",
			Type:                 "tcp",
			Port:                 &port,
			ExternalPort:         &externalPort,
			URIs:                 []string{"my-host"},
			RouterGroup:          "my-router-group",
			RegistrationInterval: "100ns",
		}}
		rootConfig.Routes = routes
		writeConfig(rootConfig)
		natsCmd = startNats()
	})

	AfterEach(func() {
		Expect(natsCmd.Process.Kill()).To(Succeed())
		routingAPIServer.Close()
		oauthServer.Close()
	})

	Context("when provided a tcp route", func() {
		var session *gexec.Session

		BeforeEach(func() {
			command := exec.Command(
				routeRegistrarBinPath,
				fmt.Sprintf("-pidfile=%s", pidFile),
				fmt.Sprintf("-configPath=%s", configFile),
			)

			var err error
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			session.Kill()
		})

		It("registers it with the routing API", func() {
			Eventually(session.Out).Should(gbytes.Say("Initializing"))
			Eventually(session.Out).Should(gbytes.Say("creating routing API connection"))
			Eventually(session.Out).Should(gbytes.Say("Writing pid"))
			Eventually(session.Out).Should(gbytes.Say("Running"))
			Eventually(session.Out).Should(gbytes.Say("Mapped new router group"))
			Eventually(session.Out).Should(gbytes.Say("Upserted route"))
			// Upserted Route content verified with expected body in the ghttp server setup
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
