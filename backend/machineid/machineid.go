// Package machineid provides machine identification utilities.
package machineid

import (
	"fmt"
	"runtime"

	machineidlib "github.com/denisbrodbeck/machineid"
)

var buildKey string

func Init(key string) {
	buildKey = key
}

func GetMachineId() string {
	salt := buildKey
	if salt == "" {
		id, err := machineidlib.ID()
		if err != nil {
			return fmt.Sprintf("fallback-%s-%d", runtime.GOOS, 0)
		}
		return id
	}

	id, err := machineidlib.ProtectedID(salt)
	if err != nil {
		id, err = machineidlib.ID()
		if err != nil {
			return fmt.Sprintf("fallback-%s-%d", runtime.GOOS, 0)
		}
	}

	return id
}
