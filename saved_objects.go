package kibana

import (
	"errors"
	"fmt"
)

// SavedObjectsService handles communication with the saved object methods
// of the Kibana API. Saved objects belong to a space: use [Client.InSpace]
// to work with the objects of another space.
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-saved-objects
type SavedObjectsService service

// SavedObjectRef identifies a saved object by its type and ID, such as
// {Type: "dashboard", ID: "7adfa750-4c81-11e8-b3d7-01146121b73d"}.
//
// Result types that identify an object embed it, so their Type and ID fields
// are its fields.
type SavedObjectRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// validateRefs returns an error if refs is empty or has an entry without a
// type or ID.
func validateRefs(refs []SavedObjectRef) error {
	if len(refs) == 0 {
		return errors.New("kibana: objects must not be empty")
	}
	for i, r := range refs {
		if r.Type == "" || r.ID == "" {
			return fmt.Errorf("kibana: object %d must have a type and an ID", i)
		}
	}
	return nil
}

// SavedObjectMeta is the display information Kibana returns for a saved
// object.
type SavedObjectMeta struct {
	Title string `json:"title,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

// ObjectError is the error of a single object in a bulk operation that
// otherwise succeeded.
type ObjectError struct {
	StatusCode int    `json:"statusCode"`
	StatusText string `json:"error"`
	Message    string `json:"message"`
}

// Error returns the status code and message of the error. Its format is not
// part of the API and may change.
func (e *ObjectError) Error() string {
	return fmt.Sprintf("kibana: %d %s", e.StatusCode, e.Message)
}

// Is reports whether target is the sentinel error for the error's status
// code, such as [ErrNotFound] for 404 Not Found.
func (e *ObjectError) Is(target error) bool {
	return e != nil && statusIs(e.StatusCode, target)
}
