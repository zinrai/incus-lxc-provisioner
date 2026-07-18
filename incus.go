package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Incus is a thin REST client for the Incus HTTP API over the local unix socket.
// All operations are fire-and-forget: an accepted async request returns nil, and
// completion is observed through List.
type Incus struct {
	hc          *http.Client
	imageServer string
}

func NewIncus(socket, imageServer string) *Incus {
	return &Incus{
		imageServer: imageServer,
		hc: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", socket)
				},
			},
		},
	}
}

// envelope is the standard Incus response wrapper (sync / async / error).
type envelope struct {
	Type      string          `json:"type"`
	ErrorCode int             `json:"error_code"`
	Error     string          `json:"error"`
	Metadata  json.RawMessage `json:"metadata"`
}

// do issues one request to the Incus daemon over the unix socket. The host in
// the URL is a placeholder, and the transport dials the socket regardless. An
// Incus "error" envelope becomes *apiError. A transport failure is returned as-is.
func (c *Incus) do(method, path string, body any) (*envelope, error) {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, "http://incus"+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "incus-lxc-provisioner")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("decode incus response: %w", err)
	}
	if env.Type == "error" {
		return nil, &apiError{
			Status:  httpStatusFor(env.ErrorCode),
			Err:     errIDFor(env.ErrorCode),
			Message: env.Error,
		}
	}
	return &env, nil
}

func projQuery(project string) string {
	return "?project=" + url.QueryEscape(project)
}

// createReq is the subset of Incus InstancesPost we send: name, type and image
// source, with no profiles or config.
type createReq struct {
	Name   string      `json:"name"`
	Type   string      `json:"type"`
	Source imageSource `json:"source"`
}

type imageSource struct {
	Type     string `json:"type"`
	Alias    string `json:"alias"`
	Server   string `json:"server"`
	Protocol string `json:"protocol"`
}

type statePut struct {
	Action  string `json:"action"`
	Timeout int    `json:"timeout"`
	Force   bool   `json:"force"`
}

func (c *Incus) Create(project, name, image string) error {
	r := createReq{
		Name: name,
		Type: "container",
		Source: imageSource{
			Type:     "image",
			Alias:    image,
			Server:   c.imageServer,
			Protocol: "simplestreams",
		},
	}
	_, err := c.do(http.MethodPost, "/1.0/instances"+projQuery(project), r)
	return err
}

func (c *Incus) Delete(project, name string) error {
	_, err := c.do(http.MethodDelete, "/1.0/instances/"+url.PathEscape(name)+projQuery(project), nil)
	return err
}

func (c *Incus) SetState(project, name, action string, force bool) error {
	body := statePut{Action: action, Timeout: 30, Force: force}
	_, err := c.do(http.MethodPut, "/1.0/instances/"+url.PathEscape(name)+"/state"+projQuery(project), body)
	return err
}

func (c *Incus) List(project string) ([]containerView, error) {
	env, err := c.do(http.MethodGet, "/1.0/instances"+projQuery(project)+"&recursion=2", nil)
	if err != nil {
		return nil, err
	}
	var recs []instanceRecord
	if err := json.Unmarshal(env.Metadata, &recs); err != nil {
		return nil, fmt.Errorf("decode instances: %w", err)
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
	return out, nil
}

// instanceRecord is the subset of a recursion=2 instance record we expose.
// Config is intentionally not read back.
type instanceRecord struct {
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	Type      string         `json:"type"`
	Location  string         `json:"location"`
	Project   string         `json:"project"`
	CreatedAt time.Time      `json:"created_at"`
	State     *instanceState `json:"state"`
}

type instanceState struct {
	Network map[string]struct {
		Addresses []struct {
			Family  string `json:"family"`
			Address string `json:"address"`
			Scope   string `json:"scope"`
		} `json:"addresses"`
	} `json:"network"`
}

// ipv4 returns the first global IPv4 address outside loopback, or "".
func (r instanceRecord) ipv4() string {
	if r.State == nil {
		return ""
	}
	for name, n := range r.State.Network {
		if name == "lo" {
			continue
		}
		for _, a := range n.Addresses {
			if a.Family == "inet" && a.Scope == "global" {
				return a.Address
			}
		}
	}
	return ""
}

func httpStatusFor(code int) int {
	if code >= 400 && code <= 599 {
		return code
	}
	return http.StatusBadGateway
}

func errIDFor(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "invalid_argument"
	case http.StatusUnauthorized:
		return "unauthenticated"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		return "incus_error"
	}
}
