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
package endpoints_test

import (
	restful "github.com/emicklei/go-restful"
	k8scontroller "github.com/tektoncd/dashboard/pkg/controllers/kubernetes"
	tektoncontroller "github.com/tektoncd/dashboard/pkg/controllers/tekton"
	. "github.com/tektoncd/dashboard/pkg/endpoints"
	logging "github.com/tektoncd/dashboard/pkg/logging"
	fakeclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	tektoninformers "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	"io"
	k8sinformers "k8s.io/client-go/informers"
	fakek8sclientset "k8s.io/client-go/kubernetes/fake"
	"net/http"
	"net/http/httptest"
	"time"
)

func dummyK8sClientset() *fakek8sclientset.Clientset {
	result := fakek8sclientset.NewSimpleClientset()
	return result
}

func dummyClientset() *fakeclientset.Clientset {
	result := fakeclientset.NewSimpleClientset()
	return result
}

func dummyResource() *Resource {
	resource := Resource{
		PipelineClient: dummyClientset(),
		K8sClient:      dummyK8sClientset(),
	}
	return &resource
}

func dummyHTTPRequest(method string, url string, body io.Reader) *http.Request {
	httpReq, _ := http.NewRequest(method, url, body)
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq
}

// Use alongside DummyHTTPRequest
func dummyServer() (*httptest.Server, *Resource) {
	wsContainer := restful.NewContainer()
	resource := dummyResource()
	resource.RegisterWeb(wsContainer)
	resource.RegisterEndpoints(wsContainer)
	resource.RegisterWebsocket(wsContainer)
	resource.RegisterHealthProbe(wsContainer)
	resource.RegisterReadinessProbe(wsContainer)
	RegisterExtensions(wsContainer)
	// K8s signals only allows for a single channel, which will panic when executed twice
	// There should be no os signals for testing purposes
	stopCh := make(<-chan struct{})
	resyncDur := time.Second * 30
	tektonInformerFactory := tektoninformers.NewSharedInformerFactory(resource.PipelineClient, resyncDur)
	kubeInformerFactory := k8sinformers.NewSharedInformerFactory(resource.K8sClient, resyncDur)
	// Add all controllers
	tektoncontroller.NewTaskRunController(tektonInformerFactory, stopCh)
	tektoncontroller.NewPipelineRunController(tektonInformerFactory, stopCh)
	// Extension controller needs restful router + clients since it will be dynamically adding/removing routes
	k8scontroller.NewExtensionController(kubeInformerFactory, stopCh, wsContainer)
	k8scontroller.NewNamespaceController(kubeInformerFactory, stopCh)
	logging.Log.Info("Starting controllers")
	tektonInformerFactory.Start(stopCh)
	kubeInformerFactory.Start(stopCh)

	server := httptest.NewServer(wsContainer)
	return server, resource
}

func dummyExtension(endpoints ...string) *httptest.Server {
	if len(endpoints) == 0 {
		endpoints = []string{""}
	}
	wsContainer := restful.NewContainer()
	ws := new(restful.WebService)
	ws.
		Path("/").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	// Stub function to validate extension route active
	routeFunction := func(request *restful.Request, response *restful.Response) {
		response.WriteHeader(http.StatusOK)
	}

	for _, endpoint := range endpoints {
		ws.Route(ws.GET(endpoint).To(routeFunction))
		ws.Route(ws.POST(endpoint).To(routeFunction))
		ws.Route(ws.PUT(endpoint).To(routeFunction))
		ws.Route(ws.DELETE(endpoint).To(routeFunction))
	}
	wsContainer.Add(ws)
	return httptest.NewServer(wsContainer)
}
