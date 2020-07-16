package help

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/jessevdk/go-flags"
	"github.com/raedahgroup/dcrextdata/app"
)

//TabWriter creates a tabwriter object that writes tab-aligned text.
func TabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 1, ' ', tabwriter.TabIndent)
}

// StdoutWriter writes tab-aligned text to os.Stdout
var StdoutWriter = TabWriter(os.Stdout)

// printOptionGroups checks if the root parser option group has nested option groups and prints all
func printOptionGroups(output io.Writer, groups []*flags.Group) {
	for _, optionGroup := range groups {
		if len(optionGroup.Groups()) > 0 {
			printOptionGroups(output, optionGroup.Groups())
		} else {
			printOptions(output, optionGroup.ShortDescription, optionGroup.Options())
		}
	}
}

// printOptions adds 2 trailing whitespace for options with short name and 6 for those without
// This is an attempt to stay consistent with the output of parser.WriteHelp
func printOptions(tabWriter io.Writer, optionDescription string, options []*flags.Option) {
	if len(options) > 0 {
		fmt.Fprintln(tabWriter, optionDescription)
		// check if there's any option in this group with short and long name
		// this will help to decide whether or not to pad options without short name to maintain readability
		if optionDescription == "Command-Line options:" {
			fmt.Fprintf(tabWriter, "   %s [command] \n\n", app.AppName)
		}

		var hasOptionsWithShortName bool
		for _, option := range options {
			if option.ShortName != 0 && option.LongName != "" {
				hasOptionsWithShortName = true
				break
			}
		}
		var optionUsage string
		for _, option := range options {
			if optionDescription == "Command-Line options:" {
				optionUsage = parseOptionUsageTextDash(option, hasOptionsWithShortName)

			} else {
				optionUsage = parseOptionUsageText(option, hasOptionsWithShortName)
			}
			description := parseOptionDescription(option)
			fmt.Fprintf(tabWriter, "  %s \t %s\n", optionUsage, description)
		}

		fmt.Fprintln(tabWriter)
	}
}

func parseOptionUsageText(option *flags.Option, hasOptionsWithShortName bool) (optionUsage string) {
	if option.ShortName != 0 && option.LongName != "" {
		optionUsage = fmt.Sprintf("/%c, /%s", option.ShortName, option.LongName)
	} else if option.ShortName != 0 {
		optionUsage = fmt.Sprintf("/%c", option.ShortName)
	} else if hasOptionsWithShortName {
		// pad long name with 4 spaces to align with options having short and long names
		optionUsage = fmt.Sprintf("    /%s", option.LongName)
	} else {
		optionUsage = fmt.Sprintf("/%s", option.LongName)
	}

	if option.Field().Type.Kind() != reflect.Bool {
		optionUsage += ":"
	}

	if len(option.Choices) > 0 {
		optionUsage += fmt.Sprintf("[%s]", strings.Join(option.Choices, ","))
	}

	return
}

func parseOptionUsageTextDash(option *flags.Option, hasOptionsWithShortName bool) (optionUsage string) {
	if option.ShortName != 0 && option.LongName != "" {
		optionUsage = fmt.Sprintf("-%c, --%s", option.ShortName, option.LongName)
	} else if option.ShortName != 0 {
		optionUsage = fmt.Sprintf("-%c", option.ShortName)
	} else if hasOptionsWithShortName {
		// pad long name with 4 spaces to align with options having short and long names
		optionUsage = fmt.Sprintf("    --%s", option.LongName)
	} else {
		optionUsage = fmt.Sprintf("--%s", option.LongName)
	}

	if option.Field().Type.Kind() != reflect.Bool {
		optionUsage += ":"
	}

	if len(option.Choices) > 0 {
		optionUsage += fmt.Sprintf("[%s]", strings.Join(option.Choices, ","))
	}

	return
}

func parseOptionDescription(option *flags.Option) (description string) {
	description = option.Description
	optionDefaultValue := reflect.ValueOf(option.Value())
	if optionDefaultValue.Kind() == reflect.String && optionDefaultValue.String() != "" {
		description += fmt.Sprintf(" (default: %s)", optionDefaultValue.String())
	}
	return
}
