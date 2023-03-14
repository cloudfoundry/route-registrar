package routingapi_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/route-registrar/config"
	"code.cloudfoundry.org/route-registrar/routingapi"
	"code.cloudfoundry.org/routing-api/fake_routing_api"
	"code.cloudfoundry.org/routing-api/models"

	"code.cloudfoundry.org/lager/lagertest"
	fakeuaa "code.cloudfoundry.org/route-registrar/routingapi/routingapifakes"
)

var _ = Describe("Routing API", func() {
	var (
		client    *fake_routing_api.FakeClient
		uaaClient *fakeuaa.FakeUaaClient

		api    *routingapi.RoutingAPI
		logger lager.Logger

		port                 int
		externalPort         int
		registrationInterval time.Duration

		maxTTL time.Duration
	)

	BeforeEach(func() {
		maxTTL = 2 * time.Minute

		logger = lagertest.NewTestLogger("routing api test")
		uaaClient = &fakeuaa.FakeUaaClient{}
		uaaClient.FetchTokenReturns(&oauth2.Token{AccessToken: "my-token"}, nil)
		client = &fake_routing_api.FakeClient{}
		api = routingapi.NewRoutingAPI(logger, uaaClient, client, maxTTL)

		port = 1234
		externalPort = 5678
		registrationInterval = 100 * time.Second
	})

	Describe("RegisterRoute", func() {
		BeforeEach(func() {
			client.RouterGroupWithNameReturns(models.RouterGroup{Guid: "router-group-guid"}, nil)
		})

		Context("when given a valid route", func() {
			It("registers the route using TTL that is larger than the registration interval", func() {
				err := api.RegisterRoute(config.Route{
					Name:                 "test-route",
					Port:                 &port,
					ExternalPort:         &externalPort,
					Host:                 "myhost",
					RegistrationInterval: registrationInterval,
					RouterGroup:          "my-router-group",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(uaaClient.FetchTokenCallCount()).To(Equal(1))

				Expect(client.SetTokenCallCount()).To(Equal(1))
				Expect(client.SetTokenArgsForCall(0)).To(Equal("my-token"))

				Expect(client.RouterGroupWithNameCallCount()).To(Equal(1))
				Expect(client.RouterGroupWithNameArgsForCall(0)).To(Equal("my-router-group"))

				expectedTTL := int((registrationInterval + routingapi.TTL_BUFFER).Seconds())
				expectedRouteMapping := models.TcpRouteMapping{TcpMappingEntity: models.TcpMappingEntity{
					RouterGroupGuid: "router-group-guid",
					HostPort:        1234,
					ExternalPort:    5678,
					HostIP:          "myhost",
					TTL:             &expectedTTL,
				}}
				Expect(client.UpsertTcpRouteMappingsCallCount()).To(Equal(1))
				Expect(client.UpsertTcpRouteMappingsArgsForCall(0)).To(Equal([]models.TcpRouteMapping{expectedRouteMapping}))
			})
		})

		Context("when the registration interval is equal to the max_ttl for routing api", func() {
			BeforeEach(func() {
				registrationInterval = maxTTL
			})

			It("does not add a buffer and caps TTL at max_ttl", func() {
				expectedRegistrationInterval := int(maxTTL.Seconds())

				err := api.RegisterRoute(config.Route{
					Name:                 "test-route",
					Port:                 &port,
					ExternalPort:         &externalPort,
					Host:                 "myhost",
					RegistrationInterval: registrationInterval,
					RouterGroup:          "my-router-group",
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(client.UpsertTcpRouteMappingsCallCount()).To(Equal(1))
				Expect(client.UpsertTcpRouteMappingsArgsForCall(0)[0].TcpMappingEntity.TTL).To(Equal(&expectedRegistrationInterval))
			})
		})

		Context("when the registration interval is greater than the  max_ttl for routing api", func() {
			BeforeEach(func() {
				registrationInterval = maxTTL + 10
			})

			It("caps TTL at max_ttl", func() {
				expectedRegistrationInterval := int(maxTTL.Seconds())

				err := api.RegisterRoute(config.Route{
					Name:                 "test-route",
					Port:                 &port,
					ExternalPort:         &externalPort,
					Host:                 "myhost",
					RegistrationInterval: registrationInterval,
					RouterGroup:          "my-router-group",
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(client.UpsertTcpRouteMappingsCallCount()).To(Equal(1))
				Expect(client.UpsertTcpRouteMappingsArgsForCall(0)[0].TcpMappingEntity.TTL).To(Equal(&expectedRegistrationInterval))
			})
		})

		Context("when the route mapping fails to register", func() {
			BeforeEach(func() {
				client.UpsertTcpRouteMappingsReturns(errors.New("registration error"))
			})

			It("returns an error", func() {
				err := api.RegisterRoute(config.Route{
					Name:                 "test-route",
					Port:                 &port,
					ExternalPort:         &externalPort,
					Host:                 "myhost",
					RegistrationInterval: time.Duration(registrationInterval) * time.Second,
					RouterGroup:          "my-router-group",
				})

				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("registration error"))
			})
		})
	})

	Describe("UnregisterRoute", func() {
		BeforeEach(func() {
			client.RouterGroupWithNameReturns(models.RouterGroup{Guid: "router-group-guid"}, nil)
		})

		Context("when given a valid route", func() {
			It("unregisters the route", func() {
				err := api.UnregisterRoute(config.Route{
					Name:                 "test-route",
					Port:                 &port,
					ExternalPort:         &externalPort,
					Host:                 "myhost",
					RegistrationInterval: registrationInterval,
					RouterGroup:          "my-router-group",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(uaaClient.FetchTokenCallCount()).To(Equal(1))

				Expect(client.SetTokenCallCount()).To(Equal(1))
				Expect(client.SetTokenArgsForCall(0)).To(Equal("my-token"))

				Expect(client.RouterGroupWithNameCallCount()).To(Equal(1))
				Expect(client.RouterGroupWithNameArgsForCall(0)).To(Equal("my-router-group"))

				expectedTTL := int((registrationInterval + routingapi.TTL_BUFFER).Seconds())
				routeMapping := models.TcpRouteMapping{TcpMappingEntity: models.TcpMappingEntity{
					RouterGroupGuid: "router-group-guid",
					HostPort:        1234,
					ExternalPort:    5678,
					HostIP:          "myhost",
					TTL:             &expectedTTL,
				}}

				Expect(client.DeleteTcpRouteMappingsCallCount()).To(Equal(1))
				Expect(client.DeleteTcpRouteMappingsArgsForCall(0)).To(Equal([]models.TcpRouteMapping{routeMapping}))
			})
		})

		Context("when the route mapping fails to unregister", func() {
			BeforeEach(func() {
				client.DeleteTcpRouteMappingsReturns(errors.New("unregistration error"))
			})
			It("returns an error", func() {
				err := api.UnregisterRoute(config.Route{
					Name:                 "test-route",
					Port:                 &port,
					ExternalPort:         &externalPort,
					Host:                 "myhost",
					RegistrationInterval: time.Duration(registrationInterval) * time.Second,
					RouterGroup:          "my-router-group",
				})

				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("unregistration error"))
			})
		})
	})

	Context("when an error occurs", func() {
		Context("when a UAA token cannot be fetched", func() {
			BeforeEach(func() {
				uaaClient.FetchTokenReturns(&oauth2.Token{}, errors.New("my fetch error"))
			})

			It("returns an error", func() {
				err := api.RegisterRoute(config.Route{})
				Expect(uaaClient.FetchTokenCallCount()).To(Equal(1))
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("my fetch error"))
			})
		})

		Context("when the router group name fails to return", func() {
			BeforeEach(func() {
				client.RouterGroupWithNameReturns(models.RouterGroup{}, errors.New("my router group failed"))
			})

			It("returns an error", func() {
				err := api.RegisterRoute(config.Route{
					Name:                 "test-route",
					Port:                 &port,
					ExternalPort:         &externalPort,
					Host:                 "myhost",
					RegistrationInterval: time.Duration(registrationInterval) * time.Second,
					RouterGroup:          "my-router-group",
				})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("my router group failed"))
			})
		})
	})
})
