package server

import (
	"errors"

	"code-harvest.conner.dev/domain"
)

// FocusGained should be called by the FocusGained autocommand. It gives us information
// about the currently active client. The duration of a coding session should not increase
// by the number of clients (VIM instances) we use. Only one will be tracked at a time.
func (server *server) FocusGained(event domain.Event, reply *string) error {
	// The heartbeat timer could fire at the exact same time.
	server.mutex.Lock()
	defer server.mutex.Unlock()

	server.lastHeartbeat = server.clock.GetTime()

	// When I jump between TMUX splits the *FocusGained* event in VIM will fire a
	// lot. I only want to end the current session, and create a new one, when I
	// open a new instance of VIM. If I'm, for example, jumping between a VIM split
	// and a terminal with test output I don't want it to result in a new coding session.
	if server.activeClientId == event.Id {
		server.log.PrintDebug("Jumped back to the same instance of VIM.", nil)
		return nil
	}

	// If the focus event is for the first instance of VIM we won't have any previous session.
	// That only occurs when using multiple splits with multiple instances of VIM.
	if server.session != nil {
		server.saveSession()
	}

	server.activeClientId = event.Id
	server.startNewSession(event.OS, event.Editor)

	// It could be an already existing VIM instance where a file buffer is already
	// open. If that is the case we can't count on getting the *OpenFile* event.
	// We might just be jumping between two VIM instances with one buffer each.
	server.updateCurrentFile(event.Path)

	*reply = "Successfully updated the client being focused."
	return nil
}

// OpenFile should be called by the *BufEnter* autocommand.
func (server *server) OpenFile(event domain.Event, reply *string) error {
	server.log.PrintDebug("Received OpenFile event", map[string]string{
		"path": event.Path,
	})

	// To not collide with the heartbeat check that runs on an interval.
	server.mutex.Lock()
	defer server.mutex.Unlock()

	server.lastHeartbeat = server.clock.GetTime()

	// The server won't receive any heartbeats if we open a buffer and then go AFK.
	// When that hserverens the session is ended. If we come back and either write the buffer,
	// or open a new file, we have to create a new session first.
	if server.session == nil {
		server.activeClientId = event.Id
		server.startNewSession(event.OS, event.Editor)
	}

	server.updateCurrentFile(event.Path)
	*reply = "Successfully updated the current file."
	return nil
}

// SendHeartbeat should be called when we want to inform the server that the session
// is still active. If we, for example, only edit a single file for a long time we
// can send it on a *BufWrite* autocommand.
func (server *server) SendHeartbeat(event domain.Event, reply *string) error {
	// In case the heartbeat check that runs on an interval occurs at the same time.
	server.mutex.Lock()
	defer server.mutex.Unlock()

	// This scenario would occur if we write the buffer when we have been
	// inactive for more than 10 minutes. The server will have ended our coding
	// session. Therefore, we have to create a new one.
	if server.session == nil {
		message := "The session was ended by a previous heartbeat check. Creating a new one."
		server.log.PrintDebug(message, map[string]string{
			"clientId": event.Id,
			"path":     event.Path,
		})
		server.activeClientId = event.Id
		server.startNewSession(event.OS, event.Editor)
		server.updateCurrentFile(event.Path)
	}

	// Update the time for the last heartbeat.
	server.lastHeartbeat = server.clock.GetTime()

	*reply = "Successfully sent heartbeat"
	return nil
}

// EndSession should be called by the *VimLeave* autocommand to inform the server that the session is done.
func (server *server) EndSession(event domain.Event, reply *string) error {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	// We have reached an undesired state if we call end session and there is another
	// active client. It means that the events are sent in an incorrect order.
	if len(server.activeClientId) > 1 && server.activeClientId != event.Id {
		server.log.PrintFatal(errors.New("was called by a client that isn't considered active"), map[string]string{
			"actualClientId":   server.activeClientId,
			"expectedClientId": event.Id,
		})
	}

	// If we go AFK and don't send any heartbeats the session will have ended by
	// itself. If we then come back and exit VIM we will get the EndSession event
	// but won't have any session that we are tracking time for.
	if server.activeClientId == "" && server.session == nil {
		message := "The session was already ended, or possibly never started. Was there a previous heatbeat check?"
		server.log.PrintDebug(message, nil)
		return nil
	}

	server.saveSession()

	*reply = "The session was ended successfully."
	return nil
}