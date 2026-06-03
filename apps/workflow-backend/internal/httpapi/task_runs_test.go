package httpapi

func sampleTaskRunBody() map[string]any {
	return map[string]any{
		"projectId":     "default",
		"site":          "ops.example.com",
		"taskTemplate":  "查看 {{service_name}} 运行状态",
		"summary":       "查看服务运行状态。",
		"originalPath":  []string{"page_a", "page_b", "page_c", "page_a", "page_d"},
		"optimizedPath": []string{"page_a", "page_d"},
		"status":        "success",
		"actionSteps": []map[string]any{
			{
				"id":               "step_1",
				"pageStateId":      "page_a",
				"stepIndex":        1,
				"actionType":       "fill",
				"targetName":       "服务名称搜索框",
				"valueTemplate":    "{{service_name}}",
				"reasoningSummary": "使用固定搜索框定位服务。",
				"resultSummary":    "搜索已提交。",
			},
		},
	}
}
