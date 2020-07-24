package helpers

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/planetdecred/dcrextdata/app/help"
)

// ValidatorFunction  validates the input string according to its custom logic.
type ValidatorFunction func(string) error

// getTextInput - Prompt for text input.
func getTextInput(prompt string) (string, error) {
	// printing the prompt with tabWriter to ensure adequate formatting of tabulated list of options
	tabWriter := help.StdoutWriter
	fmt.Fprint(tabWriter, prompt)
	tabWriter.Flush()

	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')

	if err != nil {
		return "", err
	}

	text = strings.TrimSuffix(text, "\n")
	text = strings.TrimSuffix(text, "\r")

	return text, nil
}

func skipEOFError(value string, err error) (string, error) {
	switch err {
	case io.EOF:
		return "", nil
	case nil:
		return value, nil
	default:
		return "", err
	}
}

// RequestInput requests input from the user.
// If an error other than EOF occurs while requesting input, the error is returned.
// It calls `validate` on the received input. If `validate` returns an error, the user is prompted
// again for a correct input.
func RequestInput(message string, validate ValidatorFunction) (string, error) {
	for {
		value, err := skipEOFError(getTextInput(fmt.Sprintf("%s: ", message)))
		if err != nil {
			return "", err
		}

		value = strings.TrimSpace(value)
		if err = validate(value); err != nil {
			fmt.Println(strings.TrimSpace(err.Error()))
			continue
		}
		return value, nil
	}
}

func RequestYesNoConfirmation(message, defaultOption string) (bool, error) {
	isYesOption := func(option string) bool {
		return strings.EqualFold(option, "y") || strings.EqualFold(option, "yes")
	}
	isNoOption := func(option string) bool {
		return strings.EqualFold(option, "n") || strings.EqualFold(option, "no")
	}

	validateUserResponse := func(userResponse string) error {
		if defaultOption != "" && userResponse == "" {
			return nil
		}
		if isYesOption(userResponse) || isNoOption(userResponse) {
			return nil
		}
		return fmt.Errorf("Invalid option, try again")
	}

	var options string
	if isYesOption(defaultOption) {
		options = "Y/n"
	} else if isNoOption(defaultOption) {
		options = "y/N"
	} else {
		options = "y/n"
		defaultOption = ""
	}

	// append options to message for display
	message = fmt.Sprintf("%s (%s)", message, options)
	userResponse, err := RequestInput(message, validateUserResponse)
	if err != nil {
		return false, err
	}

	userResponse = strings.TrimSpace(userResponse)
	if userResponse == "" {
		userResponse = defaultOption
	}

	return isYesOption(userResponse), nil
}
