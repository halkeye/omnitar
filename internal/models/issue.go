package models

type Issue struct {
	Title    string            `json:"title"`
	Key      string            `json:"key"`
	Type     string            `json:"type"`
	Reporter string            `json:"reporter"`
	Assignee string            `json:"assignee"`
	State    string            `json:"state"`
	Priority string            `json:"priority"`
	Fields   map[string]string `json:"fields"`
}
