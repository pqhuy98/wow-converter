package client

import (
	"fmt"
	"net/http"
)

// RESTError preserves the wow-data-server error identity across HTTP and
// in-process transports.
type RESTError struct {
	Status  int
	ID      string
	Message string
}

func (e *RESTError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.ID != "" {
		return e.ID
	}
	return fmt.Sprintf("wow-data-server request failed with status %d", e.Status)
}

func charMetaRESTError(status int, body map[string]any) error {
	id, _ := body["id"].(string)
	message, _ := body["message"].(string)
	switch {
	case status == http.StatusConflict || id == "ERR_NO_CASC":
		message = "No CASC loaded"
	case status == http.StatusBadRequest || id == "ERR_INVALID_PARAMETERS":
		message = "Invalid parameters for character metadata"
	case status >= http.StatusInternalServerError || id == "ERR_INTERNAL":
		if message == "" {
			message = "unknown"
		}
		message = "Server error during character metadata lookup: " + message
	default:
		message = "Unexpected response for character metadata"
	}
	return &RESTError{Status: status, ID: id, Message: message}
}

func buildsRESTError(body map[string]any, local bool) error {
	id, _ := body["id"].(string)
	message, _ := body["message"].(string)
	switch id {
	case "ERR_INVALID_INSTALL":
		if message == "" {
			if local {
				message = "Invalid WoW installation directory"
			} else {
				message = "Invalid CDN region"
			}
		}
	case "ERR_CASC_ACTIVE":
		message = "CASC is already active"
	default:
		message = "Failed to load CASC"
	}
	return &RESTError{ID: id, Message: message}
}
