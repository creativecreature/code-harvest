package mock

// Clock is a mock implementation of the clock.Clock interface.
type Clock struct {
	time int64
}

func (c *Clock) GetTime() int64 {
	return c.time
}

func (c *Clock) SetTime(time int64) {
	c.time = time
}

func (c *Clock) AddTime(time int64) {
	c.time += time
}
