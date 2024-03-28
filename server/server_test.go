package server_test

import (
	"io"
	"testing"

	codeharvest "github.com/creativecreature/code-harvest"
	"github.com/creativecreature/code-harvest/logger"
	"github.com/creativecreature/code-harvest/memory"
	"github.com/creativecreature/code-harvest/mock"
	"github.com/creativecreature/code-harvest/server"
)

func TestJumpingBetweenInstances(t *testing.T) {
	t.Parallel()

	mockStorage := memory.NewStorage()
	mockFileReader := mock.NewFileReader()
	mockClock := &mock.Clock{}
	mockClock.SetTime(0)

	s, err := server.New(
		"TestApp",
		server.WithLog(logger.New(io.Discard, logger.LevelOff)),
		server.WithFileReader(mockFileReader),
		server.WithStorage(mockStorage),
		server.WithClock(mockClock),
	)
	if err != nil {
		t.Error(err)
	}

	// Open an initial VIM window.
	reply := ""
	err = s.OpenFile(codeharvest.Event{
		EditorID: "123",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Add some time between the session being started, and the first buffer opened.
	// Since this is the first session we started, the duration will still count
	// towards the total. It's only for new sessions that we require a valid
	// buffer to be opened for us to start counting time.
	mockClock.AddTime(10)

	// Open a file in the first window
	mockFileReader.SetFile(
		codeharvest.GitFile{
			Name:       "install.sh",
			Filetype:   "bash",
			Repository: "dotfiles",
			Path:       "dotfiles/install.sh",
		},
	)
	err = s.OpenFile(codeharvest.Event{
		EditorID: "123",
		Path:     "/Users/conner/code/dotfiles/install.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Push the clock forward to simulate that the file was opened for 100 ms.
	mockClock.AddTime(100)

	// Open another VIM window in a new split. We did not open a buffer with a valid
	// path yet. Therefore, the time should still be counted for session 123.
	err = s.OpenFile(codeharvest.Event{
		EditorID: "345",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}
	mockClock.AddTime(20)

	// Open a file in the second VIM window.
	mockFileReader.SetFile(
		codeharvest.GitFile{
			Name:       "bootstrap.sh",
			Filetype:   "bash",
			Repository: "dotfiles",
			Path:       "dotfiles/bootstrap.sh",
		},
	)
	err = s.OpenFile(codeharvest.Event{
		EditorID: "345",
		Path:     "/Users/conner/code/dotfiles/bootstrap.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Push the clock forward to simulate that the file was opened for 50 ms.
	mockClock.AddTime(50)

	// Open a new split where we never open a file. This should not count as an
	// active session. When we later end session 123, we want to start count time
	// for session 345 because it actually has a buffer open.
	err = s.OpenFile(codeharvest.Event{
		EditorID: "678",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}
	mockClock.AddTime(10)

	// Move focus back to the first VIM window. This should
	// resumse that session, and pause the second one.
	mockFileReader.SetFile(
		codeharvest.GitFile{
			Name:       "install.sh",
			Filetype:   "bash",
			Repository: "dotfiles",
			Path:       "dotfiles/install.sh",
		},
	)
	err = s.OpenFile(codeharvest.Event{
		EditorID: "123",
		Path:     "/Users/conner/code/dotfiles/install.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Push the clock forward 200 ms and then end the session. After the session has
	// ended, we'll add 50ms to the clock. We want to ensure that this time get's added
	// to the session we'd expect, which is session 345 because it has a buffer open.
	mockClock.AddTime(200)
	err = s.EndSession(codeharvest.Event{
		EditorID: "123",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// This should be added to session 345 because it's
	// the last most recent session with a valid buffer.
	mockClock.AddTime(50)

	// Open another VIM window without opening a buffer.
	// Time should  still be counting for session 345.
	err = s.OpenFile(codeharvest.Event{
		EditorID: "678",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// This should still be added to session 345.
	mockClock.AddTime(25)

	// Now, we'll move the focus back to the second VIM window, to end that session too.
	err = s.OpenFile(codeharvest.Event{
		EditorID: "345",
		Path:     "/Users/conner/code/dotfiles/bootstrap.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}
	mockClock.AddTime(20)
	err = s.EndSession(codeharvest.Event{
		EditorID: "345",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	storedSessions, _ := mockStorage.Read()

	expectedNumberOfSessions := 2
	if len(storedSessions) != expectedNumberOfSessions {
		t.Errorf("expected len %d; got %d", expectedNumberOfSessions, len(storedSessions))
	}

	if storedSessions[0].DurationMs != 330 {
		t.Errorf("expected duration 330; got %d", storedSessions[0].DurationMs)
	}

	if storedSessions[1].DurationMs != 155 {
		t.Errorf("expected duration 155; got %d", storedSessions[1].DurationMs)
	}
}

func TestNoActivityShouldEndSession(t *testing.T) {
	t.Parallel()

	mockStorage := memory.NewStorage()
	mockClock := &mock.Clock{}
	mockFilereader := mock.NewFileReader()

	s, err := server.New(
		"testApp",
		server.WithLog(logger.New(io.Discard, logger.LevelOff)),
		server.WithClock(mockClock),
		server.WithFileReader(mockFilereader),
		server.WithStorage(mockStorage),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Send the initial focus event
	mockClock.SetTime(100)
	reply := ""
	err = s.FocusGained(codeharvest.Event{
		EditorID: "123",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	mockClock.SetTime(200)
	s.CheckHeartbeat()

	// Send an open file event. This should update the time for the last activity to 250.
	mockClock.SetTime(250)
	mockFilereader.SetFile(
		codeharvest.GitFile{
			Name:       "install.sh",
			Filetype:   "bash",
			Repository: "dotfiles",
			Path:       "dotfiles/install.sh",
		},
	)

	err = s.OpenFile(codeharvest.Event{
		EditorID: "123",
		Path:     "/Users/conner/code/dotfiles/install.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Perform another heartbeat check. Remember these checks does not update
	// the time for when we last saw activity in the session.
	mockClock.SetTime(300)
	s.CheckHeartbeat()

	// Heartbeat check that occurs 1 millisecond after the time of last activity
	// + ttl. This should result in the session being ended and saved.
	mockClock.SetTime(server.HeartbeatTTL.Milliseconds() + 250 + 1)
	s.CheckHeartbeat()

	mockClock.SetTime(server.HeartbeatTTL.Milliseconds() + 300)
	mockFilereader.SetFile(
		codeharvest.GitFile{
			Name:       "cleanup.sh",
			Filetype:   "bash",
			Repository: "dotfiles",
			Path:       "dotfiles/cleanup.sh",
		},
	)
	err = s.OpenFile(codeharvest.Event{
		EditorID: "123",
		Path:     "/Users/conner/code/dotfiles/cleanup.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	mockClock.SetTime(server.HeartbeatTTL.Milliseconds() + 400)
	s.CheckHeartbeat()

	err = s.EndSession(codeharvest.Event{
		EditorID: "123",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	expectedNumberOfSessions := 2
	storedSessions, _ := mockStorage.Read()

	if len(storedSessions) != expectedNumberOfSessions {
		t.Errorf("expected len %d; got %d", expectedNumberOfSessions, len(storedSessions))
	}
}
