package client

import (
	"errors"
	"net/http"
	"testing"
)

func TestCharMetaRESTErrorPreservesIdentityAndMessage(t *testing.T) {
	err := charMetaRESTError(http.StatusInternalServerError, map[string]any{
		"id":      "ERR_INTERNAL",
		"message": "invalid race",
	})
	var restErr *RESTError
	if !errors.As(err, &restErr) {
		t.Fatalf("expected RESTError, got %T", err)
	}
	if restErr.ID != "ERR_INTERNAL" {
		t.Fatalf("unexpected error ID: %q", restErr.ID)
	}
	if got := err.Error(); got != "Server error during character metadata lookup: invalid race" {
		t.Fatalf("unexpected message: %q", got)
	}
}

func TestBuildsRESTErrorUsesServerDetail(t *testing.T) {
	err := buildsRESTError(map[string]any{
		"id":      "ERR_INVALID_INSTALL",
		"message": "Data directory is missing",
	}, true)
	var restErr *RESTError
	if !errors.As(err, &restErr) || restErr.ID != "ERR_INVALID_INSTALL" {
		t.Fatalf("expected typed invalid-install error, got %v", err)
	}
	if got := err.Error(); got != "Data directory is missing" {
		t.Fatalf("unexpected message: %q", got)
	}
}

func TestBuildsRESTErrorDistinguishesLocalAndRemoteDefaults(t *testing.T) {
	local := buildsRESTError(map[string]any{"id": "ERR_INVALID_INSTALL"}, true)
	remote := buildsRESTError(map[string]any{"id": "ERR_INVALID_INSTALL"}, false)
	if local.Error() != "Invalid WoW installation directory" {
		t.Fatalf("unexpected local message: %q", local)
	}
	if remote.Error() != "Invalid CDN region" {
		t.Fatalf("unexpected remote message: %q", remote)
	}
}
