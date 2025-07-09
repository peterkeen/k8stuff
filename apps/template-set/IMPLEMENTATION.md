# TemplateSet Implementation Summary

## Overview

TemplateSet is a Kubernetes controller that creates one pod per node in the cluster, similar to DaemonSet but with advanced templating capabilities. It uses metacontroller's webhook pattern to manage pod lifecycle and discovery, and leverages official Kubernetes API types for type safety and consistency.

## Key Features Implemented

### 1. Node Discovery via Customize Hook
- Implements `/customize` endpoint that tells metacontroller to watch all nodes (`v1/nodes`)
- Automatically discovers all nodes in the cluster without requiring cluster-admin permissions for the webhook
- Handles dynamic node additions/removals through metacontroller's watch mechanism

### 2. Template Engine
- Supports templating in pod names, labels, and environment variables
- Template syntax: `{{.Node.Labels . 'key'}}` for node labels and `{{.Node.Name}}` for node names
- Applied during pod creation for each node

### 3. One Pod Per Node
- Creates exactly one pod per node in the cluster
- Pods are scheduled to specific nodes using `nodeName` field
- Handles both worker and master nodes (all nodes with scheduling enabled)

### 4. Service Per Pod (Optional)
- Creates a dedicated service for each pod when `servicePerPod.enabled: true`
- Services use `templateset.keen.land/podname` label selector
- Service names include deterministic suffixes to prevent conflicts

### 5. Official Kubernetes API Types
- Uses `corev1.Node` for node representation instead of custom types
- Uses `corev1.PodTemplateSpec` for pod templates with full Kubernetes API compatibility
- Uses `metav1.LabelSelector` for label selectors
- Ensures type safety and compatibility with Kubernetes ecosystem

## Architecture

### HTTP Endpoints
- `/health` - Health check endpoint
- `/customize` - Tells metacontroller which resources to watch (nodes)
- `/sync` - Main reconciliation logic, creates/updates pods and services

### Core Components

#### Node Extraction (`extractNodesFromRequest`)
- Parses node data from metacontroller's related resources into `[]corev1.Node`
- Extracts node metadata (name, labels) from `Children["Node.v1"]`
- Handles missing nodes gracefully (returns empty slice)
- Creates proper `corev1.Node` objects with `metav1.ObjectMeta`

#### Pod Creation (`createPodForNode`)
- Takes `TemplateSetSpec` with `corev1.PodTemplateSpec` and `corev1.Node` as input
- Applies templates to generate pod names, labels, and environment variables
- Adds required labels like `templateset.keen.land/podname`
- Sets `nodeName` to ensure pod scheduling to specific node
- Generates deterministic suffixes for unique naming
- Leverages official Kubernetes container and environment variable types

#### Service Creation (`createServiceForPod`)
- Creates services with selectors matching pod names
- Uses port 80 -> 8080 mapping by default
- Names services as `{pod-name}-service`

#### Template Processing (`applyTemplate`)
- Simple string replacement for node label and name templates using `corev1.Node.ObjectMeta`
- Supports `{{.Node.Name}}` and `{{.Node.Labels . 'key'}}` patterns
- Currently supports exact template matches
- Extensible for more complex template engines

## Workflow

1. **Customize Phase**: Metacontroller calls `/customize`, webhook responds with node watching configuration
2. **Sync Phase**: Metacontroller calls `/sync` with current TemplateSet and related nodes
3. **Node Processing**: For each discovered node:
   - Apply templates to generate pod configuration
   - Create pod with `nodeName` set to target node
   - Optionally create corresponding service
4. **Response**: Return all generated pods/services to metacontroller for creation

## Configuration

### Metacontroller Setup
```yaml
hooks:
  customize:
    webhook:
      url: http://templateset-webhook.metacontroller.svc.cluster.local:8080/customize
  sync:
    webhook:
      url: http://templateset-webhook.metacontroller.svc.cluster.local:8080/sync
```

### Resource Watching
- Watches `templatesets.templateset.keen.land/v1` as parent resource
- Manages `v1/pods` and `v1/services` as child resources
- Uses `Recreate` strategy for pods, `InPlace` for services

## Testing Coverage

### Unit Tests
- `TestCustomizeHandler` - Customize endpoint functionality
- `TestExtractNodesFromRequest` - Node data parsing with multiple scenarios
- `TestCreatePodForNode` - Pod generation with templating
- `TestCreateServiceForPod` - Service generation
- `TestApplyTemplate` - Template processing logic
- `TestSync` - End-to-end sync workflow

### Test Scenarios
- Single node clusters
- Multi-node clusters with different zones
- Missing node data handling
- Template variable substitution
- Service creation enablement/disablement

## Dependencies

### Kubernetes API Libraries
- `k8s.io/api v0.28.0` - Official Kubernetes API types
- `k8s.io/apimachinery v0.28.0` - Kubernetes API machinery (metav1, etc.)
- Ensures compatibility with Kubernetes v1.28+ APIs

## Deployment

### Container Image
- Built from `golang:1.21-alpine` base image
- Includes Kubernetes API dependencies for type safety
- Exposes port 8080 for webhook endpoints
- Runs as non-root user in `alpine:latest` runtime

### Kubernetes Deployment
- Deployed in `metacontroller` namespace
- Service exposes port 8080 for metacontroller communication
- Configurable resource limits (100m CPU, 128Mi memory)

## Limitations & Future Enhancements

### Current Limitations
- Simple template engine (exact string matching only)
- Fixed deterministic suffix generation
- Basic error handling for malformed templates
- No support for complex template expressions
- Limited to Kubernetes v0.28.0 API compatibility

### Potential Enhancements
- Full template engine (Go templates, Jinja2-style)
- Configurable service port mappings using Kubernetes API types
- Node selection criteria (taints, node selectors, etc.) using `corev1.NodeSelector`
- Pod update strategies beyond recreation
- Metrics and monitoring integration
- Custom resource status reporting with detailed conditions
- Support for additional Kubernetes resources (ConfigMaps, Secrets templating)
- Upgrade to newer Kubernetes API versions as they become available