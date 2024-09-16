package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/route-registrar/config"
	"code.cloudfoundry.org/route-registrar/healthchecker"
	"code.cloudfoundry.org/route-registrar/messagebus"
	"code.cloudfoundry.org/route-registrar/registrar"
	"code.cloudfoundry.org/route-registrar/routingapi"
	routing_api "code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/uaaclient"
	"code.cloudfoundry.org/tlsconfig"

	"github.com/tedsuo/ifrit"
)

func main() {
	var configPath string
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	pidfile := flags.String("pidfile", "", "Path to pid file")
	lagerflags.AddFlags(flags)

	flags.StringVar(&configPath, "configPath", "", "path to configuration file with json encoded content")
	err := flags.Set("configPath", "registrar_settings.yml")
	if err != nil {
		log.Fatalf("Failed to set up configPath flag")
	}
	// #nosec G104 - setting flags.ExitOnError means this function will never return an error
	flags.Parse(os.Args[1:])

	logger, _ := lagerflags.New("Route Registrar")

	logger.Info("Initializing")

	configSchema, err := config.NewConfigSchemaFromFile(configPath)
	if err != nil {
		logger.Fatal("error parsing file: %s\n", err)
	}

	c, err := configSchema.ParseSchemaAndSetDefaultsToConfig()
	if err != nil {
		log.Fatalln(err)
	}

	hc := healthchecker.NewHealthChecker(logger)

	logger.Info("creating nats connection")
	messageBus := messagebus.NewMessageBus(logger, c.AvailabilityZone)

	var routingAPI *routingapi.RoutingAPI
	if c.RoutingAPI.APIURL != "" {
		logger.Info("creating routing API connection")

		tlsConfig := &tls.Config{InsecureSkipVerify: c.RoutingAPI.SkipSSLValidation}
		if c.RoutingAPI.CACerts != "" {
			certBytes, err := os.ReadFile(c.RoutingAPI.CACerts)
			if err != nil {
				log.Fatalf("Failed to read ca cert file: %s", err.Error())
			}

			caCertPool := x509.NewCertPool()
			if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
				log.Fatal(errors.New("Unable to load caCert"))
			}
			tlsConfig.RootCAs = caCertPool
		}

		tr := &http.Transport{
			TLSClientConfig: tlsConfig,
		}

		httpClient := &http.Client{Transport: tr}
		httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}

		oauthUrl, err := url.Parse(c.RoutingAPI.OAuthURL)
		if err != nil {
			log.Fatalf("Could not parse RoutingAPI OAuth URL: %s", err)
		}
		port, err := strconv.Atoi(oauthUrl.Port())
		if err != nil {
			log.Fatalf("RoutingAPI OAuth port (%s) not an integer: %s", oauthUrl.Port(), err)
		}

		uaaConfig := uaaclient.Config{
			Port:              port,
			Protocol:          oauthUrl.Scheme,
			SkipSSLValidation: c.RoutingAPI.SkipSSLValidation,
			ClientName:        c.RoutingAPI.ClientID,
			ClientSecret:      c.RoutingAPI.ClientSecret,
			CACerts:           c.RoutingAPI.CACerts,
			TokenEndpoint:     oauthUrl.Hostname(),
		}
		clk := clock.NewClock()
		uaaClient, err := uaaclient.NewTokenFetcher(false, uaaConfig, clk, 3, 500*time.Millisecond, 30, logger)
		if err != nil {
			log.Fatalln(err)
		}

		apiClient, err := newAPIClient(c)

		if err != nil {
			logger.Fatal("failed-to-create-tls-config", err)
		}

		routingAPI = routingapi.NewRoutingAPI(logger, uaaClient, apiClient, c.RoutingAPI.MaxTTL)
	}

	r := registrar.NewRegistrar(*c, hc, logger, messageBus, routingAPI, 10*time.Second)

	if *pidfile != "" {
		pid := strconv.Itoa(os.Getpid())
		err := os.WriteFile(*pidfile, []byte(pid), 0644)
		logger.Info("Writing pid", lager.Data{"pid": pid, "file": *pidfile})
		if err != nil {
			logger.Fatal(
				"error writing pid to pidfile",
				err,
				lager.Data{
					"pid":     pid,
					"pidfile": *pidfile,
				},
			)
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	logger.Info("Running")

	process := ifrit.Invoke(r)
	for {
		select {
		case s := <-sigChan:
			logger.Info("Caught signal", lager.Data{"signal": s})
			process.Signal(s)
		case err := <-process.Wait():
			if err != nil {
				logger.Fatal("Exiting with error", err)
			}
			logger.Info("Exiting without error")
			os.Exit(0)
		}
	}
}

func newAPIClient(c *config.Config) (routing_api.Client, error) {
	apiURL, err := url.Parse(c.RoutingAPI.APIURL)
	if err != nil {
		return nil, err
	}

	var client routing_api.Client

	if apiURL.Scheme == "https" {
		routingAPITLSConfig, err := tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(c.RoutingAPI.ClientCertificatePath, c.RoutingAPI.ClientPrivateKeyPath),
		).Client(
			tlsconfig.WithAuthorityFromFile(c.RoutingAPI.ServerCACertificatePath),
		)
		if err != nil {
			return nil, err
		}

		client = routing_api.NewClientWithTLSConfig(c.RoutingAPI.APIURL, routingAPITLSConfig)
	} else {
		client = routing_api.NewClient(c.RoutingAPI.APIURL, c.RoutingAPI.SkipSSLValidation)
	}

	return client, nil
}
