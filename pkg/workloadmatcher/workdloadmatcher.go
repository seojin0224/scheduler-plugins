package workloadmatcher

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

// WorkloadMatcher is 
type WordloadMatcher struct {
	handle     framework.FrameworkHandle
	prometheus *PrometheusHandle
}

// Name is the name of the plugin used in the Registry and configurations.
const (
	Name = "WorkloadMatcher"
)

var _ = framework.ScorePlugin(&WordloadMatcher{})

// New initializes a new plugin and returns it.
// func New(obj runtime.Object, h framework.FrameworkHandle) (framework.Plugin, error) {
// 	args, ok := obj.(*config.WorkloadMatcherArgs)
// 	if !ok {
// 		return nil, fmt.Errorf("[WorkloadMatcher] want args to be of type WorkloadMatcherArgs, got %T", obj)
// 	}

// 	klog.Infof("[WorkloadMatcher] args received. NetworkInterface: %s; TimeRangeInMinutes: %d, Address: %s", args.NetworkInterface, args.TimeRangeInMinutes, args.Address)

// 	return &WordloadMatcher{
// 		handle:     h,
// 		prometheus: NewPrometheus(args.Address, args.NetworkInterface, time.Minute*time.Duration(args.TimeRangeInMinutes)),
// 	}, nil
// }

// Name returns name of the plugin. It is used in logs, etc.
func (n *WorkloadMatcher) Name() string {
	return Name
}

func (n *WorkloadMatcher) Score(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) (int64, *framework.Status) {
	// to implement
}

func (n *WorkloadMatcher) ScoreExtensions() framework.ScoreExtensions {
	return n
}

func (n *WorkloadMatcher) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	var higherScore int64
	for _, node := range scores {
		if higherScore < node.Score {
			higherScore = node.Score
		}
	}

	for i, node := range scores {
		scores[i].Score = framework.MaxNodeScore - (node.Score * framework.MaxNodeScore / higherScore)
	}

	klog.Infof("[WorkloadMatcher] Nodes final score: %v", scores)
	return nil
}