package routingapi

import (
	"fmt"

	"code.cloudfoundry.org/route-registrar/config"

	uaaclient "code.cloudfoundry.org/uaa-go-client"

	"code.cloudfoundry.org/routing-api/models"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/routing-api"
)

type RoutingAPI struct {
	logger          lager.Logger
	uaaClient       uaaclient.Client
	apiClient       routing_api.Client
	routerGroupGUID map[string]string
}

func NewRoutingAPI(logger lager.Logger, uaaClient uaaclient.Client, apiClient routing_api.Client) *RoutingAPI {
	return &RoutingAPI{
		uaaClient:       uaaClient,
		apiClient:       apiClient,
		logger:          logger,
		routerGroupGUID: make(map[string]string),
	}
}

func (r *RoutingAPI) refreshToken() error {
	token, err := r.uaaClient.FetchToken(false)
	if err != nil {
		return err
	}

	r.apiClient.SetToken(token.AccessToken)
	return nil
}

func (r *RoutingAPI) getRouterGroupGUID(name string) (string, error) {
	guid, exists := r.routerGroupGUID[name]
	if exists {
		return guid, nil
	}

	routerGroup, err := r.apiClient.RouterGroupWithName(name)
	if err != nil {
		return "", err
	}
	if routerGroup.Guid == "" {
		return "", fmt.Errorf("Router group '%s' not found", name)
	}

	r.logger.Info("Mapped new router group", lager.Data{
		"router_group": name,
		"guid":         routerGroup.Guid})

	r.routerGroupGUID[name] = routerGroup.Guid
	return routerGroup.Guid, nil
}

func (r *RoutingAPI) makeTcpRouteMapping(route config.Route) (models.TcpRouteMapping, error) {
	routerGroupGUID, err := r.getRouterGroupGUID(route.RouterGroup)
	if err != nil {
		return models.TcpRouteMapping{}, err
	}

	return models.NewTcpRouteMapping(
		routerGroupGUID,
		uint16(*route.ExternalPort),
		route.Host,
		uint16(*route.Port),
		int(route.RegistrationInterval.Seconds())), nil
}

func (r *RoutingAPI) RegisterRoute(route config.Route) error {
	err := r.refreshToken()
	if err != nil {
		return err
	}

	routeMapping, err := r.makeTcpRouteMapping(route)
	if err != nil {
		return err
	}

	return r.apiClient.UpsertTcpRouteMappings([]models.TcpRouteMapping{
		routeMapping})
}

func (r *RoutingAPI) UnregisterRoute(route config.Route) error {
	err := r.refreshToken()
	if err != nil {
		return err
	}

	routeMapping, err := r.makeTcpRouteMapping(route)
	if err != nil {
		return err
	}

	return r.apiClient.DeleteTcpRouteMappings([]models.TcpRouteMapping{routeMapping})
}
