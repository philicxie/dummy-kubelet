package api

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// Client 封装与 Kubernetes API Server 的交互
type Client struct {
	clientset *kubernetes.Clientset
}

// NewClient 创建 API Client
func NewClient(kubeconfig, masterURL string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	klog.Info("Successfully created Kubernetes client")
	return &Client{clientset: clientset}, nil
}

// RegisterNode 注册节点到 API Server
func (c *Client) RegisterNode(ctx context.Context, node *v1.Node) error {
	_, err := c.clientset.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			klog.Warningf("Node %s already exists, will try to update", node.Name)
			return c.updateNodeIfNeeded(ctx, node)
		}
		return fmt.Errorf("failed to register node: %w", err)
	}
	klog.Infof("Successfully registered node: %s", node.Name)
	return nil
}

// updateNodeIfNeeded 如果节点已存在则更新
func (c *Client) updateNodeIfNeeded(ctx context.Context, node *v1.Node) error {
	existingNode, err := c.clientset.CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get existing node: %w", err)
	}

	// 只在节点资源容量不同时更新
	if !nodeCapacityEqual(&existingNode.Status, &node.Status) {
		klog.Infof("Updating existing node %s capacity", node.Name)
		existingNode.Status = node.Status
		_, err = c.clientset.CoreV1().Nodes().UpdateStatus(ctx, existingNode, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update node: %w", err)
		}
	}
	return nil
}

// nodeCapacityEqual 比较两个节点的资源容量是否相等
func nodeCapacityEqual(a, b *v1.NodeStatus) bool {
	return a.Capacity.Cpu().Cmp(*b.Capacity.Cpu()) == 0 &&
		a.Capacity.Memory().Cmp(*b.Capacity.Memory()) == 0 &&
		a.Capacity.Pods().Cmp(*b.Capacity.Pods()) == 0
}

// UpdateNodeHeartbeat 更新节点心跳
func (c *Client) UpdateNodeHeartbeat(ctx context.Context, nodeName string, conditions []v1.NodeCondition) error {
	heartbeatData := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": conditions,
		},
	}
	heartbeatBytes, err := JSONMergePatch(heartbeatData)
	if err != nil {
		return fmt.Errorf("failed to create heartbeat data: %w", err)
	}

	_, err = c.clientset.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, heartbeatBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return fmt.Errorf("failed to update node heartbeat: %w", err)
	}
	return nil
}

// UpdateNodeResources 更新节点资源使用情况
func (c *Client) UpdateNodeResources(ctx context.Context, nodeName string, capacity, allocatable v1.ResourceList) error {
	resourceData := map[string]interface{}{
		"status": map[string]interface{}{
			"capacity":    capacity,
			"allocatable": allocatable,
		},
	}
	resourceBytes, err := JSONMergePatch(resourceData)
	if err != nil {
		return fmt.Errorf("failed to create resource data: %w", err)
	}

	_, err = c.clientset.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, resourceBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return fmt.Errorf("failed to update node resources: %w", err)
	}
	return nil
}

// DeleteNode 删除节点
func (c *Client) DeleteNode(ctx context.Context, nodeName string) error {
	err := c.clientset.CoreV1().Nodes().Delete(ctx, nodeName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete node: %w", err)
	}
	klog.Infof("Node %s deleted (or not found)", nodeName)
	return nil
}

// GetPods 获取分配到指定节点的 Pod 列表
func (c *Client) GetPods(ctx context.Context, nodeName string) (*v1.PodList, error) {
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	return pods, nil
}

// UpdatePodStatus 更新 Pod 状态
func (c *Client) UpdatePodStatus(ctx context.Context, namespace, podName string, status v1.PodStatus) error {
	statusData := map[string]interface{}{
		"status": status,
	}
	statusBytes, err := JSONMergePatch(statusData)
	if err != nil {
		return fmt.Errorf("failed to create status data: %w", err)
	}

	_, err = c.clientset.CoreV1().Pods(namespace).Patch(ctx, podName, types.MergePatchType, statusBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return fmt.Errorf("failed to update pod status: %w", err)
	}
	return nil
}

// GetNode 获取节点信息
func (c *Client) GetNode(ctx context.Context, nodeName string) (*v1.Node, error) {
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	return node, nil
}

// JSONMergePatch 创建 JSON Merge Patch 数据
func JSONMergePatch(data map[string]interface{}) ([]byte, error) {
	return json.Marshal(data)
}
