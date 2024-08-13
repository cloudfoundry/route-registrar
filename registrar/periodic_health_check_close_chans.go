package registrar

import (
	"reflect"

	"code.cloudfoundry.org/route-registrar/config"
)

type PeriodicHealthcheckCloseChans struct {
	chans []PeriodicHealthcheckCloseChan
}

type PeriodicHealthcheckCloseChan struct {
	route     config.Route
	closeChan chan struct{}
}

func (p *PeriodicHealthcheckCloseChans) Add(route config.Route) chan struct{} {
	closeChan := make(chan struct{})
	p.chans = append(p.chans, PeriodicHealthcheckCloseChan{
		route:     route,
		closeChan: closeChan,
	})
	return closeChan
}

func (p *PeriodicHealthcheckCloseChans) CloseForRoute(route config.Route) {
	for i, c := range p.chans {
		if reflect.DeepEqual(c.route, route) {
			close(c.closeChan)
			p.chans = append(p.chans[:i], p.chans[i+1:]...)
			return
		}
	}
}

func (p *PeriodicHealthcheckCloseChans) CloseAll() {
	for _, c := range p.chans {
		close(c.closeChan)
	}
}
