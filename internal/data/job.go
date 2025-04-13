package data

// Job definiert die Struktur eines zu verarbeitenden Jobs.
type Job struct {
	UID         string `json:"uid,omitempty"`
	Data        string `json:"data,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}
