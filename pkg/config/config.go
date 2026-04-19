package config

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

// Config 包含 Fake Kubelet 的所有配置
type Config struct {
	// 节点配置
	NodeName       string
	NodeLabels     map[string]string
	CapacityCPU    string
	CapacityMemory string
	CapacityPods   int64

	// API Server 配置
	APIServerURL string
	Kubeconfig   string

	// 运行间隔配置
	HeartbeatInterval time.Duration
	PodSyncInterval   time.Duration

	// 日志配置
	Verbosity int
}

// NewConfig 创建默认配置
func NewConfig() *Config {
	return &Config{
		NodeName:          "fake-node-1,fake-2,fake-3",
		NodeLabels:        make(map[string]string),
		CapacityCPU:       "4",
		CapacityMemory:    "16Gi",
		CapacityPods:      110,
		HeartbeatInterval: 10 * time.Second,
		PodSyncInterval:   5 * time.Second,
		Verbosity:         2,
		Kubeconfig:        "/Users/phil/.kube/config",
	}
}

// Validate 验证配置有效性
func (c *Config) Validate() error {
	if c.NodeName == "" {
		return fmt.Errorf("node-name is required")
	}

	// 验证 CPU 配置
	_, err := resource.ParseQuantity(c.CapacityCPU)
	if err != nil {
		return fmt.Errorf("invalid capacity-cpu: %v", err)
	}

	// 验证内存配置
	_, err = resource.ParseQuantity(c.CapacityMemory)
	if err != nil {
		return fmt.Errorf("invalid capacity-memory: %v", err)
	}

	return nil
}

// GetCapacityCPU 返回解析后的 CPU 资源量
func (c *Config) GetCapacityCPU() resource.Quantity {
	q, _ := resource.ParseQuantity(c.CapacityCPU)
	return q
}

// GetCapacityMemory 返回解析后的内存资源量
func (c *Config) GetCapacityMemory() resource.Quantity {
	q, _ := resource.ParseQuantity(c.CapacityMemory)
	return q
}

// ParseLabels 解析标签字符串 (key1=value1,key2=value2)
func ParseLabels(labelsStr string) map[string]string {
	labels := make(map[string]string)
	if labelsStr == "" {
		return labels
	}

	pairs := strings.Split(labelsStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 {
			labels[kv[0]] = kv[1]
		}
	}
	return labels
}

// LogConfig 打印配置信息
func (c *Config) LogConfig() {
	klog.Infof("=== Fake Kubelet Configuration ===")
	klog.Infof("Kube Config: %v", c.Kubeconfig)
	klog.Infof("Node Name: %s", c.NodeName)
	klog.Infof("Node Labels: %v", c.NodeLabels)
	klog.Infof("Capacity CPU: %s", c.CapacityCPU)
	klog.Infof("Capacity Memory: %s", c.CapacityMemory)
	klog.Infof("Capacity Pods: %d", c.CapacityPods)
	klog.Infof("APIServer URL: %s", c.APIServerURL)
	klog.Infof("Heartbeat Interval: %v", c.HeartbeatInterval)
	klog.Infof("Pod Sync Interval: %v", c.PodSyncInterval)
	klog.Infof("================================")
}
