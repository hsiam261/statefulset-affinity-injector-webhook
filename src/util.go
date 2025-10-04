package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	admissionv1 "k8s.io/api/admission/v1"
)

func getAdmissionReviewFromRequest(reader io.Reader) (*admissionv1.AdmissionReview, error) {
	var admissionReview admissionv1.AdmissionReview
	if err := json.NewDecoder(reader).Decode(&admissionReview); err != nil {
		newErr := fmt.Errorf("Could not decode request: %v", err.Error())
		return nil, newErr
	}

	return &admissionReview, nil
}

func getPodFromAdmissionRequest(admissionRequest *admissionv1.AdmissionRequest) (*corev1.Pod, error) {
	if admissionRequest.Resource.Resource != "pods" {
		err := fmt.Errorf("Admission request object should be a pod, but instead we got a %s", admissionRequest.Resource.Resource)
		return nil, err
	}

	var pod corev1.Pod
	if err := json.Unmarshal(admissionRequest.Object.Raw, &pod); err != nil {
		newErr := fmt.Errorf("Failed to parse pod object from request: %v", err)
		return nil, newErr
	}

	return &pod, nil
}

func getStatefulSetFromAdmissionRequest(admissionRequest *admissionv1.AdmissionRequest) (*appsv1.StatefulSet, error) {
	if admissionRequest.Resource.Resource != "statefulsets" {
		err := fmt.Errorf("Admission request object should be a statefulset, but instead we got a %s", admissionRequest.Resource.Resource)
		return nil, err
	}

	var statefulset appsv1.StatefulSet
	if err := json.Unmarshal(admissionRequest.Object.Raw, &statefulset); err != nil {
		newErr := fmt.Errorf("Failed to parse statefulset object from request: %v", err)
		return nil, newErr
	}

	return &statefulset, nil
}

func getMutationConfig(object K8sObject) (map[string][]string, error) {
	kind := object.GetObjectKind().GroupVersionKind().Kind
	name := object.GetName()
	namespace := object.GetNamespace()
	annotations := object.GetAnnotations()

	if _, ok := annotations["statefulset-affinity-injector-webhook.hsiam261.github.io/enabled"]; !ok {
		err := fmt.Errorf("%s %s in namespace %s does not have \"statefulset-affinity-injector-webhook.hsiam261.github.io/enabled\" annotation set",kind, name, namespace)
		return nil, err
	}

	if mutationEnabled, _ := strconv.ParseBool(annotations["statefulset-affinity-injector-webhook.hsiam261.github.io/enabled"]); !mutationEnabled {
		err := fmt.Errorf("%s %s in namespace %s does not have \"statefulset-affinity-injector-webhook.hsiam261.github.io/enabled\" annotation set to true", kind, name, namespace)
		return nil, err
	}

	mutationConfigAnnotation, ok := annotations["statefulset-affinity-injector-webhook.hsiam261.github.io/config"]
	if !ok {
		err := fmt.Errorf("%s %s in namespace %s does not have \"statefulset-affinity-injector-webhook.hsiam261.github.io/config\" annotation", kind, name, namespace)
		return nil, err
	}

	var mutationConfig map[string][]string
	if err := json.Unmarshal([]byte(mutationConfigAnnotation), &mutationConfig); err != nil {
		newErr := fmt.Errorf("Error parsing \"statefulset-affinity-injector-webhook.hsiam261.github.io/config\" value for kind %s in namespace %s: %v", kind, name, namespace, err)
		return nil, newErr
	}

	return mutationConfig, nil
}

func getStatefulsetPodIndex(pod *corev1.Pod) (int, error) {
	parts := strings.Split(pod.Name, "-")
	lastPart := parts[len(parts) - 1]

	num, err := strconv.Atoi(lastPart)
	if err != nil {
		return 0, fmt.Errorf("Pod %s in namespace %s does not have an index in it's suffix", pod.Name, pod.Namespace)
	}

	return num, nil
}

func getPodPatch(pod *corev1.Pod, mutationConfig map[string][]string) ([]map[string]interface{}, error) {
	podIndex, err := getStatefulsetPodIndex(pod)
	if err != nil {
		return nil, err
	}

	patches := make([]map[string]interface{}, 0, 5)
	if pod.Spec.Affinity == nil {
		pod.Spec.Affinity = &corev1.Affinity{}
		patch := map[string]interface{}{
			"op": "add",
			"path": "/spec/affinity",
			"value": map[string]interface{}{},
		}
		patches = append(patches, patch)
	}

	if pod.Spec.Affinity.NodeAffinity == nil {
		pod.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
		patch := map[string]interface{}{
			"op": "add",
			"path": "/spec/affinity/nodeAffinity",
			"value": map[string]interface{}{},
		}
		patches = append(patches, patch)
	}

	if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
		patch := map[string]interface{}{
			"op": "add",
			"path": "/spec/affinity/nodeAffinity/requiredDuringSchedulingIgnoredDuringExecution",
			"value": map[string]interface{}{},
		}
		patches = append(patches, patch)
	}

	if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms == nil {
		patch := map[string]interface{}{
			"op": "add",
			"path": "/spec/affinity/nodeAffinity/requiredDuringSchedulingIgnoredDuringExecution/nodeSelectorTerms",
			"value": make([]corev1.NodeSelectorTerm, 0, 0),

		}

		patches = append(patches, patch)
	}

	expressions := make([]corev1.NodeSelectorRequirement, 0, len(mutationConfig))
	for key, vals := range mutationConfig {
		expressions = append(expressions, corev1.NodeSelectorRequirement{
			Key: key,
			Operator: "In",
			Values: []string{ vals[podIndex % len(vals)] },
		})
	}

	nodeSelectorTerm := corev1.NodeSelectorTerm{
		MatchExpressions: expressions,
	}

	patch := map[string]interface{}{
		"op": "add",
		"path": "/spec/affinity/nodeAffinity/requiredDuringSchedulingIgnoredDuringExecution/nodeSelectorTerms/-",
		"value": nodeSelectorTerm,
	}

	patches = append(patches, patch)

	return patches, nil
}

func getStatefulSetPatch(statefulSet *appsv1.StatefulSet) []map[string]interface{} {
	patches := make([]map[string]interface{}, 0, 5)

	//spec.template.metadata already exists since
	//spec.template.metadata.label is required
	if statefulSet.Spec.Template.Annotations == nil {
		patch := map[string]interface{}{
			"op": "add",
			"path": "/spec/template/metadata/annotations",
			"value": map[string]interface{}{},
		}
		patches = append(patches, patch)
	}

	//How to espace in jsonpatch
	//https://jsonpatch.com/#json-pointer
	patch := map[string]interface{}{
		"op": "add",
		"path": "/spec/template/metadata/annotations/statefulset-affinity-injector-webhook.hsiam261.github.io~1enabled",
		"value": "true",
	}

	patches = append(patches, patch)

	patch = map[string]interface{}{
		"op": "add",
		"path": "/spec/template/metadata/annotations/statefulset-affinity-injector-webhook.hsiam261.github.io~1config",
		"value": statefulSet.Annotations["statefulset-affinity-injector-webhook.hsiam261.github.io/config"],
	}

	patches = append(patches, patch)

	return patches
}
