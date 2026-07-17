package main

import (
	"encoding/json"
	"os"
	"time"
)

// auditRecord is one audit line linking the SSO identity, tenant project, verb
// and result. Only mutating verbs are audited.
type auditRecord struct {
	TS      string   `json:"ts"`
	Email   string   `json:"email,omitempty"`
	Groups  []string `json:"groups,omitempty"`
	Project string   `json:"project"`
	Verb    string   `json:"verb"`
	Name    string   `json:"name"`
	Result  string   `json:"result"`
}

var auditOut = json.NewEncoder(os.Stdout)

func auditMutation(id Identity, project, verb, name string, err error) {
	result := "accepted"
	if err != nil {
		result = "rejected"
	}
	_ = auditOut.Encode(auditRecord{
		TS:      time.Now().UTC().Format(time.RFC3339),
		Email:   id.Email,
		Groups:  id.Groups,
		Project: project,
		Verb:    verb,
		Name:    name,
		Result:  result,
	})
}
