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

	a, err := server.New(
		"TestApp",
		server.WithLog(logger.New(io.Discard, logger.LevelOff)),
		server.WithFileReader(mockFileReader),
		server.WithStorage(mockStorage),
	)
	if err != nil {
		t.Error(err)
	}

	// Open a new VIM window
	reply := ""
	err = a.FocusGained(codeharvest.Event{
		EditorID: "123",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Open a file in the first window
	mockFileReader.SetFile(
		codeharvest.GitFile{
			Name:       "install.sh",
			Filetype:   "bash",
			Repository: "dotfiles",
			Path:       "dotfiles/install.sh",
		},
	)
	err = a.OpenFile(codeharvest.Event{
		EditorID: "123",
		Path:     "/Users/conner/code/dotfiles/install.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Open another VIM window in a new split. This should pause the previous session.
	err = a.FocusGained(codeharvest.Event{
		EditorID: "345",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Open a file in the second VIM window.
	mockFileReader.SetFile(
		codeharvest.GitFile{
			Name:       "bootstrap.sh",
			Filetype:   "bash",
			Repository: "dotfiles",
			Path:       "dotfiles/bootstrap.sh",
		},
	)
	err = a.OpenFile(codeharvest.Event{
		EditorID: "345",
		Path:     "/Users/conner/code/dotfiles/bootstrap.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

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
	err = a.FocusGained(codeharvest.Event{
		EditorID: "123",
		Path:     "/Users/conner/code/dotfiles/install.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// End the first session.
	err = a.EndSession(codeharvest.Event{
		EditorID: "123",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Move the focus back to the second VIM window, and end it too.
	err = a.FocusGained(codeharvest.Event{
		EditorID: "345",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}
	err = a.EndSession(codeharvest.Event{
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

func TestJumpBackAndForthToTheSameInstance(t *testing.T) {
	t.Parallel()

	mockStorage := memory.NewStorage()
	mockFilereader := mock.NewFileReader()

	a, err := server.New(
		"testApp",
		server.WithLog(logger.New(io.Discard, logger.LevelOff)),
		server.WithFileReader(mockFilereader),
		server.WithStorage(mockStorage),
	)
	if err != nil {
		t.Error(err)
	}

	// Open a new instance of VIM
	reply := ""
	err = a.FocusGained(codeharvest.Event{
		EditorID: "123",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Open a file
	mockFilereader.SetFile(
		codeharvest.GitFile{
			Name:       "install.sh",
			Filetype:   "bash",
			Repository: "dotfiles",
			Path:       "dotfiles/install.sh",
		},
	)
	err = a.OpenFile(codeharvest.Event{
		EditorID: "123",
		Path:     "/Users/conner/code/dotfiles/install.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Lets now imagine we opened another TMUX split to run testa. We then jump
	// back to VIM which will fire the focus gained event with the same client id.
	err = a.FocusGained(codeharvest.Event{
		EditorID: "123",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// We repeat the same thing again. Jump to another split in the terminal which makes
	// VIM lose focus and then back again - which will trigger another focus gained event.
	err = a.FocusGained(codeharvest.Event{
		EditorID: "123",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	mockFilereader.SetFile(
		codeharvest.GitFile{
			Name:       "bootstrap.sh",
			Filetype:   "bash",
			Repository: "dotfiles",
			Path:       "dotfiles/bootstrap.sh",
		},
	)

	err = a.OpenFile(codeharvest.Event{
		EditorID: "123",
		Path:     "/Users/conner/code/dotfiles/bootstrap.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	// Lets now end the session. This behaviour should *not* have resulted in any
	// new sessions being created. We only create a new session and end the current
	// one if we open VIM in a new split (to not count double time).
	err = a.EndSession(codeharvest.Event{
		EditorID: "123",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	expectedNumberOfSessions := 1
	storedSessions, _ := mockStorage.Read()

	if len(storedSessions) != expectedNumberOfSessions {
		t.Errorf("expected len %d; got %d", expectedNumberOfSessions, len(storedSessions))
	}
}

func TestNoActivityShouldEndSession(t *testing.T) {
	t.Parallel()

	mockStorage := memory.NewStorage()
	mockClock := &mock.Clock{}
	mockFilereader := mock.NewFileReader()

	a, err := server.New(
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
	err = a.FocusGained(codeharvest.Event{
		EditorID: "123",
		Path:     "",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	mockClock.SetTime(200)
	a.CheckHeartbeat()

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

	err = a.OpenFile(codeharvest.Event{
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
	a.CheckHeartbeat()

	// Heartbeat check that occurs 1 millisecond after the time of last activity
	// + ttl. This should result in the session being ended and saved.
	mockClock.SetTime(server.HeartbeatTTL.Milliseconds() + 250 + 1)
	a.CheckHeartbeat()

	mockClock.SetTime(server.HeartbeatTTL.Milliseconds() + 300)
	mockFilereader.SetFile(
		codeharvest.GitFile{
			Name:       "cleanup.sh",
			Filetype:   "bash",
			Repository: "dotfiles",
			Path:       "dotfiles/cleanup.sh",
		},
	)
	err = a.OpenFile(codeharvest.Event{
		EditorID: "123",
		Path:     "/Users/conner/code/dotfiles/cleanup.sh",
		Editor:   "nvim",
		OS:       "Linux",
	}, &reply)
	if err != nil {
		panic(err)
	}

	mockClock.SetTime(server.HeartbeatTTL.Milliseconds() + 400)
	a.CheckHeartbeat()

	err = a.EndSession(codeharvest.Event{
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
