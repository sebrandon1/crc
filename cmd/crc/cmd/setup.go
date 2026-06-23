package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	crcConfig "github.com/crc-org/crc/v2/pkg/crc/config"
	"github.com/crc-org/crc/v2/pkg/crc/constants"
	crcErrors "github.com/crc-org/crc/v2/pkg/crc/errors"
	"github.com/crc-org/crc/v2/pkg/crc/input"
	"github.com/crc-org/crc/v2/pkg/crc/preflight"
	"github.com/crc-org/crc/v2/pkg/crc/validation"
	crcTerminal "github.com/crc-org/crc/v2/pkg/os/terminal"

	"github.com/spf13/cobra"
	"k8s.io/client-go/util/exec"
)

var (
	checkOnly             bool
	forceShowProgressbars bool
)

func init() {
	setupCmd.Flags().Bool(crcConfig.ExperimentalFeatures, false, "Allow the use of experimental features")
	setupCmd.Flags().StringP(crcConfig.Bundle, "b", constants.GetDefaultBundlePath(crcConfig.GetPreset(config)), crcConfig.BundleHelpMsg(config))
	setupCmd.Flags().BoolVar(&checkOnly, "check-only", false, "Only run the preflight checks, don't try to fix any misconfiguration")
	setupCmd.Flags().BoolVar(&forceShowProgressbars, "show-progressbars", false, "Always show the progress bars for download and extraction")
	addOutputFormatFlag(setupCmd)
	rootCmd.AddCommand(setupCmd)
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up prerequisites for using CRC",
	Long:  "Set up local virtualization and networking infrastructure for using CRC",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := viper.BindFlagSet(cmd.Flags()); err != nil {
			return err
		}
		return runSetup(args)
	},
}

func runSetup(_ []string) error {
	if config.Get(crcConfig.ConsentTelemetry).AsString() == "" {
		fmt.Println("CRC is constantly improving and we would like to know more about usage (more details at https://developers.redhat.com/article/tool-data-collection)")
		fmt.Println("Your preference can be changed manually if desired using 'crc config set consent-telemetry <yes/no>'")
		if input.PromptUserForYesOrNo("Would you like to contribute anonymous usage statistics", false) {
			if _, err := config.Set(crcConfig.ConsentTelemetry, "yes"); err != nil {
				return err
			}
			fmt.Printf("Thanks for helping us! You can disable telemetry with the command 'crc config set %s no'.\n", crcConfig.ConsentTelemetry)
		} else {
			if _, err := config.Set(crcConfig.ConsentTelemetry, "no"); err != nil {
				return err
			}
			fmt.Printf("No worry, you can still enable telemetry manually with the command 'crc config set %s yes'.\n", crcConfig.ConsentTelemetry)
		}
	}

	if err := validation.ValidateBundle(config.Get(crcConfig.Bundle).AsString(), crcConfig.GetPreset(config)); err != nil {
		return err
	}

	if checkOnly {
		return runCheckOnly()
	}

	// set global variable to force terminal output
	crcTerminal.ForceShowOutput = forceShowProgressbars
	err := preflight.SetupHost(config, false)

	return render(&setupResult{
		Success: err == nil,
		Error:   crcErrors.ToSerializableError(err),
	}, os.Stdout, outputFormat)
}

func runCheckOnly() error {
	results := preflight.CheckHost(config)

	var passed, failed, skipped int
	for _, r := range results {
		switch r.Status {
		case preflight.StatusPassed:
			passed++
		case preflight.StatusFailed:
			failed++
		case preflight.StatusSkipped:
			skipped++
		}
	}

	result := &checkOnlyResult{
		Success: failed == 0,
		Checks:  results,
		Summary: checkSummary{
			Total:   len(results),
			Passed:  passed,
			Failed:  failed,
			Skipped: skipped,
		},
	}

	if err := render(result, os.Stdout, outputFormat); err != nil {
		return err
	}
	if failed > 0 {
		return exec.CodeExitError{
			Err:  fmt.Errorf("%d preflight check(s) failed", failed),
			Code: preflightFailedExitCode,
		}
	}
	return nil
}

type checkOnlyResult struct {
	Success bool                    `json:"success"`
	Checks  []preflight.CheckResult `json:"checks"`
	Summary checkSummary            `json:"summary"`
}

type checkSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

func (c *checkOnlyResult) prettyPrintTo(writer io.Writer) error {
	w := tabwriter.NewWriter(writer, 0, 0, 2, ' ', 0)
	for _, r := range c.Checks {
		fmt.Fprintf(w, "%s\t%s\n", r.Status.Label(), r.Description)
		if r.Status == preflight.StatusFailed {
			fmt.Fprintf(w, "\t  Error: %s\n", r.Error)
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(writer, "\nResults: %d passed, %d failed, %d skipped\n",
		c.Summary.Passed, c.Summary.Failed, c.Summary.Skipped)
	return nil
}

type setupResult struct {
	Success bool                         `json:"success"`
	Error   *crcErrors.SerializableError `json:"error,omitempty"`
}

func (s *setupResult) prettyPrintTo(writer io.Writer) error {
	if s.Error != nil {
		return s.Error
	}
	_, err := fmt.Fprintln(writer, "Your system is correctly setup for using CRC. "+
		"Use 'crc start' to start the instance")
	return err
}
