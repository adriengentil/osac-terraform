/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

const (
	// DefaultCreateTimeout is the default timeout for creating resources
	DefaultCreateTimeout = 30 * time.Minute
	// DefaultUpdateTimeout is the default timeout for updating resources
	DefaultUpdateTimeout = 30 * time.Minute
	// DefaultPollInterval is the polling interval for checking resource status
	DefaultPollInterval = 10 * time.Second
	// DefaultMinPollInterval is the minimum polling interval
	DefaultMinPollInterval = 5 * time.Second
)

// StateRefreshFunc is a function that returns the current state of a resource.
// It returns (resource, stateString, error).
// If the resource is in a failed state, it should return an error.
// This is an alias for retry.StateRefreshFunc for use in resource implementations.
type StateRefreshFunc = retry.StateRefreshFunc

// WaitForReadyConfig contains configuration for waiting for a resource to be ready.
type WaitForReadyConfig struct {
	// PendingStates are the states that indicate the resource is still being created/updated
	PendingStates []string
	// TargetStates are the states that indicate the resource is ready
	TargetStates []string
	// RefreshFunc is the function to call to get the current state
	RefreshFunc retry.StateRefreshFunc
	// Timeout is the maximum time to wait
	Timeout time.Duration
	// PollInterval is how often to poll for status
	PollInterval time.Duration
	// MinPollInterval is the minimum polling interval
	MinPollInterval time.Duration
}

// WaitForReady waits for a resource to reach a ready state using the AWS-style StateChangeConf pattern.
// Returns the final resource object and any error encountered.
func WaitForReady(ctx context.Context, config WaitForReadyConfig) (interface{}, error) {
	// Apply defaults
	if config.Timeout == 0 {
		config.Timeout = DefaultCreateTimeout
	}
	if config.PollInterval == 0 {
		config.PollInterval = DefaultPollInterval
	}
	if config.MinPollInterval == 0 {
		config.MinPollInterval = DefaultMinPollInterval
	}

	stateConf := &retry.StateChangeConf{
		Pending:    config.PendingStates,
		Target:     config.TargetStates,
		Refresh:    config.RefreshFunc,
		Timeout:    config.Timeout,
		Delay:      config.PollInterval,
		MinTimeout: config.MinPollInterval,
	}

	result, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to reach ready state: %w", err)
	}

	return result, nil
}
