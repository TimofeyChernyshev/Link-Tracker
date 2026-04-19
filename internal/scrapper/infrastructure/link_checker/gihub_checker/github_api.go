package githubchecker

import "time"

type Repository struct {
	FullName  string    `json:"full_name"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PullRequest struct {
	ID        int       `json:"id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	User      User      `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
}

type Issue struct {
	ID          int       `json:"id"`
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	User        User      `json:"user"`
	CreatedAt   time.Time `json:"created_at"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PullRequest *struct{} `json:"pull_request"`
}

type User struct {
	Login string `json:"login"`
}
