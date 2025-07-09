package main

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"

	"golang.org/x/crypto/blake2b"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TemplateSetSpec defines the desired state of TemplateSet
type TemplateSetSpec struct {
	Selector      *metav1.LabelSelector  `json:"selector"`
	Template      corev1.PodTemplateSpec `json:"template"`
	ServicePerPod *ServicePerPod         `json:"servicePerPod,omitempty"`
}

// ServicePerPod defines service per pod configuration
type ServicePerPod struct {
	Enabled bool `json:"enabled"`
}

// SyncRequest represents the webhook request from metacontroller
type SyncRequest struct {
	Controller map[string]interface{} `json:"controller"`
	Parent map[string]interface{} `json:"parent"`
	Children map[string]interface{} `json:"children"`
	Related map[string]interface{} `json:"related"`
}

// SyncResponse represents the webhook response to metacontroller
type SyncResponse struct {
	Status   map[string]interface{} `json:"status,omitempty"`
	Children []interface{}          `json:"children,omitempty"`
}

// CustomizeRequest represents the customize webhook request from metacontroller
type CustomizeRequest struct {
	Controller  map[string]interface{} `json:"controller"`
	Parent map[string]interface{} `json:"parent"`
}

// CustomizeResponse represents the customize webhook response to metacontroller
type CustomizeResponse struct {
	RelatedResources []RelatedResource `json:"relatedResources,omitempty"`
}

// RelatedResource defines a resource to watch
type RelatedResource struct {
	APIVersion    string            `json:"apiVersion"`
	Resource      string            `json:"resource"`
	LabelSelector map[string]string `json:"labelSelector,omitempty"`
}

func main() {
	http.HandleFunc("/sync", syncHandler)
	http.HandleFunc("/customize", customizeHandler)
	http.HandleFunc("/health", healthHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func customizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("CUSTOMIZE")

	var req CustomizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := customize(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Customize failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func customize(req CustomizeRequest) (*CustomizeResponse, error) {
	// Tell metacontroller to watch all nodes
	return &CustomizeResponse{
		RelatedResources: []RelatedResource{
			{
				APIVersion: "v1",
				Resource:   "nodes",
			},
		},
	}, nil
}

func syncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	b, _ := io.ReadAll(r.Body)

	var req SyncRequest
	if err := json.Unmarshal(b, &req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := sync(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Sync failed: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("RESPONSE: %v", resp)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func sync(req SyncRequest) (*SyncResponse, error) {
	// Extract TemplateSet spec from the request
	specData, ok := req.Parent["spec"]
	if !ok {
		return nil, fmt.Errorf("spec not found in object")
	}

	specBytes, err := json.Marshal(specData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal spec: %v", err)
	}

	var spec TemplateSetSpec
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal spec: %v", err)
	}

	// Extract TemplateSet name from metadata
	metadata, ok := req.Parent["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("metadata not found in object")
	}

	templateSetName, ok := metadata["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name not found in metadata")
	}

	// Extract nodes from related resources (provided by customize hook)
	nodes, err := extractNodesFromRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to extract nodes: %v", err)
	}

	log.Printf("NODES: %v", nodes)

	children := []interface{}{}

	// Create pods for each node
	for _, node := range nodes {
		pod, err := createPodForNode(spec, node, templateSetName)
		if err != nil {
			return nil, fmt.Errorf("failed to create pod for node %s: %v", node.Name, err)
		}
		children = append(children, pod)

		// Create service per pod if enabled
		if spec.ServicePerPod != nil && spec.ServicePerPod.Enabled {
			service, err := createServiceForPod(spec, node, pod, templateSetName)
			if err != nil {
				return nil, fmt.Errorf("failed to create service for pod on node %s: %v", node.Name, err)
			}
			children = append(children, service)
		}
	}

	return &SyncResponse{
		Children: children,
		Status: map[string]interface{}{
			"readyReplicas": len(nodes),
		},
	}, nil
}

func extractNodesFromRequest(req SyncRequest) ([]corev1.Node, error) {
	log.Printf("REQUEST RELATED: %v", req.Related)
	// Extract nodes from the Related field (related resources from customize hook)
	nodesData, ok := req.Related["Node.v1"]
	if !ok {
		// If no nodes found in related resources, return empty slice
		log.Printf("No nodes found in related resources, this might be expected during initial sync")
		return []corev1.Node{}, nil
	}

	log.Printf("EXTRACTED NODES: %v", nodesData)

	nodesMap, ok := nodesData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("nodes data is not a map")
	}

	var nodes []corev1.Node
	for _, nodeData := range nodesMap {
		nodeObj, ok := nodeData.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract node metadata
		metadata, ok := nodeObj["metadata"].(map[string]interface{})
		if !ok {
			continue
		}

		nodeName, ok := metadata["name"].(string)
		if !ok {
			continue
		}

		// Extract node labels
		labels := make(map[string]string)
		if labelsData, ok := metadata["labels"].(map[string]interface{}); ok {
			for k, v := range labelsData {
				if strVal, ok := v.(string); ok {
					labels[k] = strVal
				}
			}
		}

		node := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   nodeName,
				Labels: labels,
			},
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func createPodForNode(spec TemplateSetSpec, node corev1.Node, templateSetName string) (map[string]interface{}, error) {
	// Apply templating to pod name
	podName, err := applyTemplate(spec.Template.ObjectMeta.Name, node)
	if err != nil || podName == "" {
		podName = "pod"
	}
	podName = podName + "-" + generateSuffix(node.Name, templateSetName)

	// Apply templating to labels
	labels := make(map[string]string)
	for k, v := range spec.Template.ObjectMeta.Labels {
		l, err := applyTemplate(v, node)
		if err != nil {
			l = ""
		}
		labels[k] = l
	}
	labels["templateset.keen.land/podname"] = podName

	// Apply templating to environment variables
	containers := make([]map[string]interface{}, len(spec.Template.Spec.Containers))
	for i, container := range spec.Template.Spec.Containers {
		env := make([]map[string]interface{}, len(container.Env))
		for j, envVar := range container.Env {
			val, err := applyTemplate(envVar.Value, node)
			if err != nil {
				val = ""
			}
			env[j] = map[string]interface{}{
				"name":  envVar.Name,
				"value": val,
			}
		}

		containers[i] = map[string]interface{}{
			"name":  container.Name,
			"image": container.Image,
			"env":   env,
		}
	}

	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":   podName,
			"labels": labels,
		},
		"spec": map[string]interface{}{
			"containers": containers,
			"nodeName":   node.ObjectMeta.Name,
		},
	}

	log.Printf("POD: %v", pod)

	return pod, nil
}

func createServiceForPod(spec TemplateSetSpec, node corev1.Node, pod map[string]interface{}, templateSetName string) (map[string]interface{}, error) {
	podMetadata := pod["metadata"].(map[string]interface{})
	podName := podMetadata["name"].(string)

	service := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name": podName + "-service",
		},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"templateset.keen.land/podname": podName,
			},
			"ports": []map[string]interface{}{
				{
					"port":       80,
					"targetPort": 8080,
				},
			},
		},
	}

	return service, nil
}

type templateInfo struct {
	Node corev1.Node
}

func applyTemplate(inputTemplate string, node corev1.Node) (string, error) {
	buf := new(bytes.Buffer)
	tmpl, err := template.New("").Parse(inputTemplate)
	if err != nil {
		return "", err
	}
	info := templateInfo{node}
	err = tmpl.Execute(buf, info)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func generateSuffix(nodeName, templateSetName string) string {
	// Create input for hashing by combining nodeName and templateSetName
	input := nodeName + ":" + templateSetName

	// Generate BLAKE2 hash
	hash, err := blake2b.New256(nil)
	if err != nil {
		// Fallback in case of error
		return "fallback"
	}

	hash.Write([]byte(input))
	hashBytes := hash.Sum(nil)

	// Take first 5 bytes of hash and encode to base32 for readability
	// This gives us a reasonably short but unique suffix
	suffix := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hashBytes))[:5]

	return suffix
}
