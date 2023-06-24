package domain

// Buffer represents a buffer that has been opened during a coding session
type Buffer struct {
	OpenedAt   int64
	ClosedAt   int64
	Filename   string
	Repository string
	Filepath   string
	Filetype   string
	DurationMs int64
}

func NewBuffer(filename, repo, filetype, filepath string, openedAt int64) Buffer {
	return Buffer{
		Filename:   filename,
		Repository: repo,
		Filetype:   filetype,
		Filepath:   filepath,
		OpenedAt:   openedAt,
		ClosedAt:   0,
	}
}
