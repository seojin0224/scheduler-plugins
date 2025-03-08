package workloadbalance

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	"sigs.k8s.io/scheduler-plugins/pkg/apis/config"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// WorkloadBalance is 
type WorkloadBalance struct {
	handle     framework.FrameworkHandle
	prometheus *PrometheusHandle
}

type PodWorkload struct {
	CPUUsage  float64 
	MemUsage  float64 
	IOStorage float64 
	IONetwork float64 
}

// Name is the name of the plugin used in the Registry and configurations.
const (
	Name = "workloadbalance"
)

var _ = framework.ScorePlugin(&WorkloadBalance{})


// New initializes a new plugin and returns it.
func New(obj runtime.Object, h framework.FrameworkHandle) (framework.Plugin, error) {
	args, ok := obj.(*config.workloadbalanceArgs)
	if !ok {
		return nil, fmt.Errorf("[workloadbalance] want args to be of type workloadbalanceArgs, got %T", obj)
	}

	klog.Infof("[workloadbalance] args received. NetworkInterface: %s; TimeRangeInMinutes: %d, Address: %s", args.NetworkInterface, args.TimeRangeInMinutes, args.Address)

	return &WorkloadBalance{
		handle:     h,
		prometheus: NewPrometheus(args.Address, args.NetworkInterface, time.Minute*time.Duration(args.TimeRangeInMinutes)),
	}, nil
}

// Name returns name of the plugin. It is used in logs, etc.
func (w *WorkloadBalance) Name() string {
	return Name
}

func (w *WorkloadBalance) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	// Node workload metrics
	metrics, err:= w.prometheus.GetNodeMetrics(nodeName)
	if err != nil {
		klog.Errorf("[workloadbalance] Error for %s: %v", nodeName, err)
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("Error metrics: %v", err))
	}

	// pod workload metrics
	servicePodsMetrics, err := w.getServicePodsMetrics(ctx, pod)
	if err != nil {
		klog.Errorf("[workloadbalance] Error for %s: %v", nodeName, err)
		// continue anyway
		servicePodsMetrics = &PodWorkload{CPUUsage: 0.5, MemUsage: 0.5, IOStorage: 0.5, IONetwork: 0.5}
	}
	alpha, beta, gamma, delta := w.computeDynamicWeigths(servicePodsMetrics)

	score := alpha*(1-nodeMetrics.CPUUsage/nodeMetrics.MaxCPU) +
		beta*(1-nodeMetrics.MemUsage/nodeMetrics.MaxMem) +
		gamma*(1-nodeMetrics.IOStorage/nodeMetrics.MaxIOStorage) +
		delta*(1-nodeMetrics.IONetwork/nodeMetrics.MaxIONetwork)
	
	finalScore := int64(score * float64(framework.MaxNodeScore))
	klog.Infof("[workloadbalance] Score du noeud %s : %d", nodeName, finalScore)

	return finalScore, nil
}

func (w *WorkloadBalance) getServicePodsMetrics(ctx context.Context, pod *v1.Pod) (*PodWorkload, error){
	// find the service
	serviceList, err := w.handle.ClientSet().CoreV1().Services(pod.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("erreur récupération services: %v", err)
	}

	//vérifier si ça peut être optimisé
	var matchedService *v1.Services
	for _, service := range serviceList.Items {
	// verify if the service has a selector that matches the pod labels
		if service.Spec.Selector != nil {
			match := true
			for key, value := range service.Spec.Selector {
				if pod.Labels[key] != value {
					match = false
					break
				}
			}
			if match {
				matchedService = &service
				break
			}
		}
	}

	if matchedService == nil {
		// there is no matched service (requirements)
	}

	podList, err := w.handle.ClientSet().CoreV1().Pods(pod.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: matchedService.Spec.Selector}),
	})
	if err != nil {
		return nil, fmt.Errorf("error while getting service's pods: %v", err)
	}

	totalCPU, totalMem, totalIOStorage, totalIONetwork := 0.0, 0.0, 0.0, 0.0
	podCount := 0

	for _, p := range podList.Items {
		metrics, err := w.prometheus.GetPodMetrics(p.Name, p.Namespace)
		if err == nil {
			totalCPU += metrics.CPUUsage
			totalMem += metrics.MemUsage
			totalIOStorage += metrics.IOStorage
			totalIONetwork += metrics.IONetwork
			podCount++
		}
	}

	if podCount == 0 {
		return nil, fmt.Errorf("No metrics for pods of the service %s", matchedService.Name)
	}

	return &PodWorkload{
		CPUUsage: totalCPU / float64(podCount),
		MemeUsage: totalMem / float64(podCount),
		IOStorage: totalIOStorage / float64(podCount),
		IONetwork: totalIONetwork / float64(podCount),
	}, nil
}	

func (w *WorkloadMatcher) computeDynamicWeights(workload *PodWorkload) (float64, float64, float64, float64) {
	// On ajuste les poids selon la charge observée sur les pods du service
	alpha, beta, gamma, delta := 0.4, 0.3, 0.15, 0.15

	// Si le service consomme beaucoup de CPU, on donne plus de poids au CPU
	if workload.CPUUsage > 0.7 {
		alpha += 0.1
		beta -= 0.05
	}

	// Si le service est très memory-bound, on ajuste en faveur de la mémoire
	if workload.MemUsage > 0.7 {
		beta += 0.1
		alpha -= 0.05
	}

	// Si les pods sont I/O intensive, on donne plus d’importance à l’IO
	if workload.IOStorage > 0.7 {
		gamma += 0.1
	}

	if workload.IONetwork > 0.7 {
		delta += 0.1
	}

	return alpha, beta, gamma, delta
}


func (w *WorkloadBalance) ScoreExtensions() framework.ScoreExtensions {
	return n
}

func (w *WorkloadBalance) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	var higherScore int64
	for _, node := range scores {
		if higherScore < node.Score {
			higherScore = node.Score
		}
	}

	for i, node := range scores {
		scores[i].Score = framework.MaxNodeScore - (node.Score * framework.MaxNodeScore / higherScore)
	}

	klog.Infof("[WorkloadBalance] Nodes final score: %v", scores)
	return nil
}