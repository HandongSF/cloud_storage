package dis_operations

type DistributedFile struct {
	OriginalFileName string `json:"original_file_name"`
	DistributedFile  string `json:"distributed_file_name"`
	DisFileSize      int64  `json:"distributed_file_size"`
	remote           Remote
}

type FileInfo struct {
	FileName             string            `json:"original_file_name"`
	FileSize             int64             `json:"original_file_size"`
	Checksum             string            `json:"checksum"`
	DistributedFileInfos []DistributedFile `json:"distributed_file_infos"`
}

type Remote struct {
	Name string `json:"remote_name"`
	Type string `json:"remote_type"`
}

var remoteDirectory = "Distribution"
