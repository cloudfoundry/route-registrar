package routingapi

import (
	"code.cloudfoundry.org/route-registrar/config"
	"fmt"
	"time"

	uaaclient "code.cloudfoundry.org/uaa-go-client"
	uaaconfig "code.cloudfoundry.org/uaa-go-client/config"

	"code.cloudfoundry.org/routing-api/models"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api"
)

type RoutingAPI interface {
	Init(api_config config.RoutingAPI) error
	RegisterRoute(route config.Route) error
	UnregisterRoute(route config.Route) error
	Close()
}

type apiState struct {
	logger          lager.Logger
	uaaClient       uaaclient.Client
	apiClient       routing_api.Client
	routerGroupGUID map[string]string
}

func NewRoutingAPI(logger lager.Logger) RoutingAPI {
	return &apiState{
		logger:          logger,
		routerGroupGUID: make(map[string]string),
	}
}

func buildOAuthConfig(config config.RoutingAPI) *uaaconfig.Config {

	return &uaaconfig.Config{
		UaaEndpoint:           config.OAuthURL,
		SkipVerification:      config.SkipCertValidation,
		ClientName:            config.ClientID,
		ClientSecret:          config.ClientSecret,
		MaxNumberOfRetries:    3,
		RetryInterval:         500 * time.Millisecond,
		ExpirationBufferInSec: 30,
		CACerts:               config.CACerts,
	}
}

func (a *apiState) Init(config config.RoutingAPI) error {
	cfg := buildOAuthConfig(config)
	clk := clock.NewClock()

	uaaClient, err := uaaclient.NewClient(a.logger, cfg, clk)
	if err != nil {
		return err
	}

	a.uaaClient = uaaClient
	a.apiClient = routing_api.NewClient(config.APIURL, config.SkipCertValidation)

	return nil
}

func (a *apiState) refreshToken() error {
	token, err := a.uaaClient.FetchToken(false)
	if err != nil {
		return err
	}

	a.apiClient.SetToken(token.AccessToken)
	return nil
}

func (a *apiState) getRouterGroupGUID(name string) (string, error) {
	guid, exists := a.routerGroupGUID[name]
	if exists {
		return guid, nil
	}

	routerGroup, err := a.apiClient.RouterGroupWithName(name)
	if err != nil {
		return "", err
	}
	if routerGroup.Guid == "" {
		return "", fmt.Errorf("Router group '%s' not found", name)
	}

	a.logger.Info("Mapped new router group", lager.Data{
		"router_group": name,
		"guid":         routerGroup.Guid})

	a.routerGroupGUID[name] = routerGroup.Guid
	return routerGroup.Guid, nil
}

func (a *apiState) makeTcpRouteMapping(route config.Route) (models.TcpRouteMapping, error) {
	routerGroupGUID, err := a.getRouterGroupGUID(route.RouterGroup)
	if err != nil {
		return models.TcpRouteMapping{}, err
	}

	return models.NewTcpRouteMapping(
		routerGroupGUID,
		uint16(*route.Port),
		route.BackendIP,
		uint16(route.BackendPort),
		int(route.RegistrationInterval.Seconds())), nil
}

func (a *apiState) RegisterRoute(route config.Route) error {
	err := a.refreshToken()
	if err != nil {
		return err
	}

	routeMapping, err := a.makeTcpRouteMapping(route)
	if err != nil {
		return err
	}

	return a.apiClient.UpsertTcpRouteMappings([]models.TcpRouteMapping{
		routeMapping})
}

func (a *apiState) UnregisterRoute(route config.Route) error {
	err := a.refreshToken()
	if err != nil {
		return err
	}

	routeMapping, err := a.makeTcpRouteMapping(route)
	if err != nil {
		return err
	}

	return a.apiClient.DeleteTcpRouteMappings([]models.TcpRouteMapping{routeMapping})
}

func (a *apiState) Close() {
}
