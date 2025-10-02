package main

import (
	"log"
	"flag"
	"encoding/json"
	"net/http"

	// corev1 "k8s.io/api/core/v1"
	// admissionv1 "k8s.io/api/admission/v1"
)

/*
	Checks if pod is owned by a statefulset that has
	registered for this webhook. If so, then it fetches
	the stateful set annotation and from the pod name
	decides what affinity should be injected to the pod
*/
// func handleWebhook(w http.ResponseWriter, r *http.Request) {
// 	var admissionReview admissionV1.AdmissionReview

// 	if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
// 		log.Printf("Could not decode request: %v", err)
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	admissionRequest := admissionReview.Request
// 	admissionResponse := admissionv1.AdmissionResponse{}

// 	if admissionRequest.Resource != "pods" {
// 		log.Printf("Admission request object should be a pod, but instead we got a %s", admissionRequest.Resource)
// 		//NEED TO RETURN SOMETHING HERE
// 	}

// 	var pod corev1.Pod
// 	if err := json.Unmarshal(admissionRequest.Object.Raw, &pod); err != nil {
// 		log.Printf("Failed to parse pod object from request: %v", err)
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	statefulsetReferenceCount := 0
// 	for _ , owner := range pod.OwnerReferences {
// 		if owner.Kind == "StatefulSet" {
// 			statefulsetReferenceCount = statefulsetReferenceCount + 1
// 		}
// 	}

// 	if statefulsetReferenceCount == 0 {
// 		log.Printf("Pod %s in namespace %s is not owned by a statefulset", pod.Name, pod.Namespace)
// 		err := fmt.Errorf("Pod %s in namespace %s is owned by multiple statefulsets", pod.Name, pod.Namespace)
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	if statefulsetReferenceCount > 1 {
// 		log.Printf("Pod %s in namespace %s is owned by multiple statefulsets", pod.Name, pod.Namespace)
// 		err := fmt.Errorf("Pod %s in namespace %s is owned by multiple statefulsets", pod.Name, pod.Namespace)
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	annotations := pod.Annotations
// 	affinity := pod.Spec.Affinity


// 	for annotation, val := range annotations {
// 		if annotation == "statefulset-node-affinity-injector.hsiam261.github.io/config" {
// 			var affinityMap map[string]string
// 			if err := json.Unmarshal([]byte(val), &affinityMap); err != nil {
// 				log.Printf("Malformed annotation value in pod %s in namespace %s: %s", pod.Name, pod.Namespace, val)
// 				http.Error(w, err.Error(), http.StatusBadRequest)
// 				return
// 			}



// 		}
// 	}
// }

type ServerOptions struct {
	EnableTLS bool
	CertFile string
    KeyFile string
	GracefulShutdownSeconds int
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	respBytes, _ := json.Marshal(map[string]interface{}{"status": "ok"})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

func main() {
	serverOptions := ServerOptions{}
	flag.BoolVar(&serverOptions.EnableTLS, "enalbe-tls", false, "whether or not to enable TLS")
	flag.StringVar(&serverOptions.CertFile, "cert-file", "./secrets/certs/tls.crt", "filepath to .crt file, ignored if tls is not enabled")
	flag.StringVar(&serverOptions.KeyFile, "key-file", "./secrets/certs/tls.key", "filepath to .key file, ignored if tls is not enabled")
	flag.IntVar(&serverOptions.GracefulShutdownSeconds, "graceful-shutdown-seconds", 0, "number of seconds to wait before graceful shutdown")

	flag.Parse()

	mux := http.NewServeMux()

	// mux.HandleFunc("POST /webhook")
	mux.HandleFunc("/status", handleStatus)

	server := http.Server{
		Addr: "0.0.0.0:8080",
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Error starting the server: %v", err)
	}
}
