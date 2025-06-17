package main

import (
	"os"
	"runtime/debug"

	"github.com/grafana/pyroscope-go"
	"github.com/internetarchive/doppelganger/cmd"
)

var Commit = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}

	return ""
}()

func main() {
	if pyroscopeAddr := os.Getenv("PYROSCOPE_ADDRESS"); pyroscopeAddr != "" {
		pyroscope.Start(pyroscope.Config{
			ApplicationName: "doppelganger",
			Tags: map[string]string{
				"version": Commit,
			},
			ServerAddress: pyroscopeAddr,
			Logger:        pyroscope.StandardLogger,
		})
	}
	cmd.Execute()
}
