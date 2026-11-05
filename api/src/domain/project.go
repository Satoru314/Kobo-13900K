package domain

import "time"

// Project is a target repository. Dir is its directory name under the projects directory.
type Project struct {
	ID            int64
	Dir           string
	Owner         string
	Name          string
	DefaultBranch string
	CreatedAt     time.Time
}
