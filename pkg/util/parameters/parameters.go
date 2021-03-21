package parameters

import (
	"github.com/ghodss/yaml"
	"regexp"
)

// VarMatchRegex is the regex to check if a value matches the kiosk var format
var VarMatchRegex = regexp.MustCompile("(\\$+?\\{[^\\}]+\\})")

// ExpressionVarMatchRegex is the regex to check if a value matches the kiosk var format
var ExpressionVarMatchRegex = regexp.MustCompile("^\\$\\{\\{[^\\}]+\\}\\}$")

// ReplaceVarFn defines the replace function
type ReplaceVarFn func(value string) (string, error)

// ParseString parses a given string, calls replace var on found variables and returns the replaced string
func ParseString(value string, replace ReplaceVarFn) (interface{}, error) {
	// check if expression string
	if ExpressionVarMatchRegex.MatchString(value) {
		replacedValue, err := replace(value[3 : len(value)-2])
		if err != nil {
			return "", err
		}

		var obj interface{}
		err = yaml.Unmarshal([]byte(replacedValue), &obj)
		if err != nil {
			return "", err
		}

		return obj, nil
	}

	matches := VarMatchRegex.FindAllStringIndex(value, -1)

	// No vars found
	if len(matches) == 0 {
		return value, nil
	}

	newValue := value[:matches[0][0]]
	for index, match := range matches {
		var (
			matchStr    = value[match[0]:match[1]]
			newMatchStr string
		)

		if matchStr[0] == '$' && matchStr[1] == '$' {
			newMatchStr = matchStr[1:]
		} else {
			offset := 2
			replacedValue, err := replace(matchStr[offset : len(matchStr)-1])
			if err != nil {
				return "", err
			}

			newMatchStr = replacedValue
		}

		newValue += newMatchStr
		if index+1 >= len(matches) {
			newValue += value[match[1]:]
		} else {
			newValue += value[match[1]:matches[index+1][0]]
		}
	}

	return newValue, nil
}
