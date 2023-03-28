package routingapi

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	fakeuaa "code.cloudfoundry.org/route-registrar/routingapi/routingapifakes"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-registrar/config"
	"code.cloudfoundry.org/routing-api/fake_routing_api"
	"code.cloudfoundry.org/routing-api/models"
)

var _ = Describe("Routing API", func() {
	var (
		client    *fake_routing_api.FakeClient
		uaaClient *fakeuaa.FakeUaaClient

		api    *RoutingAPI
		logger lager.Logger

		port         int
		externalPort int
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("routing api test")
		uaaClient = &fakeuaa.FakeUaaClient{}
		uaaClient.FetchTokenReturns(&oauth2.Token{AccessToken: "my-token"}, nil)
		client = &fake_routing_api.FakeClient{}
		client.RouterGroupWithNameReturns(models.RouterGroup{Guid: "router-group-guid"}, nil)
		api = NewRoutingAPI(logger, uaaClient, client, 2*time.Minute)

		port = 1234
		externalPort = 5678
	})

	It("Sets SNI hostname if ServerCertDomainSAN is present.", func() {
		tcpRouteMapping, err := api.makeTcpRouteMapping(config.Route{
			Port:                &port,
			ExternalPort:        &externalPort,
			RouterGroup:         "my-router-group",
			ServerCertDomainSAN: "sniHostname",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(tcpRouteMapping.SniHostname).ToNot(BeNil())
		Expect(*tcpRouteMapping.SniHostname).To(Equal("sniHostname"))
	})

	It("SNI hostname nil if ServerCertDomainSAN is not present.", func() {
		tcpRouteMapping, err := api.makeTcpRouteMapping(config.Route{
			Port:         &port,
			ExternalPort: &externalPort,
			RouterGroup:  "my-router-group",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(tcpRouteMapping.SniHostname).To(BeNil())
	})
})
