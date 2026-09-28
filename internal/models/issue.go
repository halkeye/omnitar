package models

type Issue struct {
	Source string `json:"source"`

	Title          string            `json:"title"`
	URL            string            `json:"url"`
	Key            string            `json:"key"`
	IssueType      string            `json:"issue_type"`
	IssueTypeIcon  string            `json:"issue_type_icon"`
	Reporter       string            `json:"reporter"`
	ReporterEmail  string            `json:"reporter_email"`
	ReporterIcon   string            `json:"reporter_icon"`
	Assignee       string            `json:"assignee"`
	AssigneeEmail  string            `json:"assignee_email"`
	AssigneeIcon   string            `json:"assignee_icon"`
	Status         string            `json:"status"`
	StatusCategory string            `json:"status_category"`
	StatusColor    string            `json:"status_color"`
	Priority       string            `json:"priority"`
	PriorityIcon   string            `json:"priority_icon"`
	Fields         map[string]string `json:"fields"`
}
