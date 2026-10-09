package colonybridge

// Frame is one Goroutine Colony protocol event. Only the fields relevant to
// each type are set. The protocol speaks generic concurrency (goroutines,
// mutexes, memory) — SeatRush concepts are mapped onto it here, never in the
// browser.
type Frame struct {
	Type    string  `json:"type"`
	T       float64 `json:"t"`
	Program string  `json:"program,omitempty"`

	GID    *uint64 `json:"gid,omitempty"`
	Fn     string  `json:"fn,omitempty"`
	Label  string  `json:"label,omitempty"`
	Parent *uint64 `json:"parent,omitempty"`
	At     string  `json:"at,omitempty"`

	Mu    string `json:"mu,omitempty"`
	Mem   string `json:"mem,omitempty"`
	Group string `json:"group,omitempty"`
	Value any    `json:"value,omitempty"`
	Tone  string `json:"tone,omitempty"`
	Addr  string `json:"addr,omitempty"`

	PID *uint64 `json:"pid,omitempty"`

	Outcome string `json:"outcome,omitempty"`
	Status  string `json:"status,omitempty"`

	GIDs     []uint64 `json:"gids,omitempty"`
	Expected any      `json:"expected,omitempty"`
	Title    string   `json:"title,omitempty"`
	Detail   string   `json:"detail,omitempty"`

	App      string            `json:"app,omitempty"`
	Fields   []Field           `json:"fields,omitempty"`
	Outcomes map[string]string `json:"outcomes,omitempty"`
}

type Field struct {
	K string `json:"k"`
	V string `json:"v"`
}

func u64(v uint64) *uint64 { return &v }
