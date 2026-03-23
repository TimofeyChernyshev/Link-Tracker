package scrapperhttp

type LinkResponse struct {
	ID      int64    `json:"id"`
	URL     string   `json:"url"`
	Tags    []string `json:"tags"`
	Filters []string `json:"filters"`
}

type APIErrorResponse struct {
	Description      string   `json:"description"`
	Code             string   `json:"code"`
	ExceptionName    string   `json:"exceptionName,omitempty"`
	ExceptionMessage string   `json:"exceptionMessage,omitempty"`
	Stacktrace       []string `json:"stacktrace,omitempty"`
}

type AddLinkRequest struct {
	Link    string   `json:"link"`
	Tags    []string `json:"tags"`
	Filters []string `json:"filters"`
}

type ListLinksResponse struct {
	Links []LinkResponse `json:"links"`
	Size  int32          `json:"size"`
}

type RemoveLinkRequest struct {
	Link string `json:"link"`
}
