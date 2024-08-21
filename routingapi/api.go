package routingapi

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/route-registrar/config"
	"golang.org/x/oauth2"

	"code.cloudfoundry.org/routing-api/models"

	"code.cloudfoundry.org/lager/v3"
	routingAPI "code.cloudfoundry.org/routing-api"
)

type RoutingAPI struct {
	logger          lager.Logger
	uaaClient       uaaClient
	apiClient       routingAPI.Client
	routerGroupGUID map[string]string

	routingAPIMaxTTL time.Duration
}

//go:generate counterfeiter . uaaClient
type uaaClient interface {
	FetchToken(context.Context, bool) (*oauth2.Token, error)
}

func NewRoutingAPI(logger lager.Logger, uaaClient uaaClient, apiClient routingAPI.Client, routingAPIMaxTTL time.Duration) *RoutingAPI {
	return &RoutingAPI{
		uaaClient:       uaaClient,
		apiClient:       apiClient,
		logger:          logger,
		routerGroupGUID: make(map[string]string),

		routingAPIMaxTTL: routingAPIMaxTTL,
	}
}

func (r *RoutingAPI) refreshToken() error {
	r.logger.Info("refresh-token")
	token, err := r.uaaClient.FetchToken(context.Background(), false)
	if err != nil {
		r.logger.Error("token-error", err)
		return err
	}

	r.logger.Debug("set-token", lager.Data{"token": token})
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

	r.logger.Info("Creating mapping", lager.Data{})

	hostTLSPort := -1
	if route.TLSPort != nil {
		hostTLSPort = *route.TLSPort
	}

	return models.NewTcpRouteMapping(
		routerGroupGUID,
		uint16(*route.ExternalPort),
		route.Host,
		uint16(*route.Port),
		hostTLSPort,
		"",
		nilIfEmpty(&route.ServerCertDomainSAN),
		calculateTTL(route.RegistrationInterval, r.routingAPIMaxTTL),
		models.ModificationTag{},
	), nil
}

const TTLBuffer float64 = 2.1

// add a buffer to the registration interval so that it is not the same as the
// TTL
func calculateTTL(requestedTTL, maxTTL time.Duration) int {
	ttl := time.Duration(float64(requestedTTL) * TTLBuffer)
	if ttl > maxTTL {
		return int(maxTTL.Seconds())
	}
	// ensure a bare minimum of TTL in case registration interval is <1s
	if int(ttl.Seconds()) < 1 {
		return 1
	}
	return int(ttl.Seconds())
}

func nilIfEmpty(str *string) *string {
	if str == nil || *str == "" {
		return nil
	}
	return str
}

func (r *RoutingAPI) RegisterRoute(route config.Route) error {
	err := r.refreshToken()
	if err != nil {
		r.logger.Error("Failed to refresh UAA token", err)
		return err
	}

	routeMapping, err := r.makeTcpRouteMapping(route)
	if err != nil {
		r.logger.Error("Failed to make route mapping", err, lager.Data{"route": route})
		return err
	}

	err = r.apiClient.UpsertTcpRouteMappings([]models.TcpRouteMapping{
		routeMapping})
	if err != nil {
		r.logger.Error("Failed to upsert route mapping", err, lager.Data{"route-mapping": routeMapping})
		return err
	}

	r.logger.Info("Upserted route", lager.Data{"route-mapping": routeMapping})
	return nil
}

func (r *RoutingAPI) UnregisterRoute(route config.Route) error {
	err := r.refreshToken()
	if err != nil {
		r.logger.Error("Failed to refresh UAA token", err)
		return err
	}

	routeMapping, err := r.makeTcpRouteMapping(route)
	if err != nil {
		r.logger.Error("Failed to make route mapping", err, lager.Data{"route": route})
		return err
	}

	err = r.apiClient.DeleteTcpRouteMappings([]models.TcpRouteMapping{routeMapping})
	if err != nil {
		r.logger.Error("Failed to delete route mapping", err, lager.Data{"route-mapping": routeMapping})
		return err
	}
	r.logger.Info("Deleted route", lager.Data{"route-mapping": routeMapping})
	return nil
}
