package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/page-agent/workflow-backend/internal/registry"
)

func TestSiteTaskGuideServiceCompressesSuccessfulRestoreRun(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	seedGuidePages(t, ctx, repo)
	run := registry.TaskRun{
		ID:            "task_run_restore",
		ProjectID:     "default",
		Site:          "middleware.example.com",
		TaskTemplate:  "让 lzxpg 实例基于最新备份恢复",
		Summary:       "实例恢复完成。",
		Status:        registry.TaskRunSuccess,
		OriginalPath:  []string{"page_instances", "page_settings", "page_instances", "page_detail", "page_backups"},
		OptimizedPath: []string{"page_instances", "page_detail", "page_backups"},
		ActionSteps: []registry.ActionStep{
			{ID: "step_noise", PageStateID: "page_settings", StepIndex: 1, ActionType: registry.StepClick, TargetName: "系统设置", ResultSummary: "误入设置页。", IsBranchNoise: true},
			{ID: "step_open_instance", PageStateID: "page_instances", StepIndex: 2, ActionType: registry.StepClick, TargetName: "实例名称", ResultSummary: "进入实例详情页。"},
			{ID: "step_backup_page", PageStateID: "page_detail", StepIndex: 3, ActionType: registry.StepClick, TargetName: "数据备份", ResultSummary: "打开数据备份页。"},
			{ID: "step_full_backup", PageStateID: "page_backups", StepIndex: 4, ActionType: registry.StepClick, TargetName: "全量备份", ResultSummary: "切换到全量备份。"},
			{ID: "step_restore", PageStateID: "page_backups", StepIndex: 5, ActionType: registry.StepClick, TargetName: "数据恢复", ResultSummary: "打开恢复弹窗。"},
			{ID: "step_latest", PageStateID: "page_backups", StepIndex: 6, ActionType: registry.StepClick, TargetName: "最新备份记录", ResultSummary: "选择最新备份记录。"},
			{ID: "step_node", PageStateID: "page_backups", StepIndex: 7, ActionType: registry.StepSelect, TargetName: "节点 IP", ValueTemplate: "{{node_ip}}", ResultSummary: "选择节点。"},
			{ID: "step_confirm", PageStateID: "page_backups", StepIndex: 8, ActionType: registry.StepClick, TargetName: "开始恢复", ResultSummary: "提交恢复。"},
		},
	}

	guide, generated, err := NewSiteTaskGuideService(repo).UpsertGuideFromTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("UpsertGuideFromTaskRun failed: %v", err)
	}
	if !generated {
		t.Fatal("expected guide to be generated")
	}
	if len(guide.Steps) != 6 {
		t.Fatalf("expected six useful guide steps, got %#v", guide.Steps)
	}
	if strings.Contains(strings.Join(guideTaskGuideStepText(guide.Steps), "\n"), "系统设置") {
		t.Fatalf("guide should omit branch noise: %#v", guide.Steps)
	}
	if strings.Contains(guide.Summary, "lzxpg") || strings.Contains(guide.SearchableText(), "192.168.") {
		t.Fatalf("guide leaked instance data: %#v", guide)
	}
	if !strings.Contains(guide.Summary, "{{instance_name}}") {
		t.Fatalf("guide should template instance names, got %q", guide.Summary)
	}
	if guide.StartPageGuard.URLPattern == "" || len(guide.PageGuards) == 0 {
		t.Fatalf("expected page guards: %#v", guide)
	}
}

func TestSiteTaskGuideServiceUpdatesSameGuideWithoutTimestampCopies(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewSiteTaskGuideService(repo)
	run := registry.TaskRun{
		ID:           "task_run_1",
		ProjectID:    "default",
		Site:         "ops.example.com",
		TaskTemplate: "恢复 {{instance_name}} 实例",
		Summary:      "恢复完成。",
		Status:       registry.TaskRunSuccess,
		ActionSteps: []registry.ActionStep{
			{ID: "step_open", StepIndex: 1, ActionType: registry.StepClick, TargetName: "实例名称", ResultSummary: "进入详情页。", BeforeObservation: guideTestObservation("实例列表", "实例名称", "实例名称")},
			{ID: "step_restore", StepIndex: 2, ActionType: registry.StepClick, TargetName: "数据恢复", ResultSummary: "提交恢复。", BeforeObservation: guideTestObservation("数据备份", "数据恢复", "数据恢复")},
		},
	}
	if _, generated, err := service.UpsertGuideFromTaskRun(ctx, run); err != nil || !generated {
		t.Fatalf("first upsert failed generated=%v err=%v", generated, err)
	}
	run.ID = "task_run_2"
	run.ActionSteps = append(run.ActionSteps, registry.ActionStep{ID: "step_confirm", StepIndex: 3, ActionType: registry.StepClick, TargetName: "开始恢复", ResultSummary: "确认恢复。", BeforeObservation: guideTestObservation("恢复弹窗", "开始恢复", "开始恢复")})
	if _, generated, err := service.UpsertGuideFromTaskRun(ctx, run); err != nil || !generated {
		t.Fatalf("second upsert failed generated=%v err=%v", generated, err)
	}
	guides, err := repo.ListSiteTaskGuides(ctx, registry.SiteTaskGuideListQuery{ProjectID: "default", Site: "ops.example.com"})
	if err != nil {
		t.Fatalf("ListSiteTaskGuides failed: %v", err)
	}
	if len(guides) != 1 {
		t.Fatalf("expected one merged guide, got %#v", guides)
	}
	if guides[0].SuccessCount != 2 {
		t.Fatalf("expected success count to increase, got %#v", guides[0])
	}
}

func TestSiteTaskGuideServiceRejectsLowQualityOrDynamicGuides(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	service := NewSiteTaskGuideService(repo)
	cases := []registry.TaskRun{
		{
			ID:           "task_run_failed",
			ProjectID:    "default",
			Site:         "ops.example.com",
			TaskTemplate: "恢复 {{instance_name}}",
			Status:       registry.TaskRunFailed,
			ActionSteps:  []registry.ActionStep{{ID: "step", ActionType: registry.StepClick, TargetName: "数据恢复", ResultSummary: "失败。"}},
		},
		{
			ID:           "task_run_placeholder",
			ProjectID:    "default",
			Site:         "ops.example.com",
			TaskTemplate: "{{input_value}}",
			Status:       registry.TaskRunSuccess,
			ActionSteps:  []registry.ActionStep{{ID: "step", ActionType: registry.StepClick, TargetName: "数据恢复", ResultSummary: "完成。"}},
		},
		{
			ID:           "task_run_dynamic",
			ProjectID:    "default",
			Site:         "ops.example.com",
			TaskTemplate: "恢复实例",
			Status:       registry.TaskRunSuccess,
			ActionSteps:  []registry.ActionStep{{ID: "step", ActionType: registry.StepClick, TargetName: "element_33", ResultSummary: "完成。"}},
		},
	}
	for _, run := range cases {
		if _, generated, err := service.UpsertGuideFromTaskRun(ctx, run); err != nil || generated {
			t.Fatalf("expected low-quality run %s to be ignored, generated=%v err=%v", run.ID, generated, err)
		}
	}
}

func TestSiteTaskGuideServiceDerivesStableTargetsFromElementResultSummaries(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SavePageState(ctx, registry.PageState{
		ID:             "page_console",
		ProjectID:      "default",
		Site:           "ops.example.com",
		URLPattern:     "https://ops.example.com/console/",
		CanonicalTitle: "PostgreSQL",
		RequiredText:   []string{"PostgreSQL"},
		Status:         registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageState failed: %v", err)
	}

	run := registry.TaskRun{
		ID:            "task_run_dynamic_targets",
		ProjectID:     "default",
		Site:          "ops.example.com",
		TaskTemplate:  "让 {{instance_name}} 实例基于最新备份恢复",
		Summary:       "恢复已启动。",
		Status:        registry.TaskRunSuccess,
		OriginalPath:  []string{"page_console"},
		OptimizedPath: []string{"page_console"},
		ActionSteps: []registry.ActionStep{
			{
				ID:               "step_instance",
				PageStateID:      "page_console",
				StepIndex:        1,
				ActionType:       registry.StepClick,
				TargetName:       "element_33",
				ResultSummary:    "✅ Clicked element ([33]<button type=button>lzxpg />).",
				ReasoningSummary: "点击 lzxpg 实例名称按钮进入详情页。",
			},
			{
				ID:               "step_backup",
				PageStateID:      "page_console",
				StepIndex:        2,
				ActionType:       registry.StepClick,
				TargetName:       "element_21",
				ResultSummary:    "✅ Clicked element ([21]<div role=tab>全量备份 />).",
				ReasoningSummary: "点击全量备份标签。",
			},
			{
				ID:               "step_restore",
				PageStateID:      "page_console",
				StepIndex:        3,
				ActionType:       registry.StepClick,
				TargetName:       "element_62",
				ResultSummary:    "✅ Clicked element ([62]<button type=button>开始恢复 />).",
				ReasoningSummary: "点击\"开始恢复\"按钮。",
			},
		},
	}

	guide, generated, err := NewSiteTaskGuideService(repo).UpsertGuideFromTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("UpsertGuideFromTaskRun failed: %v", err)
	}
	if !generated {
		t.Fatal("expected guide to be generated from stable semantic targets")
	}
	if len(guide.Steps) != 3 {
		t.Fatalf("expected three guide steps, got %#v", guide.Steps)
	}
	if guide.Steps[0].Target != "实例名称" || guide.Steps[1].Target != "全量备份" || guide.Steps[2].Target != "开始恢复" {
		t.Fatalf("expected parsed stable targets, got %#v", guide.Steps)
	}
	joined := strings.Join(guideTaskGuideStepText(guide.Steps), "\n")
	if strings.Contains(joined, "element_") || strings.Contains(joined, "<button") || strings.Contains(joined, "<div") || strings.Contains(joined, "lzxpg") {
		t.Fatalf("guide leaked raw element summaries: %s", joined)
	}
}

func TestSiteTaskGuideServicePrefersNumberedTaskSummaryOverElementSteps(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	if err := repo.SavePageState(ctx, registry.PageState{
		ID:             "page_console",
		ProjectID:      "default",
		Site:           "ops.example.com",
		URLPattern:     "https://ops.example.com/console/",
		CanonicalTitle: "PostgreSQL",
		RequiredText:   []string{"PostgreSQL", "数据备份", "全量备份", "数据恢复"},
		RequiredControls: []registry.ControlSignature{
			{Role: "button", Name: "数据恢复"},
		},
		Status: registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageState failed: %v", err)
	}
	run := registry.TaskRun{
		ID:        "task_run_numbered_summary",
		ProjectID: "default",
		Site:      "ops.example.com",
		TaskTemplate: "让lzxpg实例基于最新的备份数据进行恢复操作:\n" +
			"1.点击实例名称 进入详情页\n" +
			"2.进入实例数据备份页面\n" +
			"3.切换到\"全量备份\"标签页并点击\"数据恢复\"\n" +
			"4.选择最新备份记录\n" +
			"5.选择节点 IP\n" +
			"6.点击\"开始恢复\"",
		Status:        registry.TaskRunSuccess,
		OriginalPath:  []string{"page_console"},
		OptimizedPath: []string{"page_console"},
		ActionSteps: []registry.ActionStep{
			{ID: "step_instance", PageStateID: "page_console", StepIndex: 1, ActionType: registry.StepClick, TargetName: "element_33", ReasoningSummary: "点击 lzxpg 实例名称按钮进入详情页。", ResultSummary: "✅ Clicked element ([33]<button type=button>lzxpg />)."},
			{ID: "step_backup", PageStateID: "page_console", StepIndex: 2, ActionType: registry.StepClick, TargetName: "element_21", ReasoningSummary: "点击\"数据备份\"标签进入数据备份页面。", ResultSummary: "✅ Clicked element (21)."},
			{ID: "step_restore", PageStateID: "page_console", StepIndex: 3, ActionType: registry.StepClick, TargetName: "element_28", ReasoningSummary: "点击\"数据恢复\"按钮开始数据恢复流程。", ResultSummary: "✅ Clicked element (28)."},
			{ID: "step_latest", PageStateID: "page_console", StepIndex: 4, ActionType: registry.StepClick, TargetName: "element_65", ReasoningSummary: "选择最新的备份记录 lzxpg-1777230010。", ResultSummary: "✅ Clicked element (65)."},
			{ID: "step_node", PageStateID: "page_console", StepIndex: 5, ActionType: registry.StepClick, TargetName: "element_58", ReasoningSummary: "选择第一个节点 IP 172.25.1.21。", ResultSummary: "✅ Clicked element (58)."},
			{ID: "step_confirm", PageStateID: "page_console", StepIndex: 6, ActionType: registry.StepClick, TargetName: "element_57", ReasoningSummary: "点击\"开始恢复\"按钮执行数据恢复操作。", ResultSummary: "✅ Clicked element (57)."},
		},
	}

	guide, generated, err := NewSiteTaskGuideService(repo).UpsertGuideFromTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("UpsertGuideFromTaskRun failed: %v", err)
	}
	if !generated {
		t.Fatal("expected guide to be generated")
	}
	expected := []string{
		"点击实例名称进入详情页。",
		"进入实例数据备份页面。",
		"切换到全量备份标签页并点击数据恢复。",
		"选择最新备份记录。",
		"选择节点 IP。",
		"点击开始恢复。",
	}
	got := guideTaskGuideStepText(guide.Steps)
	if strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("expected numbered summary steps, got %#v", got)
	}
	if strings.Contains(guide.SearchableText(), "lzxpg") || strings.Contains(guide.SearchableText(), "1777230010") || strings.Contains(guide.SearchableText(), "172.25.1.21") {
		t.Fatalf("guide leaked concrete task values: %#v", guide)
	}
}

func TestSiteTaskGuideServiceReplacesExistingDirtySteps(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	taskIntent := "让 {{instance_name}} 实例基于最新备份恢复"
	intentKey := stableGuideIntentKey("ops.example.com", "", safeGuideText(taskIntent))
	if err := repo.SaveSiteTaskGuide(ctx, registry.SiteTaskGuide{
		ID:                "guide_dirty",
		ProjectID:         "default",
		Site:              "ops.example.com",
		TaskIntentKey:     intentKey,
		TaskIntentSummary: safeGuideText(taskIntent),
		Steps: []registry.SiteTaskGuideStep{
			{ID: "dirty", Index: 0, ActionType: registry.StepClick, Target: "lzxpg", Text: "点击lzxpg。"},
		},
		UIStateEntries: []registry.SiteTaskGuideUIStateEntry{{
			ID:         "state_dirty",
			Name:       "PostgreSQL",
			StateType:  registry.SiteTaskGuideUIStatePage,
			StepOffset: 0,
			Evidence: registry.SiteTaskGuideUIStateEvidence{
				TitleAny: []string{"PostgreSQL"},
			},
			MinimumScore: 0.1,
			Status:       registry.StatusActive,
		}},
		Status: registry.StatusActive,
	}); err != nil {
		t.Fatalf("SaveSiteTaskGuide failed: %v", err)
	}

	run := registry.TaskRun{
		ID:           "task_run_replaces_dirty",
		ProjectID:    "default",
		Site:         "ops.example.com",
		TaskTemplate: taskIntent,
		Status:       registry.TaskRunSuccess,
		ActionSteps: []registry.ActionStep{{
			ID:               "step_instance",
			PageStateID:      "page_console",
			StepIndex:        1,
			ActionType:       registry.StepClick,
			TargetName:       "element_33",
			ResultSummary:    "✅ Clicked element ([33]<button type=button>lzxpg />).",
			ReasoningSummary: "点击 lzxpg 实例名称按钮进入详情页。",
			BeforeObservation: &registry.PageObservationSignal{
				ProjectID:         "default",
				Site:              "ops.example.com",
				URLPattern:        "https://ops.example.com/console/",
				Title:             "PostgreSQL",
				VisibleTextSample: "PostgreSQL 实例名称",
			},
		}},
	}
	guide, generated, err := NewSiteTaskGuideService(repo).UpsertGuideFromTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("UpsertGuideFromTaskRun failed: %v", err)
	}
	if !generated {
		t.Fatal("expected guide update")
	}
	if len(guide.Steps) != 1 || guide.Steps[0].Target != "实例名称" {
		t.Fatalf("expected dirty step to be replaced, got %#v", guide.Steps)
	}
}

func TestSiteTaskGuideServiceReplacesGenericExistingStepsWithSummarySteps(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	taskIntent := "让{{instance_name}}实例基于最新的备份数据进行恢复操作"
	fullTaskTemplate := taskIntent + ":\n" +
		"1.点击实例名称 进入详情页\n" +
		"2.进入实例数据备份页面\n" +
		"3.切换到\"全量备份\"标签页并点击\"数据恢复\"\n" +
		"4.选择最新备份记录\n" +
		"5.选择节点 IP\n" +
		"6.点击\"开始恢复\""
	intentKey := stableGuideIntentKey("ops.example.com", "", safeGuideText(fullTaskTemplate))
	if err := repo.SaveSiteTaskGuide(ctx, registry.SiteTaskGuide{
		ID:                "guide_generic",
		ProjectID:         "default",
		Site:              "ops.example.com",
		TaskIntentKey:     intentKey,
		TaskIntentSummary: safeGuideText(taskIntent),
		Steps: []registry.SiteTaskGuideStep{
			{ID: "step_1", Index: 0, ActionType: registry.StepClick, Target: "实例名称", Text: "点击实例名称。"},
			{ID: "step_2", Index: 1, ActionType: registry.StepClick, Target: "数据备份", Text: "点击数据备份。"},
			{ID: "step_3", Index: 2, ActionType: registry.StepClick, Target: "数据恢复", Text: "点击数据恢复。"},
			{ID: "step_4", Index: 3, ActionType: registry.StepClick, Target: "最新的备份", Text: "点击最新的备份。"},
			{ID: "step_5", Index: 4, ActionType: registry.StepClick, Target: "下一步", Text: "点击下一步。"},
			{ID: "step_6", Index: 5, ActionType: registry.StepClick, Target: "下一步", Text: "点击下一步。"},
		},
		UIStateEntries: []registry.SiteTaskGuideUIStateEntry{{
			ID:         "state_generic",
			Name:       "PostgreSQL",
			StateType:  registry.SiteTaskGuideUIStatePage,
			StepOffset: 0,
			Evidence: registry.SiteTaskGuideUIStateEvidence{
				TitleAny: []string{"PostgreSQL"},
			},
			MinimumScore: 0.1,
			Status:       registry.StatusActive,
		}},
		Status: registry.StatusActive,
	}); err != nil {
		t.Fatalf("SaveSiteTaskGuide failed: %v", err)
	}
	if err := repo.SavePageState(ctx, registry.PageState{
		ID:             "page_console",
		ProjectID:      "default",
		Site:           "ops.example.com",
		URLPattern:     "https://ops.example.com/console/",
		CanonicalTitle: "PostgreSQL",
		RequiredText:   []string{"PostgreSQL", "数据备份", "全量备份", "数据恢复"},
		RequiredControls: []registry.ControlSignature{
			{Role: "button", Name: "数据恢复"},
		},
		Status: registry.StatusActive,
	}); err != nil {
		t.Fatalf("SavePageState failed: %v", err)
	}
	run := registry.TaskRun{
		ID:            "task_run_summary_replaces_generic",
		ProjectID:     "default",
		Site:          "ops.example.com",
		TaskTemplate:  fullTaskTemplate,
		Status:        registry.TaskRunSuccess,
		OriginalPath:  []string{"page_console"},
		OptimizedPath: []string{"page_console"},
		ActionSteps: []registry.ActionStep{{
			ID:               "step_instance",
			PageStateID:      "page_console",
			StepIndex:        1,
			ActionType:       registry.StepClick,
			TargetName:       "element_33",
			ReasoningSummary: "点击 lzxpg 实例名称按钮进入详情页。",
			ResultSummary:    "✅ Clicked element ([33]<button type=button>lzxpg />).",
		}},
	}

	guide, generated, err := NewSiteTaskGuideService(repo).UpsertGuideFromTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("UpsertGuideFromTaskRun failed: %v", err)
	}
	if !generated {
		t.Fatal("expected guide update")
	}
	joined := strings.Join(guideTaskGuideStepText(guide.Steps), "\n")
	if strings.Contains(joined, "点击下一步。") || !strings.Contains(joined, "选择节点 IP。") || !strings.Contains(joined, "点击开始恢复。") {
		t.Fatalf("expected summary steps to replace generic existing steps: %s", joined)
	}
}

func TestSiteTaskGuideServiceRefreshesWeakShellGuardsFromNewUIEvidence(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	taskIntent := "submit contract review"
	intentKey := stableGuideIntentKey("contracts.example.com", "", safeGuideText(taskIntent))
	if err := repo.SaveSiteTaskGuide(ctx, registry.SiteTaskGuide{
		ID:                "guide_contract",
		ProjectID:         "default",
		Site:              "contracts.example.com",
		TaskIntentKey:     intentKey,
		TaskIntentSummary: taskIntent,
		StartPageGuard: registry.SiteTaskGuideGuard{
			RequiredText: []string{"Enterprise Portal Shell"},
		},
		PageGuards: []registry.SiteTaskGuideGuard{{
			RequiredText: []string{"Enterprise Portal Shell"},
		}},
		Steps: []registry.SiteTaskGuideStep{
			{ID: "step_open", Index: 0, ActionType: registry.StepClick, Target: "Review Queue", Text: "Open review queue."},
			{ID: "step_submit", Index: 1, ActionType: registry.StepClick, Target: "Send for Legal Review", Text: "Send for legal review."},
		},
		UIStateEntries: []registry.SiteTaskGuideUIStateEntry{{
			ID:         "state_shell",
			Name:       "Enterprise Portal Shell",
			StateType:  registry.SiteTaskGuideUIStatePage,
			StepOffset: 0,
			Evidence: registry.SiteTaskGuideUIStateEvidence{
				TitleAny: []string{"Enterprise Portal Shell"},
			},
			MinimumScore: 1,
			Status:       registry.StatusActive,
		}},
		Status: registry.StatusActive,
	}); err != nil {
		t.Fatalf("SaveSiteTaskGuide failed: %v", err)
	}

	run := registry.TaskRun{
		ID:           "task_run_contract_refresh",
		ProjectID:    "default",
		Site:         "contracts.example.com",
		TaskTemplate: taskIntent,
		Summary:      "Contract review submitted.",
		Status:       registry.TaskRunSuccess,
		ActionSteps: []registry.ActionStep{{
			ID:            "step_open",
			StepIndex:     1,
			ActionType:    registry.StepClick,
			TargetName:    "Review Queue",
			ResultSummary: "Opened review queue.",
			BeforeObservation: &registry.PageObservationSignal{
				ProjectID:         "default",
				Site:              "contracts.example.com",
				URL:               "https://contracts.example.com/reviews",
				URLPattern:        "https://contracts.example.com/reviews",
				Title:             "Contract Review",
				VisibleTextSample: "Quarterly Contract Approval",
				ControlSignatures: []registry.ControlSignature{{Role: "button", Name: "Review Queue"}},
			},
		}, {
			ID:            "step_submit",
			StepIndex:     2,
			ActionType:    registry.StepClick,
			TargetName:    "Send for Legal Review",
			ResultSummary: "Submitted for review.",
			BeforeObservation: &registry.PageObservationSignal{
				ProjectID:         "default",
				Site:              "contracts.example.com",
				URL:               "https://contracts.example.com/reviews",
				URLPattern:        "https://contracts.example.com/reviews",
				Title:             "Contract Review",
				VisibleTextSample: "Quarterly Contract Approval",
				ControlSignatures: []registry.ControlSignature{{Role: "button", Name: "Send for Legal Review"}},
			},
		}},
	}

	guide, generated, err := NewSiteTaskGuideService(repo).UpsertGuideFromTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("UpsertGuideFromTaskRun failed: %v", err)
	}
	if !generated {
		t.Fatal("expected guide update")
	}
	guards := strings.Join(guideGuardSummariesForTest(guide), "\n")
	if strings.Contains(guards, "Enterprise Portal Shell") {
		t.Fatalf("expected weak shell guard to be replaced, got %s", guards)
	}
	if !strings.Contains(guards, "Send for Legal Review") && !strings.Contains(guards, "Quarterly Contract Approval") {
		t.Fatalf("expected refreshed guard to include current UI evidence, got %s", guards)
	}
	if len(guide.Steps) == 0 || guide.Steps[0].Text != "Open review queue." {
		t.Fatalf("expected existing steps to remain reusable, got %#v", guide.Steps)
	}
}

func TestSiteTaskGuideFeedbackUpdatesCounters(t *testing.T) {
	ctx := context.Background()
	repo, err := registry.NewFileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileRepository failed: %v", err)
	}
	guide := registry.SiteTaskGuide{
		ID:            "guide_restore",
		ProjectID:     "default",
		Site:          "ops.example.com",
		TaskIntentKey: "restore_instance",
		Summary:       "Restore instance.",
		Steps:         []registry.SiteTaskGuideStep{{Text: "Click Restore.", Target: "Restore"}},
		UIStateEntries: []registry.SiteTaskGuideUIStateEntry{{
			ID:        "state_restore",
			Name:      "Restore page",
			StateType: registry.SiteTaskGuideUIStatePage,
			Evidence: registry.SiteTaskGuideUIStateEvidence{
				ControlsAll: []registry.ControlSignature{{Role: "button", Name: "Restore"}},
			},
			MinimumScore: 1,
		}},
		Status: registry.StatusActive,
	}
	if err := repo.SaveSiteTaskGuide(ctx, guide); err != nil {
		t.Fatalf("SaveSiteTaskGuide failed: %v", err)
	}
	if err := NewSiteTaskGuideService(repo).ApplyFeedback(ctx, "guide_restore", registry.SiteTaskGuideFeedback{
		Label:  registry.SiteTaskGuideFeedbackAbandonedMismatch,
		Reason: "Current page did not match backup page guard.",
	}); err != nil {
		t.Fatalf("ApplyFeedback failed: %v", err)
	}
	updated, err := repo.GetSiteTaskGuide(ctx, "guide_restore")
	if err != nil {
		t.Fatalf("GetSiteTaskGuide failed: %v", err)
	}
	if updated.AbandonedCount != 1 {
		t.Fatalf("expected abandoned count to update: %#v", updated)
	}
}

func guideTestObservation(title, visibleText, controlName string) *registry.PageObservationSignal {
	return &registry.PageObservationSignal{
		ProjectID:         "default",
		Site:              "ops.example.com",
		URL:               "https://ops.example.com/instances",
		URLPattern:        "https://ops.example.com/instances",
		Title:             title,
		VisibleTextSample: visibleText,
		ControlSignatures: []registry.ControlSignature{{Role: "button", Name: controlName}},
	}
}

func seedGuidePages(t *testing.T, ctx context.Context, repo registry.Repository) {
	t.Helper()
	pages := []registry.PageState{
		{ID: "page_instances", ProjectID: "default", Site: "middleware.example.com", URLPattern: "https://middleware.example.com/instances", CanonicalTitle: "实例列表", RequiredText: []string{"实例名称"}, RequiredControls: []registry.ControlSignature{{Role: "link", Name: "实例名称"}}, Status: registry.StatusActive},
		{ID: "page_detail", ProjectID: "default", Site: "middleware.example.com", URLPattern: "https://middleware.example.com/instances/detail", CanonicalTitle: "实例详情", RequiredText: []string{"数据备份"}, RequiredControls: []registry.ControlSignature{{Role: "tab", Name: "数据备份"}}, Status: registry.StatusActive},
		{ID: "page_backups", ProjectID: "default", Site: "middleware.example.com", URLPattern: "https://middleware.example.com/instances/backups", CanonicalTitle: "数据备份", RequiredText: []string{"全量备份", "数据恢复"}, RequiredControls: []registry.ControlSignature{{Role: "button", Name: "数据恢复"}}, Status: registry.StatusActive},
	}
	for _, page := range pages {
		if err := repo.SavePageState(ctx, page); err != nil {
			t.Fatalf("SavePageState failed: %v", err)
		}
	}
}

func guideTaskGuideStepText(steps []registry.SiteTaskGuideStep) []string {
	result := make([]string, 0, len(steps))
	for _, step := range steps {
		result = append(result, step.Text)
	}
	return result
}

func guideGuardSummariesForTest(guide registry.SiteTaskGuide) []string {
	result := []string{}
	for _, guard := range append([]registry.SiteTaskGuideGuard{guide.StartPageGuard}, guide.PageGuards...) {
		result = append(result, strings.Join(guard.RequiredText, " "))
		for _, control := range guard.RequiredControls {
			result = append(result, control.Role+" "+control.Name)
		}
	}
	return result
}
