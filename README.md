# Statefulset Affinity Injector
Statefulset-affinity-injector is a kubernetes admission webhook that allows setting separate node affinites to statefulset pods.

Normally, you can not set individual node affinities to stateful set pods. Instead every pod gets the node affinity mentioned in `spec.template.affinity.nodeAffinity` in the statefulset manifest. However, it is essential to set different node affinities for each pod in a stateful set to ensure high availability while not facing any scheduling issues to due PVs being attached in different zones. For more details, see [The Nightmare of Persistent Volumes with Multiple Availability Zones Kubernetes Cluster](https://medium.com/@calvineotieno010/the-nightmare-of-persistent-volumes-with-multiple-availability-zones-kubernetes-cluster-74cf2897a93e).

Statefulset-affinity-injector is a mutating webhook for kubernetes that solves this exact issue. The webhook listens to create and update operations for statefulsets and only create operations for statefulset pods with certain annotations and patches the node affinity for individual pods with different node affinities -- providing more configurability and allowing to run highly available stateful workloads without facing any issues with pod scheduling.

## How It Works
The `statefulset-affinity-injector` webhook works using two annotations:
- `statefulset-affinity-injector-webhook.hsiam261.github.io/enabled` : When this annotation is set to `true` on a statefulset, the webhook gets triggered when the statefulset gets created.
- `statefulset-affinity-injector-webhook.hsiam261.github.io/config`: This annotation is a json string that specifies how to add affinities to pods. The json config maps node labels to list of values. For each node label in the map, the ith pod of the stateful set has the additional constraint on of only being able to be scheduled on the ith value in the corresponding map. If i is larger than the length of the list, then it wraps around.

For example, consider the following annotation:
```
annotations:
    statefulset-affinity-injector-webhook.hsiam261.github.io/enabled : "true"
    statefulset-affinity-injector-webhook.hsiam261.github.io/config: |
    {
        "topology.kubernetes.io/zone": ["us-central1-a", "us-central1-b"],
        "node.kubernetes.io/instance-type": ["n2-standard-2", "n2-standard-4", "n2-standard-8"]
    }
```
In a stateful set with 5 replicas, we will get the following affinity:
- Pod 0 is scheduled with additional affinity for us-central1-a and n2-standard-2.
- Pod 1 is scheduled with additional affinity for us-central1-b and n2-standard-4.
- Pod 2 wraps around and uses us-central1-a and n2-standard-8.
- Pod 3 wraps again in the second list, and and uses us-central1-b and n2-standard-2.
- Finally, Pod 4 wraps again in the fist list, and and uses us-central1-a and n2-standard-4.

Please note that the webhook only injects new node affinities while keeping the old one's intact.

## How To Use
You can install this webhook using it's helm charts found in [dockerhub](https://hub.docker.com/r/hsiam261/statefulset-affinity-injector).

We provide the configurable parameters for the Helm chart and their default values in the following sections:

### Webhook Configuration
| Parameter | Description | Default | Required |
|------------|-------------|----------|-----------|
| `webhook.namespaceSelector` | Label selector to specify namespaces the webhook applies to. Only object creation in those namespace can trigger the webhook. | `{}` | No |
| `webhook.objectSelector` | Label selector to specify k8s objects the webhook applies to. Only objects that match the selector can trigger the webhook. | `{}` | No |
| `webhook.timeoutSeconds` | Timeout in seconds for the webhook to respond. | `10` | No |

---

### TLS Configuration
TLS certificate and key is used by the API server to authenticate requests to the webhook.

| Parameter | Description | Default | Required |
|------------|-------------|----------|-----------|
| `tls.cert` | Base64-encoded TLS certificate used by the webhook server. | `""` | Yes |
| `tls.key` | Base64-encoded TLS private key used by the webhook server. | `""` | Yes |

---

### Autoscaling Configuration
These values are used to enable and configure autoscaling on the webhook pods.

| Parameter | Description | Default | Required |
|------------|-------------|----------|-----------|
| `autoscaling.enabled` | Enable Horizontal Pod Autoscaler (HPA). | `false` | No |
| `autoscaling.minReplicas` | Minimum number of replicas when autoscaling is enabled. | `1` | No |
| `autoscaling.maxReplicas` | Maximum number of replicas when autoscaling is enabled. | `5` | No |
| `autoscaling.targetCPUUtilizationPercentage` | Target average CPU utilization for scaling. | `80` | No |
| `autoscaling.targetMemoryUtilizationPercentage` | Target average memory utilization for scaling. | `nil` | No |
---

### Deployment Configuration
These parameters are used configure the webhook deployment

| Parameter | Description | Default | Required |
|------------|-------------|----------|-----------|
| `replicaCount` | Number of pod replicas to deploy. | `1` | Required if auto scaling is not enabled |
| `podAnnotations` | Annotations to add to the pod metadata. | `{}` | No |
| `podLabels` | Additional labels to add to the pod metadata. | `{}` | No |
| `affinity` | Node/pod affinity rules. | `{}` | No |
| `tolerations` | List of tolerations for scheduling pods on tainted nodes. | `[]` | No |

---

### Name Overrides
The `nameOverride` and `fullnameOverride` parameters can be used to override the name and fullname of our resources. This is useful if you are considering having multiple releases of this helm chart in the same cluster.

### Image Configuration
These parameters related to the webhook docker image.

| Parameter | Description | Default | Required |
|------------|-------------|----------|-----------|
| `imagePullSecrets` | List of secret names for pulling private images. | `nil` | No |
| `image.repository` | Container image repository. | `hsiam261/statefulset-affinity-injector-webhook` | No |
| `image.tag` | Image tag to use. | `"latest"` | No |
| `image.pullPolicy` | Image pull policy (`Always`, `IfNotPresent`, `Never`). | `"IfNotPresent"` | No |

### AWS security group policy configuration
This parameter is used to attach pod security groups to the webhook pods

| Parameter | Description | Default | Required |
|------------|-------------|----------|-----------|
| `awsSecurityGroups` | List of security group ids. | `[]` | No |
---

### Example Values File
This is an example values file.
```yaml
image:
  pullPolicy: Always

webhook:
  namespaceSelector:
    matchExpressions:
      - key: "name" # This is a namespace label.
                    # Label namespaces and restrict
                    # webhook to a fixed namespaces.
                    # This is safer, cause otherwise
                    # you may face unwanted issues in
                    # important services like those
                    # running in kube-system
        operator: In
        values:
          - "namespace-1"
```

This file has the tls configurations missing however, with the certificate and key files you can set it as follows:

```bash
helm install RELEASE_NAME oci://registry-1.docker.io/hsiam261/statefulset-affinity-injector \
    --namespace WEBHOOK_NAMESPACE --create-namespace --kube-context CONTEXT \
    --values ./values-from-dockerhub.yaml \
    --set-file tls.cert=./tls.crt \
    --set-file tls.key=./tls.key
```

## Generating TLS Certificates
Mutating webhooks require TLS certificates to securely authenticate communication between the Kubernetes API server and the webhooks. Properly configured certificates prevent man-in-the-middle attacks and ensure that only trusted webhooks can receive sensitive API server requests.

When generating certificates for a mutating webhook, the certificate must authenticate the following **DNS names**:
1. **Webhook service name** – This will be the name of the service generated by the helm chart
2. **Webhook service namespace** – These will be of the format `${SERVICE_NAME}.${NAMESPACE}`, `${SERVICE}.${NAMESPACE}`, `${SERVICE}.${NAMESPACE}.svc` and `${SERVICE}.${NAMESPACE}.svc.cluster.local`.

Thus the certificate’s **Common Name (CN)** or **Subject Alternative Name (SAN)** must include all DNS names that the API server will use to contact the webhook.

To generate the certificate names, we provide a shell script in `generate-certificate/certificate-gen.sh`. You may generate the certificates by either explicitly passing the namespace and the service name or passing the namespace and the release name. In either case, it will generate valid certificate which you may use.

Example usage:
```bash
cd generate-certificate
./certificate-gen.sh --namespace NAMESPACE --release RELEASE
```

It will generate two files: `tls.crt` and `tls.key`.

Those values need to placed either in the values file as base64 encoded strings or passed from the files directly.

You can generate the base64 encoded strings in the following manner:
```bash
cat tls.crt | base64 | tr -d '\n'
cat tls.key | base64 | tr -d '\n'
```

To pass the values from the files directly, you can use the following commands:
```bash
helm install RELEASE_NAME oci://registry-1.docker.io/hsiam261/statefulset-affinity-injector \
    --namespace WEBHOOK_NAMESPACE --create-namespace --kube-context CONTEXT \
    --values ./values-from-dockerhub.yaml \
    --set-file tls.cert=./tls.crt \
    --set-file tls.key=./tls.key
```

**PLEASE NOTE THAT THE VALIDITY OF THE GENERATED CERTIFICATE IS ONLY ONE YEAR AND WOULD REQUIRE MANUAL ROTATION**

## About Security Groups on EKS
For the webhook to function, our webhook pods need to pass the liveness and readiness probes
and be able to be triggered by the API server. This is why we recommend the following inbound rules to the pod security groups:
- allow cluster security group on port 8443 for api server communication
- allow node security group on port 8443 for liveness and readiness probes
- allow all traffic from self

For the outbound rules, we can allow any traffic to anywhere.
