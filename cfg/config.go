package cfg

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPushTimeout = 10 * time.Second
	MaxChunkSize       = 1000 * 1000 // 1MB
	MinChunkSize       = 50 * 1000   // 50KB
)

// Config for storing all parameters
type Config struct {
	Region                    string
	EDEndpoint                string
	PushTimeout               time.Duration
	BatchSize                 int
	RetryInterval             time.Duration
	ForwardForwarderTags      bool
	ForwardSourceTags         bool
	ForwardLogGroupTags       bool
	SourceEnvironmentPrefixes string
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
		config.PushTimeout = defaultPushTimeout
	}

	bs := os.Getenv("ED_BATCH_SIZE")
	if bs != "" {
		batchSize, err := strconv.Atoi(bs)
		if err != nil {
			errs = append(errs, err)
		} else {
			if batchSize <= 0 {
				errs = append(errs, errors.New("batch size must be greater than 0"))
			}
			if batchSize > MaxChunkSize {
				errs = append(errs, fmt.Errorf("batch size must be less than or equal to %d", MaxChunkSize))
			}
			if batchSize < MinChunkSize {
				errs = append(errs, fmt.Errorf("batch size must be greater than or equal to %d", MinChunkSize))
			}
			config.BatchSize = batchSize
		}
	} else {
		config.BatchSize = MaxChunkSize
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

	config.SourceEnvironmentPrefixes = os.Getenv("ED_SOURCE_TAG_PREFIXES")

	config.ForwardForwarderTags = os.Getenv("ED_FORWARD_FORWARDER_TAGS") == "true"
	config.ForwardSourceTags = os.Getenv("ED_FORWARD_SOURCE_TAGS") == "true"
	config.ForwardLogGroupTags = os.Getenv("ED_FORWARD_LOG_GROUP_TAGS") == "true"

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
