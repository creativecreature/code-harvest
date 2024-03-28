package codeharvest

// File represents a file that has been opened during a coding session.
type File struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Repository string `json:"repository"`
	Filetype   string `json:"filetype"`
	DurationMs int64  `json:"duration_ms"`
}

// Turns a buffer from an active coding session into a file entry.
func fileFromBuffer(b Buffer) File {
	return File{
		Name:       b.Filename,
		Path:       b.Filepath,
		Repository: b.Repository,
		Filetype:   b.Filetype,
		DurationMs: b.Duration(),
	}
}

type Files []File
