package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHealthHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := "OK"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestSyncHandler(t *testing.T) {
	reqBody := SyncRequest{
		Parent: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-templateset",
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]string{
						"app": "test",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-pod",
						"labels": map[string]string{
							"app": "test",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test",
								"image": "nginx",
								"env": []interface{}{
									map[string]interface{}{
										"name":  "TEST_VAR",
										"value": "test-value",
									},
								},
							},
						},
					},
				},
				"servicePerPod": map[string]interface{}{
					"enabled": true,
				},
			},
		},
		Related: map[string]interface{}{
			"Node.v1": map[string]interface{}{
				"test-node": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-node",
						"labels": map[string]interface{}{
							"topology.kubernetes.io/zone": "us-east1-a",
							"kubernetes.io/hostname":      "test-node",
						},
					},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/sync", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(syncHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp SyncResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}

	if len(resp.Children) == 0 {
		t.Error("expected children in response, got none")
	}
}

func TestApplyTemplate(t *testing.T) {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				"topology.kubernetes.io/zone": "us-east1-a",
				"kubernetes.io/hostname":      "test-node",
			},
		},
	}

	tests := []struct {
		template string
		expected string
	}{
		{
			template: "{{index .Node.ObjectMeta.Labels \"topology.kubernetes.io/zone\"}}",
			expected: "us-east1-a",
		},
		{
			template: "{{index .Node.Labels \"kubernetes.io/hostname\"}}",
			expected: "test-node",
		},
		{
			template: "plain-text",
			expected: "plain-text",
		},
		{
			template: "{{index .Node.Labels \"nonexistent\"}}",
			expected: "",
		},
	}

	for _, tt := range tests {
		result, err := applyTemplate(tt.template, node)
		if err != nil {
			t.Errorf("applyTemplate(%q) err: %v", tt.template, err)
		}
		if result != tt.expected {
			t.Errorf("applyTemplate(%q) = %q, want %q", tt.template, result, tt.expected)
		}
	}
}

func TestCreatePodForNode(t *testing.T) {
	spec := TemplateSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test",
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-{{index .Node.Labels \"topology.kubernetes.io/zone\"}}",
				Labels: map[string]string{
					"app":  "test",
					"zone": "{{index .Node.Labels \"topology.kubernetes.io/zone\"}}",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test",
						Image: "nginx",
						Env: []corev1.EnvVar{
							{
								Name:  "ZONE",
								Value: "{{index .Node.Labels \"topology.kubernetes.io/zone\"}}",
							},
						},
					},
				},
			},
		},
	}

	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				"topology.kubernetes.io/zone": "us-east1-a",
			},
		},
	}

	pod, err := createPodForNode(spec, node, "test-templateset")
	if err != nil {
		t.Fatalf("createPodForNode failed: %v", err)
	}

	// Check basic pod structure
	if pod["apiVersion"] != "v1" {
		t.Error("expected apiVersion to be v1")
	}
	if pod["kind"] != "Pod" {
		t.Error("expected kind to be Pod")
	}

	// Check metadata
	metadata := pod["metadata"].(map[string]interface{})
	labels := metadata["labels"].(map[string]string)

	if labels["app"] != "test" {
		t.Error("expected app label to be test")
	}
	if labels["zone"] != "us-east1-a" {
		t.Error("expected zone label to be us-east1-a")
	}
	if _, ok := labels["templateset.keen.land/podname"]; !ok {
		t.Error("expected templateset.keen.land/podname label to be present")
	}

	// Check spec
	podSpec := pod["spec"].(map[string]interface{})
	if podSpec["nodeName"] != "test-node" {
		t.Error("expected nodeName to be test-node")
	}

	containers := podSpec["containers"].([]map[string]interface{})
	if len(containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(containers))
	}

	container := containers[0]
	if container["name"] != "test" {
		t.Error("expected container name to be test")
	}
	if container["image"] != "nginx" {
		t.Error("expected container image to be nginx")
	}

	env := container["env"].([]map[string]interface{})
	if len(env) != 1 {
		t.Errorf("expected 1 env var, got %d", len(env))
	}

	envVar := env[0]
	if envVar["name"] != "ZONE" {
		t.Error("expected env var name to be ZONE")
	}
	if envVar["value"] != "us-east1-a" {
		t.Error("expected env var value to be us-east1-a")
	}
}

func TestCreateServiceForPod(t *testing.T) {
	spec := TemplateSetSpec{}
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	}
	pod := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "test-pod-abc123",
		},
	}

	service, err := createServiceForPod(spec, node, pod, "test-templateset")
	if err != nil {
		t.Fatalf("createServiceForPod failed: %v", err)
	}

	// Check basic service structure
	if service["apiVersion"] != "v1" {
		t.Error("expected apiVersion to be v1")
	}
	if service["kind"] != "Service" {
		t.Error("expected kind to be Service")
	}

	// Check metadata
	metadata := service["metadata"].(map[string]interface{})
	if metadata["name"] != "test-pod-abc123-service" {
		t.Error("expected service name to be test-pod-abc123-service")
	}

	// Check spec
	serviceSpec := service["spec"].(map[string]interface{})
	selector := serviceSpec["selector"].(map[string]interface{})
	if selector["templateset.keen.land/podname"] != "test-pod-abc123" {
		t.Error("expected selector to match pod name")
	}

	ports := serviceSpec["ports"].([]map[string]interface{})
	if len(ports) != 1 {
		t.Errorf("expected 1 port, got %d", len(ports))
	}

	port := ports[0]
	if port["port"] != 80 {
		t.Error("expected port to be 80")
	}
	if port["targetPort"] != 8080 {
		t.Error("expected targetPort to be 8080")
	}
}

func TestSync(t *testing.T) {
	req := SyncRequest{
		Parent: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-templateset",
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]string{
						"app": "test",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-pod",
						"labels": map[string]string{
							"app": "test",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test",
								"image": "nginx",
								"env":   []interface{}{},
							},
						},
					},
				},
			},
		},
		Related: map[string]interface{}{
			"Node.v1": map[string]interface{}{
				"test-node": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-node",
						"labels": map[string]interface{}{
							"topology.kubernetes.io/zone": "us-east1-a",
							"kubernetes.io/hostname":      "test-node",
						},
					},
				},
			},
		},
	}

	resp, err := sync(req)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Should have at least one child (the pod)
	if len(resp.Children) == 0 {
		t.Error("expected at least one child, got none")
	}

	// Check status
	if resp.Status["readyReplicas"] != 1 {
		t.Errorf("expected readyReplicas to be 1, got %v", resp.Status["readyReplicas"])
	}
}

func TestCustomizeHandler(t *testing.T) {
	reqBody := CustomizeRequest{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-templateset",
			},
		},
		Related: map[string]interface{}{},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/customize", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(customizeHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp CustomizeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}

	if len(resp.RelatedResources) == 0 {
		t.Error("expected related resources in response, got none")
	}

	// Check that we're watching nodes
	found := false
	for _, resource := range resp.RelatedResources {
		if resource.APIVersion == "v1" && resource.Resource == "nodes" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find nodes in related resources")
	}
}

func TestCustomize(t *testing.T) {
	req := CustomizeRequest{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-templateset",
			},
		},
		Related: map[string]interface{}{},
	}

	resp, err := customize(req)
	if err != nil {
		t.Fatalf("customize failed: %v", err)
	}

	if len(resp.RelatedResources) == 0 {
		t.Error("expected related resources, got none")
	}

	// Should watch nodes
	if resp.RelatedResources[0].APIVersion != "v1" {
		t.Error("expected apiVersion to be v1")
	}
	if resp.RelatedResources[0].Resource != "nodes" {
		t.Error("expected resource to be nodes")
	}
}

func TestGenerateSuffix(t *testing.T) {
	tests := []struct {
		nodeName        string
		templateSetName string
		expectedSame    bool
	}{
		// Same inputs should produce the same suffix
		{
			nodeName:        "node-1",
			templateSetName: "templateset-1",
			expectedSame:    true,
		},
		// Different node names should produce different suffixes
		{
			nodeName:        "node-2",
			templateSetName: "templateset-1",
			expectedSame:    false,
		},
		// Different templateset names should produce different suffixes
		{
			nodeName:        "node-1",
			templateSetName: "templateset-2",
			expectedSame:    false,
		},
	}

	// Reference suffix for comparison
	referenceSuffix := generateSuffix("node-1", "templateset-1")

	for _, tt := range tests {
		t.Run(fmt.Sprintf("node=%s,templateset=%s", tt.nodeName, tt.templateSetName), func(t *testing.T) {
			suffix := generateSuffix(tt.nodeName, tt.templateSetName)

			// Check that the result is not empty
			if suffix == "" {
				t.Error("generateSuffix returned empty string")
			}

			// Check that the suffix is lowercase
			if suffix != strings.ToLower(suffix) {
				t.Errorf("suffix contains uppercase letters: %s", suffix)
			}

			// Verify deterministic behavior (same inputs = same outputs)
			for i := 0; i < 5; i++ {
				repeatSuffix := generateSuffix(tt.nodeName, tt.templateSetName)
				if suffix != repeatSuffix {
					t.Errorf("generateSuffix not deterministic: got %s and %s for same inputs", suffix, repeatSuffix)
				}
			}

			// Check whether it should match the reference
			if tt.expectedSame {
				if suffix != referenceSuffix {
					t.Errorf("expected suffix to match reference %s, got %s", referenceSuffix, suffix)
				}
			} else {
				if suffix == referenceSuffix {
					t.Errorf("expected suffix to be different from reference %s, got %s", referenceSuffix, suffix)
				}
			}
		})
	}
}

func TestExtractNodesFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		req      SyncRequest
		expected []corev1.Node
		wantErr  bool
	}{
		{
			name: "single node",
			req: SyncRequest{
				Related: map[string]interface{}{
					"Node.v1": map[string]interface{}{
						"test-node": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "test-node",
								"labels": map[string]interface{}{
									"topology.kubernetes.io/zone": "us-east1-a",
									"kubernetes.io/hostname":      "test-node",
								},
							},
						},
					},
				},
			},
			expected: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
						Labels: map[string]string{
							"topology.kubernetes.io/zone": "us-east1-a",
							"kubernetes.io/hostname":      "test-node",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple nodes",
			req: SyncRequest{
				Related: map[string]interface{}{
					"Node.v1": map[string]interface{}{
						"node-1": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "node-1",
								"labels": map[string]interface{}{
									"topology.kubernetes.io/zone": "us-east1-a",
								},
							},
						},
						"node-2": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "node-2",
								"labels": map[string]interface{}{
									"topology.kubernetes.io/zone": "us-east1-b",
								},
							},
						},
					},
				},
			},
			expected: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							"topology.kubernetes.io/zone": "us-east1-a",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							"topology.kubernetes.io/zone": "us-east1-b",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no nodes",
			req: SyncRequest{
				Related: map[string]interface{}{},
			},
			expected: []corev1.Node{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := extractNodesFromRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractNodesFromRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(nodes) != len(tt.expected) {
				t.Errorf("extractNodesFromRequest() got %d nodes, want %d", len(nodes), len(tt.expected))
				return
			}

			// Check each node (order doesn't matter for this test)
			nodeMap := make(map[string]corev1.Node)
			for _, node := range nodes {
				nodeMap[node.ObjectMeta.Name] = node
			}

			for _, expectedNode := range tt.expected {
				actualNode, found := nodeMap[expectedNode.ObjectMeta.Name]
				if !found {
					t.Errorf("expected node %s not found", expectedNode.ObjectMeta.Name)
					continue
				}

				if actualNode.ObjectMeta.Name != expectedNode.ObjectMeta.Name {
					t.Errorf("node name mismatch: got %s, want %s", actualNode.ObjectMeta.Name, expectedNode.ObjectMeta.Name)
				}

				if len(actualNode.ObjectMeta.Labels) != len(expectedNode.ObjectMeta.Labels) {
					t.Errorf("node %s label count mismatch: got %d, want %d", expectedNode.ObjectMeta.Name, len(actualNode.ObjectMeta.Labels), len(expectedNode.ObjectMeta.Labels))
					continue
				}

				for key, expectedValue := range expectedNode.ObjectMeta.Labels {
					if actualValue, ok := actualNode.ObjectMeta.Labels[key]; !ok || actualValue != expectedValue {
						t.Errorf("node %s label %s mismatch: got %s, want %s", expectedNode.ObjectMeta.Name, key, actualValue, expectedValue)
					}
				}
			}
		})
	}
}
