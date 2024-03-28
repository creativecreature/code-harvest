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

func (session *ActiveSession) Pause(time int64) {
	if currentBuffer := session.bufStack.peek(); currentBuffer != nil {
		currentBuffer.Close(time)
	}
	session.startStops = append(session.startStops, time)
}

func (session *ActiveSession) Resume(time int64) {
	if currentBuffer := session.bufStack.peek(); currentBuffer != nil {
		currentBuffer.Open(time)
	}
	session.startStops = append(session.startStops, time)
}

// PushBuffer pushes a new buffer to the current sessions buffer stack.
func (session *ActiveSession) PushBuffer(buffer Buffer) {
	// Stop recording time for the previous buffer.
	if currentBuffer := session.bufStack.peek(); currentBuffer != nil {
		currentBuffer.Close(buffer.LastOpened())
	}
	session.bufStack.push(buffer)
}

func (session *ActiveSession) Duration() int64 {
	var duration int64
	for i := 0; i < len(session.startStops); i += 2 {
		duration += session.startStops[i+1] - session.startStops[i]
	}
	return duration
}

// End ends the active coding sessions. It sets the total duration in
// milliseconds, and turns the stack of buffers into a slice of files.
func (session *ActiveSession) End(endedAt int64) Session {
	if currentBuffer := session.bufStack.peek(); currentBuffer != nil && currentBuffer.IsOpen() {
		currentBuffer.Close(endedAt)
	}
	session.startStops = append(session.startStops, endedAt)

	return Session{
		StartedAt:  session.StartedAt,
		EndedAt:    endedAt,
		DurationMs: session.Duration(),
		OS:         session.OS,
		Editor:     session.Editor,
		Files:      session.bufStack.files(),
	}
}
