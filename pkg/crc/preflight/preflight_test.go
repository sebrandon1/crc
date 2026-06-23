package preflight

import (
	"errors"
	"testing"

	"github.com/crc-org/crc/v2/pkg/crc/config"
	"github.com/stretchr/testify/assert"
)

func TestCheckPreflight(t *testing.T) {
	check, calls := sampleCheck(nil, nil)
	cfg := config.New(config.NewEmptyInMemoryStorage(), config.NewEmptyInMemorySecretStorage())
	doRegisterSettings(cfg, []Check{*check})

	assert.NoError(t, doPreflightChecks(cfg, []Check{*check}))
	assert.True(t, calls.checked)
	assert.False(t, calls.fixed)
}

func TestSkipPreflight(t *testing.T) {
	check, calls := sampleCheck(nil, nil)
	cfg := config.New(config.NewEmptyInMemoryStorage(), config.NewEmptyInMemorySecretStorage())
	doRegisterSettings(cfg, []Check{*check})
	_, err := cfg.Set("skip-sample", true)
	assert.NoError(t, err)

	assert.NoError(t, doPreflightChecks(cfg, []Check{*check}))
	assert.False(t, calls.checked)
}

func TestFixPreflight(t *testing.T) {
	check, calls := sampleCheck(errors.New("check failed"), nil)
	cfg := config.New(config.NewEmptyInMemoryStorage(), config.NewEmptyInMemorySecretStorage())
	doRegisterSettings(cfg, []Check{*check})

	assert.NoError(t, doFixPreflightChecks(cfg, []Check{*check}, false))
	assert.True(t, calls.checked)
	assert.True(t, calls.fixed)
}

func TestFixPreflightCheckOnly(t *testing.T) {
	check, calls := sampleCheck(errors.New("check failed"), nil)
	cfg := config.New(config.NewEmptyInMemoryStorage(), config.NewEmptyInMemorySecretStorage())
	doRegisterSettings(cfg, []Check{*check})

	assert.Error(t, doFixPreflightChecks(cfg, []Check{*check}, true))
	assert.True(t, calls.checked)
	assert.False(t, calls.fixed)
}

func TestCheckAllPreflightChecksAllPass(t *testing.T) {
	check1, _ := sampleCheck(nil, nil)
	check2, _ := sampleCheck(nil, nil)
	check2.configKeySuffix = "sample2"
	check2.checkDescription = "Sample check 2"
	cfg := config.New(config.NewEmptyInMemoryStorage(), config.NewEmptyInMemorySecretStorage())
	doRegisterSettings(cfg, []Check{*check1, *check2})

	results := doCheckAllPreflightChecks(cfg, []Check{*check1, *check2})
	assert.Len(t, results, 2)
	assert.Equal(t, StatusPassed, results[0].Status)
	assert.Equal(t, StatusPassed, results[1].Status)
	assert.Empty(t, results[0].Error)
}

func TestCheckAllPreflightChecksContinuesAfterFailure(t *testing.T) {
	failing, _ := sampleCheck(errors.New("something broke"), nil)
	passing, calls := sampleCheck(nil, nil)
	passing.configKeySuffix = "sample2"
	passing.checkDescription = "Sample check 2"
	cfg := config.New(config.NewEmptyInMemoryStorage(), config.NewEmptyInMemorySecretStorage())
	doRegisterSettings(cfg, []Check{*failing, *passing})

	results := doCheckAllPreflightChecks(cfg, []Check{*failing, *passing})
	assert.Len(t, results, 2)
	assert.Equal(t, StatusFailed, results[0].Status)
	assert.Equal(t, "something broke", results[0].Error)
	assert.Equal(t, StatusPassed, results[1].Status)
	assert.True(t, calls.checked)
}

func TestCheckAllPreflightChecksSkipped(t *testing.T) {
	check, calls := sampleCheck(nil, nil)
	cfg := config.New(config.NewEmptyInMemoryStorage(), config.NewEmptyInMemorySecretStorage())
	doRegisterSettings(cfg, []Check{*check})
	_, err := cfg.Set("skip-sample", true)
	assert.NoError(t, err)

	results := doCheckAllPreflightChecks(cfg, []Check{*check})
	assert.Len(t, results, 1)
	assert.Equal(t, StatusSkipped, results[0].Status)
	assert.False(t, calls.checked)
}

func TestCheckAllPreflightChecksExcludesCleanUpOnly(t *testing.T) {
	check, _ := sampleCheck(nil, nil)
	cleanupCheck := &Check{
		configKeySuffix:    "cleanup-only",
		checkDescription:   "Cleanup check",
		check:              func() error { return nil },
		cleanupDescription: "sample cleanup",
		cleanup:            func() error { return nil },
		flags:              CleanUpOnly,
	}
	cfg := config.New(config.NewEmptyInMemoryStorage(), config.NewEmptyInMemorySecretStorage())
	doRegisterSettings(cfg, []Check{*check, *cleanupCheck})

	results := doCheckAllPreflightChecks(cfg, []Check{*check, *cleanupCheck})
	assert.Len(t, results, 1)
	assert.Equal(t, "sample", results[0].Name)
}

func sampleCheck(checkErr, fixErr error) (*Check, *status) {
	status := &status{}
	return &Check{
		configKeySuffix:  "sample",
		checkDescription: "Sample check",
		check: func() error {
			status.checked = true
			return checkErr
		},
		fixDescription: "sample fix",
		fix: func() error {
			status.fixed = true
			return fixErr
		},
	}, status
}

type status struct {
	checked, fixed bool
}
