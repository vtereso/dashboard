/*
Copyright 2019 The Tekton Authors
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

package utils

import (
	"strings"

	restful "github.com/emicklei/go-restful"
	logging "github.com/tektoncd/dashboard/pkg/logging"
)

// RespondError - logs and writes an error response with a desired status code
func RespondError(response *restful.Response, err error, statusCode int) {
	logging.Log.Error("Error: ", strings.Replace(err.Error(), "/", "", -1))
	response.AddHeader("Content-Type", "text/plain")
	response.WriteError(statusCode, err)
}

// RespondErrorMessage - logs and writes an error message with a desired status code
func RespondErrorMessage(response *restful.Response, message string, statusCode int) {
	logging.Log.Debugf("Error message: %s", message)
	response.AddHeader("Content-Type", "text/plain")
	response.WriteErrorString(statusCode, message)
}

// RespondMessageAndLogError - logs and writes an error message with a desired status code and logs the error
func RespondMessageAndLogError(response *restful.Response, err error, message string, statusCode int) {
	logging.Log.Error("Error: ", strings.Replace(err.Error(), "/", "", -1))
	logging.Log.Debugf("Message: %s", message)
	response.AddHeader("Content-Type", "text/plain")
	response.WriteErrorString(statusCode, message)
}

func GetNamespace(request *restful.Request) string {
	namespace := request.PathParameter("namespace")
	if namespace == "*" {
		namespace = ""
	}
	return namespace
}

// url should be valid url of github repository
// If any of the return values are empty, there was an error processing the url parameter
func RepoSplit(url string) (server, org, repo string) {
	var urlNoPrefix string
	switch {
	case strings.HasPrefix(url,"https://"):
		urlNoPrefix = strings.TrimPrefix(url, "https://")
	case strings.HasPrefix(url,"http://"):
		urlNoPrefix = strings.TrimPrefix(url, "http://")
	default:
		return "", "", ""
	}
	// urlNoPrefix at this point: github.com/tektoncd/pipeline
	values := strings.Split(urlNoPrefix,"/")
	if len(values) != 3 {
		return "", "", ""
	}
	return values[0], values[1], values[2]
}