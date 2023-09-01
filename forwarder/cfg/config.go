package cfg

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config for storing all parameters
type Config struct {
	EDEndpoint    string
	PushTimeout   time.Duration
	RetryInterval time.Duration
}

func GetConfig() (*Config, error) {
	config := &Config{}

	var errs []error

	endpoint := os.Getenv("ED_ENDPOINT")
	if endpoint == "" {
		err := fmt.Errorf("ED_ENDPOINT environment variable is required")
		errs = append(errs, err)
	}

	pt := os.Getenv("ED_PUSH_TIMEOUT_MS")
	if pt != "" {
		pushTimeout, err := strconv.Atoi(pt)
		if err != nil {
			errs = append(errs, err)
		} else {
			config.PushTimeout = time.Duration(pushTimeout) * time.Millisecond
		}
	} else {
		config.PushTimeout = 1 * time.Second
	}

	ri := os.Getenv("ED_RETRY_INTERVAL_MS")
	if pt != "" {
		retryInterval, err := strconv.Atoi(ri)
		if err != nil {
			errs = append(errs, err)
		} else {
			config.RetryInterval = time.Duration(retryInterval) * time.Millisecond
		}
	} else {
		config.RetryInterval = 10 * time.Millisecond
	}

	return config, errors.Join(errs...)
}
