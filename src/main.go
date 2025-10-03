package main

import (
	"fmt"
	"strconv"
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

func getPodFromAdmissionRequest(admissionRequest *admissionv1.AdmissionRequest) (*corev1.Pod, error) {
	if admissionRequest.Resource.Resource != "pods" {
		err := fmt.Errorf("Admission request object should be a pod, but instead we got a %s", admissionRequest.Resource)
		return nil, err
	}

	var pod corev1.Pod
	if err := json.Unmarshal(admissionRequest.Object.Raw, &pod); err != nil {
		newErr := fmt.Errorf("Failed to parse pod object from request: %v", err)
		return nil, newErr
	}

	return &pod, nil
}

func getMutationConfig(pod *corev1.Pod) (map[string][]string, error) {
	if _, ok := pod.Annotations["statefulset-affinity-injector-webhook.hsiam261.github.io/enabled"]; !ok {
		err := fmt.Errorf("Pod %s in namespace %s does not have \"statefulset-affinity-injector-webhook.hsiam261.github.io/enabled\" annotation set", pod.Name, pod.Namespace)
		return nil, err
	}

	if mutationEnabled, _ := strconv.ParseBool(pod.Annotations["statefulset-affinity-injector-webhook.hsiam261.github.io/enabled"]); !mutationEnabled {
		err := fmt.Errorf("Pod %s in namespace %s does not have \"statefulset-affinity-injector-webhook.hsiam261.github.io/enabled\" annotation set to true", pod.Name, pod.Namespace)
		return nil, err
	}

	mutationConfigAnnotation, ok := pod.Annotations["statefulset-affinity-injector-webhook.hsiam261.github.io/config"]
	if !ok {
		err := fmt.Errorf("Pod %s in namespace %s does not have \"statefulset-affinity-injector-webhook.hsiam261.github.io/config\" annotation", pod.Name, pod.Namespace)
		return nil, err
	}

	var mutationConfig map[string][]string
	if err := json.Unmarshal([]byte(mutationConfigAnnotation), &mutationConfig); err != nil {
		newErr := fmt.Errorf("Error parsing \"statefulset-affinity-injector-webhook.hsiam261.github.io/config\" value for pod %s in namespace %s: %v",pod.Name, pod.Namespace, err)
		return nil, newErr
	}

	return mutationConfig, nil
}

/*
	Checks if pod is owned by a statefulset that has
	registered for this webhook. If so, then it fetches
	the stateful set annotation and from the pod name
	decides what affinity should be injected to the pod
*/
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL)

	var admissionReview admissionv1.AdmissionReview
	if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
		log.Printf("Could not decode request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	admissionRequest := admissionReview.Request
	log.Printf("Processing request : %v", admissionRequest.UID)


	pod, err := getPodFromAdmissionRequest(admissionRequest)
	if err != nil {
		log.Printf("Request ID: %v - %v", admissionRequest.UID, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	mutationConfig, err := getMutationConfig(pod)
	if err != nil {
		log.Printf("Request ID: %v - %v", admissionRequest.UID, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	log.Println(mutationConfig)

	http.Error(w, "Not Implemented Yet", http.StatusNotImplemented)
}

type ServerOptions struct {
	EnableTLS bool
	CertFile string
    KeyFile string
	GracefulShutdownSeconds int
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	// log.Println(r.Method, r.URL)
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
