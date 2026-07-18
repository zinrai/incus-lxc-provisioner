package main

// Backend is the seam between the handlers and where instances live: Incus over
// the local socket, or Memory for development without a cluster. Both return the
// same containerView, so the API contract is defined once and does not drift.
type Backend interface {
	List(project string) ([]containerView, error)
	Create(project, name, image string) error
	Delete(project, name string) error
	SetState(project, name, action string, force bool) error
}
