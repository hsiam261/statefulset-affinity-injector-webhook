package main

import (
	"fmt"
	"log"
	"flag"
	"time"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	corev1 "k8s.io/api/core/v1"
	admissionv1 "k8s.io/api/admission/v1"
)

/*
	Checks if pod is owned by a statefulset that has
	registered for this webhook. If so, then it fetches
	the stateful set annotation and from the pod name
	decides what affinity should be injected to the pod
*/
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	var admissionReview admissionV1.AdmissionReview

	if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
		log.Printf("Could not decode request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	admissionRequest := admissionReview.Request
	log.Printf("Request ID: %v", admissionReview.UID)

	if admissionRequest.Resource != "pods" {
		log.Printf("Admission request object should be a pod, but instead we got a %s", admissionRequest.Resource)
		//NEED TO RETURN SOMETHING HERE
	}

	http.Error(w, fmt.Sprintf("Admission request object should be a pod, but instead we got a %s", admissionRequest.Resource), http.StatusBadRequest)



	// var pod corev1.Pod
	// if err := json.Unmarshal(admissionRequest.Object.Raw, &pod); err != nil {
	// 	log.Printf("Failed to parse pod object from request: %v", err)
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	// statefulsetReferenceCount := 0
	// for _ , owner := range pod.OwnerReferences {
	// 	if owner.Kind == "StatefulSet" {
	// 		statefulsetReferenceCount = statefulsetReferenceCount + 1
	// 	}
	// }

	// if statefulsetReferenceCount == 0 {
	// 	log.Printf("Pod %s in namespace %s is not owned by a statefulset", pod.Name, pod.Namespace)
	// 	err := fmt.Errorf("Pod %s in namespace %s is owned by multiple statefulsets", pod.Name, pod.Namespace)
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }

	// if statefulsetReferenceCount > 1 {
	// 	log.Printf("Pod %s in namespace %s is owned by multiple statefulsets", pod.Name, pod.Namespace)
	// 	err := fmt.Errorf("Pod %s in namespace %s is owned by multiple statefulsets", pod.Name, pod.Namespace)
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }

	// annotations := pod.Annotations
	// affinity := pod.Spec.Affinity


	// for annotation, val := range annotations {
	// 	if annotation == "statefulset-node-affinity-injector.hsiam261.github.io/config" {
	// 		var affinityMap map[string]string
	// 		if err := json.Unmarshal([]byte(val), &affinityMap); err != nil {
	// 			log.Printf("Malformed annotation value in pod %s in namespace %s: %s", pod.Name, pod.Namespace, val)
	// 			http.Error(w, err.Error(), http.StatusBadRequest)
	// 			return
	// 		}



	// 	}
	// }
}

type ServerOptions struct {
	EnableTLS bool
	CertFile string
    KeyFile string
	GracefulShutdownSeconds int
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL)
	respBytes, _ := json.Marshal(map[string]interface{}{"status": "ok"})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

func runServer(serverOptions *ServerOptions) {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /webhook", handleWebhook)
	mux.HandleFunc("/status", handleStatus)

	port := 8080
	protocol := "http"
	if serverOptions.EnableTLS {
		port = 8443
		protocol = "https"
	}

	serverAddress := fmt.Sprintf("0.0.0.0:%d", port)

	server := http.Server{
		Addr: serverAddress,
		Handler: mux,
	}

	// Channel to listen for errors from server
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Server running on %s://%s",protocol, serverAddress)
		if serverOptions.EnableTLS {
			serverErrors <- server.ListenAndServeTLS(serverOptions.CertFile, serverOptions.KeyFile)
		} else {
			serverErrors <- server.ListenAndServe()
		}
	}()



	// Set up channel to listen for interrupt/terminate signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Println("Server error:", err)

	case sig := <-stop:
		log.Println("Received signal:", sig)

		// Graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(serverOptions.GracefulShutdownSeconds) * time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Println("Graceful shutdown failed:", err)
		} else {
			log.Println("Server gracefully stopped")
		}
	}
}

func main() {
	serverOptions := ServerOptions{}
	flag.BoolVar(&serverOptions.EnableTLS, "enable-tls", false, "whether or not to enable TLS")
	flag.StringVar(&serverOptions.CertFile, "cert-file", "./secrets/certs/tls.crt", "filepath to .crt file, ignored if tls is not enabled")
	flag.StringVar(&serverOptions.KeyFile, "key-file", "./secrets/certs/tls.key", "filepath to .key file, ignored if tls is not enabled")
	flag.IntVar(&serverOptions.GracefulShutdownSeconds, "graceful-shutdown-seconds", 5, "number of seconds to wait before graceful shutdown")

	flag.Parse()

	runServer(&serverOptions)
}
