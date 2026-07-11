package backend

import (
	"fmt"
	"os/exec"
	"strings"
)

// DS4 implements the Backend interface for DwarfStar
type DS4 struct {
	serviceName string
	port        int
}

func NewDS4() *DS4 {
	return &DS4{
		serviceName: "ds4-server",
		port:        8001,
	}
}

func (d *DS4) Name() string {
	return "ds4"
}

func (d *DS4) ModelKeys() []string {
	return []string{"default"}
}

func (d *DS4) ServiceName() string {
	return d.serviceName
}

func (d *DS4) Port() int {
	return d.port
}

func (d *DS4) Start() error {
	return runCommand("systemctl", "--user", "start", d.serviceName)
}

func (d *DS4) Stop() error {
	return runCommand("systemctl", "--user", "stop", d.serviceName)
}

func (d *DS4) Restart() error {
	return runCommand("systemctl", "--user", "restart", d.serviceName)
}

func (d *DS4) Status() string {
	out, err := exec.Command("systemctl", "--user", "is-active", d.serviceName).Output()
	if err != nil {
		return "inactive"
	}
	return strings.TrimSpace(string(out))
}

func (d *DS4) SwitchModel(key string) error {
	// DS4 only supports one model (DeepSeek V4 Flash)
	return fmt.Errorf("DS4 only supports one model (DeepSeek V4 Flash)")
}
