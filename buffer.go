package codeharvest

// Buffer represents a buffer that has been opened during an active coding session.
type Buffer struct {
	OpenedClosed []int64
	Filename     string
	Repository   string
	Filepath     string
	Filetype     string
}

func NewBuffer(filename, repo, filetype, filepath string, openedAt int64) Buffer {
	return Buffer{
		OpenedClosed: []int64{openedAt},
		Filename:     filename,
		Repository:   repo,
		Filetype:     filetype,
		Filepath:     filepath,
	}
}

func (b *Buffer) Open(time int64) {
	b.OpenedClosed = append(b.OpenedClosed, time)
}

func (b *Buffer) IsOpen() bool {
	return len(b.OpenedClosed)%2 == 1
}

func (b *Buffer) LastOpened() int64 {
	return b.OpenedClosed[len(b.OpenedClosed)-1]
}

func (b *Buffer) Close(time int64) {
	b.OpenedClosed = append(b.OpenedClosed, time)
}

func (b *Buffer) Duration() int64 {
	var duration int64
	for i := 0; i < len(b.OpenedClosed); i += 2 {
		duration += b.OpenedClosed[i+1] - b.OpenedClosed[i]
	}
	return duration
}
