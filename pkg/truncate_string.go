package pkg

func TruncateString(s string, previewLen int) string {
	if len(s) <= previewLen {
		return s
	}
	return s[:previewLen] + "..."
}
