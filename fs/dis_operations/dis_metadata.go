package dis_operations

// The Top Data Structure
type FileInfo struct {
	FileName             string            `json:"original_file_name"`
	FileSize             int64             `json:"original_file_size"`
	Checksum             string            `json:"checksum"`
	Padding              int64             `json:"padding_amount"`
	DistributedFileInfos []DistributedFile `json:"distributed_file_infos"`
}

type DistributedFile struct {
	DistributedFile string `json:"distributed_file_name"`
	DisFileSize     int64  `json:"distributed_file_size"`
	Remote          Remote `json:"remote"`
}

type Remote struct {
	Name string `json:"remote_name"`
	Type string `json:"remote_type"`
}

var remoteDirectory = "Distribution"
