package endpoints

import (
	"fmt"
	restful "github.com/emicklei/go-restful"

	logging "github.com/tektoncd/dashboard/pkg/logging"
	"github.com/tektoncd/dashboard/pkg/utils"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
)

func (r Resource) patchServiceAccount(request *restful.Request, response *restful.Response) {
	// Path parameters for specific SA
	namespace := request.PathParameter("namespace")
	name := request.PathParameter("name")
	
	// Secret being patched
	patchSecret := &v1.Secret{}
	if err := request.ReadEntity(&patchSecret) {
		logging.Log.Error("Error parsing request body on call to patchServiceAccount %s", err)
		utils.RespondError(response, err, http.StatusBadRequest)
		return
	}

	// Service Account being patched
	serviceAccount, err := r.K8sClient.CoreV1().ServiceAccounts(namespace).Get(name)
	if err != nil {
		errorMessage := fmt.Sprintf("Error obtaining SA %s in %s namespace. K8sClient error: %s",name, namespace, err.Error())
		utils.RespondMessageAndLogError(response, err, errorMessage, http.StatusBadRequest)
		return
	}

	var patchPath string
	// Add as the last element
	getPathPostfix := func(array []interface{}) string {
		if len(arr) != 0 {
			return fmt.Sprintf("/%d",/len(arr))
		}
	}
	switch patchSecret.Type {
	// ImagePullSecret
	case v1.SecretTypeDockerConfigJson:
		// Ensure ImagePullSecret does not already exist
		for _, imagePullSecret := range serviceAccount.ImagePullSecret {
			if imagePullSecret.Name == patchSecret.Name {
				err := fmt.Errorf("%s ImagePullSecret already exists on SA %d",patchSecret.Name,serviceAccount.Name)
				utils.RespondMessageAndLogError(response, err, errorMessage, http.StatusBadRequest)
			}
		}
		patchPatch = "/imagePullSecrets" + getPathPostfix(serviceAccount.ImagePullSecret)
	case v1.SecretTypeBasicAuth:
		// Ensure Secret does not already exist
		for _, imagePullSecret := range serviceAccount.Secrets {
			if imagePullSecret.Name == patchSecret.Name {
				err := fmt.Errorf("%s Secret already exists on SA %d",patchSecret.Name,serviceAccount.Name)
				utils.RespondMessageAndLogError(response, err, errorMessage, http.StatusBadRequest)
			}
		}
		patchPatch = "/secrets" + getPathPostfix(serviceAccount.Secrets)
	default:
		utils.RespondMessageAndLogError(response, fmt.Errorf("Secret with type %s not supported for patching",patchPatch.Type), http.StatusBadRequest)
		return
	}


	patchBuf := []byte("[{ \"op\": \"add\", \"path\": "+patchPatch+", \"value\": { \"name\": "+patchSecret.Name+"} }]")
	// Patch SA
	if _, err := r.K8sClient.CoreV1().ServiceAccounts(namespace).Patch(name,types.JSONPatchType,patchBuf); err != nil {
		utils.RespondMessageAndLogError(response, err, errorMessage, http.StatusBadRequest)
		return
	}
	response.WriteHeader(204)
}