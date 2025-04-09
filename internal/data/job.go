package data

// Job definiert die Struktur eines zu verarbeitenden Jobs.
type Job struct {
	UID  string                 `json:"uid"`
	Data map[string]interface{} `json:"data"`
}
