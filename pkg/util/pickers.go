package util

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"
	"gopkg.in/AlecAivazis/survey.v1"
)

// PickValue gets an answer to a prompt from a user's free-form input
func PickValue(message string, defaultValue string, required bool, help string, handles IOFileHandles) (string, error) {
	answer := ""
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
		Help:    help,
	}
	validator := survey.Required
	if !required {
		validator = nil
	}
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	err := survey.AskOne(prompt, &answer, validator, surveyOpts)
	if err != nil {
		return "", err
	}
	return answer, nil
}

// PickPassword gets a password (via hidden input) from a user's free-form input
func PickPassword(message string, help string, handles IOFileHandles) (string, error) {
	answer := ""
	prompt := &survey.Password{
		Message: message,
		Help:    help,
	}
	validator := survey.Required
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	err := survey.AskOne(prompt, &answer, validator, surveyOpts)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(answer), nil
}

// PickNameWithDefault gets the user to pick an option from a list of options, with a default option specified
func PickNameWithDefault(names []string, message string, defaultValue string, help string, handles IOFileHandles) (string, error) {
	name := ""
	if len(names) == 0 {
		return "", nil
	} else if len(names) == 1 {
		name = names[0]
	} else {
		prompt := &survey.Select{
			Message: message,
			Options: names,
			Default: defaultValue,
		}
		surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
		err := survey.AskOne(prompt, &name, nil, surveyOpts)
		if err != nil {
			return "", err
		}
	}
	return name, nil
}

// PickRequiredNameWithDefault gets the user to pick an option from a list of options, with a default option specified
func PickRequiredNameWithDefault(names []string, message string, defaultValue string, help string, handles IOFileHandles) (string, error) {
	name := ""
	if len(names) == 0 {
		return "", nil
	} else if len(names) == 1 {
		name = names[0]
	} else {
		prompt := &survey.Select{
			Message: message,
			Options: names,
			Default: defaultValue,
			Help:    help,
		}
		surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
		err := survey.AskOne(prompt, &name, survey.Required, surveyOpts)
		if err != nil {
			return "", err
		}
	}
	return name, nil
}

// PickName gets the user to pick an option from a list of options
func PickName(names []string, message string, help string, handles IOFileHandles) (string, error) {
	return PickNameWithDefault(names, message, "", help, handles)
}

// PickNames gets the user to pick multiple selections from a list of options
func PickNames(names []string, message string, help string, handles IOFileHandles) ([]string, error) {
	return PickNamesWithDefaults(names, nil, message, help, handles)
}

// PickNamesWithDefaults gets the user to pick multiple selections from a list of options with a set of default selections
func PickNamesWithDefaults(names []string, defaults []string, message string, help string, handles IOFileHandles) ([]string, error) {
	picked := []string{}
	if len(names) == 0 {
		return picked, nil
	} else if len(names) == 1 {
		return names, nil
	} else {
		prompt := &survey.MultiSelect{
			Message: message,
			Options: names,
			Default: defaults,
			Help:    help,
		}
		surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
		err := survey.AskOne(prompt, &picked, nil, surveyOpts)
		if err != nil {
			return picked, err
		}
	}
	return picked, nil
}

// SelectNamesWithFilter selects from a list of names with a given filter. Optionally selecting them all
func SelectNamesWithFilter(names []string, message string, selectAll bool, filter string, help string, handles IOFileHandles) ([]string, error) {
	filtered := []string{}
	for _, name := range names {
		if filter == "" || strings.Index(name, filter) >= 0 {
			filtered = append(filtered, name)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("No names match filter: %s", filter)
	}
	return SelectNames(filtered, message, selectAll, help, handles)
}

// SelectNames select which names from the list should be chosen
func SelectNames(names []string, message string, selectAll bool, help string, handles IOFileHandles) ([]string, error) {
	answer := []string{}
	if len(names) == 0 {
		return answer, fmt.Errorf("No names to choose from")
	}
	sort.Strings(names)

	prompt := &survey.MultiSelect{
		Message: message,
		Options: names,
		Help:    help,
	}
	if selectAll {
		prompt.Default = names
	}
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	err := survey.AskOne(prompt, &answer, nil, surveyOpts)
	return answer, err
}

// Confirm prompts the user to confirm something
func Confirm(message string, defaultValue bool, help string, handles IOFileHandles) (bool, error) {
	answer := defaultValue
	prompt := &survey.Confirm{
		Message: message,
		Default: defaultValue,
		Help:    help,
	}
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	err := survey.AskOne(prompt, &answer, nil, surveyOpts)
	if err != nil {
		return false, err
	}
	log.Blank()
	return answer, nil
}
