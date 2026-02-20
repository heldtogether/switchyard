package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateEnvVars(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid variables",
			env: map[string]string{
				"MY_VAR":       "value1",
				"DATABASE_URL": "postgres://...",
				"API_KEY":      "secret",
			},
			wantError: false,
		},
		{
			name:      "empty map",
			env:       map[string]string{},
			wantError: false,
		},
		{
			name:      "nil map",
			env:       nil,
			wantError: false,
		},
		{
			name: "reserved prefix SWITCHYARD_",
			env: map[string]string{
				"SWITCHYARD_FOO": "bar",
			},
			wantError: true,
			errorMsg:  "SWITCHYARD_",
		},
		{
			name: "reserved prefix SWITCHYARD_JOB_ID",
			env: map[string]string{
				"SWITCHYARD_JOB_ID": "fake-id",
			},
			wantError: true,
			errorMsg:  "SWITCHYARD_",
		},
		{
			name: "multiple variables with one reserved",
			env: map[string]string{
				"MY_VAR":            "value",
				"SWITCHYARD_CUSTOM": "bad",
				"ANOTHER_VAR":       "good",
			},
			wantError: true,
			errorMsg:  "SWITCHYARD_",
		},
		{
			name: "lowercase not reserved",
			env: map[string]string{
				"switchyard_foo": "value",
			},
			wantError: false,
		},
		{
			name: "partial match not reserved",
			env: map[string]string{
				"MY_SWITCHYARD_VAR": "value",
			},
			wantError: false,
		},
		{
			name: "underscore matters",
			env: map[string]string{
				"SWITCHYARDFOO": "value", // no underscore
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvVars(tt.env)

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
