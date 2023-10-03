package cfg

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config for storing all parameters
type Config struct {
	Region               string
	EDEndpoint           string
	PushTimeout          time.Duration
	RetryInterval        time.Duration
	ForwardLambdaTags    bool
	ForwardForwarderTags bool
}

func GetConfig() (*Config, error) {
	config := &Config{}

	var errs []error
	region := os.Getenv("AWS_REGION")
	if region == "" {
		err := fmt.Errorf("failed to get AWS_REGION from environment")
		errs = append(errs, err)
	} else {
		config.Region = region
	}

	endpoint := os.Getenv("ED_ENDPOINT")
	if endpoint == "" {
		err := fmt.Errorf("ED_ENDPOINT environment variable is required")
		errs = append(errs, err)
	} else {
		config.EDEndpoint = endpoint
	}

	pt := os.Getenv("ED_PUSH_TIMEOUT_SEC")
	if pt != "" {
		pushTimeout, err := strconv.Atoi(pt)
		if err != nil {
			errs = append(errs, err)
		} else {
			config.PushTimeout = time.Duration(pushTimeout) * time.Second
		}
	} else {
		config.PushTimeout = 10 * time.Second
	}

	ri := os.Getenv("ED_RETRY_INTERVAL_MS")
	if ri != "" {
		retryInterval, err := strconv.Atoi(ri)
		if err != nil {
			errs = append(errs, err)
		} else {
			config.RetryInterval = time.Duration(retryInterval) * time.Millisecond
		}
	} else {
		config.RetryInterval = 100 * time.Millisecond
	}

	config.ForwardLambdaTags = os.Getenv("ED_FORWARD_LAMBDA_TAGS") == "true"
	config.ForwardForwarderTags = os.Getenv("ED_FORWARD_FORWARDER_TAGS") == "true"

	if len(errs) == 0 {
		return config, nil
	}

	errorsAsStr := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		errorsAsStr = append(errorsAsStr, err.Error())
	}
	return config, errors.New(strings.Join(errorsAsStr, "\n"))
}
