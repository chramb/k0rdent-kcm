// Copyright 2024
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// Run executes the provided command within this context and returns it's
// output. Run does not wait for the command to finish, use Wait instead.
func Run(cmd *exec.Cmd) ([]byte, error) {
	command := prepareCmd(cmd)
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %s\n", command)

	output, err := cmd.Output()
	if err != nil {
		return nil, handleCmdError(err, command)
	}

	return output, nil
}

func handleCmdError(err error, command string) error {
	var exitError *exec.ExitError

	if errors.As(err, &exitError) {
		return fmt.Errorf("%s failed with error: (%v): %s", command, err, string(exitError.Stderr))
	}

	return fmt.Errorf("%s failed with error: %w", command, err)
}

func prepareCmd(cmd *exec.Cmd) string {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	return strings.Join(cmd.Args, " ")
}

// LoadImageToKindCluster loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER_NAME"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}

	kindBinary := "kind"

	if kindVersion, ok := os.LookupEnv("KIND_VERSION"); ok {
		kindBinary = fmt.Sprintf("./bin/kind-%s", kindVersion)
	}

	cmd := exec.Command(kindBinary, kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.Replace(wd, "/test/e2e", "", -1)
	return wd, nil
}

// ValidateConditionsTrue iterates over the conditions of the given
// unstructured object and returns an error if any of the conditions are not
// true.  Conditions are expected to be of type metav1.Condition.
func ValidateConditionsTrue(unstrObj *unstructured.Unstructured) error {
	objKind, objName := ObjKindName(unstrObj)

	conditions, err := GetConditionsFromUnstructured(unstrObj)
	if err != nil {
		return fmt.Errorf("failed to get conditions from unstructured object: %w", err)
	}

	var errs error

	for _, c := range conditions {
		if c.Status == metav1.ConditionTrue {
			continue
		}

		errs = errors.Join(errors.New(ConvertConditionsToString(c)), errs)
	}

	if errs != nil {
		return fmt.Errorf("%s %s is not ready with conditions:\n%w", objKind, objName, errs)
	}

	return nil
}

func ConvertConditionsToString(condition metav1.Condition) string {
	return fmt.Sprintf("Type: %s, Status: %s, Reason: %s, Message: %s",
		condition.Type, condition.Status, condition.Reason, condition.Message)
}

func GetConditionsFromUnstructured(unstrObj *unstructured.Unstructured) ([]metav1.Condition, error) {
	objKind, objName := ObjKindName(unstrObj)

	// Iterate the status conditions and ensure each condition reports a "Ready"
	// status.
	unstrConditions, found, err := unstructured.NestedSlice(unstrObj.Object, "status", "conditions")
	if !found {
		return nil, fmt.Errorf("no status conditions found for %s: %s", objKind, objName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get status conditions for %s: %s: %w", objKind, objName, err)
	}

	conditions := make([]metav1.Condition, 0, len(unstrConditions))

	for _, condition := range unstrConditions {
		conditionMap, ok := condition.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected %s: %s condition to be type map[string]interface{}, got: %T",
				objKind, objName, conditionMap)
		}

		var c *metav1.Condition

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(conditionMap, &c); err != nil {
			return nil, fmt.Errorf("failed to convert condition map to metav1.Condition: %w", err)
		}

		conditions = append(conditions, *c)
	}

	return conditions, nil
}

// ValidateObjectNamePrefix checks if the given object name has the given prefix.
func ValidateObjectNamePrefix(unstrObj *unstructured.Unstructured, clusterName string) error {
	objKind, objName := ObjKindName(unstrObj)

	// Verify the machines are prefixed with the cluster name and fail
	// the test if they are not.
	if !strings.HasPrefix(objName, clusterName) {
		return fmt.Errorf("object %s %s does not have cluster name prefix: %s", objKind, objName, clusterName)
	}

	return nil
}

func ObjKindName(unstrObj *unstructured.Unstructured) (string, string) {
	return unstrObj.GetKind(), unstrObj.GetName()
}

func WarnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "Warning: %v\n", err)
}
