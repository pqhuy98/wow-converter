package casc

import "errors"

var (
	errInvalidCDNConfig      = errors.New("invalid CDN config: unexpected start of config")
	errInvalidCDNToken       = errors.New("invalid token encountered parsing CDN config")
	errInvalidInstallManifest = errors.New("invalid file signature for install manifest")
	errMissingListfileURL     = errors.New("missing/malformed listfileURL in configuration")
)
