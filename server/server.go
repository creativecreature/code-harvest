package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	codeharvest "github.com/creativecreature/code-harvest"
	"github.com/creativecreature/code-harvest/proxy"
)

const (
	HeartbeatTTL      = time.Minute * 10
	heartbeatInterval = time.Second * 45
)

type Server struct {
	name           string
	activeEditor   string
	activeSessions map[string]*codeharvest.ActiveSession
	lastHeartbeat  int64
	clock          Clock
	fileReader     FileReader
	log            Log
	mutex          sync.Mutex
	storage        codeharvest.TemporaryStorage
}

// startNewSession creates a new session and sets it as the current session.
func (s *Server) startNewSession(id, os, editor string) {
	s.log.PrintDebug("Starting a new session", nil)
	s.activeSessions[id] = codeharvest.StartSession(id, s.clock.GetTime(), os, editor)
}

// setActiveBuffer updates the current buffer in the current session.
func (s *Server) setActiveBuffer(absolutePath string) {
	openedAt := s.clock.GetTime()
	fileData, err := s.fileReader.GitFile(absolutePath)
	if err != nil {
		exitEarlyMessage := "Could not extract metadata for the path"
		errProperties := map[string]string{
			"path":   absolutePath,
			"reason": err.Error(),
		}
		s.log.PrintDebug(exitEarlyMessage, errProperties)

		return
	}

	buf := codeharvest.NewBuffer(
		fileData.Name,
		fileData.Repository,
		fileData.Filetype,
		fileData.Path,
		openedAt,
	)
	s.activeSessions[s.activeEditor].PushBuffer(buf)
	updatedBufferMsg := "Successfully updated the current buffer"
	updatedBufferProperties := map[string]string{
		"path": absolutePath,
	}
	s.log.PrintDebug(updatedBufferMsg, updatedBufferProperties)
}

// hasOkDurations sanity checks the sessions total duration against
// the combined duration of all files that were opened.
func hasOkDurations(sessionDuration, allFilesDuration int64) bool {
	// Exclude sessions that are less than 10 minutes.
	tenMinutesMS := int64(10 * 60 * 1000)
	if sessionDuration < tenMinutesMS {
		return true
	}
	// If the session lasts for more than 10 minutes, and the time
	// differs by more than 25%, we'll want to check the session.
	larger := math.Max(float64(sessionDuration), float64(allFilesDuration))
	threshold := larger * 0.25
	difference := math.Abs(float64(sessionDuration) - float64(allFilesDuration))

	return difference <= threshold
}

func (s *Server) saveAllSessions() {
	s.log.PrintDebug("Saving all sessions.", nil)
	endedAt := s.clock.GetTime()
	for _, session := range s.activeSessions {
		if !session.HasBuffers() {
			s.log.PrintDebug("The session has not opened any buffers.", nil)
			return
		}
		finishedSession := session.End(endedAt)
		totalFileDuration := finishedSession.TotalFileDuration()
		if !hasOkDurations(finishedSession.DurationMs, totalFileDuration) {
			finishedSession.DurationMs = totalFileDuration
			err := errors.New("session had a large duration diff")
			properties := map[string]string{
				"started_at":                strconv.FormatInt(finishedSession.StartedAt, 10),
				"ended_at":                  strconv.FormatInt(endedAt, 10),
				"previous_session_duration": strconv.FormatInt(finishedSession.DurationMs, 10),
				"new_session_duration":      strconv.FormatInt(totalFileDuration, 10),
			}
			s.log.PrintError(err, properties)
		}
		err := s.storage.Write(finishedSession)
		if err != nil {
			s.log.PrintError(err, nil)
		}
		delete(s.activeSessions, session.EditorID)
	}
}

// saveSession ends the current coding session and saves it to the filesystem.
func (s *Server) saveActiveSession() {
	if !s.activeSessions[s.activeEditor].HasBuffers() {
		s.log.PrintDebug("The active session has not opened any buffers.", nil)
		return
	}

	s.log.PrintDebug("Saving the session.", nil)
	endedAt := s.clock.GetTime()
	finishedSession := s.activeSessions[s.activeEditor].End(endedAt)

	// Perform sanity checks on the durations.
	totalFileDuration := finishedSession.TotalFileDuration()
	if !hasOkDurations(finishedSession.DurationMs, totalFileDuration) {
		finishedSession.DurationMs = totalFileDuration
		err := errors.New("session had a large duration diff")
		properties := map[string]string{
			"started_at":                strconv.FormatInt(finishedSession.StartedAt, 10),
			"ended_at":                  strconv.FormatInt(endedAt, 10),
			"previous_session_duration": strconv.FormatInt(finishedSession.DurationMs, 10),
			"new_session_duration":      strconv.FormatInt(totalFileDuration, 10),
		}
		s.log.PrintError(err, properties)
	}

	err := s.storage.Write(finishedSession)
	if err != nil {
		s.log.PrintError(err, nil)
	}

	delete(s.activeSessions, s.activeEditor)
	s.activeEditor = ""

	// Check if we should resume another session.
	if len(s.activeSessions) < 1 {
		return
	}

	var editorToResume string
	var mostRecentPause int64
	for _, session := range s.activeSessions {
		if session.PauseTime() > mostRecentPause {
			editorToResume = session.EditorID
			mostRecentPause = session.PauseTime()
		}
	}

	if editorToResume != "" {
		s.activeEditor = editorToResume
		s.activeSessions[s.activeEditor].Resume(s.clock.GetTime())
	}
}

func (s *Server) startServer(port string) (*http.Server, error) {
	proxy := proxy.New(s)
	err := rpc.RegisterName(s.name, proxy)
	if err != nil {
		return nil, err
	}

	rpc.HandleHTTP()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return nil, err
	}
	httpServer := &http.Server{
		ReadHeaderTimeout: time.Second * 5,
	}
	go func() {
		serveErr := httpServer.Serve(listener)
		if !errors.Is(serveErr, http.ErrServerClosed) {
			panic(serveErr)
		}
	}()

	return httpServer, nil
}

// startECG runs a heartbeat ticker that ensures that
// the current session is not idle for more than ten minutes.
func (s *Server) startECG() *time.Ticker {
	s.log.PrintDebug("Starting the ECG", nil)
	ecg := time.NewTicker(heartbeatInterval)
	go func() {
		for range ecg.C {
			s.CheckHeartbeat()
		}
	}()

	return ecg
}

// Start starts the server on the given port.
func (s *Server) Start(port string) error {
	s.log.PrintInfo("Starting up...", nil)
	httpServer, err := s.startServer(port)
	if err != nil {
		return err
	}

	// Start the ECG. It will end inactive sessions.
	ecg := s.startECG()

	// Catch shutdown signals from the OS
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Blocks until a shutdown signal is received.
	sig := <-quit
	s.log.PrintInfo("Received shutdown signal", map[string]string{
		"signal": sig.String(),
	})

	// Stop the ECG and shutdown the http server.
	ecg.Stop()
	err = httpServer.Shutdown(context.Background())
	if err != nil {
		s.log.PrintError(err, nil)
		return err
	}

	// Save the current session before shutting down.
	s.saveAllSessions()
	s.log.PrintInfo("Shutting down...", nil)

	return nil
}
