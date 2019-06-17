package endpoints_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	restful "github.com/emicklei/go-restful"
	. "github.com/tektoncd/dashboard/pkg/endpoints"
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Exclude routes that contain any of these substring values
var excludeRoutes []string = []string{
	"/v1/websockets", // No response code
	ExtensionRoot,    // Response codes dictated by extension logic
	"health",         // Returns 204
	"readiness",      // Returns 204
}

var methodRouteMap = make(map[string][]string) // k, v := HTTP_METHOD, []route
// Stores names of resources created using POST endpoints in case id/name is modified from payload (for example, pipelineRun)
var routeNameMap = make(map[string]string)

// Populate methodMap
// Router parameters are NOT replaced
func init() {
	server, _ := dummyServer()
	mux, _ := server.Config.Handler.(*restful.Container)
	for _, registeredWebservices := range mux.RegisteredWebServices() {
		for _, route := range registeredWebservices.Routes() {
			for _, excludeRoute := range excludeRoutes {
				if strings.Contains(route.Path, excludeRoute) {
					goto SkipRoute
				}
			}
			methodRouteMap[route.Method] = append(methodRouteMap[route.Method], route.Path)
		SkipRoute:
		}
	}
}

// Validates router contract for CRUD endpoints (status codes, headers, etc.)
func TestRouterContract(t *testing.T) {
	server, r := dummyServer()
	defer server.Close()

	namespace := "ns1"
	_, err := r.K8sClient.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	if err != nil {
		t.Fatalf("Error creating namespace '%s': %s\n", namespace, err)
	}
	// map traversal is random
	// GET/PUT/DELETE <resource>/{name} requires object exists (POST)
	orderedHttpKeys := []string{
		"POST",
		"GET",
		"PUT",
		"DELETE",
	}
	methodStatusMap := map[string]int{
		"POST":   201,
		"GET":    200,
		"PUT":    204,
		"DELETE": 204,
	}

	for _, httpMethod := range orderedHttpKeys {
		statusCode := methodStatusMap[httpMethod]
		t.Logf("Checking %s endpoints for %d\n", httpMethod, statusCode)
		for _, route := range methodRouteMap[httpMethod] {
			validateRoute(t, r, httpMethod, statusCode, server, route, namespace)
		}
	}
}

func validateRoute(t *testing.T, r *Resource, httpMethod string, expectedStatusCode int, server *httptest.Server, route, namespace string) {
	var httpRequestBody io.Reader
	namelessRoute := strings.TrimSuffix(route, "/{name}")
	resource := namelessRoute[strings.LastIndex(namelessRoute, "/")+1:]
	// Remove plural
	resource = strings.TrimSuffix(resource, "s")
	resourceName := fmt.Sprintf("fake%s", resource)

	// Grab stored name, if any
	storedResourceName, ok := routeNameMap[namelessRoute]
	if ok {
		resourceName = storedResourceName
	}

	// Make request body
	if httpMethod == "POST" || httpMethod == "PUT" {
		resourceObject := fakeStub(t, r, httpMethod, resource, namespace, resourceName)
		if resourceObject == nil {
			t.Fatalf("Fake stub does not exist for %s\n", resource)
		}
		jsonBytes, err := json.Marshal(resourceObject)
		if err != nil {
			t.Fatalf("Error marshalling %s resource\n", resource)
		}
		httpRequestBody = bytes.NewReader(jsonBytes)
	} else if httpMethod == "GET" {
		// GET/{name} route
		if strings.Contains(route, "{name}") {
			// This should return nil if there is no POST fakeStub implementation
			resourceObject := fakeStub(t, r, "POST", resource, namespace, resourceName)
			if resourceObject == nil {
				makeFake(t, r, resource, namespace, resourceName)
			}
		}
	}
	// Replace path params
	serverRoute := fmt.Sprintf("%s%s", server.URL, route)
	serverRoute = strings.Replace(serverRoute, "{namespace}", namespace, 1)
	serverRoute = strings.Replace(serverRoute, "{name}", resourceName, 1)

	httpReq := dummyHTTPRequest(httpMethod, serverRoute, httpRequestBody)
	response, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("Error getting response from %s: %v\n", serverRoute, err)
	}
	if response.StatusCode != expectedStatusCode {
		t.Logf("%s route %s failure\n", httpMethod, route)
		t.Fatalf("Response code %d did not equal expected %d\n", response.StatusCode, expectedStatusCode)
	}
	// Ensure "Content-Location" header routes to resource properly
	if httpMethod == "POST" {
		contentLocationSlice := response.Header["Content-Location"]
		if len(contentLocationSlice) == 0 {
			t.Fatalf("POST route %s did not set 'Content-Location' header", route)
		}
		// "Content-Location" header should be of the form /some/path/<resource>/name
		postedResourceName := contentLocationSlice[0][strings.LastIndex(contentLocationSlice[0], "/")+1:]
		t.Log("Posted resoure name:", postedResourceName)
		routeNameMap[route] = postedResourceName
		httpReq := dummyHTTPRequest("GET", fmt.Sprintf("%s%s", server.URL, contentLocationSlice[0]), httpRequestBody)
		_, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			t.Fatalf("Unable to locate resource as specified by 'Content-Location' header: %s", contentLocationSlice[0])
		}
	}
}

// ALL and ONLY resourceTypes with POST/PUT routes must implement
// Returns stub to be marshalled for dashboard route validation
func fakeStub(t *testing.T, r *Resource, httpMethod, resourceType, namespace, resourceName string) interface{} {
	t.Logf("Getting fake resource type: %s\n", resourceType)
	// Cases based on resource substring in mux routes (lowercase)
	switch resourceType {
	case "pipelinerun":
		// ManualPipelineRun require the referenced pipeline to have the same name as the pipeline being created
		switch httpMethod {
		case "POST":
			// Avoid creation collision when checking GET routes for stub
			_, err := r.PipelineClient.TektonV1alpha1().Pipelines(namespace).Get(resourceName, metav1.GetOptions{})
			// If pipeline does not exist
			if err != nil {
				// TODO: embed in pipelineRun once pipelineSpec is supported
				// Create pipeline ref for ManualPipelineRun
				pipeline := v1alpha1.Pipeline{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
					},
					Spec: v1alpha1.PipelineSpec{},
				}
				_, err := r.PipelineClient.TektonV1alpha1().Pipelines(namespace).Create(&pipeline)
				if err != nil {
					t.Fatalf("Error creating pipeline for pipelinerun: %v\n", err)
				}
			}
			return ManualPipelineRun{
				PIPELINENAME: resourceName,
			}
		case "PUT":
			return PipelineRunUpdateBody{
				v1alpha1.PipelineRunSpecStatusCancelled,
			}
		default:
			return nil
		}
	case "credential":
		credential := Credential{
			Name:        "fakeCredential",
			Username:    "thisismyusername",
			Password:    "password",
			Description: "cred de jour",
			URL: map[string]string{
				"tekton.dev/git-0": "https://github.com",
			},
		}
		if httpMethod == "PUT" {
			credential.Username = "updated"
			credential.Password = "updated"
			credential.Description = "updated"
		}
		return credential

	default:
		return nil
	}
}

// Creates resources for GET/{name} routes without corresponding POST to prevent lookup failures
func makeFake(t *testing.T, r *Resource, resourceType, namespace, resourceName string) {
	t.Logf("Making fake resource %s with name %s\n", resourceType, resourceName)
	switch resourceType {
	case "task":
		task := v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
			},
		}
		_, err := r.PipelineClient.TektonV1alpha1().Tasks(namespace).Create(&task)
		if err != nil {
			t.Fatalf("Error creating task: %v\n", err)
		}
	case "taskrun":
		taskRun := v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
			},
		}
		_, err := r.PipelineClient.TektonV1alpha1().TaskRuns(namespace).Create(&taskRun)
		if err != nil {
			t.Fatalf("Error creating taskRun: %v\n", err)
		}
	case "pipeline":
		pipeline := v1alpha1.Pipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
			},
		}
		_, err := r.PipelineClient.TektonV1alpha1().Pipelines(namespace).Create(&pipeline)
		if err != nil {
			t.Fatalf("Error creating pipeline: %v\n", err)
		}
	case "log":
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
			},
		}
		_, err := r.K8sClient.CoreV1().Pods(namespace).Create(&pod)
		if err != nil {
			t.Fatalf("Error creating pod: %v\n", err)
		}
	case "taskrunlog":
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					corev1.Container{
						Name: ContainerPrefix + "Container",
					},
				},
			},
		}
		_, err := r.K8sClient.CoreV1().Pods(namespace).Create(&pod)
		if err != nil {
			t.Fatalf("Error creating pod: %v\n", err)
		}
		taskRun := v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
			},
			Spec: v1alpha1.TaskRunSpec{},
			Status: v1alpha1.TaskRunStatus{
				PodName: resourceName,
			},
		}
		_, err = r.PipelineClient.TektonV1alpha1().TaskRuns(namespace).Create(&taskRun)
		if err != nil {
			t.Fatalf("Error creating taskRun: %v\n", err)
		}
	case "pipelinerunlog":
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					corev1.Container{
						Name: ContainerPrefix + "Container",
					},
				},
			},
		}
		_, err := r.K8sClient.CoreV1().Pods(namespace).Create(&pod)
		if err != nil {
			t.Fatalf("Error creating pod: %v\n", err)
		}
		taskRun := v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
			},
			Spec: v1alpha1.TaskRunSpec{},
			Status: v1alpha1.TaskRunStatus{
				PodName: resourceName,
			},
		}
		_, err = r.PipelineClient.TektonV1alpha1().TaskRuns(namespace).Create(&taskRun)
		if err != nil {
			t.Fatalf("Error creating taskRun: %v\n", err)
		}
		pipelineRun := v1alpha1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
			},
			Spec: v1alpha1.PipelineRunSpec{},
			Status: v1alpha1.PipelineRunStatus{
				TaskRuns: map[string]*v1alpha1.PipelineRunTaskRunStatus{
					resourceName: &v1alpha1.PipelineRunTaskRunStatus{
						Status: &v1alpha1.TaskRunStatus{
							PodName: resourceName,
						},
					},
				},
			},
		}
		_, err = r.PipelineClient.TektonV1alpha1().PipelineRuns(namespace).Create(&pipelineRun)
		if err != nil {
			t.Fatalf("Error creating pipelineRun: %v\n", err)
		}
	case "pipelineresource":
		pipelineResource := v1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: namespace,
			},
		}
		_, err := r.PipelineClient.TektonV1alpha1().PipelineResources(namespace).Create(&pipelineResource)
		if err != nil {
			t.Fatalf("Error creating pipelineResource: %v\n", err)
		}
	}
}

func TestExtensionRegistration(t *testing.T) {
	t.Log("Checking extension registration")
	server, r := dummyServer()
	defer server.Close()

	namespace := "ns1"
	_, err := r.K8sClient.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	if err != nil {
		t.Fatalf("Error creating namespace '%s': %s\n", namespace, err)
	}

	extensionEndpoints := []string{
		"secrets",
		"pipelineRuns",
		"robots",
	}
	extensionUrlValue := strings.Join(extensionEndpoints, ExtensionEndpointDelimiter)
	extensionServers := []*httptest.Server{
		dummyExtension(extensionEndpoints...),
		dummyExtension(),
	}
	var extensionPorts []int32
	for _, extensionServer := range extensionServers {
		port, _ := strconv.Atoi(getUrlPort(extensionServer.URL))
		extensionPorts = append(extensionPorts, int32(port))
	}

	extensionServices := []corev1.Service{
		corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "extension1",
				Annotations: map[string]string{
					ExtensionUrlKey:            extensionUrlValue,
					ExtensionBundleLocationKey: "Location",
					ExtensionDisplayNameKey:    "Display Name",
				},
				Labels: map[string]string{
					ExtensionLabelKey: ExtensionLabelValue,
				},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: "127.0.0.1",
				Ports: []corev1.ServicePort{
					{
						Port: extensionPorts[0],
					},
				},
			},
		},
		// No endpoints annotation
		corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "extension2",
				Annotations: map[string]string{
					ExtensionBundleLocationKey: "Location",
					ExtensionDisplayNameKey:    "Display Name",
				},
				Labels: map[string]string{
					ExtensionLabelKey: ExtensionLabelValue,
				},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: "127.0.0.1",
				Ports: []corev1.ServicePort{
					{
						Port: extensionPorts[1],
					},
				},
			},
		},
	}

	nonExtensionServices := []corev1.Service{
		corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "none-extension",
				Annotations: map[string]string{
					ExtensionUrlKey:            "/path",
					ExtensionBundleLocationKey: "Location",
					ExtensionDisplayNameKey:    "Display Name",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 9097}},
			},
		},
	}
	services := append(extensionServices, nonExtensionServices...)
	for _, svc := range services {
		_, err := r.K8sClient.CoreV1().Services(namespace).Create(&svc)
		if err != nil {
			t.Fatalf("Error creating service '%s': %v\n", svc.Name, err)
		}
	}

	// Wait until all extensions are registered by informer
	for {
		ExtensionMutex.RLock()
		extensions := len(ExtensionMap)
		ExtensionMutex.RUnlock()
		if extensions == len(extensionServices) {
			break
		}
	}

	// Labels not supported on fake client, manual filter
	serviceList, err := r.K8sClient.CoreV1().Services(namespace).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Error obtaining services from K8sClient")
	}

	for _, service := range serviceList.Items {
		if value := service.Labels[ExtensionLabelKey]; value == ExtensionLabelValue {
			// Grab endpoints from service annotation if any
			endpoints := strings.Split(service.Annotations[ExtensionUrlKey], ExtensionEndpointDelimiter)
			if len(endpoints) == 0 {
				endpoints = []string{""}
			}
			httpMethods := []string{"GET", "POST", "PUT", "DELETE"}
			// Look for response from registered routes
			for _, endpoint := range endpoints {
				for _, method := range httpMethods {
					proxyUrl := strings.TrimSuffix(fmt.Sprintf("%s/%s/%s/%s", server.URL, ExtensionRoot, service.Name, endpoint), "/")
					httpReq := dummyHTTPRequest(method, proxyUrl, nil)
					_, err := http.DefaultClient.Do(httpReq)
					if err != nil {
						t.Fatalf("Error getting response for %s with http method %s: %v\n", proxyUrl, method, err)
					}
					// Test registered route subroute
					proxySubroute := proxyUrl + "/subroute"
					httpReq = dummyHTTPRequest(method, proxySubroute, nil)
					_, err = http.DefaultClient.Do(httpReq)
					if err != nil {
						t.Fatalf("Error getting response for %s with http method %s: %v\n", proxySubroute, method, err)
					}
				}
			}
		}
	}

	// Test getAllExtensions function
	httpReq := dummyHTTPRequest("GET", fmt.Sprintf("%s/%s", server.URL, ExtensionRoot), nil)
	response, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("Error getting response for getAllExtensions: %v\n", err)
	}
	responseExtensions := []Extension{}
	if err := json.NewDecoder(response.Body).Decode(&responseExtensions); err != nil {
		t.Fatalf("Error decoding getAllExtensions response: %v\n", err)
	}

	// Verify the response
	if len(responseExtensions) != len(extensionServices) {
		t.Fatalf("Number of entries: expected: %d, returned: %d", len(extensionServices), len(responseExtensions))
	}
}

// Incoming url should be of form http://ipaddr:port with no trailing slash
func getUrlPort(u string) string {
	url, err := url.ParseRequestURI(u)
	if err != nil {
		return ""
	}
	return url.Port()
}
