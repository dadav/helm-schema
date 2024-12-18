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
		"level of logs that should printed, one of (%s)",
		strings.Join(possibleLogLevels(), ", "),
	)
	cmd.PersistentFlags().
		StringP("chart-search-root", "c", ".", "directory to search recursively within for charts")
	cmd.PersistentFlags().
		BoolP("dry-run", "d", false, "don't actually create files just print to stdout passed")
	cmd.PersistentFlags().
		BoolP("append-newline", "a", false, "append newline to generated jsonschema at the end of the file")
	cmd.PersistentFlags().
		BoolP("keep-full-comment", "s", false, "keep the whole leading comment (default: cut at empty line)")
	cmd.PersistentFlags().
		BoolP("uncomment", "u", false, "consider yaml which is commented out")
	cmd.PersistentFlags().
		BoolP("helm-docs-compatibility-mode", "p", false, "parse and use helm-docs comments")
	cmd.PersistentFlags().
		BoolP("dont-strip-helm-docs-prefix", "x", false, "disable the removal of the helm-docs prefix (--)")
	cmd.PersistentFlags().
		BoolP("no-dependencies", "n", false, "don't analyze dependencies")
	cmd.PersistentFlags().
		BoolP("add-schema-reference", "r", false, "add reference to schema in values.yaml if not found")
	cmd.PersistentFlags().StringP("log-level", "l", "info", logLevelUsage)
	cmd.PersistentFlags().
		StringSliceP("value-files", "f", []string{"values.yaml"}, "filenames to check for chart values")
	cmd.PersistentFlags().
		StringP("output-file", "o", "values.schema.json", "jsonschema file path relative to each chart directory to which jsonschema will be written")
	cmd.PersistentFlags().
		StringSliceP("skip-auto-generation", "k", []string{}, "comma separated list of fields to skip from being created by default (possible: title, description, required, default, additionalProperties)")
	cmd.PersistentFlags().
		StringSliceP("dependencies-filter", "i", []string{}, "only generate schema for specified dependencies (comma-separated list of dependency names)")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("HELM_SCHEMA")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	err := viper.BindPFlags(cmd.PersistentFlags())

	return cmd, err
}
