// +build go1.13

package config

import "os"

func UserConfigDir() (string, error) {
	return os.UserConfigDir()
}
