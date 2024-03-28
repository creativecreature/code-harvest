package codeharvest

import (
	"golang.org/x/exp/maps"
)

// bufferStack is the stack of buffers that we have opened during an active coding session.
type bufferStack struct {
	buffers []Buffer
}

func newBufferStack() *bufferStack {
	buffers := make([]Buffer, 0)
	return &bufferStack{buffers}
}

// push pushes a buffer onto the stack.
func (s *bufferStack) push(f Buffer) {
	s.buffers = append(s.buffers, f)
}

// peek returns a pointer to the most recent buffer.
func (s *bufferStack) peek() *Buffer {
	if len(s.buffers) == 0 {
		return nil
	}
	return &s.buffers[len(s.buffers)-1]
}

// pop pops a buffer off the stack.
func (s *bufferStack) pop() (Buffer, bool) {
	if len(s.buffers) == 0 {
		return Buffer{}, false
	}

	res := s.buffers[len(s.buffers)-1]
	s.buffers = s.buffers[:len(s.buffers)-1]

	return res, true
}

// slice takes the stack of buffers, merges them by filepath, and returns the result.
func (s *bufferStack) files() Files {
	pathFile := make(map[string]File)
	for buffer, ok := s.pop(); ok; buffer, ok = s.pop() {
		if file, exists := pathFile[buffer.Filepath]; !exists {
			pathFile[buffer.Filepath] = fileFromBuffer(buffer)
		} else {
			file.DurationMs += buffer.Duration()
			pathFile[buffer.Filepath] = file
		}
	}

	return maps.Values(pathFile)
}
