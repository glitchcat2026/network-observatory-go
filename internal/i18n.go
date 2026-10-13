package internal

// Add a language by registering its HTML language tag and dashboard strings here.
var Messages = map[string]map[string]string{
	"en": {
		"title":        "Network Observatory",
		"subtitle":     "Network monitoring center · Refresh every 30 seconds",
		"records":      "Last 500 Records",
		"probe_count":  "Probe Count",
		"target_count": "Target Count",
		"time":         "Time",
		"probe":        "Probe",
		"target":       "Target",
		"ip_version":   "IP Version",
		"status":       "Status",
		"min_rtt":      "Min RTT (ms)",
		"avg_rtt":      "Avg RTT (ms)",
		"max_rtt":      "Max RTT (ms)",
		"packet_loss":  "Packet Loss",
		"history":      "Probe History",
		"updated":      "Last Updated",
		"normal":       "OK",
		"failed":       "FAILED",
		"empty":        "No data yet. Waiting for Probe reports.",
		"footer":       "Data stored in SQLite · Only received probe results are saved",
	},
	"zh-cn": {
		"title":        "Network Observatory",
		"subtitle":     "网络探测数据中心 · 页面每30秒自动刷新",
		"records":      "最近500条记录",
		"probe_count":  "探测节点数",
		"target_count": "探测目标数",
		"time":         "时间",
		"probe":        "探测节点",
		"target":       "目标",
		"ip_version":   "IP 版本",
		"status":       "状态",
		"min_rtt":      "最小 RTT (ms)",
		"avg_rtt":      "平均 RTT (ms)",
		"max_rtt":      "最大 RTT (ms)",
		"packet_loss":  "丢包率",
		"history":      "探测历史",
		"updated":      "最近更新时间",
		"normal":       "正常",
		"failed":       "失败",
		"empty":        "暂无数据。等待 Probe 上报后，这里会自动显示记录。",
		"footer":       "数据存储于 SQLite · 数据库仅保存已接收的探测结果",
	},
}

func T(lang string, key string) string {
	messages, ok := Messages[lang]
	if !ok {
		messages = Messages["en"]
	}

	text, ok := messages[key]
	if !ok {
		return key
	}
	return text
}
