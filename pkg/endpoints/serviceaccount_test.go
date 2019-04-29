package endpoints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	restful "github.com/emicklei/go-restful"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPatchServiceAccount(t *testing.T) {
	// Set Up
	const namespace string = "fake"
	wsContainer := restful.NewContainer()

	resource := dummyResource()
	resource.K8sClient.CoreV1().Namespaces().Create(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	resource.RegisterEndpoints(wsContainer)
	server = httptest.NewServer(wsContainer)

	serviceAccount := &v1.ServiceAccount{}
	serviceAccount.Name = "fake-sa"
	if _, err := resource.K8sClient.CoreV1().ServiceAccounts(namespace).Create(serviceAccount); err != nil {
		t.Fatal("Error creating service account: ",err)
	}

	// Test
	basicAuthSecrets := []*v1.Secret {
		createSecret("basicAuth1",v1.SecretTypeBasicAuth),
		createSecret("basicAuth2",v1.SecretTypeBasicAuth),
		createSecret("imagePull1",v1.SecretTypeDockerConfigJson),
		createSecret("imagePull2",v1.SecretTypeDockerConfigJson),
	}

	imagePullSecrets := []*v1.Secret {
		createSecret("imagePull1",v1.SecretTypeDockerConfigJson),
		createSecret("imagePull2",v1.SecretTypeDockerConfigJson),
	}

	patchServiceAccount := func(secrets []*v1.Secret) {
		for _, secret := range secrets {
			mBytes, _ := json.Marshal(secret)
			httpReq := dummyHTTPRequest(http.MethodPatch, fmt.Sprintf("%s/v1/namespaces/%s/serviceaccount/%s",server.URL,namespace,serviceAccount.Name), bytes.NewReader(mBytes))
			response, _ := http.DefaultClient.Do(httpReq)
			if response.StatusCode != http.StatusNoContent {
				t.Errorf("Status code for patching secret %s was %d not %d\n",secret.Name,response.StatusCode,http.StatusNoContent)
			}
		}
	}
	patchServiceAccount(basicAuthSecrets)
	patchServiceAccount(imagePullSecrets)
	// serviceAccount, err := resource.K8sClient.CoreV1().ServiceAccounts(namespace).Get(serviceAccount.Name,metav1.GetOptions{})
	// if err != nil {
	// 	t.Fatalf("Unable to locate service account %s\n",serviceAccount.Name)
	// }
	// t.Logf("Service account - Secrets: %v\n",serviceAccount.Secrets)
	// t.Logf("Service account - Image Pull Secrets: %v\n",serviceAccount.ImagePullSecrets)

	// if len(serviceAccount.Secrets) != len(basicAuthSecrets) {
	// 	t.Error("Service account does not have all basic auth secrets expected")
	// }
	// if len(serviceAccount.ImagePullSecrets) != len(imagePullSecrets) {
	// 	t.Error("Service account does not have all image pull secrets expected")
	// }
}

func createSecret(name string, secretType v1.SecretType) *v1.Secret {
	secret := &v1.Secret{
		Type: secretType,
	}
	secret.Name = name
	return secret
}