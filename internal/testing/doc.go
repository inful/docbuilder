// Package testing contains integration and helper utilities used across tests.
package testing

const (
	// File permission constants for test files and directories

	// testDirPermissions is the permission mode for creating test directories.
	testDirPermissions = 0o750

	// testFilePermissions is the permission mode for creating test files.
	testFilePermissions = 0o600

	// testDefaultPort is the default port number for test servers.
	testDefaultPort = 8080

	// testDefaultTimeout is the default timeout in seconds for test operations.
	testDefaultTimeout = 30

	// testDefaultConcurrency is the default concurrency for test operations.
	testDefaultConcurrency = 2

	// testDefaultRetries is the default number of retries for test operations.
	testDefaultRetries = 10
)

