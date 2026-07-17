package main

import (
	"encoding/json"
	"net/http"
	"time"
)

// Server holds the wiring shared by the handlers.
type Server struct {
	incus       *Incus
	authz       *Authorizer
	imageServer string
}

// createBody is the create request. Only a name and an image are accepted: the
// box's shape is the admin's project profile, so no profile or config knobs are
// exposed here.
type createBody struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

// containerView is the provisioner's list element: the inventory a tenant sees.
type containerView struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Type      string `json:"type"`
	Location  string `json:"location"`
	Project   string `json:"project"`
	IPv4      string `json:"ipv4"`
	CreatedAt string `json:"created_at"`
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	project, ok := s.authz.projectFor(identityFromHeaders(r))
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "caller has no tenant group")
		return
	}
	recs, err := s.incus.List(project)
	if err != nil {
		writeIncusError(w, err)
		return
	}
	out := make([]containerView, 0, len(recs))
	for _, rec := range recs {
		out = append(out, containerView{
			Name:      rec.Name,
			Status:    rec.Status,
			Type:      rec.Type,
			Location:  rec.Location,
			Project:   rec.Project,
			IPv4:      rec.ipv4(),
			CreatedAt: rec.CreatedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	id := identityFromHeaders(r)
	project, ok := s.authz.projectFor(id)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "caller has no tenant group")
		return
	}
	var body createBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", "invalid JSON body")
		return
	}
	if body.Name == "" || body.Image == "" {
		writeError(w, http.StatusBadRequest, "invalid_argument", "name and image are required")
		return
	}
	// Fire and return: the provisioner does not wait for the box to come up. No
	// profiles are sent, so Incus applies the project's default profile, the
	// admin-owned envelope.
	err := s.incus.Create(project, createReq{
		Name: body.Name,
		Type: "container",
		Source: imageSource{
			Type:     "image",
			Alias:    body.Image,
			Server:   s.imageServer,
			Protocol: "simplestreams",
		},
	})
	auditMutation(id, project, "create", body.Name, err)
	if err != nil {
		writeIncusError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := identityFromHeaders(r)
	project, ok := s.authz.projectFor(id)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "caller has no tenant group")
		return
	}
	// Incus rejects deleting a running instance, so the tenant stops it first via
	// the stop verb. One verb, one Incus operation.
	name := r.PathValue("name")
	err := s.incus.Delete(project, name)
	auditMutation(id, project, "delete", name, err)
	if err != nil {
		writeIncusError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) { s.changeState(w, r, "start") }

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) { s.changeState(w, r, "stop") }

func (s *Server) changeState(w http.ResponseWriter, r *http.Request, verb string) {
	id := identityFromHeaders(r)
	project, ok := s.authz.projectFor(id)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "caller has no tenant group")
		return
	}
	// stop is forced so a wedged box can always be stopped, and start needs no force.
	name := r.PathValue("name")
	err := s.incus.SetState(project, name, verb, verb == "stop")
	auditMutation(id, project, verb, name, err)
	if err != nil {
		writeIncusError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
