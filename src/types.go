package main

import (
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type K8sObject interface {
    metav1.Object
    runtime.Object
}
