package glowy

// GlowMsg
type GlowMsg struct {
	Node    int
	Process int
	OK      bool
	Value   int
}

const (
	CmdActivity = 1
	CmdIdle     = 2
)
