package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"k8s.io/klog/v2"

	"fake-kubelet/pkg/api"
	"fake-kubelet/pkg/config"
	"fake-kubelet/pkg/node"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()

	// 创建基础配置
	cfg := config.NewConfig()

	// 从环境变量或命令行参数覆盖配置
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cfg.Kubeconfig = kubeconfig
	}
	if apiServer := os.Getenv("APISERVER_URL"); apiServer != "" {
		cfg.APIServerURL = apiServer
	}
	if cpu := os.Getenv("NODE_CPU"); cpu != "" {
		cfg.CapacityCPU = cpu
	}
	if memory := os.Getenv("NODE_MEMORY"); memory != "" {
		cfg.CapacityMemory = memory
	}
	if labels := os.Getenv("NODE_LABELS"); labels != "" {
		cfg.NodeLabels = config.ParseLabels(labels)
	}
	// 节点名支持批量：fake-node-1,fake-node-2
	if nodeName := os.Getenv("NODE_NAME"); nodeName != "" {
		cfg.NodeName = nodeName
	}

	// 解析节点列表
	nodeNames := parseNodeNames(cfg.NodeName)
	if len(nodeNames) == 0 {
		klog.Fatal("No nodes to create. Please set NODE_NAME or config.NodeName")
	}

	// 验证基础配置
	if err := cfg.Validate(); err != nil {
		klog.Fatalf("Invalid configuration: %v", err)
	}
	cfg.LogConfig()

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ============================================
	// 批量创建 Client 和 NodeManager
	// ============================================
	var nodeManagers []*node.Manager

	for _, name := range nodeNames {
		klog.Infof("Initializing node: %s", name)

		// 为每个节点创建独立配置（避免共享指针导致 NodeName 竞争）
		// 如果 config.Config 字段很多，建议给 Config 添加 Copy() 方法
		nodeCfg := newNodeConfig(cfg, name)

		// 创建 API Client
		// 注：如果 api.NewClient 返回的是标准 client-go，它是并发安全的，
		//     可以考虑只创建一个 client 供所有节点共享以减少连接数
		client, err := api.NewClient(nodeCfg.Kubeconfig, nodeCfg.APIServerURL)
		if err != nil {
			klog.Fatalf("Failed to create API client for node %s: %v", name, err)
		}

		nm := node.NewManager(client, nodeCfg)
		nodeManagers = append(nodeManagers, nm)
	}

	// ============================================
	// 批量注册节点
	// ============================================
	for _, nm := range nodeManagers {
		name := nm.GetNodeName() // 或者 nm.GetNodeName()，取决于你的 Manager 实现
		klog.Infof("Registering node: %s", name)
		if err := nm.Register(ctx); err != nil {
			klog.Fatalf("Failed to register node %s: %v", name, err)
		}
	}

	// ============================================
	// 批量启动心跳（后台 goroutine）
	// ============================================
	var wg sync.WaitGroup
	for _, nm := range nodeManagers {
		wg.Add(1)
		go func(m *node.Manager) {
			defer wg.Done()
			m.StartHeartbeat(ctx)
		}(nm)
	}

	// ============================================
	// 等待退出信号
	// ============================================
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	klog.Info("Received shutdown signal, cleaning up...")

	// ============================================
	// 批量清理资源
	// ============================================
	for _, nm := range nodeManagers {
		if err := nm.Stop(ctx); err != nil {
			klog.Errorf("Failed to clean up node: %v", err)
		}
	}
	cancel() // 通知所有 goroutine 退出
	wg.Wait()
	klog.Info("Fake Kubelet stopped gracefully")
}

// parseNodeNames 解析逗号分隔的节点名
func parseNodeNames(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var names []string
	for _, p := range parts {
		if n := strings.TrimSpace(p); n != "" {
			names = append(names, n)
		}
	}
	return names
}

// newNodeConfig 基于基础配置创建节点专属配置
// 避免多个 Manager 共享同一个 *config.Config 指针
func newNodeConfig(base *config.Config, nodeName string) *config.Config {
	return &config.Config{
		Kubeconfig:        base.Kubeconfig,
		APIServerURL:      base.APIServerURL,
		NodeName:          nodeName,
		CapacityCPU:       base.CapacityCPU,
		CapacityMemory:    base.CapacityMemory,
		NodeLabels:        base.NodeLabels,
		HeartbeatInterval: base.HeartbeatInterval,
		// TODO: 如果 config.Config 有其他字段，请在此处一并复制
	}
}
