package filewatcher

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/lager/v3"
	"github.com/fsnotify/fsnotify"
)

type Filewatcher interface {
	Run(signals <-chan os.Signal, ready chan<- struct{}) error
}

type filewatcher struct {
	configFilePath string
	eventChan      chan string
	logger         lager.Logger
}

func NewFileWatcher(configFilePath string, eventChan chan string, logger lager.Logger) *filewatcher {
	return &filewatcher{
		configFilePath: configFilePath,
		eventChan:      eventChan,
		logger:         logger,
	}
}

func (f *filewatcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)
	f.logger.Info("MEOW: 🍄")
	f.logger.Info(fmt.Sprintf("🍄 Config file path at top of file: %s", f.configFilePath))

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		f.logger.Error("failed to create a new watcher", err)
		return err
	}
	f.logger.Info("🍄 made a new watcher!")

	defer watcher.Close()

	err = watcher.Add(f.configFilePath)
	if err != nil {
		f.logger.Error("Watcher add error", err)
		return err
	}
	f.logger.Info("🍄 added to the watcher")

	f.eventChan <- f.configFilePath

	f.logger.Info("🍄 did the channel thing")

	f.logger.Info(fmt.Sprintf("🍄 Config file path: %s", f.configFilePath))
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				f.logger.Info(fmt.Sprintf("🍄 NOT OK Events!!: %#v", event))
				return nil
			}

			f.logger.Info(fmt.Sprintf("🍄 config change event: %#v", event))

			if isWrite(event) || isCreate(event) {
				go func() { f.eventChan <- f.configFilePath }()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}

			f.logger.Error("🍄 config watcher error", err)

		case s := <-signals:
			f.logger.Info("Caught signal", lager.Data{"signal": s})
			return nil
		}
	}
	return nil
}

func isWrite(event fsnotify.Event) bool {
	return event.Op&fsnotify.Write == fsnotify.Write
}

func isCreate(event fsnotify.Event) bool {
	return event.Op&fsnotify.Create == fsnotify.Create
}
