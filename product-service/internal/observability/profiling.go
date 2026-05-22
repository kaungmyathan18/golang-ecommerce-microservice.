package observability

import (
	"os"

	"github.com/grafana/pyroscope-go"
)

// MaybeStartPyroscope starts continuous profiling when PYROSCOPE_SERVER_ADDRESS is set.
// It returns a no-op stop function when the env var is empty.
func MaybeStartPyroscope(applicationName string) (stop func(), err error) {
	stop = func() {}
	addr := os.Getenv("PYROSCOPE_SERVER_ADDRESS")
	if addr == "" {
		return stop, nil
	}
	p, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: applicationName,
		ServerAddress:   addr,
	})
	if err != nil {
		return nil, err
	}
	return func() { _ = p.Stop() }, nil
}
