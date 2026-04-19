package node

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"fake-kubelet/pkg/api"
	"fake-kubelet/pkg/config"
)

// Manager 负责节点注册和心跳管理
type Manager struct {
	client     *api.Client
	config     *config.Config
	node       *v1.Node
	stopCh     chan struct{}
	registered bool
}

// NewManager 创建节点管理器
func NewManager(client *api.Client, cfg *config.Config) *Manager {
	return &Manager{
		client: client,
		config: cfg,
		stopCh: make(chan struct{}),
	}
}

// Register 注册节点到 API Server
func (m *Manager) Register(ctx context.Context) error {
	node := m.createNode()
	m.node = node

	err := m.client.RegisterNode(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}
	m.registered = true
	return nil
}

// createNode 创建 Node 对象
func (m *Manager) createNode() *v1.Node {
	cpuQty := m.config.GetCapacityCPU()
	memQty := m.config.GetCapacityMemory()

	node := &v1.Node{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Node",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: m.config.NodeName,
			Labels: map[string]string{
				"kubernetes.io/hostname":  m.config.NodeName,
				"node.kubernetes.io/fake": "true",
			},
		},
		Spec: v1.NodeSpec{
			// 允许调度
			Unschedulable: false,
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:    cpuQty,
				v1.ResourceMemory: memQty,
				v1.ResourcePods:   resource.MustParse(fmt.Sprintf("%d", m.config.CapacityPods)),
			},
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    cpuQty,
				v1.ResourceMemory: memQty,
				v1.ResourcePods:   resource.MustParse(fmt.Sprintf("%d", m.config.CapacityPods)),
			},
			Conditions: []v1.NodeCondition{
				{
					Type:               v1.NodeReady,
					Status:             v1.ConditionTrue,
					Reason:             "KubeletReady",
					Message:            "kubelet is ready",
					LastTransitionTime: metav1.Now(),
					LastHeartbeatTime:  metav1.Now(),
				},
				{
					Type:               v1.NodeMemoryPressure,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasSufficientMemory",
					Message:            "kubelet has sufficient memory available",
					LastTransitionTime: metav1.Now(),
					LastHeartbeatTime:  metav1.Now(),
				},
				{
					Type:               v1.NodeDiskPressure,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasNoDiskPressure",
					Message:            "kubelet has no disk pressure",
					LastTransitionTime: metav1.Now(),
					LastHeartbeatTime:  metav1.Now(),
				},
				{
					Type:               v1.NodePIDPressure,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasSufficientPID",
					Message:            "kubelet has sufficient PID available",
					LastTransitionTime: metav1.Now(),
					LastHeartbeatTime:  metav1.Now(),
				},
				{
					Type:               v1.NodeNetworkUnavailable,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasSufficientNetwork",
					Message:            "kubelet has sufficient network available",
					LastTransitionTime: metav1.Now(),
					LastHeartbeatTime:  metav1.Now(),
				},
			},
		},
	}

	// 添加自定义标签
	for k, v := range m.config.NodeLabels {
		node.Labels[k] = v
	}

	return node
}

// StartHeartbeat 启动心跳循环
func (m *Manager) StartHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(m.config.HeartbeatInterval)
	defer ticker.Stop()

	klog.Infof("Starting heartbeat for node %s (interval: %v)", m.config.NodeName, m.config.HeartbeatInterval)

	for {
		select {
		case <-ctx.Done():
			klog.Info("Heartbeat stopped: context cancelled")
			return
		case <-m.stopCh:
			klog.Info("Heartbeat stopped: stop signal received")
			return
		case <-ticker.C:
			m.sendHeartbeat(ctx)
		}
	}
}

// sendHeartbeat 发送一次心跳
func (m *Manager) sendHeartbeat(ctx context.Context) {
	now := metav1.Now()
	conditions := []v1.NodeCondition{
		{
			Type:               v1.NodeReady,
			Status:             v1.ConditionTrue,
			Reason:             "KubeletReady",
			Message:            "kubelet is ready",
			LastTransitionTime: m.node.Status.Conditions[0].LastTransitionTime,
			LastHeartbeatTime:  now,
		},
		{
			Type:               v1.NodeMemoryPressure,
			Status:             v1.ConditionFalse,
			Reason:             "KubeletHasSufficientMemory",
			Message:            "kubelet has sufficient memory available",
			LastTransitionTime: m.node.Status.Conditions[1].LastTransitionTime,
			LastHeartbeatTime:  now,
		},
		{
			Type:               v1.NodeDiskPressure,
			Status:             v1.ConditionFalse,
			Reason:             "KubeletHasNoDiskPressure",
			Message:            "kubelet has no disk pressure",
			LastTransitionTime: m.node.Status.Conditions[2].LastTransitionTime,
			LastHeartbeatTime:  now,
		},
		{
			Type:               v1.NodePIDPressure,
			Status:             v1.ConditionFalse,
			Reason:             "KubeletHasSufficientPID",
			Message:            "kubelet has sufficient PID available",
			LastTransitionTime: m.node.Status.Conditions[3].LastTransitionTime,
			LastHeartbeatTime:  now,
		},
		{
			Type:               v1.NodeNetworkUnavailable,
			Status:             v1.ConditionFalse,
			Reason:             "KubeletHasSufficientNetwork",
			Message:            "kubelet has sufficient network available",
			LastTransitionTime: m.node.Status.Conditions[4].LastTransitionTime,
			LastHeartbeatTime:  now,
		},
	}

	err := m.client.UpdateNodeHeartbeat(ctx, m.config.NodeName, conditions)
	if err != nil {
		klog.Errorf("Failed to send heartbeat for node %s: %v", m.config.NodeName, err)
	} else {
		klog.V(4).Infof("Heartbeat sent for node %s at %v", m.config.NodeName, now.Time)
	}
}

// Stop 停止节点管理器
func (m *Manager) Stop(ctx context.Context) error {
	close(m.stopCh)
	if m.registered {
		return m.client.DeleteNode(ctx, m.config.NodeName)
	}
	return nil
}

// GetNodeName 返回节点名称
func (m *Manager) GetNodeName() string {
	return m.config.NodeName
}
