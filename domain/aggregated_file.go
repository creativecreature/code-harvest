package domain

// AggregatedFile represents a file that has been aggregated for a given time
// period. Raw sessions are aggregated by day. Daily sessions are aggregated by
// week, month, and year.
type AggregatedFile struct {
	Name       string `bson:"name"`
	Path       string `bson:"path"`
	Filetype   string `bson:"filetype"`
	DurationMs int64  `bson:"duration_ms"`
}

// merge takes two AggregatedFile, merges them, and returns a new AggregatedFile
func (a AggregatedFile) merge(b AggregatedFile) AggregatedFile {
	return AggregatedFile{
		Name:       a.Name,
		Path:       a.Path,
		Filetype:   a.Filetype,
		DurationMs: a.DurationMs + b.DurationMs,
	}
}
