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
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GET namespaces
func TestGETAllNamespaces(t *testing.T) {
	server, r := dummyServer()
	defer server.Close()

	namespaces := []string{
		"ns1",
	}
	for _, namespace := range namespaces {
		_, err := r.K8sClient.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
		if err != nil {
			t.Fatalf("Error creating namespace '%s': %v\n", namespace, err)
		}
	}

	httpReq := dummyHTTPRequest("GET", fmt.Sprintf("%s/v1/namespaces", server.URL), nil)
	response, _ := http.DefaultClient.Do(httpReq)
	responseNamespaceList := corev1.NamespaceList{}
	if err := json.NewDecoder(response.Body).Decode(&responseNamespaceList); err != nil {
		t.Fatalf("Error decoding getAllNamespaces response: %v\n", err)
	} else {
		if len(responseNamespaceList.Items) != len(namespaces) {
			t.Errorf("All expected namespaces were not returned: expected %v, actual %v", len(namespaces), len(responseNamespaceList.Items))
		}
	}
}

// GET serviceaccounts
func TestGETAllServiceAccounts(t *testing.T) {
	server, r := dummyServer()
	defer server.Close()

	namespace := "ns1"
	_, err := r.K8sClient.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	if err != nil {
		t.Fatalf("Error creating namespace '%s': %v\n", namespace, err)
	}

	serviceAccounts := []corev1.ServiceAccount{
		corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-sa",
			},
		},
	}
	for _, serviceAccount := range serviceAccounts {
		_, err := r.K8sClient.CoreV1().ServiceAccounts(namespace).Create(&serviceAccount)
		if err != nil {
			t.Fatalf("Error creating serviceAccount '%s': %v\n", serviceAccount.Name, err)
		}
	}

	httpReq := dummyHTTPRequest("GET", fmt.Sprintf("%s/v1/namespaces/%s/serviceaccounts", server.URL, namespace), nil)
	response, _ := http.DefaultClient.Do(httpReq)
	responseServiceAccountList := corev1.ServiceAccountList{}
	if err := json.NewDecoder(response.Body).Decode(&responseServiceAccountList); err != nil {
		t.Fatalf("Error decoding getAllServiceAccounts response: %v\n", err)
	} else {
		if len(responseServiceAccountList.Items) != len(serviceAccounts) {
			t.Errorf("All expected serviceAccounts were not returned: expected %v, actual %v", len(serviceAccounts), len(responseServiceAccountList.Items))
		}
	}

}
