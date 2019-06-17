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

package endpoints

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	restful "github.com/emicklei/go-restful"
	logging "github.com/tektoncd/dashboard/pkg/logging"
	"github.com/tektoncd/dashboard/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

// Label key required by services to be registered as a dashboard extension
const ExtensionLabelKey = "tekton-dashboard-extension"

// Label value required by services to be registered as a dashboard extension
const ExtensionLabelValue = "true"

// Full label required by services to be registered as a dashboard extension
const ExtensionLabel = ExtensionLabelKey + "=" + ExtensionLabelValue

// Annotation that specifies the valid extension path, defaults to "/"
const ExtensionUrlKey = "tekton-dashboard-endpoints"

// Delimiter to be used between the extension endpoints annotation value
const ExtensionEndpointDelimiter = "."

// Extension UI bundle location annotation
const ExtensionBundleLocationKey = "tekton-dashboard-bundle-location"

// Extension display name annotation
const ExtensionDisplayNameKey = "tekton-dashboard-display-name"

// extensionRoot
const ExtensionRoot = "/v1/extensions"

var webResourcesDir = os.Getenv("WEB_RESOURCES_DIR")

func (r Resource) RegisterWeb(container *restful.Container) {
	logging.Log.Info("Adding web api")

	container.Handle("/", http.FileServer(http.Dir(webResourcesDir)))
}

// Router rules
// Does not apply to websocket/extensions/probes
// GET:    200
// POST:   201
// PUT:    204
// Delete: 204

// Register APIs to interface with core Tekton/K8s pieces
func (r Resource) RegisterEndpoints(container *restful.Container) {
	wsv1 := new(restful.WebService)
	wsv1.
		Path("/v1/namespaces").
		Consumes(restful.MIME_JSON, "text/plain").
		Produces(restful.MIME_JSON, "text/plain")

	logging.Log.Info("Adding v1, and API for k8s resources and pipelines")

	wsv1.Route(wsv1.GET("/").To(r.getAllNamespaces))
	wsv1.Route(wsv1.GET("/{namespace}/pipelines").To(r.getAllPipelines))
	wsv1.Route(wsv1.GET("/{namespace}/pipelines/{name}").To(r.getPipeline))

	wsv1.Route(wsv1.GET("/{namespace}/pipelineruns").To(r.getAllPipelineRuns))
	wsv1.Route(wsv1.GET("/{namespace}/pipelineruns/{name}").To(r.getPipelineRun))
	wsv1.Route(wsv1.PUT("/{namespace}/pipelineruns/{name}").To(r.updatePipelineRun))
	wsv1.Route(wsv1.POST("/{namespace}/pipelineruns").To(r.createPipelineRun))

	wsv1.Route(wsv1.GET("/{namespace}/pipelineresources").To(r.getAllPipelineResources))
	wsv1.Route(wsv1.GET("/{namespace}/pipelineresources/{name}").To(r.getPipelineResource))

	wsv1.Route(wsv1.GET("/{namespace}/tasks").To(r.getAllTasks))
	wsv1.Route(wsv1.GET("/{namespace}/tasks/{name}").To(r.getTask))

	wsv1.Route(wsv1.GET("/{namespace}/taskruns").To(r.getAllTaskRuns))
	wsv1.Route(wsv1.GET("/{namespace}/taskruns/{name}").To(r.getTaskRun))

	wsv1.Route(wsv1.GET("/{namespace}/serviceaccounts").To(r.getAllServiceAccounts))

	wsv1.Route(wsv1.GET("/{namespace}/logs/{name}").To(r.getPodLog))

	wsv1.Route(wsv1.GET("/{namespace}/taskrunlogs/{name}").To(r.getTaskRunLog))

	wsv1.Route(wsv1.GET("/{namespace}/pipelinerunlogs/{name}").To(r.getPipelineRunLog))

	wsv1.Route(wsv1.GET("/{namespace}/credentials").To(r.getAllCredentials))
	wsv1.Route(wsv1.GET("/{namespace}/credentials/{name}").To(r.getCredential))
	wsv1.Route(wsv1.POST("/{namespace}/credentials").To(r.createCredential))
	wsv1.Route(wsv1.PUT("/{namespace}/credentials/{name}").To(r.updateCredential))
	wsv1.Route(wsv1.DELETE("/{namespace}/credentials/{name}").To(r.deleteCredential))

	container.Add(wsv1)
}

// RegisterWebsocket - this registers a websocket with which we can send log information to
func (r Resource) RegisterWebsocket(container *restful.Container) {
	logging.Log.Info("Adding API for websocket")
	wsv2 := new(restful.WebService)
	wsv2.
		Path("/v1/websockets").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	wsv2.Route(wsv2.GET("/logs").To(r.establishPipelineLogsWebsocket))
	wsv2.Route(wsv2.GET("/resources").To(r.establishResourcesWebsocket))
	container.Add(wsv2)
}

// RegisterHealthProbes - this registers the /health endpoint
func (r Resource) RegisterHealthProbe(container *restful.Container) {
	logging.Log.Info("Adding API for health")
	wsv3 := new(restful.WebService)
	wsv3.
		Path("/health")

	wsv3.Route(wsv3.GET("").To(r.checkHealth))

	container.Add(wsv3)
}

// RegisterReadinessProbes - this registers the /readiness endpoint
func (r Resource) RegisterReadinessProbe(container *restful.Container) {
	logging.Log.Info("Adding API for readiness")
	wsv4 := new(restful.WebService)
	wsv4.
		Path("/readiness")

	wsv4.Route(wsv4.GET("").To(r.checkHealth))

	container.Add(wsv4)
}

// Back-end extension: Requests to the URL are passed through to the Port of the Name service (extension)
// "label: tekton-dashboard-extension=true" in the service defines the extension
// "annotation: tekton-dashboard-endpoints=<URL>" specifies the path for the extension
type Extension struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Port           string `json:"port"`
	DisplayName    string `json:"displayname"`
	BundleLocation string `json:"bundlelocation"`
}

// Informer only receives services and must be able to map to WebService/Extension for unregister
var ExtensionMutex *sync.RWMutex = new(sync.RWMutex)
var serviceMap map[*corev1.Service]*restful.WebService = make(map[*corev1.Service]*restful.WebService)
var ExtensionMap map[*corev1.Service]Extension = make(map[*corev1.Service]Extension)

// Registers the endpoint to get all CURRENTLY registered extensions
func RegisterExtensions(container *restful.Container) {
	ws := new(restful.WebService)
	ws.
		Path(ExtensionRoot).
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ws.Route(ws.GET("").To(getAllExtensions))
	container.Add(ws)
}

/* Get all extensions in the installed namespace */
func getAllExtensions(request *restful.Request, response *restful.Response) {
	logging.Log.Debugf("In getAllExtensions")

	response.AddHeader("Content-Type", "application/json")
	extensions := []Extension{}
	ExtensionMutex.RLock()
	defer ExtensionMutex.RUnlock()
	for _, e := range ExtensionMap {
		extensions = append(extensions, e)
	}
	logging.Log.Debugf("Extension: %+v", extensions)
	response.WriteEntity(extensions)
}

// Add a discovered extension as a webservice (that must have a unique rootPath) to the container/mux
func RegisterExtension(container *restful.Container, extensionService *corev1.Service) {
	logging.Log.Infof("Adding extension %s", extensionService.Name)

	ws := new(restful.WebService)
	ws.
		Path(ExtensionRoot + "/" + extensionService.ObjectMeta.Name).
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	url, ok := extensionService.ObjectMeta.Annotations[ExtensionUrlKey]
	ext := Extension{
		Name:           extensionService.ObjectMeta.Name,
		URL:            extensionService.Spec.ClusterIP,
		Port:           getServicePort(*extensionService),
		DisplayName:    extensionService.ObjectMeta.Annotations[ExtensionDisplayNameKey],
		BundleLocation: extensionService.ObjectMeta.Annotations[ExtensionBundleLocationKey],
	}
	// Service does not have a ClusterIP
	if ext.URL == "" {
		logging.Log.Errorf("Extension service %s does not have ClusterIP. Cannot add route", extensionService.Name)
		return
	}

	// Base extension path
	paths := []string{""}
	if ok {
		if len(url) != 0 {
			paths = strings.Split(url, ExtensionEndpointDelimiter)
		}
	}
	ExtensionMutex.Lock()
	defer ExtensionMutex.Unlock()
	// Add routes for extension service
	for _, path := range paths {
		debugFullPath := strings.TrimSuffix(ws.RootPath()+"/"+path, "/")
		logging.Log.Debugf("Registering path: %s", debugFullPath)
		ws.Route(ws.GET(path).To(ext.handleExtension))
		ws.Route(ws.POST(path).To(ext.handleExtension))
		ws.Route(ws.PUT(path).To(ext.handleExtension))
		ws.Route(ws.DELETE(path).To(ext.handleExtension))
		// Route to all subroutes
		ws.Route(ws.GET(path + "/{var:*}").To(ext.handleExtension))
		ws.Route(ws.POST(path + "/{var:*}").To(ext.handleExtension))
		ws.Route(ws.PUT(path + "/{var:*}").To(ext.handleExtension))
		ws.Route(ws.DELETE(path + "/{var:*}").To(ext.handleExtension))
	}
	ExtensionMap[extensionService] = ext
	serviceMap[extensionService] = ws
	container.Add(ws)
}

// Should be called PRIOR to registration of extensionService on informer update
func UnregisterExtension(container *restful.Container, extensionService *corev1.Service) {
	logging.Log.Infof("Removing extension %s", extensionService.Name)
	ExtensionMutex.Lock()
	defer ExtensionMutex.Unlock()
	extensionWebService := serviceMap[extensionService]
	container.Remove(extensionWebService)
	delete(ExtensionMap, extensionService)
	delete(serviceMap, extensionService)
}

// HandleExtension - this routes request to the extension service
func (ext Extension) handleExtension(request *restful.Request, response *restful.Response) {
	target, err := url.Parse("http://" + ext.URL + ":" + ext.Port + "/")
	if err != nil {
		utils.RespondError(response, err, http.StatusInternalServerError)
		return
	}
	logging.Log.Debugf("Path in URL: %+v", request.Request.URL.Path)
	request.Request.URL.Path = strings.TrimPrefix(request.Request.URL.Path, fmt.Sprintf("%s/%s", ExtensionRoot, ext.Name))
	logging.Log.Debugf("Path in rerouting URL: %+v", request.Request.URL.Path)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ServeHTTP(response, request.Request)
}

// Returns target port if exists, else source == target
func getServicePort(svc corev1.Service) string {
	if svc.Spec.Ports[0].TargetPort.StrVal != "" {
		return svc.Spec.Ports[0].TargetPort.String()
	}
	return strconv.Itoa(int(svc.Spec.Ports[0].Port))
}

// Write Content-Location header within POST methods and set StatusCode to 201
// Headers MUST be set before writing to body (if any) to succeed
func writeResponseLocation(request *restful.Request, response *restful.Response, identifier string) {
	location := request.Request.URL.Path
	if request.Request.Method == http.MethodPost {
		location = location + "/" + identifier
	}
	response.AddHeader("Content-Location", location)
	response.WriteHeader(201)
}
