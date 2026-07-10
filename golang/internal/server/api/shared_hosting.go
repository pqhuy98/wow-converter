package api

import "errors"

const desktopOnlyMessage = "This feature is only available in the desktop app, not on shared hosting."

func assertDesktopOnly(isSharedHosting bool) error {
	if isSharedHosting {
		return errors.New(desktopOnlyMessage)
	}
	return nil
}
