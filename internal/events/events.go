package events

import "time"

type Type string

const (
	Ready            Type = "ready"
	Visitor          Type = "visitor"
	DownloadStart    Type = "download_start"
	DownloadComplete Type = "download_complete"
	UploadStart      Type = "upload_start"
	UploadComplete   Type = "upload_complete"
	UploadRejected   Type = "upload_rejected"
	Error            Type = "error"
)

type Event struct {
	Type   Type
	Time   time.Time
	File   string
	Size   int64
	Bytes  int64
	Client string
	Err    string
}
