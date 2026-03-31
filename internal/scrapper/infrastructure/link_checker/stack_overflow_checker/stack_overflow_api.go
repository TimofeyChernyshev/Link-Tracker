package stackoverflowchecker

type Answer struct {
	AnswerID     int    `json:"answer_id"`
	QuestionID   int64  `json:"question_id"`
	Owner        User   `json:"owner"`
	CreationDate int64  `json:"creation_date"`
	Body         string `json:"body"`
	Title        string `json:"title"`
}

type Comment struct {
	CommentID    int    `json:"comment_id"`
	Owner        User   `json:"owner"`
	CreationDate int64  `json:"creation_date"`
	Body         string `json:"body"`
}

type Question struct {
	QuestionID int    `json:"question_id"`
	Title      string `json:"title"`
}

type User struct {
	DisplayName string `json:"display_name"`
	Link        string `json:"link"`
}

type AnswersResponse struct {
	Items []Answer `json:"items"`
}

type CommentsResponse struct {
	Items []Comment `json:"items"`
}

type QuestionResponse struct {
	Items []Question `json:"items"`
}
