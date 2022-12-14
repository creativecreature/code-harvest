package app_test

import (
	"errors"
	"io"
	"testing"

	"code-harvest.conner.dev/internal/app"
	"code-harvest.conner.dev/internal/models"
	"code-harvest.conner.dev/internal/shared"
	"code-harvest.conner.dev/pkg/clock"
	"code-harvest.conner.dev/pkg/logger"
)

func TestJumpingBetweenInstances(t *testing.T) {
	t.Parallel()

	mockStorage := &MockStorage{}
	mockMetadataReader := &MockFileReader{}

	a, err := app.New(
		app.WithLog(logger.New(io.Discard, logger.LevelOff)),
		app.WithMetadataReader(mockMetadataReader),
		app.WithStorage(mockStorage),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Open a new VIM instance
	reply := ""
	mockMetadataReader.Metadata = nil
	a.FocusGained(shared.Event{
		Id:     "123",
		Path:   "",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	// Open a file in the first instance
	mockMetadataReader.Metadata = &app.FileMetadata{
		Filename:       "install.sh",
		Filetype:       "bash",
		RepositoryName: "dotfiles",
	}
	a.OpenFile(shared.Event{
		Id:     "123",
		Path:   "/Users/conner/code/dotfiles/install.sh",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	// Open another vim instance in a new split. This should end the previous session.
	mockMetadataReader.Metadata = nil
	a.FocusGained(shared.Event{
		Id:     "345",
		Path:   "",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	// Open a file in the second vim instance
	mockMetadataReader.Metadata = &app.FileMetadata{
		Filename:       "bootstrap.sh",
		Filetype:       "bash",
		RepositoryName: "dotfiles",
	}
	a.OpenFile(shared.Event{
		Id:     "345",
		Path:   "/Users/conner/code/dotfiles/bootstrap.sh",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	// Move focus back to the first VIM instance. This should end the second session.
	mockMetadataReader.Metadata = &app.FileMetadata{
		Filename:       "install.sh",
		Filetype:       "bash",
		RepositoryName: "dotfiles",
	}
	a.FocusGained(shared.Event{
		Id:     "123",
		Path:   "/Users/conner/code/dotfiles/install.sh",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	// End the last session. We should now have 3 finished sessiona.
	a.EndSession(shared.Event{
		Id:     "123",
		Path:   "",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	expectedNumberOfSessions := 3
	storedSessions := mockStorage.Get()

	if len(storedSessions) != expectedNumberOfSessions {
		t.Errorf("expected len %d; got %d", expectedNumberOfSessions, len(storedSessions))
	}
}

func TestJumpBackAndForthToTheSameInstance(t *testing.T) {
	t.Parallel()

	mockStorage := &MockStorage{}
	mockMetadataReader := &MockFileReader{}

	a, err := app.New(
		app.WithLog(logger.New(io.Discard, logger.LevelOff)),
		app.WithMetadataReader(mockMetadataReader),
		app.WithStorage(mockStorage),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Open a new instance of VIM
	reply := ""
	mockMetadataReader.Metadata = nil
	a.FocusGained(shared.Event{
		Id:     "123",
		Path:   "",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	// Open a file
	mockMetadataReader.Metadata = &app.FileMetadata{
		Filename:       "install.sh",
		Filetype:       "bash",
		RepositoryName: "dotfiles",
	}
	a.OpenFile(shared.Event{
		Id:     "123",
		Path:   "/Users/conner/code/dotfiles/install.sh",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	// Lets now imagine we opened another TMUX split to run testa. We then jump
	// back to VIM which will fire the focus gained event with the same client id.
	a.FocusGained(shared.Event{
		Id:     "123",
		Path:   "",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	// We repeat the same thing again. Jump to another split in the terminal which makes
	// VIM lose focus and then back again - which will trigger another focus gained event.
	a.FocusGained(shared.Event{
		Id:     "123",
		Path:   "",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)
	mockMetadataReader.Metadata = &app.FileMetadata{
		Filename:       "bootstrap.sh",
		Filetype:       "bash",
		RepositoryName: "dotfiles",
	}
	a.OpenFile(shared.Event{
		Id:     "123",
		Path:   "/Users/conner/code/dotfiles/bootstrap.sh",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	// Lets now end the session. This behaviour should *not* have resulted in any
	// new sessions being created. We only create a new session and end the current
	// one if we open VIM in a new split (to not count double time).
	a.EndSession(shared.Event{
		Id:     "123",
		Path:   "",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	expectedNumberOfSessions := 1
	storedSessions := mockStorage.Get()

	if len(storedSessions) != expectedNumberOfSessions {
		t.Errorf("expected len %d; got %d", expectedNumberOfSessions, len(storedSessions))
	}
}

func TestNoActivityShouldEndSession(t *testing.T) {
	t.Parallel()

	mockStorage := &MockStorage{}
	mockClock := &clock.MockClock{}
	mockMetadataReader := &MockFileReader{}
	mockMetadataReader.Metadata = nil

	a, err := app.New(
		app.WithLog(logger.New(io.Discard, logger.LevelOff)),
		app.WithClock(mockClock),
		app.WithMetadataReader(mockMetadataReader),
		app.WithStorage(mockStorage),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Send the initial focus event
	mockClock.SetTime(100)
	reply := ""
	a.FocusGained(shared.Event{
		Id:     "123",
		Path:   "",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	mockClock.SetTime(200)
	a.CheckHeartbeat()

	// Send an open file event. This should update the time for the last activity to 250.
	mockClock.SetTime(250)
	mockMetadataReader.Metadata = &app.FileMetadata{
		Filename:       "install.sh",
		Filetype:       "bash",
		RepositoryName: "dotfiles",
	}
	a.OpenFile(shared.Event{
		Id:     "123",
		Path:   "/Users/conner/code/dotfiles/install.sh",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	// Perform another heartbeat check. Remember these checks does not update
	// the time for when we last saw activity in the session.
	mockClock.SetTime(300)
	a.CheckHeartbeat()

	// Heartbeat check that occurs 1 millisecond after the time of last activity
	// + ttl. This should result in the session being ended and saved.
	mockClock.SetTime(app.HeartbeatTTL.Milliseconds() + 250 + 1)
	a.CheckHeartbeat()

	mockClock.SetTime(app.HeartbeatTTL.Milliseconds() + 300)
	mockMetadataReader.Metadata = &app.FileMetadata{
		Filename:       "cleanup.sh",
		Filetype:       "bash",
		RepositoryName: "dotfiles",
	}
	a.OpenFile(shared.Event{
		Id:     "123",
		Path:   "/Users/conner/code/dotfiles/cleanup.sh",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	mockClock.SetTime(app.HeartbeatTTL.Milliseconds() + 400)
	a.CheckHeartbeat()

	a.EndSession(shared.Event{
		Id:     "123",
		Path:   "",
		Editor: "nvim",
		OS:     "Linux",
	}, &reply)

	expectedNumberOfSessions := 2
	storedSessions := mockStorage.Get()

	if len(storedSessions) != expectedNumberOfSessions {
		t.Errorf("expected len %d; got %d", expectedNumberOfSessions, len(storedSessions))
	}
}

type MockStorage struct {
	sessions []*models.Session
}

func (m *MockStorage) Connect() func() {
	return func() {}
}

func (m *MockStorage) Save(s interface{}) error {
	result, ok := s.(*models.Session)
	if !ok {
		return errors.New("Failed to convert interface to slice of session pointers")
	}
	m.sessions = append(m.sessions, result)
	return nil
}

func (m *MockStorage) Get() []*models.Session {
	return m.sessions
}

type MockFileReader struct {
	Metadata *app.FileMetadata
}

func (f *MockFileReader) Read(path string) (app.FileMetadata, error) {
	if f.Metadata == nil {
		return app.FileMetadata{}, errors.New("metadata is nil")
	}
	return *f.Metadata, nil
}
