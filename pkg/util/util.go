/*
Copyright 2020 DevSpace Technologies Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"fmt"
	"github.com/kiosk-sh/kiosk/pkg/constants"
	"os/exec"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Run runs a specified command and pretty prints an possible error
func Run(command string, args ...string) error {
	output, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error in command: %v => %s", err, string(output))
	}

	return nil
}

// Output runs a specified command and pretty prints an possible error
func Output(command string, args ...string) (string, error) {
	output, err := exec.Command(command, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Error in command: %v => %s", err, string(output))
	}

	return string(output), nil
}

// GetAccountFromNamespace retrieves the account from a namespace meta
func GetAccountFromNamespace(namespace metav1.Object) string {
	labels := namespace.GetLabels()
	if labels == nil {
		return ""
	}

	return labels[constants.SpaceLabelAccount]
}

// IsNamespaceInitializing checks if the given namespace is initializing
func IsNamespaceInitializing(namespace metav1.Object) bool {
	annotations := namespace.GetAnnotations()
	if annotations == nil {
		return false
	}

	return annotations[constants.SpaceAnnotationInitializing] == "true"
}

// StringsEqual checks if two string arrays are equal
func StringsEqual(a []string, b []string) bool {
	// Check if all namespaces are there
	if len(a) != len(b) {
		return false
	}

	checkedIndexes := map[int]bool{}
	for _, needle := range a {
		found := false
		for i, expected := range b {
			if expected == needle && !checkedIndexes[i] {
				found = true
				checkedIndexes[i] = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
