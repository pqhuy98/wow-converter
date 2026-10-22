//go:build integration_tests

package terrain

// External integration tests reuse the independently rasterized native fixtures.
var AssertSavedBlockers = assertSavedBlockers
