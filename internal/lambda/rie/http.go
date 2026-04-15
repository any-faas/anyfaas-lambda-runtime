// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/fatalerror"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda/rapidcore/env"
)

// HandlerRequest represents a request to set/update the handler
type HandlerRequest struct {
	Handler string            `json:"handler"`
	Runtime string            `json:"runtime,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// setHandlerResponse represents the response for setting handler
type setHandlerResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// supportedRuntimes is the list of runtime values supported by the setHandler API
// These match the official AWS Lambda runtime identifiers
var supportedRuntimes = []string{
	"nodejs20.x", "nodejs22.x", "nodejs24.x",
	"python3.10", "python3.11", "python3.12", "python3.13", "python3.14",
}

// isValidRuntime checks if the runtime value is supported
func isValidRuntime(runtime string) bool {
	return slices.Contains(supportedRuntimes, runtime)
}

// getBootstrapCmd returns the bootstrap command based on the runtime
func getBootstrapCmd(runtime string) []string {
	switch runtime {
	case "nodejs20.x":
		return []string{"/var/runtime/nodejs20/bootstrap"}
	case "nodejs22.x", "":
		return []string{"/var/runtime/bootstrap"}
	case "nodejs24.x":
		return []string{"/var/runtime/nodejs24/bootstrap"}
	case "python3.10":
		return []string{"/var/lang/bin/python3.10", "/var/runtime/python3.10-bootstrap.py"}
	case "python3.11":
		return []string{"/var/lang/bin/python3.11", "/var/runtime/python3.11-bootstrap.py"}
	case "python3.12":
		return []string{"/var/lang/bin/python3.12", "/var/runtime/python3.12-bootstrap.py"}
	case "python3.13":
		return []string{"/var/lang/bin/python3.13", "/var/runtime/python3.13-bootstrap.py"}
	case "python3.14":
		return []string{"/var/lang/bin/python3.14", "/var/runtime/python3.14-bootstrap.py"}
	default:
		return []string{"/var/runtime/bootstrap"}
	}
}

// Runtime bootstrap state - simplified to always use the wrapper
type runtimeBootstrapState struct {
	workingDir string
}

func startHTTPServer(ipport string, sandbox *rapidcore.SandboxBuilder, bs interop.Bootstrap) {
	// Use a simple bootstrap that always uses /var/runtime/bootstrap wrapper
	// The wrapper script will select the correct runtime based on AWS_LAMBDA_FUNCTION_RUNTIME
	runtimeState := &runtimeBootstrapState{
		workingDir: "/var/task",
	}

	// Create a bootstrap that uses the wrapper
	dynamicBootstrap := &dynamicBootstrap{state: runtimeState}

	srv := &http.Server{
		Addr: ipport,
	}

	// Pass a channel
	http.HandleFunc("/2015-03-31/functions/function/invocations", func(w http.ResponseWriter, r *http.Request) {
		InvokeHandler(w, r, sandbox.LambdaInvokeAPI(), dynamicBootstrap)
	})

	// API endpoint to set/update handler and runtime (for container pool reuse)
	http.HandleFunc("/2015-03-31/functions/function/handler", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req HandlerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(setHandlerResponse{
				Success: false,
				Message: "Invalid request body: " + err.Error(),
			})
			return
		}

		if req.Handler == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(setHandlerResponse{
				Success: false,
				Message: "Handler cannot be empty",
			})
			return
		}

		// Determine the runtime
		runtime := req.Runtime
		if runtime == "" {
			runtime = os.Getenv("AWS_LAMBDA_FUNCTION_RUNTIME")
		}

		// Validate the runtime
		if !isValidRuntime(runtime) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(setHandlerResponse{
				Success: false,
				Message: fmt.Sprintf("Unsupported runtime: %s. Supported runtimes: %v", runtime, supportedRuntimes),
			})
			return
		}

		// Set the handler in environment variable for next invocation
		// The bootstrap wrapper at /var/runtime/bootstrap will read this and select the correct runtime
		log.Infof("Setting handler to: %s, runtime: %s", req.Handler, runtime)
		os.Setenv("AWS_LAMBDA_FUNCTION_HANDLER", req.Handler)
		os.Setenv("_HANDLER", req.Handler)
		os.Setenv("AWS_LAMBDA_FUNCTION_RUNTIME", runtime)

		// Set custom environment variables
		for k, v := range req.Env {
			os.Setenv(k, v)
			log.Infof("Set custom env var: %s=%s", k, v)
		}

		// Set PYTHONPATH for Python runtimes (needed for the wrapper to find the Lambda runtime)
		// All libraries are now in site-packages following standard structure
		switch runtime {
		case "python3.10":
			os.Setenv("PYTHONPATH", "/var/lang/python3.10/lib/python3.10/site-packages:/var/task")
			os.Setenv("AWS_EXECUTION_ENV", "AWS_Lambda_python3.10")
		case "python3.11":
			os.Setenv("PYTHONPATH", "/var/lang/python3.11/lib/python3.11/site-packages:/var/task")
			os.Setenv("AWS_EXECUTION_ENV", "AWS_Lambda_python3.11")
		case "python3.12":
			os.Setenv("PYTHONPATH", "/var/lang/lib/python3.12/site-packages:/var/task")
			os.Setenv("AWS_EXECUTION_ENV", "AWS_Lambda_python3.12")
		case "python3.13":
			os.Setenv("PYTHONPATH", "/var/lang/python3.13/lib/python3.13/site-packages:/var/task")
			os.Setenv("AWS_EXECUTION_ENV", "AWS_Lambda_python3.13")
		case "python3.14":
			os.Setenv("PYTHONPATH", "/var/lang/python3.14/lib/python3.14/site-packages:/var/task")
			os.Setenv("AWS_EXECUTION_ENV", "AWS_Lambda_python3.14")
		}

		log.Infof("Handler set: runtime=%s, handler=%s", runtime, req.Handler)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(setHandlerResponse{
			Success: true,
			Message: fmt.Sprintf("Handler updated, runtime=%s, reset initiated", runtime),
		})
	})

	// go routine (main thread waits)
	if err := srv.ListenAndServe(); err != nil {
		log.Panic(err)
	}

	log.Warnf("Listening on %s", ipport)
}

// dynamicBootstrap is a bootstrap that can be updated at runtime
type dynamicBootstrap struct {
	state *runtimeBootstrapState
}

func (b *dynamicBootstrap) Cmd() ([]string, error) {
	// Always use the bootstrap wrapper at /var/runtime/bootstrap
	// The wrapper will select the correct runtime based on AWS_LAMBDA_FUNCTION_RUNTIME
	return []string{"/var/runtime/bootstrap"}, nil
}

func (b *dynamicBootstrap) Cwd() (string, error) {
	return b.state.workingDir, nil
}

func (b *dynamicBootstrap) Env(e *env.Environment) map[string]string {
	// Get the base runtime exec env
	envVars := e.RuntimeExecEnv()

	// Add custom env vars from os environment (set by setHandler)
	// These include PYTHONPATH, AWS_EXECUTION_ENV, etc.
	runtime := os.Getenv("AWS_LAMBDA_FUNCTION_RUNTIME")
	fmt.Printf("DEBUG dynamicBootstrap.Env: runtime from env = %s\n", runtime)
	log.Infof("dynamicBootstrap.Env: runtime=%s", runtime)

	// System-reserved env var prefixes that should not be overridden by user
	reservedPrefixes := []string{
		"AWS_LAMBDA_",
		"LAMBDA_",
		"AWS_EXECUTION_ENV",
		"AWS_ACCESS_KEY",
		"AWS_SECRET_KEY",
		"AWS_SESSION_TOKEN",
	}

	// Check if a key is a user-defined env var (not system-reserved)
	isUserEnv := func(key string) bool {
		for _, prefix := range reservedPrefixes {
			if strings.HasPrefix(key, prefix) {
				return false
			}
		}
		return true
	}

	if runtime != "" {
		for _, env := range os.Environ() {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				// Add Python-related env vars and custom user env vars
				if strings.HasPrefix(key, "PYTHON") ||
				   key == "AWS_EXECUTION_ENV" ||
				   key == "AWS_LAMBDA_FUNCTION_RUNTIME" ||
				   key == "AWS_LAMBDA_FUNCTION_HANDLER" ||
				   isUserEnv(key) {
					envVars[key] = parts[1]
					log.Infof("dynamicBootstrap.Env: adding %s=%s", key, parts[1])
				}
			}
		}
	}

	return envVars
}

func (b *dynamicBootstrap) ExtraFiles() []*os.File {
	return make([]*os.File, 0)
}

func (b *dynamicBootstrap) CachedFatalError(err error) (fatalerror.ErrorType, string, bool) {
	return fatalerror.ErrorType(""), "", false
}
