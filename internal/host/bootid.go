package host

import (
	"fmt"
	"os"
	"strings"
)

const bootIDPath = "/proc/sys/kernel/random/boot_id"

func CurrentBootID() (string, error) {
	payload, err := os.ReadFile(bootIDPath)
	if err != nil {
		return "", fmt.Errorf("read boot id: %w", err)
	}

	bootID := strings.TrimSpace(string(payload))
	if bootID == "" {
		return "", fmt.Errorf("read boot id: empty value")
	}

	return bootID, nil
}
