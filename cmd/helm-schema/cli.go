package main

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func possibleLogLevels() []string {
	levels := make([]string, 0)

	for _, l := range log.AllLevels {
		levels = append(levels, l.String())
	}

	return levels
}

func configureLogging() {
	logLevelName := viper.GetString("log-level")
	logLevel, err := log.ParseLevel(logLevelName)
	if err != nil {
		log.Errorf("Failed to parse provided log level %s: %s", logLevelName, err)
		os.Exit(1)
	}

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.SetLevel(logLevel)
}

func newCommand(run func(cmd *cobra.Command, args []string) error) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:           "helm-schema",
		Short:         "helm-schema automatically generates a jsonschema file for helm charts from values files",
		Version:       version,
		RunE:          run,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	logLevelUsage := fmt.Sprintf(
		"Level of logs that should printed, one of (%s)",
		strings.Join(possibleLogLevels(), ", "),
	)
	cmd.PersistentFlags().
		StringP("chart-search-root", "c", ".", "directory to search recursively within for charts")
	cmd.PersistentFlags().
		BoolP("dry-run", "d", false, "don't actually create files just print to stdout passed")
	cmd.PersistentFlags().
		BoolP("keep-full-comment", "s", false, "Keep the whole leading comment (default: cut at empty line)")
	cmd.PersistentFlags().
		BoolP("dont-strip-helm-docs-prefix", "x", false, "Disable the removal of the helm-docs prefix (--)")
	cmd.PersistentFlags().
		BoolP("no-dependencies", "n", false, "don't analyze dependencies")
	cmd.PersistentFlags().StringP("log-level", "l", "info", logLevelUsage)
	cmd.PersistentFlags().
		StringSliceP("value-files", "f", []string{"values.yaml"}, "filenames to check for chart values")
	cmd.PersistentFlags().
		StringP("output-file", "o", "values.schema.json", "jsonschema file path relative to each chart directory to which jsonschema will be written")
	cmd.PersistentFlags().
		BoolP("omit-additional-properties", "a", false, "do not set \"additionalProperties\": false by default")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("HELM_SCHEMA")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	err := viper.BindPFlags(cmd.PersistentFlags())

	return cmd, err
}
