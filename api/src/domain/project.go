package domain

import "time"

// Project is a target repository. Dir is its directory name under the projects directory.
type Project struct {
	ID            int64     `json:"id"`
	Dir           string    `json:"dir"`
	Owner         string    `json:"owner"`
	Name          string    `json:"name"`
	DefaultBranch string    `json:"default_branch"`
	CreatedAt     time.Time `json:"created_at"`
}

// FullName is the GitHub "owner/name".
func (p Project) FullName() string {
	return p.Owner + "/" + p.Name
}
