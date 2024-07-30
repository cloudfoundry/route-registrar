package registrar

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/route-registrar/config"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

var ErrWatcherEmptyEvent = errors.New("received empty event")

type RoutesConfigSchema struct {
	Routes []config.RouteSchema `json:"routes"`
}

type routesConfigWatcher struct {
	globs                 []string
	logger                lager.Logger
	watchInterval         time.Duration
	onRouteDiscoveryFunc  func(config.Route)
	discoveredConfigFiles map[string]bool
	discoveredRoutes      []config.Route
}

func NewRoutesConfigWatcher(logger lager.Logger, watchInterval time.Duration, globs []string, onRouteDiscoveryFunc func(config.Route)) *routesConfigWatcher {
	return &routesConfigWatcher{
		globs:                 globs,
		logger:                logger.Session("routes-config-watcher"),
		watchInterval:         watchInterval,
		onRouteDiscoveryFunc:  onRouteDiscoveryFunc,
		discoveredConfigFiles: map[string]bool{},
		discoveredRoutes:      []config.Route{},
	}
}

func (r *routesConfigWatcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		r.logger.Error("failed-to-create-file-watcher", err)
		return err
	}
	defer watcher.Close()
	close(ready)

	timer := time.NewTicker(r.watchInterval)
	for {
		select {
		case <-timer.C:
			err = r.discoverRoutesFromConfigFiles(watcher)
			if err != nil {
				r.logger.Error("failed-to-discover-config-files", err)
				return err
			}

		case event, ok := <-watcher.Events:
			if !ok {
				r.logger.Error("failed-to-get-watcher-event", ErrWatcherEmptyEvent)
				return ErrWatcherEmptyEvent
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				r.registerNewRoutesFromConfigFile(event.Name)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				r.logger.Error("failed-to-get-watcher-error", ErrWatcherEmptyEvent)
				return ErrWatcherEmptyEvent
			}
			r.logger.Error("wacther-failed", err)
			return err

		case s := <-signals:
			r.logger.Info("caught-signal", lager.Data{"signal": s})
			return nil
		}
	}
}

func (r *routesConfigWatcher) discoverRoutesFromConfigFiles(watcher *fsnotify.Watcher) error {
	for _, glob := range r.globs {
		files, err := filepath.Glob(glob)
		if err != nil {
			r.logger.Error("failed-to-glob-config-files", err, lager.Data{"glob": glob})
			return err
		}

		for _, f := range files {
			if _, ok := r.discoveredConfigFiles[f]; !ok {
				r.discoveredConfigFiles[f] = true
				err = watcher.Add(f)
				if err != nil {
					r.logger.Error("failed-to-watch-config-file", err, lager.Data{"path": f})
					return err
				}

				r.registerNewRoutesFromConfigFile(f)
			}
		}
	}

	return nil
}

func (r *routesConfigWatcher) registerNewRoutesFromConfigFile(configFile string) {
	b, err := os.ReadFile(configFile)
	if err != nil {
		r.logger.Error("failed-to-read-macthed-file", err)
		return
	}
	var routesConfig RoutesConfigSchema
	err = yaml.Unmarshal(b, &routesConfig)
	if err != nil {
		r.logger.Error("failed-to-parse-file", err)
		return
	}

	for i, routeSchema := range routesConfig.Routes {
		route, err := config.RouteFromSchema(routeSchema, i)
		if err != nil {
			r.logger.Error("failed-to-parse-route", err)
			continue
		}

		if route != nil {
			if !containsRoute(r.discoveredRoutes, *route) {
				r.discoveredRoutes = append(r.discoveredRoutes, *route)
				r.onRouteDiscoveryFunc(*route)
			}
		}
	}
}

func containsRoute(routes []config.Route, route config.Route) bool {
	for _, r := range routes {
		if reflect.DeepEqual(r, route) {
			return true
		}
	}

	return false
}
