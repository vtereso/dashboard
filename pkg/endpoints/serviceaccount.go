package endpoints

import (
	"fmt"
	"net/http"
	"encoding/json"

	restful "github.com/emicklei/go-restful"

	logging "github.com/tektoncd/dashboard/pkg/logging"
	"github.com/tektoncd/dashboard/pkg/utils"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
)

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

// Only handles "op": "add"
func (r Resource) patchServiceAccount(request *restful.Request, response *restful.Response) {
	logging.Log.Debug("Inside patchServiceAccount")
	// Path parameters for specific SA
	namespace := request.PathParameter("namespace")
	name := request.PathParameter("name")
	
	// Secret being patched
	patchSecret := &v1.Secret{}
	if err := request.ReadEntity(&patchSecret); err != nil {
		logging.Log.Error("Error parsing request body on call to patchServiceAccount %s", err)
		utils.RespondError(response, err, http.StatusBadRequest)
		return
	}

	// Service Account being patched
	serviceAccount, err := r.K8sClient.CoreV1().ServiceAccounts(namespace).Get(name,metav1.GetOptions{})
	if err != nil {
		errorMessage := fmt.Sprintf("Error obtaining SA %s in %s namespace",name, namespace)
		utils.RespondMessageAndLogError(response, err, errorMessage, http.StatusBadRequest)
		return
	}

	var patchPath string
	switch patchSecret.Type {
	// ImagePullSecret
	case v1.SecretTypeDockerConfigJson:
		// Ensure ImagePullSecret does not already exist
		for _, imagePullSecret := range serviceAccount.ImagePullSecrets {
			if imagePullSecret.Name == patchSecret.Name {
				err := fmt.Errorf("%s ImagePullSecret already exists on SA %s\n",patchSecret.Name,serviceAccount.Name)
				utils.RespondError(response, err, http.StatusBadRequest)
			}
		}
		patchPath = "/spec/imagePullSecrets"
		if len(serviceAccount.ImagePullSecrets) != 0 {
			patchPath += fmt.Sprintf("/%d",len(serviceAccount.ImagePullSecrets))
		}
	case v1.SecretTypeBasicAuth:
		// Ensure Secret does not already exist
		for _, imagePullSecret := range serviceAccount.Secrets {
			if imagePullSecret.Name == patchSecret.Name {
				err := fmt.Errorf("%s Secret already exists on SA %s\n",patchSecret.Name,serviceAccount.Name)
				utils.RespondError(response, err, http.StatusBadRequest)
			}
		}
		patchPath = "/spec/secrets"
		if len(serviceAccount.Secrets) != 0 {
			patchPath += fmt.Sprintf("/%d",len(serviceAccount.ImagePullSecrets))
		}
	default:
		utils.RespondError(response, fmt.Errorf("Secret with type %s not supported for patching\n",patchSecret.Type), http.StatusBadRequest)
		return
	}

	//patchBuf := []byte(`[{ "op": "add", "path": `+patchPath+`, "value": { "name": `+patchSecret.Name+`} }]`)
	patchRequest := patchStringValue {
		Op: "add",
		Path: patchPath,
		Value: fmt.Sprintf("{ \"name\": %s }",patchSecret.Name),
	}
	patchBuf, _ := json.Marshal(&patchRequest)
	// Patch SA
	if _, err := r.K8sClient.CoreV1().ServiceAccounts(namespace).Patch(name,types.JSONPatchType,patchBuf); err != nil {
		logging.Log.Debugf("Failed to patch request: %s\n",string(patchBuf))
		utils.RespondError(response, err, http.StatusBadRequest)
		return
	}


	response.WriteHeader(204)
}