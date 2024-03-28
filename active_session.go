package codeharvest

// ActiveSession represents an ongoing coding session.
type ActiveSession struct {
	// startStops is a slice of timestamps that represent the start and stop
	// times of an active session. If we example switch between editors two
	// editors we only want to count time for one of them.
	startStops []int64
	bufStack   *bufferStack
	EditorID   string
	StartedAt  int64
	OS         string
	Editor     string
}

// StartSession creates a new active coding session.
func StartSession(editorID string, startedAt int64, os, editor string) *ActiveSession {
	return &ActiveSession{
		startStops: []int64{startedAt},
		bufStack:   newBufferStack(),
		EditorID:   editorID,
		StartedAt:  startedAt,
		OS:         os,
		Editor:     editor,
	}
}

func (s *ActiveSession) Pause(time int64) {
	if currentBuffer := s.bufStack.peek(); currentBuffer != nil {
		currentBuffer.Close(time)
	}
	s.startStops = append(s.startStops, time)
}

func (s *ActiveSession) PauseTime() int64 {
	if len(s.startStops) == 0 {
		return 0
	}
	return s.startStops[len(s.startStops)-1]
}

func (s *ActiveSession) Resume(time int64) {
	if currentBuffer := s.bufStack.peek(); currentBuffer != nil {
		currentBuffer.Open(time)
	}
	s.startStops = append(s.startStops, time)
}

// PushBuffer pushes a new buffer to the current sessions buffer stack.
func (s *ActiveSession) PushBuffer(buffer Buffer) {
	// Stop recording time for the previous buffer.
	if currentBuffer := s.bufStack.peek(); currentBuffer != nil {
		currentBuffer.Close(buffer.LastOpened())
	}
	s.bufStack.push(buffer)
}

func (s *ActiveSession) HasBuffers() bool {
	return len(s.bufStack.buffers) > 0
}

func (s *ActiveSession) Duration() int64 {
	var duration int64
	for i := 0; i < len(s.startStops); i += 2 {
		duration += s.startStops[i+1] - s.startStops[i]
	}
	return duration
}

// End ends the active coding sessions. It sets the total duration in
// milliseconds, and turns the stack of buffers into a slice of files.
func (s *ActiveSession) End(endedAt int64) Session {
	if currentBuffer := s.bufStack.peek(); currentBuffer != nil && currentBuffer.IsOpen() {
		currentBuffer.Close(endedAt)
	}
	s.startStops = append(s.startStops, endedAt)

	return Session{
		StartedAt:  s.StartedAt,
		EndedAt:    endedAt,
		DurationMs: s.Duration(),
		OS:         s.OS,
		Editor:     s.Editor,
		Files:      s.bufStack.files(),
	}
}
