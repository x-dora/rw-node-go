package xray

type ProcessCore struct {
	binaryPath string
}

func NewProcessCore(binaryPath string) *ProcessCore {
	return &ProcessCore{binaryPath: binaryPath}
}
