package server

import (
	"bytes"
	"encoding/json"
	"regexp"
)

const (
	cloudPrefixEmoji = "☁ "
	cloudPrefixText  = ":cloud: "

	variableNameMaxLength = 1024
	valueMaxLength        = 100000

	usernameMinLength = 1
	usernameMaxLength = 20

	roomIDMaxLength = 1000
)

var (
	cloudPrefixes  = []string{cloudPrefixEmoji, cloudPrefixText}
	usernameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

func isValidUsername(username string) bool {
	if len(username) < usernameMinLength || len(username) > usernameMaxLength {
		return false
	}
	return usernameRegexp.MatchString(username)
}

func isValidRoomID(id string) bool {
	return len(id) > 0 && len(id) < roomIDMaxLength
}

// isValidVariableName requires the name to start with one of the recognized
// cloud variable prefixes and to contain something beyond just the prefix.
func isValidVariableName(name string) bool {
	if len(name) > variableNameMaxLength {
		return false
	}
	for _, prefix := range cloudPrefixes {
		if name == prefix {
			return false
		}
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// isValidVariableValue checks a raw JSON value against
// the cloud variable value rules. Numeric strings or JSON numbers, bounded in length.
func isValidVariableValue(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}

	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return false
		}
		return isValidVariableValueString(s)
	}

	var f float64
	if err := json.Unmarshal(trimmed, &f); err != nil {
		return false
	}
	return len(trimmed) <= valueMaxLength
}

func isValidVariableValueString(s string) bool {
	if len(s) > valueMaxLength {
		return false
	}
	if s == "." || s == "-" {
		return false
	}

	length := len(s)
	seenDecimal := false
	i := 0
	if length > 0 && s[0] == '-' {
		i = 1
	}
	for ; i < length; i++ {
		c := s[i]
		if c == '.' {
			if seenDecimal {
				return false
			}
			seenDecimal = true
		} else if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
