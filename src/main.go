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

	admissionv1 "k8s.io/api/admission/v1"
)

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

func mutatePod(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL)

	admissionReview, err := getAdmissionReviewFromRequest(r.Body)
	if err != nil {
		log.Printf(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	podPatch, err := getPodPatch(pod, mutationConfig)
	if err != nil {
		log.Printf("Request ID: %v - %v", admissionRequest.UID, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	podPatchBytes, err := json.Marshal(podPatch)
    if err != nil {
		newErr := fmt.Errorf("Could not marshal pod patch into bytes -- possible formatting error: %v", err.Error())
		log.Printf("Request ID: %v - %v", admissionRequest.UID, newErr.Error())
		http.Error(w, newErr.Error(), http.StatusInternalServerError)
	}

	// this needs to be copied cause admissionv1.PatchTypeJSONPatch is const
	// taking the direct reference of that doesn't match type
	patchType := admissionv1.PatchTypeJSONPatch
	admissionResponse := &admissionv1.AdmissionResponse{
		UID: admissionRequest.UID,
		Allowed: true,
		Patch: podPatchBytes,
		PatchType: &patchType,
	}

	admissionReview.Request = nil
	admissionReview.Response = admissionResponse

	admissionReviewResponseBytes, err := json.Marshal(&admissionReview)
    if err != nil {
		newErr := fmt.Errorf("Could not marshal admission review response into bytes -- possible formatting error: %v", err.Error())
		log.Printf("Request ID: %v - %v", admissionRequest.UID, newErr.Error())
		http.Error(w, newErr.Error(), http.StatusInternalServerError)
	}

    w.Header().Set("Content-Type", "application/json")
    w.Write(admissionReviewResponseBytes)
}

func mutateStatefulSet(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.URL)

	admissionReview, err := getAdmissionReviewFromRequest(r.Body)
	if err != nil {
		log.Printf(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	admissionRequest := admissionReview.Request
	log.Printf("Processing request : %v", admissionRequest.UID)

	statefulSet, err := getStatefulSetFromAdmissionRequest(admissionRequest)
	if err != nil {
		log.Printf("Request ID: %v - %v", admissionRequest.UID, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	_, err = getMutationConfig(statefulSet)
	if err != nil {
		log.Printf("Request ID: %v - %v", admissionRequest.UID, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	statefulSetPatch := getStatefulSetPatch(statefulSet)

	statefulSetPatchBytes, err := json.Marshal(statefulSetPatch)
    if err != nil {
		newErr := fmt.Errorf("Could not marshal pod patch into bytes -- possible formatting error: %v", err.Error())
		log.Printf("Request ID: %v - %v", admissionRequest.UID, newErr.Error())
		http.Error(w, newErr.Error(), http.StatusInternalServerError)
	}

	// this needs to be copied cause admissionv1.PatchTypeJSONPatch is const
	// taking the direct reference of that doesn't match type
	patchType := admissionv1.PatchTypeJSONPatch
	admissionResponse := &admissionv1.AdmissionResponse{
		UID: admissionRequest.UID,
		Allowed: true,
		Patch: statefulSetPatchBytes,
		PatchType: &patchType,
	}

	admissionReview.Request = nil
	admissionReview.Response = admissionResponse

	admissionReviewResponseBytes, err := json.Marshal(&admissionReview)
    if err != nil {
		newErr := fmt.Errorf("Could not marshal admission review response into bytes -- possible formatting error: %v", err.Error())
		log.Printf("Request ID: %v - %v", admissionRequest.UID, newErr.Error())
		http.Error(w, newErr.Error(), http.StatusInternalServerError)
	}

    w.Header().Set("Content-Type", "application/json")
    w.Write(admissionReviewResponseBytes)
}

func runServer(serverOptions *ServerOptions) {
	mux := http.NewServeMux()

	mux.HandleFunc("/status", handleStatus)
	mux.HandleFunc("POST /mutate-pods", mutatePod)
	mux.HandleFunc("POST /mutate-statefulsets", mutateStatefulSet)

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
