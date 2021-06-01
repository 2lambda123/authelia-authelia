package validator

import (
	"errors"
	"fmt"

	"github.com/authelia/authelia/internal/configuration/schema"
	"github.com/authelia/authelia/internal/utils"
)

// ValidateKeys determines if a provided key is valid.
func ValidateKeys(validator *schema.StructValidator, keys []string) {
	var errStrings []string

	for _, key := range keys {
		if utils.IsStringInSlice(key, ignoredKeys) {
			continue
		}

		if utils.IsStringInSlice(key, ValidKeys) {
			continue
		}

		if IsSecretKey(key) {
			continue
		}

		if newKey, ok := replacedKeys[key]; ok {
			validator.Push(fmt.Errorf(errFmtReplacedConfigurationKey, key, newKey))
			continue
		}

		if err, ok := specificErrorKeys[key]; ok {
			if !utils.IsStringInSlice(err, errStrings) {
				errStrings = append(errStrings, err)
			}
		} else {
			validator.Push(fmt.Errorf("config key not expected: %s", key))
		}
	}

	for _, err := range errStrings {
		validator.Push(errors.New(err))
	}
}
