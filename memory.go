package main

import (
	"net/http"
	"sort"
	"sync"
	"time"
)

// Memory is an in-memory Backend for developing and demoing a client without an
// Incus cluster. Not for production.
type Memory struct {
	mu   sync.Mutex
	data map[string]map[string]*containerView // project -> name -> view
}

func NewMemory() *Memory {
	return &Memory{data: map[string]map[string]*containerView{}}
}

func (m *Memory) List(project string) ([]containerView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []containerView{}
	for _, v := range m.data[project] {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Memory) Create(project, name, image string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[project] == nil {
		m.data[project] = map[string]*containerView{}
	}
	if m.data[project][name] != nil {
		return &apiError{Status: http.StatusConflict, Err: "conflict", Message: "instance already exists"}
	}
	m.data[project][name] = &containerView{
		Name:      name,
		Status:    "Stopped",
		Type:      "container",
		Location:  "memory",
		Project:   project,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return nil
}

func (m *Memory) Delete(project, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[project][name] == nil {
		return &apiError{Status: http.StatusNotFound, Err: "not_found", Message: "instance not found"}
	}
	delete(m.data[project], name)
	return nil
}

func (m *Memory) SetState(project, name, action string, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := m.data[project][name]
	if v == nil {
		return &apiError{Status: http.StatusNotFound, Err: "not_found", Message: "instance not found"}
	}
	if action == "start" {
		v.Status, v.IPv4 = "Running", "10.0.0.42"
	} else {
		v.Status, v.IPv4 = "Stopped", ""
	}
	return nil
}
