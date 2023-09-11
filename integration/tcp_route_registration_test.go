package integration

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	tls_helpers "code.cloudfoundry.org/cf-routing-test-helpers/tls"
	"code.cloudfoundry.org/route-registrar/config"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TCP Route Registration", func() {
	var (
		oauthServer      *ghttp.Server
		routingAPIServer *ghttp.Server
		natsCmd          *exec.Cmd
		rootConfig       config.ConfigSchema
		oauthHandlers    []http.HandlerFunc
	)

	BeforeEach(func() {
		routingAPICAFileName, routingAPICAPrivateKey := tls_helpers.GenerateCa()
		_, _, serverTLSConfig := tls_helpers.GenerateCertAndKey(routingAPICAFileName, routingAPICAPrivateKey)
		routingAPIClientCertPath, routingAPIClientPrivateKeyPath, _ := tls_helpers.GenerateCertAndKey(routingAPICAFileName, routingAPICAPrivateKey)

		routingAPIServer = ghttp.NewUnstartedServer()
		routingAPIServer.HTTPTestServer.TLS = &tls.Config{}
		routingAPIServer.HTTPTestServer.TLS.RootCAs = tls_helpers.CertPool(routingAPICAFileName)
		routingAPIServer.HTTPTestServer.TLS.ClientCAs = tls_helpers.CertPool(routingAPICAFileName)
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
					"ttl": 1,
					"isolation_segment":""
				}]`),
				ghttp.RespondWith(200, ""),
			),
		}
		routingAPIServer.AppendHandlers(routingAPIResponses...)
		routingAPIServer.SetAllowUnhandledRequests(true) //sometimes multiple creates happen

		oauthServer = ghttp.NewUnstartedServer()
		oauthHandlers = []http.HandlerFunc{
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/oauth/token"),
				ghttp.RespondWith(200, `{
					"access_token": "some-access-token",
					"token_type": "bearer",
					"expires_in": 3600
				}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			),
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/oauth/token"),
				ghttp.RespondWith(200, `{
					"access_token": "some-access-token",
					"token_type": "bearer",
					"expires_in": 3600
				}`,
					http.Header{"Content-Type": []string{"application/json"}},
				),
			),
		}

		rootConfig = initConfig()
		rootConfig.RoutingAPI.ClientID = "my-client"
		rootConfig.RoutingAPI.ClientSecret = "my-secret"
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
		natsCmd = startNats()
	})

	JustBeforeEach(func() {
		oauthServer.AppendHandlers(oauthHandlers...)
		oauthServer.Start()
		rootConfig.RoutingAPI.OAuthURL = oauthServer.URL()
	})

	AfterEach(func() {
		Expect(natsCmd.Process.Kill()).To(Succeed())
		routingAPIServer.Close()
		oauthServer.Close()
	})

	Context("when provided a tcp route", func() {
		JustBeforeEach(func() {
			routingAPIServer.HTTPTestServer.Start()
			rootConfig.RoutingAPI.APIURL = routingAPIServer.URL()
			writeConfig(rootConfig)
		})

		var session *gexec.Session

		BeforeEach(func() {
			var err error
			session, err = registerRoute()
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
		})
		Context("when UAA errors intermittently occur", func() {
			BeforeEach(func() {
				oauthHandlers = []http.HandlerFunc{
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/oauth/token"),
						ghttp.RespondWith(500, `{}`, http.Header{"Content-Type": []string{"application/json"}}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/oauth/token"),
						ghttp.RespondWith(200, `{
				"access_token": "some-access-token",
				"token_type": "bearer",
				"expires_in": 3600
				}`,
							http.Header{"Content-Type": []string{"application/json"}},
						),
					)}
			})
			It("Retries UAA token refreshes if problems were encountered", func() {
				Eventually(session.Out).Should(gbytes.Say("error-fetching-token"))
				Consistently(session.Out, 5*time.Second).ShouldNot(gbytes.Say("token-error"))
			})
		})

		Context("when UAA errors consistently ooccur", func() {
			BeforeEach(func() {
				oauthHandlers = []http.HandlerFunc{
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/oauth/token"),
						ghttp.RespondWith(500, `{}`, http.Header{"Content-Type": []string{"application/json"}}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/oauth/token"),
						ghttp.RespondWith(500, `{}`, http.Header{"Content-Type": []string{"application/json"}}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/oauth/token"),
						ghttp.RespondWith(500, `{}`, http.Header{"Content-Type": []string{"application/json"}}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/oauth/token"),
						ghttp.RespondWith(500, `{}`, http.Header{"Content-Type": []string{"application/json"}}),
					),
				}
			})
			It("Gives up and returns a token error", func() {
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("token-error"))
				Eventually(session.Out, 5*time.Second).Should(gbytes.Say("error\":\"oauth2: cannot fetch token:"))
			})
		})
	})

	Context("when routing API uses TLS", func() {
		Context("when provided a tcp route", func() {
			JustBeforeEach(func() {
				routingAPIServer.HTTPTestServer.StartTLS()
				rootConfig.RoutingAPI.APIURL = routingAPIServer.URL()
				writeConfig(rootConfig)
			})

			var session *gexec.Session

			BeforeEach(func() {
				var err error
				session, err = registerRoute()
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

})

func registerRoute() (*gexec.Session, error) {
	command := exec.Command(
		routeRegistrarBinPath,
		fmt.Sprintf("-logLevel=debug"),
		fmt.Sprintf("-pidfile=%s", pidFile),
		fmt.Sprintf("-configPath=%s", configFile),
	)

	return gexec.Start(command, GinkgoWriter, GinkgoWriter)
}

func startNats() *exec.Cmd {
	natsUsername := "nats"
	natsPassword := "nats"

	natsServer, exists := os.LookupEnv("NATS_SERVER_BINARY")
	if !exists {
		fmt.Println("You need nats-server installed and set NATS_SERVER_BINARY env variable")
		os.Exit(1)
	}
	natsCmd := exec.Command(
		natsServer,
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
