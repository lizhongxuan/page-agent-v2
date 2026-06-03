package memory

import (
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestEvaluateTaskIntentGateRejectsOppositeIntent(t *testing.T) {
	guide := registry.SiteTaskGuide{
		TaskIntentKey:     "restore_instance_latest_full_backup",
		TaskIntentSummary: "restore instance from latest full backup",
		TaskIntentTerms: registry.SiteTaskIntentTerms{
			Positive: []string{"restore", "backup", "恢复", "备份"},
			Negative: []string{"delete", "remove", "删除"},
		},
	}

	result := evaluateTaskIntentGate("删除这个实例的备份记录", guide)

	if result.Passed || result.Reason == "" {
		t.Fatalf("expected opposite intent to be rejected, got %#v", result)
	}
}

func TestEvaluateTaskIntentGateAcceptsPositiveIntent(t *testing.T) {
	guide := registry.SiteTaskGuide{
		TaskIntentKey:     "restore_instance_latest_full_backup",
		TaskIntentSummary: "restore instance from latest full backup",
		TaskIntentTerms: registry.SiteTaskIntentTerms{
			Positive: []string{"restore", "backup", "恢复", "备份"},
			Negative: []string{"delete", "remove", "删除"},
		},
	}

	result := evaluateTaskIntentGate("让实例基于最新全量备份恢复", guide)

	if !result.Passed || len(result.MatchedTerms) == 0 {
		t.Fatalf("expected positive intent to pass, got %#v", result)
	}
}

func TestEvaluateTaskIntentGateAcceptsVariableInstanceRestoreTask(t *testing.T) {
	guide := registry.SiteTaskGuide{
		TaskIntentKey:     "172_25_1_91_让var实例基于最新的备份数据进行恢复操作",
		TaskIntentSummary: "让{{instance_name}}实例基于最新的备份数据进行恢复操作: 1.点击实例名称 进入详情页 2.进入实例数据备份页面 3.切换到\"全量备份\"标签页并点击\"数据恢复\" 4.选择最新备份记录 5.选择节点 IP 6.点击\"开始恢复\"",
		TaskIntentTerms: registry.SiteTaskIntentTerms{
			Positive: []string{"1.点击实例名称", "进入详情页", "2.进入实例数据备份页面", "3.切换到\"全量备份\"标签页并点击\"数据恢复\"", "4.选择最新备份记录", "5.选择节点", "ip", "6.点击\"开始恢复\"", "restore", "recover"},
			Negative: []string{"delete", "remove", "删除"},
		},
	}

	result := evaluateTaskIntentGate("使用最新备份恢复 pg-prod-02 实例", guide)

	if !result.Passed || len(result.MatchedTerms) == 0 {
		t.Fatalf("expected variable instance restore task to pass, got %#v", result)
	}
}

func TestEvaluateTaskIntentGateRejectsDifferentPrimaryAction(t *testing.T) {
	guide := registry.SiteTaskGuide{
		TaskIntentKey:     "restart_deployment_after_config_update",
		TaskIntentSummary: "restart deployment after config update",
		TaskIntentTerms: registry.SiteTaskIntentTerms{
			Positive: []string{"restart", "deployment", "config", "update"},
		},
	}

	result := evaluateTaskIntentGate("delete deployment and remove pods", guide)

	if result.Passed || result.Reason != "task_action_mismatch" {
		t.Fatalf("expected primary action mismatch, got %#v", result)
	}
}
